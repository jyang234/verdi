package workbench

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/boardio"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
)

// obligationBoardName is a STORY-class draft on a design branch — the wall
// class on which a sticky graduates into an evidence obligation
// (spec/obligation-artifact ac-3). It declares two acceptance criteria (the
// obligation targets) and one decision (a non-AC card the negative path
// refuses to bind to).
const obligationBoardName = "refi-decline-audit"

const obligationBoardSpec = `---
id: spec/refi-decline-audit
kind: spec
class: story
title: "Refinancing decline audit"
status: draft
owners: [platform-team]
story: jira:LOAN-2202
problem: { text: "decline notices are not auditable after the fact", anchor: "#problem" }
outcome: { text: "every decline notice shown is reconstructable", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "support can replay every decline notice shown", evidence: [behavioral], anchor: "#ac-1" }
  - { id: ac-2, text: "the audit log is tamper-evident", evidence: [static], anchor: "#ac-2" }
decisions:
  - { id: dc-1, text: "reuse the outbox stream as the audit source", anchor: "#dc-1" }
links:
  - { type: implements, ref: spec/escrow-autopay#ac-1 }
---
# Refinancing decline audit

## Problem

## Outcome

## ac-1

Replayable decline notices.

## ac-2

Tamper-evident audit log.

## dc-1

Outbox as the audit source.
`

func newObligationBoardFixture(t *testing.T) string {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/specs/active/" + obligationBoardName + "/spec.md": obligationBoardSpec,
			".verdi/.gitignore": "data/\n",
		},
		Message: "seed obligation board fixture",
	}})
	if err := gitx.CheckoutNewBranch(context.Background(), repo.Dir, "design/"+obligationBoardName); err != nil {
		t.Fatalf("checkout design branch: %v", err)
	}
	return repo.Dir
}

// stickyIDOnBoard creates a comment sticky and returns its minted id.
func stickyIDOnBoard(t *testing.T, h http.Handler, root, name, text string) string {
	t.Helper()
	body, err := json.Marshal(map[string]string{"text": text, "type": "comment"})
	if err != nil {
		t.Fatal(err)
	}
	rec := postBoardAPI(t, h, name, "sticky", string(body))
	if rec.Code != http.StatusOK {
		t.Fatalf("create sticky = %d\n%s", rec.Code, rec.Body.String())
	}
	annotations, err := boardio.ReadAllAnnotations(boardio.AnnotationsDir(root))
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range annotations {
		if a.Body == text {
			return a.ID
		}
	}
	t.Fatalf("no sticky with body %q found among %+v", text, annotations)
	return ""
}

