import { useQuery } from '@tanstack/react-query'
import { useEffect, useId, useRef, useState } from 'react'
import { useApi } from '../api/ApiContext'
import type { CodeSummary } from '../api/types'
import { formatNumber, titleCase } from '../lib/format'

interface Props {
  value: string
  onChange: (v: string) => void
  onPick: (c: CodeSummary) => void
  error?: string
  enabled: boolean
}

/** Returns value once it has stopped changing for ms milliseconds. */
function useDebounced<T>(value: T, ms: number): T {
  const [v, setV] = useState(value)
  useEffect(() => {
    const t = setTimeout(() => setV(value), ms)
    return () => clearTimeout(t)
  }, [value, ms])
  return v
}

/** Billing code input with keyboard-navigable autocomplete. */
export function CodeInput({ value, onChange, onPick, error, enabled }: Props) {
  const api = useApi()
  const id = useId()
  const listId = `${id}-list`
  const [open, setOpen] = useState(false)
  const [active, setActive] = useState(-1)
  const wrapRef = useRef<HTMLDivElement>(null)
  const q = useDebounced(value.trim(), 200)

  const { data: suggestions = [], isFetching } = useQuery({
    queryKey: ['codes', q],
    queryFn: ({ signal }) => api.codes(q, signal),
    enabled: enabled && q.length > 0,
    staleTime: Infinity,
  })
  const items = q.length > 0 ? suggestions : []
  const show = open && items.length > 0

  useEffect(() => setActive(-1), [q])

  useEffect(() => {
    // Close the list when clicking outside the input.
    const onDoc = (e: MouseEvent) => {
      if (!wrapRef.current?.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', onDoc)
    return () => document.removeEventListener('mousedown', onDoc)
  }, [])

  // Selects a suggestion and closes the list.
  const pick = (c: CodeSummary) => {
    onPick(c)
    setOpen(false)
  }

  // Arrow keys move through suggestions, Enter picks, Escape closes.
  const onKeyDown = (e: React.KeyboardEvent) => {
    if (!show) return
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      setActive((a) => (a + 1) % items.length)
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      setActive((a) => (a <= 0 ? items.length - 1 : a - 1))
    } else if (e.key === 'Enter' && active >= 0) {
      e.preventDefault()
      pick(items[active])
    } else if (e.key === 'Escape') {
      setOpen(false)
    }
  }

  return (
    <div className="field field-code" ref={wrapRef}>
      <label htmlFor={id}>
        Billing code <span className="req">required</span>
      </label>
      <div className="combo">
        <input
          id={id}
          value={value}
          placeholder="e.g. 99213, 45380, 490"
          autoComplete="off"
          spellCheck={false}
          role="combobox"
          aria-expanded={show}
          aria-controls={listId}
          aria-activedescendant={active >= 0 ? `${listId}-${active}` : undefined}
          aria-invalid={!!error}
          aria-describedby={error ? `${id}-err` : undefined}
          onChange={(e) => {
            onChange(e.target.value)
            setOpen(true)
          }}
          onFocus={() => setOpen(true)}
          onKeyDown={onKeyDown}
        />
        {isFetching && q && <span className="combo-spinner" aria-hidden />}
        {show && (
          <ul className="combo-list" id={listId} role="listbox">
            <li role="presentation" className="combo-header">
              {items.length === 1 ? '1 matching code' : `${formatNumber(items.length)} matching codes`}
            </li>
            {items.map((c, i) => (
              <li
                key={`${c.type}-${c.code}`}
                id={`${listId}-${i}`}
                role="option"
                aria-selected={i === active}
                className={i === active ? 'active' : ''}
                onMouseDown={(e) => {
                  e.preventDefault()
                  pick(c)
                }}
                onMouseEnter={() => setActive(i)}
              >
                <span className="combo-code">
                  {c.type ? (
                    <span className={`badge badge-${c.type.toLowerCase()}`}>{c.type}</span>
                  ) : (
                    <span className="badge">ref</span>
                  )}
                  {c.code}
                </span>
                <span className="combo-desc">{describe(c)}</span>
                <span className="combo-count">{countLabel(c)}</span>
              </li>
            ))}
          </ul>
        )}
      </div>
      {error && (
        <p className="field-error" id={`${id}-err`}>
          {error}
        </p>
      )}
    </div>
  )
}

/** One-line label for a suggestion, noting extra names under the same code. */
function describe(c: CodeSummary): string {
  if (!c.type) return c.description
  if (c.variants > 1) return `${titleCase(c.name)} + ${c.variants - 1} other names`
  return titleCase(c.name || c.description)
}

/** Rate counts shown under a suggestion, e.g. "11 rates · 42 facility". */
function countLabel(c: CodeSummary): string {
  const parts = []
  if (c.rows) parts.push(`${formatNumber(c.rows)} rates`)
  if (c.related_rows) parts.push(`${formatNumber(c.related_rows)} facility`)
  return parts.join(' · ')
}
