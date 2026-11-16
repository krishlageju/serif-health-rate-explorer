import type { Hint, SearchParams } from '../api/types'

// Example searches that return results with the default Fidelis file.
const EXAMPLES: { label: string; params: Partial<SearchParams> }[] = [
  { label: '99213 · office visit', params: { code: '99213' } },
  { label: '45380 · colonoscopy with biopsy', params: { code: '45380' } },
  { label: '490 · ambulatory surgery (revenue code)', params: { code: '490' } },
  { label: '45380 + NPI 1073587416', params: { code: '45380', npi: '1073587416' } },
]

/** Shown before the first search: what to type, plus clickable example searches. */
export function Welcome({ ready, onExample }: { ready: boolean; onExample: (p: Partial<SearchParams>) => void }) {
  return (
    <section className="card empty">
      <h2>Search by billing code</h2>
      <p className="muted">
        Enter a CPT, HCPCS or revenue code. Add an NPI or EIN to narrow results to one provider. Facility rates filed
        under a revenue code that reference your code are included too.
      </p>
      <div className="examples">
        <span className="muted small">Try:</span>
        {EXAMPLES.map((ex) => (
          <button
            key={ex.label}
            type="button"
            className="chip chip-btn"
            disabled={!ready}
            onClick={() => onExample(ex.params)}
          >
            {ex.label}
          </button>
        ))}
      </div>
    </section>
  )
}

/** Shown when the search request itself failed (network or server error). */
export function SearchError({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <section className="card empty" role="alert">
      <h2>Search failed</h2>
      <p className="muted">{message}</p>
      <button type="button" className="btn btn-ghost" onClick={onRetry}>
        Try again
      </button>
    </section>
  )
}

interface NoResultsProps {
  hint: Hint
  params: SearchParams
  onChange: (p: Partial<SearchParams>) => void
}

/** Shown for an empty result: the server's explanation plus one-click ways to widen the search. */
export function NoResults({ hint, params, onChange }: NoResultsProps) {
  return (
    <section className="card empty" role="status">
      <h2>No matching rates</h2>
      <p>{hint.message}</p>
      <div className="examples">
        {(params.npi || params.ein) && (
          <button type="button" className="chip chip-btn" onClick={() => onChange({ npi: '', ein: '', offset: 0 })}>
            Remove provider filters
          </button>
        )}
        {(params.code_type || params.billing_class) && (
          <button
            type="button"
            className="chip chip-btn"
            onClick={() => onChange({ code_type: '', billing_class: '', offset: 0 })}
          >
            Remove code type and billing class filters
          </button>
        )}
        {!params.include_related && (
          <button
            type="button"
            className="chip chip-btn"
            onClick={() => onChange({ include_related: true, offset: 0 })}
          >
            Include facility rates
          </button>
        )}
      </div>
    </section>
  )
}
