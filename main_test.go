package main

import (
	"os"
	"reflect"
	"testing"
)

func TestParseCLIArgs(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	opts, cmdArgs, err := parseCLIArgs([]string{
		"-i", "REDIS_",
		"-i", "DB_",
		"-p", "db",
		"-k", "WEB_PORT",
		"-k", "API_PORT",
		"-r", "3000-4000",
		"-f", "json",
		"-q",
		"-n",
		"npm", "start",
	})
	if err != nil {
		t.Fatalf("parseCLIArgs() unexpected error: %v", err)
	}

	if opts.CWD != cwd {
		t.Fatalf("parseCLIArgs() CWD = %s, want %s", opts.CWD, cwd)
	}
	if opts.Range != "3000-4000" {
		t.Fatalf("parseCLIArgs() Range = %s, want 3000-4000", opts.Range)
	}
	if opts.Format != "json" {
		t.Fatalf("parseCLIArgs() Format = %s, want json", opts.Format)
	}
	if !opts.Quiet {
		t.Fatal("parseCLIArgs() Quiet = false, want true")
	}
	if !opts.DryRun {
		t.Fatal("parseCLIArgs() DryRun = false, want true")
	}
	if !reflect.DeepEqual(opts.Ignores, []string{"REDIS_", "DB_"}) {
		t.Fatalf("parseCLIArgs() Ignores = %v", opts.Ignores)
	}
	if !reflect.DeepEqual(opts.Presets, []string{"db"}) {
		t.Fatalf("parseCLIArgs() Presets = %v", opts.Presets)
	}
	if !reflect.DeepEqual(opts.PortEnv, []string{"WEB_PORT", "API_PORT"}) {
		t.Fatalf("parseCLIArgs() PortEnv = %v", opts.PortEnv)
	}
	if !reflect.DeepEqual(cmdArgs, []string{"npm", "start"}) {
		t.Fatalf("parseCLIArgs() args = %v", cmdArgs)
	}
}

func TestParseCLIArgs_InvalidFormat(t *testing.T) {
	_, _, err := parseCLIArgs([]string{"-f", "yaml"})
	if err == nil {
		t.Fatal("parseCLIArgs() expected error for invalid format")
	}
}
