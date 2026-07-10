// Package installer installs a store app: it applies CasaOS env/template rules,
// writes the compose project under the data root, and brings it up via
// `docker compose` (the compose plugin, invoked out-of-process). Pre/post
// install hooks from x-casaos run through /bin/bash, matching casa-img.
package installer

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yundera/casadash/internal/appstore"
	"github.com/yundera/casadash/internal/composecmd"
	"github.com/yundera/casadash/internal/composefile"
	"github.com/yundera/casadash/internal/config"
	"github.com/yundera/casadash/internal/dockerx"
	"github.com/yundera/casadash/internal/envinject"
)

// Event is a progress update emitted during installation. Progress is split into
// two independent tracks the UI renders as two bars: Download (image pull) and
// Start (bringing the stack up in Docker).
type Event struct {
	Phase    string  `json:"phase"`    // pull | prepare | start | done | error
	Message  string  `json:"message"`  // human-readable detail
	Download float64 `json:"download"` // image-pull progress, 0-100
	Start    float64 `json:"start"`    // stack-start progress, 0-100
}

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

	// OnUpdate, if set, is called whenever a tracked install's progress changes
	// so the server can rebroadcast the app list (making the tile's progress bars
	// advance live). The server is expected to throttle it. Optional.
	OnUpdate func()

	mu       sync.Mutex
	installs map[string]*InstallState // project name -> live progress
}

// InstallState is a snapshot of one in-flight (or failed) install. It is overlaid
// onto the app list so the dashboard tile shows install progress even after the
// store panel is closed.
type InstallState struct {
	ID       string  `json:"id"`       // compose project name (== app tile id)
	Name     string  `json:"name"`     // display title, for the placeholder tile
	Icon     string  `json:"icon"`     // icon URL, for the placeholder tile
	Phase    string  `json:"phase"`    // pull | prepare | start | done | error
	Message  string  `json:"message"`  // human-readable detail
	Download float64 `json:"download"` // image-pull progress, 0-100
	Start    float64 `json:"start"`    // stack-start progress, 0-100
	Error    string  `json:"error"`    // set when Phase == error
}

// New creates an Installer. dx may be nil (pull progress is then skipped).
func New(cfg config.Config, store *appstore.Manager, dx *dockerx.Client) *Installer {
	return &Installer{cfg: cfg, store: store, dx: dx, installs: map[string]*InstallState{}}
}

// StartInstall launches a detached install of store app `id` and tracks its
// progress so it rides the live app list. The install runs on a background
// context, so it is NOT cancelled when the caller (e.g. the store panel) goes
// away. Idempotent: a second call while the same project is installing is a
// no-op. Returns the resolved compose project name (the app's tile id).
func (in *Installer) StartInstall(id string) (string, error) {
	app, raw, err := in.store.Get(id)
	if err != nil {
		return "", err
	}
	f, err := composefile.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse compose: %w", err)
	}
	project := projectName(f.Name, id)

	in.mu.Lock()
	if _, running := in.installs[project]; running {
		in.mu.Unlock()
		return project, nil // already installing — attach, don't restart
	}
	name := app.Name
	if name == "" {
		name = id
	}
	in.installs[project] = &InstallState{ID: project, Name: name, Icon: app.Icon, Phase: "pull", Message: "Queued"}
	in.mu.Unlock()
	in.notify()

	go func() {
		err := in.Install(context.Background(), id, func(ev Event) {
			in.mu.Lock()
			if st := in.installs[project]; st != nil {
				st.Phase, st.Message, st.Download, st.Start = ev.Phase, ev.Message, ev.Download, ev.Start
			}
			in.mu.Unlock()
			in.notify()
		})
		in.mu.Lock()
		if err != nil {
			log.Printf("install %s failed: %v", project, err)
			// Keep the entry so the failure stays visible on the tile until the
			// user retries (which clears it) or dismisses it.
			if st := in.installs[project]; st != nil {
				st.Phase, st.Error, st.Message = "error", err.Error(), err.Error()
			}
		} else {
			// Success: drop the overlay so the real, Docker-backed tile takes over.
			delete(in.installs, project)
		}
		in.mu.Unlock()
		in.notify()
	}()
	return project, nil
}

// Installs returns a snapshot of every tracked install (in-flight or errored).
func (in *Installer) Installs() []InstallState {
	in.mu.Lock()
	defer in.mu.Unlock()
	out := make([]InstallState, 0, len(in.installs))
	for _, st := range in.installs {
		out = append(out, *st)
	}
	return out
}

// ClearInstall drops a tracked install (used to dismiss a failed one).
func (in *Installer) ClearInstall(project string) {
	in.mu.Lock()
	_, existed := in.installs[project]
	delete(in.installs, project)
	in.mu.Unlock()
	if existed {
		in.notify()
	}
}

func (in *Installer) notify() {
	if in.OnUpdate != nil {
		in.OnUpdate()
	}
}

