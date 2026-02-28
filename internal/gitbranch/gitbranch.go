package gitbranch

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Current resolves the current git branch for repoDir.
func Current(repoDir string) (string, error) {
	if branch, err := runGitBranchCommand(repoDir, "rev-parse", "--abbrev-ref", "HEAD"); err == nil && branch != "" && branch != "HEAD" {
		return branch, nil
	}
	if branch, err := runGitBranchCommand(repoDir, "symbolic-ref", "--short", "HEAD"); err == nil && branch != "" {
		return branch, nil
	}
	return "", fmt.Errorf("resolve git branch for %s: unable to determine branch", repoDir)
}

func runGitBranchCommand(repoDir string, args ...string) (string, error) {
	allArgs := append([]string{"-C", repoDir}, args...)
	cmd := exec.Command("git", allArgs...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%s", msg)
	}
	branch := strings.TrimSpace(stdout.String())
	if branch == "" {
		return "", fmt.Errorf("empty branch output")
	}
	return branch, nil
}
