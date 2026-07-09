// Package apps builds the dashboard's view of installed applications by
// reconciling CasaDash-managed compose projects (on disk) with what is actually
// running in Docker, and surfacing externally-created x-casaos stacks as
// "unmanaged" apps.
package apps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/yundera/casadash/internal/composefile"
	"github.com/yundera/casadash/internal/config"
	"github.com/yundera/casadash/internal/dockerx"
	"github.com/yundera/casadash/internal/xcasaos"
)

// Status values for an app tile.
const (
	StatusRunning = "running"
	StatusStopped = "stopped"
	StatusPartial = "partial"
)

// App is one dashboard tile.
type App struct {
	ID       string `json:"id"`      // compose project name
	Name     string `json:"name"`    // display title
	Icon     string `json:"icon"`    // icon URL
	Status   string `json:"status"`  // running|stopped|partial
	Managed  bool   `json:"managed"` // installed by CasaDash
	Store    string `json:"store,omitempty"`
	Scheme   string `json:"scheme,omitempty"`
	Hostname string `json:"hostname,omitempty"`
	Port     string `json:"port,omitempty"`
	Index    string `json:"index,omitempty"`
	Category string `json:"category,omitempty"`
}

// Registry reconciles on-disk projects with Docker state.
type Registry struct {
	cfg config.Config
	dx  *dockerx.Client
}

// New creates a Registry.
func New(cfg config.Config, dx *dockerx.Client) *Registry {
	return &Registry{cfg: cfg, dx: dx}
}

type projectState struct {
	workingDir string
	running    int
	total      int
	svcPorts   map[string][]dockerx.Port // service name -> published ports
}

