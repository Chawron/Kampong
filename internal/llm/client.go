package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Message represents a single chat message.
type Message struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // Can be string or []ContentPart for multimodal
}

// ContentPart represents a part of multimodal content.
type ContentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

// ImageURL represents an image URL for vision models.
type ImageURL struct {
	URL string `json:"url"`
}

// ChatRequest is the OpenAI-compatible request body.
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Stream      bool      `json:"stream"`
}

// chatResponse is a non-streaming response.
type chatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

// streamChunk is a single SSE chunk from the streaming API.
type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

// Client is an OpenAI-compatible LLM client.
//
// Ollama note: Ollama serves an OpenAI-compat shim at
// `http://localhost:11434/v1/chat/completions`. Use NewClient with that URL
// and any model name you've pulled (e.g. "llama3.1", "qwen2.5", "mistral").
// The api_key field is required by NewClient but Ollama ignores it — pass "ollama".
type Client struct {
	mu         sync.RWMutex
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewClient creates a new LLM client.
func NewClient(baseURL, apiKey, model string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// SetAPIKey updates the API key used for authentication.
func (c *Client) SetAPIKey(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.apiKey = key
}

// HasAPIKey reports whether an API key is set.
func (c *Client) HasAPIKey() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.apiKey != ""
}

// APIKey returns the current API key (read-only; safe to call concurrently).
func (c *Client) APIKey() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.apiKey
}

// BaseURL returns the current base URL.
func (c *Client) BaseURL() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.baseURL
}

// Model returns the current model name.
func (c *Client) Model() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.model
}

// SetBaseURL updates the API endpoint URL.
func (c *Client) SetBaseURL(url string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.baseURL = strings.TrimRight(url, "/")
}

// SetModel updates the model name.
func (c *Client) SetModel(model string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.model = model
}

// snapshot reads the current URL / key / model under the read lock — keeps the
// request payload consistent even if a writer mutates fields mid-call.
func (c *Client) snapshot() (baseURL, apiKey, model string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.baseURL, c.apiKey, c.model
}

// Complete sends a non-streaming chat completion request.
// Transient failures (rate limits, 5xx, connection errors) are retried with
// exponential backoff (up to 3 attempts total).
func (c *Client) Complete(ctx context.Context, messages []Message, temperature float64, maxTokens int) (string, error) {
	const maxAttempts = 3
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			if err := sleepBackoff(ctx, attempt-1); err != nil {
				return "", err
			}
		}
		out, err := c.completeOnce(ctx, messages, temperature, maxTokens)
		if err == nil {
			return out, nil
		}
		lastErr = err
		if !isRetryable(err) {
			break
		}
		log.Printf("LLM RETRY attempt %d/%d: %v", attempt+1, maxAttempts, err)
	}
	return "", lastErr
}

// completeOnce performs a single non-streaming chat completion request.
func (c *Client) completeOnce(ctx context.Context, messages []Message, temperature float64, maxTokens int) (string, error) {
	baseURL, apiKey, model := c.snapshot()
	req := ChatRequest{
		Model:       model,
		Messages:    messages,
		Temperature: temperature,
		MaxTokens:   maxTokens,
		Stream:      false,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", retryable(fmt.Errorf("http request: %w", err))
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", retryable(fmt.Errorf("read response: %w", err))
	}

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("llm error (status %d): %s", resp.StatusCode, string(respBody))
		if isRetryableStatus(resp.StatusCode) {
			return "", retryable(err)
		}
		return "", err
	}

	var cr chatResponse
	if err := json.Unmarshal(respBody, &cr); err != nil {
		return "", fmt.Errorf("unmarshal response: %w (body: %s)", err, string(respBody)[:min(len(respBody), 500)])
	}

	if cr.Error != nil {
		return "", fmt.Errorf("llm api error: %s", cr.Error.Message)
	}

	if len(cr.Choices) == 0 {
		return "", fmt.Errorf("no choices in response (body: %s)", string(respBody)[:min(len(respBody), 500)])
	}

	content := cr.Choices[0].Message.Content
	
	// Handle both string and multimodal content
	switch v := content.(type) {
	case string:
		if v == "" {
			return "", fmt.Errorf("empty content in response (body: %s)", string(respBody)[:min(len(respBody), 500)])
		}
		return v, nil
	case []interface{}:
		// For multimodal responses, extract text content
		var textContent string
		for _, part := range v {
			if partMap, ok := part.(map[string]interface{}); ok {
				if partMap["type"] == "text" {
					if text, ok := partMap["text"].(string); ok {
						textContent += text
					}
				}
			}
		}
		if textContent == "" {
			return "", fmt.Errorf("no text content in multimodal response")
		}
		return textContent, nil
	default:
		return "", fmt.Errorf("unexpected content type in response")
	}
}

