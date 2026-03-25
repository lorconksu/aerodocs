/**
 * Formats a UTC timestamp string as a human-readable relative time (e.g. "5 min ago").
 * Returns '—' when the input is null or empty.
 */
export function relativeTime(dateStr: string | null): string {
  if (!dateStr) return '—'
  // Server sends UTC timestamps without 'Z' suffix — ensure correct parsing
  const normalized = dateStr.endsWith('Z') ? dateStr : dateStr.replace(' ', 'T') + 'Z'
  const date = new Date(normalized)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffSec = Math.floor(diffMs / 1000)
  if (diffSec < 60) return `${diffSec}s ago`
  const diffMin = Math.floor(diffSec / 60)
  if (diffMin < 60) return `${diffMin} min ago`
  const diffHr = Math.floor(diffMin / 60)
  if (diffHr < 24) return `${diffHr}h ago`
  const diffDay = Math.floor(diffHr / 24)
  return `${diffDay}d ago`
}
