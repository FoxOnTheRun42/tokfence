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

func writeJSONError(w http.ResponseWriter, status int, payload map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeBudgetExceeded(w http.ResponseWriter, provider string, limitCents, currentCents int64, resetsAt time.Time) {
	writeJSONError(w, http.StatusTooManyRequests, map[string]any{
		"error": map[string]any{
			"type":          "tokfence_budget_exceeded",
			"message":       fmt.Sprintf("Budget of $%.2f for %s exceeded. Current spend: $%.2f.", float64(limitCents)/100.0, provider, float64(currentCents)/100.0),
			"provider":      provider,
			"budget_limit":  float64(limitCents) / 100.0,
			"current_spend": float64(currentCents) / 100.0,
			"resets_at":     resetsAt.UTC().Format(time.RFC3339),
		},
	})
}

func writeProviderRevoked(w http.ResponseWriter, provider string) {
	writeJSONError(w, http.StatusForbidden, map[string]any{
		"error": map[string]any{
			"type":     "tokfence_provider_revoked",
			"message":  fmt.Sprintf("Provider %s is currently revoked", provider),
			"provider": provider,
		},
	})
}

func writeRateLimitExceeded(w http.ResponseWriter, provider string, rpm int) {
	writeJSONError(w, http.StatusTooManyRequests, map[string]any{
		"error": map[string]any{
			"type":     "tokfence_rate_limit_exceeded",
			"message":  fmt.Sprintf("Rate limit exceeded for %s (%d RPM)", provider, rpm),
			"provider": provider,
			"rpm":      rpm,
		},
	})
}
