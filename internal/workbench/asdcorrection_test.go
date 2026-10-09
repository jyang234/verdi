package workbench

// Codex correction (round 1) coverage — Wave 6 Task 2.
//
//   - Finding 1: a capabilities posture must never be served stale
//     across an accepted-head advance (an owner merge advances the
//     accepted head while checkout, spec bytes, and policy stay fixed —
//     the wall turns read-only accepted, and a cached pre-merge posture
//     would misstate what may write), and an operational failure must
//     never be served from a memo (closure N-1, the I-2 poisoning shape).
//     The wall shell's capabilities memo retired with the shell (SI-368
//     (30)(c), (32) T3): the wall consults no capabilities, and the
//     Readiness tab consults them afresh on every open, its cost recorded
//     as BL-196's. Closure reopen: the accepted head must be resolved at
//     the AUTHORITATIVE default-branch rev (specstate.Branch.Ref —
//     origin/<name> when the remote-tracking ref exists, the projector's
//     own preference), never the local branch NAME: in the ordinary
//     cloned shape acceptance moves on origin/<name> while
//     refs/heads/<name> stays a stale shadow.
//   - Finding 2: a projection failure AFTER designapp.MutateDraft landed
//     must disclose the landed transaction, any post-transaction
//     disclosures, and the projection failure itself, classified
//     operationally — never a generic 500 that makes a durable mutation
//     look unapplied (§4.3: no partial action effect may be hidden).

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/jyang234/verdi/internal/gitx"
)

// scriptedCapsBridge is testDesignBridge with a scripted, call-counted
// GetDesignCapabilities: each consultation is observable, so a call count
// that does not advance is a first-class assertion.
type scriptedCapsBridge struct {
	testDesignBridge
	mu     sync.Mutex
	calls  int
	script func(call int) (DesignReadOutcome, *DesignCapabilitiesView)
}

func (b *scriptedCapsBridge) GetDesignCapabilities(context.Context, string, string) (DesignReadOutcome, *DesignCapabilitiesView) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.calls++
	return b.script(b.calls)
}

func (b *scriptedCapsBridge) callCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.calls
}

// advanceRef advances one git ref (the checkout sits on the design
// branch, so neither refs/heads/main nor a remote-tracking ref is ever
// checked out) by one commit carrying that ref's SAME tree: the
// owner-merge shape as seen from an untouched design checkout. The
// worktree, its branch, its HEAD, the spec bytes, and the policy tree
// all stay byte-identical; only the advanced ref moves. Dates are pinned
// so the fixture stays deterministic across runs.
func advanceRef(t *testing.T, dir, ref string) {
	t.Helper()
	commit := exec.Command("git", "-C", dir, "commit-tree", ref+"^{tree}", "-p", ref, "-m", "advance accepted head")
	commit.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_AUTHOR_DATE=2026-01-02T03:04:05Z",
		"GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid", "GIT_COMMITTER_DATE=2026-01-02T03:04:05Z",
	)
	out, err := commit.Output()
	if err != nil {
		t.Fatalf("commit-tree %s: %v", ref, err)
	}
	sha := strings.TrimSpace(string(out))
	if raw, err := exec.Command("git", "-C", dir, "update-ref", ref, sha).CombinedOutput(); err != nil {
		t.Fatalf("update-ref %s %s: %v\n%s", ref, sha, err, raw)
	}
}

// revParse resolves one ref in the fixture repo.
func revParse(t *testing.T, root, ref string) string {
	t.Helper()
	head, err := gitx.RevParse(context.Background(), root, ref)
	if err != nil {
		t.Fatalf("rev-parse %s: %v", ref, err)
	}
	return head
}

// --- Finding 1: stale capabilities after an accepted-branch advance ------