// ── Retry helpers ──

// retryableError marks an error as transient/retryable.
type retryableError struct{ err error }

func (e *retryableError) Error() string { return e.err.Error() }
func (e *retryableError) Unwrap() error { return e.err }

func retryable(err error) error { return &retryableError{err: err} }

func isRetryable(err error) bool {
	var re *retryableError
	return errors.As(err, &re)
}

// isRetryableStatus reports whether an HTTP status warrants a retry.
func isRetryableStatus(code int) bool {
	switch code {
	case http.StatusTooManyRequests, // 429
		http.StatusInternalServerError, // 500
		http.StatusBadGateway,          // 502
		http.StatusServiceUnavailable,  // 503
		http.StatusGatewayTimeout:      // 504
		return true
	}
	return false
}

// sleepBackoff waits the exponential-backoff delay for the given attempt
// (0-based), returning early when ctx is cancelled.
func sleepBackoff(ctx context.Context, attempt int) error {
	base := time.Duration(1<<uint(attempt)) * time.Second
	if base > 8*time.Second {
		base = 8 * time.Second
	}
	jitter := time.Duration(rand.Int63n(int64(base/2) + 1))
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(base + jitter):
		return nil
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Stream sends a streaming chat completion request, calling onToken for each chunk.
// Transient failures are retried only when zero tokens were emitted (retrying
// mid-stream would duplicate already-delivered tokens).
func (c *Client) Stream(ctx context.Context, messages []Message, temperature float64, maxTokens int, onToken func(token string) error) (string, error) {
	const maxAttempts = 3
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			if err := sleepBackoff(ctx, attempt-1); err != nil {
				return "", err
			}
		}
		emitted := 0
		out, err := c.streamOnce(ctx, messages, temperature, maxTokens, func(token string) error {
			emitted++
			if onToken != nil {
				return onToken(token)
			}
			return nil
		})
		if err == nil {
			return out, nil
		}
		lastErr = err
		// Never retry once tokens have been delivered (duplicate output) or
		// when the error is not transient.
		if emitted > 0 || !isRetryable(err) {
			break
		}
		log.Printf("LLM STREAM RETRY attempt %d/%d: %v", attempt+1, maxAttempts, err)
	}
	return "", lastErr
}

// streamOnce performs a single streaming chat completion request.
func (c *Client) streamOnce(ctx context.Context, messages []Message, temperature float64, maxTokens int, onToken func(token string) error) (string, error) {
	baseURL, apiKey, model := c.snapshot()
	req := ChatRequest{
		Model:       model,
		Messages:    messages,
		Temperature: temperature,
		MaxTokens:   maxTokens,
		Stream:      true,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	if apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", retryable(fmt.Errorf("http request: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		err := fmt.Errorf("llm error (status %d): %s", resp.StatusCode, string(respBody))
		if isRetryableStatus(resp.StatusCode) {
			return "", retryable(err)
		}
		return "", err
	}

	var fullText strings.Builder

	// Scan line-by-line so SSE events that span read-chunk boundaries are
	// never dropped (a naive byte-buffer split can cut a line in half).
	scanner := bufio.NewScanner(resp.Body)
	// Providers occasionally send large chunks; allow up to 1MB lines.
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return fullText.String(), ctx.Err()
		default:
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}

		jsonStr := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if jsonStr == "[DONE]" {
			return fullText.String(), nil
		}

		var chunk streamChunk
		if err := json.Unmarshal([]byte(jsonStr), &chunk); err != nil {
			continue // skip malformed chunks
		}

		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				fullText.WriteString(choice.Delta.Content)
				if onToken != nil {
					if err := onToken(choice.Delta.Content); err != nil {
						return fullText.String(), err
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fullText.String(), ctxErr
		}
		return fullText.String(), fmt.Errorf("read stream: %w", err)
	}
	return fullText.String(), nil
}
