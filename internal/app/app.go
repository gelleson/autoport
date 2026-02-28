package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gelleson/autoport/internal/config"
	"github.com/gelleson/autoport/internal/lockfile"
	"github.com/gelleson/autoport/internal/scanner"
	"github.com/gelleson/autoport/pkg/port"
)

// Options represents the input options for the application.
type Options struct {
	Mode      string
	Ignores   []string
	Includes  []string
	Excludes  []string
	Presets   []string
	PortEnv   []string
	Range     string
	Format    string
	Quiet     bool
	DryRun    bool
	CWD       string
	Namespace string
	Seed      *uint32
	UseLock   bool
}

// ExitError allows command modes to signal specific process exit codes.
type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("exit code %d", e.Code)
	}
	return e.Err.Error()
}

func (e *ExitError) ExitCode() int {
	return e.Code
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
	stderr   io.Writer
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

// WithStderr sets the standard error writer.
func WithStderr(w io.Writer) AppOption {
	return func(a *App) { a.stderr = w }
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
		stderr:   os.Stderr,
		logger:   slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})),
		environ:  os.Environ(),
		isFree:   port.DefaultIsFree,
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

type resolvedOptions struct {
	Range      string
	Ignores    []string
	Includes   []string
	Excludes   []string
	IgnoreDirs []string
	MaxDepth   int
	Warnings   []string
	Strict     bool
}

type keyDecision struct {
	Key      string
	Source   string
	Included bool
	Reason   string
}

type assignedPort struct {
	Key       string
	Value     string
	Preferred int
	Assigned  int
	Probes    int
	FromLock  bool
}

// Run executes the main application workflow.
func (a *App) Run(ctx context.Context, opts Options, args []string) error {
	if opts.Mode == "" {
		opts.Mode = "run"
	}
	if a.config == nil {
		a.config = &config.Config{Presets: map[string]config.Preset{}}
	}
	if a.config.HasErrors() {
		return joinErrors("config", a.config.Errors)
	}

	res, err := a.resolveOptions(opts)
	if err != nil {
		return err
	}

	if opts.Mode == "doctor" {
		return a.runDoctor(ctx, opts, res)
	}

	r, err := port.ParseRange(res.Range)
	if err != nil {
		return fmt.Errorf("range: %w", err)
	}

	seed := a.computeSeed(opts)
	discoveries, scanStats, scanErr := a.scanDiscoveries(ctx, opts.CWD, res)
	if scanErr != nil {
		return fmt.Errorf("scan: %w", scanErr)
	}

	decisions, finalKeys, err := a.applySelection(discoveries, opts.PortEnv, res)
	if err != nil {
		return err
	}

	assignments, overrides, assignWarnings, err := a.assignWithOptionalLock(opts, r, seed, finalKeys)
	if err != nil {
		return err
	}
	warnings := append([]string{}, res.Warnings...)
	warnings = append(warnings, assignWarnings...)

	switch opts.Mode {
	case "explain":
		return a.renderExplain(opts, args, res, r, seed, decisions, assignments, warnings, scanStats)
	case "lock":
		return a.writeLockfile(opts, res.Range, overrides)
	case "run":
		return a.runOrExport(ctx, opts, args, res.Range, overrides, warnings)
	default:
		return fmt.Errorf("unknown mode %q", opts.Mode)
	}
}

