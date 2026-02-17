package launcher

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	dockerErrNotInstalled = "Docker is not installed. Install Docker Desktop from https://docker.com/get-started"
	dockerErrDaemonDown   = "Docker daemon is not running. Start Docker Desktop and try again."
)

var dockerBinaryCandidates = []string{
	"/opt/homebrew/bin/docker",
	"/usr/local/bin/docker",
	"/Applications/Docker.app/Contents/Resources/bin/docker",
	"/Applications/Docker.app/Contents/MacOS/com.docker.cli",
}

type ContainerOpts struct {
	Name       string
	Image      string
	Volumes    []string
	Ports      []string
	ExtraHosts []string
	Restart    string
}

func DockerAvailable(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	out, err := runDockerCommand(ctx, "info")
	if err != nil {
		return dockerAvailabilityError(err, out)
	}
	return nil
}

func PullImage(ctx context.Context, image string, w io.Writer) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	cmd, err := dockerCommandContext(ctx, "pull", image)
	if err != nil {
		return err
	}
	cmd.Stdout = w
	cmd.Stderr = w
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker pull %s: %w", image, err)
	}
	return nil
}

func RunContainer(ctx context.Context, opts ContainerOpts) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	args := []string{"run", "-d", "--name", opts.Name, "--restart", opts.Restart}
	for _, volume := range opts.Volumes {
		if strings.TrimSpace(volume) == "" {
			continue
		}
		args = append(args, "-v", volume)
	}
	for _, port := range opts.Ports {
		if strings.TrimSpace(port) == "" {
			continue
		}
		args = append(args, "-p", port)
	}
	for _, host := range opts.ExtraHosts {
		if strings.TrimSpace(host) == "" {
			continue
		}
		args = append(args, "--add-host", host)
	}
	args = append(args, opts.Image)

	out, err := runDockerCommand(ctx, args...)
	if err != nil {
		return "", fmt.Errorf("docker run: %w (%s)", err, strings.TrimSpace(out))
	}

	id := strings.TrimSpace(out)
	if len(id) < 12 {
		return "", fmt.Errorf("invalid container id returned: %q", id)
	}
	return id[:12], nil
}

func StopAndRemoveContainer(ctx context.Context, name string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if _, err := runDockerCommand(ctx, "stop", name); err != nil && !isDockerNotFoundError(err, "") {
		return err
	}
	if _, err := runDockerCommand(ctx, "rm", name); err != nil && !isDockerNotFoundError(err, "") {
		return err
	}
	return nil
}

func ContainerStatus(ctx context.Context, name string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	output, err := runDockerCommand(ctx, "inspect", "--format", "{{.State.Status}}", name)
	if err != nil {
		if isDockerNotFoundError(err, output) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(output), nil
}

func ContainerLogs(ctx context.Context, name string, follow bool, w io.Writer) error {
	args := []string{"logs"}
	if follow {
		args = append(args, "--follow")
	}
	args = append(args, name)

	cmd, err := dockerCommandContext(ctx, args...)
	if err != nil {
		return err
	}
	cmd.Stdout = w
	cmd.Stderr = w
	return cmd.Run()
}

func IsPortAvailable(port int) bool {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		return strings.Contains(strings.ToLower(err.Error()), "connection refused")
	}
	_ = conn.Close()
	return false
}

func runDockerCommand(ctx context.Context, args ...string) (string, error) {
	cmd, err := dockerCommandContext(ctx, args...)
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return out.String(), err
	}
	return out.String(), nil
}

func dockerCommandContext(ctx context.Context, args ...string) (*exec.Cmd, error) {
	bin, err := resolveDockerBinary()
	if err != nil {
		return nil, err
	}
	return exec.CommandContext(ctx, bin, args...), nil
}

func resolveDockerBinary() (string, error) {
	if path, err := exec.LookPath("docker"); err == nil {
		return path, nil
	}
	for _, candidate := range dockerBinaryCandidates {
		if isExecutableFile(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("docker binary not found")
}

func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode()&0o111 != 0
}

func isDockerNotFoundError(err error, output string) bool {
	if err == nil {
		return false
	}
	if strings.Contains(strings.ToLower(output), "no such container") {
		return true
	}
	if strings.Contains(strings.ToLower(output), "no such object") {
		return true
	}
	if strings.Contains(strings.ToLower(err.Error()), "no such container") {
		return true
	}
	if strings.Contains(strings.ToLower(err.Error()), "not found") {
		return true
	}
	return false
}

func dockerAvailabilityError(err error, output string) error {
	if err == nil {
		return nil
	}
	outputLower := strings.ToLower(output)
	errLower := strings.ToLower(err.Error())
	if strings.Contains(errLower, "executable file not found") ||
		strings.Contains(errLower, "docker binary not found") ||
		strings.Contains(errLower, "no such file or directory") ||
		strings.Contains(outputLower, "command not found") {
		return fmt.Errorf("%s", dockerErrNotInstalled)
	}
	if strings.Contains(outputLower, "cannot connect to the docker daemon") ||
		strings.Contains(outputLower, "docker daemon") ||
		strings.Contains(outputLower, "is the docker daemon running") {
		return fmt.Errorf("%s", dockerErrDaemonDown)
	}
	return fmt.Errorf("%s: %s", dockerErrDaemonDown, strings.TrimSpace(output))
}
