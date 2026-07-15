package dex

import (
	"context"
	"strings"
	"testing"
)

// TestBuild_SupersededTerminalStateIsLegible proves spec/feature-supersession-
// state ac-2 on the dex surface at BOTH rungs: a superseded spec's terminal
// `status` renders as its own status badge on the spec's own page (03 §rung 3:
// legible "without consulting backlinks"), and the predecessor additionally
// carries the computed `superseded-by` backlink to its successor.
//
// The fixtures live in examples/showcase (layers.txt layer 4, folded in
// from the former testdata/dexoverlay by Task 1.2): a superseded FEATURE
// predecessor `spec/rate-lock` superseded by
// `spec/rate-lock-v2`, and a superseded STORY predecessor `spec/escrow-notify`
// superseded by `spec/escrow-notify-v2` — the honest dc-4 scope, since verdi's
// own corpus has no superseded feature. The badge itself was latent (the
// `.badge-superseded` CSS and the `badge badge-<status>` template already
// existed); this test is the proof the AC demands that it actually renders for
// a spec at each rung, not merely that the code exists.
func TestBuild_SupersededTerminalStateIsLegible(t *testing.T) {
	repo := buildDexFixtureRepo(t)
	outDir := t.TempDir()

	if err := Build(context.Background(), Options{Root: repo.Dir, OutDir: outDir}); err != nil {
		t.Fatalf("Build: %v", err)
	}

	// The status badge is legible from the spec's OWN rendered status (co-3),
	// at both rungs, without any backlink lookup.
	t.Run("superseded story renders its status badge", func(t *testing.T) {
		page := readFile(t, outDir, "a/spec/escrow-notify/index.html")
		if !strings.Contains(page, `class="badge badge-superseded"`) {
			t.Fatalf("superseded story page missing the badge-superseded status badge; got:\n%s", page)
		}
		if !strings.Contains(page, `badge-superseded">superseded</span>`) {
			t.Fatalf("superseded story badge does not read \"superseded\"; got:\n%s", page)
		}
	})

	t.Run("superseded feature renders its status badge", func(t *testing.T) {
		page := readFile(t, outDir, "a/spec/rate-lock/index.html")
		if !strings.Contains(page, `class="badge badge-superseded"`) {
			t.Fatalf("superseded feature page missing the badge-superseded status badge; got:\n%s", page)
		}
		if !strings.Contains(page, `badge-superseded">superseded</span>`) {
			t.Fatalf("superseded feature badge does not read \"superseded\"; got:\n%s", page)
		}
	})

	// The predecessor's page ALSO carries the computed superseded-by backlink
	// to its successor — the relation the AC says must never be the ONLY way to
	// find the terminal state, proven present here alongside the status badge.
	t.Run("superseded feature page carries the superseded-by backlink", func(t *testing.T) {
		page := readFile(t, outDir, "a/spec/rate-lock/index.html")
		if !strings.Contains(page, "superseded-by") {
			t.Fatalf("superseded feature page missing a superseded-by backlink; got:\n%s", page)
		}
		if !strings.Contains(page, "spec/rate-lock-v2") {
			t.Fatalf("superseded feature page missing the backlink source spec/rate-lock-v2; got:\n%s", page)
		}
	})

	t.Run("superseded story page carries the superseded-by backlink", func(t *testing.T) {
		page := readFile(t, outDir, "a/spec/escrow-notify/index.html")
		if !strings.Contains(page, "superseded-by") {
			t.Fatalf("superseded story page missing a superseded-by backlink; got:\n%s", page)
		}
		if !strings.Contains(page, "spec/escrow-notify-v2") {
			t.Fatalf("superseded story page missing the backlink source spec/escrow-notify-v2; got:\n%s", page)
		}
	})
}
