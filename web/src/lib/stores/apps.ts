import { get, writable } from 'svelte/store'
import { live } from '../live/ws'
import { api } from '../api/client'

export interface App {
  id: string
  name: string
  icon: string
  status: 'running' | 'stopped' | 'partial'
  managed: boolean
  store?: string
  scheme?: string
  hostname?: string
  port?: string
  index?: string
  category?: string
  /** Fully-resolved click URL from x-compose-app (webui-*); wins when set. */
  url?: string
  /** Aggregated Docker health-check verdict; drives the tile status dot.
   *  Absent when no container declares a health check. */
  health?: 'healthy' | 'unhealthy' | 'starting'
  /** True while a lifecycle op (start/stop/restart/uninstall) is in flight:
   *  the tile shows a "…" overlay and hides its burger menu. */
  busy?: boolean
  /** True while a store install is in flight for this app: the tile shows two
   *  progress bars (download + start) instead of being clickable. */
  installing?: boolean
  /** Image-pull progress, 0-100 (only meaningful while installing). */
  download?: number
  /** Stack-start progress, 0-100, driven by Docker (only while installing). */
  start?: number
  /** Current install phase: pull | prepare | start | done | error. */
  phase?: string
  /** Set when an install failed; the tile shows the error until retried. */
  install_error?: string
}

// Last-known app list, cached in localStorage so a page reload paints the grid
// instantly — before the backend (or Docker) has answered. The server is now the
// source of truth for *what exists* (folder-driven, docs/app-model.md), so the
// cache only bridges the first-paint gap; a fresh snapshot supersedes it in ms.
const CACHE_KEY = 'casadash.apps'

/** Read the cached tiles, or [] when absent/corrupt. */
function cachedApps(): App[] {
  try {
    const v = JSON.parse(localStorage.getItem(CACHE_KEY) ?? '[]')
    return Array.isArray(v) ? (v as App[]) : []
  } catch {
    return []
  }
}

/** Persist tiles for the next reload, minus volatile in-flight overlays — so a
 *  reload mid-install/-op never resurrects a stuck "…" or a frozen progress bar.
 *  Identity, name, icon and last-known status/health are kept so the tile looks
 *  right immediately (greyed vs coloured) until the live state refreshes. */
function persistApps(list: App[]): void {
  try {
    const clean = list.map((a) => {
      const c: App = { ...a }
      delete c.busy
      delete c.installing
      delete c.download
      delete c.start
      delete c.phase
      delete c.install_error
      return c
    })
    localStorage.setItem(CACHE_KEY, JSON.stringify(clean))
  } catch {
    /* private mode / quota — cache is best-effort */
  }
}

/** Apply an authoritative snapshot: replace the grid and refresh the cache. */
function applySnapshot(list: App[]): void {
  apps.set(list)
  persistApps(list)
}

export const apps = writable<App[]>(cachedApps())

/** One-shot REST load (used on mount for an immediate render). The REST endpoint
 *  is authoritative — including a genuine empty list — so its result always wins;
 *  on failure we keep whatever is on screen (cache or a prior load). */
export async function loadApps(): Promise<void> {
  try {
    applySnapshot((await api.get<App[]>('/api/apps')) ?? [])
  } catch {
    /* leave previous value (cached tiles stay visible) */
  }
}

/** Live updates over the WebSocket "apps" channel. Returns unsubscribe.
 *
 *  Non-destructive: an empty live frame never blanks a populated grid. Post
 *  step-1 the server only emits [] when there genuinely are no apps or it is
 *  still warming up, and those cases are already covered by loadApps()'s
 *  authoritative REST replace (mount + after every mutation, incl. uninstall).
 *  Ignoring empty frames here trades a rare stale ghost tile — self-healing on
 *  the next reload/mutation — for never flashing an empty dashboard. */
export function subscribeApps(): () => void {
  return live.subscribe('apps', (d) => {
    const list = (d as App[]) ?? []
    if (list.length === 0 && get(apps).length > 0) return
    applySnapshot(list)
  })
}

export async function appAction(id: string, action: 'start' | 'stop' | 'restart'): Promise<void> {
  await api.post(`/api/apps/${encodeURIComponent(id)}/${action}`)
  await loadApps()
}

