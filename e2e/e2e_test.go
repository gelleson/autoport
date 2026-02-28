//go:build e2e

package e2e_test

import (
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
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

func requireTCPBindCapability(t *testing.T) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("skipping e2e test: tcp bind unavailable: %v", err)
	}
	_ = ln.Close()
}

func TestE2E_ExportsPortWhenNoEnvFound(t *testing.T) {
	requireTCPBindCapability(t)

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
	requireTCPBindCapability(t)

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

func TestE2E_PortRangeFlag(t *testing.T) {
	requireTCPBindCapability(t)

	binPath := buildAutoportBinary(t)
	projectDir := t.TempDir()

	cmd := exec.Command(binPath, "-r", "3000-3005", "-k", "CUSTOM_PORT", "sh", "-c", "echo $CUSTOM_PORT")
	cmd.Dir = projectDir
	stdout, err := cmd.Output()
	if err != nil {
		t.Fatalf("run autoport with range: %v", err)
	}

	portStr := strings.TrimSpace(string(stdout))
	portNum, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("expected numeric CUSTOM_PORT, got %q", portStr)
	}

	if portNum < 3000 || portNum > 3005 {
		t.Fatalf("expected CUSTOM_PORT between 3000 and 3005, got %d", portNum)
	}
}

func TestE2E_IgnoreFlag(t *testing.T) {
	requireTCPBindCapability(t)

	binPath := buildAutoportBinary(t)
	projectDir := t.TempDir()

	// Create a .env file
	envContent := "FOO_PORT=8080\nBAR_PORT=9090\n"
	err := os.WriteFile(filepath.Join(projectDir, ".env"), []byte(envContent), 0644)
	if err != nil {
		t.Fatalf("create .env: %v", err)
	}

	// Run without flags, should see both FOO_PORT and BAR_PORT
	cmd := exec.Command(binPath)
	cmd.Dir = projectDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run autoport: %v\n%s", err, string(output))
	}
	outStr := string(output)
	if !strings.Contains(outStr, "export FOO_PORT=") {
		t.Fatalf("expected FOO_PORT in output, got:\n%s", outStr)
	}
	if !strings.Contains(outStr, "export BAR_PORT=") {
		t.Fatalf("expected BAR_PORT in output, got:\n%s", outStr)
	}

	// Run with -i FOO_, should see BAR_PORT but NOT FOO_PORT
	cmd = exec.Command(binPath, "-i", "FOO_")
	cmd.Dir = projectDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run autoport with ignore: %v\n%s", err, string(output))
	}
	outStr = string(output)
	if strings.Contains(outStr, "export FOO_PORT=") {
		t.Fatalf("expected FOO_PORT to be ignored, but found in output:\n%s", outStr)
	}
	if !strings.Contains(outStr, "export BAR_PORT=") {
		t.Fatalf("expected BAR_PORT in output, got:\n%s", outStr)
	}
}

func TestE2E_PresetFlag(t *testing.T) {
	requireTCPBindCapability(t)

	binPath := buildAutoportBinary(t)
	projectDir := t.TempDir()

	// Create a .env file with DB ports
	envContent := "REDIS_PORT=6379\nMONGO_PORT=27017\nOTHER_PORT=1234\n"
	err := os.WriteFile(filepath.Join(projectDir, ".env"), []byte(envContent), 0644)
	if err != nil {
		t.Fatalf("create .env: %v", err)
	}

	// Run with the built-in "db" preset, which ignores REDIS and MONGO
	cmd := exec.Command(binPath, "-p", "db")
	cmd.Dir = projectDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run autoport with preset: %v\n%s", err, string(output))
	}
	outStr := string(output)

	if strings.Contains(outStr, "export REDIS_PORT=") {
		t.Fatalf("expected REDIS_PORT to be ignored by 'db' preset, found in:\n%s", outStr)
	}
	if strings.Contains(outStr, "export MONGO_PORT=") {
		t.Fatalf("expected MONGO_PORT to be ignored by 'db' preset, found in:\n%s", outStr)
	}
	if !strings.Contains(outStr, "export OTHER_PORT=") {
		t.Fatalf("expected OTHER_PORT to be included, found in:\n%s", outStr)
	}
}

