package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Preset represents configuration overrides.
type Preset struct {
	Ignore []string `json:"ignore"`
	Range  string   `json:"range"`
}

// Config stores global and preset configurations.
type Config struct {
	Presets map[string]Preset `json:"presets"`
}

// BuiltInPresets are predefined, hardcoded configurations.
var BuiltInPresets = map[string]Preset{
	"db": {
		Ignore: []string{"DB", "DATABASE", "POSTGRES", "MYSQL", "MONGO", "REDIS", "MEMCACHED", "ES", "CLICKHOUSE", "INFLUX"},
	},
}

// Load reads configuration from the provided file paths, merging them in order.
func Load(paths []string) *Config {
	cfg := &Config{
		Presets: make(map[string]Preset),
	}

	for _, path := range paths {
		localConfig, ok := loadFile(path)
		if !ok {
			continue
		}
		mergePresets(cfg.Presets, localConfig.Presets)
	}
	return cfg
}

// LoadDefault loads configurations from default locations: home dir and current dir.
func LoadDefault() *Config {
	home, _ := os.UserHomeDir()
	paths := []string{
		filepath.Join(home, ".autoport.json"),
		".autoport.json",
	}
	return Load(paths)
}

func loadFile(path string) (Config, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, false
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, false
	}

	return cfg, true
}

func mergePresets(dst, src map[string]Preset) {
	for key, value := range src {
		dst[key] = value
	}
}
