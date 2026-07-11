package appstore

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// storeZip builds a minimal store archive: a top-level folder (as GitHub's
// codeload archives have) containing an Apps/ directory, which is what
// findAppsRoot looks for.
func storeZip(t *testing.T, appName string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("AppStore-main/Apps/" + appName + "/docker-compose.yml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("services: {}\n")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// storeServer serves a zip with an ETag, honours If-None-Match with a 304, and
// counts how many times it actually sent a body.
type storeServer struct {
	*httptest.Server
	etag  string
	body  []byte
	bodies int // times the full zip was transferred
	gets   int // total GETs (including 304s)
}

func newStoreServer(t *testing.T, etag string, body []byte) *storeServer {
	t.Helper()
	s := &storeServer{etag: etag, body: body}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.gets++
		w.Header().Set("ETag", s.etag)
		if r.Header.Get("If-None-Match") == s.etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		s.bodies++
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write(s.body)
	}))
	t.Cleanup(s.Close)
	return s
}

// An unchanged store must not be re-downloaded — the conditional GET comes back
// 304 — and that must survive a restart, which is where the reference CasaOS
// implementation re-downloads (it keeps the ETag in memory only).
func TestSyncStoreSkipsUnchangedDownload(t *testing.T) {
	srv := newStoreServer(t, `"v1"`, storeZip(t, "demo"))
	cache := t.TempDir()
	ctx := context.Background()

	m := New([]string{srv.URL}, cache)
	if err := m.Refresh(ctx); err != nil {
		t.Fatalf("first refresh: %v", err)
	}
	if srv.bodies != 1 {
		t.Fatalf("first refresh: got %d body transfers, want 1", srv.bodies)
	}

	// Same process, store unchanged: 304, no body.
	if err := m.Refresh(ctx); err != nil {
		t.Fatalf("second refresh: %v", err)
	}
	if srv.bodies != 1 {
		t.Fatalf("unchanged store re-downloaded: %d body transfers, want 1", srv.bodies)
	}

	// Restart: a brand-new Manager over the same cache dir must still get a 304,
	// because the validators were persisted to disk rather than held in memory.
	restarted := New([]string{srv.URL}, cache)
	if err := restarted.Refresh(ctx); err != nil {
		t.Fatalf("refresh after restart: %v", err)
	}
	if srv.bodies != 1 {
		t.Fatalf("restart re-downloaded the store: %d body transfers, want 1", srv.bodies)
	}
	if srv.gets != 3 {
		t.Fatalf("got %d GETs, want 3 (one per refresh)", srv.gets)
	}
}

// A store whose content actually changed must be re-downloaded and re-parsed.
func TestSyncStoreRefetchesWhenChanged(t *testing.T) {
	srv := newStoreServer(t, `"v1"`, storeZip(t, "demo"))
	cache := t.TempDir()
	ctx := context.Background()

	m := New([]string{srv.URL}, cache)
	if err := m.Refresh(ctx); err != nil {
		t.Fatalf("first refresh: %v", err)
	}

	srv.etag = `"v2"`
	srv.body = storeZip(t, "other")
	if err := m.Refresh(ctx); err != nil {
		t.Fatalf("refresh after change: %v", err)
	}
	if srv.bodies != 2 {
		t.Fatalf("changed store not re-downloaded: %d body transfers, want 2", srv.bodies)
	}
}

// RefreshStore is the user hitting "refresh": it must bypass the 304 and refetch
// even when the origin still considers the store unchanged.
func TestRefreshStoreForcesDownload(t *testing.T) {
	srv := newStoreServer(t, `"v1"`, storeZip(t, "demo"))
	cache := t.TempDir()
	ctx := context.Background()

	m := New([]string{srv.URL}, cache)
	if err := m.Refresh(ctx); err != nil {
		t.Fatalf("first refresh: %v", err)
	}
	if err := m.RefreshStore(ctx, srv.URL); err != nil {
		t.Fatalf("forced refresh: %v", err)
	}
	if srv.bodies != 2 {
		t.Fatalf("forced refresh served from cache: %d body transfers, want 2", srv.bodies)
	}
}

// An origin that sends no validators at all must still work: every refresh is an
// unconditional GET, and no stale validator file is left behind to poison it.
func TestSyncStoreWithoutValidators(t *testing.T) {
	body := storeZip(t, "demo")
	var gets int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gets++
		if r.Header.Get("If-None-Match") != "" || r.Header.Get("If-Modified-Since") != "" {
			t.Errorf("sent a conditional request with no validators on record")
		}
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	m := New([]string{srv.URL}, t.TempDir())
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		if err := m.Refresh(ctx); err != nil {
			t.Fatalf("refresh %d: %v", i, err)
		}
	}
	if gets != 2 {
		t.Fatalf("got %d GETs, want 2", gets)
	}
}
