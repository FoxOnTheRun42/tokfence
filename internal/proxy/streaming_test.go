package proxy

import (
	"bytes"
	"testing"
)

type mockFlusher struct{ flushed int }

func (m *mockFlusher) Flush() { m.flushed++ }

func TestIsStreamingJSON(t *testing.T) {
	if !IsStreamingJSON([]byte(`{"stream":true}`)) {
		t.Fatal("expected stream=true to be detected")
	}
	if IsStreamingJSON([]byte(`{"stream":false}`)) {
		t.Fatal("expected stream=false to be ignored")
	}
}

func TestCopySSE(t *testing.T) {
	src := bytes.NewBufferString("data: {\"ok\":true}\n\n")
	dst := bytes.NewBuffer(nil)
	capture := bytes.NewBuffer(nil)
	flusher := &mockFlusher{}
	_, err := CopySSE(dst, src, flusher, capture)
	if err != nil {
		t.Fatalf("CopySSE() error = %v", err)
	}
	if dst.String() != "data: {\"ok\":true}\n\n" {
		t.Fatalf("unexpected copied data: %q", dst.String())
	}
	if capture.String() != dst.String() {
		t.Fatalf("capture mismatch")
	}
	if flusher.flushed == 0 {
		t.Fatalf("expected flusher to be called")
	}
}
