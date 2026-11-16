import { useEffect, useState } from 'react'
import type { BillingClass, CodeSummary, SearchParams } from '../api/types'
import { formatEINInput } from '../lib/format'
import { emptySearch } from '../lib/urlState'
import { npiWarning, validateSearch, type FieldErrors } from '../lib/validate'
import { CodeInput } from './CodeInput'

interface Props {
  initial: SearchParams
  ready: boolean
  searching: boolean
  serverField?: { field: string; message: string }
  onSearch: (p: SearchParams) => void
}

/** Search form: code (required), NPI and EIN, plus optional extra filters. */
export function SearchForm({ initial, ready, searching, serverField, onSearch }: Props) {
  const [form, setForm] = useState(initial)
  const [errors, setErrors] = useState<FieldErrors>({})
  const [showMore, setShowMore] = useState(
    () => !!(initial.code_type || initial.billing_class || !initial.include_related),
  )

  // Follow the URL when the user navigates back or forward.
  useEffect(() => {
    setForm(initial)
    setErrors({})
  }, [initial])

  const set = <K extends keyof SearchParams>(k: K, v: SearchParams[K]) => {
    setForm((f) => ({ ...f, [k]: v }))
    if (k in errors) setErrors((e) => ({ ...e, [k]: undefined }))
  }

  // Validates locally, then submits a normalized search starting at page 1.
  const submit = (e: React.FormEvent) => {
    e.preventDefault()
    const errs = validateSearch(form.code, form.npi, form.ein)
    setErrors(errs)
    if (Object.values(errs).some(Boolean)) return
    onSearch({
      ...form,
      code: form.code.trim().toUpperCase(),
      npi: form.npi.trim(),
      ein: form.ein.trim(),
      offset: 0,
    })
  }

  // Fills the code from autocomplete; reference-only codes need facility rates turned on.
  const pickCode = (c: CodeSummary) => {
    set('code', c.code)
    // Picking a reference-only code means the rates live under revenue codes.
    if (!c.type && !form.include_related) set('include_related', true)
  }

  // Resets the form and clears the search from the URL.
  const clear = () => {
    setForm(emptySearch)
    setErrors({})
    onSearch(emptySearch)
  }

  // Local validation error first, then any error the server returned for that field.
  const fieldErr = (f: keyof FieldErrors) => errors[f] ?? (serverField?.field === f ? serverField.message : undefined)
  const warn = !errors.npi ? npiWarning(form.npi) : undefined
  const hasInput = !!(form.code || form.npi || form.ein)

  return (
    <form className="search card" onSubmit={submit} noValidate aria-label="Search negotiated rates">
      <div className="search-row">
        <CodeInput
          value={form.code}
          onChange={(v) => set('code', v)}
          onPick={pickCode}
          error={fieldErr('code')}
          enabled={ready}
        />
        <div className="field">
          <label htmlFor="npi">
            NPI <span className="opt">optional</span>
          </label>
          <input
            id="npi"
            inputMode="numeric"
            placeholder="10 digits"
            maxLength={10}
            value={form.npi}
            aria-invalid={!!fieldErr('npi')}
            onChange={(e) => set('npi', e.target.value.replace(/\D/g, ''))}
          />
          {fieldErr('npi') ? (
            <p className="field-error">{fieldErr('npi')}</p>
          ) : (
            warn && <p className="field-warn">{warn}</p>
          )}
        </div>
        <div className="field">
          <label htmlFor="ein">
            EIN <span className="opt">optional</span>
          </label>
          <input
            id="ein"
            inputMode="numeric"
            placeholder="12-3456789"
            maxLength={10}
            value={form.ein}
            aria-invalid={!!fieldErr('ein')}
            onChange={(e) => set('ein', formatEINInput(e.target.value))}
          />
          {fieldErr('ein') && <p className="field-error">{fieldErr('ein')}</p>}
        </div>
        <div className="search-actions">
          <button type="submit" className="btn btn-primary" disabled={!ready || searching}>
            {searching ? 'Searching…' : 'Search'}
          </button>
          {hasInput && (
            <button type="button" className="btn btn-ghost" onClick={clear}>
              Clear
            </button>
          )}
        </div>
      </div>

      <button type="button" className="link-btn" aria-expanded={showMore} onClick={() => setShowMore((s) => !s)}>
        {showMore ? '− Fewer filters' : '+ More filters'}
      </button>

      {showMore && (
        <div className="search-more">
          <div className="field">
            <label htmlFor="code_type">Code type</label>
            <select id="code_type" value={form.code_type} onChange={(e) => set('code_type', e.target.value)}>
              <option value="">Any</option>
              <option value="CPT">CPT</option>
              <option value="HCPCS">HCPCS</option>
              <option value="RC">Revenue code (RC)</option>
            </select>
          </div>
          <div className="field">
            <label htmlFor="billing_class">Billing class</label>
            <select
              id="billing_class"
              value={form.billing_class}
              onChange={(e) => set('billing_class', e.target.value as BillingClass)}
            >
              <option value="">Any</option>
              <option value="professional">Professional</option>
              <option value="institutional">Institutional (facility)</option>
            </select>
          </div>
          <label className="check">
            <input
              type="checkbox"
              checked={form.include_related}
              onChange={(e) => set('include_related', e.target.checked)}
            />
            <span>
              Include facility rates that reference this code
              <small>
                Many hospital and surgery-center rates are filed under a revenue code (e.g. 490) with the procedure code
                in <code>additional_information</code>.
              </small>
            </span>
          </label>
        </div>
      )}
    </form>
  )
}
