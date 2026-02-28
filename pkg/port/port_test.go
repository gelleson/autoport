package port

import (
	"testing"
)

func TestParseRange(t *testing.T) {
	tests := []struct {
		name    string
		r       string
		want    Range
		wantErr bool
	}{
		{"valid range", "3000-4000", Range{Start: 3000, End: 4000}, false},
		{"invalid format", "3000", Range{}, true},
		{"invalid start", "abc-4000", Range{}, true},
		{"invalid end", "3000-abc", Range{}, true},
		{"start > end", "4000-3000", Range{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRange(tt.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRange() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseRange() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestHashPath(t *testing.T) {
	hash1 := HashPath("/path/to/projectA")
	hash2 := HashPath("/path/to/projectB")
	hash3 := HashPath("/path/to/projectA")

	if hash1 == hash2 {
		t.Errorf("HashPath() collision for different paths")
	}
	if hash1 != hash3 {
		t.Errorf("HashPath() not deterministic for same path")
	}
}

func TestAllocator_PortFor(t *testing.T) {
	seed := uint32(12345)
	r := Range{Start: 10000, End: 10009} // range size 10

	t.Run("first port free", func(t *testing.T) {
		a := Allocator{
			Seed:   seed,
			Range:  r,
			IsFree: func(p int) bool { return true },
		}
		p, err := a.PortFor(0)
		if err != nil {
			t.Errorf("PortFor() unexpected error: %v", err)
		}
		if p < r.Start || p > r.End {
			t.Errorf("PortFor() returned port out of bounds: %d", p)
		}
	})

	t.Run("first port taken, second free", func(t *testing.T) {
		expectedPort := r.Start + (int(seed)+0)%10
		a := Allocator{
			Seed:  seed,
			Range: r,
			IsFree: func(p int) bool {
				return p != expectedPort // Only the first expected one is taken
			},
		}
		p, err := a.PortFor(0)
		if err != nil {
			t.Errorf("PortFor() unexpected error: %v", err)
		}
		if p == expectedPort {
			t.Errorf("PortFor() returned taken port")
		}
	})

	t.Run("no ports free", func(t *testing.T) {
		a := Allocator{
			Seed:   seed,
			Range:  r,
			IsFree: func(p int) bool { return false },
		}
		_, err := a.PortFor(0)
		if err == nil {
			t.Errorf("PortFor() expected error when no ports free")
		}
	})
}
