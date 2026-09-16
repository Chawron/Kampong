package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kampong/debate/internal/agent"
	"github.com/kampong/debate/internal/analyzer"
	"github.com/kampong/debate/internal/api"
	"github.com/kampong/debate/internal/config"
	"github.com/kampong/debate/internal/debate"
	"github.com/kampong/debate/internal/graph"
	"github.com/kampong/debate/internal/llm"
	"github.com/kampong/debate/internal/medical"
	"github.com/kampong/debate/internal/panel"
	"github.com/kampong/debate/internal/search"
	"github.com/kampong/debate/internal/store"
	"github.com/kampong/debate/internal/ws"
)

func main() {
	configPath := flag.String("config", "config.yaml", "Path to config file")
	flag.Parse()

	// Load config
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid config: %v", err)
	}

	if cfg.LLM.APIKey == "" {
		log.Println("WARNING: No LLM API key configured. Set it in config.yaml or via KAMPONG_LLM_KEY env var.")
		log.Println("         The debate system will fail when trying to call the LLM.")
	}

	// Initialize LLM client manager for multi-provider support
	clientManager := llm.NewClientManager(cfg)
	log.Printf("LLM Providers: %v", clientManager.ListProviders())

	// Initialize legacy single LLM client for components that haven't been migrated yet
	llmClient := llm.NewClient(cfg.LLM.BaseURL, cfg.LLM.APIKey, cfg.LLM.Model, cfg.Timeouts.LLMCompletion)

	// Initialize search (DuckDuckGo by default, Tavily if key provided).
	// When both providers are configured we compose them via MultiSourceSearcher
	// so results can be cross-verified by domain.
	var searcher search.Searcher
	if cfg.Search.TavilyAPIKey != "" {
		tavily := search.NewTavilySearcher(cfg.Search.TavilyAPIKey, cfg.Timeouts.Search)
		ddg := search.NewDuckDuckGoSearcher(cfg.Timeouts.Search)
		searcher = &search.MultiSourceSearcher{
			Primary:       ddg,
			Secondary:     tavily,
			PrimaryName:   "duckduckgo",
			SecondaryName: "tavily",
		}
		log.Println("Search: multi-source (DuckDuckGo + Tavily, cross-verified by domain)")
	} else {
		searcher = search.NewDuckDuckGoSearcher(cfg.Timeouts.Search)
		log.Println("Search: DuckDuckGo (no API key required)")
	}

	// Initialize WebSocket hub
	hub := ws.NewHub()

	// Initialize analyzer
	topicAnalyzer := analyzer.NewAnalyzer(llmClient, cfg)

	// Initialize panel composer
	panelComposer := panel.NewComposer(cfg)

	// Initialize agent runtime with multi-provider client manager
	agentRuntime := agent.NewRuntime(clientManager, searcher, cfg)

	// Initialize judge
	judge := debate.NewJudge(llmClient, cfg)

	// Initialize cross-examiner
	crossExam := debate.NewCrossExaminer(llmClient)

	// Initialize research cache (Pillar 5: Cost control)
	researchCache := search.NewResearchCache(30 * time.Minute)
	agentRuntime.SetCache(researchCache)
	log.Println("Research cache initialized (30min TTL)")

	// Initialize graph store + extractor
	graphStore := graph.NewStore()
	graphExtractor := graph.NewExtractor(llmClient, graphStore)

	// Initialize session persistence store.
// `json` (legacy, one .json per debate) or `sqlite` (pure-Go driver, also
// keeps events + calibration rolling averages).
	sessionStore, err := initSessionStore(cfg)
	if err != nil {
		log.Fatalf("Failed to create session store: %v", err)
	}

	// Initialize debate engine
	engine := debate.NewEngine(agentRuntime, judge, crossExam, graphExtractor, hub, cfg)
	engine.SetClientManager(clientManager)
	if calibStore, ok := sessionStore.(store.CalibrationStore); ok {
		engine.SetCalibrationStore(calibStore)
	}
	if evStore, ok := sessionStore.(store.EventStore); ok {
		hub.SetEventStore(evStore)
		log.Println("WS event persistence: enabled (late-join replay)")
	}

	// Initialize medical API
	medicalAPI := medical.NewMedicalAPI(engine, llmClient)
	log.Println("Medical debate system initialized")

	// Initialize HTTP server
	srv := api.NewServer(cfg, llmClient, clientManager, hub, topicAnalyzer, panelComposer, agentRuntime, engine, graphExtractor, searcher, sessionStore, medicalAPI)
	srv.SetConfigPath(*configPath)
	// Wire search-cache counters into /api/metrics.
	researchCache.SetSink(srv.Metrics().IncCacheHit, srv.Metrics().IncCacheMiss)
	router := srv.Router()

	// Register medical routes
	router.Get("/api/medical/consent-text", medicalAPI.HandleConsentText)
	router.Post("/api/medical/consent", medicalAPI.HandleConsent)
	router.Post("/api/medical/debate/start", medicalAPI.HandleStartDebate)
	router.Get("/api/medical/case", medicalAPI.HandleGetCase)
	router.Post("/api/medical/redflag-check", medicalAPI.HandleRedFlagCheck)
	router.Post("/api/medical/drug-interaction-check", medicalAPI.HandleDrugInteractionCheck)
	router.Post("/api/medical/lab-interpretation", medicalAPI.HandleLabInterpretation)
	router.Post("/api/medical/image-analysis", medicalAPI.HandleImageAnalysis)
	router.Post("/api/medical/extract-lab-image", medicalAPI.HandleExtractLabImage)
	router.Post("/api/medical/guideline-match", medicalAPI.HandleGuidelineMatch)
	router.Post("/api/medical/icd-code-match", medicalAPI.HandleICDCodeMatch)
	router.Post("/api/medical/second-opinion", medicalAPI.HandleSecondOpinion)
	router.Post("/api/medical/literature-search", medicalAPI.HandleLiteratureSearch)
	router.Post("/api/medical/generate-report", medicalAPI.HandleGenerateReport)
	router.Post("/api/medical/differential-ranking", medicalAPI.HandleDifferentialRanking)
	router.Post("/api/medical/risk-score", medicalAPI.HandleRiskScore)
	router.Post("/api/medical/longitudinal/save", medicalAPI.HandleLongitudinalSave)
	router.Get("/api/medical/longitudinal/load", medicalAPI.HandleLongitudinalLoad)
	log.Println("Medical API routes registered")

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	httpServer := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("Shutting down...")
		httpServer.Close()
	}()

	log.Printf("Kampong Debate System starting on http://%s", addr)
	log.Printf("Open http://localhost:%d in your browser", cfg.Server.Port)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}