func TestE2E_ConfigPreset(t *testing.T) {
	requireTCPBindCapability(t)

	binPath := buildAutoportBinary(t)
	projectDir := t.TempDir()

	// Create .env file
	envContent := "MY_PORT=1234\nYOUR_PORT=5678\n"
	err := os.WriteFile(filepath.Join(projectDir, ".env"), []byte(envContent), 0644)
	if err != nil {
		t.Fatalf("create .env: %v", err)
	}

	// Create .autoport.json
	configContent := `{
		"presets": {
			"custom": {
				"ignore": ["MY_"]
			}
		}
	}`
	err = os.WriteFile(filepath.Join(projectDir, ".autoport.json"), []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("create .autoport.json: %v", err)
	}

	// Run with custom preset
	cmd := exec.Command(binPath, "-p", "custom")
	cmd.Dir = projectDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run autoport with custom preset: %v\n%s", err, string(output))
	}
	outStr := string(output)

	if strings.Contains(outStr, "export MY_PORT=") {
		t.Fatalf("expected MY_PORT to be ignored by 'custom' preset, found in:\n%s", outStr)
	}
	if !strings.Contains(outStr, "export YOUR_PORT=") {
		t.Fatalf("expected YOUR_PORT to be included, found in:\n%s", outStr)
	}
}

func TestE2E_JSONExport(t *testing.T) {
	requireTCPBindCapability(t)

	binPath := buildAutoportBinary(t)
	projectDir := t.TempDir()

	cmd := exec.Command(binPath, "-f", "json")
	cmd.Dir = projectDir
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("run autoport json export: %v", err)
	}

	var payload struct {
		Mode      string `json:"mode"`
		Overrides []struct {
			Key string `json:"key"`
		} `json:"overrides"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		t.Fatalf("parse json output: %v\n%s", err, string(output))
	}
	if payload.Mode != "export" {
		t.Fatalf("expected export mode, got %q", payload.Mode)
	}
	if len(payload.Overrides) == 0 {
		t.Fatalf("expected overrides in output, got %s", string(output))
	}
}

func TestE2E_QuietSuppressesSummary(t *testing.T) {
	requireTCPBindCapability(t)

	binPath := buildAutoportBinary(t)
	projectDir := t.TempDir()

	cmd := exec.Command(binPath, "-q", "sh", "-c", "echo ok")
	cmd.Dir = projectDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run autoport quiet mode: %v\n%s", err, string(output))
	}

	if strings.TrimSpace(string(output)) != "ok" {
		t.Fatalf("expected only command output, got %q", string(output))
	}
}

func TestE2E_DryRunDoesNotExecuteCommand(t *testing.T) {
	requireTCPBindCapability(t)

	binPath := buildAutoportBinary(t)
	projectDir := t.TempDir()
	marker := filepath.Join(projectDir, "ran.txt")

	cmd := exec.Command(binPath, "-n", "sh", "-c", "echo ran > ran.txt")
	cmd.Dir = projectDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run autoport dry-run mode: %v\n%s", err, string(output))
	}

	if _, err := os.Stat(marker); err == nil {
		t.Fatalf("expected command not to execute in dry-run mode")
	}
	if !strings.Contains(string(output), "autoport overrides") {
		t.Fatalf("expected preview summary output, got:\n%s", string(output))
	}
}

func TestE2E_ExplainJSON(t *testing.T) {
	requireTCPBindCapability(t)

	binPath := buildAutoportBinary(t)
	projectDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectDir, ".env"), []byte("WEB_PORT=3000\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(binPath, "explain", "-f", "json")
	cmd.Dir = projectDir
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("run explain: %v", err)
	}

	var payload struct {
		Mode        string `json:"mode"`
		Assignments []struct {
			Key string `json:"key"`
		} `json:"assignments"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		t.Fatalf("parse explain output: %v", err)
	}
	if payload.Mode != "explain" {
		t.Fatalf("mode=%q", payload.Mode)
	}
	if len(payload.Assignments) == 0 {
		t.Fatalf("expected assignments")
	}
}

