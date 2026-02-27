package port

import (
	"fmt"
	"hash/fnv"
	"net"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	// DefaultRange is the default port range used if none is specified.
	DefaultRange = "10000-20000"
)

// IsFreeFunc defines a function signature for checking if a port is free.
type IsFreeFunc func(p int) bool

// DefaultIsFree checks if a given port is available on the local machine.
func DefaultIsFree(p int) bool {
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(p))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

// ParseRange parses a range string like "10000-20000" into start and end integers.
func ParseRange(r string) (start, end int, err error) {
	parts := strings.Split(r, "-")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid range format %q, expected start-end", r)
	}
	start, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid start port %q: %w", parts[0], err)
	}
	end, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid end port %q: %w", parts[1], err)
	}
	if start > end {
		return 0, 0, fmt.Errorf("start port %d must be less than or equal to end port %d", start, end)
	}
	return start, end, nil
}

// HashPath generates a deterministic 32-bit hash for a given file path.
func HashPath(path string) uint32 {
	abs, err := filepath.Abs(path)
	if err == nil {
		path = abs
	}
	h := fnv.New32a()
	h.Write([]byte(path))
	return h.Sum32()
}

// FindDeterministic finds an available port deterministically within a range based on a seed and index.
func FindDeterministic(seed uint32, index int, start, end int, isFree IsFreeFunc) (int, error) {
	if isFree == nil {
		isFree = DefaultIsFree
	}
	size := end - start + 1
	if size <= 0 {
		return 0, fmt.Errorf("invalid range size: %d", size)
	}
	
	base := int(seed) + index

	for i := 0; i < size; i++ {
		p := start + (base+i)%size
		if isFree(p) {
			return p, nil
		}
	}
	return 0, fmt.Errorf("no free ports in range %d-%d", start, end)
}
