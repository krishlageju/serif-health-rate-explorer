import type { Status } from '../api/types'
import { formatBytes, formatNumber } from '../lib/format'

/** Shows ingestion progress, partial failures and load errors. */
export function StatusBanner({ status, error }: { status?: Status; error?: Error | null }) {
  if (error && !status) {
    return (
      <div className="banner banner-error" role="alert">
        <strong>Can't reach the server.</strong> {error.message}
      </div>
    )
  }
  if (!status) return null

  if (status.state === 'error') {
    return (
      <div className="banner banner-error" role="alert">
        <strong>Rate data failed to load.</strong> {status.message}
        <span className="banner-sub">Check the server logs, then restart the backend to retry.</span>
      </div>
    )
  }

  if (status.state === 'partial') {
    const failed = status.files.filter((f) => f.state === 'error')
    return (
      <div className="banner banner-warn" role="status">
        <strong>Some files failed to load; results may be incomplete.</strong>
        {failed.map((f) => (
          <span key={f.url} className="banner-sub">
            {f.file_name}: {f.error}
          </span>
        ))}
      </div>
    )
  }

  if (status.state === 'ready') return null

  const active = status.files.filter((f) => f.selected)
  return (
    <div className="banner banner-info" role="status" aria-live="polite">
      <span className="spinner" aria-hidden />
      <div>
        <strong>{status.message || 'Loading rate data…'}</strong>
        {active.map((f) => {
          const pct = f.bytes_total > 0 ? Math.min(100, Math.round((f.bytes_done / f.bytes_total) * 100)) : null
          return (
            <div key={f.url} className="progress-row">
              <span className="progress-label">
                {stateLabel(f.state)} {f.file_name}
                {f.bytes_total > 0 && ` (${formatBytes(f.bytes_done)} of ${formatBytes(f.bytes_total)})`}
                {f.state === 'ready' && ` · ${formatNumber(f.rows)} prices`}
              </span>
              {pct !== null && f.state !== 'ready' && (
                <span className="progress" aria-hidden>
                  <span style={{ width: `${pct}%` }} />
                </span>
              )}
            </div>
          )
        })}
        <span className="banner-sub">You can fill in the form now; search turns on when loading finishes.</span>
      </div>
    </div>
  )
}

/** Short label for a file's load state, used in the progress lines. */
function stateLabel(s: string) {
  switch (s) {
    case 'queued':
      return 'Waiting:'
    case 'downloading':
      return 'Downloading'
    case 'parsing':
      return 'Parsing'
    case 'ready':
      return 'Loaded'
    case 'error':
      return 'Failed:'
    default:
      return ''
  }
}
