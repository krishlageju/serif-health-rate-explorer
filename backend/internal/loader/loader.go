// Package loader runs ingestion in the background (index → rate files → store) and
// keeps a status record the API exposes for progress.
package loader

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime/debug"
	"sync"
	"time"

	"ptexplorer/internal/index"
	"ptexplorer/internal/ingest"
	"ptexplorer/internal/store"
	"ptexplorer/internal/tic"
)

// Fetcher gets a file by URL or local path. *fetch.Client implements it.
type Fetcher interface {
	// Open streams a file without caching (used for the small index file).
	Open(ctx context.Context, loc string) (io.ReadCloser, int64, error)
	// Cached downloads a file once and returns a local path to it.
	Cached(ctx context.Context, loc string, progress func(done, total int64)) (path string, cached bool, err error)
}

// Parser turns one in-network file into normalized rows. ingest.Parser implements it.
type Parser interface {
	Parse(ctx context.Context, r io.Reader) (*ingest.Result, error)
}

// Publisher receives the finished store. *store.Holder implements it; a future version
// could write to a database instead.
type Publisher interface {
	Publish(*store.Store)
}

// Deps are the collaborators a Loader needs. They are passed in (not created here)
// so each one can be swapped for a fake in tests or a remote service later.
type Deps struct {
	Fetcher   Fetcher
	Parser    Parser
	Publisher Publisher
	Log       *slog.Logger
}

// State is the overall load state, or one file's state.
type State string

const (
	StateStarting   State = "starting"
	StateLoading    State = "loading"
	StateReady      State = "ready"
	StatePartial    State = "partial" // some files failed, others loaded
	StateError      State = "error"
	FileQueued      State = "queued"
	FileDownloading State = "downloading"
	FileParsing     State = "parsing"
	FileNotSelected State = "not_selected"
)

// FileStatus is the progress and outcome of one in-network file.
type FileStatus struct {
	URL         string            `json:"url"`
	FileName    string            `json:"file_name"`
	PlanCount   int               `json:"plan_count"`
	Selected    bool              `json:"selected"`
	State       State             `json:"state"`
	FromCache   bool              `json:"from_cache"`
	BytesDone   int64             `json:"bytes_done"`
	BytesTotal  int64             `json:"bytes_total"`
	Rows        int               `json:"rows"`
	Stats       *ingest.FileStats `json:"stats,omitempty"`
	SkipSamples []string          `json:"skip_samples,omitempty"`
	Error       string            `json:"error,omitempty"`
	LastUpdated string            `json:"last_updated_on,omitempty"`
	DurationMS  int64             `json:"duration_ms,omitempty"`
}

// IndexInfo describes the index (table-of-contents) file that was read.
type IndexInfo struct {
	URL                 string `json:"url"`
	ReportingEntityName string `json:"reporting_entity_name"`
	ReportingEntityType string `json:"reporting_entity_type"`
	LastUpdatedOn       string `json:"last_updated_on"`
	Version             string `json:"version"`
	Structures          int    `json:"reporting_structures"`
}

// Status is everything the UI needs to show load progress.
type Status struct {
	State     State        `json:"state"`
	Message   string       `json:"message,omitempty"`
	Index     *IndexInfo   `json:"index,omitempty"`
	Files     []FileStatus `json:"files"`
	Totals    *store.Stats `json:"totals,omitempty"`
	StartedAt time.Time    `json:"started_at"`
	ReadyAt   *time.Time   `json:"ready_at,omitempty"`
}

// progressInterval limits how often download/parse progress updates the status.
const progressInterval = 200 * time.Millisecond

// Config selects the index file and which of its in-network files to load.
type Config struct {
	IndexURL string
	Files    string // see index.Select
}

// Loader runs ingestion once and records its progress. It is the write side of the app.
type Loader struct {
	cfg  Config
	deps Deps
	log  *slog.Logger

	mu      sync.RWMutex
	status  Status
	sources []index.SourceFile // every in-network file in the index
}

// New creates a Loader from its config and dependencies; call Run to start loading.
func New(cfg Config, deps Deps) *Loader {
	return &Loader{cfg: cfg, deps: deps, log: deps.Log, status: Status{State: StateStarting, Files: []FileStatus{}, StartedAt: time.Now()}}
}

// Status returns a copy of the current ingestion status.
func (l *Loader) Status() Status {
	l.mu.RLock()
	defer l.mu.RUnlock()
	s := l.status
	s.Files = append([]FileStatus(nil), l.status.Files...)
	return s
}

// IndexFiles returns every in-network file listed in the index.
func (l *Loader) IndexFiles() []index.SourceFile {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.sources
}

// update changes the status under the lock.
func (l *Loader) update(fn func(*Status)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fn(&l.status)
}

// updateFile changes one file's status under the lock.
func (l *Loader) updateFile(i int, fn func(*FileStatus)) {
	l.update(func(s *Status) { fn(&s.Files[i]) })
}

// fail logs err and marks the whole load as failed.
func (l *Loader) fail(msg string, err error) {
	l.log.Error(msg, "err", err)
	l.update(func(s *Status) {
		s.State = StateError
		s.Message = fmt.Sprintf("%s: %v", msg, err)
	})
}

