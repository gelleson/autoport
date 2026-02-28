package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/gelleson/autoport/internal/config"
	"github.com/gelleson/autoport/internal/env"
	"github.com/gelleson/autoport/internal/linkspec"
	"github.com/gelleson/autoport/internal/lockfile"
	"github.com/gelleson/autoport/internal/scanner"
	"github.com/gelleson/autoport/pkg/port"
)

type sourceValues struct {
	byActual map[string]string
	byNorm   map[string]string
}

type rewriteCandidate struct {
	SourceKey       string
	TargetRepo      string
	TargetPortKey   string
	TargetNamespace string
	SameBranch      bool
	SourceDesc      string
}

func (a *App) applyLinkRewrites(ctx context.Context, opts Options, res resolvedOptions, r port.Range, targetSpecs []linkspec.TargetEnvSpec, overrides map[string]string) ([]linkRewrite, []string, error) {
	if len(res.Links) == 0 && len(targetSpecs) == 0 {
		return nil, nil, nil
	}

	src, warnings := a.collectSourceValues(opts.CWD, res)
	candidates, candidateWarnings := a.buildRewriteCandidates(opts, res.Links, targetSpecs, src)
	warnings = append(warnings, candidateWarnings...)
	if len(candidates) == 0 {
		return nil, warnings, nil
	}

	branchCache := map[string]branchResult{}
	resolveBranch := func(repo string) (string, error) {
		repo = filepath.Clean(repo)
		if cached, ok := branchCache[repo]; ok {
			return cached.branch, cached.err
		}
		if a.resolveBranch == nil {
			err := fmt.Errorf("branch resolver unavailable")
			branchCache[repo] = branchResult{err: err}
			return "", err
		}
		branch, err := a.resolveBranch(repo)
		branchCache[repo] = branchResult{branch: branch, err: err}
		return branch, err
	}

	sourceBranch := strings.TrimSpace(opts.Branch)
	sourceBranchSet := sourceBranch != ""

	rewrites := []linkRewrite{}
	for _, candidate := range candidates {
		actualSourceKey, oldValue, ok := src.lookup(candidate.SourceKey)
		if !ok {
			warnings = append(warnings, fmt.Sprintf("%s: source key %q not found", candidate.SourceDesc, candidate.SourceKey))
			continue
		}
		if _, _, err := parseLoopbackURL(oldValue); err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: source key %q is not a localhost URL (%v)", candidate.SourceDesc, actualSourceKey, err))
			continue
		}

		targetRepo, err := absolutePath(opts.CWD, candidate.TargetRepo)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: resolve target repo %q: %v", candidate.SourceDesc, candidate.TargetRepo, err))
			continue
		}
		info, statErr := os.Stat(targetRepo)
		if statErr != nil || !info.IsDir() {
			warnings = append(warnings, fmt.Sprintf("%s: target repo %q is unavailable", candidate.SourceDesc, targetRepo))
			continue
		}

		if candidate.SameBranch {
			if !sourceBranchSet {
				resolvedSourceBranch, err := resolveBranch(opts.CWD)
				if err != nil {
					warnings = append(warnings, fmt.Sprintf("%s: source branch resolution failed: %v", candidate.SourceDesc, err))
					continue
				}
				sourceBranch = resolvedSourceBranch
				sourceBranchSet = true
			}
			targetBranch, err := resolveBranch(targetRepo)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("%s: target branch resolution failed for %q: %v", candidate.SourceDesc, targetRepo, err))
				continue
			}
			if sourceBranch != targetBranch {
				warnings = append(warnings, fmt.Sprintf("%s: branch mismatch source=%q target=%q; skipping %s", candidate.SourceDesc, sourceBranch, targetBranch, actualSourceKey))
				continue
			}
		}

		targetPort, targetKey, portSource, portWarnings, err := a.resolveTargetPort(ctx, opts, r, candidate, targetRepo, resolveBranch)
		warnings = append(warnings, portWarnings...)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: resolve target port for %q failed: %v", candidate.SourceDesc, actualSourceKey, err))
			continue
		}

		newValue, err := replaceLoopbackURLPort(oldValue, targetPort)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: rewrite %q failed: %v", candidate.SourceDesc, actualSourceKey, err))
			continue
		}

		overrides[actualSourceKey] = newValue
		rewrites = append(rewrites, linkRewrite{
			SourceKey:  actualSourceKey,
			OldValue:   oldValue,
			NewValue:   newValue,
			TargetRepo: targetRepo,
			TargetKey:  targetKey,
			PortSource: portSource,
		})
	}

	return rewrites, warnings, nil
}

