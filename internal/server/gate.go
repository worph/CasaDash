package server

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// The launch gate ("intermediate loading page").
//
// When an app is down, the mesh-router-caddy catch-all reverse-proxies its
// gateway host (`<app>-<domain>`) to CasaDash, preserving the original Host. We
// detect that we are standing in for an app host (rather than serving our own
// dashboard) and return a small, self-contained page that:
//
//  1. asks CasaDash which app this host is (`/__casadash/start`, `/__casadash/whoami`),
//  2. starts the app,
//  3. polls the app root and — because the page is served on the app's OWN
//     origin — reads the real status code same-origin (no CORS). It waits for a
//     genuine 2xx/3xx from the app, telling that apart from CasaDash's own
//     catch-all response via the `X-Casadash` marker header,
//  4. reloads into the now-live app.
//
// Apps that are already up never hit this path: caddy routes their host straight
// to the app and CasaDash is not involved.

// isDashboardHost reports whether host addresses CasaDash's own dashboard
// (rather than an app gateway host it is catching for). With no gateway domain
// configured, every request is treated as the dashboard (the gate is inert).
func (s *Server) isDashboardHost(hostport string) bool {
	if s.cfg.AppDomain() == "" {
		return true
	}
	host := hostport
	if i := strings.IndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	host = strings.ToLower(host)
	switch host {
	case "", "localhost", "127.0.0.1",
		s.cfg.AppDomain(),
		"casadash-" + s.cfg.AppDomain(),
		"casaos-" + s.cfg.AppDomain():
		return true
	}
	return false
}

// gateRouter handles requests that arrive on an app gateway host while the app
// is down (i.e. via the catch-all). Everything except the two control endpoints
// renders the loading page.
func (s *Server) gateRouter() http.Handler {
	m := chi.NewRouter()
	m.Get("/__casadash/whoami", s.gateWhoami)
	m.Post("/__casadash/start", s.gateStart)
	m.Handle("/*", http.HandlerFunc(s.gatePage))
	return m
}

func (s *Server) gateWhoami(w http.ResponseWriter, r *http.Request) {
	app, ok := s.apps.FindByHost(r.Context(), r.Host, s.cfg.AppDomain())
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown app host"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"id":     app.ID,
		"name":   app.Name,
		"icon":   app.Icon,
		"status": app.Status,
	})
}

func (s *Server) gateStart(w http.ResponseWriter, r *http.Request) {
	app, ok := s.apps.FindByHost(r.Context(), r.Host, s.cfg.AppDomain())
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown app host"})
		return
	}
	if err := s.apps.EnsureStarted(r.Context(), app.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.broadcastApps()
	writeJSON(w, http.StatusOK, map[string]string{"status": "starting", "id": app.ID})
}

func (s *Server) gatePage(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(gateHTML))
}

// gateHTML is the self-contained loading page. It carries no external assets so
// it keeps working while the app host flips from CasaDash (catch-all) to the
// real app under it. The readiness rule is strict: only a real 2xx/3xx from the
// app (no X-Casadash marker) counts as ready.
const gateHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Starting…</title>
<style>
  :root { color-scheme: light dark; }
  * { box-sizing: border-box; }
  body {
    margin: 0; min-height: 100vh; display: grid; place-items: center;
    font-family: system-ui, -apple-system, Segoe UI, Roboto, sans-serif;
    background: #0f1115; color: #e8ebf0;
  }
  @media (prefers-color-scheme: light) { body { background: #f3f5f8; color: #1b2330; } }
  .card { text-align: center; padding: 2rem; max-width: 22rem; }
  .icon {
    width: 84px; height: 84px; margin: 0 auto 1.25rem; border-radius: 20px;
    display: grid; place-items: center; overflow: hidden;
    background: #2f6df6; color: #fff; font-size: 2.4rem; font-weight: 600;
  }
  .icon img { width: 100%; height: 100%; object-fit: cover; }
  h1 { font-size: 1.15rem; margin: 0 0 0.35rem; font-weight: 600; }
  .status { font-size: 0.9rem; opacity: 0.7; min-height: 1.2em; }
  .spinner {
    width: 34px; height: 34px; margin: 1.5rem auto 0;
    border: 3px solid rgba(128,128,128,0.25); border-top-color: #2f6df6;
    border-radius: 50%; animation: spin 0.8s linear infinite;
  }
  @keyframes spin { to { transform: rotate(360deg); } }
  .late { margin-top: 1.5rem; font-size: 0.85rem; opacity: 0; transition: opacity 0.3s; }
  .late.show { opacity: 1; }
  .late a { color: #2f6df6; cursor: pointer; }
  .err { color: #e5484d; }
</style>
</head>
<body>
  <div class="card">
    <div class="icon" id="icon"><span id="letter">·</span></div>
    <h1 id="name">Starting your app…</h1>
    <div class="status" id="status">Waking it up…</div>
    <div class="spinner" id="spinner"></div>
    <div class="late" id="late">
      Still starting. <a id="anyway">Open anyway →</a>
    </div>
  </div>
<script>
(function () {
  var started = Date.now();
  var $ = function (id) { return document.getElementById(id); };
  function setStatus(t, isErr) { var el = $('status'); el.textContent = t; el.className = 'status' + (isErr ? ' err' : ''); }

  // Identify the app so we can show its name/icon.
  fetch('/__casadash/whoami', { cache: 'no-store' })
    .then(function (r) { return r.ok ? r.json() : null; })
    .then(function (a) {
      if (!a) return;
      document.title = 'Starting ' + a.name + '…';
      $('name').textContent = 'Starting ' + a.name + '…';
      if (a.icon) {
        var img = new Image();
        img.onload = function () { $('icon').innerHTML = ''; $('icon').appendChild(img); };
        img.src = a.icon; img.alt = '';
      } else {
        $('letter').textContent = (a.name || '?').charAt(0).toUpperCase();
      }
    })
    .catch(function () {});

  // Kick the app up (idempotent).
  fetch('/__casadash/start', { method: 'POST', cache: 'no-store' })
    .then(function (r) { return r.ok ? null : r.json().then(function (j) { throw new Error(j.error || 'start failed'); }); })
    .catch(function (e) { setStatus(e.message || 'Could not start the app', true); });

  // Poll the app root, SAME-ORIGIN, so we can read the real status.
  function probe() {
    return fetch('/?__cd=' + Date.now(), { cache: 'no-store', redirect: 'manual' })
      .then(function (res) {
        if (res.type === 'opaqueredirect') return 'ready';       // 3xx from the real app
        if (res.headers.get('X-Casadash')) return 'starting';    // still CasaDash catch-all
        if (res.status >= 200 && res.status < 400) return 'ready'; // genuine 2xx/3xx
        return 'wait';                                            // 5xx/502 while it boots
      })
      .catch(function () { return 'wait'; });
  }

  function loop() {
    probe().then(function (state) {
      if (state === 'ready') { setStatus('Ready — opening…'); location.replace('/'); return; }
      setStatus(state === 'starting' ? 'Starting the container…' : 'Waiting for it to respond…');
      if (Date.now() - started > 30000) $('late').classList.add('show');
      setTimeout(loop, 1500);
    });
  }
  $('anyway').addEventListener('click', function () { location.replace('/'); });
  loop();
})();
</script>
</body>
</html>`
