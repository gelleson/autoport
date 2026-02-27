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
