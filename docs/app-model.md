# App storage & lifecycle model

How CasaDash lays apps out on disk and derives their dashboard state. This model
**diverges from CasaOS** on purpose: the on-disk folder is the single source of
truth for *what apps exist*, and the live Docker state is the single source of
truth for *how each one is doing*. Nothing about an app is kept in a database or a
registry file — the filesystem and the Docker daemon **are** the state.

> This document is authoritative for the app layout and supersedes the older
> `AppData/casaos/apps/<app>` nesting described elsewhere. CasaDash uses a **flat**
> `AppData/<app>` layout.

---

## On-disk layout

Every app is one directory directly under the data root:

```
/DATA/AppData/<app>/
├── docker-compose.yml            # strict copy from the store — never modified
├── docker-compose.override.yml   # user edits from the config window (Compose override)
├── .env                          # prefilled by CasaDash on create, then user-editable
└── …                             # any other files the app needs (configs, seed data, …)
```

`<app>` is the Compose project name and the tile identity. The directory name is
what the dashboard shows.

### File roles

| File | Origin | Mutated by | Purpose |
|------|--------|-----------|---------|
| `docker-compose.yml` | **Strict copy from the store listing.** | Never — CasaDash treats it as read-only. | The pristine app definition. Keeping it byte-for-byte identical to the store is what lets updates stay clean (re-copy on update, overrides survive). |
| `docker-compose.override.yml` | Generated from the **per-app config window** (ports, env, volumes, …). | CasaDash, on every config save. | User customizations, layered on top via Compose override semantics. The running stack = `docker-compose.yml` + `docker-compose.override.yml`. |
| `.env` | **Prefilled by CasaDash on create** (PUID/PGID, TZ, `REF_*`, domain, generated secrets, …), then hand-editable. | CasaDash on create; user thereafter. | Variable substitution for both compose files. |
| everything else | The app (bind-mount targets, config files, databases, …). | The app at runtime. | User data. **Never** deleted by CasaDash (see uninstall). |

The stack is always brought up from this directory as
`docker compose -f docker-compose.yml -f docker-compose.override.yml … up`, with
`.env` resolved from the same folder — so what runs is exactly what is on disk.

### Update reference (in the override's `x-compose-app`)

On install, CasaDash records **where the app came from** so it can later pull a
fresher `docker-compose.yml`. The reference lives in the override's
`x-compose-app` block (so it survives base re-copies, and the strict base stays
byte-identical to the store):

```yaml
# docker-compose.override.yml
x-compose-app:
  store: https://github.com/Yundera/AppStore/archive/refs/heads/main.zip  # reference store
  store-app-id: jellyfin                                                  # catalog id within it
```

The per-app **Update** tab uses this to:

1. fetch the store's current listing for `store-app-id` from `store`,
2. apply the same PCS transform used at install time, and
3. compare it byte-for-byte with the installed `docker-compose.yml`.

When they differ, **Update now** overwrites the strict base with the store's
version and runs `docker compose up -d` (base + override). The override and
`.env` are never touched. Apps with no recorded reference (installed before this
feature, or unmanaged stacks) simply report "no update reference".

---

## State model — the folder and Docker together

CasaDash never invents state. A tile's existence comes from the **folder**; a
tile's appearance comes from **Docker**.

### 1. Existence — driven by the folder

```
folder present at /DATA/AppData/<app>   ≡   an app tile in CasaDash
no folder                               ≡   no tile
```

Create a folder (even by hand) → the app appears. Remove/rename it → the app
disappears. There is nothing else to register.

### 2. Appearance — driven by the live Docker state

For each existing app, the tile reflects what Docker reports for that project:

| Docker state | Tile appearance | Interaction |
|---|---|---|
| **No live stack** (folder exists, stack not started / fully down) | **Greyed** icon | Burger menu available (Start, Settings, Uninstall, …). Not clickable to "open". |
| **Operation in progress** (up / down / restart / pull mid-flight) | Greyed icon with a **`…` overlay** | **No burger menu** while the operation runs — the tile is busy. |
| **Live stack** (running) | **Full-colour, clickable** icon | Click opens the web UI; burger menu available. |

### 3. Health dot — driven by the Docker health check

A small dot in the **top-left** of the tile reflects the container health check:

| Dot | Meaning |
|---|---|
| 🟢 **Green** | Health check passing. |
| 🟠 **Orange** | Health check failing / unhealthy (or still `starting`). |

The dot only appears when Docker reports a health check for the stack; a stack
with no health check has no dot.

---

## Uninstall = archive by rename (never delete)

CasaDash **never removes user data.** Uninstalling an app **renames** its folder to
a dated archive name alongside it:

```
/DATA/AppData/<app>
      ↓ uninstall (2026-07-10)
/DATA/AppData/<app>.2026-07-10.archive
```

- The stack is stopped and its containers removed first, then the directory is
  renamed. The bytes on disk are untouched — only the name changes.
- **Zip is an option.** When enabled, the folder is compressed to
  `<app>.2026-07-10.archive.zip` instead of a plain renamed directory. Default is a
  plain rename (fast, no copy).
- To "reinstall", the operator renames the archive back to `<app>` (or unzips it) —
  data returns exactly as it was.

Because the archive name **contains dots**, it is automatically hidden from the
dashboard (next section) — an uninstalled app vanishes from the grid without any
data being destroyed.

---

## Dot in a name = hidden

`.` is a **reserved character** for CasaDash. Any entry under `AppData/` whose name
contains a `.` is **not displayed** as an app:

```
AppData/jellyfin                     → shown  (tile "jellyfin")
AppData/jellyfin.2026-07-10.archive  → hidden (uninstalled archive)
AppData/jellyfin.2026-07-10.archive.zip → hidden
AppData/.tmp-download                → hidden (scratch / hidden dir)
```

This single rule does double duty:

- It keeps **archives** (which always carry a date-dotted suffix) out of the grid.
- It gives CasaDash a namespace for **scratch / internal** folders — anything it
  doesn't want to surface, it names with a `.`.

An app that needs to be visible therefore **must not** have a `.` in its directory
name.

---

## Summary

| Concern | Source of truth |
|---|---|
| Which apps exist | Presence of `AppData/<app>/` (dot-free name) |
| App definition | `docker-compose.yml` (strict store copy) + `docker-compose.override.yml` (user edits) |
| Variables | `.env` (prefilled on create) |
| Running / stopped / busy / clickable | Live Docker state |
| Health dot | Docker health check |
| Uninstall | Rename to `<app>.<date>.archive` (optionally `.zip`) — data never deleted |
| Hidden entries | Any name containing `.` |
</content>
</invoke>
