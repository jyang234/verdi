package specimport

import (
	"context"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/designscaffold"
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

// trackerManifestYAML configures the jira provider VL-005 requires before
// any story's scheme-qualified tracker ref counts as configured.
const trackerManifestYAML = "schema: verdi.layout/v1\n" +
	"providers:\n" +
	"  jira:\n" +
	"    base_url: https://example.atlassian.net\n" +
	"    rollup_field: customfield_00000\n"

// storeRootWithComposedParent returns a store root whose manifest configures
// the jira tracker and whose corpus already holds one valid parent feature —
// composed by Compose itself, so a story's parent is exactly what this
// importer produces rather than a hand-written near-copy — plus that
// parent's slug.
func storeRootWithComposedParent(t *testing.T) (root, parentSlug string) {
	t.Helper()
	root = minimalStoreRoot(t)
	writeStoreFile(t, root, ".verdi/verdi.yaml", trackerManifestYAML)

	parent := minimalRequest()
	parent.Mappings = []Mapping{
		{Target: "ac-1", Evidence: []string{"static", "attestation"}},
		{Target: "ac-2", Evidence: []string{"static", "attestation"}},
	}
	plan, err := Normalize(parent)
	if err != nil {
		t.Fatalf("Normalize parent: %v", err)
	}
	candidate, findings, err := Compose(context.Background(), root, parent, plan)
	if err != nil {
		t.Fatalf("Compose parent: %v", err)
	}
	requireNoBlocking(t, findings, candidate)
	writeStoreFile(t, root, ".verdi/specs/active/"+parent.Target.Slug+"/spec.md", string(candidate))
	return root, parent.Target.Slug
}

// TestCompose_ExternalStory_Happy is the positive half of the import
// surface the contract's authority return names ("import one feature/story
// from selected native or Markdown sources"): a markdown-v1 STORY with a
// configured tracker, a valid parent and implements edge, and explicit
// evidence composes into a real candidate. The canonical story template
// renders no stubs: block at all, so unconditional placeholder-stub removal
// refuses every external story — this test is the reachability proof that
// TestCompose_Story_MissingImplementsEdge cannot give (it refuses three
// stages earlier, at the rendered-scaffold decode gate).
func TestCompose_ExternalStory_Happy(t *testing.T) {
	root, parentSlug := storeRootWithComposedParent(t)

	req := minimalRequest()
	req.Target = Target{Slug: "sample-story", Class: "story", Title: "Sample Story", Story: "jira:SAMPLE-1"}
	req.Links = []Link{{Type: "implements", Ref: "spec/" + parentSlug + "#ac-1"}}
	req.Mappings = []Mapping{
		{Target: "ac-1", Evidence: []string{"static"}},
		{Target: "ac-2", Evidence: []string{"static"}},
	}

	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	requireNoBlocking(t, plan.Findings, nil)

	candidate, findings, err := Compose(context.Background(), root, req, plan)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	requireNoBlocking(t, findings, candidate)
	if candidate == nil {
		t.Fatal("Compose returned nil bytes for a valid external story")
	}

	fm, body, err := artifact.SplitFrontmatter(candidate)
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v\n%s", err, candidate)
	}
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		t.Fatalf("DecodeSpec: %v\n%s", err, candidate)
	}
	if err := spec.ResolveObjectAnchors(body); err != nil {
		t.Fatalf("ResolveObjectAnchors: %v\n%s", err, candidate)
	}
	if spec.Class != artifact.ClassStory {
		t.Fatalf("composed class = %q, want story", spec.Class)
	}
	if len(spec.Stubs) != 0 {
		t.Fatalf("stubs not absent on the story candidate: %+v", spec.Stubs)
	}
	if len(spec.AcceptanceCriteria) != 2 {
		t.Fatalf("want the 2 mapped acceptance criteria, got %d: %+v\n%s", len(spec.AcceptanceCriteria), spec.AcceptanceCriteria, candidate)
	}
	if strings.Contains(string(candidate), "TODO: replace with real acceptance criteria before accept") {
		t.Fatalf("the story template's placeholder criterion survived:\n%s", candidate)
	}
	var sawImplements bool
	for _, l := range spec.Base.Links {
		if l.Type == artifact.LinkImplements && l.Ref == "spec/"+parentSlug+"#ac-1" {
			sawImplements = true
		}
	}
	if !sawImplements {
		t.Fatalf("the story's implements edge is missing from the candidate: %+v", spec.Base.Links)
	}
}

