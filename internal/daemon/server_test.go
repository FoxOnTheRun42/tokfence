package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/macfox/tokfence/internal/budget"
	"github.com/macfox/tokfence/internal/config"
	"github.com/macfox/tokfence/internal/logger"
	"github.com/macfox/tokfence/internal/vault"
)

type testVault struct {
	keys map[string]string
}

func (v *testVault) Get(_ context.Context, provider string) (string, error) {
	key, ok := v.keys[provider]
	if !ok {
		return "", vault.ErrKeyNotFound
	}
	return key, nil
}
func (v *testVault) Set(_ context.Context, provider, key string) error {
	v.keys[provider] = key
	return nil
}
func (v *testVault) Delete(_ context.Context, provider string) error {
	delete(v.keys, provider)
	return nil
}
func (v *testVault) List(_ context.Context) ([]string, error) { return nil, nil }

func newTestServer(t *testing.T, provider string, upstream http.HandlerFunc) (*Server, *logger.LogStore, func()) {
	t.Helper()
	up := httptest.NewServer(upstream)
	cfg := config.Default()
	cfg.Providers = map[string]config.ProviderConfig{
		provider: {Upstream: up.URL},
	}
	store, err := logger.Open(filepath.Join(t.TempDir(), "tokfence.db"))
	if err != nil {
		t.Fatalf("open logger: %v", err)
	}
	engine := budget.NewEngine(store.DB())
	srv := NewServer(cfg, &testVault{keys: map[string]string{provider: "sk-test"}}, store, engine)
	cleanup := func() {
		up.Close()
		store.Close()
	}
	return srv, store, cleanup
}

func TestHandleProxyForwardsInjectsAuthAndLogs(t *testing.T) {
	var authHeader string
	srv, store, cleanup := newTestServer(t, "openai", func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"1","usage":{"prompt_tokens":1000000,"completion_tokens":1000000}}`))
	})
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o"}`))
	req.Header.Set("Authorization", "Bearer should-be-stripped")
	rec := httptest.NewRecorder()

	srv.handleProxy(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	if authHeader != "Bearer sk-test" {
		t.Fatalf("upstream auth header = %q, want Bearer sk-test", authHeader)
	}
	rows, err := store.ListRequests(context.Background(), logger.QueryFilter{Limit: 10})
	if err != nil {
		t.Fatalf("ListRequests() error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 log row, got %d", len(rows))
	}
	if rows[0].EstimatedCostCents == 0 {
		t.Fatalf("expected non-zero cost")
	}
}

func TestHandleProxyRevokedProvider(t *testing.T) {
	srv, store, cleanup := newTestServer(t, "openai", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()
	if err := store.SetProviderRevoked(context.Background(), "openai", true); err != nil {
		t.Fatalf("SetProviderRevoked() error = %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o"}`))
	rec := httptest.NewRecorder()
	srv.handleProxy(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	var payload map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &payload)
	errObj := payload["error"].(map[string]any)
	if errObj["type"] != "tokfence_provider_revoked" {
		t.Fatalf("error.type = %v", errObj["type"])
	}
}

func TestHandleProxyBudgetExceededOnSecondRequest(t *testing.T) {
	srv, store, cleanup := newTestServer(t, "openai", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"1","usage":{"prompt_tokens":1000000,"completion_tokens":1000000}}`))
	})
	defer cleanup()
	engine := budget.NewEngine(store.DB())
	if err := engine.SetBudget(context.Background(), "openai", 1.00, "daily"); err != nil {
		t.Fatalf("SetBudget() error = %v", err)
	}
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o"}`))
		rec := httptest.NewRecorder()
		srv.handleProxy(rec, req)
		if i == 0 && rec.Code != http.StatusOK {
			t.Fatalf("first request status = %d, want 200", rec.Code)
		}
		if i == 1 && rec.Code != http.StatusTooManyRequests {
			t.Fatalf("second request status = %d, want 429", rec.Code)
		}
	}
}

func TestHandleProxyRateLimit(t *testing.T) {
	srv, store, cleanup := newTestServer(t, "openai", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"1","usage":{"prompt_tokens":10,"completion_tokens":10}}`))
	})
	defer cleanup()
	if err := store.SetRateLimit(context.Background(), "openai", 1); err != nil {
		t.Fatalf("SetRateLimit() error = %v", err)
	}

	firstReq := httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o"}`))
	firstRec := httptest.NewRecorder()
	srv.handleProxy(firstRec, firstReq)
	if firstRec.Code != http.StatusOK {
		t.Fatalf("first status = %d, want 200", firstRec.Code)
	}

	secondReq := httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o"}`))
	secondRec := httptest.NewRecorder()
	srv.handleProxy(secondRec, secondReq)
	if secondRec.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, want 429", secondRec.Code)
	}
}

