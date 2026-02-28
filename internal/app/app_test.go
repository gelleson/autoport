package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gelleson/autoport/internal/config"
	"github.com/gelleson/autoport/internal/lockfile"
	"github.com/gelleson/autoport/pkg/port"
)

type MockExecutor struct {
	CapturedName string
	CapturedArgs []string
	CapturedEnv  []string
	Err          error
}

func (m *MockExecutor) Run(ctx context.Context, name string, args []string, env []string, stdout, stderr io.Writer) error {
	m.CapturedName = name
	m.CapturedArgs = args
	m.CapturedEnv = env
	return m.Err
}

func TestApp_Run_Export(t *testing.T) {
	var stdout bytes.Buffer
	app := New(
		WithConfig(&config.Config{Presets: map[string]config.Preset{}}),
		WithStdout(&stdout),
		WithEnviron([]string{"PORT=8080", "IGNORE_PORT=9090"}),
		WithIsFree(func(p int) bool { return true }),
	)

	opts := Options{
		Mode:    "run",
		Ignores: []string{"IGNORE_"},
		Range:   "10000-11000",
		CWD:     "/test/path",
	}

	err := app.Run(context.Background(), opts, []string{})
	if err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "export PORT=") {
		t.Errorf("Expected PORT to be exported, got: %s", out)
	}
}

func TestApp_Run_Command(t *testing.T) {
	mockExec := &MockExecutor{}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := New(
		WithConfig(&config.Config{Presets: map[string]config.Preset{}}),
		WithExecutor(mockExec),
		WithStdout(&stdout),
		WithStderr(&stderr),
		WithEnviron([]string{"PORT=8080"}),
		WithIsFree(func(p int) bool { return true }),
	)

	opts := Options{
		Mode:  "run",
		Range: "10000-11000",
		CWD:   "/test/path",
	}

	err := app.Run(context.Background(), opts, []string{"npm", "start"})
	if err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}

	if mockExec.CapturedName != "npm" {
		t.Errorf("Expected command 'npm', got %s", mockExec.CapturedName)
	}

	foundPortOverride := false
	for _, e := range mockExec.CapturedEnv {
		if strings.HasPrefix(e, "PORT=") && e != "PORT=8080" {
			foundPortOverride = true
			break
		}
	}
	if !foundPortOverride {
		t.Errorf("Expected PORT override")
	}

	logOutput := stderr.String()
	if !strings.Contains(logOutput, "autoport overrides") {
		t.Fatalf("Expected override summary in stderr, got: %s", logOutput)
	}
}

func TestApp_Run_PresetNotFoundWarns(t *testing.T) {
	var stderr bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	app := New(
		WithConfig(&config.Config{Presets: map[string]config.Preset{}, Strict: false}),
		WithLogger(logger),
		WithIsFree(func(p int) bool { return true }),
	)

	opts := Options{Mode: "run", Presets: []string{"nonexistent"}, CWD: "/test/path"}

	err := app.Run(context.Background(), opts, []string{})
	if err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}
	if !strings.Contains(stderr.String(), "preset not found") {
		t.Errorf("Expected warning, got: %s", stderr.String())
	}
}

func TestApp_Run_StrictPresetNotFoundFails(t *testing.T) {
	app := New(
		WithConfig(&config.Config{Presets: map[string]config.Preset{}, Strict: true}),
		WithIsFree(func(p int) bool { return true }),
	)

	err := app.Run(context.Background(), Options{Mode: "run", Presets: []string{"missing"}, CWD: "/test/path"}, nil)
	if err == nil {
		t.Fatal("expected strict mode error")
	}
}

func TestApp_Run_ManualInvalidPortEnvKey(t *testing.T) {
	app := New(
		WithConfig(&config.Config{Presets: map[string]config.Preset{}}),
		WithEnviron([]string{}),
		WithIsFree(func(p int) bool { return true }),
	)

	err := app.Run(context.Background(), Options{Mode: "run", PortEnv: []string{"BAD-KEY"}, Range: "10000-11000", CWD: "/test/path"}, []string{})
	if err == nil {
		t.Fatal("Run() expected error for invalid manual key")
	}
	if !strings.Contains(err.Error(), `invalid env key "BAD-KEY"`) {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestApp_Run_JSONExport(t *testing.T) {
	var stdout bytes.Buffer
	app := New(
		WithConfig(&config.Config{Presets: map[string]config.Preset{}}),
		WithStdout(&stdout),
		WithEnviron([]string{"B_PORT=8080", "A_PORT=9090"}),
		WithIsFree(func(p int) bool { return true }),
	)

	err := app.Run(context.Background(), Options{Mode: "run", Format: "json", Range: "10000-11000", CWD: "/test/path"}, nil)
	if err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}

	var payload outputPayload
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("json output parse: %v", err)
	}
	if payload.Mode != "export" {
		t.Fatalf("payload.Mode = %q", payload.Mode)
	}
	if len(payload.Overrides) == 0 {
		t.Fatal("expected overrides")
	}
}

