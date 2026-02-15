package daemon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

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

func newTestServer(t *testing.T, upstream http.HandlerFunc) (*Server, *logger.LogStore, func()) {
	t.Helper()
	up := httptest.NewServer(upstream)
	cfg := config.Default()
	cfg.Providers = map[string]config.ProviderConfig{
		"openai": {Upstream: up.URL},
	}
	store, err := logger.Open(filepath.Join(t.TempDir(), "tokfence.db"))
	if err != nil {
		t.Fatalf("open logger: %v", err)
	}
	engine := budget.NewEngine(store.DB())
	srv := NewServer(cfg, &testVault{keys: map[string]string{"openai": "sk-test"}}, store, engine)
	cleanup := func() {
		up.Close()
		store.Close()
	}
	return srv, store, cleanup
}

func TestHandleProxyForwardsInjectsAuthAndLogs(t *testing.T) {
	var authHeader string
	srv, store, cleanup := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
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
	srv, store, cleanup := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
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
	srv, store, cleanup := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
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
	srv, store, cleanup := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
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
