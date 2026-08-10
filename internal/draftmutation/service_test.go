package draftmutation

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/artifact/splice"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/designprovenance"
	"github.com/jyang234/verdi/internal/policyauthority"
	"github.com/jyang234/verdi/internal/specstate"
	"github.com/jyang234/verdi/internal/store"
)

func TestServiceIdentityResolutionFailureDisclosesIdentityUnavailable(t *testing.T) {
	root := serviceRoot(t, []byte(baseSpec))
	request := requestFor(t, root, []byte(baseSpec), []Operation{{Op: OpSetProblem, Text: "changed", Anchor: "#problem"}})
	service := testService(root, resolvedPolicy(t, "draft-write", true), specstate.Proposed)
	service.Identity = fakeIdentityReader{err: errors.New("git facts unavailable")}

	response, diagnostic := service.Mutate(context.Background(), root, request, testAgent(t))
	if diagnostic == nil || diagnostic.Code != CodeIdentityInvalid || diagnostic.IdentityAvailable() || response.Result != nil || response.Stale != nil {
		t.Fatalf("response/diagnostic = %+v, %v", response, diagnostic)
	}
	if diagnostic.Identity != (Identity{}) {
		t.Fatalf("unavailable identity diagnostic fabricated identity %+v", diagnostic.Identity)
	}
}

