# CasaDash — Implementation Note

Companion to [`README.md`](./README.md) (the spec) and
[`FEATURE-COMPARISON.md`](./FEATURE-COMPARISON.md) (scope). This note fixes the **stack**
and the **implementation plan**. Reference source of truth for behaviour:
`/d/workspace/yundera/yundera-root/packages/casa-img` (`casa-img`).

> **As-built amendments** (the code is implemented; a few decisions were settled during
> the build):
> - **Compose lifecycle = the `docker compose` plugin, invoked out-of-process**
>   (`internal/composecmd`), *not* the compose-v2 Go library. The Docker SDK's `+incompatible`
>   module split made linking the engine costly; shelling out to the plugin is reliable,
>   keeps the Go build light, and behaves identically to Docker's own tooling. App **listing
>   and start/stop/restart** use the Docker SDK directly (`internal/dockerx`); **install and
>   config-recreate** use the plugin.
> - **Runtime image = alpine + `docker-cli-compose`** (not distroless), because the install
>   path needs the compose plugin on `PATH`. Still one process, no engine, no supervisor.
> - **x-casaos parsing** is a YAML round-trip (`internal/xcasaos`) rather than compose-go's
>   `loader.Transform`, so the read path doesn't depend on the compose engine.

---

## 1. Stack decision

| Layer | Choice | Why |
|-------|--------|-----|
| **Backend** | **Go**, single static binary | Lowest always-on footprint (~15–30 MB RSS idle, ~0 idle CPU). Native Docker SDK + the real Compose parser → highest store-compat confidence. One process. |
| **Frontend** | **Svelte + Vite** (fresh build) | Compiles away — no virtual-DOM runtime → smallest bundle & lowest browser CPU/mem. Not forking CasaOS-UI (we own parity). Node is **build-time only**. |
| **Styling** | Tailwind + copied CasaOS design tokens | Reproduce the frosted-glass CasaOS look without inheriting the legacy Vue 2 bundle. |
| **Live data** | Plain WebSocket (SSE fallback) | No socket.io / message-bus. Only samples while a client is subscribed → keeps idle CPU near zero. |
| **Packaging** | Distroless (or alpine) + embedded UI | No in-image Docker engine, no s6. Target **one process, sub-~60 MB image, tens of MB RAM**. |

**Explicitly rejected:** forking CasaOS-UI (Vue 2.7 EOL, heavy bundle), Node backend
(larger idle footprint + store-compat reimplementation risk), bundling the Docker engine
or a process supervisor.

**Accepted cost:** we re-implement the dashboard *and* the App Store browse/detail UI
pixel-perfect ourselves. That is the price of a lean, modern, non-EOL frontend.

---

## 2. Repository layout

```
CasaDash/
├── cmd/casadash/main.go          # entrypoint: wire config, docker client, router, embed UI
├── internal/
│   ├── server/                   # HTTP router (chi/echo), middleware, static+SPA serving
│   ├── docker/                   # docker/docker/client wrapper, events subscription
│   ├── compose/                  # parse (compose-go), override merge, up/down, project paths
│   ├── store/                    # multi-store fetch+cache, listing parse, x-casaos extract
│   ├── apps/                     # registry: managed apps + unmanaged (x-casaos) discovery
│   ├── xcasaos/                  # x-casaos metadata types + parsing (PORT from casa-img)
│   ├── envinject/                # env/template injection (PORT from casa-img)
│   ├── system/                   # gopsutil stats (CPU/RAM/temp/disk)
│   ├── live/                     # WS/SSE hub: status, logs, stats channels
│   └── config/                   # settings persistence (wallpaper/theme/lang/stores)
├── web/                          # Svelte app
│   ├── src/
│   │   ├── routes/               # dashboard, store, store/[app], settings, app/[id]
│   │   ├── lib/
│   │   │   ├── components/       # widgets/ appgrid/ store/ common/
│   │   │   ├── api/              # typed REST client
│   │   │   ├── live/             # WS client (status/logs/stats)
│   │   │   ├── stores/           # svelte stores (apps, system, settings)
│   │   │   └── i18n/             # locale files + runtime
│   │   └── styles/               # tailwind + tokens.css
│   ├── vite.config.ts
│   └── package.json
├── build/                        # vite output, embedded via go:embed
├── Dockerfile                    # multi-stage: node → go → distroless
├── docker-compose.yml            # deployment (from README §7)
└── go.mod
```

Static assets are compiled by Vite into `build/` and embedded into the Go binary with
`//go:embed`, so the runtime artifact is a single file that serves both UI and API.

---

## 3. Backend design

### 3.1 HTTP / WS surface (draft)

