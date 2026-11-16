import { describe, expect, it } from 'vitest'
import type { RateResult } from '../api/types'
import { formatEINInput } from './format'
import { defaultSort, nextSort, sortResults, type SortState } from './sortResults'
import { emptySearch, parseSearch, toQueryString } from './urlState'
import { isValidNPIChecksum, npiWarning, validateSearch } from './validate'

describe('validateSearch', () => {
  it('requires a code', () => {
    expect(validateSearch('  ', '', '').code).toBeDefined()
  })
  it('accepts a code with optional NPI and EIN', () => {
    expect(validateSearch(' 99213 ', '1234567893', '12-3456789')).toEqual({})
    expect(validateSearch('99213', '', '123456789')).toEqual({})
  })
  it('rejects malformed NPI, EIN and code', () => {
    const e = validateSearch('<x>', '12345', '12-34')
    expect(e.code).toBeDefined()
    expect(e.npi).toBeDefined()
    expect(e.ein).toBeDefined()
  })
})

describe('NPI checksum', () => {
  it('validates with the 80840 Luhn prefix', () => {
    expect(isValidNPIChecksum('1234567893')).toBe(true)
    expect(isValidNPIChecksum('1234567890')).toBe(false)
  })
  it('only warns, never blocks', () => {
    expect(npiWarning('1234567890')).toBeDefined()
    expect(npiWarning('1234567893')).toBeUndefined()
    expect(npiWarning('123')).toBeUndefined()
  })
})

describe('formatEINInput', () => {
  it('formats as the user types', () => {
    expect(formatEINInput('12')).toBe('12')
    expect(formatEINInput('123')).toBe('12-3')
    expect(formatEINInput('12-3456789999')).toBe('12-3456789')
    expect(formatEINInput('ab12cd345')).toBe('12-345')
  })
})

describe('URL state', () => {
  it('round-trips a search', () => {
    const p = { ...emptySearch, code: '45380', npi: '1073587416', include_related: false, limit: 50, offset: 50 }
    expect(parseSearch(toQueryString(p))).toEqual(p)
  })
  it('keeps the URL short for defaults', () => {
    expect(toQueryString({ ...emptySearch, code: '99213' })).toBe('code=99213')
    expect(toQueryString(emptySearch)).toBe('')
  })
  it('ignores invalid values', () => {
    const p = parseSearch('code=1&limit=7&offset=-3&billing_class=x')
    expect(p.limit).toBe(25)
    expect(p.offset).toBe(0)
    expect(p.billing_class).toBe('')
  })
})

describe('sortResults', () => {
  // Builds a minimal result row for the sort tests.
  const row = (id: number, rate: number, code: string, cls: string, provider: string) =>
    ({
      id,
      rate,
      billing_code: code,
      name: '',
      billing_class: cls,
      service_codes: [],
      modifiers: [],
      providers: [{ business_name: provider }],
    }) as unknown as RateResult
  const rows = [
    row(1, 300, '490', 'institutional', 'beta'),
    row(2, 100, '45380', 'professional', 'Alpha'),
    row(3, 200, '45380', 'professional', 'gamma'),
  ]
  // Sorts the sample rows and returns their ids in order.
  const ids = (s: SortState) => sortResults(rows, s).map((r) => r.id)

  it('sorts each column both ways', () => {
    expect(ids({ key: 'rate', order: 'asc' })).toEqual([2, 3, 1])
    expect(ids({ key: 'rate', order: 'desc' })).toEqual([1, 3, 2])
    expect(ids({ key: 'code', order: 'asc' })).toEqual([2, 3, 1])
    expect(ids({ key: 'code', order: 'desc' })).toEqual([1, 2, 3]) // ties stay low-to-high by rate
    expect(ids({ key: 'billing', order: 'asc' })).toEqual([1, 2, 3])
    expect(ids({ key: 'provider', order: 'asc' })).toEqual([2, 1, 3]) // case-insensitive
    expect(ids({ key: 'provider', order: 'desc' })).toEqual([3, 1, 2])
  })
  it('does not mutate the input', () => {
    sortResults(rows, { key: 'rate', order: 'desc' })
    expect(rows.map((r) => r.id)).toEqual([1, 2, 3])
  })
  it('toggles direction on the same column only', () => {
    expect(nextSort(defaultSort, 'rate')).toEqual({ key: 'rate', order: 'desc' })
    expect(nextSort({ key: 'rate', order: 'desc' }, 'provider')).toEqual({ key: 'provider', order: 'asc' })
  })
})
