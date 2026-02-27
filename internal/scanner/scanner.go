// Package scanner provides functionality to discover port-related environment
// variables from the current environment and local .env files.
package scanner

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gelleson/autoport/internal/env"
)

// Scanner handles discovering port keys from environment variables and files.
// It searches for keys that are exactly "PORT" or end with "_PORT".
type Scanner struct {
	ignores []string
	cwd     string
	environ []string
}

// Option defines a functional option for the Scanner.
type Option func(*Scanner)

// WithEnviron sets the environment variables for the scanner.
func WithEnviron(environ []string) Option {
	return func(s *Scanner) {
		s.environ = environ
	}
}

// WithIgnores sets the prefixes to ignore.
func WithIgnores(ignores []string) Option {
	return func(s *Scanner) {
		s.ignores = ignores
	}
}

// New creates a new Scanner with the given working directory and options.
func New(cwd string, opts ...Option) *Scanner {
	s := &Scanner{
		cwd:     cwd,
		environ: os.Environ(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// isIgnored checks if a given key starts with any of the ignore prefixes.
func (s *Scanner) isIgnored(key string) bool {
	for _, ignore := range s.ignores {
		if strings.HasPrefix(key, ignore) {
			return true
		}
	}
	return false
}

// Scan discovers port-related keys from the environment and .env files.
// It respects the provided context for cancellation.
func (s *Scanner) Scan(ctx context.Context) ([]string, error) {
	portKeyMap := make(map[string]struct{})

	// Scan environment variables
	for _, environmentVar := range s.environ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		parts := strings.SplitN(environmentVar, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]
		if s.isIgnored(key) {
			continue
		}
		if key == "PORT" || strings.HasSuffix(key, "_PORT") {
			portKeyMap[key] = struct{}{}
		}
	}

	// Scan filesystem for .env files using more efficient WalkDir
	err := filepath.WalkDir(s.cwd, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip files with errors
		}
		
		if err := ctx.Err(); err != nil {
			return err
		}

		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && d.Name() != "." {
				return filepath.SkipDir
			}
			return nil
		}

		name := d.Name()
		if name == ".env" || strings.HasPrefix(name, ".env.") {
			file, err := os.Open(path)
			if err != nil {
				return nil
			}
			defer file.Close()

			keys := env.ExtractPortKeys(file)
			for _, k := range keys {
				if s.isIgnored(k) {
					continue
				}
				portKeyMap[k] = struct{}{}
			}
		}
		return nil
	})

	if err != nil && err != context.Canceled && err != context.DeadlineExceeded {
		return nil, err
	}

	if !s.isIgnored("PORT") {
		portKeyMap["PORT"] = struct{}{}
	}

	var portKeys []string
	for k := range portKeyMap {
		portKeys = append(portKeys, k)
	}
	sort.Strings(portKeys)
	return portKeys, ctx.Err()
}