func serviceRoot(t *testing.T, specBytes []byte) string {
	t.Helper()
	root := t.TempDir()
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	root = resolved
	if err := os.MkdirAll(store.SpecDir(root, store.ZoneActive, "sample"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".verdi", "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.SpecPath(root, store.ZoneActive, "sample"), specBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func requestFor(t *testing.T, root string, base []byte, operations []Operation) Request {
	t.Helper()
	doc := map[string]any{
		"schema": RequestSchema, "spec": "spec/sample", "base_digest": DigestBytes(base),
		"base_spec_b64": base64.StdEncoding.EncodeToString(base),
		"expected":      map[string]any{"checkout": filepath.ToSlash(root), "branch": "design/sample", "head": strings.Repeat("a", 40)},
		"operations":    operations,
	}
	raw, err := canonjson.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	request, err := DecodeRequest(raw)
	if err != nil {
		t.Fatalf("DecodeRequest: %v", err)
	}
	return request
}

func testService(root string, policy *policyauthority.EffectivePolicy, state specstate.State) Service {
	return Service{
		Identity: fakeIdentityReader{root: root, branch: "design/sample", head: strings.Repeat("a", 40)},
		State:    fakeStateProjector{result: specstate.Result{State: state}},
		Policy:   staticPolicySource{policy: policy},
	}
}

func testAgent(t *testing.T) Actor {
	t.Helper()
	actor, err := NewDelegatedAgent("codex", "session-1")
	if err != nil {
		t.Fatal(err)
	}
	return actor
}

func TestServiceAppliesAtomicMutationAndBindsPolicyProvenance(t *testing.T) {
	root := serviceRoot(t, []byte(baseSpec))
	policy := resolvedPolicy(t, "draft-write", true)
	request := requestFor(t, root, []byte(baseSpec), []Operation{{Op: OpSetProblem, Text: "new problem", Anchor: "#problem"}})
	response, typed := testService(root, policy, specstate.Proposed).Mutate(context.Background(), root, request, testAgent(t))
	if typed != nil {
		t.Fatalf("Mutate: %v", typed)
	}
	if response.Result == nil || response.Stale != nil || response.Result.Identity.Checkout != filepath.ToSlash(root) {
		t.Fatalf("response = %+v", response)
	}
	current, err := os.ReadFile(store.SpecPath(root, store.ZoneActive, "sample"))
	if err != nil || DigestBytes(current) != response.Result.ResultDigest {
		t.Fatalf("current spec/result = %v, %v", err, response.Result)
	}
	logBytes, err := os.ReadFile(store.DesignProvenancePath(root, store.ZoneActive, "sample"))
	if err != nil {
		t.Fatal(err)
	}
	entries, err := designprovenance.DecodeLog(logBytes)
	if err != nil || len(entries) != 1 {
		t.Fatalf("DecodeLog = %+v, %v", entries, err)
	}
	policyDigest, _ := policy.Digest()
	entry := entries[0]
	if entry.PolicyDigest != policyDigest || entry.ResultDigest != response.Result.ResultDigest || !entry.Attribution.Unauthenticated || entry.Harness != "codex" || entry.Session != "session-1" {
		t.Fatalf("provenance entry = %+v", entry)
	}
	if len(response.Result.Disclosures) != 1 || response.Result.Disclosures[0].Code != DisclosureContextUnavailable {
		t.Fatalf("disclosures = %+v", response.Result.Disclosures)
	}
}

func TestServiceStaleUsesCanonicalIdentityAndWritesNothing(t *testing.T) {
	current, err := splice.ApplyDraftMutations([]byte(baseSpec), []designprovenance.Operation{{Op: designprovenance.OpSetProblem, Text: "direct change", Anchor: "#problem"}})
	if err != nil {
		t.Fatal(err)
	}
	root := serviceRoot(t, current)
	request := requestFor(t, root, []byte(baseSpec), []Operation{{Op: OpSetOutcome, Text: "new outcome", Anchor: "#outcome"}})
	before := append([]byte(nil), current...)
	response, typed := testService(root, resolvedPolicy(t, "draft-write", true), specstate.Proposed).Mutate(context.Background(), root, request, testAgent(t))
	if typed == nil || typed.Code != CodeStaleBase || response.Stale == nil || response.Result != nil {
		t.Fatalf("response/error = %+v, %v", response, typed)
	}
	if typed.Identity != response.Stale.Identity || response.Stale.Identity.Checkout != filepath.ToSlash(root) || response.Stale.CurrentDigest != DigestBytes(current) {
		t.Fatalf("stale identities = %+v / %+v", typed, response.Stale)
	}
	if len(response.Stale.ChangedTargets) != 1 || response.Stale.ChangedTargets[0] != "problem" {
		t.Fatalf("changed targets = %v", response.Stale.ChangedTargets)
	}
	after, _ := os.ReadFile(store.SpecPath(root, store.ZoneActive, "sample"))
	if !bytes.Equal(before, after) {
		t.Fatal("stale refusal changed spec")
	}
	if _, err := os.Stat(store.DesignProvenancePath(root, store.ZoneActive, "sample")); !os.IsNotExist(err) {
		t.Fatalf("stale refusal wrote provenance: %v", err)
	}
}

func TestServiceDirectMarkdownGapAndBatchRollback(t *testing.T) {
	policy := resolvedPolicy(t, "draft-write", true)
	root := serviceRoot(t, []byte(baseSpec))
	service := testService(root, policy, specstate.Proposed)
	first := requestFor(t, root, []byte(baseSpec), []Operation{{Op: OpSetProblem, Text: "typed one", Anchor: "#problem"}})
	firstResponse, typed := service.Mutate(context.Background(), root, first, testAgent(t))
	if typed != nil {
		t.Fatal(typed)
	}
	typedBytes, _ := os.ReadFile(store.SpecPath(root, store.ZoneActive, "sample"))
	directBytes, err := splice.ApplyDraftMutations(typedBytes, []designprovenance.Operation{{Op: designprovenance.OpSetOutcome, Text: "direct markdown", Anchor: "#outcome"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.SpecPath(root, store.ZoneActive, "sample"), directBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	second := requestFor(t, root, directBytes, []Operation{{Op: OpEditAC, ID: "ac-1", Text: "typed two", Evidence: []artifact.EvidenceKind{artifact.EvidenceStatic}, Anchor: "#ac-1"}})
	secondResponse, typed := service.Mutate(context.Background(), root, second, testAgent(t))
	if typed != nil {
		t.Fatal(typed)
	}
	if len(secondResponse.Result.Disclosures) != 2 || secondResponse.Result.Disclosures[0].Code != DisclosureUnclassifiedDirectEdit {
		t.Fatalf("disclosures = %+v", secondResponse.Result.Disclosures)
	}
	logBytes, _ := os.ReadFile(store.DesignProvenancePath(root, store.ZoneActive, "sample"))
	entries, err := designprovenance.DecodeLog(logBytes)
	if err != nil || len(entries) != 2 || entries[1].UnclassifiedGap == nil || entries[1].UnclassifiedGap.FromDigest != firstResponse.Result.ResultDigest || entries[1].UnclassifiedGap.ToDigest != DigestBytes(directBytes) {
		t.Fatalf("entries = %+v, %v", entries, err)
	}

	beforeSpec, _ := os.ReadFile(store.SpecPath(root, store.ZoneActive, "sample"))
	beforeLog, _ := os.ReadFile(store.DesignProvenancePath(root, store.ZoneActive, "sample"))
	rollback := requestFor(t, root, beforeSpec, []Operation{
		{Op: OpSetProblem, Text: "would apply", Anchor: "#problem"},
		{Op: OpRemoveAC, ID: "ac-missing"},
	})
	if _, typed := service.Mutate(context.Background(), root, rollback, testAgent(t)); typed == nil || typed.Code != CodeOperationInvalid {
		t.Fatalf("batch rollback error = %v", typed)
	}
	afterSpec, _ := os.ReadFile(store.SpecPath(root, store.ZoneActive, "sample"))
	afterLog, _ := os.ReadFile(store.DesignProvenancePath(root, store.ZoneActive, "sample"))
	if !bytes.Equal(beforeSpec, afterSpec) || !bytes.Equal(beforeLog, afterLog) {
		t.Fatal("invalid batch changed spec or provenance")
	}
}

func TestServiceEveryRefusalWritesNothingAndCarriesOneIdentity(t *testing.T) {
	policy := resolvedPolicy(t, "draft-write", true)
	tests := []struct {
		name    string
		mutate  func(*Request, *Service, *Actor, string)
		want    Code
		journal bool
	}{
		{"identity", func(request *Request, _ *Service, _ *Actor, _ string) {
			request.Expected.Head = strings.Repeat("b", 40)
		}, CodeIdentityInvalid, false},
		{"state", func(_ *Request, service *Service, _ *Actor, _ string) {
			service.State = fakeStateProjector{result: specstate.Result{State: specstate.Closed}}
		}, CodeStateForbidden, false},
		{"policy", func(_ *Request, service *Service, _ *Actor, _ string) {
			service.Policy = staticPolicySource{policy: resolvedPolicy(t, "off", true)}
		}, CodePolicyForbidden, false},
		{"actor", func(_ *Request, _ *Service, actor *Actor, _ string) { *actor = Actor{} }, CodeActorForbidden, false},
		{"result", func(request *Request, _ *Service, _ *Actor, _ string) {
			request.Operations = []Operation{{Op: OpSetProblem, Text: "old problem", Anchor: "#problem"}}
		}, CodeResultInvalid, false},
		{"recovery", func(_ *Request, _ *Service, _ *Actor, root string) {
			path := store.DraftMutationJournalPath(root, "sample")
			_ = os.MkdirAll(filepath.Dir(path), 0o755)
			_ = os.WriteFile(path, []byte(`{"schema":"broken"}`), 0o644)
		}, CodeRecoveryInvalid, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := serviceRoot(t, []byte(baseSpec))
			request := requestFor(t, root, []byte(baseSpec), []Operation{{Op: OpSetProblem, Text: "changed", Anchor: "#problem"}})
			service := testService(root, policy, specstate.Proposed)
			actor := testAgent(t)
			tt.mutate(&request, &service, &actor, root)
			before, _ := os.ReadFile(store.SpecPath(root, store.ZoneActive, "sample"))
			response, typed := service.Mutate(context.Background(), root, request, actor)
			if typed == nil || typed.Code != tt.want || typed.Identity.Checkout != filepath.ToSlash(root) || response.Result != nil || response.Stale != nil {
				t.Fatalf("response/error = %+v, %v", response, typed)
			}
			after, _ := os.ReadFile(store.SpecPath(root, store.ZoneActive, "sample"))
			if !bytes.Equal(before, after) {
				t.Fatal("refusal changed spec")
			}
			if _, err := os.Stat(store.DesignProvenancePath(root, store.ZoneActive, "sample")); !os.IsNotExist(err) {
				t.Fatalf("refusal wrote provenance: %v", err)
			}
			if tt.journal {
				if _, err := os.Stat(store.DraftMutationJournalPath(root, "sample")); err != nil {
					t.Fatalf("malformed journal not retained: %v", err)
				}
			}
		})
	}
}
