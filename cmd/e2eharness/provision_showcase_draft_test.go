package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

// A syntactically valid 40-hex the pinned refs in the fixtures can carry in
// unit tests — the decoders check ref/digest FORMAT, not git reality.
const testCommit40 = "78e3161594fb31fdad17f2ea8a96b52f33dbf0f3"

// TestShowcaseDraftSpecDecodes proves the draft feature spec strict-decodes
// and carries the shape the showcase bar requires: a feature draft with a
// tracker ref, both ACs declaring evidence, and oq-1 carrying the exact
// text VL-017's carried path matches against.
func TestShowcaseDraftSpecDecodes(t *testing.T) {
	front, _, err := artifact.SplitFrontmatter([]byte(showcaseDraftSpec))
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	fm, err := artifact.DecodeSpec(front)
	if err != nil {
		t.Fatalf("DecodeSpec: %v", err)
	}
	if fm.Class != artifact.ClassFeature {
		t.Errorf("class = %q, want feature", fm.Class)
	}
	if string(fm.Status) != "draft" {
		t.Errorf("status = %q, want draft", fm.Status)
	}
	if fm.Story == "" {
		t.Error("story tracker ref is missing")
	}
	if len(fm.AcceptanceCriteria) != 2 {
		t.Fatalf("acceptance criteria = %d, want 2", len(fm.AcceptanceCriteria))
	}
	for _, ac := range fm.AcceptanceCriteria {
		if len(ac.Evidence) == 0 {
			t.Errorf("ac %s declares no evidence kind (VL-006/VL-020)", ac.ID)
		}
	}
	if len(fm.OpenQuestions) != 1 || fm.OpenQuestions[0].Text != showcaseDraftOQCarried {
		t.Errorf("open_questions = %+v, want a single oq carrying %q", fm.OpenQuestions, showcaseDraftOQCarried)
	}
}

// TestShowcaseDraftSpecNegative proves a broken edition (a feature with no
// acceptance criteria) fails validation — the decoder is doing real work,
// not rubber-stamping.
func TestShowcaseDraftSpecNegative(t *testing.T) {
	broken := showcaseDraftSpec[:strings.Index(showcaseDraftSpec, "acceptance_criteria:")] +
		"---\n# Payoff quote portal\n"
	front, _, err := artifact.SplitFrontmatter([]byte(broken))
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	if _, err := artifact.DecodeSpec(front); err == nil {
		t.Fatal("DecodeSpec accepted a feature spec with no acceptance criteria")
	}
}

// TestShowcaseDraftDiagramDecodes proves the proposal diagram decodes as a
// class: proposal with a well-formed derived_from (VL-021's two format
// checks), and that its base ref parses and names a diagram.
func TestShowcaseDraftDiagramDecodes(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	sourceDigest := "sha256:" + strings.Repeat("b", 64)
	front, _, err := artifact.SplitFrontmatter([]byte(showcaseDraftDiagramDoc(testCommit40, digest, sourceDigest)))
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	d, err := artifact.DecodeDiagram(front)
	if err != nil {
		t.Fatalf("DecodeDiagram: %v", err)
	}
	if d.Class != artifact.DiagramClassProposal {
		t.Errorf("class = %q, want proposal", d.Class)
	}
	if d.DerivedFrom == nil {
		t.Fatal("derived_from is absent")
	}
	if !artifact.ValidDigest(d.DerivedFrom.Digest) {
		t.Errorf("digest %q is not sha256:<64-hex>", d.DerivedFrom.Digest)
	}
	if !artifact.ValidDigest(d.DerivedFrom.SourceDigest) {
		t.Errorf("source_digest %q is not sha256:<64-hex>", d.DerivedFrom.SourceDigest)
	}
	ref, err := artifact.ParseRef(d.DerivedFrom.Ref)
	if err != nil {
		t.Fatalf("derived_from.ref does not parse: %v", err)
	}
	if ref.Kind != artifact.KindDiagram || ref.Name != showcaseDraftBaseDiagram {
		t.Errorf("derived_from.ref = %s/%s, want diagram/%s", ref.Kind, ref.Name, showcaseDraftBaseDiagram)
	}
}

// TestShowcaseDraftAnnotationsDecode proves every seeded annotation decodes
// and that the stream carries VL-017's two twin fixtures — a resolved
// question and an open question whose body is the carried oq text — plus
// the agent-task working note.
func TestShowcaseDraftAnnotationsDecode(t *testing.T) {
	var resolved, carried, task int
	for _, line := range strings.Split(strings.TrimSpace(showcaseDraftAnnotations(testCommit40)), "\n") {
		a, err := artifact.DecodeAnnotation([]byte(line))
		if err != nil {
			t.Fatalf("DecodeAnnotation(%s): %v", line, err)
		}
		if a.Board == nil || a.Board.Story != showcaseDraftName {
			t.Errorf("annotation %s is not board-anchored to the draft wall", a.ID)
		}
		switch {
		case a.Type == artifact.AnnotationQuestion && a.Status == artifact.AnnotationResolved:
			resolved++
		case a.Type == artifact.AnnotationQuestion && a.Status == artifact.AnnotationOpen && a.Body == showcaseDraftOQCarried:
			carried++
		case a.Type == artifact.AnnotationAgentTask:
			task++
		}
	}
	if resolved != 1 || carried != 1 || task != 1 {
		t.Errorf("stream = {resolved:%d carried:%d task:%d}, want one of each", resolved, carried, task)
	}
}

