package wallbadge

// Tests for the empty-evidence-slot compute (spec/evidence-slot ac-1/
// ac-2): "empty" is the real fold's per-kind no-record state (records
// through the fold's loader, the fold's per-AC filter, and the fold's
// Current reduction; attestation through AttestationExists), the
// no-derived-tree wall is the CALM ordinary authoring state, and every
// badge is a complete fold:empty-slot derivation record whose inputs pin
// the spec, the location probed, and the record files actually read —
// digests and commit shas only, never wall-clock time (co-1).

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
)

const slotStorySpec = `---
id: spec/slot-story
kind: spec
class: story
title: "Slot story"
status: draft
owners: [platform-team]
story: jira:SLOT-1
problem: { text: "p", anchor: "#problem" }
outcome: { text: "o", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "does a thing", evidence: [static, behavioral, attestation], anchor: "#ac-1" }
  - { id: ac-2, text: "does another thing", evidence: [behavioral], anchor: "#ac-2" }
links:
  - { type: implements, ref: "spec/some-parent#ac-1" }
---
# Slot story
`

func slotSpecFM(t *testing.T) *artifact.SpecFrontmatter {
	t.Helper()
	fmBytes, _, err := artifact.SplitFrontmatter([]byte(slotStorySpec))
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	fm, err := artifact.DecodeSpec(fmBytes)
	if err != nil {
		t.Fatalf("DecodeSpec: %v", err)
	}
	return fm
}

// newSlotRepo builds a one-layer fixturegit repo carrying slotStorySpec.
func newSlotRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/specs/active/slot-story/spec.md": slotStorySpec,
			".verdi/.gitignore":                      "data/\n",
		},
		Message: "seed slot story",
	}})
}

// slotVerdictsJSON is one static pass record bound to ac-1 at commit.
func slotVerdictsJSON(commit string) string {
	return `[{"schema":"verdi.evidence/v1","evidence_for":["ac-1"],"kind":"static","verdict":"pass",` +
		`"witness":"w","producer":"prod-a","provenance":{"source":"ci","pipeline":"1","commit":"` + commit + `"},` +
		`"digest":"sha256:ab12ab12ab12ab12ab12ab12ab12ab12ab12ab12ab12ab12ab12ab12ab12ab12"}]`
}

func writeSlotDerived(t *testing.T, root, commit, body string) string {
	t.Helper()
	dir := filepath.Join(root, ".verdi", "data", "derived", "spec--slot-story", commit)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir derived: %v", err)
	}
	path := filepath.Join(dir, "verdicts.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("writing verdicts.json: %v", err)
	}
	return path
}

func writeSlotAttestation(t *testing.T, root, acID string) {
	t.Helper()
	dir := filepath.Join(root, ".verdi", "attestations", "jira-slot-1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir attestations: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, acID+".md"), []byte("attested\n"), 0o644); err != nil {
		t.Fatalf("writing attestation: %v", err)
	}
}

func slotByKind(t *testing.T, states []SlotState, kind string) SlotState {
	t.Helper()
	for _, st := range states {
		if st.Kind == kind {
			return st
		}
	}
	t.Fatalf("no %s slot in %+v", kind, states)
	return SlotState{}
}

