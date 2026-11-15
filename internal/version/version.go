// Package version exposes build metadata for go-teams and semantic version
// helpers used by the self-update command.
package version

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Build metadata, overridden at link time by .github/workflows/release.yml via
// -ldflags "-X github.com/softwaresalt/go-teams/internal/version.Version=...".
var (
	// Version is the release version of this build, or "dev" for local builds.
	Version = "dev"
	// Commit is the git commit this build was produced from.
	Commit = "unknown"
	// BuildDate is the UTC timestamp at which this build was produced.
	BuildDate = "unknown"
)

// ErrInvalidVersion reports a version string that is not a valid semantic version.
var ErrInvalidVersion = errors.New("version: invalid semantic version")

// devVersion is the placeholder reported by local builds without link-time metadata.
const devVersion = "dev"

// parsed is a validated semantic version split into comparable parts.
type parsed struct {
	major, minor, patch uint64
	pre                 []string
}

// Normalize strips an optional leading "v" or "V" and validates MAJOR.MINOR.PATCH
// with an optional pre-release suffix. SemVer build metadata ("+...") is
// ignored because it carries no precedence.
func Normalize(v string) (string, error) {
	p, err := parse(v)
	if err != nil {
		return "", err
	}
	out := fmt.Sprintf("%d.%d.%d", p.major, p.minor, p.patch)
	if len(p.pre) > 0 {
		out += "-" + strings.Join(p.pre, ".")
	}
	return out, nil
}

// Compare orders two semantic versions per SemVer 2.0.0, returning -1, 0, or +1.
func Compare(a, b string) (int, error) {
	pa, err := parse(a)
	if err != nil {
		return 0, err
	}
	pb, err := parse(b)
	if err != nil {
		return 0, err
	}
	for _, d := range [...][2]uint64{{pa.major, pb.major}, {pa.minor, pb.minor}, {pa.patch, pb.patch}} {
		if d[0] != d[1] {
			if d[0] < d[1] {
				return -1, nil
			}
			return 1, nil
		}
	}
	return comparePre(pa.pre, pb.pre), nil
}

// IsNewer reports whether candidate is a newer release than current. A current
// version of "dev" is upgradable to any valid candidate.
func IsNewer(current, candidate string) (bool, error) {
	if strings.TrimSpace(current) == devVersion {
		if _, err := parse(candidate); err != nil {
			return false, err
		}
		return true, nil
	}
	c, err := Compare(candidate, current)
	if err != nil {
		return false, err
	}
	return c > 0, nil
}

func parse(raw string) (parsed, error) {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(strings.TrimPrefix(s, "v"), "V")
	if i := strings.IndexByte(s, '+'); i >= 0 {
		for _, id := range strings.Split(s[i+1:], ".") {
			if !validPreID(id) { // build metadata uses the same identifier alphabet
				return parsed{}, fmt.Errorf("%w: %q", ErrInvalidVersion, raw)
			}
		}
		s = s[:i]
	}
	core, pre, hasPre := strings.Cut(s, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return parsed{}, fmt.Errorf("%w: %q", ErrInvalidVersion, raw)
	}
	var nums [3]uint64
	for i, part := range parts {
		n, ok := parseNumeric(part)
		if !ok {
			return parsed{}, fmt.Errorf("%w: %q", ErrInvalidVersion, raw)
		}
		nums[i] = n
	}
	p := parsed{major: nums[0], minor: nums[1], patch: nums[2]}
	if hasPre {
		ids := strings.Split(pre, ".")
		for _, id := range ids {
			if !validPreID(id) {
				return parsed{}, fmt.Errorf("%w: %q", ErrInvalidVersion, raw)
			}
		}
		p.pre = ids
	}
	return p, nil
}

// parseNumeric parses a non-negative decimal without leading zeros.
func parseNumeric(s string) (uint64, bool) {
	if s == "" || (len(s) > 1 && s[0] == '0') {
		return 0, false
	}
	n, err := strconv.ParseUint(s, 10, 64)
	return n, err == nil
}

func validPreID(id string) bool {
	if id == "" {
		return false
	}
	for _, r := range id {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r == '-') {
			return false
		}
	}
	return true
}

// comparePre orders pre-release identifier lists; no pre-release outranks any.
func comparePre(a, b []string) int {
	switch {
	case len(a) == 0 && len(b) == 0:
		return 0
	case len(a) == 0:
		return 1
	case len(b) == 0:
		return -1
	}
	for i := 0; i < len(a) && i < len(b); i++ {
		if c := comparePreID(a[i], b[i]); c != 0 {
			return c
		}
	}
	switch {
	case len(a) < len(b):
		return -1
	case len(a) > len(b):
		return 1
	}
	return 0
}

func comparePreID(a, b string) int {
	aNum, bNum := isDigits(a), isDigits(b)
	switch {
	case aNum && bNum:
		return compareDigits(a, b)
	case aNum:
		return -1
	case bNum:
		return 1
	}
	return strings.Compare(a, b)
}

// isDigits reports whether s is a non-empty all-digit identifier. No integer
// conversion is used so identifiers beyond any fixed width stay numeric.
func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// compareDigits orders two all-digit strings numerically: fewer significant
// digits is smaller, equal widths compare lexically.
func compareDigits(a, b string) int {
	a, b = strings.TrimLeft(a, "0"), strings.TrimLeft(b, "0")
	switch {
	case len(a) < len(b):
		return -1
	case len(a) > len(b):
		return 1
	}
	return strings.Compare(a, b)
}
