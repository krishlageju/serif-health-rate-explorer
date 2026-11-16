import type { RateResult, SortKey, SortOrder } from '../api/types'

/** Which column the results table is sorted by, and in which direction. */
export interface SortState {
  key: SortKey
  order: SortOrder
}

// Results arrive from the API ordered by rate, low to high.
export const defaultSort: SortState = { key: 'rate', order: 'asc' }

// Joins list fields so they compare as one string.
const joined = (xs: string[]) => xs.join(',')
// First provider's name, compared case-insensitively.
const providerName = (r: RateResult) => (r.providers[0]?.business_name ?? '').toLowerCase()

const compareBy: Record<SortKey, (a: RateResult, b: RateResult) => number> = {
  rate: (a, b) => a.rate - b.rate,
  code: (a, b) => a.billing_code.localeCompare(b.billing_code) || a.name.localeCompare(b.name),
  billing: (a, b) =>
    a.billing_class.localeCompare(b.billing_class) ||
    joined(a.service_codes).localeCompare(joined(b.service_codes)) ||
    joined(a.modifiers).localeCompare(joined(b.modifiers)),
  provider: (a, b) => providerName(a).localeCompare(providerName(b)),
}

/** Sorts the rows already on screen (no API call). Direction applies to the chosen column;
 * ties fall back to rate low to high, then id. */
export function sortResults(rows: RateResult[], { key, order }: SortState): RateResult[] {
  const dir = order === 'asc' ? 1 : -1
  return [...rows].sort((a, b) => dir * compareBy[key](a, b) || a.rate - b.rate || a.id - b.id)
}

/** Clicking the active column flips direction; a new column starts ascending. */
export function nextSort(current: SortState, key: SortKey): SortState {
  if (current.key === key) return { key, order: current.order === 'asc' ? 'desc' : 'asc' }
  return { key, order: 'asc' }
}
