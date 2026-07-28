package mcpserve

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
)

// gitCmd runs a git command in dir hermetically (a local file:// remote, never
// the network), failing the test on any error.
func gitCmd(ctx context.Context, t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s (dir %s): %v\n%s", strings.Join(args, " "), dir, err, out)
	}
}

// TestGetMatrix_ResetsShallowCachePerRequest proves the I3 wiring: GetMatrix
// re-establishes shallow-state freshness at the request boundary
// (gitx.ResetShallowCache), so the long-lived MCP server never inherits a stale
// cached `false` across a checkout reshaped full->shallow between requests —
// which would make the reachability probes evidence.LoadRecords runs serve a
// would-be-negative as a false PROVEN Unreachable.
//
// It genuinely poisons the process-global memo (full clone -> would-be-negative
// probe caches false -> reshape full->shallow in place), confirms the poison is
// live, then calls GetMatrix and proves the same gitx primitive now re-probes to
// the honest UnprovableShallow — the poisoned entry was cleared. The story need
// not resolve: the reset fires at the boundary BEFORE resolution, so even a
// GetMatrix that errors afterward proves the wiring; a future refactor dropping
// the reset call reds this test while the gitx unit test alone would stay green.
func TestGetMatrix_ResetsShallowCachePerRequest(t *testing.T) {
	ctx := context.Background()

	// Three layers so a depth-2 reshape leaves L1 beyond the shallow horizon.
	src := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"a.txt": "one\n"}, Message: "layer 1"},
		{Files: map[string]string{"a.txt": "two\n"}, Message: "layer 2"},
		{Files: map[string]string{"a.txt": "three\n"}, Message: "layer 3"},
	})
	parent := t.TempDir()
	checkout := filepath.Join(parent, "checkout")
	gitCmd(ctx, t, parent, "clone", "--quiet", "file://"+src.Dir, checkout)
	t.Cleanup(gitx.ResetShallowCache) // never leak the poisoned memo entry

	// While full, a would-be-negative (a well-formed but nonexistent sha) caches
	// shallow=false for checkout — exactly how an earlier server request populates
	// the memo before the reshape.
	const nonexistent = "0000000000000000000000000000000000000000"
	if got, err := gitx.ReachableFromHEAD(ctx, checkout, nonexistent, "HEAD"); err != nil || got != gitx.Unreachable {
		t.Fatalf("full-clone would-be-negative = %v, %v; want Unreachable, nil (caches shallow=false)", got, err)
	}

	// Reshape full->shallow in place: L1 now sits beyond the depth-2 horizon, a
	// would-be-negative whose honest answer is UnprovableShallow.
	gitCmd(ctx, t, checkout, "fetch", "--quiet", "--depth=2", "origin")
	beyond := src.Heads[0]

	// The memo is now stale: the poisoned false makes the would-be-negative read a
	// PROVEN Unreachable — the false NO the long-lived server must never serve.
	if got, err := gitx.ReachableFromHEAD(ctx, checkout, beyond, "HEAD"); err != nil || got != gitx.Unreachable {
		t.Fatalf("pre-GetMatrix stale query = %v, %v; want Unreachable (the memo still holds the poisoned false)", got, err)
	}

	// A GetMatrix call crosses the request boundary and must reset the memo. Its
	// Root need not be a resolvable store and its story need not exist — the reset
	// fires before resolution, so an erroring call still exercises the wiring.
	b := &Backend{Root: src.Dir}
	_ = b.GetMatrix(ctx, mustArgs(t, map[string]any{"story": "spec/definitely-not-a-real-spec-xyz"}))

	// Cured: the poisoned entry was cleared, so the gitx primitive re-probes the
	// now-shallow checkout and returns the honest UnprovableShallow.
	cured, err := gitx.ReachableFromHEAD(ctx, checkout, beyond, "HEAD")
	if err != nil {
		t.Fatalf("post-GetMatrix query: unexpected error %v", err)
	}
	if cured != gitx.UnprovableShallow {
		t.Fatalf("post-GetMatrix query = %v, want UnprovableShallow — GetMatrix must reset the shallow-state memo at the request boundary (gitx.ResetShallowCache), clearing the stale false", cured)
	}
}
