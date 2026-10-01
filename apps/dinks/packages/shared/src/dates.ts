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
