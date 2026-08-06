package execworkspace

// Materialize implements spec/execution-workspace §Exact workspace
// materialization's ordered state machine (steps 1 through 6, with
// sub-branches 3a/3b/4a/4b/4c) and its idempotent crash recovery — ledger
// SI-17. The step numbers in the comments below refer to that section's
// numbered list, quoted where it disambiguates an edge case.
//
// The target-specific Git worktree-registry reconciliation the spec calls
// for at step 2 and step 4c ("RECONCILING THE REGISTRY FOR A UNIT") is
// deliberately NOT implemented here: it is consumed through the Reconciler
// port (controller decision AD-4), and its real
// enumerate/resolve/claim/delete/re-verify implementation is a later
// lane's concern (spec §Implementation seam).

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/jyang234/verdi/internal/filelock"
	"github.com/jyang234/verdi/internal/gitx"
)

// Reconciler performs target-specific Git worktree-registry reconciliation
// for exactly one execution-workspace unit (spec §Workspace naming,
// "RECONCILING THE REGISTRY FOR A UNIT ... is therefore TARGET-SPECIFIC"):
// clearing any stale administrative entry whose resolved worktree path is
// unitPath, under repoRoot's $GIT_COMMON_DIR, and establishing the spec's
// re-enumeration postcondition (no surviving entry resolving to unitPath)
// before returning. A reconciliation that cannot establish that
// postcondition — a lock-marker refusal, an unresolved entry, or any other
// failure — returns a non-nil error; it never reports success by exit
// status alone.
//
// This is a consumer-side port (controller decision AD-4, the 04 §port
// pattern: interfaces are defined at the consumer). The real
// implementation is a later lane's concern; Materialize depends only on
// this interface, and tests in this package exercise it through hermetic
// fakes.
type Reconciler interface {
	ReconcileUnit(ctx context.Context, repoRoot, unitPath string) error
}

// Materializer runs spec/execution-workspace's materialization state
// machine over one store root's data/execution/ tree, cutting worktrees
// against one git repository root. Construct with NewMaterializer, which
// refuses a nil Reconciler outright — there is no silent no-op
// reconciliation mode.
type Materializer struct {
	storeRoot  string
	repoRoot   string
	reconciler Reconciler
}

// NewMaterializer builds a Materializer over storeRoot (the directory
// containing .verdi/, under which data/execution/ lives — see
// internal/execworkspace's ExecutionRoot) and repoRoot (the git repository
// worktrees are cut against). reconciler must not be nil: this
// Materializer performs Git worktree-registry mutation at steps 2 and 4c
// and refuses to silently skip the reconciliation those steps require
// (spec §Workspace naming: "Both absent-unit branches ... RECONCILE THE
// REGISTRY FOR A UNIT under that unit's lock before proceeding").
func NewMaterializer(storeRoot, repoRoot string, reconciler Reconciler) (*Materializer, error) {
	if reconciler == nil {
		return nil, fmt.Errorf("execworkspace: NewMaterializer: reconciler must not be nil")
	}
	return &Materializer{storeRoot: storeRoot, repoRoot: repoRoot, reconciler: reconciler}, nil
}

// Request is one materialization request. Identity carries both request
// shapes' full identity (Identity.Shape distinguishes them). PatchBytes is
// REQUIRED for the BasePlusPatch shape and must be the SAME canonical
// bytes Identity's PatchSHA256 was digested from (controller decision
// AD-6: "the bytes are canonical as supplied, digest and apply exactly
// those") — Materialize verifies this by re-hashing PatchBytes and
// comparing against Identity.PatchSHA256, rather than trusting the two to
// already agree. PatchBytes must be empty for the ExactSHA shape.
type Request struct {
	Identity   Identity
	PatchBytes []byte
}

// Outcome distinguishes a fresh materialization from an idempotent reuse
// (spec §Exact workspace materialization: "a request landing on an
// already-existing <workspace-id> is verified against that sidecar before
// any reuse").
type Outcome int

const (
	// OutcomeUnknown is the zero value, never returned alongside a nil
	// error — the same discipline PathUnknown/PathKind uses.
	OutcomeUnknown Outcome = iota
	// OutcomeMaterialized means this call cut the worktree and wrote the
	// completion witness (fresh materialization, including the step-4c
	// rebuild-after-incomplete-residue case).
	OutcomeMaterialized
	// OutcomeReused means an already-complete, identity-equal
	// materialization existed and was returned as-is (step 4a's "equal"
	// branch): no git call was made.
	OutcomeReused
)

