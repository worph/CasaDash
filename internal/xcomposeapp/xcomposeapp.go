// Package xcomposeapp models CasaDash's own `x-compose-app` compose extension and
// resolves an app's web-UI click URL from it.
//
// Unlike x-casaos (which declares a container port and derives a hostname at
// install time), x-compose-app declares the final web-UI URL directly — the
// `webui-host` value is the app's reverse-proxy route host, e.g. `app-${domain}`.
// The URL is built by string construction on every render, so it tracks domain
// changes and works for apps CasaDash merely discovered.
package xcomposeapp

import (
	"errors"
	"strings"

	"gopkg.in/yaml.v3"
)

// ExtensionKey is the compose extension key this package reads.
const ExtensionKey = "x-compose-app"

var (
	// ErrNoExtension is returned when a compose file has no x-compose-app block.
	ErrNoExtension = errors.New("x-compose-app extension not found")
	// ErrUnsupportedVersion is returned for a schema_version this build predates,
	// so the caller falls back to x-casaos.
	ErrUnsupportedVersion = errors.New("x-compose-app schema_version not supported")
)

// SchemaVersion is the highest x-compose-app schema this build understands.
const SchemaVersion = 1

// App is the CasaDash-native app metadata. Only the fields CasaDash consumes are
// modelled; unknown keys are ignored.
type App struct {
	Schema        int       `yaml:"schema_version,omitempty"`
	ID            string    `yaml:"id,omitempty"`
	Title         Localized `yaml:"title,omitempty"`
	Icon          string    `yaml:"icon,omitempty"`
	Category      string    `yaml:"category,omitempty"`
	Tagline       Localized `yaml:"tagline,omitempty"`
	Description   Localized `yaml:"description,omitempty"`
	Developer     string    `yaml:"developer,omitempty"`
	Screenshots   []string  `yaml:"screenshots,omitempty"`
	Thumbnail     string    `yaml:"thumbnail,omitempty"`
	Architectures []string  `yaml:"architectures,omitempty"`

	// The click URL, declared directly (see package doc).
	WebUIHost   string `yaml:"webui-host,omitempty"`
	WebUIPort   string `yaml:"webui-port,omitempty"`
	WebUIScheme string `yaml:"webui-scheme,omitempty"`
	WebUIPath   string `yaml:"webui-path,omitempty"`

	// Update reference: where this app was installed from, so CasaDash can pull a
	// fresher docker-compose.yml from the same store and re-apply it. Written into
	// the override's x-compose-app block at install time (see installer). Store is
	// the reference store URL; StoreAppID is the catalog id within that store.
	Store      string `yaml:"store,omitempty"`
	StoreAppID string `yaml:"store-app-id,omitempty"`

	Links []Link `yaml:"links,omitempty"`
}

// Link is an extra button on the app detail view (absolute URL only).
type Link struct {
	Name string `yaml:"name,omitempty"`
	URL  string `yaml:"url,omitempty"`
	Icon string `yaml:"icon,omitempty"`
}

// Localized is a value that may be written as a bare string or a locale map.
type Localized map[string]string

// UnmarshalYAML accepts either a scalar ("Jellyfin") or a map ({en_us: Jellyfin}).
func (l *Localized) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		*l = Localized{"en_us": n.Value}
		return nil
	}
	var m map[string]string
	if err := n.Decode(&m); err != nil {
		return err
	}
	*l = m
	return nil
}

// Value returns the en_us entry if present, else any entry, else "".
func (l Localized) Value() string {
	if l == nil {
		return ""
	}
	if v, ok := l["en_us"]; ok {
		return v
	}
	for _, v := range l {
		return v
	}
	return ""
}

// Parse decodes an x-compose-app extension map into an App. It returns
// ErrNoExtension when the block is absent and ErrUnsupportedVersion for a
// schema_version newer than this build — both signal the caller to fall back to
// x-casaos.
func Parse(ext map[string]any) (*App, error) {
	if ext == nil {
		return nil, ErrNoExtension
	}
	b, err := yaml.Marshal(ext)
	if err != nil {
		return nil, err
	}
	var a App
	if err := yaml.Unmarshal(b, &a); err != nil {
		return nil, err
	}
	if a.Schema != 0 && a.Schema > SchemaVersion {
		return nil, ErrUnsupportedVersion
	}
	return &a, nil
}

// WebURL builds the click URL from the webui-* fields, resolving host
// placeholders against domain (the deployment's REF_DOMAIN). It returns "" when
// there is no host or the host cannot be resolved — the tile then shows the
// "no reachable address" hint instead of a broken link.
func (a *App) WebURL(domain string) string {
	host := resolveHost(a.WebUIHost, domain)
	if host == "" {
		return ""
	}
	scheme := strings.TrimSpace(a.WebUIScheme)
	if scheme == "" {
		scheme = "https"
	}
	port := ""
	if p := strings.TrimSpace(a.WebUIPort); p != "" {
		port = ":" + p
	}
	path := a.WebUIPath
	if path == "" {
		path = "/"
	} else if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return scheme + "://" + host + port + path
}

// resolveHost substitutes deployment placeholders in a webui-host template.
// ${domain}/${DOMAIN} → domain. If the template references a domain that isn't
// configured, or any placeholder is left unresolved, it returns "" so the URL is
// reported as unreachable rather than built broken.
func resolveHost(host, domain string) string {
	h := strings.TrimSpace(host)
	if h == "" {
		return ""
	}
	if strings.Contains(h, "${domain}") || strings.Contains(h, "${DOMAIN}") {
		if domain == "" {
			return ""
		}
		h = strings.ReplaceAll(h, "${domain}", domain)
		h = strings.ReplaceAll(h, "${DOMAIN}", domain)
	}
	if strings.Contains(h, "${") {
		return "" // an unresolved placeholder we don't understand
	}
	return h
}
