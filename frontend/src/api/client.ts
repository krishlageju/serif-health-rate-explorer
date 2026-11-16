import { toQueryString } from '../lib/urlState'
import type { ApiErrorBody, CodeSummary, SearchParams, SearchResponse, SourcesResponse, Status } from './types'

/** Error from an API call, carrying the HTTP status, error code and the field it refers to. */
export class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
    readonly code: string,
    readonly field?: string,
  ) {
    super(message)
  }
}

/** Fetches JSON, turning network and HTTP failures into an ApiError with the server's message. */
async function getJSON<T>(url: string, signal?: AbortSignal): Promise<T> {
  let res: Response
  try {
    res = await fetch(url, { signal, headers: { Accept: 'application/json' } })
  } catch (err) {
    if ((err as Error).name === 'AbortError') throw err
    throw new ApiError('Could not reach the server. Is the backend running?', 0, 'network')
  }
  const body = await res.json().catch(() => null)
  if (!res.ok) {
    const e = (body as Partial<ApiErrorBody> | null)?.error
    throw new ApiError(e?.message ?? `Request failed (${res.status})`, res.status, e?.code ?? 'http_error', e?.field)
  }
  return body as T
}

/** The API operations the UI uses. Components depend on this shape, not on fetch or a URL,
 * so tests can pass a fake and a different backend (or service split) only changes the factory. */
export interface ApiClient {
  /** Ingestion state and per-file load progress. */
  status(signal?: AbortSignal): Promise<Status>
  /** Index metadata and every in-network file with its plans. */
  sources(signal?: AbortSignal): Promise<SourcesResponse>
  /** One page of rates for a billing code, with optional filters. */
  search(p: SearchParams, signal?: AbortSignal): Promise<SearchResponse>
  /** Autocomplete suggestions for a code or name fragment. */
  codes(q: string, signal?: AbortSignal): Promise<CodeSummary[]>
}

/** Creates an HTTP ApiClient for the API at baseUrl ('' means same origin, e.g. via the Vite proxy). */
export function createApiClient(baseUrl = ''): ApiClient {
  const base = baseUrl.replace(/\/+$/, '')
  return {
    status: (signal) => getJSON<Status>(`${base}/api/status`, signal),
    sources: (signal) => getJSON<SourcesResponse>(`${base}/api/sources`, signal),
    search: (p, signal) => getJSON<SearchResponse>(`${base}/api/search?${toQueryString(p)}`, signal),
    codes: (q, signal) =>
      getJSON<{ codes: CodeSummary[] }>(`${base}/api/codes?${new URLSearchParams({ q })}`, signal).then((r) => r.codes),
  }
}