// String renders Outcome for diagnostics.
func (o Outcome) String() string {
	switch o {
	case OutcomeMaterialized:
		return "materialized"
	case OutcomeReused:
		return "reused"
	default:
		return fmt.Sprintf("execworkspace.Outcome(%d)", int(o))
	}
}

// Result is Materialize's success value: which workspace, at which path,
// fresh or reused, plus every disclosed line produced along the way
// (orphan-sibling deletions, residue rebuilds, registry reconciliations).
// Disclosures is the chosen disclosure surface (documented on Materialize):
// every disclosed line this call produced, in the order produced, whether
// the call ultimately succeeded or failed. It is never nil-vs-empty
// significant; callers range over it.
type Result struct {
	WorkspaceID string
	Path        string
	Outcome     Outcome
	Disclosures []string
}

// OperationalError marks a retryable, disclosed operational failure (spec
// §Exact workspace materialization's fail-closed error class) — as opposed
// to a hard verdict error (ErrReleasedTerminal, ErrIdentityMismatch) or
// success. Op names which step/check produced it; Err is the underlying
// cause and is reachable via errors.Unwrap/errors.As/errors.Is.
type OperationalError struct {
	Op  string
	Err error
}

func (e *OperationalError) Error() string {
	return fmt.Sprintf("execworkspace: materialize: %s: %v", e.Op, e.Err)
}

func (e *OperationalError) Unwrap() error { return e.Err }

func operationalError(op string, err error) *OperationalError {
	return &OperationalError{Op: op, Err: err}
}

// ErrReleasedTerminal is step 3a's hard error: the unit's directory exists
// and its .released marker is a regular file, so this lifecycle is
// TERMINAL — never re-materialized or reused while the marker survives
// beside its directory (spec §Exact workspace materialization, step 3.1).
type ErrReleasedTerminal struct {
	WorkspaceID string
}

func (e *ErrReleasedTerminal) Error() string {
	return fmt.Sprintf(
		"execworkspace: workspace %q is released (terminal): awaiting gc reclamation, never re-materialized or reused while its .released marker survives beside its directory",
		e.WorkspaceID,
	)
}

// Materialize runs the spec's ordered state machine (steps 1-6) for req,
// under the workspace's per-operation lock held continuously across every
// step.
//
// DISCLOSURE SURFACE: every disclosed line this call produces — one per
// orphaned sibling deleted (step 2), one per residue rebuild and registry
// reconciliation (steps 2 and 4c) — is appended to the returned Result's
// Disclosures, in production order, on EVERY return path including error
// returns (Result is always populated with whatever WorkspaceID/Path/
// Disclosures were established before a failure, even though only a nil
// error's Result.Outcome is meaningful). This is the one chosen surface
// (documented here rather than also threading an io.Writer): a caller
// that wants a log sink ranges over Result.Disclosures itself.
func (m *Materializer) Materialize(ctx context.Context, req Request) (Result, error) {
	res := Result{}

	if err := req.Identity.Validate(); err != nil {
		return res, operationalError("validate request identity", err)
	}
	if err := validateRequestPatchBytes(req); err != nil {
		return res, operationalError("validate request patch bytes", err)
	}
	workspaceID, err := req.Identity.WorkspaceID()
	if err != nil {
		return res, operationalError("compute workspace id", err)
	}
	res.WorkspaceID = workspaceID
	res.Path = UnitPath(m.storeRoot, workspaceID)

	// MkdirAll of the execution root is not a unit mutation (it names no
	// <workspace-id>), so it runs before lock acquisition.
	if err := os.MkdirAll(ExecutionRoot(m.storeRoot), 0o755); err != nil {
		return res, operationalError("prepare execution root", err)
	}

	// Step 1: non-blocking acquire. ANY acquisition failure is an
	// operational error, disclosed and retryable — including ErrHeld,
	// which is reachable from the returned error via errors.As.
	lockPath := LockPath(m.storeRoot, workspaceID)
	lockFile, err := filelock.Acquire(lockPath)
	if err != nil {
		return res, operationalError("acquire lock", err)
	}

	// The lock is held CONTINUOUSLY across steps 1-6 and released here at
	// the end on every path — success or any error past acquisition.
	mErr := m.materializeLocked(ctx, req, workspaceID, &res)
	if relErr := filelock.Release(lockFile, lockPath); relErr != nil {
		relOpErr := operationalError("release lock", relErr)
		if mErr == nil {
			return res, relOpErr
		}
		// The step's own error (operational or hard-verdict) takes
		// priority; the release failure is disclosed alongside it rather
		// than masking it.
		res.Disclosures = append(res.Disclosures, fmt.Sprintf("lock release also failed: %v", relErr))
	}
	return res, mErr
}

