package source

import (
	"fmt"
	"strconv"
	"time"
)

// ParseSince converts a user-supplied duration expression or RFC-3339
// timestamp to an absolute time. Relative expressions are subtracted
// from `ref` (normally time.Now()).
//
// Accepted relative units: s, m, h, d, w. Negative values are rejected.
func ParseSince(s string, ref time.Time) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("since: empty")
	}

	// Absolute RFC-3339 first.
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), nil
	}

	// Relative: <digits><unit>.
	if len(s) < 2 {
		return time.Time{}, fmt.Errorf("since: %q too short", s)
	}
	unit := s[len(s)-1]
	numPart := s[:len(s)-1]
	n, err := strconv.Atoi(numPart)
	if err != nil {
		return time.Time{}, fmt.Errorf("since: %q is not <N><unit>", s)
	}
	if n < 0 {
		return time.Time{}, fmt.Errorf("since: negative value %q", s)
	}
	var d time.Duration
	switch unit {
	case 's':
		d = time.Duration(n) * time.Second
	case 'm':
		d = time.Duration(n) * time.Minute
	case 'h':
		d = time.Duration(n) * time.Hour
	case 'd':
		d = time.Duration(n) * 24 * time.Hour
	case 'w':
		d = time.Duration(n) * 7 * 24 * time.Hour
	default:
		return time.Time{}, fmt.Errorf("since: unknown unit %q in %q", string(unit), s)
	}
	return ref.Add(-d).UTC(), nil
}
