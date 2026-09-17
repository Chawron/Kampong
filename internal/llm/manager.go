package llm

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kampong/debate/internal/config"
)

// ProviderClient wraps a Client with provider metadata.
type ProviderClient struct {
	*Client
	Name           string
	VisionModel    string
	ProviderFamily string // resolved vendor family (openai, anthropic, google, ...)
	tokenCount     atomic.Int64 // total tokens seen through this provider (rough estimate)
}

// ClientManager manages multiple LLM clients for different providers.
type ClientManager struct {
	clients map[string]*ProviderClient
	mu      sync.RWMutex
	timeout time.Duration
}

// NewClientManager creates a new client manager from config.
func NewClientManager(cfg *config.Config) *ClientManager {
	cm := &ClientManager{
		clients: make(map[string]*ProviderClient),
		timeout: cfg.Timeouts.LLMCompletion,
	}

	// Add legacy single LLM config as "default" provider
	if cfg.LLM.BaseURL != "" {
		cm.clients["default"] = &ProviderClient{
			Client:         NewClient(cfg.LLM.BaseURL, cfg.LLM.APIKey, cfg.LLM.Model, cm.timeout),
			Name:           "default",
			ProviderFamily: inferFamilyFromLegacy(cfg.LLM.BaseURL, cfg.LLM.Model),
		}
	}

	// Add all configured providers
	for _, p := range cfg.Providers {
		if !p.Enabled {
			continue
		}
		cm.clients[p.Name] = &ProviderClient{
			Client:         NewClient(p.BaseURL, p.APIKey, p.Model, cm.timeout),
			Name:           p.Name,
			VisionModel:    p.VisionModel,
			ProviderFamily: p.ResolvedFamily(),
		}
	}

	return cm
}

func inferFamilyFromLegacy(baseURL, model string) string {
	lower := strings.ToLower(baseURL + " " + model)
	switch {
	case strings.Contains(lower, "anthropic"):
		return "anthropic"
	case strings.Contains(lower, "google"), strings.Contains(lower, "gemini"):
		return "google"
	case strings.Contains(lower, "openai"), strings.Contains(lower, "gpt-"):
		return "openai"
	case strings.Contains(lower, "ollama"):
		return "ollama"
	default:
		return "other"
	}
}

// GetClient returns the client for a given provider name.
// Falls back to "default" if provider not found.
func (cm *ClientManager) GetClient(providerName string) (*ProviderClient, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if providerName == "" {
		providerName = "default"
	}

	client, ok := cm.clients[providerName]
	if !ok {
		// Fallback to default
		client, ok = cm.clients["default"]
		if !ok {
			return nil, fmt.Errorf("no LLM client available for provider %q", providerName)
		}
	}

	return client, nil
}

// AddProvider adds or updates a provider at runtime.
func (cm *ClientManager) AddProvider(cfg config.ProviderConfig) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.clients[cfg.Name] = &ProviderClient{
		Client:         NewClient(cfg.BaseURL, cfg.APIKey, cfg.Model, cm.timeout),
		Name:           cfg.Name,
		VisionModel:    cfg.VisionModel,
		ProviderFamily: cfg.ResolvedFamily(),
	}
}

// RemoveProvider removes a provider.
func (cm *ClientManager) RemoveProvider(name string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	delete(cm.clients, name)
}

// UpdateProviderKey updates the API key for a specific provider.
func (cm *ClientManager) UpdateProviderKey(name, apiKey string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if entry, ok := cm.clients[name]; ok {
		entry.Client.SetAPIKey(apiKey)
	}
}

// ListProviders returns all configured provider names.
func (cm *ClientManager) ListProviders() []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	names := make([]string, 0, len(cm.clients))
	for name := range cm.clients {
		names = append(names, name)
	}
	return names
}

// HasVisionProvider returns true if any provider has vision capability.
// Also returns true if the default model is known to support vision (e.g. gpt-4o).
func (cm *ClientManager) HasVisionProvider() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	for _, c := range cm.clients {
		if c.VisionModel != "" {
			return true
		}
	}
	// Fallback: check if default client uses a vision-capable model
	if def, ok := cm.clients["default"]; ok {
		if isVisionModel(def.model) {
			return true
		}
	}
	return false
}