// validateRequestPatchBytes enforces AD-6: PatchBytes is required for
// BasePlusPatch and must hash to Identity.PatchSHA256 exactly — never
// trusted to already agree — and must be empty for ExactSHA.
func validateRequestPatchBytes(req Request) error {
	switch req.Identity.Shape {
	case BasePlusPatch:
		if len(req.PatchBytes) == 0 {
			return fmt.Errorf("base-plus-patch request requires non-empty patch bytes")
		}
		sum := sha256.Sum256(req.PatchBytes)
		got := hex.EncodeToString(sum[:])
		if got != req.Identity.PatchSHA256 {
			return fmt.Errorf("patch bytes sha256 %s does not match identity's patch_sha256 %s", got, req.Identity.PatchSHA256)
		}
	case ExactSHA:
		if len(req.PatchBytes) != 0 {
			return fmt.Errorf("exact-sha request must not carry patch bytes")
		}
	default:
		return fmt.Errorf("unknown request shape %s", req.Identity.Shape)
	}
	return nil
}

// materializeLocked runs steps 2 through 6 with the unit lock already
// held. res is mutated in place so disclosures accumulate even on a
// later failure.
func (m *Materializer) materializeLocked(ctx context.Context, req Request, workspaceID string, res *Result) error {
	unitPath := UnitPath(m.storeRoot, workspaceID)

	unitKind, err := LstatType(unitPath)
	if err != nil {
		// Never read as absence: an lstat failure is always operational,
		// regardless of what os.Lstat's underlying cause is.
		return operationalError("lstat unit path", err)
	}

	switch unitKind {
	case PathAbsent:
		if err := m.handleAbsentUnit(ctx, workspaceID, res); err != nil {
			return err
		}
		// Proceeds fresh at step 5, below.
	case PathDir:
		reused, err := m.handlePresentUnit(ctx, req, workspaceID, unitPath, res)
		if err != nil {
			return err
		}
		if reused {
			res.Outcome = OutcomeReused
			return nil
		}
		// Step 4c ran inside handlePresentUnit and re-enters materialization
		// at step 5, below.
	default:
		// "Any object at the unit path that is not a real directory is an
		// OPERATIONAL ERROR on this path ... the step-3b posture applied
		// one level up."
		return operationalError("lstat unit path", fmt.Errorf("unit path %q is %s, not absent or a real directory", unitPath, unitKind))
	}

	// Step 5: materialize the worktree (either shape).
	if err := m.materializeWorktree(ctx, req, unitPath); err != nil {
		return operationalError("materialize worktree (step 5)", err)
	}

	// Step 6: write the completion witness, then this function returns and
	// Materialize releases the lock.
	if err := writeCompletionWitness(m.storeRoot, workspaceID, req.Identity); err != nil {
		return operationalError("write completion witness (step 6)", err)
	}

	res.Outcome = OutcomeMaterialized
	return nil
}

// orphanSibling names one of step 2's three sibling forms and its
// classification label, in the order the spec's own sentence lists them
// ("`.request`, `.request.staging`, `.released`").
type orphanSibling struct {
	path  string
	label string
}

func orphanSiblings(storeRoot, workspaceID string) []orphanSibling {
	return []orphanSibling{
		{RequestPath(storeRoot, workspaceID), "request"},
		{RequestStagingPath(storeRoot, workspaceID), "request-staging"},
		{ReleasedPath(storeRoot, workspaceID), "released"},
	}
}

