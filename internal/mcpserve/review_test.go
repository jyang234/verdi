package mcpserve

import (
	"context"
	"errors"
	"testing"

	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/boardio"
	"github.com/OWNER/verdi/internal/fixturegit"
	"github.com/OWNER/verdi/internal/forge"
	forgefake "github.com/OWNER/verdi/internal/forge/fake"
	"github.com/OWNER/verdi/internal/gitx"
)

const reviewSpecMD = `---
id: spec/loan-update
kind: spec
class: feature
title: "Loan update"
status: draft
owners: [platform-team]
story: jira:LOAN-1482
acceptance_criteria:
  - { id: ac-2, text: "a borrower can see the change reflected", evidence: [static] }
---
# body
`

// buildReviewFixture builds a fixturegit repo with a single draft spec
// (spec/loan-update, declaring ac-2) and checks out design/loan-update —
// review population only ever applies on a design branch (05 §Review
// stickies and forge round-trip).
func buildReviewFixture(t *testing.T) *fixturegit.Repo {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files:   map[string]string{".verdi/specs/active/loan-update/spec.md": reviewSpecMD},
		Message: "draft spec",
	}})
	if err := gitx.CheckoutNewBranch(context.Background(), repo.Dir, "design/loan-update"); err != nil {
		t.Fatalf("CheckoutNewBranch: %v", err)
	}
	return repo
}

// TestReviewMirroredAnnotations_NilForge_NotConfigured proves the first of
// the three I-1(b) states: no forge configured (Forge nil AND
// ReviewUnavailable "") is silent — nil items, NO disclosure.
func TestReviewMirroredAnnotations_NilForge_NotConfigured(t *testing.T) {
	repo := buildReviewFixture(t)
	b := &Backend{Root: repo.Dir}
	items, disclosure, err := b.reviewMirroredAnnotations(context.Background(), artifact.Ref{Kind: "spec", Name: "loan-update"})
	if err != nil {
		t.Fatalf("reviewMirroredAnnotations: %v", err)
	}
	if items != nil {
		t.Fatalf("items = %+v, want nil (no forge configured)", items)
	}
	if disclosure != "" {
		t.Fatalf("disclosure = %q, want empty (unconfigured forge is silent, not disclosed)", disclosure)
	}
}

// TestReviewMirroredAnnotations_NilForge_ConfiguredButUnavailable proves
// the third I-1(b) state: a forge is configured but no live adapter could
// be built (ReviewUnavailable set) — nil items but a DISCLOSURE, never
// silence (constitution 2/10).
func TestReviewMirroredAnnotations_NilForge_ConfiguredButUnavailable(t *testing.T) {
	repo := buildReviewFixture(t)
	b := &Backend{Root: repo.Dir, ReviewUnavailable: "forge \"gitlab\" is configured but no credentials are available"}
	items, disclosure, err := b.reviewMirroredAnnotations(context.Background(), artifact.Ref{Kind: "spec", Name: "loan-update"})
	if err != nil {
		t.Fatalf("reviewMirroredAnnotations: %v", err)
	}
	if items != nil {
		t.Fatalf("items = %+v, want nil (no live forge)", items)
	}
	if disclosure == "" {
		t.Fatal("disclosure = empty, want the configured-but-unavailable reason (never silent)")
	}
}

func TestReviewMirroredAnnotations_NotDesignBranch(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files:   map[string]string{".verdi/specs/active/loan-update/spec.md": reviewSpecMD},
		Message: "draft spec",
	}})
	// Stays on fixturegit's default branch ("main"), never checked out to
	// design/loan-update.
	f := forgefake.New()
	f.SeedOpenMR("main", forge.OpenMR{ID: "1", SourceBranch: "main", Title: "irrelevant"})
	b := &Backend{Root: repo.Dir, Forge: f}
	items, _, err := b.reviewMirroredAnnotations(context.Background(), artifact.Ref{Kind: "spec", Name: "loan-update"})
	if err != nil {
		t.Fatalf("reviewMirroredAnnotations: %v", err)
	}
	if items != nil {
		t.Fatalf("items = %+v, want nil (not on a design branch)", items)
	}
}

func TestReviewMirroredAnnotations_NoOpenMR(t *testing.T) {
	repo := buildReviewFixture(t)
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	f := forgefake.New() // nothing seeded
	b := &Backend{Root: repo.Dir, Forge: f}
	items, _, err := b.reviewMirroredAnnotations(context.Background(), artifact.Ref{Kind: "spec", Name: "loan-update"})
	if err != nil {
		t.Fatalf("reviewMirroredAnnotations: %v", err)
	}
	if items != nil {
		t.Fatalf("items = %+v, want nil (no open MR for this design branch yet)", items)
	}
}

