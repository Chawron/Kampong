package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/kampong/debate/internal/agent"
	"github.com/kampong/debate/internal/analyzer"
	"github.com/kampong/debate/internal/config"
	"github.com/kampong/debate/internal/debate"
	"github.com/kampong/debate/internal/graph"
	"github.com/kampong/debate/internal/llm"
	"github.com/kampong/debate/internal/medical"
	"github.com/kampong/debate/internal/models"
	"github.com/kampong/debate/internal/panel"
	"github.com/kampong/debate/internal/report"
	"github.com/kampong/debate/internal/search"
	"github.com/kampong/debate/internal/social"
	"github.com/kampong/debate/internal/store"
	"github.com/kampong/debate/internal/ws"
)

// Server holds all HTTP dependencies and implements the API.
type Server struct {
	cfg           *config.Config
	llmClient     *llm.Client
	clientManager *llm.ClientManager
	hub           *ws.Hub
	analyzer      *analyzer.Analyzer
	composer      *panel.Composer
	agentRuntime  *agent.Runtime
	engine        *debate.Engine
	extractor     *graph.Extractor
	searcher      search.Searcher
	sessionStore  store.Session
	medicalAPI    *medical.MedicalAPI
	metrics       *Metrics

	sessionsMu sync.RWMutex
	sessions   map[string]*models.DebateSession
	cancels    map[string]context.CancelFunc

	// File upload settings
	uploadDir string

	// Config persistence
	configPath string
}

// NewServer creates a fully wired API server.
func NewServer(cfg *config.Config, llmClient *llm.Client, clientManager *llm.ClientManager, hub *ws.Hub, a *analyzer.Analyzer, c *panel.Composer,
	ar *agent.Runtime, e *debate.Engine, ex *graph.Extractor, se search.Searcher, ss store.Session, medAPI *medical.MedicalAPI) *Server {
	return &Server{
		cfg:           cfg,
		llmClient:     llmClient,
		clientManager: clientManager,
		hub:           hub,
		analyzer:      a,
		composer:      c,
		agentRuntime:  ar,
		engine:        e,
		extractor:     ex,
		searcher:      se,
		sessionStore:  ss,
		medicalAPI:    medAPI,
		metrics:       NewMetrics(),
		sessions:      make(map[string]*models.DebateSession),
		cancels:       make(map[string]context.CancelFunc),
		uploadDir:     "data/uploads",
	}
}

// SetConfigPath sets the path for config persistence.
func (s *Server) SetConfigPath(path string) { s.configPath = path }

// Router builds the chi router with all routes and middleware.
func (s *Server) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
	}))

	// API routes
	r.Route("/api", func(r chi.Router) {
		r.Post("/debate/start", s.handleStartDebate)
		r.Get("/debate/{id}/status", s.handleDebateStatus)
		r.Get("/debate/{id}/history", s.handleDebateHistory)
		r.Get("/debate/{id}/graph", s.handleDebateGraph)
		r.Get("/debate/{id}/report", s.handleDebateReport)
		r.Post("/debate/{id}/cancel", s.handleCancelDebate)
		r.Post("/debate/{id}/pause", s.handlePauseDebate)
		r.Post("/debate/{id}/resume", s.handleResumeDebate)
		r.Post("/debate/{id}/inject", s.handleInjectEvidence)
		r.Post("/debate/{id}/challenge", s.handleChallengeClaim)
		r.Get("/debates", s.handleListDebates)
		r.Delete("/debate/{id}", s.handleDeleteDebate)
		r.Get("/config", s.handleGetConfig)
		r.Put("/config", s.handleUpdateConfig)
		r.Post("/upload", s.handleFileUpload)
		r.Post("/debate/{id}/social-simulation", s.handleSocialSimulation)
		r.Get("/providers", s.handleListProviders)
		r.Post("/providers", s.handleAddProvider)
		r.Delete("/providers/{name}", s.handleDeleteProvider)
	})

	// WebSocket
	r.Get("/ws/debate/{id}", s.handleWebSocket)

	// Ops endpoints (Phase 5 — health + metrics).
	// /api/health returns a JSON snapshot of the running server. Useful for
	// load balancers and the Docker HEALTHCHECK directive.
	// /api/metrics returns a hand-rolled Prometheus text exposition so an
	// existing scraper can pick up token counts, debate durations, etc.
	r.Get("/api/health", s.handleHealth)
	r.Get("/api/metrics", s.handleMetrics)

	// Serve uploaded files (images, etc.)
	uploadFS := http.FileServer(http.Dir(s.uploadDir))
	r.Handle("/uploads/*", http.StripPrefix("/uploads/", uploadFS))

	// Serve static frontend
	fs := http.FileServer(http.Dir("web"))
	r.Handle("/*", fs)

	return r
}