func (a *App) resolveOptions(opts Options) (resolvedOptions, error) {
	res := resolvedOptions{
		Range:    port.DefaultRange,
		Ignores:  append([]string{}, opts.Ignores...),
		Includes: append([]string{}, opts.Includes...),
		Excludes: append([]string{}, opts.Excludes...),
		Strict:   a.config.Strict,
		Warnings: append([]string{}, a.config.Warnings...),
	}

	if opts.Range != "" {
		res.Range = opts.Range
	}
	if a.config.Scanner.MaxDepth > 0 {
		res.MaxDepth = a.config.Scanner.MaxDepth
	}
	if len(a.config.Scanner.IgnoreDirs) > 0 {
		res.IgnoreDirs = append([]string{}, a.config.Scanner.IgnoreDirs...)
	}

	for _, presetName := range opts.Presets {
		preset, ok := a.lookupPreset(presetName)
		if !ok {
			if res.Strict {
				return resolvedOptions{}, fmt.Errorf("unknown preset %q (strict mode)", presetName)
			}
			w := fmt.Sprintf("preset not found: %s", presetName)
			res.Warnings = append(res.Warnings, w)
			a.logger.Warn("preset not found", slog.String("preset", presetName))
			continue
		}
		res.Ignores = append(res.Ignores, preset.IgnorePrefixes...)
		res.Includes = append(res.Includes, preset.IncludeKeys...)
		res.Excludes = append(res.Excludes, preset.ExcludeKeys...)
		if preset.Range != "" && opts.Range == "" {
			res.Range = preset.Range
		}
	}

	if opts.Range != "" {
		res.Range = opts.Range
	}
	res.Ignores = dedupeSorted(res.Ignores)
	res.Includes = dedupeSorted(res.Includes)
	res.Excludes = dedupeSorted(res.Excludes)
	return res, nil
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

func (a *App) computeSeed(opts Options) uint32 {
	if opts.Seed != nil {
		return *opts.Seed
	}
	return port.SeedFor(opts.CWD, opts.Namespace)
}

func (a *App) scanDiscoveries(ctx context.Context, cwd string, res resolvedOptions) ([]scanner.Discovery, scanner.Stats, error) {
	s := scanner.New(cwd,
		scanner.WithIgnores(res.Ignores),
		scanner.WithEnviron(a.environ),
		scanner.WithIgnoreDirs(res.IgnoreDirs),
		scanner.WithMaxDepth(res.MaxDepth),
	)
	return s.ScanDetailed(ctx)
}

func (a *App) applySelection(discoveries []scanner.Discovery, manual []string, res resolvedOptions) ([]keyDecision, []string, error) {
	includeSet := makeSet(res.Includes)
	excludeSet := makeSet(res.Excludes)

	keySet := make(map[string]struct{})
	decisions := make([]keyDecision, 0, len(discoveries)+len(manual))

	for _, d := range discoveries {
		included := true
		reason := "discovered"
		if _, excluded := excludeSet[d.Key]; excluded {
			included = false
			reason = "excluded by exact key"
		}
		if len(includeSet) > 0 {
			if _, ok := includeSet[d.Key]; !ok {
				included = false
				reason = "not in include_keys"
			} else if included {
				reason = "included by include_keys"
			}
		}

		decisions = append(decisions, keyDecision{
			Key:      d.Key,
			Source:   d.Source,
			Included: included,
			Reason:   reason,
		})
		if included {
			keySet[d.Key] = struct{}{}
		}
	}

	for _, key := range manual {
		if !isValidEnvVarName(key) {
			return nil, nil, fmt.Errorf("invalid env key %q", key)
		}
		keySet[key] = struct{}{}
		decisions = append(decisions, keyDecision{
			Key:      key,
			Source:   "manual",
			Included: true,
			Reason:   "included by -k",
		})
	}

	finalKeys := make([]string, 0, len(keySet))
	for key := range keySet {
		finalKeys = append(finalKeys, key)
	}
	sort.Strings(finalKeys)
	sort.Slice(decisions, func(i, j int) bool {
		if decisions[i].Key == decisions[j].Key {
			return decisions[i].Source < decisions[j].Source
		}
		return decisions[i].Key < decisions[j].Key
	})
	return decisions, finalKeys, nil
}

func (a *App) assignWithOptionalLock(opts Options, r port.Range, seed uint32, keys []string) ([]assignedPort, map[string]string, []string, error) {
	allocator := port.Allocator{Seed: seed, Range: r, IsFree: a.isFree}
	warnings := []string{}

	locked := map[string]string{}
	if opts.UseLock {
		path := lockfile.PathFor(opts.CWD)
		lf, err := lockfile.Read(path)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("read lockfile: %w", err)
		}
		if lf.CWDFingerprint != lockfile.Fingerprint(opts.CWD) {
			return nil, nil, nil, fmt.Errorf("lockfile cwd fingerprint mismatch")
		}
		if lf.Range != opts.Range && opts.Range != "" {
			warnings = append(warnings, fmt.Sprintf("lockfile range %s differs from CLI range %s", lf.Range, opts.Range))
		}
		locked = lockfile.ToMap(lf.Assignments)
	}

	results := make([]assignedPort, 0, len(keys))
	overrides := make(map[string]string, len(keys))
	for i, key := range keys {
		if val, ok := locked[key]; ok {
			p, err := strconv.Atoi(val)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("lockfile value for %s is not numeric", key)
			}
			results = append(results, assignedPort{Key: key, Value: val, Preferred: p, Assigned: p, Probes: 0, FromLock: true})
			overrides[key] = val
			continue
		}
		assigned, preferred, probes, err := allocator.PortForWithStats(i)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("find port for %s: %w", key, err)
		}
		v := strconv.Itoa(assigned)
		results = append(results, assignedPort{Key: key, Value: v, Preferred: preferred, Assigned: assigned, Probes: probes})
		overrides[key] = v
	}
	return results, overrides, warnings, nil
}

