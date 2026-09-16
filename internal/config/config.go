package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds all application configuration.
type Config struct {
	LLM       LLMConfig        `yaml:"llm"`
	Providers []ProviderConfig `yaml:"providers"`
	Search    SearchConfig     `yaml:"search"`
	Debate    DebateConfig     `yaml:"debate"`
	Timeouts  TimeoutConfig    `yaml:"timeouts"`
	Storage   StorageConfig    `yaml:"storage"`
	Server    ServerConfig     `yaml:"server"`
}

type LLMConfig struct {
	Provider    string  `yaml:"provider"`
	BaseURL     string  `yaml:"base_url"`
	APIKey      string  `yaml:"api_key"`
	Model       string  `yaml:"model"`
	FastModel   string  `yaml:"fast_model"`
	Temperature float64 `yaml:"temperature"`
	MaxTokens   int     `yaml:"max_tokens"`
}

// ProviderConfig represents a single LLM provider configuration.
type ProviderConfig struct {
	Name        string `yaml:"name"`         // "qwen", "deepseek", "openai", "claude"
	BaseURL     string `yaml:"base_url"`     // API endpoint
	APIKey      string `yaml:"api_key"`      // API key
	Model       string `yaml:"model"`        // Default model for this provider
	VisionModel string `yaml:"vision_model"` // Vision-capable model (optional)
	Family      string `yaml:"family"`       // Vendor family for diversity routing
	Enabled     bool   `yaml:"enabled"`      // Whether this provider is active
}

// Family returns the provider's vendor family, inferring a sensible default
// from the name or URL when not explicitly set.
func (p ProviderConfig) ResolvedFamily() string {
	if p.Family != "" {
		return normalizeFamily(p.Family)
	}
	return inferFamily(p.Name, p.BaseURL, p.Model)
}

// inferFamily picks a family from name / URL / model when Family is unset.
func inferFamily(name, baseURL, model string) string {
	lower := strings.ToLower(name + " " + baseURL + " " + model)
	switch {
	case strings.Contains(lower, "openai"), strings.Contains(lower, "gpt-"):
		return "openai"
	case strings.Contains(lower, "anthropic"), strings.Contains(lower, "claude"):
		return "anthropic"
	case strings.Contains(lower, "google"), strings.Contains(lower, "gemini"):
		return "google"
	case strings.Contains(lower, "qwen"), strings.Contains(lower, "alibaba"), strings.Contains(lower, "dashscope"):
		return "alibaba"
	case strings.Contains(lower, "llama"), strings.Contains(lower, "meta"):
		return "meta"
	case strings.Contains(lower, "mistral"), strings.Contains(lower, "mixtral"):
		return "mistral"
	case strings.Contains(lower, "xai"), strings.Contains(lower, "grok"):
		return "xai"
	case strings.Contains(lower, "deepseek"):
		return "deepseek"
	case strings.Contains(lower, "ollama"):
		return "ollama"
	default:
		return "other"
	}
}

func normalizeFamily(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "openai":
		return "openai"
	case "anthropic", "claude":
		return "anthropic"
	case "google", "gemini":
		return "google"
	case "alibaba", "qwen", "dashscope":
		return "alibaba"
	case "meta", "llama":
		return "meta"
	case "mistral", "mixtral":
		return "mistral"
	case "xai", "grok":
		return "xai"
	case "deepseek":
		return "deepseek"
	case "ollama":
		return "ollama"
	default:
		return "other"
	}
}

type SearchConfig struct {
	Provider     string `yaml:"provider"`
	TavilyAPIKey string `yaml:"tavily_api_key"`
}

type PanelConfig struct {
	TotalAgents       int `yaml:"total_agents"`
	CoreCount         int `yaml:"core_count"`
	AdjacentCount     int `yaml:"adjacent_count"`
	PractitionerCount int `yaml:"practitioner_count"`
	LayCount          int `yaml:"lay_count"`
}

type ScoringConfig struct {
	EvidenceWeight float64 `yaml:"evidence_weight"`
	LogicWeight    float64 `yaml:"logic_weight"`
	NoveltyWeight  float64 `yaml:"novelty_weight"`
}

type ContextConfig struct {
	FullDetailRounds  int `yaml:"full_detail_rounds"`
	MaxContextWords   int `yaml:"max_context_words"`
	SummariseOlderThan int `yaml:"summarise_older_than"`
}

type DebateConfig struct {
	DefaultMode         string        `yaml:"default_mode"`
	Panel               PanelConfig   `yaml:"panel"`
	Scoring             ScoringConfig `yaml:"scoring"`
	Context             ContextConfig `yaml:"context"`
	CrossFamilyVerifier *bool         `yaml:"cross_family_verifier"` // default true
}

// CrossFamilyVerifierEnabled returns the on-by-default flag.
func (d DebateConfig) CrossFamilyVerifierEnabled() bool {
	if d.CrossFamilyVerifier == nil {
		return true
	}
	return *d.CrossFamilyVerifier
}

type StorageConfig struct {
	Backend    string `yaml:"backend"`     // "json" or "sqlite"; empty defaults to json
	SQLitePath string `yaml:"sqlite_path"` // path to SQLite file when backend=sqlite
}

type TimeoutConfig struct {
	LLMCompletion time.Duration `yaml:"llm_completion"`
	LLMAnalyzer   time.Duration `yaml:"llm_analyzer"`
	LLMJudge      time.Duration `yaml:"llm_judge"`
	LLMExtractor  time.Duration `yaml:"llm_extractor"`
	Search        time.Duration `yaml:"search"`
	WSPing        time.Duration `yaml:"ws_ping"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

// Load reads config from path. Environment variable KAMPONG_LLM_KEY overrides API key.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Env var overrides
	if key := os.Getenv("KAMPONG_LLM_KEY"); key != "" {
		cfg.LLM.APIKey = key
	}
	if key := os.Getenv("KAMPONG_TAVILY_KEY"); key != "" {
		cfg.Search.TavilyAPIKey = key
	}

	// Defaults
	if cfg.Debate.DefaultMode == "" {
		cfg.Debate.DefaultMode = "deep"
	}
	if cfg.Debate.Panel.TotalAgents == 0 {
		cfg.Debate.Panel.TotalAgents = 6
	}
	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Storage.Backend == "" {
		cfg.Storage.Backend = "json"
	}
	if cfg.Storage.Backend == "sqlite" && cfg.Storage.SQLitePath == "" {
		cfg.Storage.SQLitePath = "data/kampong.db"
	}

	return cfg, nil
}

// Save writes the current config back to disk.
func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// Validate checks required fields.
func (c *Config) Validate() error {
	if c.LLM.BaseURL == "" {
		return fmt.Errorf("llm.base_url is required")
	}
	if c.LLM.Model == "" {
		return fmt.Errorf("llm.model is required")
	}
	return nil
}