// --- Handlers ---

type startDebateRequest struct {
	Topic       string           `json:"topic"`
	Mode        string           `json:"mode"`
	Attachments []models.Attachment `json:"attachments"` // Optional uploaded files
}

func (s *Server) handleStartDebate(w http.ResponseWriter, r *http.Request) {
	var req startDebateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.Topic == "" {
		http.Error(w, `{"error":"topic is required"}`, http.StatusBadRequest)
		return
	}

	// Validate API key before starting — fail fast instead of wasting tokens on 401 errors
	if err := s.syncClientFromProviders(); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	mode := models.DebateMode(req.Mode)
	if mode == "" {
		mode = models.DebateMode(s.cfg.Debate.DefaultMode)
	}
	switch mode {
	case models.ModeQuick, models.ModeStandard, models.ModeDeep, models.ModeDiscussion:
	default:
		mode = models.ModeDeep
	}

	// Create session
	sessionID := uuid.New().String()
	totalRounds := s.roundsForMode(mode)

	// Build enhanced topic with attachment context
	enhancedTopic := req.Topic
	if len(req.Attachments) > 0 {
		enhancedTopic = s.buildTopicWithContext(req.Topic, req.Attachments)
	}

	session := &models.DebateSession{
		ID:          sessionID,
		Topic:       enhancedTopic,
		Mode:        mode,
		Status:      models.StatusAnalyzing,
		TotalRounds: totalRounds,
		CreatedAt:   time.Now(),
		Attachments: req.Attachments,
	}

	s.sessionsMu.Lock()
	s.sessions[sessionID] = session
	s.sessionsMu.Unlock()

	s.metrics.IncDebate()

	// Start debate in background
	ctx, cancel := context.WithCancel(context.Background())
	s.sessionsMu.Lock()
	s.cancels[sessionID] = cancel
	s.sessionsMu.Unlock()

	go s.runDebate(ctx, session)

	// Respond immediately
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"debate_id":    sessionID,
		"mode":         mode,
		"total_rounds": totalRounds,
		"ws_url":       "/ws/debate/" + sessionID,
	})
}

