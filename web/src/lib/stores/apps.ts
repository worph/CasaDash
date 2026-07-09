import { writable } from 'svelte/store'
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
}

export const apps = writable<App[]>([])

/** One-shot REST load (used on mount for an immediate render). */
export async function loadApps(): Promise<void> {
  try {
    apps.set((await api.get<App[]>('/api/apps')) ?? [])
  } catch {
    /* leave previous value */
  }
}

/** Live updates over the WebSocket "apps" channel. Returns unsubscribe. */
export function subscribeApps(): () => void {
  return live.subscribe('apps', (d) => apps.set((d as App[]) ?? []))
}

export async function appAction(id: string, action: 'start' | 'stop' | 'restart'): Promise<void> {
  await api.post(`/api/apps/${encodeURIComponent(id)}/${action}`)
  await loadApps()
}

/** Uninstall an app. When archive is true, its data folder is zipped before
 *  removal. Returns the archive filename (empty if none was created). */
export async function uninstallApp(id: string, archive = false): Promise<string> {
  const res = await api.del<{ status: string; archive?: string }>(
    `/api/apps/${encodeURIComponent(id)}?archive=${archive}`,
  )
  await loadApps()
  return res?.archive ?? ''
}

export interface AppConfig {
  base: string
  override: string
}

export function getConfig(id: string): Promise<AppConfig> {
  return api.get<AppConfig>(`/api/apps/${encodeURIComponent(id)}/config`)
}

export async function setConfig(id: string, override: string): Promise<void> {
  await api.put(`/api/apps/${encodeURIComponent(id)}/config`, { override })
  await loadApps()
}

/** Compute an app's web URL, if it has a reachable one. */
export function appUrl(a: App): string {
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
