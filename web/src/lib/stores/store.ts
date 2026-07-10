import { api } from '../api/client'

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

/** Kick off a detached install. Resolves once the server has *started* it (not
 *  when it finishes) with the app's compose project id; progress then arrives on
 *  the live "apps" channel as the tile's download/start bars. */
export function installApp(id: string): Promise<{ status: string; id: string }> {
  return api.post<{ status: string; id: string }>(`/api/store/${encodeURIComponent(id)}/install`)
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
export function refreshStoreSource(url: string): Promise<{ sources: string[] }> {
  return api.post<{ sources: string[] }>('/api/store/sources/refresh', { url })
}

