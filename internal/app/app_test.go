package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"reflect"
	"strings"
	"testing"

	"github.com/gelleson/autoport/internal/config"
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
		WithConfig(&config.Config{}),
		WithStdout(&stdout),
		WithEnviron([]string{"PORT=8080", "IGNORE_PORT=9090"}),
		WithIsFree(func(p int) bool { return true }),
	)

	opts := Options{
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
		WithConfig(&config.Config{}),
		WithExecutor(mockExec),
		WithStdout(&stdout),
		WithStderr(&stderr),
		WithEnviron([]string{"PORT=8080"}),
		WithIsFree(func(p int) bool { return true }),
	)

	opts := Options{
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
	if !strings.Contains(logOutput, "npm start") {
		t.Fatalf("Expected command context in stderr, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "| ENV") {
		t.Fatalf("Expected table header in stderr, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "| PORT") {
		t.Fatalf("Expected PORT in override summary, got: %s", logOutput)
	}
}

func TestApp_Run_PresetNotFound(t *testing.T) {
	var stderr bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	app := New(
		WithConfig(&config.Config{}),
		WithLogger(logger),
		WithIsFree(func(p int) bool { return true }),
	)

	opts := Options{
		Presets: []string{"nonexistent"},
		CWD:     "/test/path",
	}

	err := app.Run(context.Background(), opts, []string{})
	if err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}

	if !strings.Contains(stderr.String(), "preset not found") {
		t.Errorf("Expected warning, got: %s", stderr.String())
	}
}

func TestApp_resolvePresetOverrides(t *testing.T) {
	app := New(WithConfig(&config.Config{
		Presets: map[string]config.Preset{
			"web": {
				Ignore: []string{"WEB_"},
				Range:  "8000-9000",
			},
		},
	}))

	ignores, rangeSpec := app.resolvePresetOverrides(Options{
		Ignores: []string{"CUSTOM_"},
		Presets: []string{"db", "web"},
	})

	if rangeSpec != "8000-9000" {
		t.Fatalf("resolvePresetOverrides() range = %s, want 8000-9000", rangeSpec)
	}

	if !reflect.DeepEqual(ignores, []string{"CUSTOM_", "DB", "DATABASE", "POSTGRES", "MYSQL", "MONGO", "REDIS", "MEMCACHED", "ES", "CLICKHOUSE", "INFLUX", "WEB_"}) {
		t.Fatalf("resolvePresetOverrides() ignores = %v", ignores)
	}
}

func TestApp_resolvePresetOverrides_ExplicitRangeWins(t *testing.T) {
	app := New(WithConfig(&config.Config{
		Presets: map[string]config.Preset{
			"web": {
				Ignore: []string{"WEB_"},
				Range:  "8000-9000",
			},
		},
	}))

	_, rangeSpec := app.resolvePresetOverrides(Options{
		Presets: []string{"web"},
		Range:   "3000-3999",
	})
	if rangeSpec != "3000-3999" {
		t.Fatalf("resolvePresetOverrides() range = %s, want 3000-3999", rangeSpec)
	}
}

func TestApp_printExports_Sorted(t *testing.T) {
	var stdout bytes.Buffer
	app := New(WithStdout(&stdout))
	app.printExports(map[string]string{
		"Z_PORT": "12000",
		"A_PORT": "11000",
	})

	if got, want := stdout.String(), "export A_PORT=11000\nexport Z_PORT=12000\n"; got != want {
		t.Fatalf("printExports() = %q, want %q", got, want)
	}
}

func TestApp_Run_ManualPortEnvKeys(t *testing.T) {
	var stdout bytes.Buffer
	app := New(
		WithConfig(&config.Config{}),
		WithStdout(&stdout),
		WithEnviron([]string{}),
		WithIsFree(func(p int) bool { return true }),
	)

	opts := Options{
		PortEnv: []string{"WEB_PORT"},
		Range:   "10000-11000",
		CWD:     "/test/path",
	}

	err := app.Run(context.Background(), opts, []string{})
	if err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "export PORT=") {
		t.Fatalf("Expected PORT fallback to be exported, got: %s", out)
	}
	if !strings.Contains(out, "export WEB_PORT=") {
		t.Fatalf("Expected WEB_PORT to be exported, got: %s", out)
	}
}