func (a *App) writeLockfile(opts Options, rangeSpec string, overrides map[string]string) error {
	path := lockfile.PathFor(opts.CWD)
	if err := lockfile.Write(path, opts.CWD, rangeSpec, overrides); err != nil {
		return err
	}
	fmt.Fprintf(a.stdout, "wrote %s with %d assignments\n", filepath.Base(path), len(overrides))
	return nil
}

func (a *App) runOrExport(ctx context.Context, opts Options, args []string, rangeSpec string, overrides map[string]string, warnings []string) error {
	if len(args) == 0 {
		mode := "export"
		if opts.DryRun {
			mode = "preview"
		}
		a.printPrimaryOutput(opts.Format, mode, opts.CWD, rangeSpec, nil, overrides, warnings)
		return nil
	}

	if opts.DryRun {
		if opts.Format == "json" {
			a.printJSONOutput(a.stdout, "preview", opts.CWD, rangeSpec, args, overrides, warnings)
		} else {
			a.printOverrideSummary(args[0], args[1:], overrides)
		}
		return nil
	}

	env := a.buildExecEnv(overrides)
	cmdName := args[0]
	cmdArgs := args[1:]
	if !opts.Quiet {
		if opts.Format == "json" {
			a.printJSONOutput(a.stderr, "execute", opts.CWD, rangeSpec, args, overrides, warnings)
		} else {
			a.printOverrideSummary(cmdName, cmdArgs, overrides)
		}
	}
	return a.executor.Run(ctx, cmdName, cmdArgs, env, a.stdout, a.stderr)
}

type explainRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type explainInputs struct {
	Presets   []string `json:"presets"`
	Ignores   []string `json:"ignores"`
	Includes  []string `json:"includes"`
	Excludes  []string `json:"excludes"`
	Namespace string   `json:"namespace,omitempty"`
}

type explainKey struct {
	Key      string `json:"key"`
	Source   string `json:"source"`
	Included bool   `json:"included"`
	Reason   string `json:"reason"`
}

type explainAssignment struct {
	Key       string `json:"key"`
	Preferred int    `json:"preferred"`
	Assigned  int    `json:"assigned"`
	Probes    int    `json:"probes"`
}

type explainPayload struct {
	Mode        string              `json:"mode"`
	CWD         string              `json:"cwd"`
	Seed        uint32              `json:"seed"`
	Range       explainRange        `json:"range"`
	Inputs      explainInputs       `json:"inputs"`
	Keys        []explainKey        `json:"keys"`
	Assignments []explainAssignment `json:"assignments"`
	Warnings    []string            `json:"warnings,omitempty"`
	Stats       scanner.Stats       `json:"stats"`
}