// Run loads everything. It never panics; failures end up in Status.
func (l *Loader) Run(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			l.fail("ingestion crashed", fmt.Errorf("%v", r))
		}
	}()

	l.update(func(s *Status) { s.State = StateLoading; s.Message = "Reading index file" })
	idx, err := l.readIndex(ctx)
	if err != nil {
		l.fail("could not read index file", err)
		return
	}

	all := index.InNetworkFiles(idx)
	selected, err := index.Select(all, l.cfg.Files)
	if err != nil {
		l.fail("could not select in-network files", err)
		return
	}
	picked := map[string]bool{}
	for _, f := range selected {
		picked[f.URL] = true
	}

	l.mu.Lock()
	l.sources = all
	l.status.Index = &IndexInfo{
		URL:                 l.cfg.IndexURL,
		ReportingEntityName: idx.ReportingEntityName,
		ReportingEntityType: idx.ReportingEntityType,
		LastUpdatedOn:       idx.LastUpdatedOn,
		Version:             idx.Version,
		Structures:          len(idx.ReportingStructure),
	}
	fileIdx := map[string]int{}
	for _, f := range all {
		st := FileStatus{URL: f.URL, FileName: f.FileName, PlanCount: len(f.Plans), Selected: picked[f.URL], State: FileNotSelected}
		if st.Selected {
			st.State = FileQueued
			fileIdx[f.URL] = len(l.status.Files)
		}
		l.status.Files = append(l.status.Files, st)
	}
	l.mu.Unlock()
	l.log.Info("index loaded", "entity", idx.ReportingEntityName, "files", len(all), "selected", len(selected))

	b := store.NewBuilder()
	loaded, failed := 0, 0
	for _, f := range selected {
		i := fileIdx[f.URL]
		if err := l.loadFile(ctx, i, f, idx, b); err != nil {
			failed++
			l.log.Error("file failed", "file", f.FileName, "err", err)
			l.updateFile(i, func(fs *FileStatus) { fs.State = StateError; fs.Error = err.Error() })
			continue
		}
		loaded++
	}

	if loaded == 0 {
		l.fail("no in-network files could be loaded", fmt.Errorf("%d of %d failed", failed, len(selected)))
		return
	}
	st := b.Build()
	l.deps.Publisher.Publish(st)
	// Parsing leaves a lot of garbage behind; hand it back to the OS now
	// rather than holding peak memory for the life of the process.
	debug.FreeOSMemory()
	stats := st.Stats()
	now := time.Now()
	l.update(func(s *Status) {
		s.State = StateReady
		s.Message = ""
		if failed > 0 {
			s.State = StatePartial
			s.Message = fmt.Sprintf("%d of %d files failed to load; results may be incomplete", failed, len(selected))
		}
		s.Totals = &stats
		s.ReadyAt = &now
	})
	l.log.Info("store ready", "rows", stats.Rows, "providers", stats.Providers, "npis", stats.NPIs, "took", time.Since(l.status.StartedAt).Round(time.Millisecond))
}

// readIndex fetches and parses the configured index file.
func (l *Loader) readIndex(ctx context.Context) (*tic.Index, error) {
	rc, _, err := l.deps.Fetcher.Open(ctx, l.cfg.IndexURL)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return index.Parse(rc)
}

// loadFile downloads (or reuses) one rate file, parses it and adds it to b.
func (l *Loader) loadFile(ctx context.Context, i int, f index.SourceFile, idx *tic.Index, b *store.Builder) error {
	start := time.Now()
	l.updateFile(i, func(fs *FileStatus) { fs.State = FileDownloading })

	var lastTick time.Time
	progress := func(done, total int64) {
		if time.Since(lastTick) < progressInterval && done != total {
			return
		}
		lastTick = time.Now()
		l.updateFile(i, func(fs *FileStatus) { fs.BytesDone, fs.BytesTotal = done, total })
	}

	path, fromCache, err := l.deps.Fetcher.Cached(ctx, f.URL, progress)
	if err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	size := int64(-1)
	if st, err := file.Stat(); err == nil {
		size = st.Size()
	}

	l.updateFile(i, func(fs *FileStatus) {
		fs.State = FileParsing
		fs.FromCache = fromCache
		fs.BytesDone = 0
		fs.BytesTotal = size
	})
	res, err := l.deps.Parser.Parse(ctx, &countingReader{r: file, total: size, progress: progress})
	if err != nil {
		return fmt.Errorf("parse %s: %w", f.FileName, err)
	}
	data, stats, skips := res.Data, res.Stats, res.SkipSamples

	b.AddFile(store.Source{
		URL:                 f.URL,
		FileName:            f.FileName,
		Description:         f.Description,
		Plans:               f.Plans,
		ReportingEntityName: idx.ReportingEntityName,
	}, data)

	l.updateFile(i, func(fs *FileStatus) {
		fs.State = StateReady
		fs.Rows = len(data.Rows)
		fs.Stats = &stats
		fs.SkipSamples = skips
		fs.LastUpdated = data.Meta["last_updated_on"]
		fs.BytesDone = size
		fs.DurationMS = time.Since(start).Milliseconds()
	})
	l.log.Info("file loaded", "file", f.FileName, "rows", len(data.Rows), "skipped", stats.Skipped, "cache", fromCache, "took", time.Since(start).Round(time.Millisecond))
	return nil
}

// countingReader reports how many bytes have been read, to show parse progress.
type countingReader struct {
	r        io.Reader
	n, total int64
	progress func(done, total int64)
}

// Read passes bytes through and reports the running total.
func (c *countingReader) Read(b []byte) (int, error) {
	n, err := c.r.Read(b)
	c.n += int64(n)
	c.progress(c.n, c.total)
	return n, err
}
