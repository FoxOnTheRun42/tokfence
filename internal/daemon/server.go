package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/macfox/tokfence/internal/budget"
	"github.com/macfox/tokfence/internal/config"
	"github.com/macfox/tokfence/internal/logger"
	"github.com/macfox/tokfence/internal/process"
	"github.com/macfox/tokfence/internal/proxy"
	"github.com/macfox/tokfence/internal/vault"
)

const (
	defaultRequestBodyLimit = 8 * 1024 * 1024
)

func maxRequestBodyLimit() int64 {
	limit := int64(defaultRequestBodyLimit)
	if raw := strings.TrimSpace(os.Getenv("TOKFENCE_MAX_REQUEST_BODY_BYTES")); raw != "" {
		if parsed, err := strconv.ParseInt(raw, 10, 64); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	return limit
}

type Server struct {
	cfg               config.Config
	vault             vault.Vault
	store             *logger.LogStore
	budget            *budget.Engine
	limiter           *RateLimiter
	httpSrv           *http.Server
	httpClient        *http.Client
	logger            *slog.Logger
	startedAt         time.Time
	isRunning         atomic.Bool
	maxRequestBodyRaw int64
}

func NewServer(cfg config.Config, v vault.Vault, store *logger.LogStore, engine *budget.Engine) *Server {
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	return &Server{
		cfg:               cfg,
		vault:             v,
		store:             store,
		budget:            engine,
		limiter:           NewRateLimiter(),
		httpClient:        &http.Client{Transport: transport},
		logger:            slog.New(slog.NewJSONHandler(os.Stderr, nil)),
		maxRequestBodyRaw: maxRequestBodyLimit(),
	}
}

func (s *Server) Addr() string {
	return fmt.Sprintf("%s:%d", s.cfg.Daemon.Host, s.cfg.Daemon.Port)
}

func (s *Server) Run(ctx context.Context) error {
	if err := s.store.DeleteOlderThan(ctx, s.cfg.Logging.RetentionDays); err != nil {
		s.logger.Warn("failed to clean old logs", "error", err)
	}
	s.startedAt = time.Now().UTC()
	mux := http.NewServeMux()
	mux.HandleFunc("/__tokfence/health", s.handleHealth)
	mux.HandleFunc("/", s.handleProxy)

	s.httpSrv = &http.Server{
		Addr:              s.Addr(),
		Handler:           mux,
		ReadHeaderTimeout: 15 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      10 * time.Minute,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		s.isRunning.Store(true)
		err := s.httpSrv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = s.Shutdown(shutdownCtx)
		return ctx.Err()
	case err := <-errCh:
		s.isRunning.Store(false)
		if err != nil {
			return fmt.Errorf("start http server: %w", err)
		}
		return nil
	}
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpSrv == nil {
		return nil
	}
	err := s.httpSrv.Shutdown(ctx)
	s.isRunning.Store(false)
	if err != nil {
		return fmt.Errorf("shutdown server: %w", err)
	}
	return nil
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = jsonResponse(w, map[string]any{
		"name":       "tokfence",
		"status":     "ok",
		"addr":       s.Addr(),
		"started_at": s.startedAt.Format(time.RFC3339),
	})
}

func jsonResponse(w http.ResponseWriter, payload any) error {
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(payload)
}

func (s *Server) handleProxy(w http.ResponseWriter, r *http.Request) {
	requestID := ulid.Make().String()
	w.Header().Set("X-Tokfence-Request-ID", requestID)

	ctx := r.Context()
	started := time.Now()

	route, err := proxy.ResolveRoute(s.cfg, r.URL.Path, r.URL.RawQuery)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, errorPayload{
			Type:      "tokfence_invalid_route",
			Message:   err.Error(),
			RequestID: requestID,
		}, map[string]any{})
		return
	}

	revoked, err := s.store.IsProviderRevoked(ctx, route.Provider)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, errorPayload{
			Type:      "tokfence_status_lookup_failed",
			Message:   err.Error(),
			RequestID: requestID,
			Provider:  route.Provider,
		}, nil)
		return
	}
	if revoked {
		writeProviderRevoked(w, requestID, route.Provider)
		return
	}

	exceeded, err := s.budget.CheckLimit(ctx, route.Provider)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, errorPayload{
			Type:      "tokfence_budget_check_failed",
			Message:   err.Error(),
			RequestID: requestID,
			Provider:  route.Provider,
		}, nil)
		return
	}
	if exceeded != nil {
		writeBudgetExceeded(w, requestID, exceeded.Provider, exceeded.BudgetLimit, exceeded.CurrentSpend, exceeded.ResetsAt)
		return
	}

	rpm, err := s.store.GetRateLimit(ctx, route.Provider)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, errorPayload{
			Type:      "tokfence_ratelimit_lookup_failed",
			Message:   err.Error(),
			RequestID: requestID,
			Provider:  route.Provider,
		}, nil)
		return
	}
	if !s.limiter.Allow(route.Provider, rpm) {
		retryAfter := 1
		writeRateLimitExceeded(w, requestID, route.Provider, rpm, &retryAfter)
		return
	}

	limitedBody := http.MaxBytesReader(w, r.Body, s.maxRequestBodyRaw)
	requestBody, err := io.ReadAll(limitedBody)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeRequestTooLarge(w, requestID, s.maxRequestBodyRaw)
			return
		}
		writeReadBodyFailed(w, requestID, err)
		return
	}

	isStreaming := proxy.IsStreamingJSON(requestBody)
	model := logger.ExtractModelFromRequest(requestBody)
	requestHash := logger.RequestHash(requestBody)
	identity := process.Identify(ctx, r)

	upstreamReq, err := http.NewRequestWithContext(ctx, r.Method, route.ForwardedURL.String(), bytes.NewReader(requestBody))
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, errorPayload{
			Type:      "tokfence_request_build_failed",
			Message:   err.Error(),
			RequestID: requestID,
			Provider:  route.Provider,
		}, nil)
		return
	}
	upstreamReq.Header = cloneHeaders(r.Header)
	proxy.StripIncomingAuth(upstreamReq.Header)
	upstreamReq.Header.Set("X-Tokfence-Request-ID", requestID)

	key, err := s.lookupProviderKey(ctx, route.Provider)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, errorPayload{
			Type:      "tokfence_missing_api_key",
			Message:   err.Error(),
			RequestID: requestID,
			Provider:  route.Provider,
		}, nil)
		return
	}
	if err := proxy.ApplyProviderAuth(upstreamReq.Header, route.Provider, key); err != nil {
		writeJSONError(w, http.StatusUnauthorized, errorPayload{
			Type:      "tokfence_auth_injection_failed",
			Message:   err.Error(),
			RequestID: requestID,
			Provider:  route.Provider,
		}, nil)
		return
	}
	if providerCfg, ok := s.cfg.Providers[route.Provider]; ok {
		for headerName, headerValue := range providerCfg.ExtraHeaders {
			upstreamReq.Header.Set(headerName, headerValue)
		}
	}

	resp, err := s.httpClient.Do(upstreamReq)
	if err != nil {
		writeUpstreamError(w, requestID, route.Provider, "tokfence_upstream_request_failed", err.Error())
		s.logRequest(ctx, logger.RequestRecord{
			ID:           requestID,
			Timestamp:    time.Now().UTC(),
			Provider:     route.Provider,
			Model:        model,
			Endpoint:     route.Path,
			Method:       r.Method,
			StatusCode:   http.StatusBadGateway,
			LatencyMS:    int(time.Since(started).Milliseconds()),
			CallerPID:    identity.PID,
			CallerName:   identity.Name,
			IsStreaming:  isStreaming,
			ErrorType:    "upstream_request_failed",
			ErrorMessage: err.Error(),
			RequestHash:  requestHash,
		})
		return
	}
	defer resp.Body.Close()

	copyHeaders(w.Header(), resp.Header)
	responseCapture := bytes.NewBuffer(nil)
	statusCode := resp.StatusCode
	ttftMs := 0

	if isStreaming || proxy.IsSSEContentType(resp.Header.Get("Content-Type")) {
		isStreaming = true
		w.WriteHeader(statusCode)
		flusher, _ := w.(http.Flusher)
		if flusher != nil {
			flusher.Flush()
		}
		firstChunkAt := time.Time{}
		if _, readErr := proxy.CopySSE(w, resp.Body, flusher, responseCapture, func(chunk []byte) {
			if firstChunkAt.IsZero() {
				firstChunkAt = time.Now()
				ttftMs = int(firstChunkAt.Sub(started).Milliseconds())
			}
		}); readErr != nil && !errors.Is(readErr, context.Canceled) {
			s.logger.Warn("stream copy failed", "error", readErr)
		}
	} else {
		responseBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			s.logger.Warn("non-stream read failed", "error", readErr)
		}
		_, _ = responseCapture.Write(responseBody)
		w.WriteHeader(statusCode)
		if responseCapture.Len() > 0 {
			_, _ = w.Write(responseCapture.Bytes())
		}
	}

	usage := logger.TokenUsage{}
	if isStreaming {
		usage = logger.ExtractUsageFromSSE(route.Provider, responseCapture.Bytes())
	} else {
		usage = logger.ExtractUsageFromResponse(route.Provider, responseCapture.Bytes())
	}

	errorType := ""
	errorMessage := ""
	if statusCode >= 400 {
		errorType, errorMessage = logger.ExtractErrorFromBody(responseCapture.Bytes())
	}

	estimatedCost := budget.EstimateCostCents(model, usage.InputTokens, usage.OutputTokens)
	record := logger.RequestRecord{
		ID:                  requestID,
		Timestamp:           time.Now().UTC(),
		Provider:            route.Provider,
		Model:               model,
		Endpoint:            route.Path,
		Method:              r.Method,
		InputTokens:         usage.InputTokens,
		OutputTokens:        usage.OutputTokens,
		CacheReadTokens:     usage.CacheReadTokens,
		CacheCreationTokens: usage.CacheCreationTokens,
		EstimatedCostCents:  estimatedCost,
		StatusCode:          statusCode,
		LatencyMS:           int(time.Since(started).Milliseconds()),
		TTFTMS:              ttftMs,
		CallerPID:           identity.PID,
		CallerName:          identity.Name,
		IsStreaming:         isStreaming,
		ErrorType:           errorType,
		ErrorMessage:        errorMessage,
		RequestHash:         requestHash,
	}
	s.logRequest(ctx, record)
	if statusCode < 400 {
		if err := s.budget.AddSpend(ctx, route.Provider, estimatedCost); err != nil {
			s.logger.Warn("failed to add budget spend", "error", err)
		}
	}
}