// handleAbsentUnit implements step 2: nothing at all at the unit path.
// Every sibling present is orphaned metadata from a partial reclaim, a
// crashed write, or tampering — deleted by a plain unlink, one disclosed
// line each — then the registry is reconciled for this unit.
func (m *Materializer) handleAbsentUnit(ctx context.Context, workspaceID string, res *Result) error {
	for _, sib := range orphanSiblings(m.storeRoot, workspaceID) {
		kind, err := LstatType(sib.path)
		if err != nil {
			return operationalError("lstat orphaned sibling ("+sib.label+")", err)
		}
		switch kind {
		case PathAbsent:
			continue
		case PathRegular:
			if err := os.Remove(sib.path); err != nil {
				return operationalError("unlink orphaned sibling ("+sib.label+")", err)
			}
			res.Disclosures = append(res.Disclosures, fmt.Sprintf("step 2: deleted orphaned sibling metadata %s", sib.path))
		default:
			return operationalError("unlink orphaned sibling ("+sib.label+")", fmt.Errorf("unexpected object kind %s at %s", kind, sib.path))
		}
	}

	unitPath := UnitPath(m.storeRoot, workspaceID)
	if err := m.reconciler.ReconcileUnit(ctx, m.repoRoot, unitPath); err != nil {
		return operationalError("reconcile registry (step 2)", err)
	}
	res.Disclosures = append(res.Disclosures, fmt.Sprintf("step 2: reconciled worktree registry for %s", workspaceID))
	return nil
}

// handlePresentUnit implements steps 3 and 4 (the unit directory exists).
// It returns (true, nil) for step 4a's idempotent-reuse branch; (false,
// nil) after step 4c has rebuilt the residue, meaning the caller should
// proceed fresh at step 5; and a non-nil error for every other branch
// (3a's ErrReleasedTerminal, 3b/4b's operational errors, 4a's
// ErrIdentityMismatch).
func (m *Materializer) handlePresentUnit(ctx context.Context, req Request, workspaceID, unitPath string, res *Result) (reused bool, err error) {
	// Step 3: branch on the marker (.released) path.
	releasedPath := ReleasedPath(m.storeRoot, workspaceID)
	releasedKind, err := LstatType(releasedPath)
	if err != nil {
		return false, operationalError("lstat released marker", err)
	}
	switch releasedKind {
	case PathRegular:
		// 3a: RELEASED-TERMINAL.
		return false, &ErrReleasedTerminal{WorkspaceID: workspaceID}
	case PathAbsent:
		// Falls through to step 4.
	default:
		// 3b: non-regular object — operational error, never falls through
		// to step 4, never treated as released.
		return false, operationalError("lstat released marker", fmt.Errorf("released marker %q is %s, not a regular file or absent", releasedPath, releasedKind))
	}

	// Step 4: branch on .request.
	requestPath := RequestPath(m.storeRoot, workspaceID)
	requestKind, err := LstatType(requestPath)
	if err != nil {
		return false, operationalError("lstat request sidecar", err)
	}
	switch requestKind {
	case PathRegular:
		// 4a: present + regular. Decodability decides idempotent reuse vs.
		// undecodable (4b).
		data, rerr := os.ReadFile(requestPath)
		if rerr != nil {
			return false, operationalError("read request sidecar", rerr)
		}
		recorded, derr := DecodeSidecar(data)
		if derr != nil {
			// A non-decodable-but-regular sidecar is treated as 4b's
			// UNDECODABLE outcome, uniformly with a non-regular object.
			return false, operationalError("decode request sidecar", derr)
		}
		if verr := VerifyIdentity(workspaceID, req.Identity, recorded); verr != nil {
			// Hard error naming both, never a silent merge.
			return false, verr
		}
		return true, nil // idempotent reuse
	case PathAbsent:
		// 4c: incomplete residue of a crashed attempt. Rebuild.
		if err := m.rebuildIncompleteResidue(ctx, workspaceID, unitPath, res); err != nil {
			return false, err
		}
		return false, nil
	default:
		// 4b: present but undecodable (non-regular object). The lstat
		// discipline is uniform across sibling paths, so a non-regular
		// object here is UNDECODABLE, not a distinct third outcome.
		return false, operationalError("lstat request sidecar", fmt.Errorf("request sidecar %q is %s, undecodable", requestPath, requestKind))
	}
}

