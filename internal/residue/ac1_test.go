package residue

import (
	"context"
	"os"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/store"
)

// TestScan_AC1_PatternA_RED is obligation/closure-hygiene--ac-1--behavioral's
// pattern-a fixture: an active-zone spec status: accepted-pending-build
// (widget), plus a local close/widget branch — unmerged — whose own tip
// commit moves .verdi/specs/active/widget/spec.md to
// .verdi/specs/archive/widget/spec.md (mirroring the real
// close/showcase-corpus-renovation shape the spec's own problem statement
// names). Asserts the scan's PatternA finding names the spec, the
// close/widget branch, and its tip sha, and that the run is FLAGGED.
func TestScan_AC1_PatternA_RED(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/.gitignore":                  "data/\n",
			".verdi/specs/active/widget/spec.md": storySpecMD("widget", "accepted-pending-build", "feature-x"),
		},
		Message: "seed the active, accepted-pending-build widget story",
	}})
	root := repo.Dir

	cutCloseBranch(t, root, "widget")
	wantTip := runCloseRitualArchiveCommit(t, root, "widget", "close: archive spec/widget (jira:VERDI-1)")
	checkoutMain(t, root)

	mainTip, err := gitx.RevParse(context.Background(), root, "main")
	if err != nil {
		t.Fatal(err)
	}

	got, err := Scan(context.Background(), root, "main")
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if !got.DefaultBranchResolved {
		t.Fatal("Scan.DefaultBranchResolved = false, want true")
	}
	if len(got.PatternA) != 1 {
		t.Fatalf("Scan.PatternA = %+v, want exactly 1 finding (widget)", got.PatternA)
	}
	pa := got.PatternA[0]
	if pa.SpecName != "widget" {
		t.Errorf("PatternA[0].SpecName = %q, want widget", pa.SpecName)
	}
	if pa.Branch != "close/widget" {
		t.Errorf("PatternA[0].Branch = %q, want close/widget", pa.Branch)
	}
	if pa.Tip != wantTip {
		t.Errorf("PatternA[0].Tip = %q, want %q (close/widget's own tip commit)", pa.Tip, wantTip)
	}
	if pa.Tip == mainTip {
		t.Error("PatternA[0].Tip equals main's tip; want the STRANDED branch's own, distinct tip")
	}
	if !got.Flagged() {
		t.Fatal("Scan.Flagged() = false, want true (AC-1 pattern (a) flags the run)")
	}
}

// TestScan_AC1_PatternB_RED is the pattern-b fixture: a class: feature
// status: accepted-pending-build spec declaring stubs[] whose every slug
// has a matching .verdi/specs/archive/<slug>/spec.md at status: closed.
// Asserts the scan's PatternB finding names the feature and its fully-
// realized stub set, and that THIS FIXTURE ALONE (no pattern-a instance
// present) leaves the run UNFLAGGED (dc-3).
func TestScan_AC1_PatternB_RED(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/.gitignore":                            "data/\n",
			".verdi/specs/archive/forge-transport/spec.md": closedArchiveStorySpecMD("forge-transport", "code-health"),
			".verdi/specs/archive/shared-homes/spec.md":    closedArchiveStorySpecMD("shared-homes", "code-health"),
			".verdi/specs/active/code-health/spec.md":      featureSpecMD("code-health", "accepted-pending-build", "forge-transport", "shared-homes"),
		},
		Message: "seed a stub-complete, unclosed feature (spec/code-health's own witness shape)",
	}})

	got, err := Scan(context.Background(), repo.Dir, "main")
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(got.PatternB) != 1 {
		t.Fatalf("Scan.PatternB = %+v, want exactly 1 finding (code-health)", got.PatternB)
	}
	pb := got.PatternB[0]
	if pb.SpecName != "code-health" {
		t.Errorf("PatternB[0].SpecName = %q, want code-health", pb.SpecName)
	}
	want := map[string]bool{"forge-transport": true, "shared-homes": true}
	if len(pb.Stubs) != 2 || !want[pb.Stubs[0]] || !want[pb.Stubs[1]] {
		t.Errorf("PatternB[0].Stubs = %v, want both forge-transport and shared-homes", pb.Stubs)
	}
	if len(got.PatternA) != 0 {
		t.Fatalf("Scan.PatternA = %+v, want empty (no pattern-a instance in this fixture)", got.PatternA)
	}
	if got.Flagged() {
		t.Fatal("Scan.Flagged() = true, want false (dc-3: pattern (b) alone never flags)")
	}
}

