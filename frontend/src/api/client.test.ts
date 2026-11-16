import { afterEach, describe, expect, it, vi } from 'vitest'
import { emptySearch } from '../lib/urlState'
import { ApiError, createApiClient } from './client'

// Replaces global fetch with one that records the URL and returns the given response.
function stubFetch(status: number, body: unknown) {
  const fn = vi.fn<(url: string) => Promise<Response>>(async () => new Response(JSON.stringify(body), { status }))
  vi.stubGlobal('fetch', fn)
  return fn
}

afterEach(() => vi.unstubAllGlobals())

describe('createApiClient', () => {
  it('prefixes requests with the base URL', async () => {
    const fetchFn = stubFetch(200, { codes: [] })
    await createApiClient('https://api.example/').codes('992')
    expect(fetchFn.mock.calls[0][0]).toBe('https://api.example/api/codes?q=992')
  })
  it('uses same-origin paths by default', async () => {
    const fetchFn = stubFetch(200, { total: 0, results: [] })
    await createApiClient().search({ ...emptySearch, code: '99213' })
    expect(fetchFn.mock.calls[0][0]).toBe('/api/search?code=99213')
  })
  it("turns error responses into ApiError with the server's message and field", async () => {
    stubFetch(400, { error: { code: 'invalid_request', message: 'NPI must be exactly 10 digits.', field: 'npi' } })
    const err = await createApiClient()
      .status()
      .catch((e) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect(err).toMatchObject({ status: 400, field: 'npi', message: 'NPI must be exactly 10 digits.' })
  })
})
