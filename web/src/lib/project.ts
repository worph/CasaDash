/** Mirror of the backend's project-name normalization, so the store UI can tell
 *  which catalog apps are already installed (installed id = compose project). */
export function sanitizeProject(id: string): string {
  const n = id
    .toLowerCase()
    .replace(/[^a-z0-9_-]+/g, '-')
    .replace(/^[-_]+|[-_]+$/g, '')
  return n || 'app'
}
