// Capture-shape fidelity tests: prove this adapter's decode types
// (reviewCommentJSON, issueCommentJSON, reviewThreadNode) parse S6's
// LIVE-VERIFIED GitHub captures (testdata/forge-captures/github/) exactly
// as the spike found them — a stronger claim than the generic contract
// suite (forgetest), which only proves round-trip behavior against
// synthetic seeded data. These tests replay the literal captured JSON
// bytes through an httptest server and assert the parsed fields match
// what S6's findings.md recorded by hand.
package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/forge"
)

// captureBytes reads one of S6's committed GitHub captures, live-verified
// per docs/spikes/v1/spike-s6-findings.md.
func captureBytes(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "forge-captures", "github", name))
	if err != nil {
		t.Fatalf("reading capture %s: %v", name, err)
	}
	return data
}

// captureServer serves fixed capture bytes for both the diff-comment REST
// list and the GraphQL reviewThreads query, verbatim, regardless of the
// requested mrID — enough to exercise ListComments/GetThreadResolution's
// decode paths against real captured bytes.
func captureServer(t *testing.T, diffCommentsBody, graphQLBody []byte) *Adapter {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/acme/svcfix/pulls/1/comments", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(diffCommentsBody)
	})
	mux.HandleFunc("/repos/acme/svcfix/issues/1/comments", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(captureBytes(t, "02-list-issue-comments-REST.json"))
	})
	mux.HandleFunc("/graphql", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(graphQLBody)
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return New(Config{BaseURL: ts.URL, Owner: "acme", Repo: "svcfix", HTTPClient: ts.Client()})
}

// TestGitHub_Captures_ListComments_TokenBodyByteIdentical proves
// ListComments decodes capture 01 (post-both-comments, live-verified)
// with the [vd:ac-2] token's body byte-identical and the token-free
// comment present too — S6 Q3's live finding, replayed through this
// adapter's actual decode path rather than a synthetic seed.
func TestGitHub_Captures_ListComments_TokenBodyByteIdentical(t *testing.T) {
	a := captureServer(t,
		captureBytes(t, "01-list-review-comments-REST.json"),
		captureBytes(t, "03-review-threads-GraphQL-before-resolve.json"),
	)
	comments, err := a.ListComments(context.Background(), "1")
	if err != nil {
		t.Fatalf("ListComments: %v", err)
	}
	if len(comments) != 3 { // 2 diff + 1 general
		t.Fatalf("ListComments = %d comments, want 3: %+v", len(comments), comments)
	}
	var sawToken, sawTokenFreeDiff, sawGeneral bool
	for _, c := range comments {
		switch {
		case c.Body == "[vd:ac-2] outcome AC reads implementation-scoped — reword?":
			sawToken = true
			if id, ok := forge.ParseCommentToken(c.Body); !ok || id != "ac-2" {
				t.Errorf("ParseCommentToken(%q) = (%q, %v), want (\"ac-2\", true)", c.Body, id, ok)
			}
			if c.ThreadID == "" {
				t.Error("token-bearing diff comment has no ThreadID (should join to a GraphQL reviewThreads node)")
			}
		case c.Body == "nit: this comment has no vd token, should land in the inbox tray":
			sawTokenFreeDiff = true
		case c.Body == "General PR conversation comment, not tied to a diff line at all.":
			sawGeneral = true
		}
	}
	if !sawToken || !sawTokenFreeDiff || !sawGeneral {
		t.Errorf("ListComments = %+v, missing one of the three captured comments", comments)
	}
}

// TestGitHub_Captures_ThreadResolution_BeforeAfterResolve proves
// GetThreadResolution decodes captures 03/04 (live-verified,
// before/after resolveReviewThread) matching S6's hand-recorded finding:
// isResolved false -> true, resolvedBy populated.
func TestGitHub_Captures_ThreadResolution_BeforeAfterResolve(t *testing.T) {
	before := captureServer(t,
		captureBytes(t, "01-list-review-comments-REST.json"),
		captureBytes(t, "03-review-threads-GraphQL-before-resolve.json"),
	)
	threads, err := before.GetThreadResolution(context.Background(), "1")
	if err != nil {
		t.Fatalf("GetThreadResolution (before): %v", err)
	}
	if len(threads) != 2 {
		t.Fatalf("GetThreadResolution (before) = %+v, want 2 threads", threads)
	}
	for _, tr := range threads {
		if tr.Resolved {
			t.Errorf("GetThreadResolution (before) thread %q = resolved, want unresolved: %+v", tr.ThreadID, tr)
		}
	}

	after := captureServer(t,
		captureBytes(t, "01-list-review-comments-REST.json"),
		captureBytes(t, "04-review-threads-GraphQL-after-resolve.json"),
	)
	threads, err = after.GetThreadResolution(context.Background(), "1")
	if err != nil {
		t.Fatalf("GetThreadResolution (after): %v", err)
	}
	var resolvedCount int
	for _, tr := range threads {
		if tr.Resolved {
			resolvedCount++
			if tr.ResolvedBy != "jyang234" {
				t.Errorf("resolved thread ResolvedBy = %q, want %q (capture 04)", tr.ResolvedBy, "jyang234")
			}
		}
	}
	if resolvedCount != 1 {
		t.Fatalf("GetThreadResolution (after) resolved count = %d, want exactly 1 (capture 04: one of two threads resolved)", resolvedCount)
	}
}

// TestGitHub_Captures_TokenSurvivesForcePush proves the [vd:ac-2] token's
// body stays byte-identical even in capture 09/10 (live-verified,
// after force-push): position is lost (line nulled) but the body — and
// therefore the token — is untouched, and isResolved/isOutdated read
// correctly (S6 Q4's live finding).
func TestGitHub_Captures_TokenSurvivesForcePush(t *testing.T) {
	a := captureServer(t,
		captureBytes(t, "09-list-review-comments-REST-after-force-push.json"),
		captureBytes(t, "10-review-threads-GraphQL-after-force-push.json"),
	)
	comments, err := a.ListComments(context.Background(), "1")
	if err != nil {
		t.Fatalf("ListComments: %v", err)
	}
	var found bool
	for _, c := range comments {
		if c.Body != "[vd:ac-2] outcome AC reads implementation-scoped — reword?" {
			continue
		}
		found = true
		if c.Line != 0 {
			t.Errorf("post-force-push Line = %d, want 0 (position lost, capture 09: line: null)", c.Line)
		}
	}
	if !found {
		t.Fatal("token-bearing comment's body did not survive the force-push capture byte-identical")
	}

	threads, err := a.GetThreadResolution(context.Background(), "1")
	if err != nil {
		t.Fatalf("GetThreadResolution: %v", err)
	}
	var sawResolvedSurvivingForcePush bool
	for _, tr := range threads {
		if tr.Resolved {
			sawResolvedSurvivingForcePush = true
		}
	}
	if !sawResolvedSurvivingForcePush {
		t.Error("capture 10: resolution should survive the force-push (isResolved stays true) even though position is lost")
	}
}