func TestHandleProxyAnthropicStripsIncomingAuth(t *testing.T) {
	var gotXAPIKey string
	var gotAuth string
	srv, _, cleanup := newTestServer(t, "anthropic", func(w http.ResponseWriter, r *http.Request) {
		gotXAPIKey = r.Header.Get("x-api-key")
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"1","usage":{"input_tokens":10,"output_tokens":5}}`))
	})
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/anthropic/v1/messages", strings.NewReader(`{"model":"claude-sonnet-4-5-20250514"}`))
	req.Header.Set("Authorization", "Bearer leaked-client-key")
	req.Header.Set("x-api-key", "leaked-anthropic-key")
	rec := httptest.NewRecorder()
	srv.handleProxy(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	if gotXAPIKey != "sk-test" {
		t.Fatalf("upstream x-api-key = %q, want sk-test", gotXAPIKey)
	}
	if gotAuth != "" {
		t.Fatalf("upstream Authorization should be stripped for anthropic, got %q", gotAuth)
	}
}

func TestHandleProxyStreamingPassthroughFlushesAndLogsUsage(t *testing.T) {
	srv, store, cleanup := newTestServer(t, "openai", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatalf("upstream writer does not implement flusher")
		}
		_, _ = io.WriteString(w, "data: {\"message\":{\"usage\":{\"prompt_tokens\":1000000}}}\n\n")
		flusher.Flush()
		time.Sleep(700 * time.Millisecond)
		_, _ = io.WriteString(w, "data: {\"usage\":{\"completion_tokens\":2000000}}\n\n")
		flusher.Flush()
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
		flusher.Flush()
	})
	defer cleanup()

	downstream := httptest.NewServer(http.HandlerFunc(srv.handleProxy))
	defer downstream.Close()

	reqBody := `{"model":"gpt-4o","stream":true,"messages":[{"role":"user","content":"hi"}]}`
	req, err := http.NewRequest(http.MethodPost, downstream.URL+"/openai/v1/chat/completions", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := downstream.Client().Do(req)
	if err != nil {
		t.Fatalf("send request: %v", err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("Content-Type"); !strings.Contains(got, "text/event-stream") {
		t.Fatalf("content-type = %q, want text/event-stream", got)
	}

	reader := bufio.NewReader(resp.Body)
	firstDataLineAt := time.Time{}
	for {
		line, readErr := reader.ReadString('\n')
		if strings.HasPrefix(strings.TrimSpace(line), "data:") {
			firstDataLineAt = time.Now()
			break
		}
		if readErr != nil {
			t.Fatalf("failed before first SSE line: %v", readErr)
		}
	}

	if firstDataLineAt.IsZero() {
		t.Fatalf("did not receive first SSE data line")
	}
	ttfb := firstDataLineAt.Sub(start)
	if ttfb > 450*time.Millisecond {
		t.Fatalf("first SSE chunk arrived too late (%s), likely buffered", ttfb)
	}

	_, _ = io.Copy(io.Discard, reader)
	time.Sleep(50 * time.Millisecond)

	rows, err := store.ListRequests(context.Background(), logger.QueryFilter{Limit: 5, Provider: "openai"})
	if err != nil {
		t.Fatalf("ListRequests() error = %v", err)
	}
	if len(rows) == 0 {
		t.Fatalf("expected at least 1 log row")
	}
	row := rows[0]
	if !row.IsStreaming {
		t.Fatalf("expected IsStreaming=true")
	}
	if row.TTFTMS <= 0 {
		t.Fatalf("expected positive TTFTMS, got %d", row.TTFTMS)
	}
	if row.InputTokens != 1000000 {
		t.Fatalf("input tokens = %d, want 1000000", row.InputTokens)
	}
	if row.OutputTokens != 2000000 {
		t.Fatalf("output tokens = %d, want 2000000", row.OutputTokens)
	}
	if row.EstimatedCostCents <= 0 {
		t.Fatalf("expected non-zero estimated cost")
	}
}

func TestFollowLogFiltersAndLatestEntry(t *testing.T) {
	srv, store, cleanup := newTestServer(t, "openai", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"1","usage":{"prompt_tokens":25,"completion_tokens":10}}`))
	})
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o"}`))
	rec := httptest.NewRecorder()
	srv.handleProxy(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	rows, err := store.ListRequests(context.Background(), logger.QueryFilter{
		Limit:    10,
		Provider: "openai",
		Model:    "gpt-4o",
		Since:    time.Now().Add(-time.Hour),
	})
	if err != nil {
		t.Fatalf("ListRequests with filters error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 filtered row, got %d", len(rows))
	}
	if rows[0].Provider != "openai" || rows[0].Model != "gpt-4o" {
		t.Fatalf("unexpected filtered row: provider=%s model=%s", rows[0].Provider, rows[0].Model)
	}

	statsRows, err := store.Stats(context.Background(), logger.StatsFilter{
		Provider: "openai",
		Since:    time.Now().Add(-time.Hour),
		By:       "provider",
	})
	if err != nil {
		t.Fatalf("Stats with filter error = %v", err)
	}
	if len(statsRows) != 1 {
		t.Fatalf("expected 1 stats row, got %d", len(statsRows))
	}
	if statsRows[0].RequestCount != 1 {
		t.Fatalf("request count = %d, want 1", statsRows[0].RequestCount)
	}
}
