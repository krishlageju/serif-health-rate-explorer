import type { Summary } from '../api/types'
import { formatNumber, formatUSD, pluralize } from '../lib/format'

/** Stat tiles for the whole result set, not just the current page. */
export function SummaryBar({ summary, code }: { summary: Summary; code: string }) {
  const tiles = [
    { label: 'Distinct rates', value: formatNumber(summary.count) },
    { label: 'Providers (EINs)', value: formatNumber(summary.providers) },
    { label: 'Lowest', value: formatUSD(summary.min) },
    { label: 'Median', value: formatUSD(summary.median) },
    { label: 'Highest', value: formatUSD(summary.max) },
  ]
  return (
    <section className="summary" aria-label={`Summary for ${code}`}>
      <div className="tiles">
        {tiles.map((t) => (
          <div className="tile" key={t.label}>
            <span className="tile-label">{t.label}</span>
            <span className="tile-value">{t.value}</span>
          </div>
        ))}
      </div>
      {summary.related_rows > 0 && (
        <p className="summary-note">
          {formatNumber(summary.direct_rows)} billed directly as {code} ·{' '}
          {pluralize(summary.related_rows, 'facility rate')} filed under a revenue code that references {code}
        </p>
      )}
    </section>
  )
}
