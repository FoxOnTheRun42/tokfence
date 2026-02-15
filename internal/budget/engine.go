package budget

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Budget struct {
	Provider          string
	LimitCents        int64
	Period            string
	CurrentSpendCents int64
	PeriodStart       time.Time
	Enabled           bool
}

type Exceeded struct {
	Provider     string
	BudgetLimit  int64
	CurrentSpend int64
	ResetsAt     time.Time
}

type Engine struct {
	db *sql.DB
}

func NewEngine(db *sql.DB) *Engine {
	return &Engine{db: db}
}

func normalizePeriod(period string) (string, error) {
	period = strings.ToLower(strings.TrimSpace(period))
	if period != "daily" && period != "monthly" {
		return "", fmt.Errorf("invalid period %q (expected daily or monthly)", period)
	}
	return period, nil
}

func periodStart(now time.Time, period string) time.Time {
	now = now.UTC()
	switch period {
	case "daily":
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	case "monthly":
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	default:
		return now
	}
}

func nextReset(start time.Time, period string) time.Time {
	start = start.UTC()
	switch period {
	case "daily":
		return start.Add(24 * time.Hour)
	case "monthly":
		return start.AddDate(0, 1, 0)
	default:
		return start
	}
}

func (e *Engine) SetBudget(ctx context.Context, provider string, amountUSD float64, period string) error {
	if strings.TrimSpace(provider) == "" {
		return errors.New("provider is required")
	}
	period, err := normalizePeriod(period)
	if err != nil {
		return err
	}
	if amountUSD < 0 {
		return errors.New("amount must be >= 0")
	}
	limitCents := int64(amountUSD * 100)
	start := periodStart(time.Now().UTC(), period)
	_, err = e.db.ExecContext(ctx, `
INSERT INTO budgets(provider, limit_cents, period, current_spend_cents, period_start, enabled)
VALUES(?, ?, ?, 0, ?, 1)
ON CONFLICT(provider) DO UPDATE SET
    limit_cents = excluded.limit_cents,
    period = excluded.period,
    enabled = 1,
    period_start = CASE
        WHEN budgets.period = excluded.period THEN budgets.period_start
        ELSE excluded.period_start
    END
`, strings.ToLower(provider), limitCents, period, start.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("set budget: %w", err)
	}
	return nil
}

func (e *Engine) ClearBudget(ctx context.Context, provider string) error {
	_, err := e.db.ExecContext(ctx, `DELETE FROM budgets WHERE provider = ?`, strings.ToLower(provider))
	if err != nil {
		return fmt.Errorf("clear budget: %w", err)
	}
	return nil
}

func (e *Engine) Status(ctx context.Context) ([]Budget, error) {
	if err := e.ResetExpired(ctx); err != nil {
		return nil, err
	}
	rows, err := e.db.QueryContext(ctx, `SELECT provider, limit_cents, period, current_spend_cents, period_start, enabled FROM budgets ORDER BY provider`)
	if err != nil {
		return nil, fmt.Errorf("query budgets: %w", err)
	}
	defer rows.Close()
	out := []Budget{}
	for rows.Next() {
		var b Budget
		var periodStartStr string
		if err := rows.Scan(&b.Provider, &b.LimitCents, &b.Period, &b.CurrentSpendCents, &periodStartStr, &b.Enabled); err != nil {
			return nil, fmt.Errorf("scan budget: %w", err)
		}
		if parsed, err := time.Parse(time.RFC3339, periodStartStr); err == nil {
			b.PeriodStart = parsed
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (e *Engine) ResetExpired(ctx context.Context) error {
	rows, err := e.db.QueryContext(ctx, `SELECT provider, period, period_start FROM budgets WHERE enabled = 1`)
	if err != nil {
		return fmt.Errorf("query budgets for reset: %w", err)
	}
	defer rows.Close()
	type rowData struct {
		provider string
		period   string
		start    time.Time
	}
	pending := []rowData{}
	now := time.Now().UTC()
	for rows.Next() {
		var provider, period, startStr string
		if err := rows.Scan(&provider, &period, &startStr); err != nil {
			return fmt.Errorf("scan budget reset row: %w", err)
		}
		start, err := time.Parse(time.RFC3339, startStr)
		if err != nil {
			start = periodStart(now, period)
		}
		if !now.Before(nextReset(start, period)) {
			pending = append(pending, rowData{provider: provider, period: period, start: periodStart(now, period)})
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, entry := range pending {
		if _, err := e.db.ExecContext(ctx, `UPDATE budgets SET current_spend_cents = 0, period_start = ? WHERE provider = ?`, entry.start.Format(time.RFC3339), entry.provider); err != nil {
			return fmt.Errorf("reset budget %s: %w", entry.provider, err)
		}
	}
	return nil
}

func (e *Engine) CheckLimit(ctx context.Context, provider string) (*Exceeded, error) {
	provider = strings.ToLower(provider)
	if err := e.ResetExpired(ctx); err != nil {
		return nil, err
	}
	for _, candidate := range []string{provider, "global"} {
		exceeded, err := e.checkSingle(ctx, candidate)
		if err != nil {
			return nil, err
		}
		if exceeded != nil {
			return exceeded, nil
		}
	}
	return nil, nil
}

func (e *Engine) checkSingle(ctx context.Context, provider string) (*Exceeded, error) {
	row := e.db.QueryRowContext(ctx, `SELECT limit_cents, current_spend_cents, period, period_start, enabled FROM budgets WHERE provider = ?`, provider)
	var limitCents, currentSpend int64
	var period, startStr string
	var enabled bool
	if err := row.Scan(&limitCents, &currentSpend, &period, &startStr, &enabled); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("query budget %s: %w", provider, err)
	}
	if !enabled {
		return nil, nil
	}
	if currentSpend < limitCents {
		return nil, nil
	}
	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		start = periodStart(time.Now().UTC(), period)
	}
	return &Exceeded{
		Provider:     provider,
		BudgetLimit:  limitCents,
		CurrentSpend: currentSpend,
		ResetsAt:     nextReset(start, period),
	}, nil
}

func (e *Engine) AddSpend(ctx context.Context, provider string, cents int64) error {
	if cents <= 0 {
		return nil
	}
	if err := e.ResetExpired(ctx); err != nil {
		return err
	}
	for _, target := range []string{strings.ToLower(provider), "global"} {
		if _, err := e.db.ExecContext(ctx, `UPDATE budgets SET current_spend_cents = current_spend_cents + ? WHERE provider = ? AND enabled = 1`, cents, target); err != nil {
			return fmt.Errorf("add spend to %s: %w", target, err)
		}
	}
	return nil
}