// featureTemplateWithoutPlaceholderStub is a store override that declares no
// stubs: block at all — a template that can express the candidate perfectly,
// since the contract's own post-condition is "the resulting imported feature
// has `stubs` absent". Composing against it must succeed: the absence of a
// placeholder is not a defect to report, and refusing here would be the
// inverse of "reject a template/model that cannot express the candidate".
const featureTemplateWithoutPlaceholderStub = `---
id: {{safe .Ref}}
kind: spec
title: {{printf "%q" .Title}}
owners: {{safe .Owners}}
class: feature
problem: { text: {{printf "%q" .Problem}}, anchor: problem }
outcome: { text: {{printf "%q" .Outcome}}, anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "TODO: replace with real acceptance criteria before accept", evidence: [static, attestation], anchor: ac-1 }
---
# {{.Title}}

## Problem

TODO: design notes.

## Outcome

TODO: design notes.

## Ac 1

TODO: design notes.
`

// TestCompose_TemplateWithoutPlaceholderStub proves placeholder removal is
// driven by what the rendered scaffold actually declares, not by an
// unconditional operation: an override template carrying no stubs: block
// composes cleanly instead of being refused for lacking a placeholder to
// remove.
func TestCompose_TemplateWithoutPlaceholderStub(t *testing.T) {
	root := minimalStoreRoot(t)
	writeStoreFile(t, root, ".verdi/templates/feature.md", featureTemplateWithoutPlaceholderStub)

	req := minimalRequest()
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
	requireNoBlocking(t, findings, candidate)
	if candidate == nil {
		t.Fatal("Compose returned nil bytes for a template that declares no placeholder stub")
	}
	fm, body, err := artifact.SplitFrontmatter(candidate)
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v\n%s", err, candidate)
	}
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		t.Fatalf("DecodeSpec: %v\n%s", err, candidate)
	}
	if err := spec.ResolveObjectAnchors(body); err != nil {
		t.Fatalf("ResolveObjectAnchors: %v\n%s", err, candidate)
	}
	if len(spec.Stubs) != 0 {
		t.Fatalf("stubs not absent: %+v", spec.Stubs)
	}
	if len(spec.AcceptanceCriteria) != 2 {
		t.Fatalf("want the 2 mapped acceptance criteria, got %d: %+v\n%s", len(spec.AcceptanceCriteria), spec.AcceptanceCriteria, candidate)
	}
}

// TestCompose_TemplateWithNonPlaceholderStub_Refused completes B1's
// post-condition. A store override differing from the embedded default in
// nothing but its stub slug renders and validates perfectly, but its stub is
// not the generated placeholder Compose removes — so the import used to
// succeed carrying a decomposition nobody requested, against the contract's
// "the resulting imported feature has `stubs` absent (not a fabricated
// placeholder or an invented decomposition)".
//
// The template is refused explicitly, naming the stub. Silently deleting it
// is the wrong repair: a configured stub is the store owner's own data, and
// "reject a template/model that cannot express the candidate rather than
// dropping content" is the contract's stated posture for exactly this case.
func TestCompose_TemplateWithNonPlaceholderStub_Refused(t *testing.T) {
	root := minimalStoreRoot(t)
	canonical, err := designscaffold.LoadTemplate(root, "feature.md")
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	override := strings.Replace(string(canonical), placeholderStubSlug, "custom-child", 1)
	if !strings.Contains(override, "custom-child") {
		t.Fatal("test fixture assumption broken: the canonical feature template no longer declares the placeholder stub slug")
	}
	writeStoreFile(t, root, ".verdi/templates/feature.md", override)

	req := minimalRequest()
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
		t.Fatalf("Compose imported a template-declared decomposition nobody requested:\n%s", candidate)
	}
	var sawRefusal bool
	for _, f := range findings {
		if f.Blocking && f.Code == FindingUnsupportedStructure && f.Target == "template" &&
			strings.Contains(f.Message, "custom-child") {
			sawRefusal = true
		}
	}
	if !sawRefusal {
		t.Fatalf("want a blocking %s finding on target \"template\" naming the stub it cannot express, got: %+v", FindingUnsupportedStructure, findings)
	}
}

