const usd = new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD' })
const num = new Intl.NumberFormat('en-US')

/** Formats a number as US dollars, e.g. $1,234.50. */
export const formatUSD = (n: number) => usd.format(n)
/** Formats a number with thousands separators. */
export const formatNumber = (n: number) => num.format(n)

/** Formats a byte count in MB. */
export function formatBytes(n: number): string {
  if (n <= 0) return '0 MB'
  return `${(n / 1_000_000).toFixed(n < 10_000_000 ? 1 : 0)} MB`
}

/** Formats 9 digits as NN-NNNNNNN while the user types. */
export function formatEINInput(raw: string): string {
  const d = raw.replace(/\D/g, '').slice(0, 9)
  return d.length > 2 ? `${d.slice(0, 2)}-${d.slice(2)}` : d
}

/** "1 plan" or "3 plans", with thousands separators. */
export function pluralize(n: number, word: string, plural = `${word}s`) {
  return `${formatNumber(n)} ${n === 1 ? word : plural}`
}

/** Converts ALL-CAPS names from the files to Title Case. */
export function titleCase(s: string): string {
  return s.toLowerCase().replace(/\b([a-z])/g, (m) => m.toUpperCase())
}
