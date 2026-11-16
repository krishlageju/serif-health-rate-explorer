import { useQuery } from '@tanstack/react-query'
import { useApi } from '../api/ApiContext'
import type { Status } from '../api/types'
import { formatNumber, pluralize } from '../lib/format'

/** Explains where the data comes from: index file, chosen files and plans. */
export function SourcesPanel({ status }: { status?: Status }) {
  const api = useApi()
  const ready = status?.state === 'ready' || status?.state === 'partial'
  const { data } = useQuery({
    queryKey: ['sources', ready],
    queryFn: ({ signal }) => api.sources(signal),
    enabled: !!status?.index,
  })
  const idx = status?.index
  if (!idx) return null

  return (
    <details className="sources">
      <summary>
        <span className="sources-label">Data source ▾</span>
        <span className="muted small">
          {idx.reporting_entity_name} · updated {idx.last_updated_on}
          {status?.totals && ` · ${formatNumber(status.totals.rows)} rates`}
        </span>
      </summary>
      <div className="sources-body">
        <p className="small">
          Index file:{' '}
          <a href={idx.url} target="_blank" rel="noreferrer">
            {decodeURIComponent(idx.url.split('/').pop() ?? idx.url)}
          </a>{' '}
          ({idx.reporting_entity_type}, schema v{idx.version}). It lists{' '}
          {pluralize(data?.files.length ?? 0, 'in-network file')}.
        </p>
        {status?.totals && (
          <p className="small muted">
            {formatNumber(status.totals.raw_rows)} price entries read, merged into {formatNumber(status.totals.rows)}{' '}
            distinct rates · {formatNumber(status.totals.tins)} EINs · {formatNumber(status.totals.npis)} NPIs
          </p>
        )}
        <ul className="source-files">
          {data?.files.map((f) => {
            const st = status?.files.find((s) => s.url === f.url)
            return (
              <li key={f.url}>
                <div>
                  <span className={`badge ${f.loaded ? 'badge-ok' : ''}`}>{f.loaded ? 'loaded' : 'not loaded'}</span>{' '}
                  <a href={f.url} target="_blank" rel="noreferrer">
                    {f.file_name}
                  </a>
                </div>
                <span className="muted small">
                  {pluralize(f.plans.length, 'plan')}
                  {st?.last_updated_on && ` · file updated ${st.last_updated_on}`}
                  {st?.stats && ` · ${formatNumber(st.stats.prices)} prices, ${st.stats.skipped} skipped`}
                </span>
                <details className="plans">
                  <summary className="small">Show plans</summary>
                  <ul>
                    {f.plans.map((p, i) => (
                      <li key={i} className="small">
                        {p.plan_name}{' '}
                        <span className="muted">
                          ({p.plan_id_type?.toUpperCase()} {p.plan_id}, {p.plan_market_type})
                        </span>
                      </li>
                    ))}
                  </ul>
                </details>
              </li>
            )
          })}
        </ul>
      </div>
    </details>
  )
}