func (s *Server) runDebate(ctx context.Context, session *models.DebateSession) {
	defer func() {
		s.metrics.DecActive()
		if session.GetStatus() == models.StatusError {
			s.metrics.IncError()
		}
		s.sessionsMu.Lock()
		delete(s.cancels, session.ID)
		s.sessionsMu.Unlock()
	}()

	// Ensure main LLM client has credentials from providers if top-level key is empty
	if err := s.syncClientFromProviders(); err != nil {
		log.Printf("DEBAT [%s]: API key validation failed: %v", session.ID, err)
		session.SetStatus(models.StatusError)
		s.hub.Broadcast(session.ID, ws.Event{
			Event:    "error",
			DebateID: session.ID,
			Data: map[string]interface{}{
				"code":    "no_api_key",
				"message": err.Error(),
			},
		})
		return
	}

	// Phase 1: Analyze topic
	session.SetStatus(models.StatusAnalyzing)
	s.hub.Broadcast(session.ID, ws.Event{
		Event:    "phase_change",
		DebateID: session.ID,
		Data: map[string]interface{}{
			"phase":   "analyzing",
			"message": "Analyzing topic...",
		},
	})

	var agents []models.Agent

	if session.Mode == models.ModeDiscussion {
		// Discussion mode: use discussion analyzer + composer
		discSuggestion, err := s.analyzer.AnalyzeDiscussion(ctx, session.Topic)
		if err != nil {
			log.Printf("ANALYZER ERROR [%s]: %v", session.ID, err)
			session.SetStatus(models.StatusError)
			s.hub.Broadcast(session.ID, ws.Event{
				Event:    "error",
				DebateID: session.ID,
				Data: map[string]interface{}{
					"code":    "analyzer_error",
					"message": "Failed to analyze topic: " + err.Error(),
				},
			})
			return
		}

		session.SetStatus(models.StatusComposingPanel)
		s.hub.Broadcast(session.ID, ws.Event{
			Event:    "phase_change",
			DebateID: session.ID,
			Data: map[string]interface{}{
				"phase":   "composing_panel",
				"message": "Composing discussion panel...",
			},
		})

		agents, err = s.composer.ComposeDiscussion(discSuggestion, session.Topic)
		if err != nil {
			log.Printf("COMPOSER ERROR [%s]: %v", session.ID, err)
			session.SetStatus(models.StatusError)
			return
		}
	} else {
		// Debate mode: use standard analyzer + composer
		suggestion, err := s.analyzer.Analyze(ctx, session.Topic)
		if err != nil {
			log.Printf("ANALYZER ERROR [%s]: %v", session.ID, err)
			session.SetStatus(models.StatusError)
			s.hub.Broadcast(session.ID, ws.Event{
				Event:    "error",
				DebateID: session.ID,
				Data: map[string]interface{}{
					"code":    "analyzer_error",
					"message": "Failed to analyze topic: " + err.Error(),
				},
			})
			return
		}

		session.SetStatus(models.StatusComposingPanel)
		s.hub.Broadcast(session.ID, ws.Event{
			Event:    "phase_change",
			DebateID: session.ID,
			Data: map[string]interface{}{
				"phase":   "composing_panel",
				"message": "Composing expert panel...",
			},
		})

		agents, err = s.composer.Compose(suggestion, session.Topic)
		if err != nil {
			log.Printf("COMPOSER ERROR [%s]: %v", session.ID, err)
			session.SetStatus(models.StatusError)
			return
		}
	}

	session.Agents = agents

	s.hub.Broadcast(session.ID, ws.Event{
		Event:    "panel_ready",
		DebateID: session.ID,
		Data: map[string]interface{}{
			"agents": agents,
		},
	})

	// Phase 3: Run debate rounds
	s.engine.Run(ctx, session)

	// Auto-save after debate completes (verdict or error)
	if s.sessionStore != nil {
		if err := s.sessionStore.Save(session); err != nil {
			log.Printf("AUTO-SAVE ERROR [%s]: %v", session.ID, err)
		} else {
			log.Printf("Session saved: %s", session.ID)
		}
	}
}

func (s *Server) handleDebateStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	session := s.getSession(id)
	if session == nil {
		http.Error(w, `{"error":"debate not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":           session.ID,
		"topic":        session.Topic,
		"mode":         session.Mode,
		"status":       session.GetStatus(),
		"round":        session.Round,
		"total_rounds": session.TotalRounds,
		"agents":       session.Agents,
		"verdict":      session.GetVerdict(),
	})
}

func (s *Server) handleDebateHistory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	session := s.getSession(id)
	if session == nil {
		http.Error(w, `{"error":"debate not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"debate_id":  session.ID,
		"topic":      session.Topic,
		"transcript": session.GetTranscript(),
	})
}