// Install fetches app `id`, transforms its compose, writes it, and brings the
// stack up — emitting progress events (safe to pass a nil emit).
func (in *Installer) Install(ctx context.Context, id string, emit func(Event)) error {
	if emit == nil {
		emit = func(Event) {}
	}

	app, raw, err := in.store.Get(id)
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

	// Prefill the app's .env so its compose resolves offline and the operator can
	// hand-edit variables afterwards (see docs/app-model.md). Never clobber an
	// existing .env — a reinstall over a restored archive keeps the user's edits.
	envPath := filepath.Join(appDir, ".env")
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		if err := os.WriteFile(envPath, envinject.EnvFile(in.cfg, project), 0o644); err != nil {
			return err
		}
	}

	// Record where this app came from so the per-app Update tab can pull a fresher
	// docker-compose.yml from the same store later. The reference lives in the
	// override's x-compose-app block (see docs/app-model.md and update.go).
	if err := writeUpdateRef(appDir, app.StoreURL, id); err != nil {
		return err
	}

	for _, dir := range envinject.VolumeDirs(raw, in.cfg) {
		if err := os.MkdirAll(dir, 0o755); err == nil {
			chownPUID(dir, in.cfg)
		}
	}

	// Track 1 — Download: pull images with real progress (0 → 100%).
	in.pullImages(ctx, f, emit)

	env := envinject.Env(in.cfg, project)

	if pre != "" {
		emit(Event{Phase: "prepare", Message: "Running pre-install", Download: 100})
		if err := runHook(ctx, envinject.RewriteToHostPath(pre, in.cfg), env, project); err != nil {
			return fmt.Errorf("pre-install: %w", err)
		}
	}

	// Track 2 — Start: bring the stack up, then follow Docker until it is running.
	emit(Event{Phase: "start", Message: "Starting containers", Download: 100, Start: 15})
	files := []string{composePath}
	if override := filepath.Join(appDir, "docker-compose.override.yml"); fileExists(override) {
		files = append(files, override)
	}
	if err := composecmd.Up(ctx, appDir, project, files, env); err != nil {
		return err
	}
	if post != "" {
		_ = runHook(ctx, envinject.RewriteToHostPath(post, in.cfg), env, project)
	}
	// `compose up -d` returns once containers are created/started; follow their
	// live Docker state (and health checks) so the Start bar reflects real
	// readiness rather than jumping straight to 100%.
	in.awaitStart(ctx, project, emit)

	emit(Event{Phase: "done", Message: "Installed", Download: 100, Start: 100})
	return nil
}

// pullImages pulls each service image, mapping per-image download progress onto
// the 0 → 100 Download bar.
func (in *Installer) pullImages(ctx context.Context, f *composefile.File, emit func(Event)) {
	var images []string
	for _, svc := range f.Services {
		if svc.Image != "" {
			images = append(images, svc.Image)
		}
	}
	if len(images) == 0 || in.dx == nil {
		emit(Event{Phase: "pull", Message: "Preparing images", Download: 100})
		return
	}
	span := 100.0 / float64(len(images))
	for i, img := range images {
		base := float64(i) * span
		emit(Event{Phase: "pull", Message: "Pulling " + img, Download: base})
		// Pull failures are non-fatal here — `compose up` will retry the pull.
		_ = in.dx.PullImage(ctx, img, func(pct float64, status string) {
			emit(Event{Phase: "pull", Message: img + ": " + status, Download: base + pct/100*span})
		})
		emit(Event{Phase: "pull", Message: "Pulled " + img, Download: base + span})
	}
}

// awaitStart polls the project's containers for up to ~30s after `compose up`,
// advancing the Start bar from Docker's live state: the running fraction (and,
// when containers declare health checks, the healthy fraction) drive it toward
// 100%. It returns early once every container is running and healthy. dx may be
// nil (no daemon) — the caller then just reports the stack as fully started.
func (in *Installer) awaitStart(ctx context.Context, project string, emit func(Event)) {
	if in.dx == nil {
		return
	}
	for i := 0; i < 30; i++ {
		svcs, err := in.dx.ProjectServices(ctx, project)
		if err == nil && len(svcs) > 0 {
			total := len(svcs)
			running, withHealth, healthy := 0, 0, 0
			for _, s := range svcs {
				if s.State == "running" {
					running++
				}
				if s.Health != "" {
					withHealth++
					if s.Health == "healthy" {
						healthy++
					}
				}
			}
			runFrac := float64(running) / float64(total)
			// 15 (up invoked) → 100. When health checks exist, blend running and
			// healthy fractions so the bar keeps moving through the "starting" wait.
			frac := runFrac
			if withHealth > 0 {
				frac = runFrac*0.5 + (float64(healthy)/float64(withHealth))*0.5
			}
			emit(Event{Phase: "start", Message: "Starting containers", Download: 100, Start: 15 + frac*85})
			if running == total && (withHealth == 0 || healthy == withHealth) {
				return
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
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

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
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
