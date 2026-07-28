package evidence

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
)

// TestRecords_ShallowBeyondHorizon_KeptButDisclosed is the P2-10b evidence-
// loader pin: in a SHALLOW checkout, an evidence record whose provenance.commit
// is a real ancestor that sits BEYOND the horizon (its object was never
// fetched) must be KEPT in the authoritative fold — never silently excluded,
// because exclusion requires PROOF of unreachability and a shallow checkout has
// none (asymmetric honesty: shallow proves YES, never NO) — but DISCLOSED via
// UnprovableRecords so the closure gate can name that the evidence's ancestry
// is unprovable. A within-horizon record in the SAME clone is proven reachable
// and stays silent (not disclosed). Neither is ever quarantined or listed as an
// excluded/stale commit dir.
func TestRecords_ShallowBeyondHorizon_KeptButDisclosed(t *testing.T) {
	ctx := context.Background()
	src := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"a.txt": "one\n"}, Message: "layer 1"},
		{Files: map[string]string{"a.txt": "two\n"}, Message: "layer 2"},
		{Files: map[string]string{"a.txt": "three\n"}, Message: "layer 3"},
	})
	beyond := src.Heads[0] // L1 — beyond a depth-2 horizon
	within := src.Heads[1] // L2 — within the horizon, a real ancestor of the tip
	head := src.Head       // L3 — equals the shallow clone's HEAD

	clone := fixturegit.ShallowClone(t, src, 2)

	// Guard: confirm the environment genuinely excluded L1 (git clone --depth
	// semantics), so this test exercises a real horizon rather than a full copy.
	if r, err := gitx.ReachableFromHEAD(ctx, clone, beyond, head); err != nil {
		t.Fatalf("ReachableFromHEAD(beyond, shallow): %v", err)
	} else if r != gitx.UnprovableShallow {
		t.Skipf("shallow clone did not exclude L1 (git --depth semantics); beyond=%s reachability=%v", beyond, r)
	}

	derivedRoot := filepath.Join(clone, "derived", "spec--test")
	writeDerivedVerdicts(t, derivedRoot, beyond, recordJSON(beyond, "ci"))
	writeDerivedVerdicts(t, derivedRoot, within, recordJSON(within, "ci"))

	// Loader KEEPS both — within-horizon proven, beyond-horizon unprovable but
	// never dropped.
	got, err := LoadRecords(ctx, clone, derivedRoot, head)
	if err != nil {
		t.Fatalf("LoadRecords(shallow): %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("LoadRecords(shallow) = %d records, want 2 (both kept: proven within-horizon + unprovable-not-excluded beyond-horizon)", len(got))
	}

	// The beyond-horizon record is DISCLOSED (not silently counted); the
	// within-horizon one is proven and stays silent.
	unprov, err := UnprovableRecords(ctx, clone, derivedRoot, head)
	if err != nil {
		t.Fatalf("UnprovableRecords(shallow): %v", err)
	}
	if len(unprov) != 1 {
		t.Fatalf("UnprovableRecords(shallow) = %d, want exactly 1 (only the beyond-horizon record; the within-horizon one is proven)", len(unprov))
	}
	if unprov[0].Provenance.Commit != beyond {
		t.Fatalf("UnprovableRecords()[0].Provenance.Commit = %q, want the beyond-horizon commit %q", unprov[0].Provenance.Commit, beyond)
	}

	// It is NOT quarantined (kept, authoritative) and its dir is NOT listed as
	// excluded/stale — exclusion requires proof a shallow horizon lacks.
	quar, _, err := QuarantinedRecords(ctx, clone, derivedRoot, head)
	if err != nil {
		t.Fatalf("QuarantinedRecords(shallow): %v", err)
	}
	if len(quar) != 0 {
		t.Fatalf("QuarantinedRecords(shallow) = %d, want 0 (an unprovable record is kept, never quarantined)", len(quar))
	}
	excluded, err := ExcludedCommitDirs(ctx, clone, derivedRoot, head)
	if err != nil {
		t.Fatalf("ExcludedCommitDirs(shallow): %v", err)
	}
	if len(excluded) != 0 {
		t.Fatalf("ExcludedCommitDirs(shallow) = %v, want empty (a shallow-unprovable dir is not proven excluded)", excluded)
	}
}
