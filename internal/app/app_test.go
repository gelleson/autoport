package app

import (
	"bytes"
	"context"
	"io"
	"log/slog"
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