```
GET    /ping                          health → 200
GET    /api/system/stats              one-shot CPU/RAM/temp/disk
GET    /api/apps                      list: managed + unmanaged, with status
POST   /api/apps/{id}/start|stop|restart
DELETE /api/apps/{id}                 uninstall (down + remove project)
POST   /api/apps/{id}/update          re-pull store listing + recreate
GET    /api/apps/{id}/config          effective compose (base + override)
PUT    /api/apps/{id}/config          write docker-compose.override.yml, recreate
GET    /api/store                     merged catalog (multi-store), categories
GET    /api/store/{store}/{app}       app detail (x-casaos + screenshots)
POST   /api/store/{store}/{app}/install
GET    /api/settings   PUT /api/settings     wallpaper/theme/language/stores
POST   /api/links                     add external-link tile
WS     /ws                            multiplexed: system tick, app-status,
                                      per-app log stream, per-app stat stream
```

### 3.2 Docker & Compose lifecycle

- **Client:** `github.com/docker/docker/client` over the mounted socket. Subscribe to
  Docker **events** to drive live status without polling.
- **Parse / validate / merge:** `github.com/compose-spec/compose-go` — the same parser
  Docker Compose v2 uses, so store apps resolve identically. The per-app **override**
  window is just Compose's native multi-file merge (`docker-compose.yml` +
  `docker-compose.override.yml`).
- **up / down — primary approach:** embed **Docker Compose v2 as a Go library**
  (`github.com/docker/compose/v2/pkg/compose`) so lifecycle runs **in-process** — no
  external binary, single static image, reference-grade behaviour.
  **Fallback if the library integration proves heavy:** ship the `docker compose` v2
  plugin in the image and shell out. (Decision flagged in §9; primary keeps the image
  smallest.)
- **On-disk layout — CasaOS-compatible** (so listings and paths behave like CasaOS):
  ```
  ${DATA_ROOT}/AppData/casaos/apps/<app>/docker-compose.yml          # store copy, AS-IS
  ${DATA_ROOT}/AppData/casaos/apps/<app>/docker-compose.override.yml # user config edits
  ${DATA_ROOT}/AppData/<app>/...                                     # app volumes
  ```
- Store compose is written **as-is** (never rewritten); user edits live only in the
  override file. Apps attach to the shared `${REF_NET}` bridge and get a
  `{name}.${REF_DOMAIN}` hostname — same as casa-img.

### 3.3 Env / template injection (port from casa-img)

Reproduce casa-img's substitution so existing listings resolve. Port from the Yundera
`CasaOS-AppManagement` fork:

| Concern | casa-img source to port |
|---------|-------------------------|
| Inject env into a compose app | `service/compose_app.go` → `injectEnvVariableToComposeApp` |
| Base interpolation map (PUID/PGID/TZ/AppID…) | `service/compose_service.go` → `baseInterpolationMap` |
| Global env file (`/etc/casaos/env`) | `pkg/config/init.go` |
| PCS routing template (`REF_*`, `${domain}/${scheme}/${port}/${name}`) | `route/v2/appstore_pcs.go` |

Rules doc: `casa-img/docs/environment-variable-injection.md`.

### 3.4 Store

- Config: one or more store zip URLs (default
  `https://github.com/Yundera/AppStore/archive/refs/heads/main.zip`).
- Fetch → cache under `${DATA_ROOT}/AppData/casadash/appstore/<store>/`, parse listings +
  `x-casaos`, merge catalogs, expose categories/featured/search **within the store UI**
  (there is no global dashboard search — spec §2).
- **Auto-update:** periodic refresh of the cached catalogs; when an installed app's listing
  changes, offer/apply an update (re-inject env, recreate).

### 3.5 Unmanaged-app discovery

Watch Docker for Compose projects **not** created by CasaDash whose config carries
`x-casaos` (via container labels / project working-dir compose). Surface them as tiles
badged **unmanaged** — read `x-casaos` for icon/name/link, show status, but treat their
source of truth as external (limited lifecycle; depth flagged in §9).

### 3.6 System stats & live data

- `gopsutil` for CPU %, load/temp, RAM, disk used/total.
- The `live` hub samples system stats and streams per-app logs/stats **only while a client
  is subscribed** on `/ws`; app-status changes are pushed from the Docker events stream.
  This is the key idle-footprint lever — no subscribers ⇒ no sampling loop.

### 3.7 State

Plain files under `${DATA_ROOT}/AppData/casadash/` (settings, tile order, external links,
store cache). No database.

---

## 4. Frontend design (Svelte + Vite)

- **Routes:** `/` dashboard · `/store` + `/store/[app]` · `/settings` · `/app/[id]`.
- **Components:**
  - `widgets/` — Clock, SystemStatus (CPU/RAM radial gauges), Storage, WidgetSettings.
    Radial gauges done with lightweight SVG (no charting lib) to keep the bundle small.
  - `appgrid/` — AppTile (hover Open + burger menu: Open/Settings/Restart/Stop/Start/
    Uninstall), drag-to-reorder, add-menu (Add external link), unmanaged badge.
  - `store/` — category browse, featured, in-store search, app detail (screenshots,
    description, install).
  - `common/` — modal, config-override editor (form + raw compose view), toasts.
