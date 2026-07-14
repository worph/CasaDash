// Package appenv owns .env.app — the deployment's statement of what every app
// receives.
//
// CasaOS mixes the variables it needs to run with the variables it forwards to the
// apps it manages, in one environment. CasaDash separates them: what CasaDash needs
// stays in CasaDash's own environment (DATA_ROOT, APPSTORE_URL, PROTECTED_APPS, …)
// and is never forwarded; what an app receives is written in .env.app, which lives
// with the deployment's data rather than in CasaDash's compose. Nothing is in both,
// so there is no question of which one wins.
//
// The file is read on install and on every start, and each of its keys is ensured
// in the app's own .env (see Sync). It is the deployment's to write — CasaDash
// creates it once with a documented default and never overwrites it.
package appenv

import (
	_ "embed"
	"os"
	"path/filepath"

	"github.com/yundera/casadash/internal/config"
	"github.com/yundera/casadash/internal/envinject"
)

// defaultFile is the .env.app CasaDash writes when a deployment has none. It is
// heavily commented: it is the operator's primary interface to what apps receive.
//
//go:embed default.env.app
var defaultFile []byte

// Path is where the deployment's .env.app lives — beside CasaDash's own state,
// under the dot-prefixed directory the app model keeps off the dashboard.
func Path(cfg config.Config) string {
	return filepath.Join(cfg.StateDir(), ".env.app")
}

// Ensure creates .env.app with the shipped default when the deployment has none,
// and leaves it alone otherwise. Called once at boot.
//
// It never rewrites an existing file. The file is the deployment's — a PCS writes
// it at provisioning, an operator edits it by hand — and CasaDash overwriting it on
// upgrade would silently revert their domain, network and credentials.
func Ensure(cfg config.Config) error {
	path := Path(cfg)
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, defaultFile, 0o644)
}

// Load reads the deployment's app-facing variables. A key with an empty value is
// omitted: .env.app states what a deployment *has*, and an app is better off with
// an unresolved ${APP_DOMAIN} — which compose reports — than with a blank one,
// which silently routes it at nothing.
//
// A missing or unreadable file yields no variables rather than an error: apps then
// get only what CasaDash computes for them, which is a working local install.
func Load(cfg config.Config) map[string]string {
	raw, err := os.ReadFile(Path(cfg))
	if err != nil {
		return nil
	}
	out := map[string]string{}
	for _, v := range envinject.ParseEnvFile(raw) {
		if v.Value != "" {
			out[v.Key] = v.Value
		}
	}
	return out
}

// Sync ensures every variable the app should have in the app's own .env: the
// deployment's (.env.app) merged with the ones CasaDash computes per app and per
// install (envinject.BaseVars — AppID, PUID/PGID/TZ, the data root).
//
// Each key is ensured independently: one already in the app's .env is set to the
// current value in the line it already occupies, one that is missing is appended.
// Neither file's ordering matters, and a key CasaDash does not forward is never
// touched — an app's .env is free to carry the operator's own variables.
//
// This is what keeps an installed app startable after the deployment moves. The
// app's compose refers to its surroundings only through ${APP_NET}, ${DATA_ROOT},
// ${APP_DOMAIN}, … — never a baked literal (see envinject.Transform) — so a new
// network, data root, domain or IP is picked up on the next start rather than
// stranding the app until it is reinstalled.
//
// Writing the values into the .env, rather than relying on the environment
// `docker compose` inherits from CasaDash, is the point: a `docker compose up -d`
// run by hand in the app's folder must bring the app up exactly as CasaDash does.
func Sync(cfg config.Config, appID, appDir string) error {
	vars := envinject.BaseVars(cfg, appID)
	for k, v := range Load(cfg) {
		vars[k] = v
	}
	return envinject.EnsureVars(filepath.Join(appDir, ".env"), vars)
}
