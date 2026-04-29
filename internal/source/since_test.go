package source

import (
	"testing"
	"time"
)

func TestParseSinceRelative(t *testing.T) {
	ref := time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		in   string
		want time.Time
	}{
		{"30s", ref.Add(-30 * time.Second)},
		{"5m", ref.Add(-5 * time.Minute)},
		{"24h", ref.Add(-24 * time.Hour)},
		{"3d", ref.Add(-3 * 24 * time.Hour)},
		{"1w", ref.Add(-7 * 24 * time.Hour)},
	}
	for _, tt := range tests {
		got, err := ParseSince(tt.in, ref)
		if err != nil {
			t.Errorf("ParseSince(%q): %v", tt.in, err)
			continue
		}
		if !got.Equal(tt.want) {
			t.Errorf("ParseSince(%q): got %v want %v", tt.in, got, tt.want)
		}
	}
}

func TestParseSinceAbsoluteRFC3339(t *testing.T) {
	got, err := ParseSince("2026-04-01T00:00:00Z", time.Now())
	if err != nil {
		t.Fatalf("ParseSince: %v", err)
	}
	want := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %v want %v", got, want)
	}
}

func TestParseSinceInvalid(t *testing.T) {
	bad := []string{"", "abc", "3", "d3", "3x", "3.5d", "-3d"}
	for _, s := range bad {
		if _, err := ParseSince(s, time.Now()); err == nil {
			t.Errorf("ParseSince(%q): expected error, got nil", s)
		}
	}
}
