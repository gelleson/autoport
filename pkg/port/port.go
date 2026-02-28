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

// Range represents an inclusive port range.
type Range struct {
	Start int
	End   int
}

// Size returns the number of ports in the range.
func (r Range) Size() int {
	return r.End - r.Start + 1
}

// DefaultIsFree checks if a given port is available on the local machine.
func DefaultIsFree(p int) bool {
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(p))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

// ParseRange parses a range string like "10000-20000" into a Range.
func ParseRange(spec string) (Range, error) {
	parts := strings.Split(spec, "-")
	if len(parts) != 2 {
		return Range{}, fmt.Errorf("invalid range format %q, expected start-end", spec)
	}
	start, err := strconv.Atoi(parts[0])
	if err != nil {
		return Range{}, fmt.Errorf("invalid start port %q: %w", parts[0], err)
	}
	end, err := strconv.Atoi(parts[1])
	if err != nil {
		return Range{}, fmt.Errorf("invalid end port %q: %w", parts[1], err)
	}
	if start > end {
		return Range{}, fmt.Errorf("start port %d must be less than or equal to end port %d", start, end)
	}
	return Range{Start: start, End: end}, nil
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

// Allocator finds deterministic available ports for a given seed and range.
type Allocator struct {
	Seed   uint32
	Range  Range
	IsFree IsFreeFunc
}

// PortFor returns an available deterministic port for the given index.
func (a Allocator) PortFor(index int) (int, error) {
	isFree := a.IsFree
	if isFree == nil {
		isFree = DefaultIsFree
	}
	size := a.Range.Size()
	if size <= 0 {
		return 0, fmt.Errorf("invalid range size: %d", size)
	}

	base := int(a.Seed) + index

	for i := 0; i < size; i++ {
		p := a.Range.Start + (base+i)%size
		if isFree(p) {
			return p, nil
		}
	}
	return 0, fmt.Errorf("no free ports in range %d-%d", a.Range.Start, a.Range.End)
}
