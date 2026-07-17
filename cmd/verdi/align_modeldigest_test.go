package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/model"
)

// This file closes the judged finding
// ac1-align-sites-never-exercise-a-store-model-yaml: ac-1's evidence
// contract ("At least one case per site must exercise a fixture
// `.verdi/model.yaml` distinct from the embedded canonical") was met by a
// per-site store resolution only in internal/commitdesign; the three
// internal/align distinguishing tests inject a distinct digest straight into
// Input.ModelDigest, and cmd/verdi's align tests use canonical-only stores.
// So no test drove a deviation/decision/diagram report mint from a store
// whose model.yaml actually differs from canonical — the resolve→Input leg
// for the align verbs was proven only compositionally, and a cmd-level bug
// that always fed the canonical digest to align.Generate would have passed
// every test.
//
// This test drives the REAL align verb (cmdAlign, resolving its own deps
// from the store — not injected fake deps) over a store carrying a distinct
// .verdi/model.yaml, and asserts the generated deviation report's
// provenance.model equals THAT store's model digest, computed independently.
//
// One test covers the align verb's three modes because they SHARE one
// resolution: cmdAlign resolves the model digest exactly once
// (cmd/verdi/align.go's `cfg.Model.Digest()` after store.Open) into
// deps.ModelDigest, then threads that single value into every mode's Input —
// the deviation path (align.go: `align.Input{ModelDigest: deps.ModelDigest}`),
// the decision-conflict path (align_design.go: `DecisionConflictInput{...:
// deps.ModelDigest}`), and the diagram-sweep path (aligndiagramsweep.go:
// `DiagramSweepInput{...: deps.ModelDigest}`). A cmd-level bug in that single
// resolution — feeding canonical instead of the store's resolved model —
// would be caught here for all three, because this store's model.yaml is
// deliberately distinct from canonical.

// distinctModelYAML is internal/model/testdata/vocab-rename.yaml's own
// content verbatim (already proven frontier-legal by that package's own
// tests, and the same fixture internal/align/report_test.go and
// internal/commitdesign/commitdesign_test.go inline for this exact purpose):
// structurally identical to the embedded canonical model, but with
// vocabulary renames and different per-class template filenames — the
// frontier's two named exceptions — so its Digest() differs from
// model.Canonical().Digest().
const distinctModelYAML = `schema: verdi.model/v1

classes:
  feature:
    display: Feature
    decomposes: stubs
    template: custom-feature.md
  story:
    display: Story
    parent: feature
    template: custom-story.md

lifecycle:
  feature:
    states: [draft, accepted-pending-build, closed, superseded]
    terminal: [closed, superseded]
    transitions:
      - verb: accept
        from: draft
        to: accepted-pending-build
        obligations:
          - { scheme: attestation, kind: author-vouch }
      - verb: close
        from: accepted-pending-build
        to: closed
        obligations:
          - { scheme: attestation, kind: countersign, count: 1 }
          - { scheme: behavioral, kind: fold-green }
  story:
    states: [draft, accepted-pending-build, closed, superseded]
    terminal: [closed, superseded]
    transitions:
      - verb: accept
        from: draft
        to: accepted-pending-build
        obligations:
          - { scheme: attestation, kind: author-vouch }
      - verb: close
        from: accepted-pending-build
        to: closed
        obligations:
          - { scheme: attestation, kind: countersign, count: 1 }
          - { scheme: behavioral, kind: fold-green }

vocabulary:
  verbs:
    accept: "Sign off"
  states:
    accepted-pending-build: "Ready to build"
  classes:
    feature: "Initiative"
`

// alignModelDigestSpecMD is a minimal accepted feature spec with NO impacts
// and NO diagrams, so align.Compute needs no upstream toolchain invocation
// (the impacted-services loop and proposal regeneration are both skipped) —
// keeping the cmdAlign run hermetic while still exercising its real model
// resolution. frozen is any syntactically valid sha (SpecFrontmatter.Validate
// checks only the shape); its commit is never git-resolved because no
// impacted service triggers the acceptance-baseline diff.
const alignModelDigestSpecMD = `---
id: spec/model-digest-cmd
kind: spec
class: feature
title: "Model digest cmd fixture"
status: accepted-pending-build
owners: [platform-team]
story: jira:LOAN-1482
acceptance_criteria:
  - { id: ac-1, text: "static obligation holds", evidence: [static] }
frozen: { at: 2024-01-01, commit: 0000000000000000000000000000000000000a }
---
# body
`

