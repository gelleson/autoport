package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Preset represents configuration overrides.
type Preset struct {
	Range          string   `json:"range"`
	IgnorePrefixes []string `json:"ignore_prefixes,omitempty"`
	IncludeKeys    []string `json:"include_keys,omitempty"`
	ExcludeKeys    []string `json:"exclude_keys,omitempty"`

	// Legacy v1 field, mapped to IgnorePrefixes with warnings.
	Ignore []string `json:"ignore,omitempty"`
}

// ScannerConfig controls repository scanning behavior.
type ScannerConfig struct {
	IgnoreDirs []string `json:"ignore_dirs,omitempty"`
	MaxDepth   int      `json:"max_depth,omitempty"`
}

// LinkRule describes how to rewrite a source URL key based on another repository's deterministic port.
type LinkRule struct {
	SourceKey       string `json:"source_key"`
	TargetRepo      string `json:"target_repo"`
	TargetPortKey   string `json:"target_port_key,omitempty"`
	TargetNamespace string `json:"target_namespace,omitempty"`
	SameBranch      *bool  `json:"same_branch,omitempty"`
}

// Config stores global and preset configurations.
type Config struct {
	Version  int               `json:"version,omitempty"`
	Strict   bool              `json:"strict,omitempty"`
	Scanner  ScannerConfig     `json:"scanner,omitempty"`
	Presets  map[string]Preset `json:"presets"`
	Links    []LinkRule        `json:"links,omitempty"`
	Warnings []string          `json:"-"`
	Errors   []error           `json:"-"`
}

// BuiltInPresets are predefined, hardcoded configurations.
var BuiltInPresets = map[string]Preset{
	"db": {
		IgnorePrefixes: []string{"DB", "DATABASE", "POSTGRES", "MYSQL", "MONGO", "REDIS", "MEMCACHED", "ES", "CLICKHOUSE", "INFLUX"},
	},
	"queues": {
		ExcludeKeys: []string{
			"RABBITMQ_PORT",
			"AMQP_PORT",
			"NATS_PORT",
			"KAFKA_PORT",
			"PULSAR_PORT",
			"ACTIVEMQ_PORT",
			"ARTEMIS_PORT",
			"SQS_PORT",
			"NSQ_PORT",
			"RSMQ_PORT",
			"BEANSTALKD_PORT",
		},
	},
}

// Load reads configuration from the provided file paths, merging them in order.
func Load(paths []string) *Config {
	cfg := &Config{Presets: make(map[string]Preset)}

	for _, path := range paths {
		localConfig, ok := loadFile(path)
		if !ok {
			continue
		}
		cfg.Strict = cfg.Strict || localConfig.Strict
		if localConfig.Version > 0 {
			cfg.Version = localConfig.Version
		}
		if len(localConfig.Scanner.IgnoreDirs) > 0 {
			cfg.Scanner.IgnoreDirs = append([]string{}, localConfig.Scanner.IgnoreDirs...)
		}
		if localConfig.Scanner.MaxDepth > 0 {
			cfg.Scanner.MaxDepth = localConfig.Scanner.MaxDepth
		}
		if localConfig.Links != nil {
			cfg.Links = append([]LinkRule{}, localConfig.Links...)
		}
		cfg.Warnings = append(cfg.Warnings, localConfig.Warnings...)
		cfg.Errors = append(cfg.Errors, localConfig.Errors...)
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
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, false
		}
		return Config{Errors: []error{fmt.Errorf("read %s: %w", path, err)}}, true
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{Errors: []error{fmt.Errorf("parse %s: %w", path, err)}}, true
	}

	if cfg.Version != 0 && cfg.Version != 2 {
		cfg.Errors = append(cfg.Errors, fmt.Errorf("unsupported config version %d in %s", cfg.Version, path))
	}
	if cfg.Presets == nil {
		cfg.Presets = make(map[string]Preset)
	}
	for name, preset := range cfg.Presets {
		if len(preset.Ignore) > 0 {
			if len(preset.IgnorePrefixes) == 0 {
				preset.IgnorePrefixes = append([]string{}, preset.Ignore...)
			} else {
				preset.IgnorePrefixes = append(preset.IgnorePrefixes, preset.Ignore...)
			}
			cfg.Warnings = append(cfg.Warnings, fmt.Sprintf("preset %q uses deprecated field ignore; use ignore_prefixes", name))
			cfg.Presets[name] = preset
		}
	}
	for i, link := range cfg.Links {
		if strings.TrimSpace(link.SourceKey) == "" {
			cfg.Errors = append(cfg.Errors, fmt.Errorf("links[%d].source_key is required", i))
		}
		if strings.TrimSpace(link.TargetRepo) == "" {
			cfg.Errors = append(cfg.Errors, fmt.Errorf("links[%d].target_repo is required", i))
		}
		if link.TargetPortKey != "" && !isValidEnvVarName(link.TargetPortKey) {
			cfg.Errors = append(cfg.Errors, fmt.Errorf("links[%d].target_port_key %q is invalid", i, link.TargetPortKey))
		}
	}

	return cfg, true
}

func mergePresets(dst, src map[string]Preset) {
	for key, value := range src {
		dst[key] = value
	}
}

func (c *Config) HasErrors() bool {
	return c != nil && len(c.Errors) > 0
}

func isValidEnvVarName(key string) bool {
	if key == "" {
		return false
	}
	for i, r := range key {
		isUpper := r >= 'A' && r <= 'Z'
		isLower := r >= 'a' && r <= 'z'
		isDigit := r >= '0' && r <= '9'
		isUnderscore := r == '_'
		if i == 0 {
			if !(isUpper || isLower || isUnderscore) {
				return false
			}
			continue
		}
		if !(isUpper || isLower || isDigit || isUnderscore) {
			return false
		}
	}
	return true
}
