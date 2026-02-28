/*
Package main provides the CLI entry point for autoport.

autoport is a tool that helps manage port collisions by deterministically
assigning free ports based on the project's directory path.
*/
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/gelleson/autoport/internal/app"
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
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// run parses CLI flags and executes the application logic.
func run(ctx context.Context) error {
	opts, cmdArgs, err := parseCLIArgs(os.Args[1:])
	if err != nil {
		return err
	}

	application := app.New()
	return application.Run(ctx, opts, cmdArgs)
}

func parseCLIArgs(args []string) (app.Options, []string, error) {
	var ignores ignoreFlags
	var presets presetFlags
	var portEnv portEnvFlags

	fs := flag.NewFlagSet("autoport", flag.ExitOnError)
	rangeFlag := fs.String("r", "", "Port range to use (e.g., 3000-4000). Default is 10000-20000.")
	fs.Var(&ignores, "i", "Ignore environment variables starting with this prefix (can be used multiple times)")
	fs.Var(&presets, "p", "Apply a preset (built-in or from .autoport.json)")
	fs.Var(&portEnv, "k", "Include a port environment key manually (can be used multiple times)")

	if err := fs.Parse(args); err != nil {
		return app.Options{}, nil, err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return app.Options{}, nil, fmt.Errorf("get cwd: %w", err)
	}

	opts := app.Options{
		Ignores: ignores,
		Presets: presets,
		PortEnv: portEnv,
		Range:   *rangeFlag,
		CWD:     cwd,
	}
	return opts, fs.Args(), nil
}
