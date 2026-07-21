package lint

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

// lintRepoRootForRealFileProofs resolves THIS repo's module root from this
// source file's own compiled-in path (runtime.Caller), independent of the
// test binary's working directory — the same computed-root discipline
// internal/specalign's helpers use, so the real-file proof below asserts
// against this repo's actual checkout rather than a synthetic fixture.
func lintRepoRootForRealFileProofs(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	// this file lives at <root>/internal/lint/vl003_realroot_test.go
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

// findingsMentionSentinel reports whether any finding's message names the
// sentinel ref.
func findingsMentionSentinel(findings []Finding, sentinel string) bool {
	for _, f := range findings {
		if strings.Contains(f.Message, sentinel) {
			return true
		}
	}
	return false
}

// TestVL003_RealRootBindings_DiscoveredAndChecked is
// judged-ac4-real-root-bindings-proof-vacuity: a non-vacuity pin that THIS
// real repository's module-root verdi.bindings.yaml (spec/ritual-traps ac-3)
// is actually discovered into Snapshot.RootBindings AND actually consumed by
// checkBindings' root-discovery branch — so a future root-resolution
// regression (a snapshot path change, or a discovery predicate that stops
// finding the real file) cannot SILENTLY evaporate the real-file proof while
// every SYNTHETIC-fixture VL-003 test keeps passing.
//
// The existing synthetic proofs (TestVL003_RootBindings_BadBareAC_*) prove
// the MECHANISM on temp-dir fixtures; none proves the mechanism ENGAGES the
// real committed file. This does, in two independent legs whose delta is
// solely the RootBindings field:
//
//   - discovery: BuildSnapshot over the real root sets RootBindings non-nil
//     (the real repo genuinely ships this committed sidecar, sibling of
//     .verdi/, with no root .flowmap.yaml per D6-4). Red-first: neutralizing
//     discovery (snapshot.go no longer reading the real root file, or reading
//     a wrong path) reds this leg — demonstrated during development by
//     pointing snapshot.go's rootBindingsPath at a nonexistent name, which
//     turns RootBindings nil and fails here.
//   - consumption: injecting a sentinel entry naming a nonexistent target
//     spec into the real RootBindings makes checkBindings red BY that
//     sentinel — and withholding RootBindings (nil) silences exactly that
//     sentinel — proving the finding travels the root-discovery branch and
//     nothing else. A fragment entry naming a definitely-absent spec is used
//     so the proof does not depend on the real file's own owning spec
//     resolving or on any particular real AC id.
func TestVL003_RealRootBindings_DiscoveredAndChecked(t *testing.T) {
	root := lintRepoRootForRealFileProofs(t)
	snap, err := BuildSnapshot(root, Options{})
	if err != nil {
		t.Fatalf("BuildSnapshot(%s): %v", root, err)
	}

	// Discovery leg: the real committed root sidecar must be found. A nil here
	// (with no decode error) is precisely the silent root-resolution
	// regression this pin exists to catch — every synthetic VL-003 test would
	// still pass while this real-file proof quietly evaporated.
	if snap.RootBindings == nil {
		t.Fatalf("this repo's module-root verdi.bindings.yaml was not discovered into Snapshot.RootBindings (RootBindingsErr=%v) — root resolution has regressed", snap.RootBindingsErr)
	}

	in := &RunInput{Ctx: context.Background(), Root: root, Snapshot: snap, Opts: Options{}}
	r := vl003{}

	const sentinelSpec = "spec/zzz-sentinel-nonexistent-target"
	snap.RootBindings.Bindings = append(snap.RootBindings.Bindings, artifact.Binding{
		Producer: "sentinel-injected-producer",
		Kind:     artifact.EvidenceStatic,
		ACs:      []string{sentinelSpec + "#ac-1"},
	})

	withRoot := r.checkBindings(in)
	if !findingsMentionSentinel(withRoot, sentinelSpec) {
		t.Fatalf("with the sentinel injected into the REAL RootBindings, checkBindings did not red on %q — its root-discovery branch is not consuming Snapshot.RootBindings for this repo:\n%s", sentinelSpec, findingsString(withRoot))
	}

	// Consumption's control: withholding RootBindings must remove exactly the
	// sentinel finding — proving it entered only via the root-discovery path,
	// not some Service-scoped loop, so this pin genuinely guards that path.
	snap.RootBindings = nil
	snap.RootBindingsErr = nil
	if withoutRoot := r.checkBindings(in); findingsMentionSentinel(withoutRoot, sentinelSpec) {
		t.Fatalf("with RootBindings withheld, the sentinel %q still red — the finding is not gated on the root-discovery path, so this pin would not catch a root-resolution regression:\n%s", sentinelSpec, findingsString(withoutRoot))
	}
}
