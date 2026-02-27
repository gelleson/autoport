// Package env provides utilities for parsing .env files and extracting port-related keys.
package env

import (
	"bufio"
	"io"
	"strings"
)

// ExtractPortKeys scans a reader for lines matching .env format and returns keys related to ports.
func ExtractPortKeys(r io.Reader) []string {
	var keys []string
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
		if key == "PORT" || strings.HasSuffix(key, "_PORT") {
			keys = append(keys, key)
		}
	}
	return keys
}