- **State:** Svelte stores hydrated from REST, kept live from the `/ws` client.
- **i18n:** locale JSON + a tiny runtime (e.g. `svelte-i18n` / `intlayer`), language chosen
  in settings.
- **Design:** Tailwind with tokens copied from CasaOS (colors, radii, blur, spacing) over a
  configurable full-bleed wallpaper; light/dark theme toggle.
- **Build:** `vite build` → `build/`; embedded into the Go binary. No SSR, no Node at
  runtime.

---

## 5. Packaging

Multi-stage build; runtime carries only the Go binary (UI embedded) + CA certs.

```dockerfile
# 1) UI
FROM node:lts-slim AS ui
WORKDIR /ui
COPY web/package*.json ./ && RUN npm ci
COPY web/ .
RUN npm run build            # -> /ui/build

# 2) Backend (UI embedded via go:embed)
FROM golang:1.22-alpine AS backend
WORKDIR /src
COPY go.mod go.sum ./ && RUN go mod download
COPY . .
COPY --from=ui /ui/build ./build
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /casadash ./cmd/casadash

# 3) Runtime
FROM gcr.io/distroless/static-debian12
COPY --from=backend /casadash /casadash
EXPOSE 8080
ENTRYPOINT ["/casadash"]
```

- **Socket access without a shell:** distroless has no entrypoint script, so do **not**
  replicate casa-img's `casa-init.sh` GID discovery in-image. Instead grant the container
  the Docker socket's group at deploy time — `group_add` in compose (or run with a
  matching `PGID`). The Go process can also `stat` the socket at startup and fail fast with
  a clear message if it lacks access.
- **Footprint targets:** image < ~60 MB, one process, tens of MB RAM idle, ~0 idle CPU.
  Contrast casa-img: Ubuntu + Docker engine + 7 Go binaries + samba/mergerfs, multi-process,
  hundreds of MB.

### Deployment env (see README §7 for the full compose)

`DATA_ROOT`, `PUID`/`PGID`, `TZ`, `REF_NET`/`REF_PORT`/`REF_SCHEME`/`REF_DOMAIN`,
`APPSTORE_URL` (comma-separated for multi-store). Mounts: `/var/run/docker.sock` and
`${DATA_ROOT}` (`rshared`). No `/dev`, no samba/mergerfs mounts.

---

## 6. Reuse map — what to port from casa-img

| Bring over (port to Go modules) | Rebuild fresh |
|---------------------------------|---------------|
| `x-casaos` schema + parsing | Entire frontend (Svelte, no CasaOS-UI) |
| Env/template injection (§3.3) | HTTP/WS server (small, our own) |
| PCS routing templating (`REF_*`) | System-stats widget backend (gopsutil) |
| Store zip fetch/parse conventions | Live-data hub (plain WS, not socket.io) |
| On-disk app layout + install flow semantics | Packaging (distroless, no s6/engine) |

---

## 7. Build order (milestones)

1. **Skeleton** — Go server, `/ping`, embed a Svelte shell, connect Docker client, read config.
2. **System widgets** — gopsutil stats → `/ws` tick → Clock/SystemStatus/Storage widgets.
3. **App grid** — list managed compose projects + unmanaged `x-casaos` discovery; tiles,
   open, burger-menu lifecycle (start/stop/restart/uninstall), drag-reorder, external links.
4. **Store** — multi-store fetch/parse, browse/detail UI, install (env inject → write
   project → up), auto-update.
5. **Per-app config** — override-file editor window; live logs + container stats streaming.
6. **Settings & polish** — wallpaper/theme/language, i18n coverage, empty/error states.
7. **Packaging** — multi-stage Dockerfile, sample compose, footprint validation vs targets.

---

## 8. Non-goals (reaffirmed — do not build)

Auth/login, file manager, multi-user/RBAC, global search bar, promo/onboarding cards,
manual "install customized app" form, disk/RAID/Samba/remote-storage tooling, in-image
Docker engine, process supervisor.

## 9. Deferred / to validate during build

- **Compose lifecycle mechanism:** confirm the compose-v2-as-a-library approach is
  workable; else fall back to shelling out to the bundled compose plugin (bigger image).
- **Live transport:** WebSocket vs SSE for the log/stat streams (default WS).
- **Unmanaged-app depth:** how much lifecycle control to offer over externally-created
  stacks vs. read-only display.
- **i18n runtime choice** and initial locale set.
- **Distroless vs alpine:** alpine if we end up needing a shell / the compose plugin.