// buildAlignModelDigestRepo builds a one-layer fixturegit repo carrying a
// distinct .verdi/model.yaml and a no-impacts accepted spec, then checks out
// feature/model-digest-cmd (the branch cmdAlign infers its spec from).
func buildAlignModelDigestRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				// A toolchain: block so cmdAlign constructs a real upstream
				// Runner — align.Compute requires a non-nil Runner even for a
				// no-impacts spec (its "no toolchain configured" guard). The
				// module/commit are never fetched or exec'd: a no-impacts spec
				// skips the service loop and an empty diagrams corpus skips
				// proposal regeneration, so the Runner is constructed-but-inert,
				// keeping this test hermetic (CLAUDE.md: no network in any test).
				// No align: block, so no judge is required.
				".verdi/verdi.yaml":                            "schema: verdi.layout/v1\nforge: gitlab\ntoolchain:\n  module: github.com/jyang234/golang-code-graph\n  commit: cd38b1a56bb782177a207d741a39807821cf2c1c\n",
				".verdi/model.yaml":                            distinctModelYAML,
				".verdi/specs/active/model-digest-cmd/spec.md": alignModelDigestSpecMD,
			},
			Message: "scaffold + distinct model.yaml + no-impacts spec",
		},
	})
	checkoutBranch(t, repo.Dir, "feature/model-digest-cmd")
	return repo
}

// TestCmdAlign_DeviationModelDigestResolvedFromStoreModelYAML drives the real
// `verdi align` entry point over a store whose .verdi/model.yaml differs from
// the embedded canonical and asserts the generated deviation report's
// provenance.model equals THAT store's resolved digest — closing the
// resolve→Input leg for the align verbs (see this file's top comment for why
// one deviation test proves the shared resolution for all three modes).
func TestCmdAlign_DeviationModelDigestResolvedFromStoreModelYAML(t *testing.T) {
	repo := buildAlignModelDigestRepo(t)

	// The distinct store model's digest, computed INDEPENDENTLY of the code
	// path under test (decode the same .verdi/model.yaml bytes directly),
	// alongside the embedded canonical's — so "tracks the store model" cannot
	// pass by accidentally matching canonical.
	fixtureModel, err := model.DecodeModel([]byte(distinctModelYAML))
	if err != nil {
		t.Fatalf("model.DecodeModel(distinctModelYAML): %v", err)
	}
	fixtureDigest, err := fixtureModel.Digest()
	if err != nil {
		t.Fatalf("fixture model Digest(): %v", err)
	}
	canonicalDigest, err := model.Canonical().Digest()
	if err != nil {
		t.Fatalf("model.Canonical().Digest(): %v", err)
	}
	if fixtureDigest == canonicalDigest {
		t.Fatalf("distinct model digest %q equals canonical — the fixture model.yaml is not actually distinct", fixtureDigest)
	}

	// Drive the REAL align verb end to end. cmdAlign resolves the store's
	// model itself (store.Open(root).Model.Digest(), align.go's own inline
	// resolution) and threads it into align.Input.ModelDigest — nothing is
	// injected. t.Chdir so cmdAlign's store.FindRoot(".") lands on the fixture.
	t.Chdir(repo.Dir)
	var stdout, stderr bytes.Buffer
	if code := cmdAlign(nil, &stdout, &stderr); code != 0 {
		t.Fatalf("cmdAlign exit = %d, want 0\nstdout: %s\nstderr: %s", code, stdout.String(), stderr.String())
	}

	reportPath := filepath.Join(repo.Dir, ".verdi", "specs", "active", "model-digest-cmd", "deviation-report.md")
	raw, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("reading generated report %s: %v", reportPath, err)
	}
	fmBytes, _, err := artifact.SplitFrontmatter(raw)
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	decoded, err := artifact.DecodeDeviation(fmBytes)
	if err != nil {
		t.Fatalf("DecodeDeviation(generated report): %v\n%s", err, raw)
	}
	if decoded.Provenance == nil {
		t.Fatal("generated report carries no provenance block")
	}
	if decoded.Provenance.Model != fixtureDigest {
		t.Fatalf("generated report's provenance.model = %q, want %q — cmdAlign must resolve the STORE's .verdi/model.yaml digest, not the embedded canonical %q", decoded.Provenance.Model, fixtureDigest, canonicalDigest)
	}
}
