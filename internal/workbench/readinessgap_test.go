package workbench

// TestReadinessGap_WallVersusLoader is spec/readiness-recovery ac-5 /
// R-RR1-10: the wall shell (this package's own ASD derivation,
// deriveASDShell/buildASDView) is a deliberate lag behind the continuous
// readiness loader (dc-3 — "the wall shell keeps its derivation with a
// pinned gap witness"). It is not parity: it is a witness that the gap is
// KNOWN and EXPLICIT, so a change to either derivation that shrinks or
// grows the gap fails this test until the committed golden is
// deliberately updated.
//
// R-RR1-20 (fix round 1): the wall side derives with the design bridge
// wired (readinessGapCapsBridge, below) — the same posture `verdi serve`
// always establishes — rather than the bare, unwired boardSpecServer the
// original round used, which left DesignWired false and under-reported the
// wall's real concern vocabulary.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/readinessload"
)

// addClaimWallManifest commits a minimal .verdi/verdi.yaml onto the
// claim-wall fixture's already-checked-out design branch: newClaimWallFixture
// (boardspecasd_test.go) never writes one, because the wall shell's own
// loadBoard/loadASD path never calls store.Open — only readinessload.Load
// does (it opens the store to resolve the operating model). Adding the
// commit here, rather than editing the shared fixture, keeps this test's
// one extra requirement local to it.
func addClaimWallManifest(t *testing.T, root string) {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(root, ".verdi", "verdi.yaml")
	if err := os.WriteFile(path, []byte("schema: verdi.layout/v1\n"), 0o644); err != nil {
		t.Fatalf("writing verdi.yaml: %v", err)
	}
	if err := gitx.AddAll(ctx, root); err != nil {
		t.Fatalf("git add verdi.yaml: %v", err)
	}
	if _, err := gitx.CreateCommit(ctx, root, "fixture: add verdi.yaml for the readiness gap witness"); err != nil {
		t.Fatalf("committing verdi.yaml: %v", err)
	}
}

// family reduces one concern id (from either the wall shell's asdConcern.ID
// or the loader's readinesspilot.Concern.ID) to its family: the first two
// path segments, with any remaining tail collapsed to the literal "<id>"
// (e.g. "shape/question/oq-1" -> "shape/question/<id>",
// "success/blocker/x/y" -> "success/blocker/<id>"). shape/board/<kind>/<id>
// rows keep three segments instead of two — the item kind (question,
// agent-task) stays a distinguishing part of the family; only the
// trailing item id collapses ("shape/board/question/oq-2" ->
// "shape/board/question/<id>"). An id with no tail beyond the kept
// segments (e.g. "context/verdict", "shape/board" itself) is returned
// unchanged.
func family(id string) string {
	parts := strings.Split(id, "/")
	keep := 2
	if len(parts) >= 4 && parts[0] == "shape" && parts[1] == "board" {
		keep = 3
	}
	if len(parts) <= keep {
		return id
	}
	return strings.Join(parts[:keep], "/") + "/<id>"
}

// readinessGapCapsBridge is R-RR1-20's fixture bridge: the wall side must
// be derived with the design bridge wired (as `verdi serve` always wires
// it), never with the brief's original bare `&boardSpecServer{root: root}`
// literal, which left DesignWired false and under-reported the wall's real
// concern vocabulary (context/policy, context/agent-writes,
// context/draft-writes never fired — boardspecasd.go:307-337). The scripted
// outcome mirrors asdcorrection_test.go:131-137's Mutable:true posture, so
// the wall's context/agent-writes row (boardspecasd.go:318-323) fires.
func readinessGapCapsBridge() *scriptedCapsBridge {
	return &scriptedCapsBridge{script: func(int) (DesignReadOutcome, *DesignCapabilitiesView) {
		return DesignReadOutcome{JSON: []byte(`{}`)}, &DesignCapabilitiesView{Mutable: true, PolicyMode: "draft-write", PolicyDigest: "sha256:caps-gap"}
	}}
}

// renderReadinessGap renders the gap witness's exact committed text layout:
// a three-line header comment (R-RR1-20's third line disclosing the design
// bridge is wired for this derivation), then the wall-only, loader-only,
// and blocking-disagreement sections, each a sorted, two-space-indented
// list of families.
func renderReadinessGap(wallOnly, loaderOnly, disagreement []string) string {
	var b strings.Builder
	b.WriteString("# readiness gap — the wall shell versus the continuous derivation (spec/readiness-recovery ac-5)\n")
	b.WriteString("# This is a GAP LIST, not parity. The post-design workbench lane consumes it; a change here must be deliberate.\n")
	b.WriteString("# Derived on the claim-wall fixture with the design bridge wired (R-RR1-20); families neither side emits on that fixture are not listed.\n")
	b.WriteString("wall-only:\n")
	for _, f := range wallOnly {
		fmt.Fprintf(&b, "  %s\n", f)
	}
	b.WriteString("loader-only:\n")
	for _, f := range loaderOnly {
		fmt.Fprintf(&b, "  %s\n", f)
	}
	b.WriteString("blocking-disagreement:\n")
	for _, f := range disagreement {
		fmt.Fprintf(&b, "  %s\n", f)
	}
	return b.String()
}

func TestReadinessGap_WallVersusLoader(t *testing.T) {
	root := newClaimWallFixture(t)
	addClaimWallManifest(t, root)
	ctx := context.Background()

	_, _, asd, err := (&boardSpecServer{root: root, design: readinessGapCapsBridge()}).loadASD(ctx, claimWallName)
	if err != nil {
		t.Fatalf("loadASD: %v", err)
	}
	wallFamilies := map[string]bool{}
	wallBlocking := map[string]bool{}
	for _, c := range asd.Shell.All {
		f := family(c.ID)
		wallFamilies[f] = true
		if c.Blocking {
			wallBlocking[f] = true
		}
	}

	snap, err := readinessload.Load(ctx, root, "spec/"+claimWallName, readinessload.Options{})
	if err != nil {
		t.Fatalf("readinessload.Load: %v", err)
	}
	loaderFamilies := map[string]bool{}
	loaderBlocking := map[string]bool{}
	for _, c := range snap.AllConcerns {
		f := family(c.ID)
		loaderFamilies[f] = true
		if c.Blocking {
			loaderBlocking[f] = true
		}
	}

	var wallOnly, loaderOnly, disagreement []string
	for f := range wallFamilies {
		if !loaderFamilies[f] {
			wallOnly = append(wallOnly, f)
		}
	}
	for f := range loaderFamilies {
		if !wallFamilies[f] {
			loaderOnly = append(loaderOnly, f)
		}
	}
	for f := range wallFamilies {
		if loaderFamilies[f] && wallBlocking[f] != loaderBlocking[f] {
			disagreement = append(disagreement, f)
		}
	}
	sort.Strings(wallOnly)
	sort.Strings(loaderOnly)
	sort.Strings(disagreement)

	got := renderReadinessGap(wallOnly, loaderOnly, disagreement)

	goldenPath := filepath.Join("testdata", "readiness-gap.golden")
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("reading golden %s: %v", goldenPath, err)
	}
	if got != string(want) {
		t.Fatalf("readiness gap does not match the committed golden %s.\n--- got ---\n%s--- want ---\n%s", goldenPath, got, string(want))
	}
}