// featureTemplateWithCustomFields is a store override carrying a
// team-sanctioned custom: extension key and its own extra body section, on
// top of the canonical placeholder shape — the "preserve template-defined
// custom fields ... rather than dropping content" half of the contract's
// candidate rules.
const featureTemplateWithCustomFields = `---
id: {{safe .Ref}}
kind: spec
title: {{printf "%q" .Title}}
owners: {{safe .Owners}}
class: feature
problem: { text: {{printf "%q" .Problem}}, anchor: problem }
outcome: { text: {{printf "%q" .Outcome}}, anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "TODO: replace with real acceptance criteria before accept", evidence: [static, attestation], anchor: ac-1 }
stubs:
  - { slug: todo-replace-stub-slug, acceptance_criteria: [ac-1] }
custom:
  rollout_plan: "staged"
---
# {{.Title}}

## Problem

TODO: design notes.

## Outcome

TODO: design notes.

## Ac 1

TODO: design notes.

## Rollout Plan

Ship behind a flag.
`

// TestCompose_PreservesCustomTemplateFields proves neither the placeholder
// removal nor the anchor rewrite drops a template's own custom data: the
// custom: key and the extra body section both survive into the candidate.
func TestCompose_PreservesCustomTemplateFields(t *testing.T) {
	root := minimalStoreRoot(t)
	writeStoreFile(t, root, ".verdi/templates/feature.md", featureTemplateWithCustomFields)

	req := minimalRequest()
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
	requireNoBlocking(t, findings, candidate)

	fm, body, err := artifact.SplitFrontmatter(candidate)
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v\n%s", err, candidate)
	}
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		t.Fatalf("DecodeSpec: %v\n%s", err, candidate)
	}
	if got := spec.Custom["rollout_plan"]; got != "staged" {
		t.Fatalf("custom.rollout_plan = %v, want %q:\n%s", got, "staged", candidate)
	}
	if !strings.Contains(string(body), "## Rollout Plan\n\nShip behind a flag.") {
		t.Fatalf("the template's own extra body section was dropped:\n%s", body)
	}
	if len(spec.Stubs) != 0 {
		t.Fatalf("the declared placeholder stub was not removed: %+v", spec.Stubs)
	}
}

// sourceWithAllObjectKinds labels one object of every generated kind, in a
// fixed source order, so the composed candidate's anchors, body text and
// insertion order can all be pinned at once.
const sourceWithAllObjectKinds = "# Sample Feature\n" +
	"\n" +
	"## Problem\n" +
	"\n" +
	"First line.\n" +
	"Second line.\n" +
	"\n" +
	"## Outcome\n" +
	"\n" +
	"Users get value.\n" +
	"\n" +
	"## Acceptance Criteria\n" +
	"\n" +
	"- Criterion one.\n" +
	"\n" +
	"## Constraints\n" +
	"\n" +
	"- Must stay offline.\n" +
	"\n" +
	"## Decisions\n" +
	"\n" +
	"- Use the existing store.\n" +
	"\n" +
	"## Open Questions\n" +
	"\n" +
	"- Who owns rollout?\n"

