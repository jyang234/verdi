package main

// Wave 6 Task 2's semantic REDs (task-2 brief §METHOD), evolved to their
// GREEN target form after the migration landed. The captured RED (verbatim
// in the lane report, base 7899b5f6) drove both:
//
//	--- FAIL: TestASDWorkbenchRoutesMutationsThroughDesignApp
//	    board mutation left no design-provenance sidecar (the legacy
//	    splice bypass): stat .../design-provenance.jsonl: no such file
//	--- FAIL: TestASDWorkbenchPreservesUnsavedState
//	    GET /board/spec/sample/snapshot = 404, want 200
//
// GREEN proves, over the PRODUCTION serve bridge (newServeDesignBridge —
// the exact designapp wiring `verdi serve` injects):
//
//  1. one browser mutate_draft action lands the spec edit AND its
//     matching design-provenance record atomically (AC-1/AC-4), carrying
//     the kernel's explicit unauthenticated-human attribution (SI-163)
//     and the v2 resolved policy arm on this adopted-policy fixture
//     (SI-176); and
//  2. the /snapshot projection exists with a deterministic revision token
//     over its rendered facts, answers 304 for an unchanged token, and
//     changes its token when a rendered fact changes (SI-165) — the
//     server half of unsaved-state-preserving conditional refresh.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/designapp"
	"github.com/jyang234/verdi/internal/designprovenance"
	"github.com/jyang234/verdi/internal/draftmutation"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/workbench"
)

// asdWorkbenchHandler builds the production workbench handler over the
// designMutateStore fixture with the SAME designapp bridge `verdi serve`
// injects — the served path, not a test double.
func asdWorkbenchHandler(t *testing.T, root string) http.Handler {
	t.Helper()
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	return workbench.NewHandlerWith(filepath.FromSlash(root), workbench.Deps{Design: newServeDesignBridge()})
}

func TestASDWorkbenchRoutesMutationsThroughDesignApp(t *testing.T) {
	root, head, base := designMutateStore(t)
	handler := asdWorkbenchHandler(t, root)

	request := designMutateRequest(t, root, "design/sample", head, base, []map[string]any{
		{"op": "edit-ac", "id": "ac-1", "text": "first, routed through designapp", "evidence": []string{"static"}, "anchor": "#ac-1"},
	})
	envelope, err := json.Marshal(map[string]any{"request": json.RawMessage(request)})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/board/spec/sample/api/mutate_draft", strings.NewReader(string(envelope)))
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("mutate_draft = %d, want 200; body:\n%s", rec.Code, rec.Body.String())
	}
	var outcome struct {
		Result     *json.RawMessage `json:"result"`
		Projection *struct {
			Dirty      bool   `json:"dirty"`
			Revision   string `json:"revision"`
			BaseDigest string `json:"base_digest"`
		} `json:"projection"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &outcome); err != nil || outcome.Result == nil {
		t.Fatalf("mutate_draft response carries no typed result (err=%v):\n%s", err, rec.Body.String())
	}
	// §4.3's clean contract: success rides one fresh projection whose base
	// digest names the NEW bytes.
	if outcome.Projection == nil || !outcome.Projection.Dirty || outcome.Projection.Revision == "" {
		t.Fatalf("clean mutation carries no fresh projection: %s", rec.Body.String())
	}

	specBytes, err := os.ReadFile(store.SpecPath(filepath.FromSlash(root), store.ZoneActive, "sample"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(specBytes), "routed through designapp") {
		t.Fatalf("spec.md does not carry the mutation:\n%s", specBytes)
	}
	if got := draftmutation.DigestBytes(specBytes); outcome.Projection.BaseDigest != got {
		t.Fatalf("projection base digest = %q, want the fresh bytes' digest %q", outcome.Projection.BaseDigest, got)
	}

	logBytes, err := os.ReadFile(store.DesignProvenancePath(filepath.FromSlash(root), store.ZoneActive, "sample"))
	if err != nil {
		t.Fatalf("board mutation left no design-provenance sidecar (the legacy splice bypass): %v", err)
	}
	entries, err := designprovenance.DecodeLog(logBytes)
	if err != nil || len(entries) != 1 {
		t.Fatalf("provenance entries = %+v, %v; want exactly one", entries, err)
	}
	entry := entries[0]
	if entry.Schema != designprovenance.SchemaV2 {
		t.Fatalf("provenance schema = %q, want %q", entry.Schema, designprovenance.SchemaV2)
	}
	if !entry.Attribution.Unauthenticated || entry.Attribution.PrincipalID != "" || entry.Harness != "" || entry.Session != "" {
		t.Fatalf("attribution = %+v harness=%q, want the kernel's explicit unauthenticated-human attribution with no harness/session", entry.Attribution, entry.Harness)
	}
	if entry.Policy == nil || entry.Policy.State != designprovenance.PolicyResolved || entry.Policy.Digest == "" {
		t.Fatalf("policy = %+v, want the resolved v2 policy arm on this adopted-policy fixture", entry.Policy)
	}
}

// TestASDWorkbenchFailureSchemaMatchesDesignApp pins the workbench's
// re-declared failure envelope literal to designapp's own constant (the
// workbench cannot import designapp — designapp imports workbench — so the
// byte equality is proven here, where both are importable).
func TestASDWorkbenchFailureSchemaMatchesDesignApp(t *testing.T) {
	root, head, base := designMutateStore(t)
	handler := asdWorkbenchHandler(t, root)
	// Force a kernel VERDICT through the browser action: an operation
	// addressing an undeclared object (operation-invalid).
	request := designMutateRequest(t, root, "design/sample", head, base, []map[string]any{
		{"op": "edit-ac", "id": "ac-99", "text": "x", "evidence": []string{"static"}, "anchor": "#ac-99"},
	})
	envelope, _ := json.Marshal(map[string]any{"request": json.RawMessage(request)})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/board/spec/sample/api/mutate_draft", strings.NewReader(string(envelope))))
	if rec.Code != http.StatusOK {
		t.Fatalf("stale-expected mutation = %d\n%s", rec.Code, rec.Body.String())
	}
	var out struct {
		Failure *designapp.Failure `json:"failure"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.Failure == nil {
		t.Fatalf("no typed failure decoded (err=%v):\n%s", err, rec.Body.String())
	}
	if out.Failure.Schema != designapp.FailureSchema {
		t.Fatalf("failure schema = %q, want designapp.FailureSchema %q", out.Failure.Schema, designapp.FailureSchema)
	}
	if out.Failure.Classification != designapp.ClassificationVerdict {
		t.Fatalf("classification = %q, want verdict (operation-invalid)", out.Failure.Classification)
	}
}

