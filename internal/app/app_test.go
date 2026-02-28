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