// TestCachedCapabilities_RefreshesOnAcceptedHeadAdvance reproduces the
// Codex witness in unit form, in BOTH default-branch authority shapes
// (specstate.Branch.Ref): the hermetic no-origin fixture resolves the
// LOCAL branch; the ordinary cloned shape resolves origin/<name>, which
// the state projector deliberately prefers because local refs can be a
// stale shadow. In each shape acceptance advances on the AUTHORITATIVE
// ref while the design checkout, its HEAD, the spec bytes, and the
// policy tree are all held fixed — and the second render's accepted-head
// facts (posture-header AcceptedHead, ahead/behind) must follow that ref,
// while the capabilities posture the wall shows, in its Readiness tab's
// policy guide, is consulted afresh (SI-368 (32) T3).
func TestCachedCapabilities_RefreshesOnAcceptedHeadAdvance(t *testing.T) {
	t.Run("local-branch authority (no origin ref)", func(t *testing.T) {
		root := newBoardFixture(t)
		assertAcceptedAdvanceRefreshes(t, root, "refs/heads/main", "main")
	})
	t.Run("remote-tracking authority (origin ref advances)", func(t *testing.T) {
		root := newBoardFixture(t)
		// The ordinary cloned shape: refs/remotes/origin/main exists, so
		// the resolved default-branch REF is origin/main while the local
		// main ref stays behind as a possibly-stale shadow (closure
		// reopen of finding 1 — the reviewer's overlay probe).
		if raw, err := exec.Command("git", "-C", root, "update-ref", "refs/remotes/origin/main", "refs/heads/main").CombinedOutput(); err != nil {
			t.Fatalf("creating refs/remotes/origin/main: %v\n%s", err, raw)
		}
		assertAcceptedAdvanceRefreshes(t, root, "refs/remotes/origin/main", "origin/main")
	})
}

// forbiddenCaps is a capabilities consultation refused policy-forbidden
// with detail.
func forbiddenCaps(detail string) (DesignReadOutcome, *DesignCapabilitiesView) {
	return DesignReadOutcome{Failure: &DesignFailure{Classification: "verdict", Code: "policy-forbidden", Detail: detail}}, nil
}

// openGuide GETs the wall's Readiness tab on h and returns the policy
// guide's variant it carries, "" for none.
func openGuide(t *testing.T, h http.Handler, name string) string {
	t.Helper()
	rec := tabGet(t, t.Context(), h, "/board/spec/"+name+"/readiness")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s's tab = %d\n%s", name, rec.Code, rec.Body.String())
	}
	m := regexp.MustCompile(`data-policy-guide="([^"]*)"`).FindStringSubmatch(rec.Body.String())
	if m == nil {
		return ""
	}
	return m[1]
}

// assertAcceptedAdvanceRefreshes renders once, advances ONLY the named
// authority ref (same tree), renders again, and asserts the second
// render's accepted-head facts ride the authoritative ref, that neither
// render consults capabilities (the wall shows none: F3CR-7), and that
// the Readiness tab opened after the advance shows the fresh posture, not
// the one before it.
func assertAcceptedAdvanceRefreshes(t *testing.T, root, advanceable, authorityRef string) {
	t.Helper()
	bridge := &scriptedCapsBridge{script: func(call int) (DesignReadOutcome, *DesignCapabilitiesView) {
		if call == 1 {
			return forbiddenCaps(policyNotAdoptedDetail)
		}
		return forbiddenCaps(noDesignAssistanceDetail)
	}}
	s := &boardSpecServer{root: root, design: bridge}
	h := NewHandlerWith(root, Deps{Design: bridge})
	ctx := context.Background()

	_, _, first, err := s.loadASD(ctx, boardFixtureName)
	if err != nil {
		t.Fatalf("first loadASD: %v", err)
	}
	if got := bridge.callCount(); got != 0 {
		t.Fatalf("the wall's render consulted capabilities %d times, want none (F3CR-7)", got)
	}
	if !first.AheadBehindKnown {
		t.Fatal("first render resolved no ahead/behind; the fixture cannot witness the counts riding the authoritative ref")
	}
	if got := openGuide(t, h, boardFixtureName); got != string(policyGuideNotAdopted) {
		t.Fatalf("the tab before the advance shows guide %q, want the scripted %q", got, policyGuideNotAdopted)
	}

	advanceRef(t, root, advanceable)
	authorityHead := revParse(t, root, authorityRef)
	if authorityHead == first.AcceptedHead {
		t.Fatalf("the authoritative ref %s did not advance (%q); the fixture cannot exercise the advance", authorityRef, authorityHead)
	}

	_, _, second, err := s.loadASD(ctx, boardFixtureName)
	if err != nil {
		t.Fatalf("second loadASD: %v", err)
	}
	// Fixture premise witnesses: ONLY the authoritative ref moved.
	if second.WorktreeHead != first.WorktreeHead || second.Branch != first.Branch || second.BaseDigest != first.BaseDigest {
		t.Fatalf("fixture moved more than the accepted head: worktree %q->%q branch %q->%q digest %q->%q",
			first.WorktreeHead, second.WorktreeHead, first.Branch, second.Branch, first.BaseDigest, second.BaseDigest)
	}
	// The accepted head is the AUTHORITATIVE ref's — the same rev the
	// state projector reads accepted bytes at — never a stale local
	// shadow (the posture header prints exactly this value).
	if second.AcceptedHead != authorityHead {
		t.Fatalf("render 2 AcceptedHead = %q, want the authoritative %s head %q — the posture header prints a stale accepted head", second.AcceptedHead, authorityRef, authorityHead)
	}
	// Ahead/behind rides the same ref: one same-tree commit landed on it.
	if !second.AheadBehindKnown || second.Behind != first.Behind+1 {
		t.Fatalf("render 2 behind = %d (known %v), want %d: ahead/behind is not counted against the authoritative ref", second.Behind, second.AheadBehindKnown, first.Behind+1)
	}
	// The posture must be FRESH, never the one before the advance.
	if got := openGuide(t, h, boardFixtureName); got != string(policyGuideNoDesignAssistance) {
		t.Fatalf("the tab after the advance shows guide %q, want the fresh %q — a stale posture was served", got, policyGuideNoDesignAssistance)
	}
	if got := bridge.callCount(); got != 2 {
		t.Fatalf("capabilities consultations = %d, want 2: one per tab open, none by the wall's renders", got)
	}
}

