# `x-compose-app` — CasaDash's compose extension

`x-compose-app` is a Compose top-level extension that CasaDash reads to render an
app tile, open its web UI, and populate the store. It exists **alongside**
`x-casaos`, not instead of it:

- CasaDash still consumes the **unmodified CasaOS App Store** (`x-casaos`). Nothing
  here requires changing existing store apps.
- When an app *also* carries `x-compose-app`, CasaDash **prefers** it for every
  field it defines, and falls back to `x-casaos` (then to derivation) for anything
  it omits.
- An app may ship `x-compose-app` **alone** — CasaDash renders it fully without an
  `x-casaos` block.

The design goal is a **click URL that mirrors the reverse-proxy route**. Instead of
CasaOS's approach — asking for a container port and *deriving* a hostname at
install time — `x-compose-app` lets the author declare the **final web-UI URL**
directly, the same way they already declare the app's Caddy route. The
`webui-host` value *is* the Caddy label's host:

```yaml
services:
  jellyfin:
    labels:
      caddy_0: jellyfin-${DOMAIN}            # the route
x-compose-app:
  webui-host: jellyfin-${domain}             # the click URL host — same string
```

> Scope: this document specifies **only what CasaDash consumes**. Unknown keys are
> tolerated and skipped.

---

## Top-level shape

```yaml
# docker-compose.yml
name: jellyfin
services:
  jellyfin:
    image: jellyfin/jellyfin:10.9.11
    expose: ["8096"]
    labels:
      caddy_0: jellyfin-${DOMAIN}
      caddy_0.reverse_proxy: "{{upstreams 8096}}"

x-compose-app:
  schema_version: 1
  id: jellyfin
  title: Jellyfin
  icon: https://cdn.example.com/jellyfin.svg
  category: Media
  tagline: Free software media system
  description: |
    Jellyfin is a media server for organizing and streaming your collection.
  developer: Jellyfin
  screenshots:
    - https://cdn.example.com/jellyfin/1.png

  # --- the click URL ---
  webui-host: jellyfin-${domain}
  webui-path: /web/
```

### Fields

| Field | Type | Required | Meaning / CasaDash use | `x-casaos` fallback |
|---|---|---|---|---|
| `schema_version` | int | no (default `1`) | Spec version. CasaDash refuses versions it doesn't understand and falls back to `x-casaos`. | — |
| `id` | string | no | Stable app identifier (should equal the Compose project `name`). Defaults to the project name. | `store_app_id` |
| `title` | string \| localized | no | Tile + store display name. | `title` |
| `icon` | url | no | Tile icon. | `icon` |
| `category` | string | no | Store grouping. | `category` |
| `tagline` | string \| localized | no | One-line store summary. | `tagline` |
| `description` | string \| localized | no | Store long description (Markdown). | `description` |
| `developer` | string | no | Store attribution. | `developer` / `author` |
| `screenshots` | url[] | no | Store gallery. | `screenshot_link` |
| `thumbnail` | url | no | Store card image. | `thumbnail` |
| `architectures` | string[] | no | e.g. `[amd64, arm64]`. Advisory. | `architectures` |
| **`webui-host`** | string | **yes\*** | The web UI's **host** — the final URL host, templated exactly like the app's Caddy route (e.g. `jellyfin-${domain}`). Omit for headless apps. | derived from `hostname` |
| `webui-port` | string | no (default `""`) | The **URL** port, appended as `:<port>`. **Not** the container port — it exists only to build the URL and is empty in the common gateway case (standard 443). | derived from `port_map` |
| `webui-scheme` | `http` \| `https` | no (default `https`) | The URL scheme the **browser** uses. | `scheme` |
| `webui-path` | string | no (default `/`) | Path appended to the host. May include a query string (e.g. `/?hash=$AUTH_HASH`). | `index` |
| `links` | object[] | no | Extra buttons on the detail view: `{ name, url, icon? }` with an **absolute** `url`. Never the tile's default action. | — |
| `tips` | object | no | `{ before_install: string \| localized }` shown in the install dialog. | `tips.before_install` |
| `hooks` | object | no | `{ pre_install: string, post_install: string }` — host shell at install/uninstall, same contract as `x-casaos` `pre-install-cmd` / `post-install-cmd`. | `pre-install-cmd` / `post-install-cmd` |

\* `webui-host` is required only to have a **clickable app**. An app with no
`webui-host` (and no `x-casaos` fallback) is headless — its tile has no "open"
action.

**Localized** means either a plain string (`title: Jellyfin`) or a locale map
(`title: { en_us: Jellyfin, fr_fr: Jellyfin }`). CasaDash prefers `en_us`.

---

## The web-UI URL

CasaDash builds the click URL by **direct string construction** — no container
ports, no reading routes back, no baked-in state:

```
<webui-scheme>://<resolved webui-host><:webui-port if set><webui-path>
```

| Part | Source | Default |
|---|---|---|
| scheme | `webui-scheme` | `https` |
| host | `webui-host`, after placeholder resolution | — (required) |
| port | `webui-port` | `""` → omitted |
| path | `webui-path` | `/` |

### Host placeholders

`webui-host` may contain deployment placeholders, resolved by CasaDash from its
own configuration (so the value can be shared verbatim with the Caddy label):

| Placeholder | Resolves to | Source |
|---|---|---|
| `${domain}` / `${DOMAIN}` | the deployment's base domain | CasaDash `REF_DOMAIN` |

- Resolution happens on **every render**, so the URL tracks a domain change and
  works for **unmanaged/discovered** apps CasaDash never installed — nothing is
  stored.
- If `webui-host` references `${domain}` but the deployment has no domain
  configured (`REF_DOMAIN` empty), the URL is treated as **unresolvable**: the tile
  shows the "no reachable address" hint rather than a broken link.
- A literal `webui-host` (no placeholder, e.g. `nas.example.com`) is used verbatim.

### Examples

**Gateway app** (behind Caddy — the common case):

```yaml
x-compose-app:
  webui-host: jellyfin-${domain}
  webui-path: /web/
# REF_DOMAIN=app.localhost  →  https://jellyfin-app.localhost/web/
```

**Direct port access** (no gateway) — set a literal host and a URL port:

```yaml
x-compose-app:
  webui-host: nas.example.com
  webui-scheme: http
  webui-port: "8096"
# → http://nas.example.com:8096/
```

**Headless app**: omit `webui-host` → the tile has no "open" action.

**Extra buttons**:

```yaml
x-compose-app:
  webui-host: photoprism-${domain}
  links:
    - name: Docs
      url: https://docs.photoprism.app
```

---

## Precedence

For any concern, CasaDash reads in this order and stops at the first hit:

```
x-compose-app  →  x-casaos  →  runtime derivation (published host port)
```

So `webui-host` wins over the `x-casaos` `hostname`/`port_map` derivation, which in
turn wins over "guess a published host port". An author adds `webui-host` to pin
the click URL and keeps `x-casaos` for CasaOS-store compatibility.

## Minimum viable block

The smallest `x-compose-app` that changes CasaDash's behavior is just the host:

```yaml
x-compose-app:
  webui-host: myapp-${domain}
```

Everything else falls back to `x-casaos` or Compose metadata.