func TestApp_Run_InvalidManualPortEnvKey(t *testing.T) {
	app := New(
		WithConfig(&config.Config{}),
		WithEnviron([]string{}),
		WithIsFree(func(p int) bool { return true }),
	)

	opts := Options{
		PortEnv: []string{"BAD-KEY"},
		Range:   "10000-11000",
		CWD:     "/test/path",
	}

	err := app.Run(context.Background(), opts, []string{})
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
		WithConfig(&config.Config{}),
		WithStdout(&stdout),
		WithEnviron([]string{"B_PORT=8080", "A_PORT=9090"}),
		WithIsFree(func(p int) bool { return true }),
	)

	opts := Options{
		Format: "json",
		Range:  "10000-11000",
		CWD:    "/test/path",
	}
	err := app.Run(context.Background(), opts, nil)
	if err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}

	var payload outputPayload
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("json decode: %v", err)
	}

	if payload.Mode != "export" {
		t.Fatalf("payload mode = %q, want export", payload.Mode)
	}
	if payload.CWD != "/test/path" {
		t.Fatalf("payload cwd = %q, want /test/path", payload.CWD)
	}
	if payload.Range != "10000-11000" {
		t.Fatalf("payload range = %q, want 10000-11000", payload.Range)
	}
	if len(payload.Command) != 0 {
		t.Fatalf("payload command = %v, want empty", payload.Command)
	}
	if len(payload.Overrides) < 2 {
		t.Fatalf("expected at least 2 overrides, got %v", payload.Overrides)
	}
	if payload.Overrides[0].Key != "A_PORT" || payload.Overrides[1].Key != "B_PORT" {
		t.Fatalf("expected sorted overrides, got %v", payload.Overrides)
	}
}

func TestApp_Run_JSONCommandSummaryToStderr(t *testing.T) {
	mockExec := &MockExecutor{}
	var stderr bytes.Buffer
	app := New(
		WithConfig(&config.Config{}),
		WithExecutor(mockExec),
		WithStderr(&stderr),
		WithEnviron([]string{"PORT=8080"}),
		WithIsFree(func(p int) bool { return true }),
	)

	opts := Options{
		Format: "json",
		Range:  "10000-11000",
		CWD:    "/test/path",
	}
	err := app.Run(context.Background(), opts, []string{"npm", "start"})
	if err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}

	var payload outputPayload
	if err := json.Unmarshal(stderr.Bytes(), &payload); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	if payload.Mode != "execute" {
		t.Fatalf("payload mode = %q, want execute", payload.Mode)
	}
	if !reflect.DeepEqual(payload.Command, []string{"npm", "start"}) {
		t.Fatalf("payload command = %v", payload.Command)
	}
}

func TestApp_Run_QuietSuppressesCommandSummary(t *testing.T) {
	mockExec := &MockExecutor{}
	var stderr bytes.Buffer
	app := New(
		WithConfig(&config.Config{}),
		WithExecutor(mockExec),
		WithStderr(&stderr),
		WithEnviron([]string{"PORT=8080"}),
		WithIsFree(func(p int) bool { return true }),
	)

	opts := Options{
		Quiet: true,
		Range: "10000-11000",
		CWD:   "/test/path",
	}
	err := app.Run(context.Background(), opts, []string{"npm", "start"})
	if err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no summary output in quiet mode, got %q", stderr.String())
	}
}

func TestApp_Run_DryRun_DoesNotExecute(t *testing.T) {
	mockExec := &MockExecutor{}
	var stderr bytes.Buffer
	app := New(
		WithConfig(&config.Config{}),
		WithExecutor(mockExec),
		WithStderr(&stderr),
		WithEnviron([]string{"PORT=8080"}),
		WithIsFree(func(p int) bool { return true }),
	)

	opts := Options{
		DryRun: true,
		Range:  "10000-11000",
		CWD:    "/test/path",
	}
	err := app.Run(context.Background(), opts, []string{"npm", "start"})
	if err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}
	if mockExec.CapturedName != "" {
		t.Fatalf("expected command not to execute, got %q", mockExec.CapturedName)
	}
	if !strings.Contains(stderr.String(), "autoport overrides") {
		t.Fatalf("expected preview summary output, got: %s", stderr.String())
	}
}

func TestApp_Run_InvalidFormat(t *testing.T) {
	app := New(
		WithConfig(&config.Config{}),
		WithEnviron([]string{"PORT=8080"}),
		WithIsFree(func(p int) bool { return true }),
	)
	err := app.Run(context.Background(), Options{
		Format: "yaml",
		Range:  "10000-11000",
		CWD:    "/test/path",
	}, nil)
	if err == nil {
		t.Fatal("Run() expected invalid format error")
	}
}