/** Uninstall an app. Its folder is always preserved — renamed to
 *  `<app>.<date>.archive` (never deleted). When zip is true it is compressed to
 *  a `.zip` archive instead of a plain rename. Returns the archive's name. */
export async function uninstallApp(id: string, zip = false): Promise<string> {
  const res = await api.del<{ status: string; archive?: string }>(
    `/api/apps/${encodeURIComponent(id)}?zip=${zip}`,
  )
  await loadApps()
  return res?.archive ?? ''
}

/** Effective opening-URL (x-compose-app webui-*) config for an app, plus the
 *  server-resolved preview URL. Editing it writes into the app's override. */
export interface WebUI {
  scheme: string
  host: string
  port: string
  path: string
  url: string
}

export interface AppConfig {
  base: string
  override: string
  webui: WebUI
  /** Store-provided guidance (x-casaos tips), read-only. */
  tips: string
  /** The user's own editable note, persisted per-app. */
  note: string
}

export function getConfig(id: string): Promise<AppConfig> {
  return api.get<AppConfig>(`/api/apps/${encodeURIComponent(id)}/config`)
}

/** Save (or clear, when blank) the user's per-app note. */
export async function setNote(id: string, note: string): Promise<void> {
  await api.put(`/api/apps/${encodeURIComponent(id)}/note`, { note })
}

export async function setConfig(id: string, override: string): Promise<void> {
  await api.put(`/api/apps/${encodeURIComponent(id)}/config`, { override })
  await loadApps()
}

/** Save the opening-URL fields into the app's override (webui-* shortcut). */
export async function setWebUI(id: string, w: Omit<WebUI, 'url'>): Promise<void> {
  await api.put(`/api/apps/${encodeURIComponent(id)}/webui`, w)
  await loadApps()
}

/** Whether an app's reference store carries a newer docker-compose.yml than the
 *  installed copy. Resolved from the store reference recorded in the override. */
export interface UpdateStatus {
  /** The app records where it was installed from (needed to check at all). */
  has_ref: boolean
  /** The store's compose differs from the installed one. */
  available: boolean
  /** Reference store URL and the catalog id within it. */
  store: string
  store_app_id: string
  /** Non-fatal lookup failure (store unreachable, app removed from catalog, …). */
  error?: string
}

/** Check whether the app's reference store has a newer compose than installed. */
export function checkUpdate(id: string): Promise<UpdateStatus> {
  return api.get<UpdateStatus>(`/api/apps/${encodeURIComponent(id)}/update`)
}

/** Pull the store's current compose (when it differs) and bring the stack back
 *  up. Returns true when an update was actually applied. */
export async function applyUpdate(id: string): Promise<boolean> {
  const res = await api.post<{ status: string; updated: boolean }>(
    `/api/apps/${encodeURIComponent(id)}/update`,
  )
  await loadApps()
  return res?.updated ?? false
}

/** One container of a multi-service stack, with live state and health. */
export interface AppService {
  service: string
  container_id: string
  state: string
  health: string // '', starting, healthy, unhealthy
}

export function getServices(id: string): Promise<AppService[]> {
  return api.get<AppService[]>(`/api/apps/${encodeURIComponent(id)}/services`)
}

/** Compute an app's web URL, if it has a reachable one. */
export function appUrl(a: App): string {
  // x-compose-app (webui-*) resolves the full URL server-side; use it verbatim.
  if (a.url) return a.url
  const scheme = a.scheme || 'http'
  const index = a.index && a.index !== '/' ? a.index : ''
  if (a.hostname) return `${scheme}://${a.hostname}${index}`
  if (a.port) return `${scheme}://${location.hostname}:${a.port}${index}`
  return ''
}

/** Open an app's web UI, or warn if it has no reachable URL. */
export function openApp(a: App): void {
  const url = appUrl(a)
  if (url) {
    window.open(url, '_blank', 'noopener')
  } else {
    alert(
      `${a.name} has no directly reachable web address.\n\n` +
        `It exposes its UI to a reverse-proxy/gateway rather than a host port. ` +
        `Add a published port via the app's Settings → override, or put it behind a gateway.`,
    )
  }
}
