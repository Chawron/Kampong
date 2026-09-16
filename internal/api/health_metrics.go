package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync/atomic"
	"time"
)

// Metrics is a tiny in-memory counter set exposed via /api/metrics (Prometheus
// text format) and included in /api/health for quick inspection.
//
// Counters are intentionally atomic primitives — no external Prometheus
// client library is needed for this scope, and a single binary that emits
// its own text is easier to scrape from any monitoring stack.
type Metrics struct {
	startTime time.Time

	debatesTotal  atomic.Int64
	debatesActive atomic.Int64
	debatesError  atomic.Int64

	searchCacheHits   atomic.Int64
	searchCacheMisses atomic.Int64

	wsDrops    atomic.Int64
	wsReplays  atomic.Int64
}

// NewMetrics creates a Metrics struct with startTime set to now.
func NewMetrics() *Metrics { return &Metrics{startTime: time.Now()} }

// Metrics returns the server's metrics struct so main.go can wire it into
// other subsystems (cache sinks, etc.).
func (s *Server) Metrics() *Metrics { return s.metrics }

func (m *Metrics) IncDebate() {
	if m == nil {
		return
	}
	m.debatesTotal.Add(1)
	m.debatesActive.Add(1)
}
func (m *Metrics) DecActive() {
	if m == nil {
		return
	}
	m.debatesActive.Add(-1)
}
func (m *Metrics) IncError() {
	if m == nil {
		return
	}
	m.debatesError.Add(1)
}
func (m *Metrics) IncCacheHit() {
	if m == nil {
		return
	}
	m.searchCacheHits.Add(1)
}
func (m *Metrics) IncCacheMiss() {
	if m == nil {
		return
	}
	m.searchCacheMisses.Add(1)
}
func (m *Metrics) IncWSDrop() {
	if m == nil {
		return
	}
	m.wsDrops.Add(1)
}
func (m *Metrics) IncWSReplay(n int) {
	if m == nil || n <= 0 {
		return
	}
	m.wsReplays.Add(int64(n))
}

// Snapshot returns a map of counter name → value for inclusion in /api/health.
func (m *Metrics) Snapshot() map[string]int64 {
	if m == nil {
		return nil
	}
	return map[string]int64{
		"debates_total":       m.debatesTotal.Load(),
		"debates_active":      m.debatesActive.Load(),
		"debates_error":       m.debatesError.Load(),
		"search_cache_hits":   m.searchCacheHits.Load(),
		"search_cache_misses": m.searchCacheMisses.Load(),
		"ws_drops":            m.wsDrops.Load(),
		"ws_replays":          m.wsReplays.Load(),
	}
}

// handleHealth returns a JSON snapshot useful for load balancers and the
// Docker HEALTHCHECK directive. Cheap to call.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(s.metrics.startTime).Round(time.Second).String()

	providerCount := 0
	if s.clientManager != nil {
		providerCount = len(s.clientManager.ListProviders())
	}

	storageBackend := "json"
	if s.cfg != nil && s.cfg.Storage.Backend != "" {
		storageBackend = s.cfg.Storage.Backend
	}

	body := map[string]interface{}{
		"status":           "ok",
		"uptime":           uptime,
		"providers":        providerCount,
		"storage_backend":  storageBackend,
		"active_debates":   s.metrics.debatesActive.Load(),
		"debates_total":    s.metrics.debatesTotal.Load(),
		"counters":         s.metrics.Snapshot(),
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(body)
}

// handleMetrics returns Prometheus text exposition format.
//
// We hand-format the response (no client_golang dependency) — the surface
// here is tiny and we want the binary to stay small.
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	var b strings.Builder

	// Uptime
	fmt.Fprintf(&b, "# HELP kampong_uptime_seconds seconds since process start\n")
	fmt.Fprintf(&b, "# TYPE kampong_uptime_seconds counter\n")
	fmt.Fprintf(&b, "kampong_uptime_seconds %.0f\n", time.Since(s.metrics.startTime).Seconds())

	// Debate counters
	writeCounter(&b, "kampong_debates_total", s.metrics.debatesTotal.Load(),
		"total debates started since process start")
	writeCounter(&b, "kampong_debates_active", s.metrics.debatesActive.Load(),
		"debates currently running")
	writeCounter(&b, "kampong_debates_error", s.metrics.debatesError.Load(),
		"debates that exited with an error")

	// Cache counters
	writeCounter(&b, "kampong_search_cache_hits_total", s.metrics.searchCacheHits.Load(),
		"search cache hits")
	writeCounter(&b, "kampong_search_cache_misses_total", s.metrics.searchCacheMisses.Load(),
		"search cache misses")

	// WebSocket counters
	writeCounter(&b, "kampong_ws_drops_total", s.metrics.wsDrops.Load(),
		"WebSocket frames dropped (client buffer full)")
	writeCounter(&b, "kampong_ws_replays_total", s.metrics.wsReplays.Load(),
		"WebSocket events replayed to late joiners")

	// Per-provider token totals
	if s.clientManager != nil {
		tokens := s.clientManager.TokensByProvider()
		// Sort provider names so the output is deterministic and easy to diff.
		names := make([]string, 0, len(tokens))
		for k := range tokens {
			names = append(names, k)
		}
		sort.Strings(names)
		fmt.Fprintf(&b, "# HELP kampong_llm_tokens_total total tokens seen per provider\n")
		fmt.Fprintf(&b, "# TYPE kampong_llm_tokens_total counter\n")
		for _, n := range names {
			fmt.Fprintf(&b, "kampong_llm_tokens_total{provider=%q} %d\n", n, tokens[n])
		}
	}

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	_, _ = w.Write([]byte(b.String()))
}

func writeCounter(b *strings.Builder, name string, value int64, help string) {
	fmt.Fprintf(b, "# HELP %s %s\n", name, help)
	fmt.Fprintf(b, "# TYPE %s counter\n", name)
	fmt.Fprintf(b, "%s %d\n", name, value)
}