// Capture-shape fidelity tests, DOC-DERIVED, UNVERIFIED AGAINST LIVE
// (gitlab.go's package doc note; S6 disclosure carried forward verbatim
// here and in every test name below): prove this adapter's decode types
// (noteJSON, discussionJSON) parse S6's doc-derived GitLab captures
// (testdata/forge-captures/gitlab/) as documented at
// https://docs.gitlab.com/ee/api/discussions.html. Unlike the GitHub
// sibling file, these tests prove adapter-vs-DOCUMENTED-shape, never
// adapter-vs-live — no GitLab credentials existed in the build
// environment (S6 findings.md), so this file's names say so explicitly
// rather than reading as if they carried the same weight as the GitHub
// captures.
package gitlab

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/forge"
)

func docDerivedCaptureBytes(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "forge-captures", "gitlab", name))
	if err != nil {
		t.Fatalf("reading doc-derived capture %s: %v", name, err)
	}
	return data
}

// TestGitLab_DocDerivedUNVERIFIED_ListComments_MatchesDocumentedShape
// proves ListComments decodes capture 01 (doc-derived, UNVERIFIED against
// live — assembled from GitLab's own published API docs, never a real
// response) with both a token-bearing DiffNote and a token-free
// individual note present, matching the documented field shapes
// (`individual_note`, `resolvable`, `position.new_line`).
func TestGitLab_DocDerivedUNVERIFIED_ListComments_MatchesDocumentedShape(t *testing.T) {
	// The capture's example_response is nested under a top-level object,
	// not a bare array — extract it the way a real GitLab client would
	// see the endpoint's actual response (the capture wraps it with
	// _capture_status/_source/_note metadata for the spike write-up only).
	var wrapper struct {
		ExampleResponse []discussionJSON `json:"example_response"`
	}
	data := docDerivedCaptureBytes(t, "01-doc-derived-UNVERIFIED-list-discussions.json")
	if err := json.Unmarshal(data, &wrapper); err != nil {
		t.Fatalf("decoding capture: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/projects/42/merge_requests/1/discussions", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(wrapper.ExampleResponse)
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	a := New(Config{BaseURL: ts.URL, ProjectID: "42", HTTPClient: ts.Client()})

	comments, err := a.ListComments(context.Background(), "1")
	if err != nil {
		t.Fatalf("ListComments: %v", err)
	}
	if len(comments) != 2 {
		t.Fatalf("ListComments = %d comments, want 2 (doc-derived capture has one DiffNote + one individual note): %+v", len(comments), comments)
	}
	var sawToken, sawUnanchored bool
	for _, c := range comments {
		if id, ok := forge.ParseCommentToken(c.Body); ok {
			sawToken = true
			if id != "ac-2" {
				t.Errorf("ParseCommentToken = %q, want ac-2", id)
			}
			if c.ThreadID == "" {
				t.Error("doc-derived DiffNote (resolvable) has no ThreadID")
			}
		} else {
			sawUnanchored = true
			if c.ThreadID != "" {
				t.Errorf("doc-derived individual note has ThreadID %q, want \"\"", c.ThreadID)
			}
		}
	}
	if !sawToken || !sawUnanchored {
		t.Errorf("ListComments = %+v, want both a token-bearing and a token-free comment (doc-derived shape)", comments)
	}
}

// TestGitLab_DocDerivedUNVERIFIED_ThreadResolution_MatchesDocumentedShape
// proves GetThreadResolution decodes capture 03 (doc-derived, UNVERIFIED
// against live — the documented PUT .../discussions/:id?resolved=true
// response shape) with resolved/resolved_by read correctly, and that the
// individual (non-resolvable) note from capture 01 never appears here.
func TestGitLab_DocDerivedUNVERIFIED_ThreadResolution_MatchesDocumentedShape(t *testing.T) {
	var resolvedWrapper struct {
		ExampleResponse discussionJSON `json:"example_response"`
	}
	if err := json.Unmarshal(docDerivedCaptureBytes(t, "03-doc-derived-UNVERIFIED-resolve-discussion-response.json"), &resolvedWrapper); err != nil {
		t.Fatalf("decoding capture: %v", err)
	}

	var listWrapper struct {
		ExampleResponse []discussionJSON `json:"example_response"`
	}
	if err := json.Unmarshal(docDerivedCaptureBytes(t, "01-doc-derived-UNVERIFIED-list-discussions.json"), &listWrapper); err != nil {
		t.Fatalf("decoding capture: %v", err)
	}
	// Replace the (unresolved) diff discussion with the resolved shape
	// from capture 03, keeping capture 01's individual note alongside it —
	// mirrors calling GET .../discussions again after a PUT resolve.
	discussions := []discussionJSON{resolvedWrapper.ExampleResponse, listWrapper.ExampleResponse[1]}

	mux := http.NewServeMux()
	mux.HandleFunc("/projects/42/merge_requests/1/discussions", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(discussions)
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	a := New(Config{BaseURL: ts.URL, ProjectID: "42", HTTPClient: ts.Client()})

	threads, err := a.GetThreadResolution(context.Background(), "1")
	if err != nil {
		t.Fatalf("GetThreadResolution: %v", err)
	}
	if len(threads) != 1 {
		t.Fatalf("GetThreadResolution = %+v, want exactly 1 (the individual note is not a substantive thread)", threads)
	}
	if !threads[0].Resolved || threads[0].ResolvedBy != "spike-s6" {
		t.Errorf("GetThreadResolution[0] = %+v, want Resolved=true ResolvedBy=\"spike-s6\" (doc-derived capture 03)", threads[0])
	}
}
