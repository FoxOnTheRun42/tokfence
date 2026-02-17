# UDS Transport + ImmuneFence Status

> This document records completed implementation and current behavior.

## Current status

Implemented:
- dual-listener daemon transport (UDS + TCP fallback)
- socket defaults and permissions (`~/.tokfence/tokfence.sock`, `0660`, cleanup on shutdown)
- UDS/TCP probing support in CLI flow
- ImmuneFence pipeline: capabilities, risk machine, sensors, and canary escalation

## Context
Tokfence is a local-first AI API gateway that proxies requests to LLM providers (OpenAI, Anthropic, etc.) and injects API keys from an encrypted vault. It runs on TCP for external-facing tool traffic (`127.0.0.1:9471`) and UDS for local control-plane clients.

For the security model details, see [`security-model.md`](security-model.md).

## Repository layout
```
cmd/tokfence/main.go          — CLI (cobra), daemon start/stop/status, all subcommands
internal/config/config.go      — Config struct, TOML loading, defaults
internal/config/config_test.go — Config tests
internal/daemon/server.go      — HTTP server (ListenAndServe on TCP)
internal/daemon/server_test.go — Server integration tests (httptest)
internal/daemon/middleware.go   — Rate limiter, error response helpers
```

The macOS Desktop app (`apps/TokfenceDesktop/`) talks to the daemon **exclusively via CLI** (`tokfence status --json`, `tokfence launch status --json`, etc.) — it does NOT make direct HTTP calls to the daemon. Therefore the Desktop app code does NOT need changes.

## What to change

### 1. `internal/config/config.go`

Add `SocketPath` field to `DaemonConfig`:

```go
type DaemonConfig struct {
    Port       int    `toml:"port"`
    Host       string `toml:"host"`
    SocketPath string `toml:"socket_path"`
}
```

**Default socket path:** `~/.tokfence/tokfence.sock`

Add a helper:

```go
func DefaultSocketPath() string {
    return "~/.tokfence/tokfence.sock"
}
```

In `Default()`, set `SocketPath: DefaultSocketPath()`.

In `Load()`:
- If `loaded.Daemon.SocketPath != ""`, use it.
- After expansion, resolve `SocketPath` with `ExpandPath()` just like `DBPath`.
- Validate: socket path parent directory must exist or be creatable. Socket path length must be < 104 bytes (macOS `sun_path` limit). Add validation error if exceeded.

**Keep TCP host/port in the config** — they are still used by:
- `tokfence env` command (generates `OPENAI_BASE_URL=http://...` for agents)
- `tokfence setup openclaw` command  
- The OpenClaw config generator (`internal/launcher/config_gen.go`)

These **external-facing URLs must remain TCP** because Docker containers and external tools cannot connect to UDS inside the host filesystem. The UDS is only for daemon ↔ CLI communication and local proxy traffic from processes that can reach the socket file.

### 2. `internal/daemon/server.go`

Change `Server` to support UDS listening:

**Add field:**
```go
type Server struct {
    // ... existing fields ...
    socketPath string  // resolved UDS path, empty = use TCP fallback
}
```

**Update `NewServer`** to accept the resolved socket path (pass from config after `ExpandPath`).

