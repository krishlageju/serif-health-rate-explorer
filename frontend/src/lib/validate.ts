/** Validation message per form field; a missing key means the field is valid. */
export interface FieldErrors {
  code?: string
  npi?: string
  ein?: string
}

/** Luhn check used by NPIs, with the 80840 prefix CMS specifies. */
export function isValidNPIChecksum(npi: string): boolean {
  if (!/^\d{10}$/.test(npi)) return false
  const digits = `80840${npi}`
  let sum = 0
  for (let i = 0; i < digits.length; i++) {
    let d = Number(digits[digits.length - 1 - i])
    if (i % 2 === 1) {
      d *= 2
      if (d > 9) d -= 9
    }
    sum += d
  }
  return sum % 10 === 0
}

/** Checks the form fields and returns an error message for each invalid one. */
export function validateSearch(code: string, npi: string, ein: string): FieldErrors {
  const errors: FieldErrors = {}
  const c = code.trim()
  if (!c) errors.code = 'Enter a billing code, e.g. 99213.'
  else if (!/^[A-Za-z0-9][A-Za-z0-9.-]{0,14}$/.test(c)) errors.code = 'Use letters, digits, "." or "-" only.'

  const n = npi.trim()
  if (n && !/^\d{10}$/.test(n)) errors.npi = 'NPI must be exactly 10 digits.'

  const e = ein.replace(/[\s-]/g, '')
  if (ein.trim() && !/^\d{9}$/.test(e)) errors.ein = 'EIN must be 9 digits, e.g. 12-3456789.'
  return errors
}

/** A soft warning, not a blocker: some files contain NPIs that fail the checksum. */
export function npiWarning(npi: string): string | undefined {
  const n = npi.trim()
  if (/^\d{10}$/.test(n) && !isValidNPIChecksum(n)) {
    return "This NPI's check digit looks wrong. Searching anyway."
  }
  return undefined
}
