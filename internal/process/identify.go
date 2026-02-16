package process

import (
	"context"
	"net"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const unknownSource = "unknown"

// ProcessIdentity holds evidence-backed caller identity.
type ProcessIdentity struct {
	PID        int
	Name       string
	Source     string
	Confidence float64
}

// Identify returns the best available process identity for the request.
//
// Evidence requirements:
// - Explicit process headers take precedence.
// - Fallback to local socket-owner inspection via lsof.
// - Unknown when no evidence is available.
func Identify(_ context.Context, r *http.Request) ProcessIdentity {
	if v := strings.TrimSpace(r.Header.Get("X-Tokfence-Agent")); v != "" {
		return ProcessIdentity{Name: v, Source: "header-x-tokfence-agent", Confidence: 1.0}
	}

	_, port, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil || port == "" {
		return ProcessIdentity{Source: unknownSource}
	}
	identity := discoverProcessByPort(port)
	if identity.Source == unknownSource {
		return identity
	}
	identity.Confidence = 0.8
	return identity
}

var (
	processPIDRegex  = regexp.MustCompile(`^p(\d+)$`)
	processNameRegex = regexp.MustCompile(`^c(.+)$`)
)

func discoverProcessByPort(port string) ProcessIdentity {
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, "lsof", "-nP", "-iTCP:"+port, "-sTCP:ESTABLISHED", "-Fpc")
	out, _ := cmd.Output()
	if ctx.Err() == context.DeadlineExceeded {
		return ProcessIdentity{Source: unknownSource}
	}
	text := string(out)
	identity := ProcessIdentity{Source: unknownSource}
	for _, token := range strings.Split(text, "\n") {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		if m := processPIDRegex.FindStringSubmatch(token); len(m) == 2 {
			identity.PID, _ = strconv.Atoi(m[1])
			continue
		}
		if m := processNameRegex.FindStringSubmatch(token); len(m) == 2 {
			identity.Name = strings.TrimSpace(m[1])
		}
	}
	if identity.PID <= 0 || identity.Name == "" {
		return ProcessIdentity{Source: unknownSource}
	}
	identity.Source = "lsof-socket"
	return identity
}
