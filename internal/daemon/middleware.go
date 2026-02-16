package daemon

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sync"
	"time"
)

type tokenBucket struct {
	capacity   float64
	tokens     float64
	ratePerSec float64
	lastRefill time.Time
}

type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*tokenBucket
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{buckets: map[string]*tokenBucket{}}
}

func (r *RateLimiter) Allow(provider string, rpm int) bool {
	if rpm <= 0 {
		return true
	}
	now := time.Now()
	ratePerSec := float64(rpm) / 60.0
	capacity := math.Max(1.0, float64(rpm))

	r.mu.Lock()
	defer r.mu.Unlock()
	bucket, ok := r.buckets[provider]
	if !ok || bucket.capacity != capacity || bucket.ratePerSec != ratePerSec {
		bucket = &tokenBucket{
			capacity:   capacity,
			tokens:     capacity,
			ratePerSec: ratePerSec,
			lastRefill: now,
		}
		r.buckets[provider] = bucket
	}

	elapsed := now.Sub(bucket.lastRefill).Seconds()
	bucket.tokens = math.Min(bucket.capacity, bucket.tokens+elapsed*bucket.ratePerSec)
	bucket.lastRefill = now
	if bucket.tokens < 1.0 {
		return false
	}
	bucket.tokens -= 1.0
	return true
}

type errorPayload struct {
	Type       string `json:"type"`
	Message    string `json:"message"`
	RequestID  string `json:"request_id"`
	Provider   string `json:"provider,omitempty"`
	RetryAfter *int   `json:"retry_after,omitempty"`
}

func writeJSONError(w http.ResponseWriter, status int, payload errorPayload, extras map[string]any) {
	if payload.RequestID != "" {
		w.Header().Set("X-Tokfence-Request-ID", payload.RequestID)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	errorBody := map[string]any{
		"type":       payload.Type,
		"message":    payload.Message,
		"request_id": payload.RequestID,
	}
	if payload.Provider != "" {
		errorBody["provider"] = payload.Provider
	}
	if payload.RetryAfter != nil {
		errorBody["retry_after"] = *payload.RetryAfter
	}
	if extras != nil {
		for key, val := range extras {
			errorBody[key] = val
		}
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"error": errorBody})
}

func writeBudgetExceeded(w http.ResponseWriter, requestID, provider string, limitCents, currentCents int64, resetsAt time.Time) {
	writeJSONError(w, http.StatusTooManyRequests,
		errorPayload{
			Type:      "tokfence_budget_exceeded",
			Message:   fmt.Sprintf("Budget of $%.2f for %s exceeded. Current spend: $%.2f.", float64(limitCents)/100.0, provider, float64(currentCents)/100.0),
			RequestID: requestID,
			Provider:  provider,
		}, map[string]any{
			"budget_limit":  float64(limitCents) / 100.0,
			"current_spend": float64(currentCents) / 100.0,
			"resets_at":     resetsAt.UTC().Format(time.RFC3339),
		})
}

func writeProviderRevoked(w http.ResponseWriter, requestID, provider string) {
	writeJSONError(w, http.StatusForbidden,
		errorPayload{
			Type:      "tokfence_provider_revoked",
			Message:   fmt.Sprintf("Provider %s is currently revoked", provider),
			RequestID: requestID,
			Provider:  provider,
		}, nil)
}

func writeRateLimitExceeded(w http.ResponseWriter, requestID, provider string, rpm int, retryAfter *int) {
	retry := retryAfter
	if retry == nil {
		tmp := int(1)
		retry = &tmp
	}
	writeJSONError(w, http.StatusTooManyRequests,
		errorPayload{
			Type:       "tokfence_rate_limit_exceeded",
			Message:    fmt.Sprintf("Rate limit exceeded for %s (%d RPM)", provider, rpm),
			RequestID:  requestID,
			Provider:   provider,
			RetryAfter: retry,
		}, map[string]any{
			"rpm": rpm,
		})
}

func writeReadBodyFailed(w http.ResponseWriter, requestID string, err error) {
	writeJSONError(w, http.StatusBadRequest,
		errorPayload{
			Type:      "tokfence_read_request_failed",
			Message:   err.Error(),
			RequestID: requestID,
		}, nil)
}

func writeRequestTooLarge(w http.ResponseWriter, requestID string, limitBytes int64) {
	writeJSONError(w, http.StatusRequestEntityTooLarge,
		errorPayload{
			Type:      "tokfence_request_too_large",
			Message:   fmt.Sprintf("request body exceeds maximum size of %d bytes", limitBytes),
			RequestID: requestID,
		}, nil)
}

func writeUpstreamError(w http.ResponseWriter, requestID, provider, errType, message string) {
	writeJSONError(w, http.StatusBadGateway,
		errorPayload{
			Type:      errType,
			Message:   message,
			RequestID: requestID,
			Provider:  provider,
		}, nil)
}
