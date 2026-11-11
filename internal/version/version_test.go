package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultsMatchReleaseLdflagsTargets(t *testing.T) {
	assert.Equal(t, "dev", Version)
	assert.Equal(t, "unknown", Commit)
	assert.Equal(t, "unknown", BuildDate)
}

func TestNormalize(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"plain", "1.2.3", "1.2.3", false},
		{"lower v prefix", "v1.2.3", "1.2.3", false},
		{"upper V prefix", "V1.2.3", "1.2.3", false},
		{"surrounding space", "  v0.10.0 ", "0.10.0", false},
		{"pre-release", "v1.2.3-rc.1", "1.2.3-rc.1", false},
		{"build metadata dropped", "v1.2.3+abc", "1.2.3", false},
		{"build metadata with dots and hyphens", "1.2.3+build.2-x", "1.2.3", false},
		{"empty build metadata", "1.2.3+", "", true},
		{"build metadata path traversal", "1.2.3+../x", "", true},
		{"build metadata empty identifier", "1.2.3+a..b", "", true},
		{"empty", "", "", true},
		{"dev", "dev", "", true},
		{"two parts", "1.2", "", true},
		{"four parts", "1.2.3.4", "", true},
		{"non numeric", "v1.x.3", "", true},
		{"leading zero", "01.2.3", "", true},
		{"empty pre-release", "1.2.3-", "", true},
		{"negative", "-1.2.3", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Normalize(tt.in)
			if tt.wantErr {
				require.ErrorIs(t, err, ErrInvalidVersion)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCompare(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want int
	}{
		{"equal", "1.2.3", "v1.2.3", 0},
		{"major", "2.0.0", "1.9.9", 1},
		{"minor", "1.2.0", "1.10.0", -1},
		{"patch", "1.2.4", "1.2.3", 1},
		{"numeric not lexical", "1.10.0", "1.9.0", 1},
		{"pre-release before release", "1.2.3-rc.1", "1.2.3", -1},
		{"release after pre-release", "1.2.3", "1.2.3-rc.1", 1},
		{"pre-release numeric ids", "1.2.3-rc.2", "1.2.3-rc.10", -1},
		{"pre-release alpha vs numeric", "1.2.3-alpha", "1.2.3-1", 1},
		{"pre-release longer wins", "1.2.3-alpha", "1.2.3-alpha.1", -1},
		{"pre-release lexical", "1.2.3-alpha", "1.2.3-beta", -1},
		{"huge numeric pre-release ids beyond uint64", "1.2.3-20000000000000000000", "1.2.3-100000000000000000000", -1},
		{"huge numeric pre-release equal", "1.2.3-18446744073709551616", "1.2.3-18446744073709551616", 0},
		{"huge numeric before alphanumeric", "1.2.3-99999999999999999999999", "1.2.3-alpha", -1},
		{"build metadata ignored", "1.2.3+build.1", "1.2.3+build.2", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Compare(tt.a, tt.b)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			rev, err := Compare(tt.b, tt.a)
			require.NoError(t, err)
			assert.Equal(t, -tt.want, rev)
		})
	}
}

func TestCompareRejectsMalformed(t *testing.T) {
	_, err := Compare("1.2.3", "nope")
	require.ErrorIs(t, err, ErrInvalidVersion)
	_, err = Compare("", "1.2.3")
	require.ErrorIs(t, err, ErrInvalidVersion)
}

func TestIsNewer(t *testing.T) {
	tests := []struct {
		name      string
		current   string
		candidate string
		want      bool
		wantErr   bool
	}{
		{"newer", "1.0.0", "v1.1.0", true, false},
		{"same", "v1.1.0", "1.1.0", false, false},
		{"older", "1.2.0", "1.1.0", false, false},
		{"dev upgrades", "dev", "0.0.1", true, false},
		{"dev upgrades prefixed", "dev", "v2.0.0", true, false},
		{"pre-release to release", "1.2.3-rc.1", "1.2.3", true, false},
		{"dev to malformed", "dev", "garbage", false, true},
		{"malformed current", "garbage", "1.0.0", false, true},
		{"malformed candidate", "1.0.0", "", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IsNewer(tt.current, tt.candidate)
			if tt.wantErr {
				require.ErrorIs(t, err, ErrInvalidVersion)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
