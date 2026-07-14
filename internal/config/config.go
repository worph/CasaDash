// Package config holds runtime configuration derived from the environment and
// the persisted settings file.
package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/yundera/casadash/internal/domains"
)

// Config is the process-wide runtime configuration.
type Config struct {
	Addr string // listen address, e.g. ":8080"

	// DataRoot is the data folder as seen INSIDE this container (the bind-mount
	// target). CasaDash reads/writes its own files here (compose projects, store
	// cache, settings).
	DataRoot string

	// DataHostPath is the SAME data folder's path on the Docker host. Because
	// app bind-mount sources are resolved by the host daemon (not inside this
	// container), generated app compose files must reference host paths. Defaults
	// to DataRoot when the container mount point equals the host path.
	DataHostPath string

	// PCS / store templating (see internal/envinject). Empty is fine for local use.
	RefNet    string
	RefPort   string
	RefScheme string
	RefDomain string
	RefSep    string

	PUID string
	PGID string
	TZ   string

	StoreURLs []string // app-store zip URLs (multi-store)

	// ProtectedApps names apps the user must not uninstall from the dashboard —
	// the platform's own pieces (casadash, casaos, …), which appear as ordinary
	// tiles but whose Uninstall entry is hidden and whose DELETE is refused. An
	// entry matches an app's store ID (x-casaos store_app_id / x-compose-app id)
	// or, failing that, its compose project name. Case-insensitive.
	ProtectedApps []string

	// Domains returns the additional domains every app is published on, beyond the
	// primary one its compose already routes (see internal/caddyroutes). It is a
	// function, not a slice, because the operator edits the list at runtime while
	// Config is a value copied once at boot — reading it live is what lets a
	// settings change reach the next `docker compose up` without a restart.
	//
	// nil means the feature is unwired (a Config built outside the server), and no
	// override is ever touched.
	Domains func() []domains.Domain
}

// FromEnv builds a Config from environment variables with sensible defaults.
func FromEnv() Config {
	dataRoot := envOr("DATA_ROOT", "/DATA")
	c := Config{
		Addr:         envOr("CASADASH_ADDR", ":8080"),
		DataRoot:     dataRoot,
		DataHostPath: envOr("DATA_HOST_PATH", dataRoot),
		RefNet:       os.Getenv("REF_NET"),
		RefPort:      os.Getenv("REF_PORT"),
		RefScheme:    os.Getenv("REF_SCHEME"),
		RefDomain:    os.Getenv("REF_DOMAIN"),
		RefSep:       envOr("REF_SEPARATOR", "-"),
		PUID:         envOr("PUID", "1000"),
		PGID:         envOr("PGID", "1000"),
		TZ:           os.Getenv("TZ"),
		StoreURLs: splitList(envOr("APPSTORE_URL",
			"https://github.com/Yundera/AppStore/archive/refs/heads/main.zip")),
		ProtectedApps: splitList(os.Getenv("PROTECTED_APPS")),
	}
	return c
}

// IsProtected reports whether an app is exempt from uninstall. Both identifiers
// are tested: storeID (the store's app id, e.g. "casadash") and project (the
// compose project / folder name), so a protected app is caught whether or not it
// carries store metadata.
func (c Config) IsProtected(storeID, project string) bool {
	for _, p := range c.ProtectedApps {
		if storeID != "" && strings.EqualFold(p, storeID) {
			return true
		}
		if project != "" && strings.EqualFold(p, project) {
			return true
		}
	}
	return false
}

// AppsDir is the flat root that holds one directory per app
// (${DATA_ROOT}/AppData/<app>). Each app directory carries its own
// docker-compose.yml, docker-compose.override.yml, .env, and data — the folder's
// presence is what makes an app appear on the dashboard. See docs/app-model.md.
func (c Config) AppsDir() string {
	return filepath.Join(c.DataRoot, "AppData")
}

// StateDir is where CasaDash's own state (settings, store cache) lives. It sits
// under AppData with a dot-prefixed name so the app model's "a dot in the name
// hides it" rule keeps it off the dashboard.
func (c Config) StateDir() string {
	return filepath.Join(c.DataRoot, "AppData", ".casadash")
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
