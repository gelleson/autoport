package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoad(t *testing.T) {
	tmpDir := t.TempDir()
	configA := filepath.Join(tmpDir, "configA.json")
	configB := filepath.Join(tmpDir, "configB.json")

	err := os.WriteFile(configA, []byte(`{
		"version": 2,
		"presets": {
			"web": { "ignore_prefixes": ["AWS_"], "range": "8000-9000", "include_keys": ["WEB_PORT"] }
		}
	}`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(configB, []byte(`{
		"strict": true,
		"scanner": {"ignore_dirs": ["node_modules"], "max_depth": 3},
		"presets": {
			"web": { "ignore_prefixes": ["GCP_"], "range": "9000-10000" },
			"db2": { "ignore_prefixes": ["REDIS_"] }
		}
	}`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("single file", func(t *testing.T) {
		cfg := Load([]string{configA})
		expected := &Config{
			Version: 2,
			Presets: map[string]Preset{
				"web": {IgnorePrefixes: []string{"AWS_"}, Range: "8000-9000", IncludeKeys: []string{"WEB_PORT"}},
			},
		}
		if !reflect.DeepEqual(cfg.Presets, expected.Presets) || cfg.Version != expected.Version {
			t.Errorf("Load() = %v, want %v", cfg, expected)
		}
	})

	t.Run("multiple files merge override", func(t *testing.T) {
		cfg := Load([]string{configA, configB})
		expected := map[string]Preset{
			"web": {IgnorePrefixes: []string{"GCP_"}, Range: "9000-10000"},
			"db2": {IgnorePrefixes: []string{"REDIS_"}, Range: ""},
		}
		if !reflect.DeepEqual(cfg.Presets, expected) {
			t.Errorf("Load() presets = %v, want %v", cfg.Presets, expected)
		}
		if !cfg.Strict {
			t.Fatalf("expected strict=true")
		}
		if cfg.Scanner.MaxDepth != 3 || !reflect.DeepEqual(cfg.Scanner.IgnoreDirs, []string{"node_modules"}) {
			t.Fatalf("unexpected scanner config: %+v", cfg.Scanner)
		}
	})

	t.Run("non-existent file", func(t *testing.T) {
		cfg := Load([]string{"/path/to/nowhere.json"})
		if len(cfg.Presets) != 0 {
			t.Errorf("Load() should be empty for non-existent files")
		}
	})

	t.Run("invalid json is reported", func(t *testing.T) {
		broken := filepath.Join(tmpDir, "broken.json")
		if err := os.WriteFile(broken, []byte(`{"presets":`), 0644); err != nil {
			t.Fatal(err)
		}

		cfg := Load([]string{broken})
		if !cfg.HasErrors() {
			t.Fatalf("expected parse errors")
		}
	})
}

func TestLoad_LegacyIgnoreMapping(t *testing.T) {
	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "legacy.json")
	if err := os.WriteFile(p, []byte(`{
		"presets": {
			"web": {"ignore": ["OLD_"]}
		}
	}`), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := Load([]string{p})
	if got := cfg.Presets["web"].IgnorePrefixes; !reflect.DeepEqual(got, []string{"OLD_"}) {
		t.Fatalf("IgnorePrefixes = %v", got)
	}
	if len(cfg.Warnings) == 0 {
		t.Fatalf("expected migration warning")
	}
}