func (s *Server) lookupProviderKey(ctx context.Context, provider string) (string, error) {
	key, err := s.vault.Get(ctx, provider)
	if err == nil && strings.TrimSpace(key) != "" {
		return key, nil
	}
	fallbackEnv := map[string]string{
		"anthropic":  "ANTHROPIC_API_KEY",
		"openai":     "OPENAI_API_KEY",
		"google":     "GOOGLE_API_KEY",
		"mistral":    "MISTRAL_API_KEY",
		"openrouter": "OPENROUTER_API_KEY",
		"groq":       "GROQ_API_KEY",
	}
	if envName, ok := fallbackEnv[provider]; ok {
		if fallback := strings.TrimSpace(os.Getenv(envName)); fallback != "" {
			return fallback, nil
		}
	}
	if err != nil {
		return "", err
	}
	return "", fmt.Errorf("no key configured for provider %s", provider)
}

func (s *Server) logRequest(ctx context.Context, record logger.RequestRecord) {
	if err := s.store.LogRequest(ctx, record); err != nil {
		s.logger.Warn("failed to store request log", "error", err)
	}
}

func copyHeaders(dst, src http.Header) {
	for key, values := range src {
		dst.Del(key)
		for _, v := range values {
			dst.Add(key, v)
		}
	}
}

func cloneHeaders(src http.Header) http.Header {
	out := make(http.Header, len(src))
	for key, values := range src {
		copied := make([]string, len(values))
		copy(copied, values)
		out[key] = copied
	}
	return out
}
