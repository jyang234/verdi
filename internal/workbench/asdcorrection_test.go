package workbench

// Codex correction (round 1) coverage — Wave 6 Task 2.
//
//   - Finding 1: the capabilities memo key must include the accepted/
//     default-branch head identity (an owner merge advances the accepted
//     head while checkout, spec bytes, and policy stay fixed — the wall
//     turns read-only accepted, and a cached pre-merge Mutable:true
//     posture would claim delegated agents can write on a sealed wall),
//     and operational failures must never be memoized (closure N-1, the
//     I-2 poisoning shape: cache successes only).
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
	"strings"
	"sync"
	"testing"
)

// scriptedCapsBridge is testDesignBridge with a scripted, call-counted
// GetDesignCapabilities: each render's consultation is observable, so a
// cache hit (call count not advancing) is a first-class assertion.
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

// advanceDefaultBranch advances the fixture's resolved default branch
// (the local `main` ref — buildAuthoringFixture leaves the checkout on
// the design branch, so refs/heads/main is safe to move) by one commit
// carrying main's SAME tree: the owner-merge shape as seen from an
// untouched design checkout. The worktree, its branch, its HEAD, the
// spec bytes, and the policy tree all stay byte-identical; only the
// accepted head moves. Dates are pinned so the fixture stays
// deterministic across runs.
func advanceDefaultBranch(t *testing.T, dir string) {
	t.Helper()
	commit := exec.Command("git", "-C", dir, "commit-tree", "main^{tree}", "-p", "main", "-m", "advance accepted head")
	commit.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_AUTHOR_DATE=2026-01-02T03:04:05Z",
		"GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid", "GIT_COMMITTER_DATE=2026-01-02T03:04:05Z",
	)
	out, err := commit.Output()
	if err != nil {
		t.Fatalf("commit-tree: %v", err)
	}
	sha := strings.TrimSpace(string(out))
	if raw, err := exec.Command("git", "-C", dir, "update-ref", "refs/heads/main", sha).CombinedOutput(); err != nil {
		t.Fatalf("update-ref refs/heads/main %s: %v\n%s", sha, err, raw)
	}
}

// --- Finding 1: stale capabilities after an accepted-branch advance ------

// TestCachedCapabilities_RefreshesOnAcceptedHeadAdvance reproduces the
// Codex witness in unit form: the accepted head advances while the design
// checkout, its HEAD, the spec bytes, and the policy tree are all held
// fixed. The second render's agent posture must be freshly consulted —
// a memo key that omits the accepted head serves the pre-advance
// Mutable:true posture on a wall whose authority state moved.
func TestCachedCapabilities_RefreshesOnAcceptedHeadAdvance(t *testing.T) {
	root := newBoardFixture(t)
	bridge := &scriptedCapsBridge{script: func(call int) (DesignReadOutcome, *DesignCapabilitiesView) {
		if call == 1 {
			return DesignReadOutcome{JSON: []byte(`{}`)}, &DesignCapabilitiesView{Mutable: true, PolicyMode: "draft-write", PolicyDigest: "sha256:caps-1"}
		}
		return DesignReadOutcome{JSON: []byte(`{}`)}, &DesignCapabilitiesView{Mutable: false, RefusalPrecondition: "policy-mode", RefusalDetail: "design_assistance mode off forbids delegated-agent writes", PolicyMode: "off", PolicyDigest: "sha256:caps-2"}
	}}
	s := &boardSpecServer{root: root, design: bridge}
	ctx := context.Background()

	_, _, first, err := s.loadASD(ctx, boardFixtureName)
	if err != nil {
		t.Fatalf("first loadASD: %v", err)
	}
	if first.Caps == nil || !first.Caps.Mutable {
		t.Fatalf("first render Caps = %+v, want the scripted Mutable:true posture", first.Caps)
	}

	advanceDefaultBranch(t, root)

	_, _, second, err := s.loadASD(ctx, boardFixtureName)
	if err != nil {
		t.Fatalf("second loadASD: %v", err)
	}
	// Fixture premise witnesses: ONLY the accepted head moved.
	if second.AcceptedHead == first.AcceptedHead {
		t.Fatalf("accepted head did not advance (%q); the fixture cannot exercise the memo key", second.AcceptedHead)
	}
	if second.WorktreeHead != first.WorktreeHead || second.Branch != first.Branch || second.BaseDigest != first.BaseDigest {
		t.Fatalf("fixture moved more than the accepted head: worktree %q->%q branch %q->%q digest %q->%q",
			first.WorktreeHead, second.WorktreeHead, first.Branch, second.Branch, first.BaseDigest, second.BaseDigest)
	}
	// The posture must be FRESH, not the cached pre-advance one.
	if got := bridge.callCount(); got != 2 {
		t.Fatalf("capabilities consultations = %d, want 2: the memo served a stale posture across an accepted-head advance", got)
	}
	if second.Caps == nil || second.Caps.Mutable {
		t.Fatalf("second render Caps = %+v, want the fresh Mutable:false policy-mode refusal — the wall claims delegated agents can write on a wall whose accepted state moved", second.Caps)
	}
	if second.Caps.RefusalPrecondition != "policy-mode" {
		t.Fatalf("second render RefusalPrecondition = %q, want the fresh %q", second.Caps.RefusalPrecondition, "policy-mode")
	}
}

