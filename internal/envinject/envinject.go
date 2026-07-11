// Package envinject reproduces CasaOS/casa-img's environment and template
// handling for store apps: the base interpolation variables (consumed by
// `docker compose` interpolation) and the PCS structural transforms
// (DATA_ROOT volume rewrite, external REF_NET attach, PUID:PGID user).
//
// Ported from casa-img CasaOS-AppManagement: service/compose_service.go
// (baseInterpolationMap) and route/v2/appstore_pcs.go (modifyServices).
package envinject

import (
	"os"
	"path"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/yundera/casadash/internal/config"
)

// BaseVars returns the CasaOS/casa-img base interpolation variables for an app
// (PUID, PGID, TZ, DATA_ROOT, REF_*, AppID, …). These are what `docker compose`
// substitutes into ${VAR} references and what CasaDash seeds an app's .env with.
func BaseVars(cfg config.Config, appID string) map[string]string {
	tz := cfg.TZ
	if tz == "" {
		tz = "UTC"
	}
	// DATA_ROOT / DATA_HOST_PATH are exported as the HOST path: `${DATA_ROOT}` in
	// an app's compose is a bind-mount source, and the host daemon resolves it.
	extra := map[string]string{
		"AppID":           appID,
		"DefaultUserName": "admin",
		"DefaultPassword": "casaos",
		"PUID":            cfg.PUID,
		"PGID":            cfg.PGID,
		"TZ":              tz,
		"DATA_ROOT":       cfg.DataHostPath,
		"DATA_HOST_PATH":  cfg.DataHostPath,
	}
	addIf(extra, "REF_NET", cfg.RefNet)
	addIf(extra, "REF_PORT", cfg.RefPort)
	addIf(extra, "REF_SCHEME", cfg.RefScheme)
	addIf(extra, "REF_DOMAIN", cfg.RefDomain)
	addIf(extra, "REF_SEPARATOR", cfg.RefSep)
	return extra
}

// Env returns the process environment plus the base interpolation variables so
// that `docker compose` resolves ${PUID}, ${DATA_ROOT}, ${AppID}, etc.
func Env(cfg config.Config, appID string) []string {
	env := os.Environ()
	for k, v := range BaseVars(cfg, appID) {
		env = append(env, k+"="+v)
	}
	return env
}

// EnvFile renders the base variables as the contents of an app's .env file
// (sorted `KEY=VALUE` lines). CasaDash prefills this on install so the app's
// compose resolves offline and the operator can hand-edit it afterwards — the
// .env is the app's persistent variable record. See docs/app-model.md.
func EnvFile(cfg config.Config, appID string) []byte {
	vars := BaseVars(cfg, appID)
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(vars[k])
		b.WriteByte('\n')
	}
	return []byte(b.String())
}

// Render substitutes ${VAR} / $VAR references in s using the same variables the
// install-time `docker compose` run sees (Env): the process environment, overlaid
// with the app's base interpolation variables (BaseVars), overlaid with the
// KEY=VALUE lines of its persisted .env (operator edits win). References we can't
// resolve are left intact. Used to render an app's tips for display — store tips
// routinely reference the ambient vars (APP_DEFAULT_PASSWORD, DOMAIN, …) that
// only exist in the process environment.
func Render(s string, cfg config.Config, appID string, envFile []byte) string {
	vars := map[string]string{}
	for _, kv := range os.Environ() {
		if k, v, ok := strings.Cut(kv, "="); ok {
			vars[k] = v
		}
	}
	for k, v := range BaseVars(cfg, appID) {
		vars[k] = v
	}
	for k, v := range EnvFileVars(envFile) {
		vars[k] = v
	}
	return os.Expand(s, func(k string) string {
		if v, ok := vars[k]; ok {
			return v
		}
		return "${" + k + "}" // leave references we don't own untouched
	})
}

// EnvFileVars parses simple KEY=VALUE lines (the format EnvFile writes),
// skipping blanks and # comments.
func EnvFileVars(b []byte) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		out[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return out
}

