// Package env provides utilities for parsing .env files and extracting values.
package env

import (
	"bufio"
	"io"
	"sort"
	"strings"
)

// Parse reads dotenv-like lines and returns key-value assignments.
func Parse(r io.Reader) map[string]string {
	values := map[string]string{}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		if key == "" {
			continue
		}
		value := strings.TrimSpace(parts[1])
		values[key] = stripQuotes(value)
	}
	return values
}

// ExtractPortKeys scans a reader for lines matching .env format and returns keys related to ports.
func ExtractPortKeys(r io.Reader) []string {
	parsed := Parse(r)
	keys := make([]string, 0, len(parsed))
	for key := range parsed {
		if isPortKey(key) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func isPortKey(key string) bool {
	up := strings.ToUpper(key)
	return up == "PORT" || strings.HasSuffix(up, "_PORT")
}

func stripQuotes(value string) string {
	if len(value) < 2 {
		return value
	}
	if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
		return value[1 : len(value)-1]
	}
	return value
}
