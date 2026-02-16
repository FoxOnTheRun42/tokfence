package proxy

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

func IsStreamingJSON(requestBody []byte) bool {
	if len(requestBody) == 0 {
		return false
	}
	var payload map[string]any
	if err := json.Unmarshal(requestBody, &payload); err != nil {
		return false
	}
	streamRaw, ok := payload["stream"]
	if !ok {
		return false
	}
	stream, ok := streamRaw.(bool)
	return ok && stream
}

func IsSSEContentType(contentType string) bool {
	contentType = strings.ToLower(contentType)
	return strings.Contains(contentType, "text/event-stream")
}

func CopySSE(dst io.Writer, src io.Reader, flusher http.Flusher, capture *bytes.Buffer, onChunk ...func([]byte)) (int64, error) {
	reader := bufio.NewReader(src)
	buf := make([]byte, 16*1024)
	var cb func([]byte)
	if len(onChunk) > 0 {
		cb = onChunk[0]
	}
	var total int64
	for {
		n, readErr := reader.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			written, writeErr := dst.Write(chunk)
			total += int64(written)
			if capture != nil {
				_, _ = capture.Write(chunk)
			}
			if writeErr != nil {
				return total, writeErr
			}
			if cb != nil {
				cb(chunk)
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				return total, nil
			}
			return total, readErr
		}
	}
}
