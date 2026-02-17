package launcher

import (
	"net"
	"testing"
)

func TestIsPortAvailable(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen test port: %v", err)
	}

	addr := ln.Addr().(*net.TCPAddr)
	if !addr.IP.Equal(net.ParseIP("127.0.0.1")) {
		t.Fatalf("unexpected bind address: %v", addr.IP)
	}
	inUsePort := addr.Port
	if IsPortAvailable(inUsePort) {
		t.Fatalf("in-use port %d reported as available", inUsePort)
	}
	ln.Close()
	if !IsPortAvailable(inUsePort) {
		t.Fatalf("free port %d reported as unavailable", inUsePort)
	}
}
