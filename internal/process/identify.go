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

var (
	pidRegex  = regexp.MustCompile(`\np(\d+)`)
	nameRegex = regexp.MustCompile(`\nc([^\n]+)`)
)

func Identify(ctx context.Context, r *http.Request) (int, string) {
	if agent := strings.TrimSpace(r.Header.Get("X-Tokfence-Agent")); agent != "" {
		return 0, agent
	}
	if agent := strings.TrimSpace(r.Header.Get("X-Glassbox-Agent")); agent != "" {
		return 0, agent
	}
	_, port, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil || port == "" {
		return 0, ""
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(timeoutCtx, "lsof", "-nP", "-iTCP:"+port, "-sTCP:ESTABLISHED", "-Fpc")
	out, err := cmd.Output()
	if err != nil {
		return 0, ""
	}
	text := string(out)
	pid := 0
	if match := pidRegex.FindStringSubmatch(text); len(match) == 2 {
		pid, _ = strconv.Atoi(match[1])
	}
	name := ""
	if match := nameRegex.FindStringSubmatch(text); len(match) == 2 {
		name = strings.TrimSpace(match[1])
	}
	return pid, name
}