type branchResult struct {
	branch string
	err    error
}

func (a *App) resolveTargetPort(ctx context.Context, opts Options, defaultRange port.Range, candidate rewriteCandidate, targetRepo string, resolveBranch func(repo string) (string, error)) (int, string, string, []string, error) {
	warnings := []string{}
	lockPath := lockfile.PathFor(targetRepo)

	fallbackRange := defaultRange
	if lf, err := lockfile.Read(lockPath); err == nil {
		if parsedRange, rangeErr := port.ParseRange(lf.Range); rangeErr == nil {
			fallbackRange = parsedRange
		}
		if key, value, ok := chooseAssignment(lf.Assignments, candidate.TargetPortKey); ok {
			p, parseErr := strconv.Atoi(value)
			if parseErr == nil {
				return p, key, "lockfile", warnings, nil
			}
			warnings = append(warnings, fmt.Sprintf("target lockfile %q contains non-numeric value for %q", lockPath, key))
		} else if candidate.TargetPortKey != "" {
			warnings = append(warnings, fmt.Sprintf("target lockfile %q missing key %q; falling back to deterministic lookup", lockPath, candidate.TargetPortKey))
		} else {
			warnings = append(warnings, fmt.Sprintf("target lockfile %q missing APP_PORT/PORT; falling back to deterministic lookup", lockPath))
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		warnings = append(warnings, fmt.Sprintf("target lockfile read failed for %q: %v; falling back to deterministic lookup", lockPath, err))
	}

	keys, err := discoverPortKeys(ctx, targetRepo)
	if err != nil {
		return 0, "", "", warnings, err
	}
	targetKey, targetIndex := chooseTargetKey(keys, candidate.TargetPortKey)
	if targetIndex < 0 {
		if candidate.TargetPortKey != "" {
			return 0, "", "", warnings, fmt.Errorf("target key %q was not discovered in %q", candidate.TargetPortKey, targetRepo)
		}
		return 0, "", "", warnings, fmt.Errorf("neither APP_PORT nor PORT was discovered in %q", targetRepo)
	}

	targetSeed, seedWarnings := a.computeSeedForRepo(targetRepo, candidate.TargetNamespace, opts.SeedBranch, resolveBranch)
	warnings = append(warnings, seedWarnings...)
	targetPort, err := preferredPort(targetSeed, fallbackRange, targetIndex)
	if err != nil {
		return 0, "", "", warnings, err
	}
	return targetPort, targetKey, "deterministic", warnings, nil
}

func (a *App) computeSeedForRepo(repoDir, namespace string, seedBranch bool, resolveBranch func(repo string) (string, error)) (uint32, []string) {
	if !seedBranch {
		return port.SeedFor(repoDir, namespace), nil
	}
	branch, err := resolveBranch(repoDir)
	if err != nil {
		return port.SeedFor(repoDir, namespace), []string{
			fmt.Sprintf("seed-branch enabled but branch resolution failed for %s: %v; falling back to non-branch seed", repoDir, err),
		}
	}
	return port.SeedFor(repoDir, appendBranchNamespace(namespace, branch)), nil
}

func preferredPort(seed uint32, r port.Range, index int) (int, error) {
	size := r.Size()
	if size <= 0 {
		return 0, fmt.Errorf("invalid range size: %d", size)
	}
	base := int(seed) + index
	return r.Start + base%size, nil
}

func discoverPortKeys(ctx context.Context, repoDir string) ([]string, error) {
	s := scanner.New(repoDir, scanner.WithEnviron([]string{}))
	discoveries, _, err := s.ScanDetailed(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(discoveries))
	for _, d := range discoveries {
		out = append(out, d.Key)
	}
	sort.Strings(out)
	return out, nil
}

func chooseTargetKey(keys []string, requested string) (string, int) {
	if requested != "" {
		for idx, key := range keys {
			if strings.EqualFold(key, requested) {
				return key, idx
			}
		}
		return "", -1
	}
	for _, prefer := range []string{"APP_PORT", "PORT"} {
		for idx, key := range keys {
			if strings.EqualFold(key, prefer) {
				return key, idx
			}
		}
	}
	return "", -1
}

func chooseAssignment(assignments []lockfile.Assignment, requested string) (string, string, bool) {
	if requested != "" {
		for _, a := range assignments {
			if strings.EqualFold(a.Key, requested) {
				return a.Key, a.Value, true
			}
		}
		return "", "", false
	}
	for _, prefer := range []string{"APP_PORT", "PORT"} {
		for _, a := range assignments {
			if strings.EqualFold(a.Key, prefer) {
				return a.Key, a.Value, true
			}
		}
	}
	return "", "", false
}

func (a *App) buildRewriteCandidates(opts Options, configLinks []config.LinkRule, specs []linkspec.TargetEnvSpec, src sourceValues) ([]rewriteCandidate, []string) {
	all := []rewriteCandidate{}
	warnings := []string{}

	for _, spec := range specs {
		if spec.Mode != linkspec.ModeExplicit {
			continue
		}
		targetPath, err := absolutePath(opts.CWD, spec.EnvPath)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("target-env %q cannot be resolved: %v", spec.Raw, err))
			continue
		}
		all = append(all, rewriteCandidate{
			SourceKey:     spec.SourceKey,
			TargetRepo:    filepath.Dir(targetPath),
			TargetPortKey: spec.TargetPortKey,
			SameBranch:    true,
			SourceDesc:    fmt.Sprintf("target-env explicit (%s)", spec.Raw),
		})
	}

	for i, link := range configLinks {
		sameBranch := true
		if link.SameBranch != nil {
			sameBranch = *link.SameBranch
		}
		all = append(all, rewriteCandidate{
			SourceKey:       link.SourceKey,
			TargetRepo:      link.TargetRepo,
			TargetPortKey:   link.TargetPortKey,
			TargetNamespace: link.TargetNamespace,
			SameBranch:      sameBranch,
			SourceDesc:      fmt.Sprintf("config link[%d]", i),
		})
	}

	for _, spec := range specs {
		if spec.Mode != linkspec.ModeSmart {
			continue
		}
		inferred, inferWarnings := inferSmartCandidates(opts.CWD, spec, src)
		warnings = append(warnings, inferWarnings...)
		all = append(all, inferred...)
	}

	out := make([]rewriteCandidate, 0, len(all))
	seen := map[string]struct{}{}
	for _, candidate := range all {
		norm := normalizeEnvKey(candidate.SourceKey)
		if _, ok := seen[norm]; ok {
			continue
		}
		seen[norm] = struct{}{}
		out = append(out, candidate)
	}
	return out, warnings
}

