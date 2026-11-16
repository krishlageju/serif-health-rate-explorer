import type { SearchParams, SortKey, SortOrder } from '../api/types'
import type { SortState } from '../lib/sortResults'

interface Props {
  params: SearchParams
  sort: SortState
  onSort: (s: SortState) => void
}

/** Active-filter chips, plus a sort menu for phones (where the column headers are hidden). */
export function ResultsToolbar({ params, sort, onSort }: Props) {
  return (
    <div className="toolbar">
      <ActiveFilters params={params} />
      <label className="inline sort-mobile">
        <span>Sort</span>
        <select
          value={`${sort.key}:${sort.order}`}
          onChange={(e) => {
            const [key, order] = e.target.value.split(':') as [SortKey, SortOrder]
            onSort({ key, order })
          }}
        >
          <option value="rate:asc">Rate: low to high</option>
          <option value="rate:desc">Rate: high to low</option>
          <option value="code:asc">Billing code: A to Z</option>
          <option value="code:desc">Billing code: Z to A</option>
          <option value="billing:asc">Billing class: A to Z</option>
          <option value="billing:desc">Billing class: Z to A</option>
          <option value="provider:asc">Provider: A to Z</option>
          <option value="provider:desc">Provider: Z to A</option>
        </select>
      </label>
    </div>
  )
}

/** Chips summarizing the active filters above the results. */
function ActiveFilters({ params }: { params: SearchParams }) {
  const parts = [`Code ${params.code}`]
  if (params.npi) parts.push(`NPI ${params.npi}`)
  if (params.ein) parts.push(`EIN ${params.ein}`)
  if (params.code_type) parts.push(params.code_type)
  if (params.billing_class) parts.push(params.billing_class)
  if (!params.include_related) parts.push('direct only')
  return (
    <div className="active-filters small">
      {parts.map((p) => (
        <span key={p} className="chip">
          {p}
        </span>
      ))}
    </div>
  )
}