// TestEmptySlotBadges_FilledVersusEmpty is ac-1's core: a derived-tree
// record of a declared kind (static, bound to ac-1) fills exactly that
// kind's slot; the sibling declared kinds stay empty; an attestation
// file on disk fills the attestation slot; and a record bound to ac-1
// never leaks onto ac-2 (the fold's own per-AC filter). The badge's
// derivation record pins the spec digest, HEAD (the ancestry filter's
// reference), and the record file actually read with its exact-bytes
// sha256 (ac-2/dc-3).
func TestEmptySlotBadges_FilledVersusEmpty(t *testing.T) {
	repo := newSlotRepo(t)
	body := slotVerdictsJSON(repo.Head)
	writeSlotDerived(t, repo.Dir, repo.Head, body)
	writeSlotAttestation(t, repo.Dir, "ac-1")
	fm := slotSpecFM(t)

	slots, badges, err := EmptySlotBadges(context.Background(), repo.Dir, ".verdi/specs/active/slot-story/spec.md", "sha256:fefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefe", fm)
	if err != nil {
		t.Fatalf("EmptySlotBadges: %v", err)
	}

	ac1 := slots["ac-1"]
	if len(ac1) != 3 {
		t.Fatalf("ac-1 slots = %+v, want 3 (one per declared kind)", ac1)
	}
	if st := slotByKind(t, ac1, "static"); st.Empty || st.Records != 1 {
		t.Errorf("static slot = %+v, want held with 1 record", st)
	}
	if st := slotByKind(t, ac1, "behavioral"); !st.Empty || st.Records != 0 {
		t.Errorf("behavioral slot = %+v, want empty", st)
	}
	if st := slotByKind(t, ac1, "attestation"); st.Empty || st.Records != 1 {
		t.Errorf("attestation slot = %+v, want held (file on disk)", st)
	}
	// Declared order preserved: static, behavioral, attestation.
	if ac1[0].Kind != "static" || ac1[1].Kind != "behavioral" || ac1[2].Kind != "attestation" {
		t.Errorf("ac-1 slot order = %+v, want the AC's declared kind order", ac1)
	}

	// ac-2's behavioral slot stays empty: the ac-1-bound record must not
	// leak across ACs.
	ac2 := slots["ac-2"]
	if len(ac2) != 1 || !ac2[0].Empty {
		t.Fatalf("ac-2 slots = %+v, want one empty behavioral slot", ac2)
	}

	// Both ACs hold an empty slot, so both badge.
	if len(badges) != 2 {
		t.Fatalf("badges = %+v, want 2 (ac-1 and ac-2 each hold an empty slot)", badges)
	}
	byTarget := map[string]DerivationRecord{}
	for _, b := range badges {
		byTarget[b.Target] = b
	}
	b1 := byTarget["ac-1"]
	if b1.Source != "fold:empty-slot" || b1.Label != "empty slot" {
		t.Errorf("ac-1 badge = %+v, want source fold:empty-slot, label \"empty slot\" (one empty kind)", b1)
	}

	// Inputs: derived-tree pinned to HEAD, the record file read with the
	// sha256 of its exact bytes, and the caller's spec digest — sorted by
	// name, no timestamp anywhere (co-1).
	sum := sha256.Sum256([]byte(body))
	wantInputs := map[string][2]string{
		"derived-tree":                           {".verdi/data/derived/spec--slot-story", repo.Head},
		"record:" + repo.Head + "/verdicts.json": {".verdi/data/derived/spec--slot-story/" + repo.Head + "/verdicts.json", "sha256:" + hex.EncodeToString(sum[:])},
		"spec":                                   {".verdi/specs/active/slot-story/spec.md", "sha256:fefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefe"},
	}
	if len(b1.Inputs) != len(wantInputs) {
		t.Fatalf("ac-1 badge inputs = %+v, want %d entries", b1.Inputs, len(wantInputs))
	}
	for _, in := range b1.Inputs {
		want, ok := wantInputs[in.Name]
		if !ok {
			t.Errorf("unexpected input %+v", in)
			continue
		}
		if in.Path != want[0] || in.Revision != want[1] {
			t.Errorf("input %s = {Path: %q, Revision: %q}, want {%q, %q}", in.Name, in.Path, in.Revision, want[0], want[1])
		}
	}

	// Records disclose per-kind what was found or that nothing was
	// (ac-2), sorted (the package's deterministic-record contract).
	wantRecords := []string{
		"attestation: attestation file present",
		"behavioral: no current record",
		"static: 1 current record",
	}
	if len(b1.Records) != len(wantRecords) {
		t.Fatalf("ac-1 badge records = %+v, want %+v", b1.Records, wantRecords)
	}
	for i, want := range wantRecords {
		if b1.Records[i] != want {
			t.Errorf("ac-1 badge records[%d] = %q, want %q", i, b1.Records[i], want)
		}
	}
}

