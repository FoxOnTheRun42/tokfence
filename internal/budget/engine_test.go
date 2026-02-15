package budget

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/macfox/tokfence/internal/logger"
)

func TestBudgetEnforcement(t *testing.T) {
	store, err := logger.Open(filepath.Join(t.TempDir(), "tokfence.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	engine := NewEngine(store.DB())
	ctx := context.Background()

	if err := engine.SetBudget(ctx, "openai", 1.00, "daily"); err != nil {
		t.Fatalf("SetBudget() error = %v", err)
	}
	if err := engine.AddSpend(ctx, "openai", 120); err != nil {
		t.Fatalf("AddSpend() error = %v", err)
	}
	exceeded, err := engine.CheckLimit(ctx, "openai")
	if err != nil {
		t.Fatalf("CheckLimit() error = %v", err)
	}
	if exceeded == nil {
		t.Fatalf("expected budget to be exceeded")
	}
	if exceeded.Provider != "openai" {
		t.Fatalf("provider = %s, want openai", exceeded.Provider)
	}
}

func TestEstimateCostCents(t *testing.T) {
	cost := EstimateCostCents("gpt-4o", 1_000_000, 1_000_000)
	if cost != 1250 {
		t.Fatalf("EstimateCostCents() = %d, want 1250", cost)
	}
}