// TestCachedCapabilities_RetriesAfterOperationalFailure pins closure N-1:
// an operational capabilities failure is answered to that request alone
// and never served again — the next open with identical facts consults
// afresh (the review-fix I-2 shape). With the memo retired (SI-368 (32)
// T3), every open of the Readiness tab consults exactly once, success or
// failure, and nothing is served from a memo.
func TestCachedCapabilities_RetriesAfterOperationalFailure(t *testing.T) {
	root := newBoardFixture(t)
	bridge := &scriptedCapsBridge{script: func(call int) (DesignReadOutcome, *DesignCapabilitiesView) {
		if call == 1 {
			return DesignReadOutcome{Failure: &DesignFailure{Classification: "operational", Code: "io-failure", Detail: "transient: simulated first-consultation failure"}}, nil
		}
		return forbiddenCaps(policyNotAdoptedDetail)
	}}
	h := NewHandlerWith(root, Deps{Design: bridge})

	if got := openGuide(t, h, boardFixtureName); got != "" {
		t.Fatalf("the tab shows guide %q on an operational failure, want none", got)
	}
	if got := openGuide(t, h, boardFixtureName); got != string(policyGuideNotAdopted) {
		t.Fatalf("the second open shows guide %q, want the retried consultation's %q", got, policyGuideNotAdopted)
	}
	if got := bridge.callCount(); got != 2 {
		t.Fatalf("capabilities consultations = %d, want 2: the operational failure was served again instead of retried", got)
	}
	if got := openGuide(t, h, boardFixtureName); got != string(policyGuideNotAdopted) || bridge.callCount() != 3 {
		t.Fatalf("the third open shows guide %q after %d consultations, want %q after 3: each open consults afresh", got, bridge.callCount(), policyGuideNotAdopted)
	}
}

// --- Finding 2: projection failure after a landed transaction ------------

