# Serif Rate Explorer: Searchable Price Transparency Explorer

A small full-stack app that starts from a CMS Transparency in Coverage **index file** and uses it to choose an **in-network rate file**. It downloads and normalizes that file, then lets you search negotiated rates by **billing code**, optionally narrowed by **NPI** and/or **EIN**.

- **Backend:** Go 1.25+ with Gin. It streams the JSON, keeps the data in memory and serves a JSON API.
- **Frontend:** React 18, TypeScript, Vite and TanStack Query.
- **Data:** Centene / Fidelis Care index `2026-08-31_fidelis_index.json`.

---

## Quick start

Requirements: Go ≥ 1.25, Node ≥ 18.

```bash
make install   # go mod download + npm install
make dev       # API on :8080, UI on http://localhost:5173
```

Or as a single process, with the API and the built UI both on http://localhost:8080:

```bash
make start
```

Without `make`:

```bash
cd backend && go run ./cmd/server          # terminal 1
cd frontend && npm install && npm run dev  # terminal 2 → http://localhost:5173
```

**The first run downloads the selected rate file (89 MB, a few seconds).** It is cached in `backend/data/cache/`, so later starts load in under a second. The server starts serving right away; the UI shows a progress banner until the data is ready.

Checks:
- `make test`: `go vet`, the Go tests with the race detector, the TypeScript type check and the Vitest tests.
- `make lint`: `gofmt`, `go vet`, Prettier and ESLint (with the React hooks rules). `make format` fixes formatting.
- CI (`.github/workflows/ci.yml`) runs the same checks plus both builds on every push and pull request.

### Configuration (environment variables)

| Variable | Default | Meaning |
|---|---|---|
| `INDEX_URL` | Fidelis index URL | Index (table-of-contents) file. A URL or a local path. |
| `TIC_FILES` | `first` | Which in-network files from the index to load: `first`, `all`, or comma-separated URL substrings (e.g. `fidelis-es`). |
| `CACHE_DIR` | `./data/cache` | Where downloaded rate files are kept. |
| `PORT` | `8080` | API port. |
| `STATIC_DIR` | `../frontend/dist` | Built UI to serve, if present. |
| `FETCH_TIMEOUT` | `15m` | Timeout for each download. |
| `CORS_ORIGINS` | (empty) | Comma-separated browser origins allowed to call the API, for when the UI is hosted separately. Empty means same-origin only. |

Frontend: set `VITE_API_BASE_URL` (e.g. in `frontend/.env.local`) to point the UI at an API on another host. Empty means same origin, which is what `make dev` (Vite proxy) and `make start` use.

If the index URL goes stale, a newer one is listed at https://www.centene.com/price-transparency-files.html. Set it with `INDEX_URL=…`.

---

## What the data looks like

I profiled the files before designing anything. These findings drove most of the decisions below.

**Index:** 2 reporting structures. Each points to **one** in-network file:

| File | Plans | Size |
|---|---|---|
| `…fidelis-ex_in-network.json` | 88 Ambetter from Fidelis Care plans | 89 MB |
| `…fidelis-es_in-network.json` | 6 plans | 174 MB |

**In-network file (`fidelis-ex`):** 1,519 billing codes (CPT 607, HCPCS 78, revenue codes 834), 179,948 price entries, 1,355 provider references (8,030 TIN groups, about 43.8k NPIs).

Four things matter for search:

1. **NPI and EIN are not on the price.** Each `negotiated_rate` holds a `provider_references: [id]`, and the NPI list and EIN live in the top-level `provider_references` block. Search has to join the two.
2. **Most facility rates hide the procedure code.** About 154k of the 180k prices are revenue-code rows (e.g. RC `490`, ambulatory surgery) that carry a CPT code in `additional_information` (e.g. `"45380"`, colonoscopy with biopsy). Matching only `billing_code` would miss most of the facility prices for a procedure.
3. **The same price is repeated under unrelated names.** RC `490` appears as 162 separate `in_network` items ("KNEE ARTHROSCOPY/SURGERY", "COLONOSCOPY W/LESION REMOVAL", …) with identical prices for identical providers. The 179,948 entries contain only **7,727 distinct rates**. Without merging, a search for 45380 returns 6,815 rows of which 53 are distinct, and the median is skewed by the repeats.
4. All values are `ffs` / `negotiated` / `outpatient` / `9999-12-31` in this file. The code still treats them as data rather than constants.