func inferSmartCandidates(cwd string, spec linkspec.TargetEnvSpec, src sourceValues) ([]rewriteCandidate, []string) {
	warnings := []string{}
	targetPath, err := absolutePath(cwd, spec.EnvPath)
	if err != nil {
		return nil, []string{fmt.Sprintf("target-env smart (%s): resolve path failed: %v", spec.Raw, err)}
	}

	file, err := os.Open(targetPath)
	if err != nil {
		return nil, []string{fmt.Sprintf("target-env smart (%s): open failed: %v", spec.Raw, err)}
	}
	defer file.Close()

	targetValues := env.Parse(file)
	portToTargetKeys := map[string][]string{}
	for key, value := range targetValues {
		if !isPortLikeKey(key) {
			continue
		}
		if _, err := strconv.Atoi(value); err != nil {
			continue
		}
		portToTargetKeys[value] = append(portToTargetKeys[value], key)
	}

	out := []rewriteCandidate{}
	for sourceKey, sourceValue := range src.byActual {
		_, sourcePort, err := parseLoopbackURL(sourceValue)
		if err != nil {
			continue
		}
		targetKeys := portToTargetKeys[strconv.Itoa(sourcePort)]
		if len(targetKeys) == 0 {
			continue
		}
		if len(targetKeys) > 1 {
			warnings = append(warnings, fmt.Sprintf("target-env smart (%s): source %q matched multiple target keys %v", spec.Raw, sourceKey, targetKeys))
			continue
		}
		out = append(out, rewriteCandidate{
			SourceKey:     sourceKey,
			TargetRepo:    filepath.Dir(targetPath),
			TargetPortKey: targetKeys[0],
			SameBranch:    true,
			SourceDesc:    fmt.Sprintf("target-env smart (%s)", spec.Raw),
		})
	}
	if len(out) == 0 {
		warnings = append(warnings, fmt.Sprintf("target-env smart (%s): no matching localhost URL keys found", spec.Raw))
	}
	return out, warnings
}