// SeedVars appends the named variables to an app's .env when they are missing,
// taking each value from the environment CasaDash itself runs with.
//
// An app's compose references deployment variables that live in CasaDash's
// environment but not in its own base variables — the Caddy labels are templated
// with ${APP_DOMAIN} and ${APP_PUBLIC_IP_DASH}. `docker compose` run *by CasaDash*
// resolves those from the process environment (see Env), but a `docker compose
// up -d` the operator runs by hand in the app's folder has only the .env, and
// would resolve them to nothing — silently routing the app at an empty host.
// Seeding them keeps the folder self-contained, which is the whole promise of the
// app model.
//
// Only missing keys are added, and nothing is reordered or rewritten: the .env is
// the operator's file, and a value they changed by hand must win over the ambient
// one, on every up, forever.
func SeedVars(cfg config.Config, appID, envPath string, keys []string) error {
	raw, err := os.ReadFile(envPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	have := EnvFileVars(raw)
	vars := BaseVars(cfg, appID)

	var add strings.Builder
	for _, k := range keys {
		if _, ok := have[k]; ok {
			continue
		}
		v, ok := vars[k]
		if !ok {
			v = os.Getenv(k)
		}
		if v == "" {
			continue // nothing to seed it with — leave the reference unresolved
		}
		add.WriteString(k)
		add.WriteByte('=')
		add.WriteString(v)
		add.WriteByte('\n')
	}
	if add.Len() == 0 {
		return nil
	}

	out := raw
	if len(out) > 0 && !strings.HasSuffix(string(out), "\n") {
		out = append(out, '\n')
	}
	return os.WriteFile(envPath, append(out, add.String()...), 0o644)
}

// Transform applies the PCS structural rewrites to a store compose file:
//   - rewrite volume sources under /DATA to the configured DATA_ROOT
//   - attach the main service to the external REF_NET network (if set)
//
// It intentionally leaves ${VAR} interpolation to `docker compose`.
func Transform(raw []byte, cfg config.Config, mainService string) ([]byte, error) {
	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}

	services, _ := doc["services"].(map[string]any)

	// Rewrite literal /DATA volume sources to the host path so the host daemon
	// resolves the bind mounts correctly (no-op when they already match).
	if cfg.DataHostPath != "" && cfg.DataHostPath != "/DATA" {
		rewriteDataRoot(services, cfg.DataHostPath)
	}

	if cfg.RefNet != "" && services != nil {
		attachExternalNetwork(doc, services, mainService, cfg.RefNet)
	}

	return yaml.Marshal(doc)
}

// VolumeDirs returns the CONTAINER-side directories that back an app's bind
// mounts, so the installer can pre-create them (they then exist on the host via
// CasaDash's own data mount, ready for the app's bind sources). File-style bind
// sources (a '.' in the last segment) are skipped — Docker would otherwise
// create a directory where a file is expected.
func VolumeDirs(raw []byte, cfg config.Config) []string {
	var doc map[string]any
	if yaml.Unmarshal(raw, &doc) != nil {
		return nil
	}
	services, _ := doc["services"].(map[string]any)

	seen := map[string]bool{}
	var dirs []string
	add := func(src string) {
		container := toContainerPath(src, cfg)
		if container == "" || seen[container] {
			return
		}
		seen[container] = true
		dirs = append(dirs, container)
	}

	for _, s := range services {
		svc, ok := s.(map[string]any)
		if !ok {
			continue
		}
		vols, ok := svc["volumes"].([]any)
		if !ok {
			continue
		}
		for _, v := range vols {
			switch vol := v.(type) {
			case string:
				if i := strings.Index(vol, ":"); i > 0 {
					add(vol[:i])
				}
			case map[string]any:
				if src, ok := vol["source"].(string); ok {
					add(src)
				}
			}
		}
	}
	return dirs
}

// toContainerPath maps a compose bind source to the path CasaDash can create
// inside its own data mount, or "" if it isn't a data-root bind directory.
func toContainerPath(src string, cfg config.Config) string {
	src = ContainerPath(src, cfg)
	// Skip sources with unresolved variables we don't own (e.g. $AppID) — those
	// are handled by compose interpolation / install hooks, not pre-creation.
	if strings.Contains(src, "$") {
		return ""
	}
	// Only manage absolute paths under our data root; skip named volumes and
	// file-style binds.
	if !strings.HasPrefix(src, cfg.DataRoot) {
		return ""
	}
	if strings.Contains(path.Base(src), ".") {
		return ""
	}
	return src
}

