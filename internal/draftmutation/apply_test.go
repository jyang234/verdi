package draftmutation

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/designprovenance"
)

func testIdentity() Identity {
	return Identity{Checkout: "/tmp/repository", Branch: "design/sample", Head: strings.Repeat("a", 40), Spec: "spec/sample"}
}

func decodedRequest(t *testing.T) Request {
	t.Helper()
	req, err := DecodeRequest(validRequestBytes(t))
	if err != nil {
		t.Fatal(err)
	}
	return req
}

func TestApplyOrderedChangesWarningsAndExcerpts(t *testing.T) {
	req := decodedRequest(t)
	req.Operations = []Operation{
		{Op: OpEditAC, ID: "ac-1", Text: strings.Repeat("x", 1001), Evidence: []artifact.EvidenceKind{artifact.EvidenceStatic}, Anchor: "#ac-1"},
		{Op: OpReorderAC, ID: "ac-2"},
		{Op: OpRemoveConstraint, ID: "co-1"},
		{Op: OpAddContextRef, Ref: "spec/other@abcdef2"},
	}
	req.Excerpts = []ExcerptRequest{{Target: "ac-1", Classification: designprovenance.ClassificationAIInferred, Representation: designprovenance.RepresentationParaphrase, Text: "expanded criterion"}}
	before := []byte(baseSpec)
	applied, err := Apply(before, req, testIdentity())
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !bytes.Equal(before, []byte(baseSpec)) {
		t.Fatal("Apply mutated caller-owned base bytes")
	}
	wantTargets := []string{"ac-1", "ac-1", "ac-2", "co-1", "context/spec%2Fother%40abcdef2"}
	gotTargets := make([]string, len(applied.Result.Changes))
	for i, change := range applied.Result.Changes {
		gotTargets[i] = change.Target
	}
	if !reflect.DeepEqual(gotTargets, wantTargets) {
		t.Fatalf("change targets = %v, want %v", gotTargets, wantTargets)
	}
	wantWarnings := []string{"large-replacement/ac-1", "semantic-reorder/ac-1", "semantic-reorder/ac-2", "destructive-removal/co-1", "relationship-change/context/spec%2Fother%40abcdef2"}
	gotWarnings := make([]string, len(applied.Result.Warnings))
	for i, warning := range applied.Result.Warnings {
		gotWarnings[i] = string(warning.Code) + "/" + warning.Target
	}
	if !reflect.DeepEqual(gotWarnings, wantWarnings) {
		t.Fatalf("warnings = %v, want %v", gotWarnings, wantWarnings)
	}
	if len(applied.ProvenanceExcerpts) != 1 || applied.ProvenanceExcerpts[0].TargetDigest == "" {
		t.Fatalf("provenance excerpts = %+v", applied.ProvenanceExcerpts)
	}
	semantic, err := snapshot(applied.Spec)
	if err != nil {
		t.Fatal(err)
	}
	if applied.ProvenanceExcerpts[0].TargetDigest != semantic["ac-1"].ObjectDigest {
		t.Fatalf("excerpt target digest = %q, want resulting object digest %q", applied.ProvenanceExcerpts[0].TargetDigest, semantic["ac-1"].ObjectDigest)
	}
	if applied.Result.Identity != testIdentity() || applied.Result.PreviousDigest != DigestBytes(before) || applied.Result.ResultDigest != DigestBytes(applied.Spec) {
		t.Fatalf("result identity/digests = %+v", applied.Result)
	}
}

func TestSemanticDiffChangedTargetsSorted(t *testing.T) {
	req := decodedRequest(t)
	req.Operations = []Operation{
		{Op: OpSetProblem, Text: "changed", Anchor: "#problem"},
		{Op: OpRemoveContextRef, Ref: "spec/base@abcdef0"},
		{Op: OpAddStub, Slug: "third-story", AcceptanceCriteria: []string{"ac-1"}},
	}
	applied, err := Apply([]byte(baseSpec), req, testIdentity())
	if err != nil {
		t.Fatal(err)
	}
	got, err := ChangedTargets([]byte(baseSpec), applied.Spec)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"context/spec%2Fbase%40abcdef0", "problem", "stub/third-story"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ChangedTargets = %v, want %v", got, want)
	}
	if got := PercentComponent("a/é "); got != "a%2F%C3%A9%20" {
		t.Fatalf("PercentComponent = %q", got)
	}
}

