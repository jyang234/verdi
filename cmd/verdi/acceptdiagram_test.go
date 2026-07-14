package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
)

// diagramBodyIdiosyncratic is spec/proposal-artifact obligation
// ac-2--behavioral's required shape: trailing spaces, mixed (space/tab)
// indentation, a `%%` mermaid comment, and a non-final blank line.
const diagramBodyIdiosyncratic = "graph TD\n" +
	"  loansvc --> notification-svc   \n" +
	"\tcharge-svc --> loansvc\n" +
	"\n" +
	"%% a note about future work\n" +
	"  end\n"

// buildAcceptDiagramRepo builds a one-layer fixturegit repo carrying a
// single class: proposal diagram at .verdi/diagrams/<name>.mermaid, whose
// frontmatter is fmText (no leading/trailing "---" delimiters — this
// helper adds them) and whose mermaid body is diagramBodyIdiosyncratic.
func buildAcceptDiagramRepo(t *testing.T, name, fmText string) *fixturegit.Repo {
	t.Helper()
	raw := "---\n" + fmText + "---\n" + diagramBodyIdiosyncratic
	return fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/diagrams/" + name + ".mermaid": raw,
			},
			Message: "init store: one proposal diagram",
		},
	})
}

// readDiagram strict-decodes the diagram at .verdi/diagrams/<name>.mermaid
// under root and also returns SplitFrontmatter's own body slice, failing
// the test on any read/split/decode error.
func readDiagram(t *testing.T, root, name string) (*artifact.DiagramFrontmatter, []byte) {
	t.Helper()
	path := filepath.Join(root, ".verdi", "diagrams", name+".mermaid")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	fm, body, err := artifact.SplitFrontmatter(raw)
	if err != nil {
		t.Fatalf("splitting frontmatter of %s: %v", path, err)
	}
	diag, err := artifact.DecodeDiagram(fm)
	if err != nil {
		t.Fatalf("decoding diagram %s: %v", path, err)
	}
	return diag, body
}

const proposedDiagramFM = `id: diagram/loansvc-target-topology
kind: diagram
title: "LoanSvc target topology"
class: proposal
status: proposed
owners: [platform-team]
`

// TestRunAccept_Diagram_Happy is spec/proposal-artifact obligation
// ac-3--behavioral case (1): accepts a class: proposal, status: proposed
// diagram, asserting the file on disk now reads status: accepted and
// carries a frozen: {at, commit} stamp with commit == HEAD's sha at
// acceptance time.
func TestRunAccept_Diagram_Happy(t *testing.T) {
	repo := buildAcceptDiagramRepo(t, "loansvc-target-topology", proposedDiagramFM)
	ctx := context.Background()

	preFlipHead, err := gitx.RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatalf("RevParse(HEAD): %v", err)
	}

	var stdout, stderr bytes.Buffer
	got := runAccept(ctx, repo.Dir, "diagram/loansvc-target-topology", &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runAccept = %d, want 0; stderr=%s", got, stderr.String())
	}

	diag, _ := readDiagram(t, repo.Dir, "loansvc-target-topology")
	if diag.Status != "accepted" {
		t.Fatalf("diag.Status = %q, want accepted", diag.Status)
	}
	if diag.Frozen == nil {
		t.Fatal("diag.Frozen is nil, want a frozen stamp")
	}
	if diag.Frozen.Commit != preFlipHead {
		t.Fatalf("diag.Frozen.Commit = %q, want the pre-flip HEAD %q", diag.Frozen.Commit, preFlipHead)
	}

	newHead, err := gitx.RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatalf("RevParse(HEAD): %v", err)
	}
	if newHead == preFlipHead {
		t.Fatal("accept did not create a new commit for the flip")
	}
}

// TestRunAccept_Diagram_ByteIdentityRegression is spec/proposal-artifact
// obligation ac-2--behavioral: the diagram's mermaid body must be
// byte-identical (SHA-256 compared) before and after the accept ritual's
// frontmatter-only status flip — the one real write path this story
// builds that persists a diagram file. A normalized/whitespace-insensitive
// comparison would not satisfy this; this test compares raw SHA-256 sums.
func TestRunAccept_Diagram_ByteIdentityRegression(t *testing.T) {
	repo := buildAcceptDiagramRepo(t, "loansvc-target-topology", proposedDiagramFM)
	ctx := context.Background()

	_, bodyBefore := readDiagram(t, repo.Dir, "loansvc-target-topology")
	shaBefore := sha256.Sum256(bodyBefore)

	var stdout, stderr bytes.Buffer
	if got := runAccept(ctx, repo.Dir, "diagram/loansvc-target-topology", &stdout, &stderr); got != 0 {
		t.Fatalf("runAccept = %d, want 0; stderr=%s", got, stderr.String())
	}

	_, bodyAfter := readDiagram(t, repo.Dir, "loansvc-target-topology")
	shaAfter := sha256.Sum256(bodyAfter)

	if shaBefore != shaAfter {
		t.Fatalf("body SHA-256 changed across the accept ritual's frontmatter-only edit:\nbefore: %x\nafter:  %x\nbefore body: %q\nafter body:  %q", shaBefore, shaAfter, bodyBefore, bodyAfter)
	}
	if !bytes.Equal(bodyBefore, []byte(diagramBodyIdiosyncratic)) {
		t.Fatalf("fixture body was not read back as written before any edit — test fixture itself is broken:\ngot:  %q\nwant: %q", bodyBefore, diagramBodyIdiosyncratic)
	}
}

