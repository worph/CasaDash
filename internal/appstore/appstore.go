// Package appstore fetches CasaOS-compatible app stores (GitHub zip archives),
// extracts them, and builds a merged catalog of installable apps keyed by app
// id. Layout ported from casa-img: <root>/Apps/<name>/docker-compose.yml plus
// category-list.json and recommend-list.json.
package appstore

import (
	"archive/zip"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yundera/casadash/internal/composefile"
	"github.com/yundera/casadash/internal/xcasaos"
)

// CatalogApp is one installable app from a store.
type CatalogApp struct {
	ID          string   `json:"id"`   // compose project name (Apps/<name>)
	Name        string   `json:"name"` // display title
	Tagline     string   `json:"tagline"`
	Description  string   `json:"description"`
	Icon        string   `json:"icon"`
	Thumbnail   string   `json:"thumbnail"`
	Screenshots []string `json:"screenshots"`
	Category    string   `json:"category"`
	Developer   string   `json:"developer"`
	Author      string   `json:"author"`
	MinMemory   int      `json:"min_memory,omitempty"`
	StoreURL    string   `json:"store"`

	composePath string // absolute path to the app's compose file
}

// Manager holds the merged catalog across all configured stores.
type Manager struct {
	urls     []string
	cacheDir string

	mu        sync.RWMutex
	catalog   map[string]*CatalogApp
	order     []string // stable catalog order
	cats      []string
	recommend []string
	lastETag  map[string]string
}

// New creates a Manager for the given store URLs, caching under cacheDir.
func New(urls []string, cacheDir string) *Manager {
	return &Manager{
		urls:     urls,
		cacheDir: cacheDir,
		catalog:  map[string]*CatalogApp{},
		lastETag: map[string]string{},
	}
}

// URLs returns the configured store URLs.
func (m *Manager) URLs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]string(nil), m.urls...)
}

// SetURLs replaces the store URL list (caller should Refresh afterwards).
func (m *Manager) SetURLs(urls []string) {
	m.mu.Lock()
	m.urls = append([]string(nil), urls...)
	m.mu.Unlock()
}

// Catalog returns all apps sorted by name.
func (m *Manager) Catalog() []*CatalogApp {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*CatalogApp, 0, len(m.order))
	for _, id := range m.order {
		out = append(out, m.catalog[id])
	}
	return out
}

// Categories returns the distinct category list.
func (m *Manager) Categories() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]string(nil), m.cats...)
}

// Recommend returns featured app ids.
func (m *Manager) Recommend() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]string(nil), m.recommend...)
}

// Get returns an app by id and its raw compose bytes.
func (m *Manager) Get(id string) (*CatalogApp, []byte, error) {
	m.mu.RLock()
	app := m.catalog[id]
	m.mu.RUnlock()
	if app == nil {
		return nil, nil, fmt.Errorf("app %q not found", id)
	}
	raw, err := os.ReadFile(app.composePath)
	if err != nil {
		return nil, nil, err
	}
	return app, raw, nil
}

// Refresh downloads and reparses every configured store.
func (m *Manager) Refresh(ctx context.Context) error {
	catalog := map[string]*CatalogApp{}
	var order []string
	catSet := map[string]bool{}
	var recommend []string

	for _, u := range m.URLs() {
		root, err := m.syncStore(ctx, u)
		if err != nil {
			// One bad store shouldn't sink the rest.
			continue
		}
		apps, cats, rec := parseStore(root, u)
		for _, a := range apps {
			if _, exists := catalog[a.ID]; exists {
				continue // first store wins on id collision
			}
			catalog[a.ID] = a
			order = append(order, a.ID)
		}
		for _, c := range cats {
			catSet[c] = true
		}
		recommend = append(recommend, rec...)
	}

	sort.Slice(order, func(i, j int) bool {
		return strings.ToLower(catalog[order[i]].Name) < strings.ToLower(catalog[order[j]].Name)
	})
	cats := make([]string, 0, len(catSet))
	for c := range catSet {
		cats = append(cats, c)
	}
	sort.Strings(cats)

	m.mu.Lock()
	m.catalog = catalog
	m.order = order
	m.cats = cats
	m.recommend = recommend
	m.mu.Unlock()
	return nil
}

// StartAutoRefresh refreshes now and then every interval until ctx is done.
func (m *Manager) StartAutoRefresh(ctx context.Context, interval time.Duration) {
	go func() {
		_ = m.Refresh(ctx)
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				_ = m.Refresh(ctx)
			}
		}
	}()
}

