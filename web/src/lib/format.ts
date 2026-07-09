/** Human-readable byte size, matching CasaOS's "7.76 GB" / "386.43 GB" style. */
export function renderSize(bytes: number): string {
  if (!bytes || bytes < 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  let i = 0
  let n = bytes
  while (n >= 1024 && i < units.length - 1) {
    n /= 1024
    i++
  }
  const dec = i >= 2 ? 2 : i === 1 ? 1 : 0
  return `${n.toFixed(dec)} ${units[i]}`
}