// rebuildIncompleteResidue implements step 4c: direct filesystem removal
// of the unit directory (never gitx.WorktreeRemove, never --force — the
// absence of .request is the mechanical proof no consumer ever received
// this directory, so there is no consumer-visible work to protect), an
// unlink of any .request.staging residue beside it, then the registry
// reconciliation for this unit.
func (m *Materializer) rebuildIncompleteResidue(ctx context.Context, workspaceID, unitPath string, res *Result) error {
	if err := os.RemoveAll(unitPath); err != nil {
		return operationalError("remove incomplete residue (step 4c)", err)
	}
	res.Disclosures = append(res.Disclosures, fmt.Sprintf("step 4c: removed incomplete residue directory %s", unitPath))

	stagingPath := RequestStagingPath(m.storeRoot, workspaceID)
	stagingKind, err := LstatType(stagingPath)
	if err != nil {
		return operationalError("lstat staging residue (step 4c)", err)
	}
	switch stagingKind {
	case PathAbsent:
		// Nothing to unlink.
	case PathRegular:
		if err := os.Remove(stagingPath); err != nil {
			return operationalError("unlink staging residue (step 4c)", err)
		}
		res.Disclosures = append(res.Disclosures, fmt.Sprintf("step 4c: deleted staging residue %s", stagingPath))
	default:
		return operationalError("unlink staging residue (step 4c)", fmt.Errorf("unexpected object kind %s at %s", stagingKind, stagingPath))
	}

	if err := m.reconciler.ReconcileUnit(ctx, m.repoRoot, unitPath); err != nil {
		return operationalError("reconcile registry (step 4c)", err)
	}
	res.Disclosures = append(res.Disclosures, fmt.Sprintf("step 4c: reconciled worktree registry for %s", workspaceID))
	return nil
}

// materializeWorktree implements step 5 for either request shape: the
// exact-SHA shape is a single WorktreeAddDetached; the base-plus-patch
// shape is WorktreeAddDetached at the base sha followed by ApplyPatch with
// exactly req.PatchBytes (already verified in Materialize to be the same
// canonical bytes Identity.PatchSHA256 was digested from, AD-6). A
// failure at either call is surfaced as-is; any partial directory left
// behind carries no witness and is exactly the 4c residue the next
// identity-equal attempt removes and rebuilds — this function performs no
// cleanup of its own.
func (m *Materializer) materializeWorktree(ctx context.Context, req Request, unitPath string) error {
	switch req.Identity.Shape {
	case ExactSHA:
		return gitx.WorktreeAddDetached(ctx, m.repoRoot, unitPath, req.Identity.CommitSHA)
	case BasePlusPatch:
		if err := gitx.WorktreeAddDetached(ctx, m.repoRoot, unitPath, req.Identity.CommitSHA); err != nil {
			return err
		}
		return gitx.ApplyPatch(ctx, unitPath, req.PatchBytes)
	default:
		return fmt.Errorf("unknown request shape %s", req.Identity.Shape)
	}
}

// writeCompletionWitness implements step 6: stage id's canonical sidecar
// bytes at RequestStagingPath, then atomically rename it into
// RequestPath — the completion witness. The staging path is lstat-typed
// first: an absent or REGULAR object is truncated/overwritten
// (O_CREATE|O_WRONLY|O_TRUNC, deliberately never O_EXCL — an exclusive
// create would wedge forever against a crash-left staging residue); any
// other object there is an operational error, never followed and never
// written through.
func writeCompletionWitness(storeRoot, workspaceID string, id Identity) error {
	data, err := EncodeSidecar(id)
	if err != nil {
		return fmt.Errorf("encoding completion witness: %w", err)
	}

	stagingPath := RequestStagingPath(storeRoot, workspaceID)
	stagingKind, err := LstatType(stagingPath)
	if err != nil {
		return fmt.Errorf("lstat staging path: %w", err)
	}
	switch stagingKind {
	case PathAbsent, PathRegular:
		f, oerr := os.OpenFile(stagingPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if oerr != nil {
			return fmt.Errorf("opening staging witness: %w", oerr)
		}
		if _, werr := f.Write(data); werr != nil {
			_ = f.Close()
			return fmt.Errorf("writing staging witness: %w", werr)
		}
		if cerr := f.Close(); cerr != nil {
			return fmt.Errorf("closing staging witness: %w", cerr)
		}
	default:
		return fmt.Errorf("staging path %q is %s, not absent or a regular file: never followed, never written through", stagingPath, stagingKind)
	}

	requestPath := RequestPath(storeRoot, workspaceID)
	if err := os.Rename(stagingPath, requestPath); err != nil {
		return fmt.Errorf("renaming staging witness into place: %w", err)
	}
	return nil
}
