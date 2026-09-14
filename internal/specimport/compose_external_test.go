package specimport

import (
	"context"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/model"
)

// TestCompose_DuplicateIdentity proves an external candidate whose id
// collides with an existing committed spec is refused: lint.CheckCandidate's
// VL-002 global-uniqueness check, reused unchanged, surfaces a blocking
// finding rather than silently producing a second document under the same
// ref.
func TestCompose_DuplicateIdentity(t *testing.T) {
	root := minimalStoreRoot(t)
	writeStoreFile(t, root, ".verdi/specs/active/sample-feature/spec.md", nativeSpecFixture)
	// nativeSpecFixture declares id: spec/native-widget; retarget the
	// existing on-disk copy to collide with minimalRequest()'s own slug.
	existing := strings.Replace(nativeSpecFixture, "id: spec/native-widget", "id: spec/sample-feature", 1)
	writeStoreFile(t, root, ".verdi/specs/active/sample-feature/spec.md", existing)

	req := minimalRequest() // Target.Slug == "sample-feature"
	req.Mappings = []Mapping{
		{Target: "ac-1", Evidence: []string{"static", "attestation"}},
		{Target: "ac-2", Evidence: []string{"static", "attestation"}},
	}
	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}

	candidate, findings, err := Compose(context.Background(), root, req, plan)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	if candidate != nil {
		t.Fatalf("Compose composed a candidate colliding with an existing spec id: %s", candidate)
	}
	var sawDuplicate bool
	for _, f := range findings {
		if f.Blocking && f.Code == FindingInvalidCandidate && strings.Contains(f.Message, "VL-002") {
			sawDuplicate = true
		}
	}
	if !sawDuplicate {
		t.Fatalf("want a blocking VL-002 duplicate-ref finding, got: %+v", findings)
	}
}

// TestCompose_FeatureAttestationFloor proves a feature acceptance
// criterion whose mapped evidence omits "attestation" is refused —
// vl006.go's checkFeatureACAttestation, the outcome floor's minimum
// satisfying evidence kind, reused unchanged through lint.CheckCandidate.
func TestCompose_FeatureAttestationFloor(t *testing.T) {
	root := minimalStoreRoot(t)
	req := minimalRequest()
	req.Mappings = []Mapping{
		{Target: "ac-1", Evidence: []string{"static"}}, // no attestation: fails the outcome floor
		{Target: "ac-2", Evidence: []string{"static", "attestation"}},
	}
	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}

	candidate, findings, err := Compose(context.Background(), root, req, plan)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	if candidate != nil {
		t.Fatalf("Compose composed a candidate whose feature AC omits the attestation outcome floor: %s", candidate)
	}
	var sawFloor bool
	for _, f := range findings {
		if f.Blocking && strings.Contains(f.Message, "attestation") && strings.Contains(f.Message, "outcome floor") {
			sawFloor = true
		}
	}
	if !sawFloor {
		t.Fatalf("want a blocking finding naming the outcome floor's attestation requirement, got: %+v", findings)
	}
}

// TestCompose_InvalidLink proves a well-formed but unresolvable explicit
// link is refused: it is written into the candidate (Compose synthesizes
// nothing and drops nothing), and lint.CheckCandidate's VL-003 flags it as
// not resolving.
func TestCompose_InvalidLink(t *testing.T) {
	root := minimalStoreRoot(t)
	req := minimalRequest()
	req.Mappings = []Mapping{
		{Target: "ac-1", Evidence: []string{"static", "attestation"}},
		{Target: "ac-2", Evidence: []string{"static", "attestation"}},
	}
	req.Links = []Link{{Type: "depends-on", Ref: "spec/does-not-exist"}}

	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}

	candidate, findings, err := Compose(context.Background(), root, req, plan)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	if candidate != nil {
		t.Fatalf("Compose composed a candidate with a dangling explicit link: %s", candidate)
	}
	var sawUnresolved bool
	for _, f := range findings {
		if f.Blocking && strings.Contains(f.Message, "does not resolve") {
			sawUnresolved = true
		}
	}
	if !sawUnresolved {
		t.Fatalf("want a blocking finding naming the unresolved link, got: %+v", findings)
	}
}

// TestCompose_Story_MissingImplementsEdge proves a story candidate with no
// explicit implements (or spike resolves) edge is refused — "Stories
// require their existing valid parent/implements ... relationship ... no
// TODO tracker is synthesized" (spec-import-contract.md): Compose never
// invents one to make the candidate pass.
func TestCompose_Story_MissingImplementsEdge(t *testing.T) {
	root := minimalStoreRoot(t)
	req := Request{
		Schema:          RequestSchema,
		Target:          Target{Slug: "orphan-story", Class: "story", Title: "Orphan Story", Story: "jira:LOAN-1"},
		Format:          FormatManualV1,
		Primary:         "source",
		Sources:         []Source{{ID: "source", Label: "s.md", Data: []byte("content\n")}},
		DeferStatements: true, // isolate the missing-edge refusal from unrelated missing-statement noise
		RetainUnmapped:  true,
	}
	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}

	candidate, findings, err := Compose(context.Background(), root, req, plan)
	if err != nil {
		t.Fatalf("Compose: want a blocking finding, not an error: %v", err)
	}
	if candidate != nil {
		t.Fatalf("Compose composed a story candidate with no implements edge: %s", candidate)
	}
	var sawImplements bool
	for _, f := range findings {
		if f.Blocking && strings.Contains(f.Message, "implements") {
			sawImplements = true
		}
	}
	if !sawImplements {
		t.Fatalf("want a blocking finding about the missing implements edge, got: %+v", findings)
	}
}

// TestCompose_IncompatibleModelTemplate proves a model/template
// combination that cannot express the requested class is refused rather
// than silently accepted under the wrong class: a store override binds
// the feature class's template to a template that renders class: story.
func TestCompose_IncompatibleModelTemplate(t *testing.T) {
	root := minimalStoreRoot(t)
	customModel := strings.Replace(string(model.CanonicalYAML()), "template: feature.md", "template: mismatched-feature.md", 1)
	if !strings.Contains(customModel, "mismatched-feature.md") {
		t.Fatal("test fixture assumption broken: canonical.yaml's feature template literal changed")
	}
	writeStoreFile(t, root, ".verdi/model.yaml", customModel)

	const mismatchedTemplate = `---
id: {{safe .Ref}}
kind: spec
title: {{printf "%q" .Title}}
owners: {{safe .Owners}}
class: story
story: "jira:PLACEHOLDER-1"
problem: { text: {{printf "%q" .Problem}}, anchor: problem }
outcome: { text: {{printf "%q" .Outcome}}, anchor: outcome }
links:
  - { type: implements, ref: "spec/whatever#ac-1" }
---
# {{.Title}}

## Problem

TODO: design notes.

## Outcome

TODO: design notes.
`
	writeStoreFile(t, root, ".verdi/templates/mismatched-feature.md", mismatchedTemplate)

	req := minimalRequest() // Target.Class == "feature"
	req.Mappings = []Mapping{
		{Target: "ac-1", Evidence: []string{"static", "attestation"}},
		{Target: "ac-2", Evidence: []string{"static", "attestation"}},
	}
	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}

	candidate, findings, err := Compose(context.Background(), root, req, plan)
	if err != nil {
		t.Fatalf("Compose: want a blocking finding, not an error: %v", err)
	}
	if candidate != nil {
		t.Fatalf("Compose composed a candidate under an incompatible model/template binding: %s", candidate)
	}
	if !hasBlocking(findings) {
		t.Fatalf("want a blocking finding refusing the incompatible model/template binding, got: %+v", findings)
	}
}
