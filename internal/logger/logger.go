package logger

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type RequestRecord struct {
	ID                  string
	Timestamp           time.Time
	Provider            string
	Model               string
	Endpoint            string
	Method              string
	InputTokens         int
	OutputTokens        int
	CacheReadTokens     int
	CacheCreationTokens int
	EstimatedCostCents  int64
	StatusCode          int
	LatencyMS           int
	TTFTMS              int
	CallerPID           int
	CallerName          string
	IsStreaming         bool
	ErrorType           string
	ErrorMessage        string
	RequestHash         string
}

type QueryFilter struct {
	Limit    int
	Provider string
	Model    string
	Since    time.Time
}

type StatsFilter struct {
	Provider string
	Since    time.Time
	By       string
}

type StatsRow struct {
	Group              string
	RequestCount       int
	InputTokens        int64
	OutputTokens       int64
	EstimatedCostCents int64
}

type LogStore struct {
	db *sql.DB
}

func Open(dbPath string) (*LogStore, error) {
	if strings.TrimSpace(dbPath) == "" {
		return nil, errors.New("db path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}
	db, err := sql.Open("sqlite3", dbPath+"?_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}
	if err := os.Chmod(dbPath, 0o600); err != nil && !os.IsNotExist(err) {
		_ = db.Close()
		return nil, fmt.Errorf("set db perms: %w", err)
	}
	store := &LogStore{db: db}
	if err := store.Init(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := os.Chmod(dbPath, 0o600); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set db perms: %w", err)
	}
	return store, nil
}

func (s *LogStore) DB() *sql.DB {
	return s.db
}

func (s *LogStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *LogStore) Init(ctx context.Context) error {
	schema := `
CREATE TABLE IF NOT EXISTS requests (
    id TEXT PRIMARY KEY,
    timestamp DATETIME NOT NULL DEFAULT (datetime('now')),
    provider TEXT NOT NULL,
    model TEXT,
    endpoint TEXT NOT NULL,
    method TEXT NOT NULL DEFAULT 'POST',
    input_tokens INTEGER,
    output_tokens INTEGER,
    cache_read_tokens INTEGER,
    cache_creation_tokens INTEGER,
    estimated_cost_cents INTEGER,
    status_code INTEGER,
    latency_ms INTEGER,
    ttft_ms INTEGER,
    caller_pid INTEGER,
    caller_name TEXT,
    is_streaming BOOLEAN DEFAULT FALSE,
    error_type TEXT,
    error_message TEXT,
    request_hash TEXT
);
CREATE INDEX IF NOT EXISTS idx_requests_timestamp ON requests(timestamp);
CREATE INDEX IF NOT EXISTS idx_requests_provider ON requests(provider);
CREATE INDEX IF NOT EXISTS idx_requests_model ON requests(model);

CREATE TABLE IF NOT EXISTS budgets (
    provider TEXT PRIMARY KEY,
    limit_cents INTEGER NOT NULL,
    period TEXT NOT NULL,
    current_spend_cents INTEGER DEFAULT 0,
    period_start DATETIME NOT NULL,
    enabled BOOLEAN DEFAULT TRUE
);

CREATE TABLE IF NOT EXISTS provider_status (
    provider TEXT PRIMARY KEY,
    revoked BOOLEAN DEFAULT FALSE,
    revoked_at DATETIME
);

CREATE TABLE IF NOT EXISTS ratelimits (
    provider TEXT PRIMARY KEY,
    rpm INTEGER NOT NULL
);
`
	_, err := s.db.ExecContext(ctx, schema)
	if err != nil {
		return fmt.Errorf("init schema: %w", err)
	}
	return nil
}

func (s *LogStore) LogRequest(ctx context.Context, record RequestRecord) error {
	query := `
INSERT INTO requests (
    id, timestamp, provider, model, endpoint, method,
    input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens,
    estimated_cost_cents, status_code, latency_ms, ttft_ms,
    caller_pid, caller_name, is_streaming, error_type, error_message, request_hash
) VALUES (
    ?, ?, ?, ?, ?, ?,
    ?, ?, ?, ?,
    ?, ?, ?, ?,
    ?, ?, ?, ?, ?, ?
)`
	_, err := s.db.ExecContext(ctx, query,
		record.ID,
		record.Timestamp.UTC().Format(time.RFC3339),
		record.Provider,
		record.Model,
		record.Endpoint,
		record.Method,
		record.InputTokens,
		record.OutputTokens,
		record.CacheReadTokens,
		record.CacheCreationTokens,
		record.EstimatedCostCents,
		record.StatusCode,
		record.LatencyMS,
		record.TTFTMS,
		record.CallerPID,
		record.CallerName,
		record.IsStreaming,
		record.ErrorType,
		record.ErrorMessage,
		record.RequestHash,
	)
	if err != nil {
		return fmt.Errorf("insert request log: %w", err)
	}
	return nil
}

func (s *LogStore) GetRequest(ctx context.Context, id string) (RequestRecord, error) {
	query := `SELECT id, timestamp, provider, model, endpoint, method,
       input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens,
       estimated_cost_cents, status_code, latency_ms, ttft_ms,
       caller_pid, caller_name, is_streaming, error_type, error_message, request_hash
FROM requests WHERE id = ?`
	row := s.db.QueryRowContext(ctx, query, id)
	return scanRequest(row)
}

func (s *LogStore) ListRequests(ctx context.Context, filter QueryFilter) ([]RequestRecord, error) {
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	where := []string{"1=1"}
	args := make([]any, 0, 4)
	if filter.Provider != "" {
		where = append(where, "provider = ?")
		args = append(args, filter.Provider)
	}
	if filter.Model != "" {
		where = append(where, "model = ?")
		args = append(args, filter.Model)
	}
	if !filter.Since.IsZero() {
		where = append(where, "timestamp >= ?")
		args = append(args, filter.Since.UTC().Format(time.RFC3339))
	}
	args = append(args, filter.Limit)
	query := `SELECT id, timestamp, provider, model, endpoint, method,
       input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens,
       estimated_cost_cents, status_code, latency_ms, ttft_ms,
       caller_pid, caller_name, is_streaming, error_type, error_message, request_hash
FROM requests
WHERE ` + strings.Join(where, " AND ") + `
ORDER BY timestamp DESC
LIMIT ?`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query requests: %w", err)
	}
	defer rows.Close()
	out := []RequestRecord{}
	for rows.Next() {
		record, err := scanRequest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	return out, rows.Err()
}

func (s *LogStore) Stats(ctx context.Context, filter StatsFilter) ([]StatsRow, error) {
	groupExpr := "provider"
	switch strings.ToLower(filter.By) {
	case "model":
		groupExpr = "COALESCE(model, '')"
	case "hour":
		groupExpr = "strftime('%Y-%m-%d %H:00', timestamp)"
	case "", "provider":
		groupExpr = "provider"
	}
	where := []string{"1=1"}
	args := make([]any, 0, 2)
	if filter.Provider != "" {
		where = append(where, "provider = ?")
		args = append(args, filter.Provider)
	}
	if !filter.Since.IsZero() {
		where = append(where, "timestamp >= ?")
		args = append(args, filter.Since.UTC().Format(time.RFC3339))
	}
	query := `SELECT ` + groupExpr + ` as grp,
       COUNT(*) as request_count,
       COALESCE(SUM(input_tokens), 0),
       COALESCE(SUM(output_tokens), 0),
       COALESCE(SUM(estimated_cost_cents), 0)
FROM requests WHERE ` + strings.Join(where, " AND ") + `
GROUP BY grp ORDER BY request_count DESC`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query stats: %w", err)
	}
	defer rows.Close()
	result := []StatsRow{}
	for rows.Next() {
		var row StatsRow
		if err := rows.Scan(&row.Group, &row.RequestCount, &row.InputTokens, &row.OutputTokens, &row.EstimatedCostCents); err != nil {
			return nil, fmt.Errorf("scan stats row: %w", err)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (s *LogStore) DeleteOlderThan(ctx context.Context, days int) error {
	if days <= 0 {
		return nil
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -days)
	_, err := s.db.ExecContext(ctx, `DELETE FROM requests WHERE timestamp < ?`, cutoff.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("delete old requests: %w", err)
	}
	return nil
}

func (s *LogStore) IsProviderRevoked(ctx context.Context, provider string) (bool, error) {
	var revoked bool
	row := s.db.QueryRowContext(ctx, `SELECT revoked FROM provider_status WHERE provider = ?`, provider)
	err := row.Scan(&revoked)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("query provider status: %w", err)
	}
	return revoked, nil
}

func (s *LogStore) SetProviderRevoked(ctx context.Context, provider string, revoked bool) error {
	var revokedAt any
	if revoked {
		revokedAt = time.Now().UTC().Format(time.RFC3339)
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO provider_status(provider, revoked, revoked_at)
VALUES (?, ?, ?)
ON CONFLICT(provider) DO UPDATE SET revoked = excluded.revoked, revoked_at = excluded.revoked_at
`, provider, revoked, revokedAt)
	if err != nil {
		return fmt.Errorf("set provider status: %w", err)
	}
	return nil
}

func (s *LogStore) SetAllProvidersRevoked(ctx context.Context, providers []string, revoked bool) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin provider status tx: %w", err)
	}
	defer tx.Rollback()
	for _, provider := range providers {
		var revokedAt any
		if revoked {
			revokedAt = time.Now().UTC().Format(time.RFC3339)
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO provider_status(provider, revoked, revoked_at)
VALUES (?, ?, ?)
ON CONFLICT(provider) DO UPDATE SET revoked = excluded.revoked, revoked_at = excluded.revoked_at
`, provider, revoked, revokedAt); err != nil {
			return fmt.Errorf("set provider %s status: %w", provider, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit provider status tx: %w", err)
	}
	return nil
}

func (s *LogStore) SetRateLimit(ctx context.Context, provider string, rpm int) error {
	if rpm <= 0 {
		return errors.New("rpm must be > 0")
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO ratelimits(provider, rpm) VALUES(?, ?)
ON CONFLICT(provider) DO UPDATE SET rpm = excluded.rpm
`, provider, rpm)
	if err != nil {
		return fmt.Errorf("set ratelimit: %w", err)
	}
	return nil
}

func (s *LogStore) ClearRateLimit(ctx context.Context, provider string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM ratelimits WHERE provider = ?`, provider)
	if err != nil {
		return fmt.Errorf("clear ratelimit: %w", err)
	}
	return nil
}

func (s *LogStore) GetRateLimit(ctx context.Context, provider string) (int, error) {
	var rpm int
	err := s.db.QueryRowContext(ctx, `SELECT rpm FROM ratelimits WHERE provider = ?`, provider).Scan(&rpm)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("query ratelimit: %w", err)
	}
	return rpm, nil
}

func (s *LogStore) ListRateLimits(ctx context.Context) (map[string]int, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT provider, rpm FROM ratelimits`)
	if err != nil {
		return nil, fmt.Errorf("list ratelimits: %w", err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var provider string
		var rpm int
		if err := rows.Scan(&provider, &rpm); err != nil {
			return nil, fmt.Errorf("scan ratelimit row: %w", err)
		}
		out[provider] = rpm
	}
	return out, rows.Err()
}

func scanRequest(scanner interface{ Scan(dest ...any) error }) (RequestRecord, error) {
	var record RequestRecord
	var timestamp string
	if err := scanner.Scan(
		&record.ID,
		&timestamp,
		&record.Provider,
		&record.Model,
		&record.Endpoint,
		&record.Method,
		&record.InputTokens,
		&record.OutputTokens,
		&record.CacheReadTokens,
		&record.CacheCreationTokens,
		&record.EstimatedCostCents,
		&record.StatusCode,
		&record.LatencyMS,
		&record.TTFTMS,
		&record.CallerPID,
		&record.CallerName,
		&record.IsStreaming,
		&record.ErrorType,
		&record.ErrorMessage,
		&record.RequestHash,
	); err != nil {
		return RequestRecord{}, fmt.Errorf("scan request row: %w", err)
	}
	if parsed, err := time.Parse(time.RFC3339, timestamp); err == nil {
		record.Timestamp = parsed
	} else {
		record.Timestamp = time.Now().UTC()
	}
	return record, nil
}
