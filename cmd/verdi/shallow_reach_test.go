package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
	forgepkg "github.com/jyang234/verdi/internal/forge"
	"github.com/jyang234/verdi/internal/gitx"
)

// TestQuarantineUnreachable_ShallowBeyondHorizon_NotQuarantined is the P2-10b
// sync-time pin: a fetched record whose provenance.commit is a real ancestor
// sitting BEYOND a shallow horizon is left UN-quarantined — kept byte-for-byte,
// never annotated — because shallow history cannot prove unreachability. It is
// counted separately for sync's own disclosure notice. (Contrast the full-clone
// TestRunSync_CIFetch_QuarantinesUnreachableCommitRecord: a PROVEN-unreachable
// commit is still quarantined — X-15 holds.)
func TestQuarantineUnreachable_ShallowBeyondHorizon_NotQuarantined(t *testing.T) {
	ctx := context.Background()
	src := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"a.txt": "one\n"}, Message: "layer 1"},
		{Files: map[string]string{"a.txt": "two\n"}, Message: "layer 2"},
		{Files: map[string]string{"a.txt": "three\n"}, Message: "layer 3"},
	})
	beyond := src.Heads[0]
	head := src.Head
	clone := fixturegit.ShallowClone(t, src, 2)

	// Guard: confirm the environment genuinely excluded L1.
	if r, err := gitx.ReachableFromHEAD(ctx, clone, beyond, head); err != nil {
		t.Fatalf("ReachableFromHEAD(beyond, shallow): %v", err)
	} else if r != gitx.UnprovableShallow {
		t.Skipf("shallow clone did not exclude L1 (git --depth semantics); beyond=%s reachability=%v", beyond, r)
	}

	key := "spec--x/" + beyond + "/verdicts.json"
	original := []byte(quarantineTestRecord(beyond))
	tree := forgepkg.DerivedTree{key: append([]byte(nil), original...)}

	quarantined, unprovable, undecodable, err := quarantineUnreachable(ctx, clone, tree, head)
	if err != nil {
		t.Fatalf("quarantineUnreachable(shallow): %v", err)
	}
	if quarantined != 0 {
		t.Fatalf("quarantined = %d, want 0 (a shallow horizon cannot prove unreachability — honest evidence is never quarantined)", quarantined)
	}
	if unprovable != 1 {
		t.Fatalf("unprovableShallow = %d, want 1 (the beyond-horizon record, counted only for disclosure)", unprovable)
	}
	if len(undecodable) != 0 {
		t.Fatalf("undecodable = %v, want none", undecodable)
	}
	if !bytes.Equal(tree[key], original) {
		t.Fatalf("record file was rewritten under a shallow horizon; want byte-for-byte unchanged (never annotated):\n got: %s\nwant: %s", tree[key], original)
	}
}

// TestUnprovableDisclosures_NamesACCommitAndShallow pins the closure gate's
// P2-10b disclosure text: one disclosed-unproven line per (kept-but-unprovable
// record, evidenced AC), naming the AC, the commit, and the shallow reason,
// rendered through the shared disclosure seam.
func TestUnprovableDisclosures_NamesACCommitAndShallow(t *testing.T) {
	const commit = "cafebabecafebabecafebabecafebabecafebabe"
	recs := []artifact.Evidence{{
		Kind:        artifact.EvidenceStatic,
		Witness:     "w",
		EvidenceFor: []string{"ac-1", "ac-2"},
		Provenance:  artifact.EvidenceProvenance{Source: artifact.SourceCI, Commit: commit},
	}}

	lines := unprovableDisclosures(recs)
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2 (one per evidenced AC):\n%s", len(lines), strings.Join(lines, "\n"))
	}
	for _, l := range lines {
		if !strings.HasPrefix(l, "disclosed-unproven [gate:evidence-unprovable] ") {
			t.Errorf("line = %q, want a \"disclosed-unproven [gate:evidence-unprovable] ...\" seam line", l)
		}
		if !strings.Contains(l, commit) || !strings.Contains(l, "shallow history cannot prove reachability") {
			t.Errorf("line = %q, want it to name the commit and the shallow reason", l)
		}
	}
	if got := unprovableDisclosures(nil); got != nil {
		t.Errorf("unprovableDisclosures(nil) = %v, want nil (no records, no lines)", got)
	}
}
