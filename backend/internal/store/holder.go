package store

import "sync/atomic"

// Holder keeps the current Store. Ingestion publishes into it and search reads from it,
// so the two sides never reference each other directly.
type Holder struct {
	current atomic.Pointer[Store]
}

// Publish swaps in a fully built store; readers see the old or new one, never a partial one.
func (h *Holder) Publish(s *Store) { h.current.Store(s) }

// Current returns the latest published store, or nil while nothing has loaded yet.
func (h *Holder) Current() *Store { return h.current.Load() }
