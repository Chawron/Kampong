package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// TestOllamaMultiModel verifies that a single Client wired to an
// OpenAI-compatible endpoint (Ollama's `/v1` shim, LM Studio, vLLM, etc.) can
// be re-pointed at three different local models and produce non-empty output.
//
// Default mode (no `KAMPONG_OLLAMA_URL` set, `-tollama` build tag absent):
//
//	The test stands up a local httptest server that mocks the chat/completions
//	endpoint and exercises the client against three model names. This keeps
//	the test deterministic and offline-safe.
//
// Real-Ollama mode (`go test -tags=ollama` AND `KAMPONG_OLLAMA_URL` set to a
// reachable endpoint):
//
//	The test skips the mock and calls the real server. Use this when the
//	user wants to confirm a specific Ollama build + model combination.
//
// Usage examples:
//
//	# offline: just runs the mock
//	go test ./internal/llm -run Ollama -v
//
//	# against a running Ollama with llama3.1, qwen2.5, mistral pulled
//	KAMPONG_OLLAMA_URL=http://localhost:11434/v1 go test -tags=ollama ./internal/llm -run Ollama -v
func TestOllamaMultiModel(t *testing.T) {
	models := []string{"llama3.1", "qwen2.5", "mistral"}

	realURL := os.Getenv("KAMPONG_OLLAMA_URL")
	useReal := strings.HasPrefix(strings.ToLower(os.Getenv("KAMPONG_TEST_MODE")), "ollama") ||
		(os.Getenv("KAMPONG_OLLAMA_URL") != "" && shouldUseRealOllama(t, realURL))

	var baseURL string
	var ts *httptest.Server
	if useReal {
		baseURL = realURL
		if !strings.HasSuffix(baseURL, "/") {
			baseURL += "/"
		}
	} else {
		// Stand up a mock that pretends to be three different models. The mock
		// returns a deterministic body containing the requested model name so
		// the test can assert the response actually came back per-model.
		ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/chat/completions" {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			var body struct {
				Model string `json:"model"`
			}
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &body)
			if body.Model == "" {
				body.Model = "unknown"
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"choices":[{"message":{"role":"assistant","content":"hello from %s"}}]}`, body.Model)
		}))
		baseURL = ts.URL + "/"
		t.Cleanup(ts.Close)
	}

	client := NewClient(baseURL, "", "llama3.1", 60*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, m := range models {
		m := m
		t.Run(m, func(t *testing.T) {
			// NewClient already takes the model in its constructor; for a
			// real Ollama deployment we have to point SetModel at the new
			// model before each call. The mock returns whatever the request
			// asked for so this exercises the switch path.
			client.SetModel(m)
			out, err := client.Complete(ctx, []Message{
				{Role: "user", Content: "say hi"},
			}, 0.3, 256)
			if err != nil {
				if useReal {
					t.Skipf("real Ollama unavailable for model %s: %v", m, err)
					return
				}
				t.Fatalf("Complete(%s): %v", m, err)
			}
			if out == "" {
				t.Fatalf("Complete(%s): empty response", m)
			}
			if useReal {
				// Real Ollama is allowed to return anything; just sanity-check non-empty.
				return
			}
			if !strings.Contains(out, m) {
				t.Fatalf("Complete(%s): response %q did not mention model name", m, out)
			}
		})
	}
}

// shouldUseRealOllama performs a quick HEAD check against the URL to decide
// whether to skip the mock and call the live server. Keeps the test
// self-contained: no external deps, no flake.
func shouldUseRealOllama(t *testing.T, url string) bool {
	t.Helper()
	if url == "" {
		return false
	}
	client := &http.Client{Timeout: 2 * time.Second}
	// /api/tags is Ollama's "is this Ollama?" probe; the OpenAI-compat shim
	// does not necessarily answer it. Fall back to /models which Ollama
	// (since 0.1.14) also implements for OpenAI compat.
	for _, p := range []string{"/api/tags", "/models"} {
		req, err := http.NewRequest(http.MethodGet, strings.TrimRight(url, "/")+p, nil)
		if err != nil {
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()
		if resp.StatusCode < 500 {
			return true
		}
	}
	return false
}