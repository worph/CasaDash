import { writable } from 'svelte/store'

// Whether the App Store modal is open. StorePanel (M3) renders when true.
export const storeOpen = writable(false)

// Optional deep-link into a store app detail (store id + app id).
export const storeApp = writable<{ store: string; app: string } | null>(null)

// The app whose Settings modal is open (null = closed).
export const settingsApp = writable<{ id: string; name: string; managed: boolean } | null>(null)

// The app pending uninstall confirmation (null = no dialog).
export const uninstallTarget = writable<{ id: string; name: string } | null>(null)

// True while a tile is being dragged (and for one tick after the drop). A mouse
// drop still fires a trailing `click` on the tile, which would otherwise open the
// app; tiles consult this flag in their click handler so a reorder never opens.
export const tileDragging = writable(false)