// TestScan_AC1_PatternB_RequiresMergedNotJustOnDisk is Defect 2's
// RED-direction witness: AC-1 pattern (b)'s "realized by a closed, MERGED
// story" must be evaluated against the audited default-branch tip, not the
// working tree. A stub whose archive move rides only an unmerged
// close/<slug> branch — the close-branch-checked-out shape spec/close-verb
// dc-3 leaves — is NOT realized by a merged story, even though its
// archive/<slug>/spec.md sits on disk at status: closed. Once that closure
// merges into the default branch, the stub IS realized and pattern (b)
// fires (never flagging, dc-3).
func TestScan_AC1_PatternB_RequiresMergedNotJustOnDisk(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/.gitignore":                       "data/\n",
			".verdi/specs/active/code-health/spec.md": featureSpecMD("code-health", "accepted-pending-build", "solo-stub"),
			".verdi/specs/active/solo-stub/spec.md":   storySpecMD("solo-stub", "accepted-pending-build", "code-health"),
		},
		Message: "a feature and its one still-open stub story, both active on main",
	}})
	root := repo.Dir
	ctx := context.Background()

	// Strand the stub's closure on an unmerged close/solo-stub branch, left
	// checked out: flip the story to closed and archive-move it (mirroring
	// the real ritual). archive/solo-stub/spec.md now sits on disk and on
	// this branch's own tip — but NOT on the default branch (main).
	cutCloseBranch(t, root, "solo-stub")
	if err := os.WriteFile(store.ActiveSpecPath(root, "solo-stub"), []byte(closedArchiveStorySpecMD("solo-stub", "code-health")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.ArchiveMove(root, "solo-stub"); err != nil {
		t.Fatalf("store.ArchiveMove(solo-stub): %v", err)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "--quiet", "-m", "close: archive spec/solo-stub (jira:RESIDUE-1)")

	// RED: with the archive riding only the unmerged branch, code-health's
	// stub is not realized by a MERGED story — pattern (b) must not fire.
	got, err := Scan(ctx, root, "main")
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(got.PatternB) != 0 {
		t.Fatalf("Scan.PatternB = %+v, want empty: solo-stub's archive rides only the unmerged close/solo-stub branch, so code-health is not stub-complete via a merged story (AC-1(b))", got.PatternB)
	}

	// GREEN: merge the closure into the default branch → the archive is now
	// present at the audited tip → the stub is realized → pattern (b) fires.
	checkoutMain(t, root)
	runGit(t, root, "merge", "--quiet", "--no-ff", "-m", "merge close/solo-stub", "close/solo-stub")

	got2, err := Scan(ctx, root, "main")
	if err != nil {
		t.Fatalf("Scan (after merge): %v", err)
	}
	if len(got2.PatternB) != 1 || got2.PatternB[0].SpecName != "code-health" {
		t.Fatalf("Scan.PatternB = %+v, want exactly 1 (code-health) once the archive is merged at the default tip", got2.PatternB)
	}
	if got2.Flagged() {
		t.Fatal("Scan.Flagged() = true, want false (dc-3: pattern (b) never flags, even when it fires)")
	}
}

// TestScan_AC1_GREEN is the GREEN-direction fixture: every active-zone
// spec's status consistent with git reality, including one status:
// superseded spec left in place, unarchived (dc-2). Asserts neither
// pattern fires and the run reports clean.
func TestScan_AC1_GREEN(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/.gitignore": "data/\n",
			// An ordinary in-flight story: nothing contradicts it.
			".verdi/specs/active/widget/spec.md": storySpecMD("widget", "accepted-pending-build", "feature-x"),
			// A feature with an UNREALIZED stub: pattern (b) must not fire.
			".verdi/specs/active/gadget/spec.md": featureSpecMD("gadget", "accepted-pending-build", "not-yet-closed-story"),
			// dc-2's own witness: status: superseded, left in specs/active/,
			// never archived — correct, permanent, never a finding.
			".verdi/specs/active/old-approach/spec.md": storySpecMD("old-approach", "superseded", "feature-x"),
		},
		Message: "every spec's status consistent with git reality",
	}})

	got, err := Scan(context.Background(), repo.Dir, "main")
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(got.PatternA) != 0 {
		t.Fatalf("Scan.PatternA = %+v, want empty (GREEN)", got.PatternA)
	}
	if len(got.PatternB) != 0 {
		t.Fatalf("Scan.PatternB = %+v, want empty (GREEN)", got.PatternB)
	}
	if got.Flagged() {
		t.Fatal("Scan.Flagged() = true, want false (GREEN)")
	}
}

// TestScan_AC1_DC2_SupersededNeverCheckedEvenWhenOtherwiseShaped proves
// dc-2's exclusion holds even when a superseded spec would OTHERWISE match
// pattern (a)'s or (b)'s conditions were the exclusion not applied first
// — a close/<name> branch archiving a superseded-named spec, and a
// superseded feature with every stub realized, must BOTH stay silent.
func TestScan_AC1_DC2_SupersededNeverCheckedEvenWhenOtherwiseShaped(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/.gitignore":                              "data/\n",
			".verdi/specs/active/superseded-story/spec.md":   storySpecMD("superseded-story", "superseded", "feature-x"),
			".verdi/specs/archive/realized-one/spec.md":      closedArchiveStorySpecMD("realized-one", "superseded-feature"),
			".verdi/specs/active/superseded-feature/spec.md": featureSpecMD("superseded-feature", "superseded", "realized-one"),
		},
		Message: "two superseded specs that would otherwise match pattern (a)/(b)",
	}})
	root := repo.Dir

	// superseded-story's own close/<name> branch, tip archived — would be
	// pattern (a)-shaped if status: superseded were not excluded first.
	cutCloseBranch(t, root, "superseded-story")
	runCloseRitualArchiveCommit(t, root, "superseded-story", "close: archive spec/superseded-story")
	checkoutMain(t, root)

	got, err := Scan(context.Background(), root, "main")
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(got.PatternA) != 0 {
		t.Fatalf("Scan.PatternA = %+v, want empty (dc-2: superseded-story's own status excludes it)", got.PatternA)
	}
	if len(got.PatternB) != 0 {
		t.Fatalf("Scan.PatternB = %+v, want empty (dc-2: superseded-feature's own status excludes it)", got.PatternB)
	}
	if got.Flagged() {
		t.Fatal("Scan.Flagged() = true, want false")
	}
}
