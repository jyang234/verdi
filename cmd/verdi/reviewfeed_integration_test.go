package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/forge"
	"github.com/jyang234/verdi/internal/forge/fake"
	"github.com/jyang234/verdi/internal/workbench"
)

const reviewFeedIntegSpecName = "refi-decline-flow"

const reviewFeedIntegSpec = `---
id: spec/refi-decline-flow
kind: spec
class: feature
title: "Refinancing decline flow"
status: draft
owners: [platform-team]
problem: { text: "applicants act on stale decline reasons", anchor: "#problem" }
outcome: { text: "declined applicants see the current decline state", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "a declined applicant sees the current reason", evidence: [attestation], anchor: "#ac-1" }
  - { id: ac-2, text: "a reversed decline clears the notice", evidence: [attestation], anchor: "#ac-2" }
---
# body
`

// TestReviewFeed_Integration_ForgeFakeThroughBoard is W4's cross-phase
// integration test: it drives the board's review-mode HTTP surface
// (internal/workbench, V1-P6) with the CommentFeed backed by the REAL
// forgeCommentFeed adapter (reviewfeed.go, V1-P7) over the forge FAKE — not
// the canned file. It exercises the whole join the two phases meet at:
// design-branch → open-MR discovery, comment feed, and thread-resolution
// state, rendered into the board fragment. Seeds a token-bearing comment
// on a resolved thread, a token-free comment, and one resolved thread, then
// asserts the anchored (resolved) sticky rides its object's card and the
// token-free comment lands in the inbox tray.
func TestReviewFeed_Integration_ForgeFakeThroughBoard(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main") // adapter resolves the default branch hermetically (no git remote)

	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/verdi.yaml": "schema: verdi.layout/v1\n",
			".verdi/.gitignore": "data/\n",
			".verdi/specs/active/" + reviewFeedIntegSpecName + "/spec.md": reviewFeedIntegSpec,
		},
		Message: "seed a draft feature spec for the review-mode board",
	}})
	checkoutBranch(t, repo.Dir, "design/"+reviewFeedIntegSpecName)

	// The forge fake stands in for a live GitLab/GitHub (no network,
	// CLAUDE.md): one open MR whose source branch is this spec's design
	// branch, a token-bearing comment on a resolved thread, and a token-free
	// general comment.
	f := fake.New()
	f.SeedOpenMR("main", forge.OpenMR{ID: "77", SourceBranch: "design/" + reviewFeedIntegSpecName})
	f.SeedComment("77", forge.Comment{ID: "c1", Author: "alice", Body: "[vd:ac-1] please reword this to be observable", ThreadID: "t1"})
	f.SeedComment("77", forge.Comment{ID: "c2", Author: "bob", Body: "overall direction looks right"})
	f.SeedThreadResolution("77", forge.ThreadResolution{ThreadID: "t1", Resolved: true, ResolvedBy: "alice"})

	// The adapter under test — the real forge port over workbench.CommentFeed.
	feed := newForgeCommentFeed(f, repo.Dir)
	h := workbench.NewHandlerWith(repo.Dir, workbench.Deps{CommentFeed: feed})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/board/spec/"+reviewFeedIntegSpecName+"/fragment", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET fragment = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	// An open MR discovered through the adapter puts the board in review mode.
	if !strings.Contains(body, `data-board-mode="review"`) {
		t.Fatalf("board is not in review mode:\n%s", body)
	}

	// The token-bearing comment anchors to ac-1's card AND carries the
	// resolved state joined from its forge thread.
	if !strings.Contains(body, `data-anchor="ac-1"><span class="sticky-type">review · resolved`) {
		t.Errorf("anchored review sticky for ac-1 is missing or not marked resolved:\n%s", body)
	}
	// And it rides the object's card, not the tray.
	acCard := sliceBetween(body, `data-testid="card-ac-1"`, `data-testid="card-ac-2"`)
	if !strings.Contains(acCard, `data-annotation-type="review" data-anchor="ac-1"`) {
		t.Errorf("anchored comment does not ride ac-1's card:\n%s", acCard)
	}

	// The token-free comment is never dropped: it lands in the inbox tray.
	trayStart := strings.Index(body, `aria-label="Inbox tray"`)
	if trayStart < 0 {
		t.Fatal("no inbox tray rendered")
	}
	if !strings.Contains(body[trayStart:], "overall direction looks right") {
		t.Errorf("token-free comment missing from the inbox tray:\n%s", body[trayStart:])
	}
	// The whole feed is conserved: exactly the two seeded comments render.
	if got := strings.Count(body, `data-annotation-type="review"`); got != 2 {
		t.Errorf("review stickies = %d, want 2 (nothing dropped, nothing duplicated)", got)
	}
}

// sliceBetween returns the substring of s from the first occurrence of from
// up to (not including) the next occurrence of to; the tail from `from` if
// `to` is absent.
func sliceBetween(s, from, to string) string {
	i := strings.Index(s, from)
	if i < 0 {
		return ""
	}
	rest := s[i:]
	if j := strings.Index(rest, to); j >= 0 {
		return rest[:j]
	}
	return rest
}