func TestE2E_DoctorWarningsExitOne(t *testing.T) {
	requireTCPBindCapability(t)

	binPath := buildAutoportBinary(t)
	projectDir := t.TempDir()
	// Small range should trigger warning.
	cmd := exec.Command(binPath, "doctor", "-r", "10000-10002")
	cmd.Dir = projectDir
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected warning exit")
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected ExitError, got %T", err)
	}
	if exitErr.ExitCode() != 1 {
		t.Fatalf("expected exit code 1, got %d\n%s", exitErr.ExitCode(), string(output))
	}
}

func TestE2E_NamespaceChangesPort(t *testing.T) {
	requireTCPBindCapability(t)

	binPath := buildAutoportBinary(t)
	projectDir := t.TempDir()

	cmdA := exec.Command(binPath, "--namespace", "svc-a", "-k", "WEB_PORT", "sh", "-c", "echo $WEB_PORT")
	cmdA.Dir = projectDir
	outA, err := cmdA.Output()
	if err != nil {
		t.Fatalf("namespace a run: %v", err)
	}

	cmdB := exec.Command(binPath, "--namespace", "svc-b", "-k", "WEB_PORT", "sh", "-c", "echo $WEB_PORT")
	cmdB.Dir = projectDir
	outB, err := cmdB.Output()
	if err != nil {
		t.Fatalf("namespace b run: %v", err)
	}

	if strings.TrimSpace(string(outA)) == strings.TrimSpace(string(outB)) {
		t.Fatalf("expected different ports for different namespaces, got %q", string(outA))
	}
}

func TestE2E_LockAndUseLock(t *testing.T) {
	requireTCPBindCapability(t)

	binPath := buildAutoportBinary(t)
	projectDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectDir, ".env"), []byte("WEB_PORT=3000\n"), 0644); err != nil {
		t.Fatal(err)
	}

	lockCmd := exec.Command(binPath, "lock", "-r", "12000-12010")
	lockCmd.Dir = projectDir
	if output, err := lockCmd.CombinedOutput(); err != nil {
		t.Fatalf("lock command failed: %v\n%s", err, string(output))
	}
	if _, err := os.Stat(filepath.Join(projectDir, ".autoport.lock.json")); err != nil {
		t.Fatalf("expected lockfile: %v", err)
	}

	useCmd := exec.Command(binPath, "--use-lock", "-f", "json")
	useCmd.Dir = projectDir
	output, err := useCmd.Output()
	if err != nil {
		t.Fatalf("use-lock command failed: %v", err)
	}

	var payload struct {
		Overrides []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"overrides"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		t.Fatalf("parse output: %v", err)
	}
	if len(payload.Overrides) == 0 {
		t.Fatalf("expected overrides")
	}
	if payload.Overrides[0].Value < "12000" || payload.Overrides[0].Value > "12010" {
		t.Fatalf("expected lock-range value, got %s", payload.Overrides[0].Value)
	}
}

func TestE2E_ScannerIgnoreDirsAndMaxDepth(t *testing.T) {
	requireTCPBindCapability(t)

	binPath := buildAutoportBinary(t)
	projectDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectDir, "node_modules", "x"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "node_modules", ".env"), []byte("HIDDEN_PORT=3000\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, ".env"), []byte("VISIBLE_PORT=3001\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := `{
  "version": 2,
  "scanner": {"ignore_dirs": ["node_modules"], "max_depth": 1}
}`
	if err := os.WriteFile(filepath.Join(projectDir, ".autoport.json"), []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(binPath)
	cmd.Dir = projectDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run failed: %v\n%s", err, string(output))
	}
	out := string(output)
	if strings.Contains(out, "export HIDDEN_PORT=") {
		t.Fatalf("HIDDEN_PORT should be ignored: %s", out)
	}
	if !strings.Contains(out, "export VISIBLE_PORT=") {
		t.Fatalf("VISIBLE_PORT missing: %s", out)
	}
}
