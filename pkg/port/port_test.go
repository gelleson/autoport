package port

import (
	"testing"
)

func TestParseRange(t *testing.T) {
	tests := []struct {
		name      string
		r         string
		wantStart int
		wantEnd   int
		wantErr   bool
	}{
		{"valid range", "3000-4000", 3000, 4000, false},
		{"invalid format", "3000", 0, 0, true},
		{"invalid start", "abc-4000", 0, 0, true},
		{"invalid end", "3000-abc", 0, 0, true},
		{"start > end", "4000-3000", 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, err := ParseRange(tt.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRange() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if start != tt.wantStart {
				t.Errorf("ParseRange() start = %v, want %v", start, tt.wantStart)
			}
			if end != tt.wantEnd {
				t.Errorf("ParseRange() end = %v, want %v", end, tt.wantEnd)
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

func TestFindDeterministic(t *testing.T) {
	seed := uint32(12345)
	start := 10000
	end := 10009 // range size 10

	t.Run("first port free", func(t *testing.T) {
		isFree := func(p int) bool { return true }
		p, err := FindDeterministic(seed, 0, start, end, isFree)
		if err != nil {
			t.Errorf("FindDeterministic() unexpected error: %v", err)
		}
		if p < start || p > end {
			t.Errorf("FindDeterministic() returned port out of bounds: %d", p)
		}
	})

	t.Run("first port taken, second free", func(t *testing.T) {
		expectedPort := start + (int(seed)+0)%10
		isFree := func(p int) bool {
			return p != expectedPort // Only the first expected one is taken
		}
		p, err := FindDeterministic(seed, 0, start, end, isFree)
		if err != nil {
			t.Errorf("FindDeterministic() unexpected error: %v", err)
		}
		if p == expectedPort {
			t.Errorf("FindDeterministic() returned taken port")
		}
	})

	t.Run("no ports free", func(t *testing.T) {
		isFree := func(p int) bool { return false }
		_, err := FindDeterministic(seed, 0, start, end, isFree)
		if err == nil {
			t.Errorf("FindDeterministic() expected error when no ports free")
		}
	})
}
