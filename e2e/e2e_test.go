//go:build e2e

package e2e_test

import (
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func buildAutoportBinary(t *testing.T) string {
	t.Helper()

	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "autoport")

	build := exec.Command("go", "build", "-o", binPath, "..")
	output, err := build.CombinedOutput()
	if err != nil {
		t.Fatalf("build autoport: %v\n%s", err, string(output))
	}
	return binPath
}

func TestE2E_ExportsPortWhenNoEnvFound(t *testing.T) {
	binPath := buildAutoportBinary(t)
	projectDir := t.TempDir()

	cmd := exec.Command(binPath)
	cmd.Dir = projectDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run autoport: %v\n%s", err, string(output))
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	found := false
	re := regexp.MustCompile(`^export PORT=\d+$`)
	for _, line := range lines {
		if re.MatchString(line) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected PORT export line, got:\n%s", string(output))
	}
}

func TestE2E_ManualPortKeyIsAppliedToCommand(t *testing.T) {
	binPath := buildAutoportBinary(t)
	projectDir := t.TempDir()

	cmd := exec.Command(binPath, "-k", "WEB_PORT", "sh", "-c", "printf '%s|%s' \"$PORT\" \"$WEB_PORT\"")
	cmd.Dir = projectDir
	stdout, err := cmd.Output()
	if err != nil {
		t.Fatalf("run autoport command mode: %v", err)
	}

	parts := strings.Split(string(stdout), "|")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		t.Fatalf("expected PORT and WEB_PORT values, got %q", string(stdout))
	}
	if !regexp.MustCompile(`^\d+$`).MatchString(parts[0]) {
		t.Fatalf("expected numeric PORT, got %q", parts[0])
	}
	if !regexp.MustCompile(`^\d+$`).MatchString(parts[1]) {
		t.Fatalf("expected numeric WEB_PORT, got %q", parts[1])
	}
}

func TestE2E_ManualInvalidPortKeyFails(t *testing.T) {
	binPath := buildAutoportBinary(t)
	projectDir := t.TempDir()

	cmd := exec.Command(binPath, "-k", "BAD-KEY")
	cmd.Dir = projectDir
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected failure for invalid key, got success:\n%s", string(output))
	}
	if !strings.Contains(string(output), `invalid env key "BAD-KEY"`) {
		t.Fatalf("expected invalid key error, got:\n%s", string(output))
	}
}
