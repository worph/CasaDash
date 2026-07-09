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
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/yundera/casadash/internal/config"
)

// Env returns the process environment plus the base interpolation variables so
// that `docker compose` resolves ${PUID}, ${DATA_ROOT}, ${AppID}, etc.
func Env(cfg config.Config, appID string) []string {
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

	env := os.Environ()
	for k, v := range extra {
		env = append(env, k+"="+v)
	}
	return env
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
	// Resolve the host-path placeholders back to the container mount point.
	for _, tok := range []string{"${DATA_ROOT}", "$DATA_ROOT", "${DATA_HOST_PATH}", "$DATA_HOST_PATH"} {
		src = strings.ReplaceAll(src, tok, cfg.DataRoot)
	}
	if cfg.DataHostPath != "" && strings.HasPrefix(src, cfg.DataHostPath) {
		src = cfg.DataRoot + src[len(cfg.DataHostPath):]
	} else if strings.HasPrefix(src, "/DATA") {
		src = cfg.DataRoot + src[len("/DATA"):]
	}
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

// RewriteToHostPath replaces literal /DATA and ${DATA_ROOT} references with the
// host data path. Used on x-casaos install hooks, whose commands run against the
// host daemon (via DOCKER_HOST) and must therefore use host paths.
func RewriteToHostPath(s string, cfg config.Config) string {
	if cfg.DataHostPath == "" || cfg.DataHostPath == "/DATA" {
		return s
	}
	s = strings.ReplaceAll(s, "${DATA_ROOT}", cfg.DataHostPath)
	s = strings.ReplaceAll(s, "$DATA_ROOT", cfg.DataHostPath)
	s = strings.ReplaceAll(s, "/DATA", cfg.DataHostPath)
	return s
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
		svc["networks"] = append(nets, refNet)
	case map[string]any:
		nets[refNet] = nil
	default:
		svc["networks"] = []any{refNet}
	}
}