func TestApplyRejectsUnknownExcerptTarget(t *testing.T) {
	req := decodedRequest(t)
	req.Excerpts = []ExcerptRequest{{Target: "co-missing", Classification: designprovenance.ClassificationUnresolved, Representation: designprovenance.RepresentationVerbatim, Text: "unknown"}}
	if _, err := Apply([]byte(baseSpec), req, testIdentity()); err == nil || !strings.Contains(err.Error(), "excerpt target") {
		t.Fatalf("Apply error = %v", err)
	}
}

func TestApplyReportsSecondarySemanticTargetsInOperationThenTargetOrder(t *testing.T) {
	req := decodedRequest(t)
	req.Operations = []Operation{
		{Op: OpReorderAC, ID: "ac-2"},
		{Op: OpRemoveDecision, ID: "dc-1"},
	}
	applied, err := Apply([]byte(baseSpec), req, testIdentity())
	if err != nil {
		t.Fatal(err)
	}
	gotChanges := make([]string, len(applied.Result.Changes))
	for i, change := range applied.Result.Changes {
		gotChanges[i] = string(change.Change) + "/" + change.Target
	}
	wantChanges := []string{
		"reordered/ac-1", "reordered/ac-2",
		"removed/dc-1", "relationship-removed/link/dc-1/depends-on/spec%2Fbase",
	}
	if !reflect.DeepEqual(gotChanges, wantChanges) {
		t.Fatalf("changes = %v, want %v", gotChanges, wantChanges)
	}
	gotWarnings := make([]string, len(applied.Result.Warnings))
	for i, warning := range applied.Result.Warnings {
		gotWarnings[i] = string(warning.Code) + "/" + warning.Target
	}
	wantWarnings := []string{
		"semantic-reorder/ac-1", "semantic-reorder/ac-2",
		"destructive-removal/dc-1", "relationship-change/link/dc-1/depends-on/spec%2Fbase",
	}
	if !reflect.DeepEqual(gotWarnings, wantWarnings) {
		t.Fatalf("warnings = %v, want %v", gotWarnings, wantWarnings)
	}
}

func TestApplyCanonicalResultGoldenAndTypedRefusals(t *testing.T) {
	req := decodedRequest(t)
	applied, err := Apply([]byte(baseSpec), req, testIdentity())
	if err != nil {
		t.Fatal(err)
	}
	raw, err := EncodeResult(applied.Result)
	if err != nil {
		t.Fatal(err)
	}
	goldenPath := filepath.Join("testdata", "result.json")
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, want) {
		t.Fatalf("result golden mismatch\ngot:  %s\nwant: %s", raw, want)
	}

	refusal := StaleRefusal{Schema: RefusalSchema, Identity: testIdentity(), Code: CodeStaleBase, CurrentDigest: DigestBytes([]byte(baseSpec + "\n")), ChangedTargets: []string{"problem"}}
	refusalRaw, err := EncodeStaleRefusal(refusal)
	if err != nil || !bytes.Contains(refusalRaw, []byte(`"identity":{"branch":"design/sample"`)) {
		t.Fatalf("EncodeStaleRefusal = %s, %v", refusalRaw, err)
	}
	for _, code := range []Code{CodeStateForbidden, CodePolicyForbidden, CodeActorForbidden, CodeOperationInvalid, CodeResultInvalid, CodeInputInvalid, CodeIdentityInvalid, CodeAuthorityInvalid, CodeRecoveryInvalid, CodeIOFailure} {
		typed := NewError(code, testIdentity(), "detail")
		if typed.Identity != testIdentity() || typed.Code != code {
			t.Fatalf("typed refusal = %+v", typed)
		}
	}
}