func TestASDWorkbenchPreservesUnsavedState(t *testing.T) {
	root, _, base := designMutateStore(t)
	handler := asdWorkbenchHandler(t, root)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/board/spec/sample/snapshot", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /board/spec/sample/snapshot = %d, want 200 (the conditional-refresh projection SI-165 fixes); body:\n%s", rec.Code, rec.Body.String())
	}
	var snap struct {
		Revision   string `json:"revision"`
		HTML       string `json:"html"`
		BaseDigest string `json:"base_digest"`
		BaseSpec   string `json:"base_spec_b64"`
		Expected   struct {
			Checkout string `json:"checkout"`
			Branch   string `json:"branch"`
			Head     string `json:"head"`
		} `json:"expected"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatalf("snapshot decode: %v", err)
	}
	if snap.Revision == "" || snap.HTML == "" || snap.BaseSpec == "" {
		t.Fatalf("snapshot missing facts: revision=%q html-len=%d base-len=%d", snap.Revision, len(snap.HTML), len(snap.BaseSpec))
	}
	if want := draftmutation.DigestBytes(base); snap.BaseDigest != want {
		t.Fatalf("snapshot base_digest = %q, want the current spec bytes' digest %q", snap.BaseDigest, want)
	}
	if snap.Expected.Branch != "design/sample" || snap.Expected.Head == "" || snap.Expected.Checkout == "" {
		t.Fatalf("snapshot expected identity = %+v, want the kernel's canonical identity", snap.Expected)
	}
	etag := rec.Header().Get("ETag")
	if etag == "" {
		t.Fatal("snapshot carries no ETag revision token")
	}

	// Unchanged facts → 304 with no body: the poll leaves the page (and
	// its unsaved state) completely untouched.
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/board/spec/sample/snapshot", nil)
	req2.Header.Set("If-None-Match", etag)
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusNotModified {
		t.Fatalf("unchanged snapshot = %d, want 304", rec2.Code)
	}
	if rec2.Body.Len() != 0 {
		t.Fatalf("304 carries a body (%d bytes)", rec2.Body.Len())
	}

	// A changed rendered fact must change the token: a legal direct
	// Markdown edit lands and the same If-None-Match now yields 200 with a
	// fresh revision.
	specPath := store.SpecPath(filepath.FromSlash(root), store.ZoneActive, "sample")
	changed := strings.Replace(string(base), "old problem", "new problem", 1)
	if err := os.WriteFile(specPath, []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	rec3 := httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodGet, "/board/spec/sample/snapshot", nil)
	req3.Header.Set("If-None-Match", etag)
	handler.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusOK {
		t.Fatalf("changed snapshot = %d, want 200", rec3.Code)
	}
	var snap3 struct {
		Revision string `json:"revision"`
	}
	if err := json.Unmarshal(rec3.Body.Bytes(), &snap3); err != nil || snap3.Revision == snap.Revision || snap3.Revision == "" {
		t.Fatalf("revision did not change with the rendered facts (err=%v, revision=%q)", err, snap3.Revision)
	}
}

// designMutateStoreWithoutPolicy is designMutateStore minus the adopted
// .verdi/policy tree: the AI-free store shape (no design_assistance
// authority at all) every plain checkout starts in.
func designMutateStoreWithoutPolicy(t *testing.T) (root, head string, base []byte) {
	t.Helper()
	root, head, base = designMutateStore(t)
	if err := os.RemoveAll(filepath.Join(filepath.FromSlash(root), ".verdi", "policy")); err != nil {
		t.Fatal(err)
	}
	return root, head, base
}

// TestASDWorkbenchProvenancePolicyUnionOnTheWire is the parked reviewer
// item F5 (Task 1A closure), now owned by Task 2: the BROWSER
// get_design_provenance output path carries the exact
// verdi.design-provenance/v2 closed policy union AT THE WIRE LEVEL — raw
// JSON keys, not a typed re-decode — for both arms:
//
//   - resolved  (adopted-policy store: {"state":"resolved","digest":"sha256:..."}), and
//   - not-applicable (unadopted store: {"state":"not-applicable"}, digest FORBIDDEN),
//
// the second being SI-176's honest-absence posture for the explicit
// browser-human mutation (never a fabricated digest or hash of absence).
func TestASDWorkbenchProvenancePolicyUnionOnTheWire(t *testing.T) {
	cases := []struct {
		name    string
		fixture func(*testing.T) (string, string, []byte)
		state   string
	}{
		{"adopted policy resolves its digest", designMutateStore, "resolved"},
		{"unadopted policy is honestly not-applicable", designMutateStoreWithoutPolicy, "not-applicable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, head, base := tc.fixture(t)
			handler := asdWorkbenchHandler(t, root)

			request := designMutateRequest(t, root, "design/sample", head, base, []map[string]any{
				{"op": "set-problem", "text": "policy union probe", "anchor": "#problem"},
			})
			envelope, _ := json.Marshal(map[string]any{"request": json.RawMessage(request)})
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/board/spec/sample/api/mutate_draft", strings.NewReader(string(envelope))))
			if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"result"`) {
				t.Fatalf("browser mutation = %d\n%s", rec.Code, rec.Body.String())
			}

			prov := httptest.NewRecorder()
			handler.ServeHTTP(prov, httptest.NewRequest(http.MethodPost, "/board/spec/sample/api/get_design_provenance", strings.NewReader(`{}`)))
			if prov.Code != http.StatusOK {
				t.Fatalf("get_design_provenance = %d\n%s", prov.Code, prov.Body.String())
			}
			// Wire-level: walk the RAW JSON.
			var raw struct {
				Entries []map[string]json.RawMessage `json:"entries"`
			}
			if err := json.Unmarshal(prov.Body.Bytes(), &raw); err != nil || len(raw.Entries) != 1 {
				t.Fatalf("provenance wire decode: err=%v entries=%d\n%s", err, len(raw.Entries), prov.Body.String())
			}
			entry := raw.Entries[0]
			policyRaw, ok := entry["policy"]
			if !ok {
				t.Fatalf("v2 entry carries no top-level policy object:\n%s", prov.Body.String())
			}
			var policy map[string]json.RawMessage
			if err := json.Unmarshal(policyRaw, &policy); err != nil {
				t.Fatal(err)
			}
			var state string
			if err := json.Unmarshal(policy["state"], &state); err != nil || state != tc.state {
				t.Fatalf("policy state = %q (err=%v), want %q", state, err, tc.state)
			}
			digestRaw, hasDigest := policy["digest"]
			if tc.state == "resolved" {
				var digest string
				if !hasDigest || json.Unmarshal(digestRaw, &digest) != nil || !strings.HasPrefix(digest, "sha256:") {
					t.Fatalf("resolved arm digest = %s, want a sha256 digest", digestRaw)
				}
			} else if hasDigest {
				t.Fatalf("not-applicable arm carries a digest field: %s", policyRaw)
			}
			if _, hasLegacy := entry["policy_digest"]; hasLegacy {
				t.Fatalf("v2 entry still carries the v1 policy_digest field:\n%s", prov.Body.String())
			}
		})
	}
}