**Update `Run(ctx)`:**
```go
func (s *Server) Run(ctx context.Context) error {
    // ... existing log cleanup ...
    
    // Determine listener
    var listener net.Listener
    var listenAddr string
    
    if s.socketPath != "" {
        // Remove stale socket file
        _ = os.Remove(s.socketPath)
        
        // Ensure parent directory exists with 0700
        sockDir := filepath.Dir(s.socketPath)
        if err := os.MkdirAll(sockDir, 0o700); err != nil {
            return fmt.Errorf("create socket directory: %w", err)
        }
        
        ul, err := net.Listen("unix", s.socketPath)
        if err != nil {
            return fmt.Errorf("listen unix %s: %w", s.socketPath, err)
        }
        // Set socket permissions: owner rw, group rw, no other
        if err := os.Chmod(s.socketPath, 0o660); err != nil {
            ul.Close()
            return fmt.Errorf("chmod socket: %w", err)
        }
        listener = ul
        listenAddr = s.socketPath
    } else {
        // TCP fallback
        addr := s.Addr()
        tl, err := net.Listen("tcp", addr)
        if err != nil {
            return fmt.Errorf("listen tcp %s: %w", addr, err)
        }
        listener = tl
        listenAddr = addr
    }
    
    // ... setup mux as before ...
    
    s.httpSrv = &http.Server{
        Handler:           mux,
        // Remove Addr field — we pass our own listener
        ReadHeaderTimeout: 15 * time.Second,
        ReadTimeout:       30 * time.Second,
        WriteTimeout:      10 * time.Minute,
        IdleTimeout:       120 * time.Second,
    }
    
    errCh := make(chan error, 1)
    go func() {
        s.isRunning.Store(true)
        err := s.httpSrv.Serve(listener)  // Serve, not ListenAndServe
        if err != nil && !errors.Is(err, http.ErrServerClosed) {
            errCh <- err
            return
        }
        errCh <- nil
    }()
    
    // ... rest unchanged ...
}
```

**Update `Shutdown`:** After `httpSrv.Shutdown`, clean up socket file:
```go
if s.socketPath != "" {
    _ = os.Remove(s.socketPath)
}
```

**Add method** for getting the listen address string (for logging/PID file):
```go
func (s *Server) ListenAddr() string {
    if s.socketPath != "" {
        return "unix:" + s.socketPath
    }
    return s.Addr()
}
```

### 3. `cmd/tokfence/main.go`

**`runForeground()`:**
- Resolve `cfg.Daemon.SocketPath` via `config.ExpandPath()`.
- Pass resolved socket path to `daemon.NewServer(...)`.
- Update `writePIDFile` call: use `server.ListenAddr()` instead of `server.Addr()`.
- Update the user-facing print: `"tokfence daemon listening on unix:%s\n"` or `"http://%s\n"` depending on mode.

**`spawnBackground()`:**
- Update the print message to show socket path when UDS is active.

**`newStatusCommand()`:**
- Replace the TCP `net.DialTimeout("tcp", addr, ...)` probe with a function that:
  - If PID file `addr` starts with `"unix:"` → dial `net.DialTimeout("unix", socketPath, ...)`
  - Otherwise → dial TCP as before
- Update JSON output: include `"socket": "/path/to/tokfence.sock"` when UDS.

**`newStopCommand()`:** No changes needed (uses PID/signal, not network).

**`newEnvCommand()`:**
- Keep generating `http://host:port/provider` URLs — these are for agents/tools that need HTTP endpoints.
- No change needed here.

**`newSetupCommand()` (openclaw):**
- Keep using TCP addr for the generated config (OpenClaw runs in Docker and needs TCP).
- The `--test` flag's daemon reachability check should use UDS if configured:
  ```go
  // Instead of: net.DialTimeout("tcp", addr, 2*time.Second)
  // Use: dialDaemon(cfg) which checks socket first, TCP second
  ```

**`collectWidgetSnapshot()`:**
- Update to show socket path in snapshot when UDS active.

**Add helper:**
```go
func dialDaemon(cfg config.Config) (net.Conn, error) {
    if cfg.Daemon.SocketPath != "" {
        expanded, err := config.ExpandPath(cfg.Daemon.SocketPath)
        if err == nil {
            conn, err := net.DialTimeout("unix", expanded, 500*time.Millisecond)
            if err == nil {
                return conn, nil
            }
        }
    }
    addr := fmt.Sprintf("%s:%d", cfg.Daemon.Host, cfg.Daemon.Port)
    return net.DialTimeout("tcp", addr, 500*time.Millisecond)
}
```

### 4. `internal/daemon/server_test.go`

Tests use `httptest.NewRecorder()` and call `srv.handleProxy()` directly — they do NOT go through the network listener. **These tests should continue to work without changes.**