func (a *App) renderExplain(opts Options, args []string, res resolvedOptions, r port.Range, seed uint32, decisions []keyDecision, assignments []assignedPort, warnings []string, stats scanner.Stats) error {
	if opts.Format == "json" {
		payload := explainPayload{
			Mode:  "explain",
			CWD:   opts.CWD,
			Seed:  seed,
			Range: explainRange{Start: r.Start, End: r.End},
			Inputs: explainInputs{
				Presets:   append([]string{}, opts.Presets...),
				Ignores:   append([]string{}, res.Ignores...),
				Includes:  append([]string{}, res.Includes...),
				Excludes:  append([]string{}, res.Excludes...),
				Namespace: opts.Namespace,
			},
			Warnings: append([]string{}, warnings...),
			Stats:    stats,
		}
		for _, d := range decisions {
			payload.Keys = append(payload.Keys, explainKey{Key: d.Key, Source: d.Source, Included: d.Included, Reason: d.Reason})
		}
		for _, as := range assignments {
			payload.Assignments = append(payload.Assignments, explainAssignment{Key: as.Key, Preferred: as.Preferred, Assigned: as.Assigned, Probes: as.Probes})
		}
		enc := json.NewEncoder(a.stdout)
		return enc.Encode(payload)
	}

	fmt.Fprintf(a.stdout, "autoport explain\n")
	fmt.Fprintf(a.stdout, "cwd: %s\n", opts.CWD)
	fmt.Fprintf(a.stdout, "seed: %d\n", seed)
	fmt.Fprintf(a.stdout, "range: %d-%d\n", r.Start, r.End)
	fmt.Fprintf(a.stdout, "presets: %s\n", strings.Join(opts.Presets, ","))
	fmt.Fprintf(a.stdout, "ignores: %s\n", strings.Join(res.Ignores, ","))
	fmt.Fprintf(a.stdout, "includes: %s\n", strings.Join(res.Includes, ","))
	fmt.Fprintf(a.stdout, "excludes: %s\n", strings.Join(res.Excludes, ","))
	fmt.Fprintf(a.stdout, "\nkeys:\n")
	for _, d := range decisions {
		mark := "x"
		if d.Included {
			mark = "✓"
		}
		fmt.Fprintf(a.stdout, "  [%s] %s (%s) - %s\n", mark, d.Key, d.Source, d.Reason)
	}
	fmt.Fprintf(a.stdout, "\nassignments:\n")
	for _, as := range assignments {
		suffix := ""
		if as.FromLock {
			suffix = " (lock)"
		}
		fmt.Fprintf(a.stdout, "  %s: preferred=%d assigned=%d probes=%d%s\n", as.Key, as.Preferred, as.Assigned, as.Probes, suffix)
	}
	fmt.Fprintf(a.stdout, "\nscan stats: files=%d env_files=%d skipped_ignore_dirs=%d skipped_max_depth=%d\n", stats.FilesVisited, stats.EnvFilesParsed, stats.SkippedIgnore, stats.SkippedMaxDepth)
	if len(warnings) > 0 {
		fmt.Fprintf(a.stdout, "\nwarnings:\n")
		for _, w := range warnings {
			fmt.Fprintf(a.stdout, "  - %s\n", w)
		}
	}
	return nil
}

type doctorCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type doctorPayload struct {
	Mode   string        `json:"mode"`
	Checks []doctorCheck `json:"checks"`
}

