import { formatNumber } from '../lib/format'
import { PAGE_SIZES } from '../lib/urlState'

interface Props {
  total: number
  limit: number
  offset: number
  onChange: (offset: number, limit: number) => void
}

/** Result range, page-size picker and prev/next buttons. */
export function Pagination({ total, limit, offset, onChange }: Props) {
  const from = total === 0 ? 0 : offset + 1
  const to = Math.min(offset + limit, total)
  const page = Math.floor(offset / limit) + 1
  const pages = Math.max(1, Math.ceil(total / limit))

  return (
    <nav className="pagination" aria-label="Results pages">
      <span className="muted">
        {formatNumber(from)}–{formatNumber(to)} of {formatNumber(total)}
      </span>
      <div className="pagination-controls">
        <label className="inline">
          <span>Per page</span>
          <select value={limit} onChange={(e) => onChange(0, Number(e.target.value))}>
            {PAGE_SIZES.map((n) => (
              <option key={n} value={n}>
                {n}
              </option>
            ))}
          </select>
        </label>
        <button
          type="button"
          className="btn btn-ghost btn-sm"
          disabled={offset === 0}
          onClick={() => onChange(Math.max(0, offset - limit), limit)}
        >
          ← Prev
        </button>
        <span className="muted small">
          Page {page} of {pages}
        </span>
        <button
          type="button"
          className="btn btn-ghost btn-sm"
          disabled={to >= total}
          onClick={() => onChange(offset + limit, limit)}
        >
          Next →
        </button>
      </div>
    </nav>
  )
}