func (a *App) collectSourceValues(cwd string, res resolvedOptions) (sourceValues, []string) {
	out := sourceValues{
		byActual: map[string]string{},
		byNorm:   map[string]string{},
	}
	warnings := []string{}
	for _, kv := range a.environ {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			continue
		}
		out.add(parts[0], parts[1])
	}

	walkErr := filepath.WalkDir(cwd, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, relErr := filepath.Rel(cwd, path)
		if relErr != nil {
			rel = path
		}
		depth := relativeDepth(rel)
		if d.IsDir() {
			if isHiddenDirName(d.Name()) {
				return filepath.SkipDir
			}
			for _, ignored := range res.IgnoreDirs {
				if ignored != "" && d.Name() == ignored {
					return filepath.SkipDir
				}
			}
			if res.MaxDepth > 0 && depth > res.MaxDepth {
				return filepath.SkipDir
			}
			return nil
		}
		if !isEnvFileName(d.Name()) {
			return nil
		}

		file, openErr := os.Open(path)
		if openErr != nil {
			warnings = append(warnings, fmt.Sprintf("source env read failed (%s): %v", rel, openErr))
			return nil
		}
		defer file.Close()
		parsed := env.Parse(file)
		for key, value := range parsed {
			out.add(key, value)
		}
		return nil
	})
	if walkErr != nil {
		warnings = append(warnings, fmt.Sprintf("source env scan failed: %v", walkErr))
	}
	return out, warnings
}

func (s *sourceValues) add(key, value string) {
	norm := normalizeEnvKey(key)
	if _, exists := s.byNorm[norm]; exists {
		return
	}
	s.byNorm[norm] = key
	s.byActual[key] = value
}

func (s *sourceValues) lookup(key string) (string, string, bool) {
	actual, ok := s.byNorm[normalizeEnvKey(key)]
	if !ok {
		return "", "", false
	}
	value, ok := s.byActual[actual]
	return actual, value, ok
}

func normalizeEnvKey(key string) string {
	return strings.ToUpper(strings.TrimSpace(key))
}

func parseLoopbackURL(raw string) (*url.URL, int, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, 0, err
	}
	host := u.Hostname()
	if host != "localhost" && host != "127.0.0.1" {
		return nil, 0, fmt.Errorf("host %q is not loopback", host)
	}
	portStr := u.Port()
	if portStr == "" {
		return nil, 0, fmt.Errorf("missing port")
	}
	p, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid port %q", portStr)
	}
	return u, p, nil
}

func replaceLoopbackURLPort(raw string, p int) (string, error) {
	u, _, err := parseLoopbackURL(raw)
	if err != nil {
		return "", err
	}
	host := u.Hostname()
	u.Host = net.JoinHostPort(host, strconv.Itoa(p))
	return u.String(), nil
}

func absolutePath(base, path string) (string, error) {
	full := path
	if !filepath.IsAbs(path) {
		full = filepath.Join(base, path)
	}
	return filepath.Abs(full)
}

func relativeDepth(rel string) int {
	if rel == "." || rel == "" {
		return 0
	}
	return len(strings.Split(rel, string(filepath.Separator))) - 1
}

func isHiddenDirName(name string) bool {
	return strings.HasPrefix(name, ".") && name != "."
}

func isEnvFileName(name string) bool {
	return name == ".env" || strings.HasPrefix(name, ".env.")
}

func isPortLikeKey(key string) bool {
	up := strings.ToUpper(key)
	return up == "PORT" || strings.HasSuffix(up, "_PORT")
}
