// Minimal path router for the two URLs CasaDash exposes:
//
//   /                        the dashboard
//   /store                   the store, browsing the catalog
//   /store/<app>             the store, on that app's detail page
//   /store/<app>?store=<url> ...pinned to one store, which may be a store the
//                            user has not added (AppDetail warns before install)
//
// The server serves index.html for any unknown path (internal/server/spa.go), so
// these deep links load the SPA directly. The legacy `#store` / `#settings:<id>`
// hash links still work — App.svelte reads them once at mount.
//
// State lives in the `storeOpen` / `storeApp` ui stores; this module is the only
// place that reads or writes the URL.
import { storeOpen, storeApp } from './stores/ui'

export interface Route {
  store: boolean
  app: string // '' = catalog
  storeURL: string // '' = merged catalog
}

const DASHBOARD: Route = { store: false, app: '', storeURL: '' }

export function parse(url: URL): Route {
  const seg = url.pathname.split('/').filter(Boolean)
  if (seg[0] !== 'store') return DASHBOARD
  return {
    store: true,
    app: seg[1] ? decodeURIComponent(seg[1]) : '',
    storeURL: seg[1] ? (url.searchParams.get('store') ?? '') : '',
  }
}

export function href(r: Route): string {
  if (!r.store) return '/'
  if (!r.app) return '/store'
  const q = r.storeURL ? `?store=${encodeURIComponent(r.storeURL)}` : ''
  return `/store/${encodeURIComponent(r.app)}${q}`
}

/** Apply a route to the ui stores (does not touch the URL). */
function apply(r: Route) {
  storeOpen.set(r.store)
  storeApp.set(r.store && r.app ? { store: r.storeURL, app: r.app } : null)
}

// Each history entry we create carries how deep into *our* stack it is. depth 0 is
// the entry the SPA loaded on; the one below it belongs to wherever the user came
// from, so history.back() from depth 0 would leave CasaDash altogether. Keeping the
// depth in the entry (rather than a counter) keeps it right under Back *and* Forward.
type Entry = Route & { depth: number }

const depth = (): number => (history.state as Entry | null)?.depth ?? 0

/** Navigate: push the route onto the history stack and apply it. */
export function go(r: Route) {
  if (href(r) !== location.pathname + location.search) {
    history.pushState({ ...r, depth: depth() + 1 } satisfies Entry, '', href(r))
  }
  apply(r)
}

export const openStore = () => go({ store: true, app: '', storeURL: '' })
export const openStoreApp = (app: string, storeURL = '') => go({ store: true, app, storeURL })
export const closeStore = () => go(DASHBOARD)

/** Back out of an app detail to the catalog. Steps back through history when the
 *  detail page is one we pushed, so the arrow and the browser's Back button agree
 *  — but a deep link opened in a fresh tab has nothing of ours to step back to, so
 *  there it pushes the catalog instead of navigating off the site. */
export function backToCatalog() {
  if (depth() > 0) history.back()
  else openStore()
}

/** Start routing: apply the current URL and follow Back/Forward. Returns an
 *  unsubscribe for onMount. */
export function start(): () => void {
  const onPop = (e: PopStateEvent) => apply((e.state as Entry | null) ?? parse(new URL(location.href)))
  const initial = parse(new URL(location.href))
  history.replaceState({ ...initial, depth: 0 } satisfies Entry, '', href(initial))
  apply(initial)
  addEventListener('popstate', onPop)
  return () => removeEventListener('popstate', onPop)
}