func (a *App) runDoctor(ctx context.Context, opts Options, res resolvedOptions) error {
	checks := []doctorCheck{}
	fatal := false
	warn := false

	if len(a.config.Errors) > 0 {
		checks = append(checks, doctorCheck{Name: "config", Status: "fatal", Message: joinErrors("config", a.config.Errors).Error()})
		fatal = true
	} else if len(a.config.Warnings) > 0 {
		checks = append(checks, doctorCheck{Name: "config", Status: "warn", Message: strings.Join(a.config.Warnings, "; ")})
		warn = true
	} else {
		checks = append(checks, doctorCheck{Name: "config", Status: "ok", Message: "configuration parsed successfully"})
	}

	r, err := port.ParseRange(res.Range)
	if err != nil {
		checks = append(checks, doctorCheck{Name: "range", Status: "fatal", Message: err.Error()})
		fatal = true
	} else {
		status := "ok"
		msg := fmt.Sprintf("range %d-%d (size=%d)", r.Start, r.End, r.Size())
		if r.Size() < 10 {
			status = "warn"
			msg = msg + "; very small range may cause collisions"
			warn = true
		}
		checks = append(checks, doctorCheck{Name: "range", Status: status, Message: msg})
	}

	start := time.Now()
	discoveries, stats, scanErr := a.scanDiscoveries(ctx, opts.CWD, res)
	dur := time.Since(start)
	if scanErr != nil {
		checks = append(checks, doctorCheck{Name: "scan", Status: "fatal", Message: scanErr.Error()})
		fatal = true
	} else {
		status := "ok"
		msg := fmt.Sprintf("found %d keys in %s; files=%d env_files=%d", len(discoveries), dur.Truncate(time.Millisecond), stats.FilesVisited, stats.EnvFilesParsed)
		if stats.SkippedMaxDepth > 0 {
			status = "warn"
			msg = msg + fmt.Sprintf("; max_depth skipped %d directories", stats.SkippedMaxDepth)
			warn = true
		}
		checks = append(checks, doctorCheck{Name: "scan", Status: status, Message: msg})
	}

	if _, err := port.ParseRange(res.Range); err == nil {
		freeCount := 0
		sample := []int{r.Start, (r.Start + r.End) / 2, r.End}
		for _, p := range sample {
			if a.isFree(p) {
				freeCount++
			}
		}
		if freeCount == 0 {
			checks = append(checks, doctorCheck{Name: "port_availability", Status: "fatal", Message: "no sampled ports are available"})
			fatal = true
		} else if freeCount < len(sample) {
			checks = append(checks, doctorCheck{Name: "port_availability", Status: "warn", Message: fmt.Sprintf("%d/%d sampled ports are available", freeCount, len(sample))})
			warn = true
		} else {
			checks = append(checks, doctorCheck{Name: "port_availability", Status: "ok", Message: "sampled ports are available"})
		}
	}

	lockPath := lockfile.PathFor(opts.CWD)
	if _, statErr := os.Stat(lockPath); statErr == nil {
		lf, err := lockfile.Read(lockPath)
		if err != nil {
			checks = append(checks, doctorCheck{Name: "lockfile", Status: "warn", Message: err.Error()})
			warn = true
		} else {
			status := "ok"
			msg := fmt.Sprintf("lockfile version=%d assignments=%d", lf.Version, len(lf.Assignments))
			if lf.CWDFingerprint != lockfile.Fingerprint(opts.CWD) {
				status = "warn"
				msg = "lockfile cwd fingerprint mismatch"
				warn = true
			}
			checks = append(checks, doctorCheck{Name: "lockfile", Status: status, Message: msg})
		}
	} else if errors.Is(statErr, os.ErrNotExist) {
		checks = append(checks, doctorCheck{Name: "lockfile", Status: "ok", Message: "no lockfile present"})
	}

	if opts.Format == "json" {
		payload := doctorPayload{Mode: "doctor", Checks: checks}
		enc := json.NewEncoder(a.stdout)
		if err := enc.Encode(payload); err != nil {
			return err
		}
	} else {
		fmt.Fprintln(a.stdout, "autoport doctor")
		for _, c := range checks {
			fmt.Fprintf(a.stdout, "- [%s] %s: %s\n", c.Status, c.Name, c.Message)
		}
	}

	if fatal {
		return &ExitError{Code: 2, Err: errors.New("doctor found fatal issues")}
	}
	if warn {
		return &ExitError{Code: 1, Err: errors.New("doctor found warnings")}
	}
	return nil
}

func (a *App) printExports(overrides map[string]string) {
	keys := sortedKeys(overrides)
	for _, key := range keys {
		fmt.Fprintf(a.stdout, "export %s=%s\n", key, overrides[key])
	}
}

