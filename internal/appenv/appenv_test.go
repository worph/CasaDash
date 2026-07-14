package appenv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yundera/casadash/internal/config"
	"github.com/yundera/casadash/internal/envinject"
)

// newCfg gives a Config rooted at a temp dir, with a .env.app holding envApp.
func newCfg(t *testing.T, envApp string) config.Config {
	t.Helper()
	cfg := config.Config{
		DataRoot:     t.TempDir(),
		DataHostPath: "/host/DATA",
		PUID:         "1000",
		PGID:         "1000",
		TZ:           "Europe/Paris",
	}
	if err := os.MkdirAll(cfg.StateDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if envApp != "" {
		if err := os.WriteFile(Path(cfg), []byte(envApp), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg.AppEnv = func() map[string]string { return Load(cfg) }
	return cfg
}

func appDir(t *testing.T, cfg config.Config, dotEnv string) string {
	t.Helper()
	dir := filepath.Join(cfg.AppsDir(), "psitransfer")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if dotEnv != "" {
		if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(dotEnv), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func readEnv(t *testing.T, dir string) map[string]string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	return envinject.EnvFileVars(raw)
}

// Ensure writes the documented default once, and never overwrites the
// deployment's own file — overwriting on upgrade would silently revert its
// domain, network and credentials.
func TestEnsureSeedsDefaultAndNeverOverwrites(t *testing.T) {
	cfg := config.Config{DataRoot: t.TempDir()}

	if err := Ensure(cfg); err != nil {
		t.Fatal(err)
	}
	if v := Load(cfg)["APP_NET"]; v != "mesh" {
		t.Errorf("default APP_NET = %q, want mesh", v)
	}

	if err := os.WriteFile(Path(cfg), []byte("APP_NET=pcs\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Ensure(cfg); err != nil {
		t.Fatal(err)
	}
	if v := Load(cfg)["APP_NET"]; v != "pcs" {
		t.Errorf("Ensure clobbered the deployment's file: APP_NET = %q, want pcs", v)
	}
}

// The core contract: every key of .env.app is ensured in the app's .env, merged
// with the vars CasaDash computes — and nothing else in the file is disturbed.
func TestSyncEnsuresDeploymentAndComputedVars(t *testing.T) {
	cfg := newCfg(t, "APP_NET=pcs\nAPP_DOMAIN=example.com\n")
	// A stale value, a comment, and a key that is the operator's alone.
	dir := appDir(t, cfg, "# psitransfer\nAPP_DOMAIN=old.example\nPSI_RETENTION=7d\n")

	if err := Sync(cfg, "psitransfer", dir); err != nil {
		t.Fatal(err)
	}
	got := readEnv(t, dir)

	for k, want := range map[string]string{
		"APP_DOMAIN":     "example.com", // refreshed from .env.app
		"APP_NET":        "pcs",         // appended from .env.app
		"AppID":          "psitransfer", // computed by CasaDash
		"DATA_ROOT":      "/host/DATA",  // computed by CasaDash
		"DATA_HOST_PATH": "/host/DATA",
		"TZ":             "Europe/Paris",
		"PSI_RETENTION":  "7d", // the operator's — untouched
	} {
		if got[k] != want {
			t.Errorf("%s = %q, want %q", k, got[k], want)
		}
	}

	raw, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(raw), "# psitransfer\n") {
		t.Errorf("the operator's comment must survive:\n%s", raw)
	}
}

// Keys are ensured one by one, so neither file's ordering can matter.
func TestSyncIsOrderIndependent(t *testing.T) {
	a := newCfg(t, "APP_NET=pcs\nAPP_DOMAIN=example.com\nAPP_EMAIL=x@y.z\n")
	b := newCfg(t, "APP_EMAIL=x@y.z\nAPP_DOMAIN=example.com\nAPP_NET=pcs\n")

	dirA := appDir(t, a, "APP_DOMAIN=old\nPSI=1\n")
	dirB := appDir(t, b, "APP_DOMAIN=old\nPSI=1\n")

	if err := Sync(a, "psitransfer", dirA); err != nil {
		t.Fatal(err)
	}
	if err := Sync(b, "psitransfer", dirB); err != nil {
		t.Fatal(err)
	}

	rawA, _ := os.ReadFile(filepath.Join(dirA, ".env"))
	rawB, _ := os.ReadFile(filepath.Join(dirB, ".env"))
	if string(rawA) != string(rawB) {
		t.Errorf(".env.app ordering leaked into the result:\n--- a ---\n%s\n--- b ---\n%s", rawA, rawB)
	}
}

func TestSyncIsIdempotent(t *testing.T) {
	cfg := newCfg(t, "APP_NET=pcs\nAPP_DOMAIN=example.com\n")
	dir := appDir(t, cfg, "APP_DOMAIN=old.example\n")

	if err := Sync(cfg, "psitransfer", dir); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(filepath.Join(dir, ".env"))
	if err := Sync(cfg, "psitransfer", dir); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(filepath.Join(dir, ".env"))

	if string(first) != string(second) {
		t.Errorf("Sync is not idempotent:\n--- 1 ---\n%s\n--- 2 ---\n%s", first, second)
	}
}

// An empty value in .env.app means "the deployment does not have this". Writing it
// blank would route an app at an empty host; leaving the reference unresolved at
// least makes compose say so.
func TestSyncSkipsEmptyValues(t *testing.T) {
	cfg := newCfg(t, "APP_DOMAIN=\nAPP_NET=pcs\n")
	dir := appDir(t, cfg, "")

	if err := Sync(cfg, "psitransfer", dir); err != nil {
		t.Fatal(err)
	}
	if v, ok := readEnv(t, dir)["APP_DOMAIN"]; ok {
		t.Errorf("empty APP_DOMAIN must be skipped, got %q", v)
	}
}

// Sync creates the .env for a fresh install rather than requiring one to exist.
func TestSyncCreatesEnvForAFreshInstall(t *testing.T) {
	cfg := newCfg(t, "APP_NET=pcs\n")
	dir := appDir(t, cfg, "")

	if err := Sync(cfg, "psitransfer", dir); err != nil {
		t.Fatal(err)
	}
	got := readEnv(t, dir)
	if got["AppID"] != "psitransfer" || got["APP_NET"] != "pcs" {
		t.Errorf("fresh .env not prefilled: %v", got)
	}
}
