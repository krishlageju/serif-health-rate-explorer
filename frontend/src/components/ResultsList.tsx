import { useState } from 'react'
import type { Provider, RateResult, SortKey, SortOrder } from '../api/types'
import { placeOfService, revenueCodeName } from '../lib/codeLabels'
import { formatNumber, formatUSD, pluralize, titleCase } from '../lib/format'

const COLUMNS: { key: SortKey; label: string }[] = [
  { key: 'rate', label: 'Rate' },
  { key: 'code', label: 'Service' },
  { key: 'billing', label: 'Billing' },
  { key: 'provider', label: 'Provider' },
]

interface Props {
  results: RateResult[]
  code: string
  sort: SortKey
  order: SortOrder
  onSort: (key: SortKey) => void
}

/** Results table with clickable column headers that sort the current page. */
export function ResultsList({ results, code, sort, order, onSort }: Props) {
  return (
    <div className="results" role="table" aria-label={`Negotiated rates for ${code}`}>
      <div className="results-head" role="row">
        {COLUMNS.map((c) => {
          const active = c.key === sort
          return (
            <span
              key={c.key}
              role="columnheader"
              aria-sort={active ? (order === 'asc' ? 'ascending' : 'descending') : 'none'}
            >
              <button
                type="button"
                className={`sort-btn ${active ? 'active' : ''}`}
                onClick={() => onSort(c.key)}
                title={`Sort by ${c.label.toLowerCase()}`}
              >
                {c.label}
                <span className="sort-arrow" aria-hidden>
                  {active ? (order === 'asc' ? '▲' : '▼') : ''}
                </span>
              </button>
            </span>
          )
        })}
        <span role="columnheader" className="sr-only">
          Details
        </span>
      </div>
      {results.map((r) => (
        <ResultRow key={r.id} r={r} code={code} />
      ))}
    </div>
  )
}

/** One rate: a summary row plus an expandable details panel. */
function ResultRow({ r, code }: { r: RateResult; code: string }) {
  const [open, setOpen] = useState(false)
  const [first, ...others] = r.providers
  const detailsId = `details-${r.id}`

  return (
    <div className={`result ${open ? 'open' : ''}`} role="rowgroup">
      <div className="result-main" role="row">
        <div className="cell cell-rate" role="cell">
          <span className="rate">{formatUSD(r.rate)}</span>
          <span className="muted">{r.negotiated_type}</span>
        </div>

        <div className="cell cell-service" role="cell">
          <div className="code-line">
            <span className={`badge badge-${r.billing_code_type.toLowerCase()}`}>{r.billing_code_type}</span>
            <strong>{r.billing_code}</strong>
            {r.match === 'additional_information' && (
              <span
                className="badge badge-via"
                title={`Filed under revenue code ${r.billing_code}; procedure code ${code} is in additional_information`}
              >
                references {r.additional_information}
              </span>
            )}
          </div>
          <ServiceLabel r={r} />
        </div>

        <div className="cell cell-billing" role="cell">
          <span className="cap">{r.billing_class || '—'}</span>
          <span className="chips">
            {r.service_codes.slice(0, 3).map((s) => (
              <span key={s} className="chip" title={placeOfService(s)}>
                POS {s}
              </span>
            ))}
            {r.service_codes.length > 3 && <span className="chip">+{r.service_codes.length - 3}</span>}
            {r.modifiers.map((m) => (
              <span key={m} className="chip chip-mod" title="Billing code modifier">
                mod {m}
              </span>
            ))}
          </span>
        </div>

        <div className="cell cell-provider" role="cell">
          {first ? (
            <>
              <span className="provider-name">
                {first.matched && (
                  <span className="dot" title="Matches your NPI/EIN filter" aria-label="matches filter" />
                )}
                {first.business_name || 'Unnamed provider'}
              </span>
              <span className="muted small">
                {first.tin_type.toUpperCase()} {first.tin} · {pluralize(first.npi_count, 'NPI')}
                {others.length > 0 && ` · +${others.length} more ${others.length === 1 ? 'group' : 'groups'}`}
              </span>
            </>
          ) : (
            <span className="muted">No provider listed</span>
          )}
        </div>

        <div className="cell cell-toggle" role="cell">
          <button
            type="button"
            className="btn btn-ghost btn-sm"
            aria-expanded={open}
            aria-controls={detailsId}
            onClick={() => setOpen((o) => !o)}
          >
            {open ? 'Hide' : 'Details'}
          </button>
        </div>
      </div>

      {open && (
        <div className="result-details" id={detailsId}>
          <dl className="kv">
            <dt>Description</dt>
            <dd>{r.description || '—'}</dd>
            <dt>Code version</dt>
            <dd>
              {r.billing_code_type} {r.billing_code_type_version || ''}
            </dd>
            <dt>Arrangement</dt>
            <dd>{r.negotiation_arrangement.toUpperCase() || '—'}</dd>
            <dt>Setting</dt>
            <dd className="cap">{r.setting || '—'}</dd>
            <dt>Place of service</dt>
            <dd>{r.service_codes.map((s) => `${s} (${placeOfService(s)})`).join(', ') || '—'}</dd>
            {r.additional_information && (
              <>
                <dt>Additional info</dt>
                <dd>{r.additional_information}</dd>
              </>
            )}
            <dt>Expires</dt>
            <dd>{r.expiration_date === '9999-12-31' ? 'No expiration (9999-12-31)' : r.expiration_date}</dd>
            {r.listings > 1 && (
              <>
                <dt>Source listings</dt>
                <dd>
                  This exact price (same provider, code, rate and place of service) appears {formatNumber(r.listings)}{' '}
                  times in the file under different item names, so it is shown once here. Names:{' '}
                  {[r.name, ...r.other_names].map(titleCase).join(', ')}
                  {r.other_name_count > r.other_names.length &&
                    `, and ${r.other_name_count - r.other_names.length} more`}
                </dd>
              </>
            )}
            {r.source && (
              <>
                <dt>Source file</dt>
                <dd>
                  <a href={r.source.url} target="_blank" rel="noreferrer">
                    {r.source.file_name}
                  </a>
                  <span className="muted">
                    {' '}
                    · updated {r.source.last_updated_on} · applies to {pluralize(r.source.plan_count, 'plan')}
                  </span>
                </dd>
              </>
            )}
          </dl>

          <h4>Provider groups ({r.providers.length})</h4>
          <ul className="providers">
            {r.providers.map((p) => (
              <ProviderItem key={`${p.provider_group_id ?? 'inline'}-${p.tin}`} p={p} />
            ))}
          </ul>
        </div>
      )}
    </div>
  )
}