func (s *Server) handleDebateGraph(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	session := s.getSession(id)
	if session == nil {
		http.Error(w, `{"error":"debate not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"debate_id": session.ID,
		"nodes":     session.Graph.Nodes,
		"edges":     session.Graph.Edges,
	})
}

func (s *Server) handleDebateReport(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	session := s.getSession(id)
	if session == nil {
		http.Error(w, `{"error":"debate not found"}`, http.StatusNotFound)
		return
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "md"
	}

	md := report.GenerateMarkdown(session)

	if format == "html" {
		// Return HTML for PDF printing
		html := report.MarkdownToHTML(session.Topic, md)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(html))
		return
	}

	// Default: markdown download
	filename := sanitizeFilename(session.Topic) + ".md"
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Write([]byte(md))
}

func sanitizeFilename(s string) string {
	// Replace non-filename chars
	replacer := strings.NewReplacer(
		"/", "_", "\\", "_", ":", "_", "*", "_",
		"?", "_", "\"", "_", "<", "_", ">", "_", "|", "_",
	)
	name := replacer.Replace(s)
	if len(name) > 80 {
		name = name[:80]
	}
	return strings.TrimSpace(name)
}

func (s *Server) handleSocialSimulation(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	session := s.getSession(id)
	if session == nil {
		http.Error(w, `{"error":"debate not found"}`, http.StatusNotFound)
		return
	}

	// Check that debate has a verdict or synthesis
	if session.GetVerdict() == nil && session.GetSynthesis() == nil {
		http.Error(w, `{"error":"debate has no verdict or synthesis yet"}`, http.StatusBadRequest)
		return
	}

	// Ensure LLM client has credentials
	if err := s.syncClientFromProviders(); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	// Extract verdict and synthesis text
	verdictText := ""
	verdict := session.GetVerdict()
	if verdict != nil {
		verdictText = verdict.Reasoning
		if verdict.Synthesis != "" {
			verdictText += "\n\n" + verdict.Synthesis
		}
	}
	synthesisText := ""
	synthesis := session.GetSynthesis()
	if synthesis != nil {
		synthesisText = synthesis.Overview
	}

	// Broadcast phase change
	session.SetStatus(models.StatusSocialReaction)
	s.hub.Broadcast(session.ID, ws.Event{
		Event:    "phase_change",
		DebateID: session.ID,
		Data: map[string]interface{}{
			"phase":   "social_reaction",
			"message": "Running social reaction simulation...",
		},
	})

	// Run the simulation
	result, err := social.RunSimulation(r.Context(), s.llmClient, session.Topic, verdictText, synthesisText)
	if err != nil {
		log.Printf("SOCIAL SIMULATION ERROR [%s]: %v", session.ID, err)
		http.Error(w, `{"error":"social simulation failed: `+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	// Store result in session
	session.SetSocialResult(result)

	// Broadcast social_simulation WebSocket event
	s.hub.Broadcast(session.ID, ws.Event{
		Event:    "social_simulation",
		DebateID: session.ID,
		Data:     result,
	})

	// Auto-save
	if s.sessionStore != nil {
		if err := s.sessionStore.Save(session); err != nil {
			log.Printf("AUTO-SAVE ERROR [%s]: %v", session.ID, err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *Server) handleCancelDebate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	s.sessionsMu.Lock()
	cancel, ok := s.cancels[id]
	s.sessionsMu.Unlock()

	if !ok {
		http.Error(w, `{"error":"debate not found or not running"}`, http.StatusNotFound)
		return
	}
	cancel()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "cancelled"})
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.cfg)
}

func (s *Server) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	// Apply simple top-level updates
	if v, ok := updates["api_key"]; ok {
		if key, ok := v.(string); ok {
			s.cfg.LLM.APIKey = key
			s.llmClient.SetAPIKey(key)
			// Propagate to ALL components
			if s.medicalAPI != nil {
				s.medicalAPI.SetLLMClient(s.llmClient)
			}
			if s.engine != nil {
				s.engine.SetLLMClient(s.llmClient)
			}
		}
	}
	if v, ok := updates["base_url"]; ok {
		if url, ok := v.(string); ok {
			s.cfg.LLM.BaseURL = url
			s.llmClient.SetBaseURL(url)
			if s.medicalAPI != nil {
				s.medicalAPI.SetLLMClient(s.llmClient)
			}
		}
	}
	if v, ok := updates["model"]; ok {
		if model, ok := v.(string); ok {
			s.cfg.LLM.Model = model
			s.llmClient.SetModel(model)
			if s.medicalAPI != nil {
				s.medicalAPI.SetLLMClient(s.llmClient)
			}
		}
	}
	if v, ok := updates["tavily_key"]; ok {
		if key, ok := v.(string); ok {
			s.cfg.Search.TavilyAPIKey = key
		}
	}

	// Persist config to disk so API key survives restarts
	if s.configPath != "" {
		if err := s.cfg.Save(s.configPath); err != nil {
			log.Printf("CONFIG: Failed to save config: %v", err)
		} else {
			log.Printf("CONFIG: Saved config to %s", s.configPath)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// syncClientFromProviders ensures the main LLM client has credentials.
// If the top-level API key is empty, it copies credentials from the first
// available provider (so the analyzer, composer, and graph extractor work).
// Returns an error if no API key is available anywhere.
func (s *Server) syncClientFromProviders() error {
	if s.llmClient.HasAPIKey() {
		return nil
	}
	best := s.clientManager.GetBestClient()
	if best == nil {
		return fmt.Errorf("no API key configured. Please add your API key in Settings or add a Provider.")
	}
	log.Printf("CONFIG: Main LLM client has no API key, syncing from provider %q", best.Name)
	s.llmClient.SetAPIKey(best.APIKey())
	s.llmClient.SetBaseURL(best.BaseURL())
	if best.Model() != "" {
		s.llmClient.SetModel(best.Model())
	}
	// Propagate to ALL components that use the LLM client
	if s.medicalAPI != nil {
		s.medicalAPI.SetLLMClient(s.llmClient)
	}
	if s.engine != nil {
		s.engine.SetLLMClient(s.llmClient)
	}
	return nil
}

// WebSocket handler
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	session := s.getSession(id)
	if session == nil {
		http.Error(w, `{"error":"debate not found"}`, http.StatusNotFound)
		return
	}

	// ?from_seq=N triggers a server-side replay of all stored events with
	// seq > N before the client joins the live broadcast. Combined with the
	// hub's atomic sequence counter, this gives late joiners a consistent
	// view of everything since they last disconnected.
	var fromSeq int64
	if v := r.URL.Query().Get("from_seq"); v != "" {
		// Hand-rolled parsing to avoid the strconv import bloat.
		var n int64
		for _, ch := range v {
			if ch < '0' || ch > '9' {
				n = 0
				break
			}
			n = n*10 + int64(ch-'0')
		}
		fromSeq = n
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WS UPGRADE ERROR: %v", err)
		return
	}
	defer conn.Close()

	send := make(chan []byte, 256)
	client := s.hub.Subscribe(id, send)
	defer s.hub.Unsubscribe(client)

	// 1) Server-side event replay (preferred path when an EventStore is wired).
	replayed := 0
	if n, err := s.hub.ReplayFrom(id, fromSeq, send); err != nil {
		log.Printf("WS REPLAY ERROR [%s]: %v", id, err)
	} else if n > 0 {
		replayed = n
		log.Printf("WS REPLAY [%s] %d events from seq>%d", id, n, fromSeq)
	}

	// 2) Fallback synthetic replay ONLY when the event store produced nothing
	// (JSON backend, or debate started before the store was wired). Sending
	// both would deliver the verdict/transcript twice.
	if replayed == 0 {
		s.replayState(conn, session)
	}

	// Write pump: send messages to the WebSocket
	go func() {
		for msg := range send {
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		}
	}()

	// Read pump: keep connection alive and handle pings
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

// replayState sends the current debate state to a reconnecting client.
func (s *Server) replayState(conn *websocket.Conn, session *models.DebateSession) {
	// Send current phase
	phaseEvt := ws.Event{
		Event:    "phase_change",
		DebateID: session.ID,
		Data: map[string]interface{}{
			"phase":        string(session.GetStatus()),
			"round":        session.Round,
			"total_rounds": session.TotalRounds,
			"mode":         string(session.Mode),
		},
	}
	if data, err := json.Marshal(phaseEvt); err == nil {
		conn.WriteMessage(websocket.TextMessage, data)
	}

	// Send panel if ready
	if len(session.Agents) > 0 {
		panelEvt := ws.Event{
			Event:    "panel_ready",
			DebateID: session.ID,
			Data: map[string]interface{}{"agents": session.Agents},
		}
		if data, err := json.Marshal(panelEvt); err == nil {
			conn.WriteMessage(websocket.TextMessage, data)
		}
	}

	// Send existing transcript entries
	for _, entry := range session.GetTranscript() {
		doneEvt := ws.Event{
			Event:    "agent_done",
			DebateID: session.ID,
			Data: map[string]interface{}{
				"agent_id":  entry.AgentID,
				"full_text": entry.Text,
			},
		}
		if data, err := json.Marshal(doneEvt); err == nil {
			conn.WriteMessage(websocket.TextMessage, data)
		}
	}

	// Send graph state
	if len(session.Graph.Nodes) > 0 {
		graphEvt := ws.Event{
			Event:    "graph_update",
			DebateID: session.ID,
			Data: map[string]interface{}{
				"new_nodes": session.Graph.Nodes,
				"new_edges": session.Graph.Edges,
			},
		}
		if data, err := json.Marshal(graphEvt); err == nil {
			conn.WriteMessage(websocket.TextMessage, data)
		}
	}

	// Send verdict if available
	verdict := session.GetVerdict()
	if verdict != nil {
		verdictEvt := ws.Event{
			Event:    "verdict",
			DebateID: session.ID,
			Data:     verdict,
		}
		if data, err := json.Marshal(verdictEvt); err == nil {
			conn.WriteMessage(websocket.TextMessage, data)
		}
	}
}

func (s *Server) getSession(id string) *models.DebateSession {
	s.sessionsMu.RLock()
	sess := s.sessions[id]
	s.sessionsMu.RUnlock()

	if sess != nil {
		return sess
	}

	// Try loading from persistent store
	if s.sessionStore != nil {
		loaded, err := s.sessionStore.Load(id)
		if err != nil {
			log.Printf("SESSION LOAD ERROR [%s]: %v", id, err)
			return nil
		}
		if loaded != nil {
			// Register back into memory
			s.sessionsMu.Lock()
			s.sessions[id] = loaded
			s.sessionsMu.Unlock()
			return loaded
		}
	}

	return nil
}

func (s *Server) roundsForMode(mode models.DebateMode) int {
	switch mode {
	case models.ModeQuick:
		return 2
	case models.ModeStandard:
		return 3
	case models.ModeDiscussion:
		return 4
	default:
		return 5
	}
}

// handleListDebates returns all saved/past debates.
func (s *Server) handleListDebates(w http.ResponseWriter, r *http.Request) {
	// Merge in-memory active sessions with persisted ones
	type debateItem struct {
		ID         string `json:"id"`
		Topic      string `json:"topic"`
		Mode       string `json:"mode"`
		Status     string `json:"status"`
		Agents     int    `json:"agents"`
		Rounds     int    `json:"rounds"`
		HasVerdict bool   `json:"has_verdict"`
		IsActive   bool   `json:"is_active"`
	}

	seen := make(map[string]bool)
	var items []debateItem

	// Active sessions first
	s.sessionsMu.RLock()
	for _, sess := range s.sessions {
		seen[sess.ID] = true
		items = append(items, debateItem{
			ID:         sess.ID,
			Topic:      sess.Topic,
			Mode:       string(sess.Mode),
			Status:     string(sess.Status),
			Agents:     len(sess.Agents),
			Rounds:     sess.Round,
			HasVerdict: sess.Verdict != nil,
			IsActive:   true,
		})
	}
	s.sessionsMu.RUnlock()

	// Persisted sessions
	if s.sessionStore != nil {
		metas, err := s.sessionStore.List()
		if err == nil {
			for _, m := range metas {
				if seen[m.ID] {
					continue
				}
				items = append(items, debateItem{
					ID:         m.ID,
					Topic:      m.Topic,
					Mode:       m.Mode,
					Status:     m.Status,
					Agents:     m.Agents,
					Rounds:     m.Rounds,
					HasVerdict: m.HasVerdict,
					IsActive:   false,
				})
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"debates": items})
}

// handleDeleteDebate removes a debate from memory and disk.
func (s *Server) handleDeleteDebate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	s.sessionsMu.Lock()
	delete(s.sessions, id)
	s.sessionsMu.Unlock()

	if s.sessionStore != nil {
		if err := s.sessionStore.Delete(id); err != nil {
			log.Printf("DELETE SESSION ERROR [%s]: %v", id, err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

// buildTopicWithContext builds an enhanced topic string with attachment context.
func (s *Server) buildTopicWithContext(topic string, attachments []models.Attachment) string {
	if len(attachments) == 0 {
		return topic
	}

	var sb strings.Builder
	sb.WriteString(topic)
	sb.WriteString("\n\n--- Attached Context ---\n")

	for i, att := range attachments {
		sb.WriteString(fmt.Sprintf("\n[Attachment %d: %s (%s)]\n", i+1, att.Name, att.Type))

		if att.Type == "image" && att.Analysis != "" {
			sb.WriteString("Image Analysis:\n")
			sb.WriteString(att.Analysis)
			sb.WriteString("\n")
		} else if att.TextContent != "" {
			sb.WriteString("Content:\n")
			// Truncate long content
			content := att.TextContent
			if len(content) > 2000 {
				content = content[:2000] + "\n... (truncated)"
			}
			sb.WriteString(content)
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n--- End Attached Context ---\n")
	return sb.String()
}

// handleFileUpload handles multipart file uploads with image analysis.
func (s *Server) handleFileUpload(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form (max 10MB)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, `{"error":"failed to parse multipart form"}`, http.StatusBadRequest)
		return
	}

	// Get the file from form
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, `{"error":"file is required"}`, http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Generate unique ID
	fileID := uuid.New().String()
	mime := header.Header.Get("Content-Type")

	// Ensure upload directory exists
	if err := os.MkdirAll(s.uploadDir, 0755); err != nil {
		log.Printf("UPLOAD ERROR: failed to create upload dir: %v", err)
		http.Error(w, `{"error":"storage error"}`, http.StatusInternalServerError)
		return
	}

	// Save file
	filePath := filepath.Join(s.uploadDir, fileID+"_"+header.Filename)
	dst, err := os.Create(filePath)
	if err != nil {
		log.Printf("UPLOAD ERROR: failed to save file: %v", err)
		http.Error(w, `{"error":"storage error"}`, http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		log.Printf("UPLOAD ERROR: failed to write file: %v", err)
		http.Error(w, `{"error":"storage error"}`, http.StatusInternalServerError)
		return
	}

	// Determine file type and process
	fileType := "document"
	var analysisResult string
	var isImage bool

	if strings.HasPrefix(mime, "image/") {
		fileType = "image"
		isImage = true

		// Analyze image with vision model if available
		analysisResult = s.analyzeImage(r.Context(), filePath, mime)
	}

	// Read file content for text-based files
	var textContent string
	if mime == "text/plain" || mime == "text/markdown" || strings.HasSuffix(header.Filename, ".txt") || strings.HasSuffix(header.Filename, ".md") {
		content, err := os.ReadFile(filePath)
		if err == nil {
			textContent = string(content)
			if len(textContent) > 5000 {
				textContent = textContent[:5000] + "..."
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":           fileID,
		"name":         header.Filename,
		"size":         header.Size,
		"mime_type":    mime,
		"type":         fileType,
		"is_image":     isImage,
		"file_path":    filePath,
		"analysis":     analysisResult,
		"text_content": textContent,
	})
}

// analyzeImage analyzes an image using a vision-capable model.
func (s *Server) analyzeImage(ctx context.Context, filePath, mime string) string {
	// Check if we have a vision-capable provider
	visionClient, err := s.clientManager.GetVisionClient()
	if err != nil {
		log.Printf("IMAGE ANALYSIS: No vision provider available: %v", err)
		return ""
	}

	// Read and encode image
	imageData, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("IMAGE ANALYSIS: Failed to read file: %v", err)
		return ""
	}

	// Convert to base64 data URL
	base64Data := base64.StdEncoding.EncodeToString(imageData)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mime, base64Data)

	// Intelligent analysis prompt — auto-detects image type and applies the right strategy
	prompt := `First, identify what type of image this is. Then analyze it accordingly.

## Image Type Detection
Look at the image and classify it as ONE of these types:
- **Lab Report** — a table/list of lab test results with values and reference ranges
- **X-ray** — radiograph (chest, bone, dental, etc.)
- **CT/MMRI** — cross-sectional or detailed radiology scan
- **Ultrasound** — sonogram image
- **ECG/EKG** — heart rhythm strip
- **Chart/Graph** — data visualization, trend graph, vital signs chart
- **Clinical Photo** — wound, rash, physical examination photo
- **Diagram/Illustration** — anatomical drawing, surgical diagram, educational figure
- **Medication/Prescription** — drug label, prescription sheet
- **General Photo** — any other image not fitting above

## Analysis Rules by Type

**If Lab Report:** Extract ALL test results as a structured list:
- Test name, Value, Unit, Reference range, Flag (H/L/normal)
- Note any critically abnormal values

**If X-ray/CT/MRI/Ultrasound:** Provide radiologist-style analysis:
- Describe anatomical structures visible
- Note any abnormalities (fractures, masses, effusions, infiltrates)
- Overall impression
- Urgency level if concerning findings

**If ECG/EKG:** Analyze the rhythm:
- Heart rate, rhythm regularity
- P wave, QRS complex, ST segment observations
- Any arrhythmia or conduction abnormality

**If Chart/Graph:** Read the data:
- What variables are shown
- Key trends, peaks, valleys
- Notable values at specific timepoints

**If Clinical Photo:** Describe:
- Location and appearance
- Size, color, texture observations
- Possible conditions suggested

**If General Photo/Diagram:** Describe:
- What is shown
- Key elements and their relevance
- How it relates to medical/health context

## Output Format
Start with: **[Image Type: <detected type>]**
Then provide the analysis following the rules above.
Be specific, use proper terminology, and note any limitations.`

	messages := []llm.Message{
		{
			Role: "system",
			Content: `You are an intelligent medical image analysis assistant. You can identify any type of image — medical imaging, lab reports, charts, clinical photos, diagrams, or general pictures — and provide the appropriate analysis. Always be specific and educational. Note limitations. This is for educational discussion purposes only.`,
		},
		{
			Role: "user",
			Content: []llm.ContentPart{
				{Type: "text", Text: prompt},
				{Type: "image_url", ImageURL: &llm.ImageURL{URL: dataURL}},
			},
		},
	}

	if visionClient.VisionModel != "" {
		log.Printf("IMAGE ANALYSIS: Analyzing with provider %q, vision model %s", visionClient.Name, visionClient.VisionModel)
	} else {
		log.Printf("IMAGE ANALYSIS: Analyzing with provider %q (default model)", visionClient.Name)
	}

	analysis, err := visionClient.Complete(ctx, messages, 0.3, 2048)
	if err != nil {
		log.Printf("IMAGE ANALYSIS ERROR: %v", err)
		return ""
	}

	return analysis
}

// handleListProviders returns all configured LLM providers.
func (s *Server) handleListProviders(w http.ResponseWriter, r *http.Request) {
	providers := make([]map[string]interface{}, 0, len(s.cfg.Providers))
	for _, p := range s.cfg.Providers {
		providers = append(providers, map[string]interface{}{
			"name":         p.Name,
			"base_url":     p.BaseURL,
			"model":        p.Model,
			"vision_model": p.VisionModel,
			"enabled":      p.Enabled,
			"has_api_key":  p.APIKey != "",
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"providers": providers})
}

// handleAddProvider adds a new LLM provider configuration.
func (s *Server) handleAddProvider(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		BaseURL     string `json:"base_url"`
		APIKey      string `json:"api_key"`
		Model       string `json:"model"`
		VisionModel string `json:"vision_model"`
		Enabled     bool   `json:"enabled"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.BaseURL == "" || req.Model == "" {
		http.Error(w, `{"error":"name, base_url, and model are required"}`, http.StatusBadRequest)
		return
	}

	// Add to config
	provider := config.ProviderConfig{
		Name:        req.Name,
		BaseURL:     req.BaseURL,
		APIKey:      req.APIKey,
		Model:       req.Model,
		VisionModel: req.VisionModel,
		Enabled:     req.Enabled,
	}

	// Check if provider already exists and update it
	found := false
	for i, p := range s.cfg.Providers {
		if p.Name == req.Name {
			s.cfg.Providers[i] = provider
			found = true
			break
		}
	}
	if !found {
		s.cfg.Providers = append(s.cfg.Providers, provider)
	}

	// Update the client manager with the new provider (only if enabled)
	if req.Enabled {
		s.clientManager.AddProvider(provider)
		log.Printf("PROVIDER: Added/updated provider %s (vision: %s)", req.Name, req.VisionModel)
	}

	// Persist to disk
	if s.configPath != "" {
		if err := s.cfg.Save(s.configPath); err != nil {
			log.Printf("CONFIG: Failed to save config: %v", err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "added"})
}

// handleDeleteProvider removes an LLM provider configuration.
func (s *Server) handleDeleteProvider(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	// Remove from config
	for i, p := range s.cfg.Providers {
		if p.Name == name {
			s.cfg.Providers = append(s.cfg.Providers[:i], s.cfg.Providers[i+1:]...)
			break
		}
	}

	// Remove from client manager
	s.clientManager.RemoveProvider(name)
	log.Printf("PROVIDER: Removed provider %s", name)

	// Persist to disk
	if s.configPath != "" {
		if err := s.cfg.Save(s.configPath); err != nil {
			log.Printf("CONFIG: Failed to save config: %v", err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}
