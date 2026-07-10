package xcomposeapp

import "testing"

func TestWebURL(t *testing.T) {
	cases := []struct {
		name   string
		app    App
		domain string
		want   string
	}{
		{"gateway default scheme+path", App{WebUIHost: "jellyfin-${domain}"}, "app.localhost", "https://jellyfin-app.localhost/"},
		{"gateway with path", App{WebUIHost: "jellyfin-${domain}", WebUIPath: "/web/"}, "app.localhost", "https://jellyfin-app.localhost/web/"},
		{"uppercase placeholder (caddy-label style)", App{WebUIHost: "nc-${DOMAIN}"}, "example.com", "https://nc-example.com/"},
		{"direct host+port+scheme", App{WebUIHost: "nas.example.com", WebUIScheme: "http", WebUIPort: "8096"}, "", "http://nas.example.com:8096/"},
		{"literal host no placeholder, empty domain ok", App{WebUIHost: "nas.local"}, "", "https://nas.local/"},
		{"path missing leading slash", App{WebUIHost: "a-${domain}", WebUIPath: "app"}, "d", "https://a-d/app"},
		{"query-string path", App{WebUIHost: "t-${domain}", WebUIPath: "/?hash=x"}, "d", "https://t-d/?hash=x"},
		{"no host -> no url", App{}, "app.localhost", ""},
		{"domain placeholder but no domain -> unreachable", App{WebUIHost: "j-${domain}"}, "", ""},
		{"unknown placeholder -> unreachable", App{WebUIHost: "j-${weird}"}, "app.localhost", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.app.WebURL(c.domain); got != c.want {
				t.Fatalf("WebURL(%q) = %q, want %q", c.domain, got, c.want)
			}
		})
	}
}

func TestParseVersionGate(t *testing.T) {
	if _, err := Parse(map[string]any{"schema_version": SchemaVersion + 1, "webui-host": "x"}); err != ErrUnsupportedVersion {
		t.Fatalf("future schema_version: got err %v, want ErrUnsupportedVersion", err)
	}
	a, err := Parse(map[string]any{"webui-host": "j-${domain}", "title": "Jellyfin"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if a.Title.Value() != "Jellyfin" {
		t.Fatalf("title = %q", a.Title.Value())
	}
	if _, err := Parse(nil); err != ErrNoExtension {
		t.Fatalf("nil: got %v, want ErrNoExtension", err)
	}
}
