import { useCallback, useEffect, useState } from 'react'
import type { BillingClass, SearchParams } from '../api/types'

export const PAGE_SIZES = [25, 50, 100] as const

export const emptySearch: SearchParams = {
  code: '',
  npi: '',
  ein: '',
  code_type: '',
  billing_class: '',
  include_related: true,
  limit: 25,
  offset: 0,
}

/** Reads search params from a query string, ignoring invalid values. */
export function parseSearch(qs: string): SearchParams {
  const q = new URLSearchParams(qs)
  const limit = Number(q.get('limit'))
  const offset = Number(q.get('offset'))
  const bc = q.get('billing_class')
  return {
    code: q.get('code') ?? '',
    npi: q.get('npi') ?? '',
    ein: q.get('ein') ?? '',
    code_type: (q.get('code_type') ?? '').toUpperCase(),
    billing_class: bc === 'professional' || bc === 'institutional' ? (bc as BillingClass) : '',
    include_related: q.get('include_related') !== 'false',
    limit: (PAGE_SIZES as readonly number[]).includes(limit) ? limit : 25,
    offset: Number.isInteger(offset) && offset > 0 ? offset : 0,
  }
}

/** Serializes a search to a short query string (defaults left out). Used for the URL and the API call. */
export function toQueryString(p: SearchParams): string {
  if (!p.code.trim()) return ''
  const q = new URLSearchParams({ code: p.code.trim() })
  if (p.npi.trim()) q.set('npi', p.npi.trim())
  if (p.ein.trim()) q.set('ein', p.ein.trim())
  if (p.code_type) q.set('code_type', p.code_type)
  if (p.billing_class) q.set('billing_class', p.billing_class)
  if (!p.include_related) q.set('include_related', 'false')
  if (p.limit !== 25) q.set('limit', String(p.limit))
  if (p.offset) q.set('offset', String(p.offset))
  return q.toString()
}

/** Keeps the submitted search in the URL, so searches can be shared and back/forward work. */
export function useUrlSearch(): [SearchParams, (p: SearchParams) => void] {
  const [params, setParams] = useState(() => parseSearch(window.location.search))

  useEffect(() => {
    // Follow back/forward navigation.
    const onPop = () => setParams(parseSearch(window.location.search))
    window.addEventListener('popstate', onPop)
    return () => window.removeEventListener('popstate', onPop)
  }, [])

  // Pushes a history entry for the new search and updates state.
  const update = useCallback((p: SearchParams) => {
    const qs = toQueryString(p)
    const next = qs ? `?${qs}` : ''
    if (next !== window.location.search) window.history.pushState(null, '', next || window.location.pathname)
    setParams(p)
  }, [])

  return [params, update]
}
