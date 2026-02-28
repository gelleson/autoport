package scanner

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestScanner_ScanEnv(t *testing.T) {
	environ := []string{
		"PORT=8080",
		"API_PORT=9090",
		"DB_PORT=5432",
		"OTHER_VAR=123",
		"INVALID_ENV_VAR",
	}
	ignores := []string{"DB_"}

	tmpDir := t.TempDir()

	s := New(tmpDir, WithIgnores(ignores), WithEnviron(environ))
	got, err := s.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"API_PORT", "PORT"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("Scanner.Scan() = %v, want %v", got, want)
	}
}

func TestScanner_ScanFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a dummy .env file
	envContent := []byte("WEB_PORT=3000\nREDIS_PORT=6379\n")
	err := os.WriteFile(filepath.Join(tmpDir, ".env"), envContent, 0644)
	if err != nil {
		t.Fatal(err)
	}

	ignores := []string{"REDIS_"}
	s := New(tmpDir, WithIgnores(ignores), WithEnviron([]string{}))
	got, err := s.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"PORT", "WEB_PORT"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("Scanner.Scan() = %v, want %v", got, want)
	}
}

func TestScanner_Cancel(t *testing.T) {
	tmpDir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	s := New(tmpDir)
	_, err := s.Scan(ctx)
	if err == nil {
		t.Error("Expected error from cancelled context")
	}
}

func TestScanner_ScanFiles_SkipsHiddenDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	hiddenDir := filepath.Join(tmpDir, ".hidden")
	if err := os.Mkdir(hiddenDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hiddenDir, ".env"), []byte("HIDDEN_PORT=3001\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, ".env"), []byte("VISIBLE_PORT=3000\n"), 0644); err != nil {
		t.Fatal(err)
	}

	s := New(tmpDir, WithEnviron([]string{}))
	got, err := s.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"PORT", "VISIBLE_PORT"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Scanner.Scan() = %v, want %v", got, want)
	}
}

func TestScanner_ScanDetailed_StatsAndSources(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(tmpDir, "node_modules"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, ".env.local"), []byte("A_PORT=3000\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "node_modules", ".env"), []byte("B_PORT=3001\n"), 0644); err != nil {
		t.Fatal(err)
	}

	s := New(tmpDir,
		WithEnviron([]string{"C_PORT=3333"}),
		WithIgnoreDirs([]string{"node_modules"}),
	)

	discoveries, stats, err := s.ScanDetailed(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	found := map[string]string{}
	for _, d := range discoveries {
		found[d.Key] = d.Source
	}
	if found["A_PORT"] != ".env.local" {
		t.Fatalf("A_PORT source = %q", found["A_PORT"])
	}
	if found["C_PORT"] != "env" {
		t.Fatalf("C_PORT source = %q", found["C_PORT"])
	}
	if _, ok := found["B_PORT"]; ok {
		t.Fatalf("B_PORT should be skipped via ignore dir")
	}
	if stats.SkippedIgnore == 0 {
		t.Fatalf("expected ignored directories count")
	}
}
