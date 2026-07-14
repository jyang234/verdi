package align

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/upstream"
)

// hex64Diagram is a syntactically well-formed sha256 digest body (64 lowercase
// hex chars) — this test file's own copy of the constant every package in
// this module re-declares locally rather than exporting (internal/artifact's
// common_test.go has its own hex64; there is no shared test-only export).
const hex64Diagram = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcd"

// writeDiagramFixture writes root/.verdi/diagrams/<name>.mermaid with the
// given frontmatter (already terminated by its own trailing newline) and
// mermaid body.
func writeDiagramFixture(t *testing.T, root, name, frontmatter, body string) {
	t.Helper()
	dir := filepath.Join(root, ".verdi", "diagrams")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	content := "---\n" + frontmatter + "---\n" + body
	path := filepath.Join(dir, name+".mermaid")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

// writeSpecFixture writes root/.verdi/specs/active/<name>/spec.md with the
// given frontmatter and body.
func writeSpecFixture(t *testing.T, root, name, frontmatter, body string) {
	t.Helper()
	dir := filepath.Join(root, ".verdi", "specs", "active", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	content := "---\n" + frontmatter + "---\n" + body
	path := filepath.Join(dir, "spec.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

func diagramFrontmatterYAML(id, status string) string {
	return "id: " + id + "\nkind: diagram\ntitle: T\nclass: proposal\nstatus: " + status +
		"\nowners: [platform-team]\n" +
		"frozen: { at: 2026-07-14, commit: 3e91ab2 }\n"
}

// TestDiscoverAcceptedProposals_CorpusWideNoLeakage is obligation
// ac-1--behavioral's discovery half: two accepted proposals and one
// status: proposed (not yet accepted) diagram in the corpus — discovery
// returns exactly the two accepted ones, corpus-wide, and never the
// not-yet-accepted one (dc-1: no ownership filter of any kind).
func TestDiscoverAcceptedProposals_CorpusWideNoLeakage(t *testing.T) {
	root := t.TempDir()
	writeDiagramFixture(t, root, "loan-flow-a",
		diagramFrontmatterYAML("diagram/loan-flow-a", "accepted"),
		"flowchart LR\n    Alpha[\"Alpha\"]\n")
	writeDiagramFixture(t, root, "loan-flow-b",
		diagramFrontmatterYAML("diagram/loan-flow-b", "accepted"),
		"flowchart LR\n    Beta[\"Beta\"]\n")
	writeDiagramFixture(t, root, "loan-flow-c-not-yet-accepted",
		"id: diagram/loan-flow-c-not-yet-accepted\nkind: diagram\ntitle: T\nclass: proposal\nstatus: proposed\nowners: [platform-team]\n",
		"flowchart LR\n    Gamma[\"Gamma\"]\n")

	got, err := DiscoverAcceptedProposals(root)
	if err != nil {
		t.Fatalf("DiscoverAcceptedProposals: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("DiscoverAcceptedProposals = %+v, want exactly 2 accepted proposals", got)
	}
	names := map[string]bool{}
	for _, p := range got {
		names[p.Name] = true
	}
	if !names["loan-flow-a"] || !names["loan-flow-b"] {
		t.Fatalf("names = %v, want loan-flow-a and loan-flow-b", names)
	}
	if names["loan-flow-c-not-yet-accepted"] {
		t.Fatalf("names = %v, want the proposed (not accepted) diagram excluded", names)
	}
}

// TestDiscoverAcceptedProposals_EmptyCorpus_ExplicitEmpty proves an absent
// diagrams/ directory (no diagram ever authored) returns a disclosed empty
// slice rather than an error or a nil masking a read failure.
func TestDiscoverAcceptedProposals_EmptyCorpus_ExplicitEmpty(t *testing.T) {
	root := t.TempDir()
	got, err := DiscoverAcceptedProposals(root)
	if err != nil {
		t.Fatalf("DiscoverAcceptedProposals: %v", err)
	}
	if got == nil {
		t.Fatal("DiscoverAcceptedProposals returned nil, want an explicit empty slice")
	}
	if len(got) != 0 {
		t.Fatalf("DiscoverAcceptedProposals = %+v, want empty", got)
	}
}

// TestDiscoverIllustrativeFigures_ScopedToCurrentSpecOnly is obligation
// ac-1--behavioral's illustrative half: a spec body with one fenced
// mermaid block and a SECOND, unrelated spec's body also carrying a fenced
// block — discovery for the spec under test (via readCurrentSpecBody, the
// function that supplies DiscoverIllustrativeFigures its input) returns
// only its own body's figure, never the other spec's.
func TestDiscoverIllustrativeFigures_ScopedToCurrentSpecOnly(t *testing.T) {
	root := t.TempDir()
	specAFrontmatter := "id: spec/spec-a\nkind: spec\ntitle: A\nclass: component\nstatus: draft\nowners: [platform-team]\n"
	specABody := "# Spec A\n\nSome prose.\n\n```mermaid\nflowchart LR\n    X --> Y\n```\n\nMore prose.\n"
	writeSpecFixture(t, root, "spec-a", specAFrontmatter, specABody)

	specBFrontmatter := "id: spec/spec-b\nkind: spec\ntitle: B\nclass: component\nstatus: draft\nowners: [platform-team]\n"
	specBBody := "# Spec B\n\n```mermaid\nflowchart LR\n    P --> Q\n```\n"
	writeSpecFixture(t, root, "spec-b", specBFrontmatter, specBBody)

	specA := &artifact.SpecFrontmatter{Base: artifact.Base{ID: "spec/spec-a"}}
	bodyA, err := readCurrentSpecBody(root, specA)
	if err != nil {
		t.Fatalf("readCurrentSpecBody(spec-a): %v", err)
	}
	figsA := DiscoverIllustrativeFigures(bodyA)
	if len(figsA) != 1 {
		t.Fatalf("figsA = %+v, want exactly 1 (spec-a's own fenced block)", figsA)
	}
	if strings.Contains(bodyA, "P --> Q") {
		t.Fatalf("bodyA leaked spec-b's content: %q", bodyA)
	}

	specB := &artifact.SpecFrontmatter{Base: artifact.Base{ID: "spec/spec-b"}}
	bodyB, err := readCurrentSpecBody(root, specB)
	if err != nil {
		t.Fatalf("readCurrentSpecBody(spec-b): %v", err)
	}
	figsB := DiscoverIllustrativeFigures(bodyB)
	if len(figsB) != 1 {
		t.Fatalf("figsB = %+v, want exactly 1 (spec-b's own fenced block)", figsB)
	}
	if strings.Contains(bodyB, "X --> Y") {
		t.Fatalf("bodyB leaked spec-a's content: %q", bodyB)
	}
}

// TestDiscoverIllustrativeFigures_NoFencedBlocks_ExplicitEmpty proves a
// spec body with no fenced mermaid block returns a disclosed empty slice.
func TestDiscoverIllustrativeFigures_NoFencedBlocks_ExplicitEmpty(t *testing.T) {
	got := DiscoverIllustrativeFigures("# Title\n\nJust prose, no diagrams.\n")
	if got == nil {
		t.Fatal("DiscoverIllustrativeFigures returned nil, want an explicit empty slice")
	}
	if len(got) != 0 {
		t.Fatalf("DiscoverIllustrativeFigures = %+v, want empty", got)
	}
}

// minimalGraphJSON builds a canned flowmap graph.json body naming exactly
// the given fully-qualified names as first-party nodes — enough for
// diagramverify.TruthShortNames/TruthFQNs, with no edges.
func minimalGraphJSON(fqns ...string) []byte {
	var b strings.Builder
	b.WriteString(`{"nodes":[`)
	for i, fqn := range fqns {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(`{"fqn":"` + fqn + `"}`)
	}
	b.WriteString(`]}`)
	return []byte(b.String())
}

// TestComputeDiagramAlignment_Realized_FullCoverage is obligation
// ac-2--behavioral case (1): an accepted, from-scratch proposal whose
// declared elements are still all present in a canned truth graph
// discloses realized (full coverage).
func TestComputeDiagramAlignment_Realized_FullCoverage(t *testing.T) {
	root := t.TempDir()
	writeDiagramFixture(t, root, "loan-flow-clean",
		diagramFrontmatterYAML("diagram/loan-flow-clean", "accepted"),
		"flowchart LR\n    Alpha[\"Alpha\"]\n")

	runner := upstream.NewFakeRunner()
	runner.Enqueue("flowmap", "graph", upstream.Result{Stdout: minimalGraphJSON("pkg.Alpha"), ExitCode: 0})

	spec := &artifact.SpecFrontmatter{Base: artifact.Base{ID: "spec/whatever"}}
	findings, entries, illustrative, err := ComputeDiagramAlignment(context.Background(), root, runner, spec, "deadbeef")
	if err != nil {
		t.Fatalf("ComputeDiagramAlignment: %v", err)
	}
	if len(illustrative) != 0 {
		t.Fatalf("illustrative = %+v, want none (no spec.md on disk)", illustrative)
	}
	if len(findings) != 1 || len(entries) != 1 {
		t.Fatalf("findings/entries = %+v/%+v, want exactly 1 each", findings, entries)
	}
	f := findings[0]
	if f.ID != "diagram-loan-flow-clean" {
		t.Fatalf("finding id = %q, want diagram-loan-flow-clean", f.ID)
	}
	if f.Kind != artifact.FindingComputed {
		t.Fatalf("finding kind = %q, want computed", f.Kind)
	}
	if f.Text != "realized (full coverage)" {
		t.Fatalf("finding text = %q, want %q", f.Text, "realized (full coverage)")
	}
	if entries[0].Divergent {
		t.Fatalf("entry = %+v, want not divergent", entries[0])
	}
}

// TestComputeDiagramAlignment_Divergent_WithWitness is obligation
// ac-2--behavioral case (2): a derived proposal whose base-inherited
// element the canned truth graph no longer has discloses divergent, naming
// that element's candidate witness commit sha — over a real fixturegit
// history where a scripted commit removed the identity string, exactly
// diagramverify's own compare_test.go pattern.
func TestComputeDiagramAlignment_Divergent_WithWitness(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"svc/a.go": "package svc\n\nfunc LegacyStep() {}\n"}, Message: "add LegacyStep"},
		{Files: map[string]string{"svc/a.go": "package svc\n"}, Message: "remove LegacyStep"},
	})
	root := repo.Dir

	writeDiagramFixture(t, root, "loan-flow-base",
		"id: diagram/loan-flow-base\nkind: diagram\ntitle: Base\nowners: [platform-team]\nstatus: active\n",
		"flowchart LR\n    LegacyStep[\"LegacyStep\"]\n")
	writeDiagramFixture(t, root, "loan-flow-target",
		"id: diagram/loan-flow-target\nkind: diagram\ntitle: Target\nclass: proposal\nstatus: accepted\nowners: [platform-team]\n"+
			"frozen: { at: 2026-07-14, commit: 3e91ab2 }\n"+
			"derived_from: { ref: diagram/loan-flow-base, digest: sha256:"+hex64Diagram+" }\n",
		"flowchart LR\n    LegacyStep[\"LegacyStep\"]\n")

	runner := upstream.NewFakeRunner()
	// Truth no longer has LegacyStep (an unrelated node keeps the graph
	// non-empty) — served sticky for both the direct RegenerateTruth call
	// and StaleBase's own internal one.
	runner.Enqueue("flowmap", "graph", upstream.Result{Stdout: minimalGraphJSON("pkg.StillThere"), ExitCode: 0})

	spec := &artifact.SpecFrontmatter{Base: artifact.Base{ID: "spec/whatever"}}
	findings, entries, _, err := ComputeDiagramAlignment(context.Background(), root, runner, spec, repo.Head)
	if err != nil {
		t.Fatalf("ComputeDiagramAlignment: %v", err)
	}
	if len(findings) != 1 || len(entries) != 1 {
		t.Fatalf("findings/entries = %+v/%+v, want exactly 1 each", findings, entries)
	}
	f := findings[0]
	if !strings.Contains(f.Text, "divergent") {
		t.Fatalf("finding text = %q, want it to disclose divergent", f.Text)
	}
	if !strings.Contains(f.Text, "LegacyStep") {
		t.Fatalf("finding text = %q, want it to name the LegacyStep identity", f.Text)
	}
	if !strings.Contains(f.Text, repo.Heads[1]) {
		t.Fatalf("finding text = %q, want it to name the candidate witness commit %q", f.Text, repo.Heads[1])
	}
	if !entries[0].Divergent {
		t.Fatalf("entry = %+v, want Divergent", entries[0])
	}
}

// TestComputeDiagramAlignment_PartialCoverage_StillRealized is obligation
// ac-2--behavioral case (3): a proposal whose mermaid source falls outside
// verification-extractor's declared grammar (an unrecognized `subgraph`
// construct) but whose comparable elements show no divergence discloses
// partial coverage explicitly — never a bare "realized" indistinguishable
// from the full-coverage case.
func TestComputeDiagramAlignment_PartialCoverage_StillRealized(t *testing.T) {
	root := t.TempDir()
	writeDiagramFixture(t, root, "loan-flow-partial",
		diagramFrontmatterYAML("diagram/loan-flow-partial", "accepted"),
		"flowchart LR\n    Alpha[\"Alpha\"]\n    subgraph Cluster\n    end\n")

	runner := upstream.NewFakeRunner()
	runner.Enqueue("flowmap", "graph", upstream.Result{Stdout: minimalGraphJSON("pkg.Alpha"), ExitCode: 0})

	spec := &artifact.SpecFrontmatter{Base: artifact.Base{ID: "spec/whatever"}}
	findings, entries, _, err := ComputeDiagramAlignment(context.Background(), root, runner, spec, "deadbeef")
	if err != nil {
		t.Fatalf("ComputeDiagramAlignment: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("findings = %+v, want exactly 1", findings)
	}
	f := findings[0]
	if !strings.HasPrefix(f.Text, "realized") {
		t.Fatalf("finding text = %q, want it to start with realized", f.Text)
	}
	if f.Text == "realized (full coverage)" {
		t.Fatalf("finding text = %q, want partial coverage explicitly disclosed, not a bare realized", f.Text)
	}
	if !strings.Contains(f.Text, "partial coverage") {
		t.Fatalf("finding text = %q, want it to disclose partial coverage", f.Text)
	}
	if entries[0].Divergent {
		t.Fatalf("entry = %+v, want not divergent", entries[0])
	}
}

// TestComputeDiagramAlignment_UsesSpecBodyForIllustrative proves
// ComputeDiagramAlignment's illustrative half is wired to the real
// spec.md on disk when one exists (the earlier realized-full-coverage test
// only proves the "no spec.md" degrade path).
func TestComputeDiagramAlignment_UsesSpecBodyForIllustrative(t *testing.T) {
	root := t.TempDir()
	writeSpecFixture(t, root, "current-spec",
		"id: spec/current-spec\nkind: spec\ntitle: Current\nclass: component\nstatus: draft\nowners: [platform-team]\n",
		"# Current\n\n```mermaid\nflowchart LR\n    X --> Y\n```\n")

	runner := upstream.NewFakeRunner()
	spec := &artifact.SpecFrontmatter{Base: artifact.Base{ID: "spec/current-spec"}}
	_, _, illustrative, err := ComputeDiagramAlignment(context.Background(), root, runner, spec, "deadbeef")
	if err != nil {
		t.Fatalf("ComputeDiagramAlignment: %v", err)
	}
	if len(illustrative) != 1 {
		t.Fatalf("illustrative = %+v, want exactly 1", illustrative)
	}
}