func TestApp_Explain_JSON(t *testing.T) {
	var stdout bytes.Buffer
	app := New(
		WithConfig(&config.Config{Presets: map[string]config.Preset{}}),
		WithStdout(&stdout),
		WithEnviron([]string{"WEB_PORT=3000"}),
		WithIsFree(func(p int) bool { return true }),
	)

	err := app.Run(context.Background(), Options{Mode: "explain", Format: "json", Range: "10000-11000", CWD: "/test/path"}, nil)
	if err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}

	var payload explainPayload
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("json parse: %v", err)
	}
	if payload.Mode != "explain" {
		t.Fatalf("mode=%q", payload.Mode)
	}
	if len(payload.Assignments) == 0 {
		t.Fatalf("expected assignments")
	}
}

func TestApp_Doctor_ExitWarning(t *testing.T) {
	var stdout bytes.Buffer
	app := New(
		WithConfig(&config.Config{Presets: map[string]config.Preset{}, Warnings: []string{"deprecated"}}),
		WithStdout(&stdout),
		WithIsFree(func(p int) bool { return true }),
	)

	err := app.Run(context.Background(), Options{Mode: "doctor", CWD: "/test/path"}, nil)
	if err == nil {
		t.Fatalf("expected warning exit")
	}
	e, ok := err.(*ExitError)
	if !ok || e.Code != 1 {
		t.Fatalf("unexpected error: %T %v", err, err)
	}
}

