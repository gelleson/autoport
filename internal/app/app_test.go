package app

import (
	"bytes"
	"context"
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
	app := New(
		WithConfig(&config.Config{}),
		WithExecutor(mockExec),
		WithStdout(&stdout),
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
}

func TestApp_Run_PresetNotFound(t *testing.T) {
	var stderr bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	app := New(
		WithConfig(&config.Config{}),
		WithLogger(logger),
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
