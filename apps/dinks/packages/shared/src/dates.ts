/**
 * Local-calendar date helpers. Shared by web and mobile so a day means the same
 * thing in both: `toISO` reads the Date's local getters rather than toISOString,
 * because a member logs the day they woke up on, not the UTC one.
 */
export function todayISO() {
  return toISO(new Date())
}

export function toISO(date: Date) {
  const y = date.getFullYear()
  const m = String(date.getMonth() + 1).padStart(2, '0')
  const d = String(date.getDate()).padStart(2, '0')
  return `${y}-${m}-${d}`
}

export function toDate(iso: string) {
  const [y, m, d] = iso.split('-').map(Number)
  return new Date(y, m - 1, d)
}

export const MONTHS_SHORT = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']

/** "Mar 30" — the compact label both shells use for a predicted day. */
export function formatShortDate(iso: string) {
  const date = toDate(iso)
  return `${MONTHS_SHORT[date.getMonth()]} ${date.getDate()}`
}

/** "Mar 30 – Apr 3", collapsing to a single date when start === end. */
export function formatDateRange(start: string, end: string) {
  return start === end ? formatShortDate(start) : `${formatShortDate(start)} – ${formatShortDate(end)}`
}