// syncStore downloads+extracts a store zip (skipping if the ETag is unchanged)
// and returns the store root directory (the parent of the Apps/ folder).
func (m *Manager) syncStore(ctx context.Context, storeURL string) (string, error) {
	workdir := m.workdir(storeURL)

	if etag, err := headETag(ctx, storeURL); err == nil && etag != "" {
		m.mu.RLock()
		prev := m.lastETag[storeURL]
		m.mu.RUnlock()
		if prev == etag {
			if root, err := findAppsRoot(workdir); err == nil {
				return root, nil
			}
		}
		defer func() {
			m.mu.Lock()
			m.lastETag[storeURL] = etag
			m.mu.Unlock()
		}()
	}

	if err := download(ctx, storeURL, workdir); err != nil {
		// Fall back to any previously extracted copy.
		if root, ferr := findAppsRoot(workdir); ferr == nil {
			return root, nil
		}
		return "", err
	}
	return findAppsRoot(workdir)
}

func (m *Manager) workdir(storeURL string) string {
	u, err := url.Parse(storeURL)
	if err != nil {
		sum := md5.Sum([]byte(storeURL))
		return filepath.Join(m.cacheDir, hex.EncodeToString(sum[:]))
	}
	sum := md5.Sum([]byte(strings.ToLower(u.Path)))
	return filepath.Join(m.cacheDir, u.Host, hex.EncodeToString(sum[:]))
}

func headETag(ctx context.Context, u string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, u, nil)
	if err != nil {
		return "", err
	}
	c := &http.Client{Timeout: 10 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	return resp.Header.Get("ETag"), nil
}

// download fetches a zip and extracts it into dest (replacing any prior copy).
func download(ctx context.Context, u, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("store %s: http %d", u, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	zr, err := zip.NewReader(strings.NewReader(string(body)), int64(len(body)))
	if err != nil {
		return err
	}

	tmp := dest + ".tmp"
	_ = os.RemoveAll(tmp)
	if err := extractZip(zr, tmp); err != nil {
		return err
	}
	_ = os.RemoveAll(dest)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	return os.Rename(tmp, dest)
}

func extractZip(zr *zip.Reader, dest string) error {
	for _, f := range zr.File {
		target := filepath.Join(dest, f.Name) //nolint:gosec // sanitized below
		if !strings.HasPrefix(target, filepath.Clean(dest)+string(os.PathSeparator)) {
			continue // zip-slip guard
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(out, rc) //nolint:gosec // store content, size-bounded by GitHub
		out.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// findAppsRoot locates the directory containing an "Apps" subfolder.
func findAppsRoot(dir string) (string, error) {
	var found string
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || found != "" {
			return nil
		}
		if d.IsDir() && d.Name() == "Apps" {
			found = filepath.Dir(path)
		}
		return nil
	})
	if found == "" {
		return "", fmt.Errorf("no Apps/ directory under %s", dir)
	}
	return found, nil
}

func parseStore(root, storeURL string) (apps []*CatalogApp, cats, recommend []string) {
	appsDir := filepath.Join(root, "Apps")
	entries, err := os.ReadDir(appsDir)
	if err != nil {
		return nil, nil, nil
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		composePath := filepath.Join(appsDir, e.Name(), "docker-compose.yml")
		if _, err := os.Stat(composePath); err != nil {
			composePath = filepath.Join(appsDir, e.Name(), "docker-compose.yaml")
			if _, err := os.Stat(composePath); err != nil {
				continue
			}
		}
		f, err := composefile.Load(composePath)
		if err != nil {
			continue
		}
		si, err := f.StoreInfo()
		if err != nil {
			continue
		}
		apps = append(apps, catalogApp(e.Name(), si, composePath, storeURL))
	}

	cats = readCategories(root)
	recommend = readRecommend(root)
	return apps, cats, recommend
}

func catalogApp(id string, si *xcasaos.StoreInfo, composePath, storeURL string) *CatalogApp {
	name := xcasaos.Localized(si.Title)
	if name == "" {
		name = id
	}
	return &CatalogApp{
		ID:          id,
		Name:        name,
		Tagline:     xcasaos.Localized(si.Tagline),
		Description: xcasaos.Localized(si.Description),
		Icon:        si.Icon,
		Thumbnail:   si.Thumbnail,
		Screenshots: si.ScreenshotLink,
		Category:    si.Category,
		Developer:   si.Developer,
		Author:      si.Author,
		MinMemory:   si.MinMemory,
		StoreURL:    storeURL,
		composePath: composePath,
	}
}

func readCategories(root string) []string {
	b, err := os.ReadFile(filepath.Join(root, "category-list.json"))
	if err != nil {
		return nil
	}
	var raw []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, c := range raw {
		if c.Name != "" {
			out = append(out, c.Name)
		}
	}
	return out
}

func readRecommend(root string) []string {
	b, err := os.ReadFile(filepath.Join(root, "recommend-list.json"))
	if err != nil {
		return nil
	}
	var raw []struct {
		AppID string `json:"appid"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, r := range raw {
		if r.AppID != "" {
			out = append(out, r.AppID)
		}
	}
	return out
}