/** Labels a rate's service. Revenue-code rows filed under many unrelated item names
 * show the revenue code and the procedure it references instead. */
function ServiceLabel({ r }: { r: RateResult }) {
  const isRC = r.billing_code_type === 'RC'
  const rcName = isRC ? revenueCodeName(r.billing_code) : undefined
  const procedure = r.additional_information
  const unreliableName = isRC && r.other_name_count > 0

  return (
    <>
      <span className="service-name">
        {unreliableName ? (rcName ?? `Revenue code ${r.billing_code}`) : titleCase(r.name || r.description)}
        {!unreliableName && rcName && <span className="muted"> · {rcName}</span>}
      </span>
      {isRC && procedure && (
        <span className="small">
          for procedure <strong>{procedure}</strong>
          {r.additional_information_name && ` · ${titleCase(r.additional_information_name)}`}
        </span>
      )}
      {r.other_name_count > 0 && (
        <span className="muted small">
          published under {pluralize(r.other_name_count + 1, 'item name')} (see details)
        </span>
      )}
    </>
  )
}

/** One provider group in the details panel, with its EIN and NPIs. */
function ProviderItem({ p }: { p: Provider }) {
  const hidden = p.npi_count - p.npis.length
  return (
    <li className={p.matched ? 'matched' : ''}>
      <div className="provider-head">
        <strong>{p.business_name || 'Unnamed provider'}</strong>
        {p.matched && <span className="badge badge-match">matches filter</span>}
      </div>
      <span className="muted small">
        {p.tin_type.toUpperCase()} {p.tin}
        {p.provider_group_id ? ` · provider_group_id ${p.provider_group_id}` : ' · inline group'}
        {p.networks.length > 0 && ` · ${p.networks.join(', ')}`}
      </span>
      <div className="npis">
        {p.npis.map((n) => (
          <code key={n}>{n}</code>
        ))}
        {hidden > 0 && <span className="muted small">+{formatNumber(hidden)} more NPIs</span>}
      </div>
    </li>
  )
}