// TestEmptySlotBadges_NoDerivedTreeIsCalm is dc-1: a story wall with no
// derived tree at all — the ordinary design-branch authoring state —
// renders every declared kind as an empty slot with NO error and no git
// dependency, and the derivation record still names the location probed
// with the honest "absent" revision. The root here is deliberately NOT a
// git repository: the never-synced state must not even need one.
func TestEmptySlotBadges_NoDerivedTreeIsCalm(t *testing.T) {
	root := t.TempDir()
	fm := slotSpecFM(t)

	slots, badges, err := EmptySlotBadges(context.Background(), root, ".verdi/specs/active/slot-story/spec.md", "sha256:fefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefe", fm)
	if err != nil {
		t.Fatalf("EmptySlotBadges over a never-synced store: %v (want the calm empty state, not an error)", err)
	}
	for acID, states := range slots {
		for _, st := range states {
			if !st.Empty || st.Records != 0 {
				t.Errorf("%s %s slot = %+v, want empty (no derived tree)", acID, st.Kind, st)
			}
		}
	}
	if len(badges) != 2 {
		t.Fatalf("badges = %+v, want one per AC (both hold only empty slots)", badges)
	}
	for _, b := range badges {
		var tree *InputRecord
		for i := range b.Inputs {
			if b.Inputs[i].Name == "derived-tree" {
				tree = &b.Inputs[i]
			}
			if b.Inputs[i].Name != "derived-tree" && b.Inputs[i].Name != "spec" {
				t.Errorf("%s badge cites input %+v — nothing was read, so only the probe and the spec may be cited", b.Target, b.Inputs[i])
			}
		}
		if tree == nil {
			t.Fatalf("%s badge inputs = %+v, want the derived-tree location probed (dc-1: the receipt names what was looked at)", b.Target, b.Inputs)
		}
		if tree.Path != ".verdi/data/derived/spec--slot-story" || tree.Revision != "absent" {
			t.Errorf("derived-tree input = %+v, want the probed path with revision \"absent\"", *tree)
		}
	}
	// ac-1's badge counts all three empty kinds.
	for _, b := range badges {
		if b.Target == "ac-1" && b.Label != "3 empty slots" {
			t.Errorf("ac-1 badge label = %q, want \"3 empty slots\"", b.Label)
		}
	}
	// No wall-clock anywhere in the serialized record (co-1): every
	// revision is a digest, a commit sha, or the literal "absent".
	revShape := regexp.MustCompile(`^(sha256:[0-9a-f]{64}|[0-9a-f]{7,40}|absent)$`)
	for _, b := range badges {
		for _, in := range b.Inputs {
			if !revShape.MatchString(in.Revision) {
				t.Errorf("input %s revision %q is not a digest/sha/absent (co-1: never wall-clock)", in.Name, in.Revision)
			}
		}
	}
}

