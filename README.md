# CasaDash

**CasaDash** is a lightweight self-hosted dashboard for managing Docker apps on a
single host. It is a *dashboard-only* reimagining of [CasaOS](https://casaos.io):
same look, same feel, same app-store — but stripped down to the one thing most
people actually use it for: **a home screen for your apps + a store to install more.**

CasaDash is **not** CasaOS and is not derived from its codebase at runtime. It only
*mirrors* CasaOS's UX and is **100% compatible with the CasaOS App Store format**, so
the exact same store used by [`casa-img`](https://github.com/Yundera/casa-img) works
here unchanged.

> **Reference implementation:** the full CasaOS bundle we are slimming down lives at
> `D:\workspace\yundera\yundera-root\packages\casa-img`
> (container path `/d/workspace/yundera/yundera-root/packages/casa-img`).
> When in doubt about a behaviour, that is the source of truth.

## Status

Implemented (Go backend + Svelte 5 frontend, single container). Working: system widgets,
app grid with unmanaged-app discovery + lifecycle, the CasaOS-compatible App Store
(browse/search/install), per-app config override, live logs/stats, settings + i18n. See
[`IMPLEMENTATION.md`](./IMPLEMENTATION.md) for the architecture and
[`FEATURE-COMPARISON.md`](./FEATURE-COMPARISON.md) for scope.

```bash
# Build & run the container (drives the host Docker engine over the mounted socket):
docker compose up -d --build      # dashboard on http://localhost:8080

# Local development:
go run ./cmd/casadash              # backend on :8080 (serves embedded UI)
npm --prefix web run dev           # Vite dev server with API/WS proxy to :8080
```

### Data folder & the host path (important)

Apps installed by CasaDash are brought up on the **host** Docker daemon (via the mounted
socket), so their bind-mount **sources are resolved on the host, not inside this
container**. Two env vars keep that straight:

| Env | Meaning | Default |
|-----|---------|---------|
| `DATA_ROOT` | Where the data folder is mounted **inside** the container (the bind target). CasaDash reads/writes its own files here. | `/DATA` |
| `DATA_HOST_PATH` | The **host** path of that same data folder. Written into every installed app's compose so the host daemon binds the right directory. | = `DATA_ROOT` |

Set `DATA_HOST_PATH` to wherever the data folder actually lives on the host, and bind that
same host path to `/DATA`:

```yaml
environment:
  DATA_ROOT: "/DATA"
  DATA_HOST_PATH: "/opt/casadash/DATA"   # <- the host directory
volumes:
  - type: bind
    source: /opt/casadash/DATA           # <- MUST equal DATA_HOST_PATH
    target: /DATA
```

CasaDash then (a) writes each app's project under `${DATA_ROOT}/AppData/<app>` (flat —
one folder per app, holding its compose + override + `.env` + data; see
[`docs/app-model.md`](./docs/app-model.md)), (b)
rewrites app volume sources (`/DATA/...`, `${DATA_ROOT}`) to the host path, and (c)
pre-creates each app's bind directories (owned `PUID:PGID`). Use a **host bind**, not a
named volume — bind mounts can't point inside another container's named volume. See
`.env.example` and `docker-compose.yml`.

**Parity corrections** discovered while matching CasaOS-UI exactly (these supersede a few
earlier notes below): theming is **wallpaper-only** (CasaOS has no light/dark toggle); the
App Store is a **modal/panel**, not a route; settings live in a **top-bar dropdown**
(wallpaper / language / widget toggles); only the **App Store** system tile is prepended
(no Files).

---

## 1. Goals

- **Dashboard-first.** The app grid + system widgets are the product. Everything else
  is trimmed.
- **Pixel-for-pixel CasaOS UX.** Layout, widgets, interactions, and visual design match
  the CasaOS dashboard exactly (see [§4](#4-ux-specification)). A CasaOS user should feel
  no difference on the home screen.
- **CasaOS App Store, unchanged.** Consume the identical app-store zip that `casa-img`
  uses. No new store format, no forked catalog. **Multi-store** supported. Install /
  uninstall / **auto-update** multi-service Docker Compose apps that follow the CasaOS
  `x-casaos` compose convention.
- **Same app layout as CasaOS.** Apps use the same on-disk file structure; store compose
  files are copied and brought up **as-is** — CasaDash never rewrites them.
- **Unmanaged-app discovery.** Any Compose stack on the host that carries `x-casaos`
  metadata — even one CasaDash didn't install — surfaces as an app tile (marked
  unmanaged). This replaces CasaOS's manual "install a customized app" form.
- **Single lightweight container.** Ship as one image, deployed via Docker Compose with
  the host Docker socket bind-mounted — the same deployment model as `casa-img`, but far
  smaller.
- **Multi-language (i18n).** Localized UI.
- **Zero-config.** Boots to a usable dashboard with no setup wizard and no login.

## 2. Non-goals (explicit exclusions)

| Excluded | Why |
|----------|-----|
| **File manager / Files app** | CasaDash is dashboard-only. No `/DATA` browser, no upload/download UI. |
| **Authentication / users / login** | No login screen, no sessions, no "magic link". CasaDash assumes it sits behind a trusted network boundary (VPN, reverse proxy, LAN). The operator is responsible for access control. |
| **Multi-user / RBAC** | Single implicit operator. No accounts. |
| **Global search bar** | The dashboard has no search box. |
| **Promo / onboarding cards** | No getting-started cards on the home screen. |
| **Manual "Install a customized app" form** | Replaced by unmanaged-app auto-discovery ([§4.3](#43-main-column)). |
| **Terminal / SSH / hardware tools** | No disk/RAID management, network shares, or remote-storage tooling. |
| **Being CasaOS** | Not a fork you can drop CasaOS plugins into. Compatibility is *store format only*. |

## 3. Positioning vs CasaOS / casa-img

`casa-img` is a full CasaOS bundle: a dashboard **plus** a file manager, user accounts,
disk/storage management, network shares, and remote-storage tooling.

CasaDash keeps only the dashboard + app-management surface. Feature-level, that means:

| Feature area | casa-img | CasaDash |
|--------------|----------|----------|
| App-grid dashboard (same UX) | ✅ | ✅ |
| App store: browse / install / uninstall / **auto-update** | ✅ | ✅ |
| Multi-store | ✅ | ✅ |
| Multi-service Compose apps, CasaOS on-disk layout | ✅ | ✅ |
| System widgets (CPU / RAM / storage stats) | ✅ | ✅ |
| Per-app logs, container stats, live status | ✅ | ✅ |
| i18n / multi-language | ✅ | ✅ |
| Authentication / users | ✅ | ❌ |
| File manager / Files | ✅ | ❌ |
| Global search bar | ✅ | ❌ |
| Promo / onboarding cards | ✅ | ❌ |
| Manual "install customized app" form | ✅ | ❌ → unmanaged-app discovery |
| Per-app config edits | ✅ (rewrites compose) | 🔶 override-compose file instead |
| Disk/RAID mgmt, Samba shares, remote storage | ✅ | ❌ |

> See [`FEATURE-COMPARISON.md`](./FEATURE-COMPARISON.md) for the full row-by-row table.

**Net effect:** a much smaller, dashboard-only container. Nothing here fixes an
implementation (language, runtime, process model) — that is deliberately left open at
spec stage.

---

## 4. UX specification

The dashboard is a **1:1 copy of the CasaOS home screen**. Reference:
<https://demo.nsl.sh/#/> (the casa-img demo — minus the login page and Files tile, which
CasaDash does not have).

### 4.1 Layout

```
┌──────────────────────────────────────────────────────────────────────────┐
│  [top bar: brand · social links · account/settings/wallpaper icons]       │  ← top toolbar
├───────────────────────┬──────────────────────────────────────────────────┤
│  ┌─────────────────┐  │                                                    │
│  │  12:28          │  │   App                                        [ + ]│  ← app grid header
│  │  Thu, 9 Jul     │  │  ┌──────┐  ┌──────┐  ┌──────┐  ┌──────┐           │
│  └─────────────────┘  │  │ App  │  │ App  │  │ App  │  │ App  │  ...       │  ← installed apps
│  ┌─────────────────┐  │  │Store │  │  A   │  │  B   │  │Settings          │
│  │  System status  │  │  └──────┘  └──────┘  └──────┘  └──────┘           │
│  │   12% CPU  0°C  │  │  ┌──────┐  ┌──────┐                                │
│  │    9% RAM       │  │  │ App  │  │ App* │   * = unmanaged (auto-detected)│
│  └─────────────────┘  │  │  C   │  │  D   │                                │
│  ┌─────────────────┐  │  └──────┘  └──────┘                                │
│  │  Storage        │  │                                                    │
│  │  Healthy        │  │   (no global search bar, no promo cards)           │
│  │  12.95/386 GB   │  │                                                    │
│  │  ▓░░░░░░░░       │  │                                                    │
│  └─────────────────┘  │                                                    │
│  [ Widget settings › ]│                                                    │
└───────────────────────┴──────────────────────────────────────────────────┘
```

### 4.2 Left column — widgets

- **Clock/date widget** — large local time + full date (`Thursday, 9 July 2026`).
- **System status widget** — two radial gauges: **CPU %** (with temperature below, `0°C`
  when unavailable) and **RAM %** (with absolute value, `7.76 GB`). Header links (`›`) to
  a fuller system view. Live-updating.
- **Storage widget** — disk health badge (`Healthy`), `Used: X / Total: Y`, usage bar.
- **Widget settings** (`›`) — toggle which widgets show.

### 4.3 Main column

The main column is **just the app grid** — no global search bar and no promo/onboarding
cards (both dropped vs CasaOS).

- **App grid** — the heart of CasaDash:
  - Each **app tile** = icon + name. Hover reveals an **Open** button and a **burger (⋯)
    menu** with the same options as CasaOS: **Open, Settings, Restart, Stop, Start,
    Uninstall** ("Settings" opens the per-app config window — see [§5.1](#51-per-app-configuration)).
  - Tiles are **drag-to-reorder**.
  - **Unmanaged apps** — any Compose stack on the host carrying `x-casaos` metadata that
    CasaDash didn't install appears automatically as a tile, badged **unmanaged** (not
    tied to a store entry). CasaDash reads its `x-casaos` for icon/name/link.
  - **`+` (add) menu** in the grid header:
    - **Add external link** → pin a bookmark tile to any URL.
    - *(No "install customized app" form — that role is covered by unmanaged-app
      discovery above.)*
  - The **App Store** tile lives in the grid and opens the store (see §5).

### 4.4 Top toolbar

Brand mark, social/help links, and a **settings** dropdown. No login/logout (no auth).

### 4.5 Settings (top-bar dropdown)

- Appearance: **configurable background / wallpaper** (theming is wallpaper-only, matching
  CasaOS — no light/dark toggle).
- **Language (i18n)** selection.
- Widget visibility toggles.
- App-store source URL(s) — **multiple stores** (via `APPSTORE_URL`).

CasaDash inherits CasaOS's visual language: frosted-glass cards over a full-bleed
wallpaper, rounded tiles, the same iconography and spacing.

---

## 5. App Store (CasaOS-compliant)

CasaDash's store is **100% compliant with the CasaOS App Store** and consumes the **same
store `casa-img` uses**.

- **Store source:** a GitHub zip of Compose listings, identical to
  `casa-img`'s configuration:
  ```
  https://github.com/Yundera/AppStore/archive/refs/heads/main.zip
  ```
  **Multiple stores** may be configured (as CasaOS supports); each catalog is downloaded
  and cached locally (equivalent of `/var/lib/casaos/appstore`).
- **App format:** standard `docker-compose.yml` + the CasaOS **`x-casaos`** extension
  block (title, icon, tagline, category, screenshots, main port/scheme/path, per-service
  metadata). CasaDash reads and honours these fields unchanged. Apps are full
  **multi-service Compose stacks**, same as CasaOS.
- **Store UI:** CasaOS-style — category browse, featured/most-popular, in-store search,
  and an app **detail page** (screenshots, description, developer, install button).
- **Install flow:** picking an app writes its compose project under the app-data root
  using the **same on-disk file structure as CasaOS**, and brings it up **as-is** via the
  Docker socket (CasaDash does not rewrite the store's yml), attaching it to the shared
  app network with a `{name}.{domain}` hostname — same behaviour as `casa-img`. Installed
  apps then appear as tiles on the dashboard.
- **Automated updates:** CasaDash can update installed store apps automatically when the
  store listing changes.
- **Env / template injection:** honour the CasaOS/`casa-img` template variables so
  existing store listings resolve correctly — `DATA_ROOT`, `REF_NET`, `REF_PORT`,
  `REF_DOMAIN`, `REF_SCHEME`, plus base vars (`PUID`, `PGID`, `TZ`, `AppID`) and the app's
  own `.env`. See
  `casa-img/docs/environment-variable-injection.md` for the exact substitution rules.

### 5.1 Per-app configuration (diverges from CasaOS)

CasaDash keeps a **CasaOS-style app config window** (edit ports, environment, volumes,
etc.), but **diverges on how edits are persisted**:

- The store's original `docker-compose.yml` is **never modified** — it stays exactly as
  shipped, so updates stay clean.
- User edits are written to a **separate override compose file** layered on top of the
  original (Compose override semantics). The running app = original + override.
- This applies to store apps. Unmanaged apps (discovered externally) are surfaced
  read-mostly; their source of truth is whatever created them.

### 5.2 App operations & observability

- **Lifecycle controls** (from the tile burger menu): Open, Settings, Restart, Stop,
  Start, Uninstall — same set as CasaOS.
- **Live per-app logs** and **per-container resource stats** are viewable in-app.
- **Real-time status** (running/stopped/health) updates live on the dashboard. *(The
  transport for live updates is intentionally left unspecified at spec stage — see §8.)*

---

## 6. Architecture (spec-level)

> No language, runtime, or process model is chosen yet. This describes the **logical
> capabilities** the container must expose, not how they're implemented.

```
┌──────────────────────── CasaDash container ─────────────────────────┐
│                                                                      │
│   Dashboard UI (CasaOS-style)  ──►  application logic                 │
│                                      ├── system stats (CPU/RAM/disk) │
│                                      ├── app registry (managed +     │
│                                      │     unmanaged/x-casaos)        │
│                                      ├── app store (fetch/parse,      │
│                                      │     multi-store, auto-update)  │
│                                      ├── per-app logs / stats / status│
│                                      └── compose control (as-is +     │
│                                            override file) ────────────┼──► /var/run/docker.sock
│                                                                      │        (host engine)
│   Serves on one HTTP port (:8080)                                     │
└──────────────────────────────────────────────────────────────────────┘
        │                                        │
        ▼                                        ▼
   reverse proxy / VPN                    app containers on shared
   (operator-provided)                    "mesh"/bridge network
```

- **One HTTP port (`:8080`).**
- **No in-image Docker engine.** CasaDash talks to the **host** daemon over the mounted
  socket. It needs access to the socket's group to drive it (the `casa-img` socket-GID
  approach — see `casa-img/s6-overlay/scripts/casa-init.sh` — is a proven reference).
- **App unit = a multi-service Docker Compose project**, stored with the **same on-disk
  layout as CasaOS** under the app-data root, managed through the socket (create / up /
  down / remove). Store apps run **as-is**; user config is applied via a separate
  **override compose file** (§5.1).
- **Unmanaged apps:** CasaDash also observes Docker for externally-created Compose stacks
  carrying `x-casaos` and lists them (without managing their lifecycle source).
- **Live data:** per-app logs, container stats, and real-time status are exposed to the
  UI (transport TBD, §8).
- **State** lives on-disk under the app-data root (app registry, store cache,
  settings/wallpaper/language) — no database required.

## 7. Deployment

Same model as `casa-img`: one container, host Docker socket bind-mounted, one published
port, apps on a shared bridge network.

```yaml
services:
  casadash:
    image: casadash:latest
    container_name: casadash
    hostname: casadash
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      DOCKER_API_VERSION: "1.44"   # pin SDK to host daemon
      PUID: "1000"
      PGID: "1000"
      TZ: "Europe/Paris"
      DATA_ROOT: "${DATA_HOST_PATH:-/DATA}"
      # app-store routing template (as in casa-img)
      REF_NET: "mesh"
      REF_PORT: "80"
      REF_SCHEME: "http"
      REF_DOMAIN: "${DOMAIN:-}"
      # store source (CasaOS-compliant zip)
      APPSTORE_URL: "https://github.com/Yundera/AppStore/archive/refs/heads/main.zip"
    networks:
      - mesh
    volumes:
      - type: bind                    # app data + installed compose projects
        source: ${DATA_HOST_PATH:-/DATA}
        target: /DATA
        bind:
          propagation: rshared
      - type: bind                    # host Docker socket — how apps get managed
        source: /var/run/docker.sock
        target: /var/run/docker.sock

networks:
  mesh:
    driver: bridge
    name: mesh
```

**Notes**

- **`/var/run/docker.sock`** is the only privilege CasaDash needs — it is how it
  installs and runs apps. (Mounting the socket is equivalent to root on the host; run
  CasaDash only on hosts you trust it on.)
- **`/DATA`** (bind, `rshared`) holds app data and the compose projects CasaDash writes,
  so volumes created inside are visible on the host.
- **No `/dev`, no samba/mergerfs mounts** — those existed in `casa-img` for the file/disk
  features CasaDash drops.
- **Health:** `GET /ping` (or `/`) → 200.
- **TLS / public routing** is out of scope — front CasaDash with a reverse proxy
  (Caddy/Traefik/mesh-router) exactly as `casa-img` is fronted.
- **No auth:** because there is no login, do **not** expose `:8080` directly to the
  internet. Put it behind a VPN or an authenticating proxy.

## 8. Open questions / to decide during build

Implementation choices deferred until we leave spec stage:

- **Live-update transport:** how real-time status / logs / stats reach the UI (push vs
  poll, and the wire protocol). The *feature* is committed; the mechanism is open.
- **UI reuse:** fork/trim the existing CasaOS dashboard UI for guaranteed pixel-parity, or
  rebuild fresh against the same design? Parity requirement favours reusing the CasaOS UI.
- **Language / runtime:** not yet chosen.
- **Unmanaged-app depth:** exactly how much lifecycle control (if any) to offer over
  externally-created stacks vs. read-only display.

## 9. Reference material

- **Slim-from source of truth:** `D:\workspace\yundera\yundera-root\packages\casa-img`
  (`/d/workspace/yundera/yundera-root/packages/casa-img`).
  - `Dockerfile` — the 8-service bundle CasaDash trims.
  - `dev/docker-compose.yml` — socket + `/DATA` mount, network, env (deployment model).
  - `conf/app-management/app-management.conf` — store URL + app paths.
  - `s6-overlay/scripts/casa-init.sh` — socket-GID discovery trick.
  - `docs/environment-variable-injection.md` — store template/env substitution rules.
- **Live UX reference:** <https://demo.nsl.sh/#/> (casa-img demo).
- **CasaOS App Store format:** <https://github.com/Yundera/AppStore> and upstream
  <https://github.com/IceWhaleTech/CasaOS-AppStore>.