func TestApp_Lock_WriteAndUse(t *testing.T) {
	tmp := t.TempDir()
	var stdout bytes.Buffer
	app := New(
		WithConfig(&config.Config{Presets: map[string]config.Preset{}}),
		WithStdout(&stdout),
		WithEnviron([]string{"WEB_PORT=3000"}),
		WithIsFree(func(p int) bool { return true }),
	)

	err := app.Run(context.Background(), Options{Mode: "lock", Range: "10000-10010", CWD: tmp}, nil)
	if err != nil {
		t.Fatalf("lock run error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, lockfile.FileName)); err != nil {
		t.Fatalf("expected lockfile: %v", err)
	}

	stdout.Reset()
	err = app.Run(context.Background(), Options{Mode: "run", UseLock: true, Range: "10000-10010", CWD: tmp, Format: "json"}, nil)
	if err != nil {
		t.Fatalf("use-lock run error: %v", err)
	}
	if !strings.Contains(stdout.String(), "WEB_PORT") {
		t.Fatalf("expected WEB_PORT output")
	}
}

func TestApp_Run_NewFormats(t *testing.T) {
	cases := []string{"dotenv", "yaml"}
	for _, format := range cases {
		t.Run(format, func(t *testing.T) {
			var stdout bytes.Buffer
			app := New(
				WithConfig(&config.Config{Presets: map[string]config.Preset{}}),
				WithStdout(&stdout),
				WithEnviron([]string{"WEB_PORT=3000"}),
				WithIsFree(func(p int) bool { return true }),
			)
			err := app.Run(context.Background(), Options{Mode: "run", Format: format, Range: "10000-11000", CWD: "/test/path"}, nil)
			if err != nil {
				t.Fatalf("Run() error: %v", err)
			}
			if !strings.Contains(stdout.String(), "WEB_PORT") {
				t.Fatalf("expected WEB_PORT in output: %s", stdout.String())
			}
		})
	}
}

func TestApp_Run_BranchAwareSeedUsesResolvedBranch(t *testing.T) {
	var stdoutA bytes.Buffer
	var stdoutB bytes.Buffer
	cfg := &config.Config{Presets: map[string]config.Preset{}}

	appA := New(
		WithConfig(cfg),
		WithStdout(&stdoutA),
		WithEnviron([]string{}),
		WithIsFree(func(p int) bool { return true }),
		WithBranchResolver(func(repo string) (string, error) { return "branch-a", nil }),
	)
	appB := New(
		WithConfig(cfg),
		WithStdout(&stdoutB),
		WithEnviron([]string{}),
		WithIsFree(func(p int) bool { return true }),
		WithBranchResolver(func(repo string) (string, error) { return "branch-b", nil }),
	)

	opts := Options{Mode: "run", Format: "json", CWD: "/test/path", SeedBranch: true}
	if err := appA.Run(context.Background(), opts, nil); err != nil {
		t.Fatal(err)
	}
	if err := appB.Run(context.Background(), opts, nil); err != nil {
		t.Fatal(err)
	}

	var payloadA outputPayload
	var payloadB outputPayload
	if err := json.Unmarshal(stdoutA.Bytes(), &payloadA); err != nil {
		t.Fatalf("json A parse: %v", err)
	}
	if err := json.Unmarshal(stdoutB.Bytes(), &payloadB); err != nil {
		t.Fatalf("json B parse: %v", err)
	}
	getPort := func(p outputPayload) string {
		for _, b := range p.Overrides {
			if strings.EqualFold(b.Key, "PORT") {
				return b.Value
			}
		}
		return ""
	}
	if getPort(payloadA) == "" || getPort(payloadB) == "" {
		t.Fatalf("expected PORT in both payloads")
	}
	if getPort(payloadA) == getPort(payloadB) {
		t.Fatalf("expected different PORT values for different branches; both were %q", getPort(payloadA))
	}
}

func TestApp_Run_ExplicitTargetEnvRewrite(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()
	targetEnv := filepath.Join(targetDir, ".env")
	if err := os.WriteFile(filepath.Join(sourceDir, ".env"), []byte("monitoring_url=http://localhost:31413/rpc\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(targetEnv, []byte("app_port=31413\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	app := New(
		WithConfig(&config.Config{Presets: map[string]config.Preset{}}),
		WithStdout(&stdout),
		WithEnviron([]string{}),
		WithIsFree(func(p int) bool { return true }),
		WithBranchResolver(func(repo string) (string, error) { return "feature-x", nil }),
	)

	spec := "monitoring_url=" + targetEnv + ":app_port"
	opts := Options{
		Mode:           "run",
		Format:         "json",
		CWD:            sourceDir,
		Range:          "12000-12010",
		SeedBranch:     true,
		TargetEnvSpecs: []string{spec},
	}
	if err := app.Run(context.Background(), opts, nil); err != nil {
		t.Fatal(err)
	}

	var payload outputPayload
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("json parse: %v", err)
	}
	var monitoring string
	for _, b := range payload.Overrides {
		if b.Key == "monitoring_url" {
			monitoring = b.Value
		}
	}
	if monitoring == "" {
		t.Fatalf("expected monitoring_url override in %+v", payload.Overrides)
	}
	_, rewrittenPort, err := parseLoopbackURL(monitoring)
	if err != nil {
		t.Fatalf("expected rewritten localhost URL, got %q (%v)", monitoring, err)
	}
	targetSeed := port.SeedFor(targetDir, appendBranchNamespace("", "feature-x"))
	expected := 12000 + (int(targetSeed)+1)%11 // keys are PORT, app_port => app_port index 1
	if rewrittenPort != expected {
		t.Fatalf("rewritten port=%d, expected=%d", rewrittenPort, expected)
	}
}

func TestApp_Run_SmartTargetEnvRewrite(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()
	targetEnv := filepath.Join(targetDir, ".env")
	if err := os.WriteFile(filepath.Join(sourceDir, ".env"), []byte("monitoring_url=http://localhost:31413/rpc\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(targetEnv, []byte("app_port=31413\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	app := New(
		WithConfig(&config.Config{Presets: map[string]config.Preset{}}),
		WithStdout(&stdout),
		WithEnviron([]string{}),
		WithIsFree(func(p int) bool { return true }),
		WithBranchResolver(func(repo string) (string, error) { return "feature-x", nil }),
	)
	opts := Options{
		Mode:           "run",
		Format:         "json",
		CWD:            sourceDir,
		Range:          "12000-12010",
		SeedBranch:     true,
		TargetEnvSpecs: []string{targetEnv},
	}
	if err := app.Run(context.Background(), opts, nil); err != nil {
		t.Fatal(err)
	}

	var payload outputPayload
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("json parse: %v", err)
	}
	var found bool
	for _, b := range payload.Overrides {
		if b.Key == "monitoring_url" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected smart rewrite to produce monitoring_url override")
	}
}

func TestApp_Run_BranchMismatchWarnsAndSkips(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()
	targetEnv := filepath.Join(targetDir, ".env")
	if err := os.WriteFile(filepath.Join(sourceDir, ".env"), []byte("monitoring_url=http://localhost:31413/rpc\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(targetEnv, []byte("app_port=31413\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	app := New(
		WithConfig(&config.Config{Presets: map[string]config.Preset{}}),
		WithStdout(&stdout),
		WithEnviron([]string{}),
		WithIsFree(func(p int) bool { return true }),
		WithBranchResolver(func(repo string) (string, error) {
			if repo == sourceDir {
				return "branch-a", nil
			}
			return "branch-b", nil
		}),
	)
	spec := "monitoring_url=" + targetEnv + ":app_port"
	opts := Options{
		Mode:           "run",
		Format:         "json",
		CWD:            sourceDir,
		Range:          "12000-12010",
		SeedBranch:     true,
		TargetEnvSpecs: []string{spec},
	}
	if err := app.Run(context.Background(), opts, nil); err != nil {
		t.Fatal(err)
	}

	var payload outputPayload
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("json parse: %v", err)
	}
	for _, b := range payload.Overrides {
		if b.Key == "monitoring_url" {
			t.Fatalf("monitoring_url should not be rewritten on branch mismatch")
		}
	}
	joined := strings.Join(payload.Warnings, "\n")
	if !strings.Contains(joined, "branch mismatch") {
		t.Fatalf("expected branch mismatch warning, got %q", joined)
	}
}

func TestApp_Run_TargetLockfilePreferred(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()
	targetEnv := filepath.Join(targetDir, ".env")
	if err := os.WriteFile(filepath.Join(sourceDir, ".env"), []byte("monitoring_url=http://localhost:31413/rpc\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(targetEnv, []byte("app_port=31413\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := lockfile.Write(lockfile.PathFor(targetDir), targetDir, "12000-12010", map[string]string{"app_port": "18080"}); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	app := New(
		WithConfig(&config.Config{Presets: map[string]config.Preset{}}),
		WithStdout(&stdout),
		WithEnviron([]string{}),
		WithIsFree(func(p int) bool { return true }),
		WithBranchResolver(func(repo string) (string, error) { return "feature-x", nil }),
	)
	spec := "monitoring_url=" + targetEnv + ":app_port"
	opts := Options{
		Mode:           "explain",
		Format:         "json",
		CWD:            sourceDir,
		Range:          "12000-12010",
		SeedBranch:     true,
		TargetEnvSpecs: []string{spec},
	}
	if err := app.Run(context.Background(), opts, nil); err != nil {
		t.Fatal(err)
	}

	var payload explainPayload
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("json parse: %v", err)
	}
	if len(payload.LinkRewrites) != 1 {
		t.Fatalf("expected one link rewrite, got %d", len(payload.LinkRewrites))
	}
	if payload.LinkRewrites[0].PortSource != "lockfile" {
		t.Fatalf("expected lockfile source, got %q", payload.LinkRewrites[0].PortSource)
	}
	_, gotPort, err := parseLoopbackURL(payload.LinkRewrites[0].NewValue)
	if err != nil {
		t.Fatalf("parse rewritten URL: %v", err)
	}
	if gotPort != 18080 {
		t.Fatalf("expected lockfile port 18080, got %d", gotPort)
	}
}

func TestApp_Run_ConfigLinkFallbackDeterministic(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceDir, ".env"), []byte("monitoring_url=http://localhost:31413/rpc\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, ".env"), []byte("app_port=31413\n"), 0644); err != nil {
		t.Fatal(err)
	}

	relativeTarget, err := filepath.Rel(sourceDir, targetDir)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		Presets: map[string]config.Preset{},
		Links: []config.LinkRule{
			{
				SourceKey:     "monitoring_url",
				TargetRepo:    relativeTarget,
				TargetPortKey: "app_port",
			},
		},
	}
	var stdout bytes.Buffer
	app := New(
		WithConfig(cfg),
		WithStdout(&stdout),
		WithEnviron([]string{}),
		WithIsFree(func(p int) bool { return true }),
		WithBranchResolver(func(repo string) (string, error) { return "feature-x", nil }),
	)
	opts := Options{
		Mode:       "explain",
		Format:     "json",
		CWD:        sourceDir,
		Range:      "12000-12010",
		SeedBranch: true,
	}
	if err := app.Run(context.Background(), opts, nil); err != nil {
		t.Fatal(err)
	}

	var payload explainPayload
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("json parse: %v", err)
	}
	if len(payload.LinkRewrites) != 1 {
		t.Fatalf("expected one link rewrite, got %d", len(payload.LinkRewrites))
	}
	if payload.LinkRewrites[0].PortSource != "deterministic" {
		t.Fatalf("expected deterministic source, got %q", payload.LinkRewrites[0].PortSource)
	}
	_, rewrittenPort, err := parseLoopbackURL(payload.LinkRewrites[0].NewValue)
	if err != nil {
		t.Fatal(err)
	}
	seed := port.SeedFor(targetDir, appendBranchNamespace("", "feature-x"))
	expected := 12000 + (int(seed)+1)%11
	if rewrittenPort != expected {
		t.Fatalf("deterministic port=%d, expected=%d", rewrittenPort, expected)
	}
}
