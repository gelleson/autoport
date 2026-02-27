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
	var config Config
	config.Presets = make(map[string]Preset)

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err == nil {
			var localConfig Config
			if err := json.Unmarshal(data, &localConfig); err == nil {
				for k, v := range localConfig.Presets {
					config.Presets[k] = v
				}
			}
		}
	}
	return &config
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
