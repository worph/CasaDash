import { api } from '../api/client'
import { openStream } from '../live/stream'

export interface StoreApp {
  id: string
  name: string
  tagline: string
  description: string
  icon: string
  thumbnail: string
  screenshots: string[]
  category: string
  developer: string
  author: string
  min_memory?: number
  store: string
}

export interface StoreData {
  apps: StoreApp[]
  categories: string[]
  recommend: string[]
}

export function fetchStore(): Promise<StoreData> {
  return api.get<StoreData>('/api/store')
}

export function fetchStoreApp(id: string): Promise<StoreApp> {
  return api.get<StoreApp>(`/api/store/app/${encodeURIComponent(id)}`)
}

export function installApp(id: string): Promise<void> {
  return api.post(`/api/store/${encodeURIComponent(id)}/install`)
}

export function fetchStoreSources(): Promise<{ sources: string[] }> {
  return api.get<{ sources: string[] }>('/api/store/sources')
}
export function addStoreSource(url: string): Promise<{ sources: string[] }> {
  return api.post<{ sources: string[] }>('/api/store/sources', { url })
}
export function removeStoreSource(url: string): Promise<{ sources: string[] }> {
  return api.del<{ sources: string[] }>('/api/store/sources', { url })
}

export interface InstallEvent {
  phase: 'pull' | 'prepare' | 'start' | 'done' | 'error'
  message: string
  percent: number
}

/** Install `id` while streaming progress events over a WebSocket. Returns a
 *  function that closes the stream. */
export function installAppStream(
  id: string,
  onEvent: (e: InstallEvent) => void,
  onClose?: () => void,
): () => void {
  return openStream(
    `/api/store/${encodeURIComponent(id)}/install/ws`,
    (raw) => {
      try {
        onEvent(JSON.parse(raw) as InstallEvent)
      } catch {
        /* ignore malformed frame */
      }
    },
    onClose,
  )
}