Add one new test:

```go
func TestServerListensOnUnixSocket(t *testing.T) {
    socketPath := filepath.Join(t.TempDir(), "tokfence.sock")
    cfg := config.Default()
    store, err := logger.Open(filepath.Join(t.TempDir(), "tokfence.db"))
    if err != nil {
        t.Fatalf("open logger: %v", err)
    }
    defer store.Close()
    engine := budget.NewEngine(store.DB())
    v := &testVault{keys: map[string]string{}}
    srv := NewServer(cfg, v, store, engine)
    srv.socketPath = socketPath  // or however you expose it

    ctx, cancel := context.WithCancel(context.Background())
    errCh := make(chan error, 1)
    go func() { errCh <- srv.Run(ctx) }()

    // Wait for socket to appear
    deadline := time.Now().Add(3 * time.Second)
    for time.Now().Before(deadline) {
        if _, err := os.Stat(socketPath); err == nil {
            break
        }
        time.Sleep(50 * time.Millisecond)
    }

    // Connect via UDS
    client := &http.Client{
        Transport: &http.Transport{
            DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
                return net.DialTimeout("unix", socketPath, 2*time.Second)
            },
        },
    }
    resp, err := client.Get("http://localhost/__tokfence/health")
    if err != nil {
        t.Fatalf("health check via UDS: %v", err)
    }
    defer resp.Body.Close()
    if resp.StatusCode != 200 {
        t.Fatalf("health status = %d, want 200", resp.StatusCode)
    }

    // Check socket permissions
    info, err := os.Stat(socketPath)
    if err != nil {
        t.Fatalf("stat socket: %v", err)
    }
    perm := info.Mode().Perm()
    if perm != 0o660 {
        t.Fatalf("socket permissions = %o, want 0660", perm)
    }

    cancel()
    <-errCh

    // Socket should be cleaned up
    if _, err := os.Stat(socketPath); !os.IsNotExist(err) {
        t.Fatal("socket file should be removed after shutdown")
    }
}
```

### 5. `internal/config/config_test.go`

Add test:
```go
func TestSocketPathValidation(t *testing.T) {
    // Test that overly long socket paths are rejected (macOS sun_path limit = 104)
    // Test that default socket path resolves correctly
    // Test that SocketPath is preserved through Load/Save cycle
}
```

## What NOT to change

- **`internal/launcher/config_gen.go`** — OpenClaw config uses TCP `host.docker.internal:9471` to reach the daemon from inside Docker. This MUST stay TCP.
- **`apps/TokfenceDesktop/`** — Desktop app uses CLI, not direct HTTP. No changes.
- **`internal/proxy/`** — Proxy logic is transport-agnostic (operates on `http.Request`/`http.ResponseWriter`). No changes.
- **`internal/vault/`**, **`internal/budget/`**, **`internal/logger/`** — No network code. No changes.
- **`README.md`**, **`SECURITY.md`** — Do NOT update docs in this PR.

## Acceptance criteria

1. `tokfence start` creates `~/.tokfence/tokfence.sock` with permissions `0660`
2. `tokfence start` prints `tokfence daemon listening on unix:/path/to/tokfence.sock`
3. `tokfence status` detects running daemon via UDS
4. `tokfence stop` still works (uses PID signal, not network)
5. `tokfence env` still outputs `http://127.0.0.1:9471/provider` URLs (for agent consumption)
6. `tokfence launch` / OpenClaw still uses TCP to reach daemon from Docker container
7. Stale socket files from crashed daemon are cleaned up on next `tokfence start`
8. Socket file is removed on clean shutdown
9. All existing tests pass (`go test ./...`)
10. New UDS listener test passes
11. Config: `socket_path` in TOML is respected; default `~/.tokfence/tokfence.sock` when omitted
12. Socket paths > 103 bytes are rejected with a clear error at config load time

## Build & test commands

```bash
cd /path/to/tokfence
go test ./...
go build -o tokfence ./cmd/tokfence
```
