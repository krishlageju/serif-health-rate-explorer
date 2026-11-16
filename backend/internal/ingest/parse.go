// Package ingest streams an in-network rate file and normalizes it into store rows,
// decoding one array element at a time to keep memory low.
package ingest

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"ptexplorer/internal/tic"
)

// Handler receives decoded parts of an in-network file in file order.
type Handler struct {
	OnMeta              func(key, value string)
	OnProviderReference func(tic.ProviderReference)
	OnItem              func(tic.InNetworkItem)
	// OnSkip is called for an element that is valid JSON but does not match
	// the schema. Parsing continues with the next element.
	OnSkip func(section string, index int, err error)
}

// Parse walks an in-network file (gzip is auto-detected). A syntax error stops parsing;
// an element that doesn't match the schema is skipped.
func Parse(ctx context.Context, r io.Reader, h Handler) error {
	br := bufio.NewReaderSize(r, 1<<20)
	var src io.Reader = br
	if magic, err := br.Peek(2); err == nil && magic[0] == 0x1f && magic[1] == 0x8b {
		gz, err := gzip.NewReader(br)
		if err != nil {
			return fmt.Errorf("open gzip: %w", err)
		}
		defer gz.Close()
		src = gz
	}

	dec := json.NewDecoder(src)
	if err := expectDelim(dec, '{'); err != nil {
		return err
	}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		key, ok := tok.(string)
		if !ok {
			return fmt.Errorf("expected object key, got %v", tok)
		}
		switch key {
		case "provider_references":
			err = eachElement(ctx, dec, func(i int, raw json.RawMessage) {
				var ref tic.ProviderReference
				if err := json.Unmarshal(raw, &ref); err != nil {
					skip(h, key, i, err)
					return
				}
				if h.OnProviderReference != nil {
					h.OnProviderReference(ref)
				}
			})
		case "in_network":
			err = eachElement(ctx, dec, func(i int, raw json.RawMessage) {
				var item tic.InNetworkItem
				if err := json.Unmarshal(raw, &item); err != nil {
					skip(h, key, i, err)
					return
				}
				if h.OnItem != nil {
					h.OnItem(item)
				}
			})
		default:
			var raw json.RawMessage
			if err = dec.Decode(&raw); err == nil && h.OnMeta != nil {
				var s string
				if json.Unmarshal(raw, &s) == nil {
					h.OnMeta(key, s)
				}
			}
		}
		if err != nil {
			return fmt.Errorf("%s: %w", key, err)
		}
	}
	return expectDelim(dec, '}')
}

// skip reports a skipped element to the handler, if it has an OnSkip callback.
func skip(h Handler, section string, i int, err error) {
	if h.OnSkip != nil {
		h.OnSkip(section, i, err)
	}
}

// expectDelim reads the next token and checks it is the given delimiter.
func expectDelim(dec *json.Decoder, want json.Delim) error {
	tok, err := dec.Token()
	if err != nil {
		return fmt.Errorf("expected %q: %w", want, err)
	}
	if d, ok := tok.(json.Delim); !ok || d != want {
		return fmt.Errorf("expected %q, got %v", want, tok)
	}
	return nil
}

// eachElement calls fn for every element of the array at the decoder's
// position. A null value is treated as an empty array.
func eachElement(ctx context.Context, dec *json.Decoder, fn func(int, json.RawMessage)) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	if tok == nil {
		return nil
	}
	if d, ok := tok.(json.Delim); !ok || d != '[' {
		return fmt.Errorf("expected array, got %v", tok)
	}
	for i := 0; dec.More(); i++ {
		if i%1000 == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return err
		}
		fn(i, raw)
	}
	_, err = dec.Token() // closing ]
	return err
}