---

## Architecture and data flow

```
 index.json ──► index.Parse ──► InNetworkFiles (dedupe URLs, keep plans per file)
                                     │ index.Select(TIC_FILES)
                                     ▼
 rate file  ──► fetch.Cached (download once → disk cache, atomic rename)
                                     ▼
                ingest.Parse   json.Decoder token walk, one array element at a time,
                               gzip auto-detected, bad elements skipped and counted
                                     ▼
                ingest.Load    flatten in_network → negotiated_rates → negotiated_prices,
                               resolve provider_group_id → provider groups after the pass
                                     ▼
                store.Build    merge duplicate prices, build indexes:
                               code→rows, additional_information→rows, NPI→groups, EIN→groups
                                     ▼  atomic.Pointer swap (readers never see a partial store)
                api (Gin)      /api/status /api/sources /api/search /api/codes
                                     ▼
                React UI       search form + autocomplete, status banner, summary,
                               results table/cards with column sorting (current page),
                               pagination, URL-synced state, data-source footer
```

```
backend/
  cmd/server/          main: read config, build the app, run until Ctrl+C
  internal/app/        composition root: creates each service and wires them through interfaces
  internal/tic/        CMS schema structs (tolerant of string/number variations)
  internal/index/      parse index, list files with their plans, select files
  internal/fetch/      URL or local file, disk cache, progress reporting
  internal/ingest/     streaming parser + normalizer
  internal/store/      in-memory store, dedupe, indexes, Search, Suggest; Holder shares the current store
  internal/loader/     ingestion (write side): index → files → store, plus progress status
  internal/search/     search service (read side): runs queries, shapes results for the API
  internal/api/        Gin handlers: validation, JSON encoding, error mapping
  testdata/            small hand-written fixture covering edge cases
frontend/src/
  api/                 typed client + response types
  components/          SearchForm, CodeInput, StatusBanner, SummaryBar, ResultsToolbar, ResultsList,
                       Pagination, EmptyStates, SourcesPanel, ErrorBoundary
  lib/                 formatting, code labels, validation (incl. NPI checksum), URL state, page sorting
```

### Service boundaries and dependency injection

Each part depends on a small interface, not on another part's concrete code. Only `internal/app` (the composition root) chooses the implementations and connects them:

```
api ──Searcher──────▶ search.Service ──StoreReader──▶ store.Holder ◀──Publisher── loader
  └─StatusReader──────────────────────────────────────────────────────────────▶ loader ──Fetcher──▶ fetch.Client
                                                                                         └─Parser───▶ ingest.Parser
```

| Interface | Defined in (consumer) | Implemented by today | Could become |
|---|---|---|---|
| `api.Searcher` | api | `search.Service` | an HTTP/gRPC client for a separate search service |
| `api.StatusReader` | api | `loader.Loader` | a client for a separate ingestion service |
| `search.StoreReader` | search | `store.Holder` | a database-backed reader |
| `loader.Publisher` | loader | `store.Holder` | a database writer or message-queue publisher |
| `loader.Fetcher` | loader | `fetch.Client` | object storage (S3/GCS) |
| `loader.Parser` | loader | `ingest.Parser` | a parsing worker pool |

Interfaces live in the package that uses them (the Go convention), so splitting into microservices means writing a new implementation and changing one line in `internal/app`, with no edits to the handlers or services. `app.go` also has compile-time checks that each implementation still satisfies its interface. The tests use the same seam: `api_test.go` runs the router against fake services.

The frontend follows the same idea: components get an `ApiClient` from React context (`useApi()`), and `main.tsx` is the one place that creates it. Tests (or a different backend) can supply another client.

**How the work is split:** the backend owns fetching, parsing, normalizing and filtering. The browser sends queries, renders a page of results and sorts that page locally, because about 8k distinct rates and 44k NPIs are too much to ship and filter client-side.

