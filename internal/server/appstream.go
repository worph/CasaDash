package server

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"

	"github.com/coder/websocket"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/go-chi/chi/v5"
)

type configBody struct {
	Override string `json:"override"`
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	if !s.requireApps(w) {
		return
	}
	cfg, err := s.apps.GetConfig(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (s *Server) handlePutConfig(w http.ResponseWriter, r *http.Request) {
	if !s.requireApps(w) {
		return
	}
	var body configBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	if err := s.apps.SetConfig(r.Context(), chi.URLParam(r, "id"), body.Override); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.broadcastApps()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleAppLogs streams a managed/unmanaged app's container logs over a WebSocket.
func (s *Server) handleAppLogs(w http.ResponseWriter, r *http.Request) {
	if s.dx == nil {
		http.Error(w, "docker unavailable", http.StatusServiceUnavailable)
		return
	}
	cid, err := s.dx.FirstContainerID(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: []string{"*"}})
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")
	ctx := conn.CloseRead(r.Context()) // cancels when the client disconnects

	reader, err := s.dx.ContainerLogs(ctx, cid, "300", true)
	if err != nil {
		return
	}
	defer reader.Close()

	// Demux Docker's stdout/stderr multiplexed stream into plain lines.
	pr, pw := io.Pipe()
	go func() {
		_, _ = stdcopy.StdCopy(pw, pw, reader)
		_ = pw.Close()
	}()

	sc := bufio.NewScanner(pr)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		if err := conn.Write(ctx, websocket.MessageText, sc.Bytes()); err != nil {
			return
		}
	}
}

// handleAppStats streams computed CPU%/memory for an app's main container.
func (s *Server) handleAppStats(w http.ResponseWriter, r *http.Request) {
	if s.dx == nil {
		http.Error(w, "docker unavailable", http.StatusServiceUnavailable)
		return
	}
	cid, err := s.dx.FirstContainerID(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: []string{"*"}})
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")
	ctx := conn.CloseRead(r.Context())

	body, err := s.dx.ContainerStatsStream(ctx, cid)
	if err != nil {
		return
	}
	defer body.Close()

	dec := json.NewDecoder(body)
	for {
		var v container.StatsResponse
		if err := dec.Decode(&v); err != nil {
			return
		}
		out := statsFrame{
			CPUPercent: computeCPU(v),
			MemUsage:   v.MemoryStats.Usage,
			MemLimit:   v.MemoryStats.Limit,
		}
		raw, _ := json.Marshal(out)
		if err := conn.Write(ctx, websocket.MessageText, raw); err != nil {
			return
		}
	}
}

type statsFrame struct {
	CPUPercent float64 `json:"cpu_percent"`
	MemUsage   uint64  `json:"mem_usage"`
	MemLimit   uint64  `json:"mem_limit"`
}

// computeCPU derives a CPU percentage from a streamed stats frame (which carries
// the previous sample for delta computation).
func computeCPU(v container.StatsResponse) float64 {
	cpuDelta := float64(v.CPUStats.CPUUsage.TotalUsage) - float64(v.PreCPUStats.CPUUsage.TotalUsage)
	sysDelta := float64(v.CPUStats.SystemUsage) - float64(v.PreCPUStats.SystemUsage)
	online := float64(v.CPUStats.OnlineCPUs)
	if online == 0 {
		online = float64(len(v.CPUStats.CPUUsage.PercpuUsage))
	}
	if sysDelta > 0 && cpuDelta > 0 {
		pct := (cpuDelta / sysDelta) * online * 100
		return float64(int64(pct*10+0.5)) / 10
	}
	return 0
}