// ContainerPath maps a host-side data path (the form written into an app's
// compose: `/DATA/...`, `${DATA_ROOT}/...`, or the literal host path) to the
// same location as seen INSIDE this container, so CasaDash can create it through
// its own data mount. Paths that don't live under the data root are returned
// unchanged — the caller decides whether to reject them.
func ContainerPath(src string, cfg config.Config) string {
	for _, tok := range []string{"${DATA_ROOT}", "$DATA_ROOT", "${DATA_HOST_PATH}", "$DATA_HOST_PATH"} {
		src = strings.ReplaceAll(src, tok, cfg.DataRoot)
	}
	if cfg.DataHostPath != "" && strings.HasPrefix(src, cfg.DataHostPath) {
		return cfg.DataRoot + src[len(cfg.DataHostPath):]
	}
	if strings.HasPrefix(src, "/DATA") {
		return cfg.DataRoot + src[len("/DATA"):]
	}
	return src
}

// HostPath maps a path inside this container's data mount to the same location as
// the Docker host sees it — the inverse of ContainerPath. Use it on real paths
// (an app's directory, a bind source); use RewriteToHostPath on script text, which
// carries the /DATA and ${DATA_ROOT} spellings instead of a resolved path.
func HostPath(p string, cfg config.Config) string {
	if cfg.DataRoot == "" || cfg.DataHostPath == "" || cfg.DataRoot == cfg.DataHostPath {
		return p
	}
	if strings.HasPrefix(p, cfg.DataRoot) {
		return cfg.DataHostPath + p[len(cfg.DataRoot):]
	}
	return p
}

// RewriteToHostPath replaces literal /DATA and ${DATA_ROOT} references with the
// host data path. Used on x-casaos install hooks, whose commands run against the
// host daemon (via DOCKER_HOST) and must therefore use host paths.
func RewriteToHostPath(s string, cfg config.Config) string {
	if cfg.DataHostPath == "" || cfg.DataHostPath == "/DATA" {
		return s
	}
	// One pass, not three ReplaceAll calls: a host path normally *ends* in /DATA
	// (e.g. /opt/casadash/DATA), so expanding ${DATA_ROOT} first and then rewriting
	// /DATA would rewrite the path we just wrote — /opt/casadash/opt/casadash/DATA.
	// A Replacer scans left to right and never re-scans what it emitted.
	return strings.NewReplacer(
		"${DATA_ROOT}", cfg.DataHostPath,
		"$DATA_ROOT", cfg.DataHostPath,
		"/DATA", cfg.DataHostPath,
	).Replace(s)
}

func addIf(m map[string]string, k, v string) {
	if v != "" {
		m[k] = v
	}
}

// rewriteDataRoot replaces a leading "/DATA" in every volume source.
func rewriteDataRoot(services map[string]any, dataRoot string) {
	for _, s := range services {
		svc, ok := s.(map[string]any)
		if !ok {
			continue
		}
		vols, ok := svc["volumes"].([]any)
		if !ok {
			continue
		}
		for i, v := range vols {
			switch vol := v.(type) {
			case string:
				vols[i] = replaceLeadingDATA(vol, dataRoot)
			case map[string]any:
				if src, ok := vol["source"].(string); ok {
					vol["source"] = replaceLeadingDATA(src, dataRoot)
				}
			}
		}
	}
}

func replaceLeadingDATA(s, dataRoot string) string {
	const p = "/DATA"
	if len(s) >= len(p) && s[:len(p)] == p {
		return dataRoot + s[len(p):]
	}
	return s
}

// attachExternalNetwork adds an external network and joins the main service to it.
func attachExternalNetwork(doc, services map[string]any, mainService, refNet string) {
	networks, _ := doc["networks"].(map[string]any)
	if networks == nil {
		networks = map[string]any{}
		doc["networks"] = networks
	}
	networks[refNet] = map[string]any{"name": refNet, "external": true}

	// Default the main service to the first one if unspecified.
	if mainService == "" {
		for name := range services {
			mainService = name
			break
		}
	}
	svc, ok := services[mainService].(map[string]any)
	if !ok {
		return
	}
	switch nets := svc["networks"].(type) {
	case []any:
		for _, n := range nets {
			if s, ok := n.(string); ok && s == refNet {
				return // already attached — avoid a duplicate list entry
			}
		}
		svc["networks"] = append(nets, refNet)
	case map[string]any:
		nets[refNet] = nil // idempotent
	default:
		svc["networks"] = []any{refNet}
	}
}
