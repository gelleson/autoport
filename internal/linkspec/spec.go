package linkspec

import (
	"fmt"
	"strings"
)

// Mode describes how a target env spec should be interpreted.
type Mode string

const (
	ModeSmart    Mode = "smart"
	ModeExplicit Mode = "explicit"
)

// TargetEnvSpec is one parsed -e/--target-env input.
type TargetEnvSpec struct {
	Raw           string
	Mode          Mode
	SourceKey     string
	EnvPath       string
	TargetPortKey string
}

// ParseMany parses multiple target env specs.
func ParseMany(values []string) ([]TargetEnvSpec, error) {
	specs := make([]TargetEnvSpec, 0, len(values))
	for _, value := range values {
		spec, err := Parse(value)
		if err != nil {
			return nil, err
		}
		specs = append(specs, spec)
	}
	return specs, nil
}

// Parse parses a single target env spec.
func Parse(value string) (TargetEnvSpec, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return TargetEnvSpec{}, fmt.Errorf("target env spec cannot be empty")
	}

	if !strings.Contains(raw, "=") {
		return TargetEnvSpec{
			Raw:     raw,
			Mode:    ModeSmart,
			EnvPath: raw,
		}, nil
	}

	parts := strings.SplitN(raw, "=", 2)
	sourceKey := strings.TrimSpace(parts[0])
	right := strings.TrimSpace(parts[1])
	if sourceKey == "" {
		return TargetEnvSpec{}, fmt.Errorf("invalid target env spec %q: missing source key", raw)
	}
	if !isValidEnvVarName(sourceKey) {
		return TargetEnvSpec{}, fmt.Errorf("invalid target env spec %q: invalid source key %q", raw, sourceKey)
	}
	if right == "" {
		return TargetEnvSpec{}, fmt.Errorf("invalid target env spec %q: missing env path", raw)
	}
	if strings.HasSuffix(right, ":") {
		return TargetEnvSpec{}, fmt.Errorf("invalid target env spec %q: missing target port key after ':'", raw)
	}

	pathPart := right
	targetPortKey := ""
	if idx := strings.LastIndex(right, ":"); idx > 0 && idx < len(right)-1 {
		candidate := strings.TrimSpace(right[idx+1:])
		if isValidEnvVarName(candidate) {
			pathPart = strings.TrimSpace(right[:idx])
			targetPortKey = candidate
		}
	}
	if pathPart == "" {
		return TargetEnvSpec{}, fmt.Errorf("invalid target env spec %q: missing env path", raw)
	}

	return TargetEnvSpec{
		Raw:           raw,
		Mode:          ModeExplicit,
		SourceKey:     sourceKey,
		EnvPath:       pathPart,
		TargetPortKey: targetPortKey,
	}, nil
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
