package apps

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/yundera/casadash/internal/composecmd"
	"github.com/yundera/casadash/internal/composefile"
	"github.com/yundera/casadash/internal/envinject"
	"github.com/yundera/casadash/internal/xcasaos"
)

// noteFile is the app-local file CasaDash persists the user's editable note in.
const noteFile = "note.md"

// Config carries a managed app's base compose, its user override, and the
// effective web-UI (opening URL) configuration derived from both.
type Config struct {
	Base     string `json:"base"`
	Override string `json:"override"`
	WebUI    WebUI  `json:"webui"`
	// Tips is the store-provided guidance (x-casaos tips: before_install +
	// custom), read-only — it ships with the app definition.
	Tips string `json:"tips"`
	// Note is the user's own editable note, persisted in the app folder. It
	// doubles as CasaOS's per-app "tips" scratchpad.
	Note string `json:"note"`
}

// WebUI is the effective x-compose-app webui-* configuration (base + override
// merged), plus the resolved click URL for preview.
type WebUI struct {
	Scheme string `json:"scheme"`
	Host   string `json:"host"`
	Port   string `json:"port"`
	Path   string `json:"path"`
	URL    string `json:"url"`
}

// GetConfig reads a managed app's compose + override files and resolves its
// effective web-UI configuration.
func (r *Registry) GetConfig(id string) (*Config, error) {
	dir := filepath.Join(r.cfg.AppsDir(), id)
	base, err := os.ReadFile(filepath.Join(dir, "docker-compose.yml"))
	if err != nil {
		return nil, err
	}
	override, _ := os.ReadFile(filepath.Join(dir, "docker-compose.override.yml"))
	cfg := &Config{Base: string(base), Override: string(override)}

	if baseF, err := composefile.Parse(base); err == nil {
		overF, _ := composefile.Parse(override)
		si, ca := mergedMeta(baseF, overF)
		if ca != nil {
			cfg.WebUI = WebUI{
				Scheme: ca.WebUIScheme,
				Host:   ca.WebUIHost,
				Port:   ca.WebUIPort,
				Path:   ca.WebUIPath,
				URL:    ca.WebURL(r.cfg.RefDomain),
			}
		}
		if si != nil {
			cfg.Tips = storeTips(si)
		}
	}
	cfg.Note, _ = readNote(dir)
	return cfg, nil
}

// storeTips flattens the x-casaos tips (localized before_install guidance plus
// the free-form custom note) into a single block for display.
func storeTips(si *xcasaos.StoreInfo) string {
	var parts []string
	if b := strings.TrimSpace(xcasaos.Localized(si.Tips.BeforeInstall)); b != "" {
		parts = append(parts, b)
	}
	if c := strings.TrimSpace(si.Tips.Custom); c != "" {
		parts = append(parts, c)
	}
	return strings.Join(parts, "\n\n")
}

// readNote returns the user's per-app note (empty if none has been written).
func readNote(dir string) (string, error) {
	b, err := os.ReadFile(filepath.Join(dir, noteFile))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(b), nil
}

// SetNote persists (or clears) the user's per-app note. It touches only the
// app-local note file — no Docker recreate, since the note is pure metadata.
func (r *Registry) SetNote(id, note string) error {
	dir := filepath.Join(r.cfg.AppsDir(), id)
	if _, err := os.Stat(dir); err != nil {
		return err
	}
	notePath := filepath.Join(dir, noteFile)
	if strings.TrimSpace(note) == "" {
		if err := os.Remove(notePath); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return os.WriteFile(notePath, []byte(note), 0o644)
}

// SetConfig writes the override file (or removes it when empty) and recreates
// the app as base + override — CasaDash never mutates the store-provided base.
func (r *Registry) SetConfig(ctx context.Context, id, override string) error {
	dir := filepath.Join(r.cfg.AppsDir(), id)
	basePath := filepath.Join(dir, "docker-compose.yml")
	if _, err := os.Stat(basePath); err != nil {
		return err // only managed apps have an editable config
	}
	overridePath := filepath.Join(dir, "docker-compose.override.yml")

	files := []string{basePath}
	if strings.TrimSpace(override) != "" {
		if err := os.WriteFile(overridePath, []byte(override), 0o644); err != nil {
			return err
		}
		files = append(files, overridePath)
	} else {
		_ = os.Remove(overridePath)
	}
	return composecmd.Up(ctx, dir, id, files, envinject.Env(r.cfg, id))
}

// SetWebUI writes the opening-URL fields into the override's x-compose-app block
// and recreates the app. It is a friendly shortcut for editing those webui-*
// keys by hand: the rest of the override (service tweaks, etc.) is preserved, and
// empty fields are removed so they fall back to the base compose / x-casaos.
func (r *Registry) SetWebUI(ctx context.Context, id string, w WebUI) error {
	dir := filepath.Join(r.cfg.AppsDir(), id)
	basePath := filepath.Join(dir, "docker-compose.yml")
	if _, err := os.Stat(basePath); err != nil {
		return err // only managed apps have an editable config
	}
	overridePath := filepath.Join(dir, "docker-compose.override.yml")

	doc := map[string]any{}
	if raw, err := os.ReadFile(overridePath); err == nil {
		_ = yaml.Unmarshal(raw, &doc)
		if doc == nil {
			doc = map[string]any{}
		}
	}

	xca, _ := doc["x-compose-app"].(map[string]any)
	if xca == nil {
		xca = map[string]any{}
	}
	setOrDelete(xca, "webui-host", w.Host)
	setOrDelete(xca, "webui-port", w.Port)
	setOrDelete(xca, "webui-scheme", w.Scheme)
	setOrDelete(xca, "webui-path", w.Path)
	if len(xca) == 0 {
		delete(doc, "x-compose-app")
	} else {
		doc["x-compose-app"] = xca
	}

	files := []string{basePath}
	if len(doc) == 0 {
		_ = os.Remove(overridePath) // nothing left to override
	} else {
		out, err := yaml.Marshal(doc)
		if err != nil {
			return err
		}
		if err := os.WriteFile(overridePath, out, 0o644); err != nil {
			return err
		}
		files = append(files, overridePath)
	}
	return composecmd.Up(ctx, dir, id, files, envinject.Env(r.cfg, id))
}

// setOrDelete sets m[k]=v, or removes the key when v is blank.
func setOrDelete(m map[string]any, k, v string) {
	if strings.TrimSpace(v) == "" {
		delete(m, k)
		return
	}
	m[k] = v
}