func (a *App) printDotenv(overrides map[string]string) {
	keys := sortedKeys(overrides)
	for _, key := range keys {
		fmt.Fprintf(a.stdout, "%s=%s\n", key, overrides[key])
	}
}

func (a *App) printYAML(overrides map[string]string) {
	keys := sortedKeys(overrides)
	for _, key := range keys {
		fmt.Fprintf(a.stdout, "%s: \"%s\"\n", key, overrides[key])
	}
}

type outputBinding struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type outputPayload struct {
	Mode      string          `json:"mode"`
	CWD       string          `json:"cwd"`
	Range     string          `json:"range"`
	Command   []string        `json:"command,omitempty"`
	Overrides []outputBinding `json:"overrides"`
	Warnings  []string        `json:"warnings,omitempty"`
}

func (a *App) printPrimaryOutput(format, mode, cwd, rangeSpec string, command []string, overrides map[string]string, warnings []string) {
	switch format {
	case "json":
		a.printJSONOutput(a.stdout, mode, cwd, rangeSpec, command, overrides, warnings)
	case "dotenv":
		a.printDotenv(overrides)
	case "yaml":
		a.printYAML(overrides)
	default:
		a.printExports(overrides)
	}
}

func (a *App) printJSONOutput(w io.Writer, mode, cwd, rangeSpec string, command []string, overrides map[string]string, warnings []string) {
	bindings := make([]outputBinding, 0, len(overrides))
	keys := sortedKeys(overrides)
	for _, key := range keys {
		bindings = append(bindings, outputBinding{
			Key:   key,
			Value: overrides[key],
		})
	}

	payload := outputPayload{
		Mode:      mode,
		CWD:       cwd,
		Range:     rangeSpec,
		Overrides: bindings,
		Warnings:  append([]string{}, warnings...),
	}
	if len(command) > 0 {
		payload.Command = append([]string{}, command...)
	}

	enc := json.NewEncoder(w)
	if err := enc.Encode(payload); err != nil {
		a.logger.Error("failed to encode JSON output", slog.String("error", err.Error()))
	}
}

func (a *App) buildExecEnv(overrides map[string]string) []string {
	env := append([]string{}, a.environ...)
	for key, value := range overrides {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}
	return env
}

func (a *App) printOverrideSummary(cmdName string, cmdArgs []string, overrides map[string]string) {
	keys := sortedKeys(overrides)

	keyWidth := len("ENV")
	valueWidth := len("PORT")
	for _, key := range keys {
		if len(key) > keyWidth {
			keyWidth = len(key)
		}
		if len(overrides[key]) > valueWidth {
			valueWidth = len(overrides[key])
		}
	}

	command := cmdName
	if len(cmdArgs) > 0 {
		command = fmt.Sprintf("%s %s", cmdName, strings.Join(cmdArgs, " "))
	}

	border := fmt.Sprintf("+-%s-+-%s-+\n", strings.Repeat("-", keyWidth), strings.Repeat("-", valueWidth))
	fmt.Fprintf(a.stderr, "\nautoport overrides (%d) -> %s\n", len(keys), command)
	fmt.Fprint(a.stderr, border)
	fmt.Fprintf(a.stderr, "| %-*s | %-*s |\n", keyWidth, "ENV", valueWidth, "PORT")
	fmt.Fprint(a.stderr, border)
	for _, key := range keys {
		fmt.Fprintf(a.stderr, "| %-*s | %-*s |\n", keyWidth, key, valueWidth, overrides[key])
	}
	fmt.Fprint(a.stderr, border)
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
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

func makeSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, v := range values {
		set[v] = struct{}{}
	}
	return set
}

func dedupeSorted(values []string) []string {
	set := make(map[string]struct{}, len(values))
	for _, v := range values {
		if v == "" {
			continue
		}
		set[v] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for v := range set {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func joinErrors(prefix string, errs []error) error {
	parts := make([]string, 0, len(errs))
	for _, err := range errs {
		parts = append(parts, err.Error())
	}
	return fmt.Errorf("%s: %s", prefix, strings.Join(parts, "; "))
}
