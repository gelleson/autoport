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
	finalIgnores := append([]string{}, opts.Ignores...)
	finalRange := port.DefaultRange

	for _, pName := range opts.Presets {
		p, ok := config.BuiltInPresets[pName]
		if !ok && a.config != nil {
			p, ok = a.config.Presets[pName]
		}
		if ok {
			finalIgnores = append(finalIgnores, p.Ignore...)
			if p.Range != "" && opts.Range == "" {
				finalRange = p.Range
			}
		} else {
			a.logger.Warn("preset not found", slog.String("preset", pName))
		}
	}

	if opts.Range != "" {
		finalRange = opts.Range
	}

	start, end, err := port.ParseRange(finalRange)
	if err != nil {
		return fmt.Errorf("range: %w", err)
	}

	seed := port.HashPath(opts.CWD)

	s := scanner.New(opts.CWD, 
		scanner.WithIgnores(finalIgnores),
		scanner.WithEnviron(a.environ),
	)
	
	portKeys, err := s.Scan(ctx)
	if err != nil {
		return fmt.Errorf("scan: %w", err)
	}

	overrides := make(map[string]string)
	for i, key := range portKeys {
		p, err := port.FindDeterministic(seed, i, start, end, a.isFree)
		if err != nil {
			return fmt.Errorf("find port for %s: %w", key, err)
		}
		overrides[key] = strconv.Itoa(p)
	}

	if len(args) == 0 {
		var keys []string
		for k := range overrides {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(a.stdout, "export %s=%s\n", k, overrides[k])
		}
		return nil
	}

	cmdName := args[0]
	cmdArgs := args[1:]

	env := append([]string{}, a.environ...)
	for k, v := range overrides {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	return a.executor.Run(ctx, cmdName, cmdArgs, env, a.stdout, os.Stderr)
}
