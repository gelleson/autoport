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
		"presets": {
			"web": { "ignore": ["AWS_"], "range": "8000-9000" }
		}
	}`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(configB, []byte(`{
		"presets": {
			"web": { "ignore": ["GCP_"], "range": "9000-10000" },
			"db2": { "ignore": ["REDIS_"] }
		}
	}`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("single file", func(t *testing.T) {
		cfg := Load([]string{configA})
		expected := &Config{
			Presets: map[string]Preset{
				"web": {Ignore: []string{"AWS_"}, Range: "8000-9000"},
			},
		}
		if !reflect.DeepEqual(cfg, expected) {
			t.Errorf("Load() = %v, want %v", cfg, expected)
		}
	})

	t.Run("multiple files merge override", func(t *testing.T) {
		cfg := Load([]string{configA, configB})
		expected := &Config{
			Presets: map[string]Preset{
				"web": {Ignore: []string{"GCP_"}, Range: "9000-10000"},
				"db2": {Ignore: []string{"REDIS_"}, Range: ""},
			},
		}
		if !reflect.DeepEqual(cfg, expected) {
			t.Errorf("Load() = %v, want %v", cfg, expected)
		}
	})

	t.Run("non-existent file", func(t *testing.T) {
		cfg := Load([]string{"/path/to/nowhere.json"})
		if len(cfg.Presets) != 0 {
			t.Errorf("Load() should be empty for non-existent files")
		}
	})

	t.Run("invalid json is ignored", func(t *testing.T) {
		broken := filepath.Join(tmpDir, "broken.json")
		if err := os.WriteFile(broken, []byte(`{"presets":`), 0644); err != nil {
			t.Fatal(err)
		}

		cfg := Load([]string{broken, configA})
		expected := &Config{
			Presets: map[string]Preset{
				"web": {Ignore: []string{"AWS_"}, Range: "8000-9000"},
			},
		}
		if !reflect.DeepEqual(cfg, expected) {
			t.Errorf("Load() = %v, want %v", cfg, expected)
		}
	})
}
