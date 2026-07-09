// Package installer installs a store app: it applies CasaOS env/template rules,
// writes the compose project under the data root, and brings it up via
// `docker compose` (the compose plugin, invoked out-of-process). Pre/post
// install hooks from x-casaos run through /bin/bash, matching casa-img.
package installer

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/yundera/casadash/internal/appstore"
	"github.com/yundera/casadash/internal/composecmd"
	"github.com/yundera/casadash/internal/composefile"
	"github.com/yundera/casadash/internal/config"
	"github.com/yundera/casadash/internal/dockerx"
	"github.com/yundera/casadash/internal/envinject"
)

// Event is a progress update emitted during installation.
type Event struct {
	Phase   string  `json:"phase"`   // pull | prepare | start | done | error
	Message string  `json:"message"` // human-readable detail
	Percent float64 `json:"percent"` // overall 0-100
}

// pullBudget is the share of the progress bar allocated to image pulls (the
// slowest, most variable phase); the remainder covers prepare/start.
const pullBudget = 75.0

var invalidProjectChars = regexp.MustCompile(`[^a-z0-9_-]+`)

// projectName derives a Docker-compatible (lowercase) compose project name,
// preferring the compose file's own `name:` and falling back to a sanitized id.
func projectName(fileName, id string) string {
	name := fileName
	if name == "" {
		name = id
	}
	name = invalidProjectChars.ReplaceAllString(strings.ToLower(name), "-")
	name = strings.Trim(name, "-_")
	if name == "" {
		name = "app"
	}
	return name
}

// Installer installs store apps.
type Installer struct {
	cfg   config.Config
	store *appstore.Manager
	dx    *dockerx.Client // optional; used for image-pull progress
}

// New creates an Installer. dx may be nil (pull progress is then skipped).
func New(cfg config.Config, store *appstore.Manager, dx *dockerx.Client) *Installer {
	return &Installer{cfg: cfg, store: store, dx: dx}
}

// Install fetches app `id`, transforms its compose, writes it, and brings the
// stack up — emitting progress events (safe to pass a nil emit).
func (in *Installer) Install(ctx context.Context, id string, emit func(Event)) error {
	if emit == nil {
		emit = func(Event) {}
	}

	_, raw, err := in.store.Get(id)
	if err != nil {
		return err
	}

	f, err := composefile.Parse(raw)
	if err != nil {
		return fmt.Errorf("parse compose: %w", err)
	}
	si, _ := f.StoreInfo()
	main, pre, post := "", "", ""
	if si != nil {
		main, pre, post = si.Main, si.PreInstallCmd, si.PostInstallCmd
	}

	project := projectName(f.Name, id)

	transformed, err := envinject.Transform(raw, in.cfg, main)
	if err != nil {
		return fmt.Errorf("transform: %w", err)
	}

	appDir := filepath.Join(in.cfg.AppsDir(), project)
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return err
	}
	composePath := filepath.Join(appDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, transformed, 0o644); err != nil {
		return err
	}

	for _, dir := range envinject.VolumeDirs(raw, in.cfg) {
		if err := os.MkdirAll(dir, 0o755); err == nil {
			chownPUID(dir, in.cfg)
		}
	}

	// Pull images with real progress (0 → pullBudget%).
	in.pullImages(ctx, f, emit)

	env := envinject.Env(in.cfg, project)

	if pre != "" {
		emit(Event{Phase: "prepare", Message: "Running pre-install", Percent: 80})
		if err := runHook(ctx, envinject.RewriteToHostPath(pre, in.cfg), env, project); err != nil {
			return fmt.Errorf("pre-install: %w", err)
		}
	}

	emit(Event{Phase: "start", Message: "Starting containers", Percent: 90})
	if err := composecmd.Up(ctx, appDir, project, []string{composePath}, env); err != nil {
		return err
	}
	if post != "" {
		_ = runHook(ctx, envinject.RewriteToHostPath(post, in.cfg), env, project)
	}

	emit(Event{Phase: "done", Message: "Installed", Percent: 100})
	return nil
}

// pullImages pulls each service image, mapping per-image download progress onto
// the 0 → pullBudget band of the overall bar.
func (in *Installer) pullImages(ctx context.Context, f *composefile.File, emit func(Event)) {
	var images []string
	for _, svc := range f.Services {
		if svc.Image != "" {
			images = append(images, svc.Image)
		}
	}
	if len(images) == 0 || in.dx == nil {
		emit(Event{Phase: "pull", Message: "Preparing images", Percent: pullBudget})
		return
	}
	span := pullBudget / float64(len(images))
	for i, img := range images {
		base := float64(i) * span
		emit(Event{Phase: "pull", Message: "Pulling " + img, Percent: base})
		// Pull failures are non-fatal here — `compose up` will retry the pull.
		_ = in.dx.PullImage(ctx, img, func(pct float64, status string) {
			emit(Event{Phase: "pull", Message: img + ": " + status, Percent: base + pct/100*span})
		})
		emit(Event{Phase: "pull", Message: "Pulled " + img, Percent: base + span})
	}
}

// chownPUID sets ownership of a freshly-created app directory to PUID:PGID so
// the app (which usually drops privileges) can write to it.
func chownPUID(dir string, cfg config.Config) {
	uid, err1 := strconv.Atoi(cfg.PUID)
	gid, err2 := strconv.Atoi(cfg.PGID)
	if err1 == nil && err2 == nil {
		_ = os.Chown(dir, uid, gid)
	}
}

func runHook(ctx context.Context, script string, env []string, appID string) error {
	cmd := exec.CommandContext(ctx, "/bin/bash", "-c", script)
	cmd.Env = append(env,
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"DOCKER_HOST=unix:///var/run/docker.sock",
		"AppID="+appID,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, out)
	}
	return nil
}