// TestRunAccept_Diagram_RefusesIncumbent is spec/proposal-artifact
// obligation ac-3--behavioral case (2): refuses (naming the target and
// reason, non-zero exit) an accept attempt against an incumbent diagram
// (class absent).
func TestRunAccept_Diagram_RefusesIncumbent(t *testing.T) {
	incumbentFM := `id: diagram/loansvc-topology
kind: diagram
title: "LoanSvc topology"
status: active
owners: [platform-team]
`
	repo := buildAcceptDiagramRepo(t, "loansvc-topology", incumbentFM)
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	got := runAccept(ctx, repo.Dir, "diagram/loansvc-topology", &stdout, &stderr)
	if got == 0 {
		t.Fatalf("runAccept = 0, want non-zero for an incumbent diagram; stdout=%s", stdout.String())
	}
	msg := stderr.String()
	if !strings.Contains(msg, "diagram/loansvc-topology") {
		t.Errorf("refusal does not name the target: %s", msg)
	}
	if !strings.Contains(msg, "not a class: proposal diagram") && !strings.Contains(msg, "incumbent") {
		t.Errorf("refusal does not name the reason (incumbent, no class): %s", msg)
	}
}

// TestRunAccept_Diagram_RefusesAlreadyAccepted is spec/proposal-artifact
// obligation ac-3--behavioral case (3): refuses an accept attempt against
// a class: proposal diagram already status: accepted.
func TestRunAccept_Diagram_RefusesAlreadyAccepted(t *testing.T) {
	acceptedFM := `id: diagram/loansvc-target-topology
kind: diagram
title: "LoanSvc target topology"
class: proposal
status: accepted
owners: [platform-team]
frozen: { at: 2026-07-01, commit: 3e91ab2 }
`
	repo := buildAcceptDiagramRepo(t, "loansvc-target-topology", acceptedFM)
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	got := runAccept(ctx, repo.Dir, "diagram/loansvc-target-topology", &stdout, &stderr)
	if got == 0 {
		t.Fatalf("runAccept = 0, want non-zero for an already-accepted proposal; stdout=%s", stdout.String())
	}
	msg := stderr.String()
	if !strings.Contains(msg, "diagram/loansvc-target-topology") {
		t.Errorf("refusal does not name the target: %s", msg)
	}
	if !strings.Contains(msg, "accepted") {
		t.Errorf("refusal does not name the reason (status already accepted): %s", msg)
	}
}

// TestRunAccept_Diagram_RefusesUnresolvedTarget is spec/proposal-artifact
// obligation ac-3--behavioral case (4): refuses an accept attempt against
// a ref that does not resolve to any diagram at all.
func TestRunAccept_Diagram_RefusesUnresolvedTarget(t *testing.T) {
	repo := buildAcceptDiagramRepo(t, "loansvc-target-topology", proposedDiagramFM)
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	got := runAccept(ctx, repo.Dir, "diagram/does-not-exist", &stdout, &stderr)
	if got == 0 {
		t.Fatalf("runAccept = 0, want non-zero for an unresolved diagram ref; stdout=%s", stdout.String())
	}
	msg := stderr.String()
	if !strings.Contains(msg, "diagram/does-not-exist") {
		t.Errorf("refusal does not name the target: %s", msg)
	}
}

// TestRunAccept_Diagram_RefusesNonDiagramNonSpecRef proves the top-level
// accept dispatch (accept.go) still refuses a ref that is neither a spec
// nor a diagram ref (e.g. an adr/... ref), naming the reason.
func TestRunAccept_Diagram_RefusesNonDiagramNonSpecRef(t *testing.T) {
	repo := buildAcceptDiagramRepo(t, "loansvc-target-topology", proposedDiagramFM)
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	got := runAccept(ctx, repo.Dir, "adr/0001-outbox-events", &stdout, &stderr)
	if got == 0 {
		t.Fatalf("runAccept = 0, want non-zero for a non-spec, non-diagram ref; stdout=%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "not a spec or diagram ref") {
		t.Errorf("refusal does not name the reason: %s", stderr.String())
	}
}
