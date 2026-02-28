package app

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"sort"
	"strconv"

	"github.com/gelleson/autoport/internal/config"
	"github.com/gelleson/autoport/internal/scanner"
	"github.com/gelleson/autoport/pkg/port"
)

// Options represents the input options for the application.
type Options struct {
	Ignores []string
	Presets []string
	PortEnv []string
	Range   string
	CWD     string
}

// Executor defines how to execute a command.
type Executor interface {
	Run(ctx context.Context, name string, args []string, env []string, stdout, stderr io.Writer) error
}

// DefaultExecutor is the standard implementation that runs OS commands.
type DefaultExecutor struct{}

// Run executes the command using the standard library's os/exec.
func (d DefaultExecutor) Run(ctx context.Context, name string, args []string, env []string, stdout, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	return cmd.Run()
}

// App encapsulates the main application logic and its dependencies.
type App struct {
	config   *config.Config
	executor Executor
	stdout   io.Writer
	logger   *slog.Logger
	environ  []string
	isFree   port.IsFreeFunc
}

// AppOption defines a functional option for configuring the App.
type AppOption func(*App)

// WithConfig sets a custom configuration.
func WithConfig(cfg *config.Config) AppOption {
	return func(a *App) { a.config = cfg }
}

// WithExecutor sets a custom command executor.
func WithExecutor(e Executor) AppOption {
	return func(a *App) { a.executor = e }
}

// WithStdout sets the standard output writer.
func WithStdout(w io.Writer) AppOption {
	return func(a *App) { a.stdout = w }
}

// WithLogger sets the logger.
func WithLogger(l *slog.Logger) AppOption {
	return func(a *App) { a.logger = l }
}

// WithEnviron sets the base environment variables.
func WithEnviron(env []string) AppOption {
	return func(a *App) { a.environ = env }
}

// WithIsFree sets the port availability checker.
func WithIsFree(fn port.IsFreeFunc) AppOption {
	return func(a *App) { a.isFree = fn }
}

// New creates a new App with default dependencies and optional overrides.
func New(opts ...AppOption) *App {
	a := &App{
		config:   config.LoadDefault(),
		executor: DefaultExecutor{},
		stdout:   os.Stdout,
		logger:   slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})),
		environ:  os.Environ(),
		isFree:   port.DefaultIsFree,
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// Run executes the main application workflow.
func (a *App) Run(ctx context.Context, opts Options, args []string) error {
	finalIgnores, finalRange := a.resolvePresetOverrides(opts)
	r, err := port.ParseRange(finalRange)
	if err != nil {
		return fmt.Errorf("range: %w", err)
	}

	portKeys, err := a.scanPortKeys(ctx, opts.CWD, finalIgnores)
	if err != nil {
		return fmt.Errorf("scan: %w", err)
	}
	portKeys, err = mergePortKeys(portKeys, opts.PortEnv)
	if err != nil {
		return fmt.Errorf("manual port env: %w", err)
	}

	overrides, err := a.assignPorts(opts.CWD, r, portKeys)
	if err != nil {
		return err
	}

	if len(args) == 0 {
		a.printExports(overrides)
		return nil
	}

	env := a.buildExecEnv(overrides)
	cmdName := args[0]
	cmdArgs := args[1:]
	return a.executor.Run(ctx, cmdName, cmdArgs, env, a.stdout, os.Stderr)
}

func (a *App) resolvePresetOverrides(opts Options) (ignores []string, rangeSpec string) {
	ignores = append([]string{}, opts.Ignores...)
	rangeSpec = port.DefaultRange

	for _, presetName := range opts.Presets {
		preset, ok := a.lookupPreset(presetName)
		if !ok {
			a.logger.Warn("preset not found", slog.String("preset", presetName))
			continue
		}

		ignores = append(ignores, preset.Ignore...)
		if preset.Range != "" && opts.Range == "" {
			rangeSpec = preset.Range
		}
	}

	if opts.Range != "" {
		rangeSpec = opts.Range
	}
	return ignores, rangeSpec
}

func (a *App) lookupPreset(name string) (config.Preset, bool) {
	if preset, ok := config.BuiltInPresets[name]; ok {
		return preset, true
	}
	if a.config == nil {
		return config.Preset{}, false
	}
	preset, ok := a.config.Presets[name]
	return preset, ok
}

func (a *App) scanPortKeys(ctx context.Context, cwd string, ignores []string) ([]string, error) {
	s := scanner.New(cwd,
		scanner.WithIgnores(ignores),
		scanner.WithEnviron(a.environ),
	)
	return s.Scan(ctx)
}

func (a *App) assignPorts(cwd string, r port.Range, portKeys []string) (map[string]string, error) {
	allocator := port.Allocator{
		Seed:   port.HashPath(cwd),
		Range:  r,
		IsFree: a.isFree,
	}
	overrides := make(map[string]string)
	for i, key := range portKeys {
		p, err := allocator.PortFor(i)
		if err != nil {
			return nil, fmt.Errorf("find port for %s: %w", key, err)
		}
		overrides[key] = strconv.Itoa(p)
	}
	return overrides, nil
}

func (a *App) printExports(overrides map[string]string) {
	keys := sortedKeys(overrides)
	for _, key := range keys {
		fmt.Fprintf(a.stdout, "export %s=%s\n", key, overrides[key])
	}
}

func (a *App) buildExecEnv(overrides map[string]string) []string {
	env := append([]string{}, a.environ...)
	for key, value := range overrides {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}
	return env
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func mergePortKeys(scanned, manual []string) ([]string, error) {
	keySet := make(map[string]struct{}, len(scanned)+len(manual))
	for _, key := range scanned {
		keySet[key] = struct{}{}
	}

	for _, key := range manual {
		if !isValidEnvVarName(key) {
			return nil, fmt.Errorf("invalid env key %q", key)
		}
		keySet[key] = struct{}{}
	}

	out := make([]string, 0, len(keySet))
	for key := range keySet {
		out = append(out, key)
	}
	sort.Strings(out)
	return out, nil
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