// GetVisionClient returns a client that supports vision, or error if none available.
// Falls back to the default client if its model supports vision (e.g. gpt-4o).
func (cm *ClientManager) GetVisionClient() (*ProviderClient, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// First: look for an explicit vision model
	for _, c := range cm.clients {
		if c.VisionModel != "" {
			return c, nil
		}
	}
	// Fallback: use default client if its model supports vision
	if def, ok := cm.clients["default"]; ok {
		if isVisionModel(def.model) {
			return def, nil
		}
	}
	return nil, fmt.Errorf("no vision-capable provider configured")
}

// GetBestClient returns the first provider client with a non-empty API key,
// or nil if no provider has credentials.
func (cm *ClientManager) GetBestClient() *ProviderClient {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	for _, c := range cm.clients {
		if c.HasAPIKey() {
			return c
		}
	}
	return nil
}

// FailoverCandidates returns providers to try when the provider named
// `excludeName` fails hard (after its own retries are exhausted). Ordering:
// different-family providers first (preserves vendor diversity), then
// same-family providers, then the legacy "default" client last.
func (cm *ClientManager) FailoverCandidates(excludeName string) []*ProviderClient {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	excluded := cm.clients[excludeName]
	var diffFam, sameFam []*ProviderClient
	for name, c := range cm.clients {
		if name == excludeName || name == "default" {
			continue
		}
		if !c.HasAPIKey() {
			continue
		}
		if excluded == nil || c.ProviderFamily != excluded.ProviderFamily {
			diffFam = append(diffFam, c)
		} else {
			sameFam = append(sameFam, c)
		}
	}
	out := make([]*ProviderClient, 0, len(diffFam)+len(sameFam)+1)
	out = append(out, diffFam...)
	out = append(out, sameFam...)
	if def, ok := cm.clients["default"]; ok && def.HasAPIKey() {
		out = append(out, def)
	}
	return out
}

// PickOutOfFamilyProvider returns an enabled provider whose family is NOT
// represented by any agent in `agents`. Used by the cross-family verifier.
//
// The "default" legacy client is skipped (its family is rarely a real vendor
// family and using it would defeat the purpose of cross-family review).
//
// Deterministic ordering: providers come back in the order they were added to
// the manager. The first match wins.
func (cm *ClientManager) PickOutOfFamilyProvider(agents []AgentFamilySource) (*ProviderClient, error) {
	used := make(map[string]bool)
	for _, a := range agents {
		f := a.Family()
		if f != "" {
			used[f] = true
		}
	}
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	for _, c := range cm.clients {
		if c.Name == "default" {
			continue
		}
		if !c.HasAPIKey() {
			continue
		}
		if c.ProviderFamily == "" {
			continue
		}
		if !used[c.ProviderFamily] {
			return c, nil
		}
	}
	return nil, fmt.Errorf("no out-of-family provider available (all enabled providers come from the same families as the agents)")
}

// AgentFamilySource is the minimal interface PickOutOfFamilyProvider needs.
// models.Agent satisfies it via its Family() method; tests can pass stubs.
type AgentFamilySource interface {
	Family() string
}

// RecordTokens adds n tokens to the running counter for this provider.
// Cheap atomic op; safe from any goroutine.
func (c *ProviderClient) RecordTokens(n int) {
	if c == nil || n <= 0 {
		return
	}
	c.tokenCount.Add(int64(n))
}

// Tokens returns the running total for this provider.
func (c *ProviderClient) Tokens() int64 {
	if c == nil {
		return 0
	}
	return c.tokenCount.Load()
}

// TokensByProvider returns a snapshot of every provider's token total.
func (cm *ClientManager) TokensByProvider() map[string]int64 {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	out := make(map[string]int64, len(cm.clients))
	for _, c := range cm.clients {
		out[c.Name] = c.Tokens()
	}
	return out
}

// isVisionModel checks if a model name is known to support vision/multimodal input.
func isVisionModel(model string) bool {
	model = strings.ToLower(model)
	for _, prefix := range []string{"gpt-4o", "gpt-4v", "gpt-4.1", "claude-3", "claude-sonnet-4", "claude-opus-4", "gemini-1.5", "gemini-2", "qwen-vl", "qwen2.5-vl"} {
		if strings.Contains(model, prefix) {
			return true
		}
	}
	return false
}