// TestReviewMirroredAnnotations_TokenResolvesAndResolutionState proves
// the core round-trip: a token-bearing comment targeting ac-2 is mirrored
// with the object id resolved and the forge-native resolution state
// reflected; a token-free comment and a comment naming an object id this
// spec never declares are both excluded (05: the inbox tray and other
// targets' comments are not THIS ref's list_annotations concern).
func TestReviewMirroredAnnotations_TokenResolvesAndResolutionState(t *testing.T) {
	repo := buildReviewFixture(t)
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	f := forgefake.New()
	f.SeedOpenMR("main", forge.OpenMR{ID: "5", SourceBranch: "design/loan-update", Title: "Loan update"})
	f.SeedComment("5", forge.Comment{ID: "c1", ThreadID: "t1", Body: "[vd:ac-2] outcome AC reads implementation-scoped — reword?", Author: "reviewer", CreatedAt: "2026-07-11T18:00:00Z"})
	f.SeedComment("5", forge.Comment{ID: "c2", Body: "nit: no token here, inbox tray material"})
	f.SeedComment("5", forge.Comment{ID: "c3", ThreadID: "t3", Body: "[vd:ac-99] this targets a different spec's object entirely"})
	f.SeedThreadResolution("5", forge.ThreadResolution{ThreadID: "t1", Resolved: true, ResolvedBy: "reviewer"})

	b := &Backend{Root: repo.Dir, Forge: f}
	items, _, err := b.reviewMirroredAnnotations(context.Background(), artifact.Ref{Kind: "spec", Name: "loan-update"})
	if err != nil {
		t.Fatalf("reviewMirroredAnnotations: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %+v, want exactly 1 (only the ac-2 token-bearing comment resolves against this spec)", items)
	}
	got := items[0]
	if got.ObjectID != "ac-2" {
		t.Errorf("ObjectID = %q, want ac-2", got.ObjectID)
	}
	if got.Type != "review" {
		t.Errorf("Type = %q, want review", got.Type)
	}
	if got.Status != "resolved" {
		t.Errorf("Status = %q, want resolved (thread t1 was seeded resolved)", got.Status)
	}
	if got.Author != "reviewer" {
		t.Errorf("Author = %q, want reviewer", got.Author)
	}
	if got.Body != "[vd:ac-2] outcome AC reads implementation-scoped — reword?" {
		t.Errorf("Body = %q, want byte-identical to the forge comment", got.Body)
	}
}

func TestReviewMirroredAnnotations_UnresolvedThreadStaysOpen(t *testing.T) {
	repo := buildReviewFixture(t)
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	f := forgefake.New()
	f.SeedOpenMR("main", forge.OpenMR{ID: "5", SourceBranch: "design/loan-update", Title: "Loan update"})
	f.SeedComment("5", forge.Comment{ID: "c1", ThreadID: "t1", Body: "[vd:ac-2] reword?"})

	b := &Backend{Root: repo.Dir, Forge: f}
	items, _, err := b.reviewMirroredAnnotations(context.Background(), artifact.Ref{Kind: "spec", Name: "loan-update"})
	if err != nil {
		t.Fatalf("reviewMirroredAnnotations: %v", err)
	}
	if len(items) != 1 || items[0].Status != "open" {
		t.Fatalf("items = %+v, want exactly 1 with Status open", items)
	}
}

// erroringOpenMRForge wraps the fake forge but fails ListOpenMRs, proving
// a live-forge transport failure DEGRADES to a disclosure (I-1(b)/I-2:
// non-blocking, never silence, never a hard tool error) rather than
// failing the whole read — the local annotations still return.
type erroringOpenMRForge struct{ *forgefake.Forge }

func (erroringOpenMRForge) ListOpenMRs(ctx context.Context, targetBranch string) ([]forge.OpenMR, error) {
	return nil, errors.New("forge: simulated transport failure")
}

func TestReviewMirroredAnnotations_ForgeErrorDegradesToDisclosure(t *testing.T) {
	repo := buildReviewFixture(t)
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	b := &Backend{Root: repo.Dir, Forge: erroringOpenMRForge{forgefake.New()}}
	items, disclosure, err := b.reviewMirroredAnnotations(context.Background(), artifact.Ref{Kind: "spec", Name: "loan-update"})
	if err != nil {
		t.Fatalf("reviewMirroredAnnotations: want nil error (degrades to disclosure), got %v", err)
	}
	if items != nil {
		t.Fatalf("items = %+v, want nil on a failing feed", items)
	}
	if disclosure == "" {
		t.Fatal("disclosure = empty, want a review-population-unavailable notice (never silence)")
	}
}

