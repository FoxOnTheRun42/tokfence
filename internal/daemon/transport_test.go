package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"strconv"
	"testing"

	"github.com/macfox/tokfence/internal/budget"
	"github.com/macfox/tokfence/internal/config"
	"github.com/macfox/tokfence/internal/logger"
)

func waitForUDSSocket(t *testing.T, socketPath string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(socketPath); err == nil {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("socket did not appear: %s", socketPath)
}

func waitForTCP(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("tcp endpoint not reachable: %s", addr)
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("allocate free tcp port: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func TestRunUsesUnixSocketAndTcpAndCleansSocket(t *testing.T) {
	socketPath := filepath.Join("/tmp", fmt.Sprintf("tokfence-%d.sock", time.Now().UnixNano()))
	cfg := config.Default()
	cfg.Daemon.SocketPath = socketPath
	cfg.Daemon.Port = freeTCPPort(t)

	store, err := logger.Open(filepath.Join(t.TempDir(), "tokfence.db"))
	if err != nil {
		t.Fatalf("open logger: %v", err)
	}
	defer store.Close()
	srv := NewServer(cfg, &testVault{keys: map[string]string{}}, store, budget.NewEngine(store.DB()))

	ctx, cancel := context.WithCancel(context.Background())
	runErr := make(chan error, 1)
	go func() {
		runErr <- srv.Run(ctx)
	}()

	waitForUDSSocket(t, socketPath)
	waitForTCP(t, net.JoinHostPort(cfg.Daemon.Host, strconv.Itoa(cfg.Daemon.Port)))

	udsClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
			},
		},
	}
	udsResp, err := udsClient.Get("http://tokfence/__tokfence/health")
	if err != nil {
		cancel()
		<-runErr
		t.Fatalf("UDS health request failed: %v", err)
	}
	_ = udsResp.Body.Close()
	if udsResp.StatusCode != http.StatusOK {
		cancel()
		<-runErr
		t.Fatalf("UDS health status = %d", udsResp.StatusCode)
	}

	resp, err := http.Get("http://" + net.JoinHostPort(cfg.Daemon.Host, strconv.Itoa(cfg.Daemon.Port)) + "/__tokfence/health")
	if err != nil {
		cancel()
		<-runErr
		t.Fatalf("tcp health request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		cancel()
		<-runErr
		t.Fatalf("tcp health status = %d", resp.StatusCode)
	}

	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		cancel()
		<-runErr
		t.Fatalf("decode health response: %v", err)
	}
	if payload["status"] != "ok" {
		cancel()
		<-runErr
		t.Fatalf("health status payload = %#v", payload["status"])
	}

	socketInfo, err := os.Stat(socketPath)
	if err != nil {
		cancel()
		<-runErr
		t.Fatalf("socket stat failed: %v", err)
	}
	if socketInfo.Mode().Perm() != 0o660 {
		cancel()
		<-runErr
		t.Fatalf("socket mode = %o, expected 660", socketInfo.Mode().Perm())
	}

	cancel()
	_ = <-runErr
	if _, err := os.Stat(socketPath); !os.IsNotExist(err) {
		t.Fatalf("socket should be removed after shutdown, stat err=%v", err)
	}
}