// TestMutateDraft_ProjectionFailureDisclosesLandedTransaction pins §4.3's
// "no partial action effect may be hidden" for the window Codex probed:
// the kernel transaction and every post-transaction effect have committed
// when the fresh-projection render fails. The response must disclose the
// landed result and classify the projection failure operationally — never
// a generic 500 that presents the durable mutation as unapplied.
func TestMutateDraft_ProjectionFailureDisclosesLandedTransaction(t *testing.T) {
	root := newBoardFixture(t)
	h := newBoardTestHandler(root)

	injected := errors.New("injected: state resolution failed after the transaction committed")
	hook := func(string) error { return injected }
	mutationSnapshotTestHook.Store(&hook)
	t.Cleanup(func() { mutationSnapshotTestHook.Store(nil) })

	const marker = "projection failure discloses the landed write [f2-witness]"
	rec, out := postMutate(t, h, root, boardFixtureName, []map[string]any{
		{"op": "edit-ac", "id": "ac-1", "text": marker, "evidence": []string{"attestation"}, "anchor": "#ac-1"},
	}, nil, nil)

	// The transaction LANDED: the spec bytes on disk carry the edit.
	raw, err := os.ReadFile(filepath.Join(root, ".verdi", "specs", "active", boardFixtureName, "spec.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "[f2-witness]") {
		t.Fatalf("the edit did not land; this fixture cannot witness the disclosure contract:\n%s", raw)
	}

	// The overall posture is operational (the projection failed)…
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 (operational posture)\n%s", rec.Code, rec.Body.String())
	}
	// …but the landed transaction is DISCLOSED, never hidden.
	if out.Result == nil {
		t.Fatalf("response hides the landed transaction result (a durable mutation presented as unapplied):\n%s", rec.Body.String())
	}
	var result struct {
		Changes []struct {
			Target string `json:"target"`
			Change string `json:"change"`
		} `json:"changes"`
	}
	if err := json.Unmarshal(out.Result, &result); err != nil {
		t.Fatalf("decoding disclosed result: %v\n%s", err, rec.Body.String())
	}
	if len(result.Changes) == 0 {
		t.Fatalf("disclosed result names no applied change:\n%s", rec.Body.String())
	}
	// The projection failure itself is typed and classified operationally.
	if out.ProjectionFailure == nil {
		t.Fatalf("response carries no typed projection_failure disclosure:\n%s", rec.Body.String())
	}
	if out.ProjectionFailure.Classification != "operational" {
		t.Fatalf("projection_failure classification = %q, want %q", out.ProjectionFailure.Classification, "operational")
	}
	if !strings.Contains(out.ProjectionFailure.Detail, "injected: state resolution failed") {
		t.Fatalf("projection_failure detail %q does not carry the underlying failure", out.ProjectionFailure.Detail)
	}
	// And no fresh projection is claimed (§4.3: never a favorable body).
	if out.Projection != nil {
		t.Fatalf("response claims a fresh projection although rendering it failed:\n%s", rec.Body.String())
	}
}

// TestMutateDraft_ProjectionFailureCarriesPostTransactionDisclosure pins
// the (b) half of the disclosure triple: a post-transaction follow-up
// failure recorded before the projection render is preserved beside the
// landed result and the projection failure, not dropped with them.
func TestMutateDraft_ProjectionFailureCarriesPostTransactionDisclosure(t *testing.T) {
	root := newBoardFixture(t)
	h := newBoardTestHandler(root)

	hook := func(string) error { return errors.New("injected: projection render refused") }
	mutationSnapshotTestHook.Store(&hook)
	t.Cleanup(func() { mutationSnapshotTestHook.Store(nil) })

	// Make the annotations dir unwritable so the graduation follow-up
	// fails AFTER the clean transaction (the disclosed post_transaction
	// shape), then restore it for cleanup.
	// A sticky must exist first, while the zone is still writable.
	rec := postBoardAPI(t, h, boardFixtureName, "sticky", `{"text":"post-transaction disclosure probe","type":"comment"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("seeding sticky = %d\n%s", rec.Code, rec.Body.String())
	}
	proj, _, _, _, err := (&boardSpecServer{root: root}).loadBoard(context.Background(), boardFixtureName)
	if err != nil {
		t.Fatal(err)
	}
	if len(proj.Stickies) == 0 {
		t.Fatal("no sticky on the board after seeding")
	}
	stickyID := proj.Stickies[0].ID
	annotations := filepath.Join(root, ".verdi", "data", "mutable", "annotations")
	if err := os.Chmod(annotations, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(annotations, 0o755) })

	mrec, out := postMutate(t, h, root, boardFixtureName, []map[string]any{
		{"op": "add-ac", "id": "ac-9", "text": "graduated with a failing follow-up", "evidence": []string{"attestation"}, "anchor": "#ac-9"},
	}, []string{stickyID}, nil)
	if mrec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500\n%s", mrec.Code, mrec.Body.String())
	}
	if out.Result == nil || out.ProjectionFailure == nil {
		t.Fatalf("landed result and projection failure must both be disclosed:\n%s", mrec.Body.String())
	}
	if !strings.Contains(out.PostTransactionError, "graduating annotations") {
		t.Fatalf("post_transaction_error %q was dropped alongside the projection", out.PostTransactionError)
	}
}
