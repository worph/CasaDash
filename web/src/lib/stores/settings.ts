import { writable, derived } from 'svelte/store'
import { api } from '../api/client'
import { locale } from '../i18n'

// Operator preferences, persisted server-side via /api/settings so they follow
// the server rather than a single browser.
export interface Settings {
  wallpaper: string
  language: string
  widgets: Record<string, boolean>
}

const DEFAULTS: Settings = {
  wallpaper: '/wallpapers/default_wallpaper.jpg',
  language: 'en_us',
  widgets: { clock: true, system: true, storage: true },
}

export const settings = writable<Settings>({ ...DEFAULTS })

let loaded = false
let saveTimer: number | undefined

/** Load persisted settings from the server (call once on startup). */
export async function loadSettings(): Promise<void> {
  try {
    const s = await api.get<Settings>('/api/settings')
    settings.set({ ...DEFAULTS, ...s })
  } catch {
    /* keep defaults */
  } finally {
    loaded = true
  }
}

// Keep the active locale in sync, and persist changes (debounced) after load.
settings.subscribe((s) => {
  locale.set(s.language)
  if (!loaded) return
  clearTimeout(saveTimer)
  saveTimer = window.setTimeout(() => {
    api.put('/api/settings', s).catch(() => {})
  }, 400)
})

export const wallpaper = derived(settings, ($s) => $s.wallpaper)
