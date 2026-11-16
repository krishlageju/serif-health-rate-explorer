// Types mirror the Go API responses in backend/internal/api.

export type LoadState = 'starting' | 'loading' | 'ready' | 'partial' | 'error'
export type FileState = 'not_selected' | 'queued' | 'downloading' | 'parsing' | 'ready' | 'error'

/** Load progress and outcome for one in-network file. */
export interface FileStatus {
  url: string
  file_name: string
  plan_count: number
  selected: boolean
  state: FileState
  from_cache: boolean
  bytes_done: number
  bytes_total: number
  rows: number
  error?: string
  last_updated_on?: string
  duration_ms?: number
  stats?: { items: number; prices: number; skipped: number; dangling_refs: number; remote_refs: number }
}

/** The index (table-of-contents) file that was read. */
export interface IndexInfo {
  url: string
  reporting_entity_name: string
  reporting_entity_type: string
  last_updated_on: string
  version: string
  reporting_structures: number
}

/** Counts for everything loaded. */
export interface Totals {
  raw_rows: number
  rows: number
  providers: number
  npis: number
  tins: number
  codes: number
}

/** Response of GET /api/status. */
export interface Status {
  state: LoadState
  message?: string
  index?: IndexInfo
  files: FileStatus[]
  totals?: Totals
  started_at: string
  ready_at?: string
}

/** A reporting plan listed in the index. */
export interface Plan {
  plan_name: string
  plan_id_type: string
  plan_id: string
  plan_market_type: string
  issuer_name?: string
}

/** An in-network file from the index, and whether it was loaded. */
export interface SourceFile {
  url: string
  file_name: string
  description: string
  plans: Plan[]
  loaded: boolean
  source_id?: number
}

/** Response of GET /api/sources. */
export interface SourcesResponse {
  index?: IndexInfo
  files: SourceFile[]
}

/** One autocomplete suggestion. */
export interface CodeSummary {
  code: string
  type: string
  name: string
  description: string
  rows: number
  variants: number
  related_rows: number
}

export type SortKey = 'rate' | 'code' | 'billing' | 'provider'
export type SortOrder = 'asc' | 'desc'
export type BillingClass = '' | 'professional' | 'institutional'

/** The search as entered in the form and stored in the URL. */
export interface SearchParams {
  code: string
  npi: string
  ein: string
  code_type: string
  billing_class: BillingClass
  include_related: boolean
  limit: number
  offset: number
}

/** A provider group (one TIN and its NPIs) on a rate. */
export interface Provider {
  provider_group_id?: number
  tin_type: string
  tin: string
  business_name: string
  npi_count: number
  npis: number[]
  networks: string[]
  matched: boolean
}

/** One negotiated rate in the results. */
export interface RateResult {
  id: number
  match: 'billing_code' | 'additional_information'
  billing_code: string
  billing_code_type: string
  billing_code_type_version: string
  name: string
  description: string
  negotiation_arrangement: string
  negotiated_type: string
  rate: number
  expiration_date: string
  billing_class: string
  setting: string
  service_codes: string[]
  modifiers: string[]
  additional_information?: string
  additional_information_name?: string
  listings: number
  other_names: string[]
  other_name_count: number
  providers: Provider[]
  source: { id: number; file_name: string; url: string; plan_count: number; last_updated_on: string } | null
}

/** Stats over every match, not just the current page. */
export interface Summary {
  count: number
  min: number
  max: number
  median: number
  mean: number
  providers: number
  direct_rows: number
  related_rows: number
}

/** Why a search returned nothing. */
export interface Hint {
  code: string
  message: string
}

/** Response of GET /api/search. */
export interface SearchResponse {
  total: number
  limit: number
  offset: number
  summary: Summary | null
  results: RateResult[]
  hint?: Hint
}

/** Body of every API error response. */
export interface ApiErrorBody {
  error: { code: string; message: string; field?: string }
}
