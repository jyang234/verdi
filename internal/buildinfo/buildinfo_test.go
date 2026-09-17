package buildinfo

import (
	"runtime/debug"
	"strings"
	"testing"
)

// TestFormat is the table-driven happy/negative-path test for format,
// Line's pure core: every case ac-1 (spec/uat-round-1) names — a normal
// build, a dirty build, an unversioned "(devel)" build with VCS info, and
// build info entirely unavailable — plus the never-fabricate negative
// path (empty version, no settings at all).
func TestFormat(t *testing.T) {
	cases := []struct {
		name string
		info *debug.BuildInfo
		ok   bool
		want string
	}{
		{
			name: "build info unavailable prints the honest placeholder",
			info: nil,
			ok:   false,
			want: "verdi (devel)",
		},
		{
			name: "tagged version with a clean VCS revision",
			info: &debug.BuildInfo{
				Main: debug.Module{Version: "v1.2.3"},
				Settings: []debug.BuildSetting{
					{Key: "vcs.revision", Value: "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0"},
					{Key: "vcs.modified", Value: "false"},
				},
			},
			ok:   true,
			want: "verdi v1.2.3 rev=a1b2c3d4e5f6",
		},
		{
			name: "dirty checkout appends the modified marker",
			info: &debug.BuildInfo{
				Main: debug.Module{Version: "v1.2.3"},
				Settings: []debug.BuildSetting{
					{Key: "vcs.revision", Value: "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0"},
					{Key: "vcs.modified", Value: "true"},
				},
			},
			ok:   true,
			want: "verdi v1.2.3 rev=a1b2c3d4e5f6 modified",
		},
		{
			name: "empty main version never fabricates one, falls back to (devel)",
			info: &debug.BuildInfo{
				Main:     debug.Module{Version: ""},
				Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "deadbeefcafefeed0011"}},
			},
			ok:   true,
			want: "verdi (devel) rev=deadbeefcafe",
		},
		{
			name: "ok build info but no VCS settings at all",
			info: &debug.BuildInfo{Main: debug.Module{Version: "v0.9.0"}},
			ok:   true,
			want: "verdi v0.9.0",
		},
		{
			name: "revision shorter than the short length is used verbatim, never padded",
			info: &debug.BuildInfo{
				Main:     debug.Module{Version: "v0.9.0"},
				Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "abc123"}},
			},
			ok:   true,
			want: "verdi v0.9.0 rev=abc123",
		},
		{
			name: "ok true but info nil is treated the same as unavailable",
			info: nil,
			ok:   true,
			want: "verdi (devel)",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := format(tc.info, tc.ok); got != tc.want {
				t.Fatalf("format(%+v, %v) = %q, want %q", tc.info, tc.ok, got, tc.want)
			}
		})
	}
}

// TestLine_RealBuildInfo is a light integration check of the real,
// unmocked seam: in a `go test` binary, runtime/debug.ReadBuildInfo
// succeeds, so Line must return a non-empty line starting with "verdi "
// — never empty, never a panic — without asserting the exact VCS
// content (environment-dependent: whether this checkout's git metadata
// is embedded depends on the toolchain, not this package's logic, which
// TestFormat already covers exhaustively).
func TestLine_RealBuildInfo(t *testing.T) {
	got := Line()
	if !strings.HasPrefix(got, "verdi ") {
		t.Fatalf("Line() = %q, want it to start with %q", got, "verdi ")
	}
}