// TestEmptySlotBadges_Negative covers the fail-closed edges: a spec
// declaring no evidence kinds computes nothing (no slots exist, so
// nothing is empty); a derived tree that exists in a non-git checkout is
// an operational error (the fold's current set is undefined without
// ancestry), never a silent guess; and Current's latest-per-identity
// reduction is genuinely in the path (a same-producer retry counts once).
func TestEmptySlotBadges_Negative(t *testing.T) {
	t.Run("no declared kinds", func(t *testing.T) {
		fm := slotSpecFM(t)
		for i := range fm.AcceptanceCriteria {
			fm.AcceptanceCriteria[i].Evidence = nil
		}
		slots, badges, err := EmptySlotBadges(context.Background(), t.TempDir(), "spec.md", "sha256:fefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefe", fm)
		if err != nil {
			t.Fatalf("EmptySlotBadges: %v", err)
		}
		if slots != nil || badges != nil {
			t.Fatalf("slots = %+v, badges = %+v, want nil/nil (no kinds declared anywhere)", slots, badges)
		}
	})

	t.Run("derived tree without git is operational", func(t *testing.T) {
		root := t.TempDir()
		dir := filepath.Join(root, ".verdi", "data", "derived", "spec--slot-story")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if _, _, err := EmptySlotBadges(context.Background(), root, "spec.md", "sha256:fefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefe", slotSpecFM(t)); err == nil {
			t.Fatal("a present derived tree in a non-git root: want an operational error, got nil")
		}
	})

	t.Run("same-producer retry reduces to one record", func(t *testing.T) {
		repo := newSlotRepo(t)
		retry := `[` +
			`{"schema":"verdi.evidence/v1","evidence_for":["ac-2"],"kind":"behavioral","verdict":"fail",` +
			`"witness":"w","producer":"prod-b","provenance":{"source":"ci","pipeline":"1","commit":"` + repo.Head + `"},` +
			`"digest":"sha256:ab12ab12ab12ab12ab12ab12ab12ab12ab12ab12ab12ab12ab12ab12ab12ab12"},` +
			`{"schema":"verdi.evidence/v1","evidence_for":["ac-2"],"kind":"behavioral","verdict":"pass",` +
			`"witness":"w","producer":"prod-b","provenance":{"source":"ci","pipeline":"2","commit":"` + repo.Head + `"},` +
			`"digest":"sha256:ab12ab12ab12ab12ab12ab12ab12ab12ab12ab12ab12ab12ab12ab12ab12ab12"}]`
		writeSlotDerived(t, repo.Dir, repo.Head, retry)

		slots, badges, err := EmptySlotBadges(context.Background(), repo.Dir, "spec.md", "sha256:fefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefe", slotSpecFM(t))
		if err != nil {
			t.Fatalf("EmptySlotBadges: %v", err)
		}
		st := slotByKind(t, slots["ac-2"], "behavioral")
		if st.Empty || st.Records != 1 {
			t.Errorf("ac-2 behavioral slot = %+v, want held with 1 record (Current reduces a same-producer retry)", st)
		}
		// ac-2 is fully held: it must NOT badge. ac-1 still does.
		for _, b := range badges {
			if b.Target == "ac-2" {
				t.Errorf("ac-2 badged %+v despite every declared kind holding a record", b)
			}
		}
	})
}

// TestEmptySlotStaticCallSites is evidence-slot ac-1's STATIC evidence
// (co-3, dc-1): the slot's emptiness is computed from the evidence
// package's own seams — the fold's loader (LoadRecordsWithSources), the
// fold's per-AC filter (RecordsForAC), the fold's Current reduction, and
// AttestationExists — with no wall-local record parsing, no private
// latest-per-identity reduction, and no derived-tree walking of this
// package's own. The same deliberately-minimal source-text witness
// TestLadderStaticCallSites already established for this package.
func TestEmptySlotStaticCallSites(t *testing.T) {
	src, err := os.ReadFile("emptyslot.go")
	if err != nil {
		t.Fatalf("reading emptyslot.go: %v", err)
	}
	text := string(src)

	for _, want := range []string{
		"evidence.LoadRecordsWithSources(",
		"evidence.RecordsForAC(",
		"evidence.Current(",
		"evidence.AttestationExists(",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("emptyslot.go does not call %s — evidence-slot co-3 requires the fold's own seam, not a lookalike", want)
		}
	}

	// Negative: the markers of a wall-private record scan or reduction.
	// os.ReadDir/os.ReadFile/json.Unmarshal would be a second derived-tree
	// walker or record parser; "pipeline"/"laterProvenance" would be a
	// second latest-per-identity fold; "verdicts.json" would hardcode the
	// loader's own file layout here.
	for _, bad := range []string{"os.ReadDir", "os.ReadFile", "json.Unmarshal", "laterProvenance", "verdicts.json", "runtime.json"} {
		if strings.Contains(text, bad) {
			t.Errorf("emptyslot.go contains %q — a wall-private record scan/reduction marker (evidence-slot co-3)", bad)
		}
	}
}
