package lockfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteReadRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, FileName)
	overrides := map[string]string{"A_PORT": "10001", "B_PORT": "10002"}

	if err := Write(path, tmp, "10000-10100", overrides); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	lf, err := Read(path)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if lf.Range != "10000-10100" {
		t.Fatalf("range=%q", lf.Range)
	}
	if lf.CWDFingerprint != Fingerprint(tmp) {
		t.Fatalf("fingerprint mismatch")
	}
	if len(lf.Assignments) != 2 {
		t.Fatalf("assignments=%d", len(lf.Assignments))
	}
}

func TestRead_UnsupportedVersion(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, FileName)
	if err := os.WriteFile(path, []byte(`{"version":2}`), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(path); err == nil {
		t.Fatalf("expected version error")
	}
}