---

## API

`GET /api/search`

| Param | Required | Notes |
|---|---|---|
| `code` | yes | Trimmed and upper-cased. Letters, digits, `.` and `-`. |
| `npi` | no | Exactly 10 digits. |
| `ein` | no | 9 digits, with or without the dash. |
| `code_type` | no | `CPT`, `HCPCS`, `RC`, … (filters on the row's type) |
| `billing_class` | no | `professional` or `institutional` |
| `include_related` | no | Default `true`: also match rates whose `additional_information` is this code. |
| `sort` | no | Column to sort by: `rate` (default), `code`, `billing`, `provider`. Sorts all matches, not just the page. |
| `order` | no | `asc` (default) or `desc`. Ties are broken by rate, low to high. |
| `limit` / `offset` | no | Page size 1–200 (default 25). |

The response holds the normalized query, `total`, a `summary` over **all** matches (count, min, median, max, mean, distinct EINs, direct vs. facility rows) and one page of `results`. Each result carries:
- `match`: either `billing_code` or `additional_information`.
- Every provider group on the rate (EIN, business name, NPI count, first 25 NPIs, and `matched: true` for the ones that satisfy the NPI/EIN filter, listed first).
- `listings` and `other_names` for merged duplicates.
- A `source` block: file name, URL, `last_updated_on`, number of plans.

**Empty results come with a `hint`** that says why, so the UI can show a useful message:

| `hint.code` | Meaning |
|---|---|
| `code_not_found` | No rate for this code in the loaded files. |
| `npi_not_found` / `ein_not_found` | That identifier isn't in any provider group in the file. |
| `npi_ein_mismatch` | Both exist, but never in the same provider group. |
| `no_rates_for_provider` | The provider is in the file but has no rate for this code. |
| `filtered_out` | The code has rates, but the code type / billing class filters remove them all. |

Other endpoints:
- `/api/status`: ingestion state (`loading`, `ready`, `partial`, `error`), per-file progress and counts.
- `/api/sources`: index metadata, all in-network files and their plans.
- `/api/codes?q=`: autocomplete by code prefix or by name. Returns every match by default (the full code list is about 1.1k entries); pass `limit` to cap it.

Invalid input returns **400** with `{error: {code, message, field}}`. Searching before the data is ready returns **503** with the current status.

---

## Design decisions and tradeoffs

- **Use the index as the source of truth.** File URLs come from `reporting_structure[].in_network_files`. URLs listed more than once are merged, with all their plans attached for traceability. `TIC_FILES=first` (default) loads the 89 MB file so the first run is quick. `TIC_FILES=all` loads both (about 263 MB, 540k price entries → 17,549 distinct rates, about 6 s including the download) through the same code path.
- **Stream the parse instead of `json.Unmarshal` on the whole file.** Decoding one `provider_references` or `in_network` element at a time keeps memory near the size of the normalized data. It also works whatever order the two arrays appear in: references are resolved after the pass, so a file that lists rates first still works (the test fixture does this on purpose).
- **Cache the download on disk.** It is written to a temp file and renamed only when complete, so an interrupted download never looks like a valid cache. The tradeoff is no freshness check yet (see below).
- **Flatten to one row per negotiated price, then merge exact duplicates.** The key is everything except the item name and description. The other names are kept (`other_names`, `listings`), so the source data is still traceable from the UI's details panel.
- **Match related codes by default.** A search for a CPT code also returns facility rates whose `additional_information` holds that code. They get a "references 45380" badge, are counted separately in the summary, and can be turned off with a checkbox. Values in `additional_information` are indexed only when they look like a code (`^[A-Z0-9]{3,7}$`), so free text is ignored.
- **Column sorting in the UI happens in the browser.** Clicking a header reorders the rows already on the current page, with no extra request. The API also accepts `sort`/`order` to sort across all matches, for clients that need that.
- **NPI + EIN means the same provider group.** When both are given, the NPI must belong to the group with that EIN. Otherwise you'd get "NPI X's rates or EIN Y's rates", which isn't what someone asking about a specific provider means.
- **Facility rows get a meaningful label.** For revenue-code rows that were published under many item names, the UI shows the revenue code's meaning (a small UB-04 lookup) plus the referenced procedure and its CPT name, instead of a misleading item name.
- **Resilient parsing.** Codes, NPIs, TINs and rates accept either strings or numbers. An element that is valid JSON but doesn't fit the schema is skipped and counted, with the first few errors kept in `/api/status`. A file that fails completely is marked `error`, and other files still load (state `partial`). The loader recovers from panics, and Gin's recovery middleware covers the handlers.
- **In-memory store behind an atomic pointer.** Searches take no locks, and a future reload can build a new store and swap it in. Search is a map lookup plus a linear filter over that code's rows. The largest code (RC 490, 486 distinct rates) returns in about 0.2 ms server-side.

### Assumptions

- One loaded file per reporting structure is enough to show the flow. All plans attached to a file are treated as sharing its rates. There is no plan-level filtering.
- `expiration_date: 9999-12-31` means "no expiration".
- EINs are compared on digits only (`12-3456789` == `123456789`).
- NPI format is validated strictly (10 digits). The NPI check digit only produces a warning in the UI, because published files sometimes contain NPIs that fail it.

### Known limitations

- `provider_references[].location` (provider groups stored in a separate remote file) is counted but not followed. The Fidelis files don't use it.
- `bundled_codes`, `covered_services` and allowed-amount files are ignored.
- No freshness check on the cache: delete `backend/data/cache` (`make clean-cache`) to pick up a new file.
- Provider names are TIN business names. Individual NPIs aren't resolved to people or addresses.
- Memory scales with the data. After loading, the live Go heap is about 23 MB for the default file and about 60 MB for both files. Peak memory during the parse is higher (about 650 MB for both files), because every raw price is held until duplicates are merged. That's fine here, but not for the multi-GB files other payers publish; merging per file while streaming would cut the peak.

---

## What I'd do with more time

1. **Storage:** load normalized rows into Postgres or DuckDB, with indexes on code, NPI and EIN. That allows payers with multi-GB files, restarts without re-parsing, and SQL-backed analytics.
2. **Freshness:** re-read the index on a schedule. When `last_updated_on` or the file URL changes, build a new store in the background and swap it in atomically (the store is already immutable).
3. **More filters:** plan, issuer, place of service and setting. Rate comparison across providers (a histogram or box plot per code). CSV export.
4. **Provider enrichment:** look up NPIs in the NPPES registry to show provider name, specialty and location, then search by provider name or zip code.
5. **Scale the parser:** parse multiple files concurrently; follow remote `provider_references`; stream directly from HTTP with a range-resumable download.
6. **Tests:** handler fuzzing on query params, a Playwright smoke test for the main search flows, and a golden test against a trimmed real file.

---

## Code conventions

- Every function has a short comment (two lines at most) saying what it does. Go comments start with the function name, as `go doc` expects.
- Formatting is automatic: `gofmt` for Go, Prettier for the frontend. ESLint and `go vet` catch common mistakes. CI fails on any of these.
- Handlers stay thin: validation and encoding live in `internal/api`, result shaping in `internal/search`, query logic in `internal/store`, and parsing in `internal/ingest`.
- Dependencies are injected, never created inside a service: constructors take a `Deps` struct of interfaces, and `internal/app` wires them.
- The UI never crashes to a blank page: an error boundary shows a message with a reload button.

## Review follow-ups (not done, would change behavior)

A self-review found these. They were left as-is because each one changes what the app does:

- **Self-healing cache:** if a cached rate file fails to parse, delete it so the next start downloads a fresh copy. Today you must run `make clean-cache`.
- **Provider count with a filter:** with an NPI/EIN filter, the "Providers (EINs)" tile still counts every group on the matching rates, not only the matching provider.
- **Unsent form edits:** changing page resets the form to the last submitted search, which drops anything typed but not yet submitted.
- **Sort on new search:** the chosen column sort carries over to a new code's results; resetting it to "rate, low to high" may be less surprising.
- **Hint wording in the API:** hint messages are built in the store package. Moving the text to the API layer would keep the domain code free of display strings.