// TestDiagramBodyBytes proves the frontmatter split (happy) and the
// no-fence passthrough (negative).
func TestDiagramBodyBytes(t *testing.T) {
	got := string(diagramBodyBytes([]byte("---\nid: diagram/x\nkind: diagram\n---\ngraph TD\n  a --> b\n")))
	if want := "graph TD\n  a --> b\n"; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
	raw := "graph TD\n  a --> b\n"
	if got := string(diagramBodyBytes([]byte(raw))); got != raw {
		t.Errorf("fenceless body = %q, want the input unchanged %q", got, raw)
	}
}

// newShowcaseDraftTestStore builds a real git store carrying the showcase
// committed zone on main plus the board suite's designBranch (the serving
// state provisionShowcaseDraft restores to), so the provisioner runs
// exactly as it does under the harness.
func newShowcaseDraftTestStore(t *testing.T) string {
	t.Helper()
	storeRoot := filepath.Join(t.TempDir(), "store")
	if err := copyTree(filepath.Join("..", "..", "examples", "showcase", ".verdi"), filepath.Join(storeRoot, ".verdi")); err != nil {
		t.Fatalf("copying showcase committed zone: %v", err)
	}
	if err := os.WriteFile(filepath.Join(storeRoot, ".verdi", ".gitignore"), []byte("data/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := gitInitAndCommit(storeRoot); err != nil {
		t.Fatalf("git init/commit: %v", err)
	}
	if err := runGit(storeRoot, nil, "config", "user.name", "verdi-e2e"); err != nil {
		t.Fatal(err)
	}
	if err := runGit(storeRoot, nil, "config", "user.email", "e2e@verdi.invalid"); err != nil {
		t.Fatal(err)
	}
	// The board suite's serving branch must exist for the provisioner's
	// closing checkout to land on it (provisionBoard cuts it in the harness).
	if err := runGit(storeRoot, nil, "checkout", "--quiet", "-b", designBranch); err != nil {
		t.Fatal(err)
	}
	return storeRoot
}

// TestProvisionShowcaseDraft_Happy proves the provisioner cuts the draft
// branch, restores the serving checkout, and pre-cuts + seeds the branch's
// managed worktree at wtmanager's deterministic path.
func TestProvisionShowcaseDraft_Happy(t *testing.T) {
	storeRoot := newShowcaseDraftTestStore(t)

	if err := provisionShowcaseDraft(storeRoot); err != nil {
		t.Fatalf("provisionShowcaseDraft: %v", err)
	}

	if err := runGit(storeRoot, nil, "rev-parse", "--verify", "refs/heads/"+showcaseDraftBranch); err != nil {
		t.Errorf("draft branch %s was not created: %v", showcaseDraftBranch, err)
	}
	head, err := gitOutput(storeRoot, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if head != designBranch {
		t.Errorf("serving checkout HEAD = %q, want restored to %q", head, designBranch)
	}

	worktree := filepath.Join(storeRoot, ".verdi", "data", "worktrees", showcaseDraftName)
	for _, rel := range []string{
		filepath.Join(".verdi", "specs", "active", showcaseDraftName, "spec.md"),
		filepath.Join(".verdi", "diagrams", showcaseDraftDiagram+".mermaid"),
		filepath.Join(".verdi", "data", "mutable", "annotations", "spec--"+showcaseDraftName+".jsonl"),
	} {
		if _, err := os.Stat(filepath.Join(worktree, rel)); err != nil {
			t.Errorf("worktree missing %s: %v", rel, err)
		}
	}

	// The spec committed on the branch decodes as the exemplary draft.
	spec, err := os.ReadFile(filepath.Join(worktree, ".verdi", "specs", "active", showcaseDraftName, "spec.md"))
	if err != nil {
		t.Fatal(err)
	}
	front, _, err := artifact.SplitFrontmatter(spec)
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	if _, err := artifact.DecodeSpec(front); err != nil {
		t.Errorf("provisioned spec does not decode: %v", err)
	}
}

// TestProvisionShowcaseDraft_Negative_NoRepo proves the provisioner fails
// loudly against a directory that is not a git repository.
func TestProvisionShowcaseDraft_Negative_NoRepo(t *testing.T) {
	if err := provisionShowcaseDraft(t.TempDir()); err == nil {
		t.Fatal("provisionShowcaseDraft over a non-repo: got nil error")
	}
}

// TestGitShowBytes proves the pinned-commit read is byte-exact — trailing
// newline included, the byte gitOutput's TrimSpace would eat and thereby
// corrupt a content digest — and that a path absent from the commit errors.
func TestGitShowBytes(t *testing.T) {
	dir := t.TempDir()
	const content = "graph TD\n  a --> b\n"
	if err := os.WriteFile(filepath.Join(dir, "d.mermaid"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := gitInitAndCommit(dir); err != nil {
		t.Fatal(err)
	}
	sha, err := gitOutput(dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	got, err := gitShowBytes(dir, sha, "d.mermaid")
	if err != nil {
		t.Fatalf("gitShowBytes: %v", err)
	}
	if string(got) != content {
		t.Errorf("gitShowBytes = %q, want byte-exact %q (trailing newline preserved)", got, content)
	}

	if _, err := gitShowBytes(dir, sha, "no-such-file"); err == nil {
		t.Fatal("gitShowBytes on an absent path: got nil error")
	}
}
