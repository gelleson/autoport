/*
Package main provides the CLI entry point for autoport.

autoport is a tool that helps manage port collisions by deterministically
assigning free ports based on the project's directory path.
*/
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/gelleson/autoport/internal/app"
)

var (
	version   = "dev"
	buildTime = "unknown"
)

// ignoreFlags is a custom flag type to collect multiple ignore prefixes.
type ignoreFlags []string

func (i *ignoreFlags) String() string {
	return strings.Join(*i, ",")
}

func (i *ignoreFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

// presetFlags is a custom flag type to collect multiple preset names.
type presetFlags []string

func (p *presetFlags) String() string {
	return strings.Join(*p, ",")
}

func (p *presetFlags) Set(value string) error {
	*p = append(*p, value)
	return nil
}

// portEnvFlags is a custom flag type to collect manual port env keys.
type portEnvFlags []string

func (p *portEnvFlags) String() string {
	return strings.Join(*p, ",")
}

func (p *portEnvFlags) Set(value string) error {
	*p = append(*p, value)
	return nil
}

func main() {
	// Handle termination signals gracefully.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		if exitErr, ok := err.(interface{ ExitCode() int }); ok {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// run parses CLI flags and executes the application logic.
func run(ctx context.Context) error {
	opts, cmdArgs, err := parseCLIArgs(os.Args[1:])
	if err != nil {
		var helpErr *helpRequestedError
		if errors.As(err, &helpErr) {
			printHelp(os.Stdout, helpErr.Mode)
			return nil
		}
		return err
	}
	if opts.Mode == "version" || isVersionCommand(cmdArgs) {
		fmt.Fprintln(os.Stdout, versionString())
		return nil
	}

	application := app.New()
	return application.Run(ctx, opts, cmdArgs)
}

func isVersionCommand(args []string) bool {
	return len(args) == 1 && args[0] == "version"
}

func versionString() string {
	return fmt.Sprintf("%s (built %s)", version, buildTime)
}

func parseCLIArgs(args []string) (app.Options, []string, error) {
	var ignores ignoreFlags
	var presets presetFlags
	var portEnv portEnvFlags
	var includes portEnvFlags
	var excludes portEnvFlags
	var format string
	var quiet bool
	var dryRun bool
	var namespace string
	var seed string
	var branch string
	var seedBranch bool
	var useLock bool
	var targetEnvs portEnvFlags

	targetMode := "run"
	if len(args) > 0 {
		switch args[0] {
		case "version", "explain", "doctor", "lock":
			targetMode = args[0]
			args = args[1:]
		}
	}

	fs := flag.NewFlagSet("autoport", flag.ContinueOnError)
	fs.SetOutput(ioDiscard{})
	rangeFlag := fs.String("r", "", "Port range to use (e.g., 3000-4000). Default is 10000-20000.")
	fs.StringVar(&format, "f", defaultFormatForMode(targetMode), "Output format")
	fs.StringVar(&format, "format", defaultFormatForMode(targetMode), "Output format")
	fs.BoolVar(&quiet, "q", false, "Suppress command-mode override summary output")
	fs.BoolVar(&quiet, "quiet", false, "Suppress command-mode override summary output")
	fs.BoolVar(&dryRun, "n", false, "Preview mode: print planned overrides and do not execute command")
	fs.BoolVar(&dryRun, "dry-run", false, "Preview mode: print planned overrides and do not execute command")
	fs.StringVar(&namespace, "namespace", "", "Namespace for deterministic seed")
	fs.StringVar(&seed, "seed", "", "Explicit deterministic seed (uint32)")
	fs.StringVar(&branch, "branch", "", "Explicit branch name for branch-aware seed/link checks")
	fs.BoolVar(&seedBranch, "seed-branch", false, "Include git branch name in deterministic seed material")
	fs.BoolVar(&useLock, "use-lock", false, "Use .autoport.lock.json assignments")
	fs.Var(&targetEnvs, "e", "Target env link spec: <path> or <SOURCE_KEY>=<path>[:<TARGET_PORT_KEY>] (repeatable)")
	fs.Var(&targetEnvs, "target-env", "Target env link spec: <path> or <SOURCE_KEY>=<path>[:<TARGET_PORT_KEY>] (repeatable)")
	fs.Var(&ignores, "i", "Ignore environment variables starting with this prefix (can be used multiple times)")
	fs.Var(&presets, "p", "Apply a preset (built-in or from .autoport.json)")
	fs.Var(&portEnv, "k", "Include a port environment key manually (can be used multiple times)")
	fs.Var(&includes, "include", "Include exact port key (can be used multiple times)")
	fs.Var(&excludes, "exclude", "Exclude exact port key (can be used multiple times)")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return app.Options{}, nil, &helpRequestedError{Mode: targetMode}
		}
		return app.Options{}, nil, err
	}

	if err := validateFormat(targetMode, format); err != nil {
		return app.Options{}, nil, err
	}
	if err := app.ValidateTargetEnvSpecs([]string(targetEnvs)); err != nil {
		return app.Options{}, nil, err
	}

	var seedPtr *uint32
	if seed != "" {
		v, err := strconv.ParseUint(seed, 10, 32)
		if err != nil {
			return app.Options{}, nil, fmt.Errorf("invalid --seed %q: %w", seed, err)
		}
		tmp := uint32(v)
		seedPtr = &tmp
	}

	cwd, err := os.Getwd()
	if err != nil {
		return app.Options{}, nil, fmt.Errorf("get cwd: %w", err)
	}

	opts := app.Options{
		Mode:           targetMode,
		Ignores:        ignores,
		Includes:       includes,
		Excludes:       excludes,
		Presets:        presets,
		PortEnv:        portEnv,
		Range:          *rangeFlag,
		Format:         format,
		Quiet:          quiet,
		DryRun:         dryRun,
		CWD:            cwd,
		Namespace:      namespace,
		Seed:           seedPtr,
		Branch:         branch,
		SeedBranch:     seedBranch,
		TargetEnvSpecs: []string(targetEnvs),
		UseLock:        useLock,
	}
	return opts, fs.Args(), nil
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) {
	return len(p), nil
}

