package workbench

// Shared ASD test scaffolding (Wave 6 Task 2). Test files are exempt from
// the draftmutation import boundary (boundary_test.go guards production
// files only), so the in-package test bridge routes MutateDraft onto the
// SAME draftmutation kernel designapp.MutateDraft passes through — the
// mutation path under test is byte-identical to production; only the five
// read operations are canned here (their production encoding is proven by
// cmd/verdi's bridge tests and the conformance suite).

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/draftmutation"
)

// testASDView is the render tests' minimal valid ASD view: empty maps,
// no capabilities, a derived shell over empty facts — enough for every
// pre-Wave-6 render assertion to keep exercising its own concern while
// the region now also carries the posture header and shell.
func testASDView() *asdView {
	v := &asdView{
		SlugPattern:    specNameRe.String(),
		NextIDs:        map[string]string{"ac": "ac-1", "co": "co-1", "dc": "dc-1", "oq": "oq-1"},
		ObjectAnchors:  map[string]string{},
		ObjectEvidence: map[string]string{},
		StickySlugs:    map[string]string{},
		EdgeFacts:      map[string][]asdEdgeFact{},
	}
	v.Shell = deriveASDShell(asdShellInput{})
	return v
}

// testDesignBridge satisfies DesignBridge for handler tests: the mutation
// is the real kernel; reads are canned empty envelopes.
type testDesignBridge struct{}

func (testDesignBridge) MutateDraft(ctx context.Context, start string, request draftmutation.Request, actor draftmutation.Actor) (draftmutation.Response, *draftmutation.Error) {
	return draftmutation.NewService().Mutate(ctx, start, request, actor)
}

func cannedRead() DesignReadOutcome { return DesignReadOutcome{JSON: []byte(`{}`)} }

func (testDesignBridge) GetBoard(context.Context, string, string) DesignReadOutcome {
	return cannedRead()
}
func (testDesignBridge) GetDesignContext(context.Context, string, string, []string) DesignReadOutcome {
	return cannedRead()
}
func (testDesignBridge) GetDesignCapabilities(context.Context, string, string) (DesignReadOutcome, *DesignCapabilitiesView) {
	return cannedRead(), nil
}
func (testDesignBridge) GetDesignProvenance(context.Context, string, string) DesignReadOutcome {
	return cannedRead()
}
func (testDesignBridge) PrepareDesignReview(context.Context, string, string) DesignReadOutcome {
	return cannedRead()
}

// mutateEnvelope builds one strict browser mutation envelope against the
// CURRENT spec bytes at root — exactly what the migrated client posts:
// the verdi.draftmutation/v1 request (base digest + bytes + resolved
// expected identity) plus the gesture's annotation routing.
func mutateEnvelope(t *testing.T, root, name string, operations []map[string]any, graduate, remove []string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, ".verdi", "specs", "active", name, "spec.md"))
	if err != nil {
		t.Fatal(err)
	}
	checkout, branch, head, err := resolveExpectedIdentity(context.Background(), root, name)
	if err != nil {
		t.Fatalf("resolving expected identity: %v", err)
	}
	request := map[string]any{
		"schema":        draftmutation.RequestSchema,
		"spec":          "spec/" + name,
		"base_digest":   draftmutation.DigestBytes(raw),
		"base_spec_b64": base64.StdEncoding.EncodeToString(raw),
		"expected":      map[string]any{"checkout": checkout, "branch": branch, "head": head},
		"operations":    operations,
	}
	envelope := map[string]any{"request": request}
	if len(graduate) > 0 {
		envelope["graduate_annotations"] = graduate
	}
	if len(remove) > 0 {
		envelope["delete_annotations"] = remove
	}
	encoded, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

// newBoardTestHandler is the migrated handler-test constructor: the full
// workbench handler with the kernel-backed test bridge wired.
func newBoardTestHandler(root string) http.Handler {
	return NewHandlerWith(root, Deps{Design: testDesignBridge{}})
}

// mutateOutcome decodes the mutation action's response union.
type mutateOutcome struct {
	Result     json.RawMessage `json:"result"`
	Stale      json.RawMessage `json:"stale"`
	Failure    *DesignFailure  `json:"failure"`
	Projection *struct {
		Dirty bool `json:"dirty"`
	} `json:"projection"`
	PostTransactionError string `json:"post_transaction_error"`
}

// postMutate posts one browser mutation envelope built from the CURRENT
// store state and decodes the outcome.
func postMutate(t *testing.T, h http.Handler, root, name string, operations []map[string]any, graduate, remove []string) (*httptest.ResponseRecorder, mutateOutcome) {
	t.Helper()
	body := mutateEnvelope(t, root, name, operations, graduate, remove)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/board/spec/"+name+"/api/mutate_draft", strings.NewReader(body))
	h.ServeHTTP(rec, req)
	var out mutateOutcome
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec, out
}