// TestCompose_GeneratedAnchorsAreBareObjectIDs pins the contract's declared
// candidate output for all four object kinds at once: "Generated anchors are
// bare `problem`, `outcome` and object IDs with `## Problem`, `## Outcome`,
// `## ac-1`, etc." ResolveObjectAnchors is slug-symmetric and would accept a
// "#ac-1" anchor too, so only an explicit assertion keeps the declared
// output honest. Body text and insertion order are pinned in the same pass
// ("Map order: statements, then objects in source/explicit insertion order").
func TestCompose_GeneratedAnchorsAreBareObjectIDs(t *testing.T) {
	root := minimalStoreRoot(t)
	req := minimalRequest()
	req.Sources[0].Data = []byte(sourceWithAllObjectKinds)
	req.Mappings = []Mapping{{Target: "ac-1", Evidence: []string{"static", "attestation"}}}

	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	requireNoBlocking(t, plan.Findings, nil)

	candidate, findings, err := Compose(context.Background(), root, req, plan)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	requireNoBlocking(t, findings, candidate)

	fm, body, err := artifact.SplitFrontmatter(candidate)
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v\n%s", err, candidate)
	}
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		t.Fatalf("DecodeSpec: %v\n%s", err, candidate)
	}
	if err := spec.ResolveObjectAnchors(body); err != nil {
		t.Fatalf("ResolveObjectAnchors: %v\n%s", err, candidate)
	}

	if spec.Problem == nil || spec.Problem.Anchor != "problem" {
		t.Fatalf("problem anchor = %+v, want the bare %q", spec.Problem, "problem")
	}
	if spec.Outcome == nil || spec.Outcome.Anchor != "outcome" {
		t.Fatalf("outcome anchor = %+v, want the bare %q", spec.Outcome, "outcome")
	}

	type objectCheck struct{ id, anchor, text string }
	got := []objectCheck{}
	for _, o := range spec.AcceptanceCriteria {
		got = append(got, objectCheck{o.ID, o.Anchor, o.Text})
	}
	for _, o := range spec.Constraints {
		got = append(got, objectCheck{o.ID, o.Anchor, o.Text})
	}
	for _, o := range spec.Decisions {
		got = append(got, objectCheck{o.ID, o.Anchor, o.Text})
	}
	for _, o := range spec.OpenQuestions {
		got = append(got, objectCheck{o.ID, o.Anchor, o.Text})
	}
	want := []objectCheck{
		{"ac-1", "ac-1", "Criterion one."},
		{"co-1", "co-1", "Must stay offline."},
		{"dc-1", "dc-1", "Use the existing store."},
		{"oq-1", "oq-1", "Who owns rollout?"},
	}
	if len(got) != len(want) {
		t.Fatalf("composed objects = %+v, want %+v\n%s", got, want, candidate)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("object %d = %+v, want %+v (bare anchors, verbatim text)\n%s", i, got[i], want[i], candidate)
		}
	}

	bodyStr := string(body)
	for _, o := range want {
		section := "## " + o.id + "\n\n" + o.text + "\n"
		if !strings.Contains(bodyStr, section) {
			t.Fatalf("body is missing the %q section %q:\n%s", o.id, section, bodyStr)
		}
	}
	// Insertion order: every object section appears in source order.
	prev := -1
	for _, o := range want {
		at := strings.Index(bodyStr, "## "+o.id+"\n")
		if at <= prev {
			t.Fatalf("object sections are out of source order at %q (offset %d, previous %d):\n%s", o.id, at, prev, bodyStr)
		}
		prev = at
	}
}

// sourceWithNoAcceptanceCriteria labels Problem and Outcome but declares no
// criteria at all — a feature requirement that is not deferrable.
const sourceWithNoAcceptanceCriteria = "# Sample Feature\n" +
	"\n" +
	"## Problem\n" +
	"\n" +
	"First line.\n" +
	"\n" +
	"## Outcome\n" +
	"\n" +
	"Users get value.\n"

// TestCompose_FeatureWithNoAcceptanceCriteria_Refused proves a
// non-deferrable requirement blocks rather than being dropped to make
// validation pass: a feature whose source declares no criteria loses the
// scaffold's placeholder (it is never retained as a real imported
// requirement) and is then refused by the shared validate-before-write gate
// with nil bytes.
func TestCompose_FeatureWithNoAcceptanceCriteria_Refused(t *testing.T) {
	root := minimalStoreRoot(t)
	req := minimalRequest()
	req.Sources[0].Data = []byte(sourceWithNoAcceptanceCriteria)

	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}

	candidate, findings, err := Compose(context.Background(), root, req, plan)
	if err != nil {
		t.Fatalf("Compose: want a blocking finding, not an error: %v", err)
	}
	if candidate != nil {
		t.Fatalf("Compose composed a feature with no acceptance criteria:\n%s", candidate)
	}
	var sawRefusal bool
	for _, f := range findings {
		if f.Blocking && f.Code == FindingInvalidCandidate &&
			strings.Contains(f.Message, "feature spec must declare at least one acceptance criterion") {
			sawRefusal = true
		}
	}
	if !sawRefusal {
		t.Fatalf("want a blocking invalid-candidate finding naming the missing criterion requirement, got: %+v", findings)
	}
}