type helpRequestedError struct {
	Mode string
}

func (e *helpRequestedError) Error() string {
	return "help requested"
}

func printHelp(w io.Writer, mode string) {
	fmt.Fprintln(w, "autoport - deterministic port wrapper")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  autoport [flags] [command ...]")
	fmt.Fprintln(w, "  autoport explain [flags]")
	fmt.Fprintln(w, "  autoport doctor [flags]")
	fmt.Fprintln(w, "  autoport lock [flags]")
	fmt.Fprintln(w, "  autoport version")
	fmt.Fprintln(w)
	switch mode {
	case "explain":
		fmt.Fprintln(w, "Explain flags: -r, -p, -i, --include, --exclude, -k, --namespace, --seed, --seed-branch, --branch, -e, -f text|json")
	case "doctor":
		fmt.Fprintln(w, "Doctor flags: -r, -p, -i, --include, --exclude, -k, --namespace, --seed, --seed-branch, --branch, --use-lock, -f text|json")
	case "lock":
		fmt.Fprintln(w, "Lock flags: -r, -p, -i, --include, --exclude, -k, --namespace, --seed, --seed-branch, --branch")
	default:
		fmt.Fprintln(w, "Run/export flags: -r, -p, -i, --include, --exclude, -k, -e, --namespace, --seed, --seed-branch, --branch, --use-lock, -f shell|json|dotenv|yaml, -q, -n")
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  autoport npm start")
	fmt.Fprintln(w, "  autoport explain -f json")
	fmt.Fprintln(w, "  autoport doctor")
	fmt.Fprintln(w, "  autoport lock && autoport --use-lock npm start")
}

func defaultFormatForMode(mode string) string {
	switch mode {
	case "explain", "doctor":
		return "text"
	default:
		return "shell"
	}
}

func validateFormat(mode, format string) error {
	allowed := map[string]bool{}
	switch mode {
	case "explain", "doctor":
		allowed["text"] = true
		allowed["json"] = true
	default:
		allowed["shell"] = true
		allowed["json"] = true
		allowed["dotenv"] = true
		allowed["yaml"] = true
	}
	if !allowed[format] {
		return fmt.Errorf("invalid format %q for mode %q", format, mode)
	}
	return nil
}
