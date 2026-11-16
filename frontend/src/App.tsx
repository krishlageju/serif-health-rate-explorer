import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { useEffect, useMemo, useState } from 'react'
import { useApi } from './api/ApiContext'
import type { ApiError } from './api/client'
import type { SearchParams, SearchResponse } from './api/types'
import { NoResults, SearchError, Welcome } from './components/EmptyStates'
import { Pagination } from './components/Pagination'
import { ResultsList } from './components/ResultsList'
import { ResultsToolbar } from './components/ResultsToolbar'
import { SearchForm } from './components/SearchForm'
import { SourcesPanel } from './components/SourcesPanel'
import { StatusBanner } from './components/StatusBanner'
import { SummaryBar } from './components/SummaryBar'
import { defaultSort, nextSort, sortResults, type SortState } from './lib/sortResults'
import { emptySearch, useUrlSearch } from './lib/urlState'

export const APP_NAME = 'Serif Rate Explorer'

/** Main page: status banner, search form, then results or an empty state. */
export default function App() {
  const api = useApi()
  const [params, setParams] = useUrlSearch()

  const statusQ = useQuery({
    queryKey: ['status'],
    queryFn: ({ signal }) => api.status(signal),
    refetchInterval: (q) => {
      const s = q.state.data?.state
      return s === 'ready' || s === 'partial' || s === 'error' ? false : 1000
    },
    retry: true,
    retryDelay: 2000,
  })
  const status = statusQ.data
  const ready = status?.state === 'ready' || status?.state === 'partial'

  const searchQ = useQuery<SearchResponse, ApiError>({
    queryKey: ['search', params],
    queryFn: ({ signal }) => api.search(params, signal),
    enabled: ready && !!params.code,
    placeholderData: keepPreviousData,
    staleTime: Infinity,
    retry: (n, err) => err.status === 503 && n < 3,
  })

  useEffect(() => {
    document.title = params.code ? `${params.code} · ${APP_NAME}` : APP_NAME
  }, [params.code])

  // Column sorting happens in the browser on the rows of the current page;
  // it never triggers another API call.
  const [sort, setSort] = useState<SortState>(defaultSort)
  const pageResults = params.code ? searchQ.data?.results : undefined
  const sortedResults = useMemo(() => (pageResults ? sortResults(pageResults, sort) : []), [pageResults, sort])

  // Merges changes into the current search and updates the URL.
  const update = (p: Partial<SearchParams>) => setParams({ ...params, ...p })
  const err = searchQ.error
  const serverField = err?.status === 400 && err.field ? { field: err.field, message: err.message } : undefined
  const data = params.code ? searchQ.data : undefined

  return (
    <div className="app">
      <header className="header">
        <div className="header-inner">
          <div className="brand">
            <span className="logo" aria-hidden>
              S
            </span>
            <div>
              <h1>{APP_NAME}</h1>
              <p className="muted small">Search negotiated in-network rates · Serif Health take-home</p>
            </div>
          </div>
        </div>
      </header>

      <main className="main">
        <StatusBanner status={status} error={statusQ.error} />

        <SearchForm
          initial={params}
          ready={ready}
          searching={searchQ.isFetching}
          serverField={serverField}
          onSearch={setParams}
        />

        {!params.code && <Welcome ready={ready} onExample={(p) => setParams({ ...emptySearch, ...p })} />}

        {params.code && err && !serverField && <SearchError message={err.message} onRetry={() => searchQ.refetch()} />}

        {data && data.total === 0 && data.hint && <NoResults hint={data.hint} params={params} onChange={update} />}

        {data && data.total > 0 && data.summary && (
          <section
            className={`results-wrap ${searchQ.isPlaceholderData ? 'stale' : ''}`}
            aria-busy={searchQ.isFetching}
          >
            <SummaryBar summary={data.summary} code={params.code} />
            <ResultsToolbar params={params} sort={sort} onSort={setSort} />
            <ResultsList
              results={sortedResults}
              code={params.code}
              sort={sort.key}
              order={sort.order}
              onSort={(key) => setSort((s) => nextSort(s, key))}
            />
            <Pagination
              total={data.total}
              limit={params.limit}
              offset={params.offset}
              onChange={(offset, limit) => {
                update({ offset, limit })
                window.scrollTo({ top: 0, behavior: 'smooth' })
              }}
            />
          </section>
        )}
      </main>

      <footer className="footer small">
        <SourcesPanel status={status} />
        <p className="muted footer-note">
          Data: CMS Transparency in Coverage machine-readable files. Rates are as published by the payer and may be
          incomplete or out of date.
        </p>
      </footer>
    </div>
  )
}
