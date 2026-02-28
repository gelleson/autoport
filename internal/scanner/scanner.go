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

// Discovery records a discovered port key and its source.
type Discovery struct {
	Key    string
	Source string
}

// Stats captures scanner execution metrics for explain/doctor.
type Stats struct {
	FilesVisited    int
	EnvFilesParsed  int
	SkippedIgnore   int
	SkippedMaxDepth int
}

// Scanner handles discovering port keys from environment variables and files.
// It searches for keys that are exactly "PORT" or end with "_PORT".
type Scanner struct {
	ignores    []string
	cwd        string
	environ    []string
	ignoreDirs map[string]struct{}
	maxDepth   int
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

// WithIgnoreDirs sets directory names to skip when scanning.
func WithIgnoreDirs(dirs []string) Option {
	return func(s *Scanner) {
		if s.ignoreDirs == nil {
			s.ignoreDirs = make(map[string]struct{}, len(dirs))
		}
		for _, d := range dirs {
			if d == "" {
				continue
			}
			s.ignoreDirs[d] = struct{}{}
		}
	}
}

// WithMaxDepth sets the maximum relative directory depth to scan (0 = unlimited).
func WithMaxDepth(depth int) Option {
	return func(s *Scanner) {
		s.maxDepth = depth
	}
}

// New creates a new Scanner with the given working directory and options.
func New(cwd string, opts ...Option) *Scanner {
	s := &Scanner{
		cwd:        cwd,
		environ:    os.Environ(),
		ignoreDirs: map[string]struct{}{},
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

func isPortKey(key string) bool {
	up := strings.ToUpper(key)
	return up == "PORT" || strings.HasSuffix(up, "_PORT")
}

// Scan discovers port-related keys from the environment and .env files.
// It respects the provided context for cancellation.
func (s *Scanner) Scan(ctx context.Context) ([]string, error) {
	discoveries, _, err := s.ScanDetailed(ctx)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(discoveries))
	for _, d := range discoveries {
		keys = append(keys, d.Key)
	}
	return keys, nil
}

// ScanDetailed discovers keys with source metadata and scanner stats.
func (s *Scanner) ScanDetailed(ctx context.Context) ([]Discovery, Stats, error) {
	stats := Stats{}
	keySource := make(map[string]string)

	if err := s.scanEnvironment(ctx, keySource); err != nil {
		return nil, stats, err
	}

	err := s.scanEnvFiles(ctx, keySource, &stats)
	if err != nil && err != context.Canceled && err != context.DeadlineExceeded {
		return nil, stats, err
	}

	if !s.isIgnored("PORT") {
		if _, ok := keySource["PORT"]; !ok {
			keySource["PORT"] = "default"
		}
	}

	keys := make([]string, 0, len(keySource))
	for key := range keySource {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	discoveries := make([]Discovery, 0, len(keys))
	for _, key := range keys {
		discoveries = append(discoveries, Discovery{Key: key, Source: keySource[key]})
	}

	return discoveries, stats, ctx.Err()
}

func (s *Scanner) scanEnvironment(ctx context.Context, out map[string]string) error {
	for _, environmentVar := range s.environ {
		if err := ctx.Err(); err != nil {
			return err
		}

		parts := strings.SplitN(environmentVar, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		if s.isIgnored(key) || !isPortKey(key) {
			continue
		}
		if _, exists := out[key]; !exists {
			out[key] = "env"
		}
	}
	return nil
}

func (s *Scanner) scanEnvFiles(ctx context.Context, out map[string]string, stats *Stats) error {
	return filepath.WalkDir(s.cwd, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}

		if err := ctx.Err(); err != nil {
			return err
		}

		rel, err := filepath.Rel(s.cwd, path)
		if err != nil {
			rel = path
		}
		depth := pathDepth(rel)

		if d.IsDir() {
			if isHiddenDir(d.Name()) {
				return filepath.SkipDir
			}
			if _, skip := s.ignoreDirs[d.Name()]; skip {
				stats.SkippedIgnore++
				return filepath.SkipDir
			}
			if s.maxDepth > 0 && depth > s.maxDepth {
				stats.SkippedMaxDepth++
				return filepath.SkipDir
			}
			return nil
		}

		stats.FilesVisited++
		if !isEnvFile(d.Name()) {
			return nil
		}
		stats.EnvFilesParsed++

		file, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer file.Close()

		keys := env.ExtractPortKeys(file)
		source := rel
		for _, key := range keys {
			if s.isIgnored(key) || !isPortKey(key) {
				continue
			}
			if _, exists := out[key]; !exists {
				out[key] = source
			}
		}
		return nil
	})
}

func pathDepth(rel string) int {
	if rel == "." || rel == "" {
		return 0
	}
	return len(strings.Split(rel, string(filepath.Separator))) - 1
}

func isHiddenDir(name string) bool {
	return strings.HasPrefix(name, ".") && name != "."
}

func isEnvFile(name string) bool {
	return name == ".env" || strings.HasPrefix(name, ".env.")
}