func TestBoardSpec_ObligationGraduate(t *testing.T) {
	root := newObligationBoardFixture(t)
	h := NewHandler(root)

	// A comment sticky on the story wall — its handwriting becomes the
	// obligation's prose (title + body).
	const prose = "the stale-decline retry is proven end to end"
	stickyID := stickyIDOnBoard(t, h, root, obligationBoardName, prose)

	// Graduate it onto ac-1 as behavioral evidence — the sticky "dropped on
	// a story AC" (ref) with its for_kind chosen (kind).
	rec := postBoardAPI(t, h, obligationBoardName, "sticky-graduate",
		`{"id":"`+stickyID+`","ref":"ac-1","kind":"obligation:behavioral"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("obligation graduate = %d\n%s", rec.Code, rec.Body.String())
	}
	var resp boardAPIResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if !resp.Dirty {
		t.Error("authoring an obligation did not dirty the tree (the new committed-zone file is untracked)")
	}

	// The obligation file lands at the DC-2 path and round-trips through the
	// single artifact seam with the frontmatter ac-3 requires.
	path := filepath.Join(root, ".verdi", "obligations", obligationBoardName, "ac-1--behavioral.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("obligation file not written at %s: %v", path, err)
	}
	fm, body, err := artifact.SplitFrontmatter(data)
	if err != nil {
		t.Fatalf("split obligation frontmatter: %v", err)
	}
	ob, err := artifact.DecodeObligation(fm)
	if err != nil {
		t.Fatalf("DecodeObligation: %v\n%s", err, data)
	}
	if ob.ID != "obligation/refi-decline-audit--ac-1--behavioral" {
		t.Errorf("id = %q, want obligation/refi-decline-audit--ac-1--behavioral", ob.ID)
	}
	if ob.ForKind != artifact.EvidenceBehavioral {
		t.Errorf("for_kind = %q, want behavioral", ob.ForKind)
	}
	if len(ob.Links) != 1 || ob.Links[0].Type != artifact.LinkVerifies || ob.Links[0].Ref != "spec/refi-decline-audit" {
		t.Errorf("links = %+v, want a single verifies edge → spec/refi-decline-audit (the WHOLE story spec, no fragment)", ob.Links)
	}
	if ob.Frozen == nil {
		t.Error("obligation carries no frozen stamp (obligations are frozen unconditionally)")
	}
	if !strings.Contains(string(body), prose) {
		t.Errorf("obligation body did not carry the sticky's handwriting: %q", body)
	}

	// The sticky flipped to graduated (the same GraduateStickies machinery
	// sticky-graduate uses) and no longer renders.
	annotations, _ := boardio.ReadAllAnnotations(boardio.AnnotationsDir(root))
	if len(annotations) != 1 || annotations[0].Status != artifact.AnnotationGraduated {
		t.Errorf("sticky not flipped to graduated: %+v", annotations)
	}
	if strings.Contains(getBoard(t, h, obligationBoardName).Body.String(), `data-testid="sticky-`+stickyID+`"`) {
		t.Error("a graduated sticky still renders on the board")
	}
}

// TestBoardSpec_ObligationGraduate_Negative pins every refusal path: none
// writes a malformed obligation, and each names the offending input.
func TestBoardSpec_ObligationGraduate_Negative(t *testing.T) {
	root := newObligationBoardFixture(t)
	h := NewHandler(root)
	stickyID := stickyIDOnBoard(t, h, root, obligationBoardName, "candidate obligation")

	cases := map[string]struct {
		body string
		want string
	}{
		"dropped on a non-AC (a decision) target": {
			`{"id":"` + stickyID + `","ref":"dc-1","kind":"obligation:behavioral"}`,
			"not a declared AC",
		},
		"dropped on an undeclared AC": {
			`{"id":"` + stickyID + `","ref":"ac-9","kind":"obligation:static"}`,
			"not a declared AC",
		},
		"unknown for_kind fails closed": {
			`{"id":"` + stickyID + `","ref":"ac-1","kind":"obligation:bogus"}`,
			"not a known evidence kind",
		},
		"a missing sticky": {
			`{"id":"a-01J8Z0K3AAAAAAAAAAAAAAAAAA","ref":"ac-1","kind":"obligation:static"}`,
			"no sticky",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rec := postBoardAPI(t, h, obligationBoardName, "sticky-graduate", tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("%s = %d, want 400\n%s", name, rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), tc.want) {
				t.Errorf("refusal %q did not name %q: %s", name, tc.want, rec.Body.String())
			}
		})
	}

	// No refusal ever wrote an obligation.
	if _, err := os.Stat(filepath.Join(root, ".verdi", "obligations")); !os.IsNotExist(err) {
		t.Errorf("a refused graduation left an obligations dir behind: %v", err)
	}
}

// TestBoardSpec_ObligationGraduate_FeatureWallRefused proves obligations are
// story-only: the same request on a FEATURE wall is refused, naming the rule
// (spec/obligation-artifact ac-2/DC-3; 03 §The feature fold).
func TestBoardSpec_ObligationGraduate_FeatureWallRefused(t *testing.T) {
	root := newBoardFixture(t) // boardFixtureName is class feature
	h := NewHandler(root)
	stickyID := stickyIDOnBoard(t, h, root, boardFixtureName, "misplaced obligation")

	rec := postBoardAPI(t, h, boardFixtureName, "sticky-graduate",
		`{"id":"`+stickyID+`","ref":"ac-1","kind":"obligation:behavioral"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("obligation graduate on a feature wall = %d, want 400\n%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "story-only") {
		t.Errorf("refusal did not name the story-only rule: %s", rec.Body.String())
	}
}

// TestWriteObligationFileUsesAtomicWrite is Task 1 of the
// extensibility-phase1 plan (audit CLEANUP-BEFORE #1): the sticky-graduate
// write path (writeObligationFile) had its own hand-rolled
// CreateTemp->Write->Close->Rename sequence with no fsync before the
// rename. This proves the fixed write leaves no temp sibling behind under
// .verdi/obligations/ and lands the exact prose requested, across more than
// one target AC/kind combination.
func TestWriteObligationFileUsesAtomicWrite(t *testing.T) {
	tests := []struct {
		name  string
		ref   string
		kind  string
		prose string
	}{
		{"behavioral evidence on ac-1", "ac-1", "obligation:behavioral", "the stale-decline retry is proven end to end [atomic-write-check]"},
		{"static evidence on ac-2", "ac-2", "obligation:static", "the audit log tamper check runs on every write [atomic-write-check]"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := newObligationBoardFixture(t)
			h := NewHandler(root)
			stickyID := stickyIDOnBoard(t, h, root, obligationBoardName, tc.prose)

			rec := postBoardAPI(t, h, obligationBoardName, "sticky-graduate",
				`{"id":"`+stickyID+`","ref":"`+tc.ref+`","kind":"`+tc.kind+`"}`)
			if rec.Code != http.StatusOK {
				t.Fatalf("obligation graduate(%s,%s) = %d\n%s", tc.ref, tc.kind, rec.Code, rec.Body.String())
			}

			dir := filepath.Join(root, ".verdi", "obligations", obligationBoardName)
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatalf("ReadDir(%s): %v", dir, err)
			}
			for _, e := range entries {
				if strings.Contains(e.Name(), ".tmp") {
					t.Fatalf("leftover temp file %s", e.Name())
				}
			}

			evidenceSuffix := strings.TrimPrefix(tc.kind, "obligation:")
			path := filepath.Join(dir, tc.ref+"--"+evidenceSuffix+".md")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("obligation file not written at %s: %v", path, err)
			}
			if !strings.Contains(string(data), tc.prose) {
				t.Fatalf("obligation file does not contain the sticky's prose %q:\n%s", tc.prose, data)
			}
		})
	}
}

// TestObligationAuthor_AtomicWrite_NoDirectCreateTemp is a source-text
// witness: obligationauthor.go must route writeObligationFile through
// atomicfile.Write, not its own private CreateTemp->Rename copy (the
// audit's CLEANUP-BEFORE #1 — this file's copy also lacked the fsync
// atomicfile.Write already fixed for boardio/boardlayout/boarddiagram).
func TestObligationAuthor_AtomicWrite_NoDirectCreateTemp(t *testing.T) {
	data, err := os.ReadFile("obligationauthor.go")
	if err != nil {
		t.Fatalf("reading obligationauthor.go: %v", err)
	}
	if strings.Contains(string(data), "os.CreateTemp") {
		t.Error("obligationauthor.go calls os.CreateTemp directly — writeObligationFile must route through internal/atomicfile.Write instead (CLEANUP-BEFORE #1)")
	}
}
