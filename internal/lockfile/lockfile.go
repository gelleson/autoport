package lockfile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/gelleson/autoport/pkg/port"
)

const (
	FileName = ".autoport.lock.json"
	Version  = 1
)

type Assignment struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type LockFile struct {
	Version        int          `json:"version"`
	CWDFingerprint string       `json:"cwd_fingerprint"`
	Range          string       `json:"range"`
	Assignments    []Assignment `json:"assignments"`
	CreatedAt      string       `json:"created_at"`
}

func Fingerprint(cwd string) string {
	return fmt.Sprintf("%08x", port.HashPath(cwd))
}

func PathFor(cwd string) string {
	return filepath.Join(cwd, FileName)
}

func Write(path, cwd, rangeSpec string, overrides map[string]string) error {
	keys := make([]string, 0, len(overrides))
	for k := range overrides {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	assignments := make([]Assignment, 0, len(keys))
	for _, k := range keys {
		assignments = append(assignments, Assignment{Key: k, Value: overrides[k]})
	}

	lf := LockFile{
		Version:        Version,
		CWDFingerprint: Fingerprint(cwd),
		Range:          rangeSpec,
		Assignments:    assignments,
		CreatedAt:      time.Now().UTC().Format(time.RFC3339),
	}

	data, err := json.MarshalIndent(lf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal lockfile: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write lockfile: %w", err)
	}
	return nil
}

func Read(path string) (LockFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return LockFile{}, err
	}
	var lf LockFile
	if err := json.Unmarshal(data, &lf); err != nil {
		return LockFile{}, fmt.Errorf("parse lockfile: %w", err)
	}
	if lf.Version != Version {
		return LockFile{}, fmt.Errorf("unsupported lockfile version %d", lf.Version)
	}
	return lf, nil
}

func ToMap(assignments []Assignment) map[string]string {
	m := make(map[string]string, len(assignments))
	for _, a := range assignments {
		m[a.Key] = a.Value
	}
	return m
}
