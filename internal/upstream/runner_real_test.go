package upstream

import (
	"context"
	"testing"
)

// TestRealRunner_Negative_MissingPin proves RealRunner refuses to exec at
// all when the toolchain isn't pinned, rather than silently execing an
// unpinned `go run`. This is the only RealRunner path this module's tests
// exercise: an actual `go run …@pin` invocation needs network (module
// proxy resolution, PLAN.md I-4's CI note), which CLAUDE.md forbids in
// tests. The exec-and-decode path is covered by spike S1 and, optionally,
// TestIntegration_LocalBinaries (localbin_test.go) when S1's prebuilt
// binaries are available on disk.
func TestRealRunner_Negative_MissingPin(t *testing.T) {
	cases := []RealRunner{
		{Module: "", Commit: "deadbeef"},
		{Module: "example.com/mod", Commit: ""},
		{},
	}
	for _, r := range cases {
		if _, err := r.Run(context.Background(), Request{Bin: "flowmap", Subcommand: "version"}); err == nil {
			t.Fatalf("RealRunner%+v.Run: want error for an unpinned toolchain, got nil", r)
		}
	}
}