// TestListAnnotations_MergesLocalAndReviewItems drives the real
// list_annotations tool end to end, proving local mutable-zone
// annotations and mirrored review items appear together in one result
// (05 §MCP server: list_annotations "covers... and (mirrored) review
// stickies").
func TestListAnnotations_MergesLocalAndReviewItems(t *testing.T) {
	repo := buildReviewFixture(t)
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	f := forgefake.New()
	f.SeedOpenMR("main", forge.OpenMR{ID: "5", SourceBranch: "design/loan-update", Title: "Loan update"})
	f.SeedComment("5", forge.Comment{ID: "c1", ThreadID: "t1", Body: "[vd:ac-2] reword?"})

	b := &Backend{Root: repo.Dir, Forge: f}
	target := &artifact.Target{Ref: "spec/loan-update@" + repo.Head, Selector: artifact.Selector{Heading: "ac-2"}}
	local := &artifact.Annotation{ID: "a-01ARZ3NDEKTSV4RRFFQ69G5FAV", TS: "2026-07-11T18:00:00Z", Author: "john", Target: target, Type: artifact.AnnotationComment, Body: "local scratch note", Status: artifact.AnnotationOpen}
	if err := boardio.AppendAnnotation(b.annotationsDir(), annotationFileForTarget(artifact.Ref{Kind: "spec", Name: "loan-update"}), local); err != nil {
		t.Fatalf("AppendAnnotation: %v", err)
	}

	result := b.ListAnnotations(context.Background(), mustArgs(t, map[string]string{"ref": "spec/loan-update"}))
	if isToolError(result) {
		t.Fatalf("ListAnnotations returned an error result: %s", toolResultText(t, result))
	}
	var decoded struct {
		Annotations []annotationItem `json:"annotations"`
	}
	toolResultJSON(t, result, &decoded)
	if len(decoded.Annotations) != 2 {
		t.Fatalf("Annotations = %+v, want 2 (1 local + 1 mirrored review)", decoded.Annotations)
	}
	var sawLocal, sawReview bool
	for _, a := range decoded.Annotations {
		switch a.Type {
		case "comment":
			sawLocal = true
		case "review":
			sawReview = true
			if a.ObjectID != "ac-2" {
				t.Errorf("review item ObjectID = %q, want ac-2", a.ObjectID)
			}
		}
	}
	if !sawLocal || !sawReview {
		t.Fatalf("Annotations = %+v, want both a local comment and a mirrored review item", decoded.Annotations)
	}
}

// TestListAnnotations_ConfiguredButUnavailable_Disclosed proves I-1(b) on
// the machine read surface: a forge configured but with no live adapter
// (Backend.ReviewUnavailable set, Forge nil) makes list_annotations return
// a review_unavailable disclosure field rather than silently omitting the
// review layer — while still returning whatever local annotations exist.
func TestListAnnotations_ConfiguredButUnavailable_Disclosed(t *testing.T) {
	repo := buildReviewFixture(t)
	b := &Backend{Root: repo.Dir, ReviewUnavailable: `forge "gitlab" is configured but no credentials are available`}

	result := b.ListAnnotations(context.Background(), mustArgs(t, map[string]string{"ref": "spec/loan-update"}))
	if isToolError(result) {
		t.Fatalf("ListAnnotations returned an error result: %s", toolResultText(t, result))
	}
	var decoded struct {
		ReviewUnavailable string           `json:"review_unavailable"`
		Annotations       []annotationItem `json:"annotations"`
	}
	toolResultJSON(t, result, &decoded)
	if decoded.ReviewUnavailable == "" {
		t.Fatal("review_unavailable field absent — a configured-but-unavailable forge must disclose, never stay silent")
	}
}

// TestListAnnotations_NoForge_NoDisclosure proves the silent state on the
// machine surface: no forge configured at all yields NO review_unavailable
// field (an unconfigured integration is legitimately silent).
func TestListAnnotations_NoForge_NoDisclosure(t *testing.T) {
	repo := buildReviewFixture(t)
	b := &Backend{Root: repo.Dir}

	result := b.ListAnnotations(context.Background(), mustArgs(t, map[string]string{"ref": "spec/loan-update"}))
	var raw map[string]any
	toolResultJSON(t, result, &raw)
	if _, present := raw["review_unavailable"]; present {
		t.Fatalf("review_unavailable present with no forge configured, want absent (silent): %#v", raw)
	}
}