// List returns all app tiles, sorted by display name.
func (r *Registry) List(ctx context.Context) ([]App, error) {
	conts, err := r.dx.ListProjectContainers(ctx)
	if err != nil {
		return nil, err
	}

	projects := map[string]*projectState{}
	for _, c := range conts {
		ps := projects[c.Project]
		if ps == nil {
			ps = &projectState{workingDir: c.WorkingDir, svcPorts: map[string][]dockerx.Port{}}
			projects[c.Project] = ps
		}
		ps.total++
		if c.State == "running" {
			ps.running++
		}
		if len(c.Ports) > 0 {
			ps.svcPorts[c.Service] = c.Ports
		}
	}

	seen := map[string]bool{}
	var out []App

	for name, ps := range projects {
		managed := r.isManaged(name)
		si := r.storeInfoFor(name, ps.workingDir)
		if si == nil && !managed {
			// A non-CasaDash stack without x-casaos metadata: not our concern.
			continue
		}
		out = append(out, buildApp(name, si, managed, statusOf(ps.running, ps.total), ps.svcPorts))
		seen[name] = true
	}

	// Managed projects that exist on disk but have no containers (installed but down).
	for _, name := range r.managedDirs() {
		if seen[name] {
			continue
		}
		si := r.storeInfoFor(name, "")
		out = append(out, buildApp(name, si, true, StatusStopped, nil))
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (r *Registry) isManaged(project string) bool {
	_, err := os.Stat(filepath.Join(r.cfg.AppsDir(), project, "docker-compose.yml"))
	return err == nil
}

func (r *Registry) managedDirs() []string {
	entries, err := os.ReadDir(r.cfg.AppsDir())
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() && r.isManaged(e.Name()) {
			names = append(names, e.Name())
		}
	}
	return names
}

// storeInfoFor loads x-casaos for a project, preferring the CasaDash-managed
// compose file, then the working dir reported by Docker.
func (r *Registry) storeInfoFor(project, workingDir string) *xcasaos.StoreInfo {
	candidates := []string{
		filepath.Join(r.cfg.AppsDir(), project, "docker-compose.yml"),
	}
	if workingDir != "" {
		candidates = append(candidates,
			filepath.Join(workingDir, "docker-compose.yml"),
			filepath.Join(workingDir, "docker-compose.yaml"),
		)
	}
	for _, path := range candidates {
		f, err := composefile.Load(path)
		if err != nil {
			continue
		}
		if si, err := f.StoreInfo(); err == nil {
			return si
		}
	}
	return nil
}

func statusOf(running, total int) string {
	switch {
	case total == 0 || running == 0:
		return StatusStopped
	case running == total:
		return StatusRunning
	default:
		return StatusPartial
	}
}

func buildApp(name string, si *xcasaos.StoreInfo, managed bool, status string, svcPorts map[string][]dockerx.Port) App {
	app := App{ID: name, Name: name, Managed: managed, Status: status}
	if si != nil {
		if t := xcasaos.Localized(si.Title); t != "" {
			app.Name = t
		}
		app.Icon = si.Icon
		app.Scheme = si.Scheme
		app.Hostname = si.Hostname
		app.Port = si.PortMap
		app.Index = si.Index
		app.Category = si.Category
		app.Store = si.StoreAppID
	}
	// Prefer the container's ACTUAL published host port so "Open" works without a
	// gateway. Only when no hostname (gateway route) is configured.
	if app.Hostname == "" && svcPorts != nil {
		main := ""
		if si != nil {
			main = si.Main
		}
		webui := 0
		if si != nil {
			webui, _ = strconv.Atoi(si.WebUIPort)
		}
		if hp := reachableHostPort(svcPorts, main, webui); hp > 0 {
			app.Port = strconv.Itoa(hp)
		}
	}
	return app
}

// reachableHostPort picks a published host port to open: the one bound to the
// web-UI port on the main service if present, otherwise the first published port.
func reachableHostPort(svcPorts map[string][]dockerx.Port, main string, webui int) int {
	try := func(ports []dockerx.Port) int {
		if webui > 0 {
			for _, p := range ports {
				if int(p.Private) == webui && p.Public > 0 {
					return int(p.Public)
				}
			}
		}
		for _, p := range ports {
			if p.Public > 0 {
				return int(p.Public)
			}
		}
		return 0
	}
	if main != "" {
		if hp := try(svcPorts[main]); hp > 0 {
			return hp
		}
	}
	for _, ports := range svcPorts {
		if hp := try(ports); hp > 0 {
			return hp
		}
	}
	return 0
}

// Start/Stop/Restart/Uninstall proxy to Docker and (for uninstall) remove the
// on-disk project.
func (r *Registry) Start(ctx context.Context, id string) error {
	return r.dx.StartProject(ctx, id)
}

func (r *Registry) Stop(ctx context.Context, id string) error {
	return r.dx.StopProject(ctx, id)
}

func (r *Registry) Restart(ctx context.Context, id string) error {
	return r.dx.RestartProject(ctx, id)
}

// Uninstall stops+removes the project's containers and deletes the managed
// compose project directory. App data under ${DATA_ROOT}/AppData/<id> is
// preserved unless archiveData is set, in which case it is first zipped to a
// timestamped archive alongside it and then removed (matching CasaOS).
func (r *Registry) Uninstall(ctx context.Context, id string, archiveData bool) (string, error) {
	_ = r.dx.RemoveProject(ctx, id)
	if r.isManaged(id) {
		_ = os.RemoveAll(filepath.Join(r.cfg.AppsDir(), id))
	}

	dataDir := filepath.Join(r.cfg.DataRoot, "AppData", id)
	if _, err := os.Stat(dataDir); err != nil {
		return "", nil // no app data
	}
	if !archiveData {
		return "", nil // keep app data untouched
	}

	stamp := time.Now().Format("20060102-150405")
	zipName := id + "_" + stamp + ".zip"
	zipPath := filepath.Join(r.cfg.DataRoot, "AppData", zipName)
	if err := archiveDir(dataDir, zipPath); err != nil {
		return "", fmt.Errorf("archive app data: %w", err)
	}
	if err := os.RemoveAll(dataDir); err != nil {
		return zipName, fmt.Errorf("remove app data: %w", err)
	}
	return zipName, nil
}
