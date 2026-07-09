package apps

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/yundera/casadash/internal/composecmd"
	"github.com/yundera/casadash/internal/envinject"
)

// Config carries a managed app's base compose and its user override.
type Config struct {
	Base     string `json:"base"`
	Override string `json:"override"`
}

// GetConfig reads a managed app's compose + override files.
func (r *Registry) GetConfig(id string) (*Config, error) {
	dir := filepath.Join(r.cfg.AppsDir(), id)
	base, err := os.ReadFile(filepath.Join(dir, "docker-compose.yml"))
	if err != nil {
		return nil, err
	}
	override, _ := os.ReadFile(filepath.Join(dir, "docker-compose.override.yml"))
	return &Config{Base: string(base), Override: string(override)}, nil
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