// TestCompose_Story_UnconfiguredTracker proves the story-only rule the
// shared candidate seam exists to reuse actually fires on a COMPOSED story:
// with a valid parent and implements edge but no jira provider configured,
// VL-005 refuses the candidate. Before the placeholder-removal fix this rule
// was unreachable on the external path — every story died at operation[0].
func TestCompose_Story_UnconfiguredTracker(t *testing.T) {
	root, parentSlug := storeRootWithComposedParent(t)
	// Drop the provider configuration the parent's composition needed nothing
	// from; the story's own jira:SAMPLE-1 tracker now has no configured scheme.
	writeStoreFile(t, root, ".verdi/verdi.yaml", "schema: verdi.layout/v1\n")

	req := minimalRequest()
	req.Target = Target{Slug: "sample-story", Class: "story", Title: "Sample Story", Story: "jira:SAMPLE-1"}
	req.Links = []Link{{Type: "implements", Ref: "spec/" + parentSlug + "#ac-1"}}
	req.Mappings = []Mapping{
		{Target: "ac-1", Evidence: []string{"static"}},
		{Target: "ac-2", Evidence: []string{"static"}},
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
		t.Fatalf("Compose composed a story whose tracker scheme is not configured:\n%s", candidate)
	}
	var sawTracker bool
	for _, f := range findings {
		if f.Blocking && f.Code == FindingInvalidCandidate && strings.Contains(f.Message, "VL-005") {
			sawTracker = true
		}
	}
	if !sawTracker {
		t.Fatalf("want a blocking VL-005 finding about the unconfigured tracker, got: %+v", findings)
	}
}

// TestCompose_Story_UnresolvableParent is the second story-only reachability
// proof: a configured tracker and a well-formed implements edge naming a
// parent that does not exist is refused by VL-003 on the composed story,
// rather than silently accepted.
func TestCompose_Story_UnresolvableParent(t *testing.T) {
	root, _ := storeRootWithComposedParent(t)

	req := minimalRequest()
	req.Target = Target{Slug: "sample-story", Class: "story", Title: "Sample Story", Story: "jira:SAMPLE-1"}
	req.Links = []Link{{Type: "implements", Ref: "spec/no-such-feature#ac-1"}}
	req.Mappings = []Mapping{
		{Target: "ac-1", Evidence: []string{"static"}},
		{Target: "ac-2", Evidence: []string{"static"}},
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
		t.Fatalf("Compose composed a story implementing a nonexistent parent:\n%s", candidate)
	}
	var sawUnresolved bool
	for _, f := range findings {
		if f.Blocking && strings.Contains(f.Message, "VL-003") && strings.Contains(f.Message, "does not resolve") {
			sawUnresolved = true
		}
	}
	if !sawUnresolved {
		t.Fatalf("want a blocking VL-003 finding naming the unresolvable parent, got: %+v", findings)
	}
}

// TestCompose_Story_MissingImplementsEdge proves a story candidate with no
// explicit implements (or spike resolves) edge is refused — "Stories
// require their existing valid parent/implements ... relationship ... no
// TODO tracker is synthesized" (spec-import-contract.md): Compose never
// invents one to make the candidate pass. The refusal comes from the
// rendered-scaffold decode gate, three stages before the candidate lint
// seam, because a story scaffold with no edge at all cannot even be
// rendered into a decodable spec — pinned here so a regression cannot move
// it silently to some other gate.
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
		if f.Blocking && f.Code == FindingUnsupportedStructure &&
			strings.Contains(f.Message, "rendered scaffold does not decode") &&
			strings.Contains(f.Message, "story spec requires >=1 implements edge") {
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
	var sawIncompatible bool
	for _, f := range findings {
		if f.Blocking && f.Code == FindingUnsupportedStructure && f.Target == "target.class" &&
			strings.Contains(f.Message, `rendered content declares class "story", want "feature"`) {
			sawIncompatible = true
		}
	}
	if !sawIncompatible {
		t.Fatalf("want a blocking %s finding naming the class mismatch, got: %+v", FindingUnsupportedStructure, findings)
	}
}