// TestCachedCapabilities_RetriesAfterOperationalFailure pins closure N-1:
// an operational capabilities failure is returned to that render alone
// and never memoized — the next render with identical facts re-consults
// (the review-fix I-2 shape, applied to the capabilities memo). The
// following render then proves the SUCCESS is cached.
func TestCachedCapabilities_RetriesAfterOperationalFailure(t *testing.T) {
	root := newBoardFixture(t)
	bridge := &scriptedCapsBridge{script: func(call int) (DesignReadOutcome, *DesignCapabilitiesView) {
		if call == 1 {
			return DesignReadOutcome{Failure: &DesignFailure{Classification: "operational", Code: "io-failure", Detail: "transient: simulated first-consultation failure"}}, nil
		}
		return DesignReadOutcome{JSON: []byte(`{}`)}, &DesignCapabilitiesView{Mutable: true, PolicyMode: "draft-write", PolicyDigest: "sha256:caps-ok"}
	}}
	s := &boardSpecServer{root: root, design: bridge}
	ctx := context.Background()

	_, _, first, err := s.loadASD(ctx, boardFixtureName)
	if err != nil {
		t.Fatalf("first loadASD: %v", err)
	}
	if first.CapsFailure == nil || first.CapsFailure.Code != "io-failure" {
		t.Fatalf("first render CapsFailure = %+v, want the scripted io-failure", first.CapsFailure)
	}

	_, _, second, err := s.loadASD(ctx, boardFixtureName)
	if err != nil {
		t.Fatalf("second loadASD: %v", err)
	}
	if got := bridge.callCount(); got != 2 {
		t.Fatalf("capabilities consultations = %d, want 2: the operational failure was served from the memo instead of being retried", got)
	}
	if second.CapsFailure != nil || second.Caps == nil || !second.Caps.Mutable {
		t.Fatalf("second render = (caps %+v, failure %+v), want the retried Mutable:true success", second.Caps, second.CapsFailure)
	}

	// The success IS memoized: a third identical render answers from the
	// cache without a fourth consultation.
	_, _, third, err := s.loadASD(ctx, boardFixtureName)
	if err != nil {
		t.Fatalf("third loadASD: %v", err)
	}
	if got := bridge.callCount(); got != 2 {
		t.Fatalf("capabilities consultations after the cached success = %d, want still 2", got)
	}
	if third.Caps == nil || !third.Caps.Mutable {
		t.Fatalf("third render Caps = %+v, want the cached success", third.Caps)
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
