package specimport

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/designprovenance"
	"github.com/jyang234/verdi/internal/draftmutation"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/store"
)

// PublishFaults injects deterministic failure points around Git
// publication for hermetic fault-injection tests (spec-import-contract.md
// plan Task 3: "Drive faults before publication and after successful
// publication but before response through a hermetic repository port").
// The zero value never injects a fault; production Service never sets it.
type PublishFaults struct {
	// BeforeRefUpdate runs immediately before the create-only ref update —
	// after every blob/tree/commit object is built but before anything
	// becomes reachable. A non-nil return aborts publication; the objects
	// built so far are ordinary disposable plumbing (spec-import-
	// contract.md: "Failure before ref publication has no visible branch/
	// import; unreachable Git objects are ordinary disposable plumbing").
	BeforeRefUpdate func() error
	// AfterRefUpdate runs immediately after the create-only ref update
	// succeeds, before Apply returns its Result — proving a "publication
	// succeeded but the response was lost" retry still reconciles to the
	// exact same commit.
	AfterRefUpdate func() error
}

// designBranchRef is designBranch's full ref form.
func designBranchRef(slug string) string { return "refs/heads/" + designBranch(slug) }

// Apply recomputes preview and rejects a changed digest or any blocking
// finding before Git publication (spec-import-contract.md, "Preview,
// identity and atomic publication"). It funnels through Normalize first,
// then — under the one shared checkout-wide writer lock cooperating import
// operations use — attempts reconciliation against an already-existing
// target BEFORE any cleanliness/collision gate, so a retry (even over a
// since-dirtied checkout) reaches the identical already-created result
// rather than a spurious refusal.
func (s *Service) Apply(ctx context.Context, root string, request Request, expectedDigest string, actor draftmutation.Actor) (Result, error) {
	plan, err := Normalize(request)
	if err != nil {
		return Result{}, err
	}

	var result Result
	lockErr := draftmutation.WithWriterLock(ctx, root, draftmutation.Coordinator{}, func(*draftmutation.LockedWriter) error {
		res, applyErr := s.applyLocked(ctx, root, request, plan, expectedDigest, actor)
		if applyErr != nil {
			return applyErr
		}
		result = res
		return nil
	})
	if lockErr != nil {
		return Result{}, lockErr
	}
	return result, nil
}

// applyLocked is Apply's entire body, run under the checkout-wide writer
// lock. Every error it returns propagates through WithWriterLock unchanged
// (it returns work's own error verbatim), so this function's sentinel
// wrapping is Apply's actual public error contract.
func (s *Service) applyLocked(ctx context.Context, root string, request Request, plan Plan, expectedDigest string, actor draftmutation.Actor) (Result, error) {
	slug := request.Target.Slug
	branch := designBranch(slug)

	exists, err := gitx.HasLocalBranch(ctx, root, branch)
	if err != nil {
		return Result{}, fmt.Errorf("%w: checking target branch %q: %v", ErrIOFailure, branch, err)
	}
	if exists {
		result, ok, reconcileErr := s.reconcileExisting(ctx, root, request, plan, expectedDigest, actor, branch, slug)
		if reconcileErr != nil {
			return Result{}, reconcileErr
		}
		if ok {
			return result, nil
		}
		return Result{}, fmt.Errorf("%w: design/%s already exists and does not match this request/preview/actor", ErrTargetExists, slug)
	}

	if err := s.checkCleanContext(ctx, root); err != nil {
		return Result{}, err
	}
	preview, err := s.buildPreviewResult(ctx, root, request, plan)
	if err != nil {
		return Result{}, err
	}
	if preview.Digest != expectedDigest {
		return Result{}, fmt.Errorf("%w: expected preview digest %q does not match the recomputed digest %q; source/project context changed since preview", ErrStalePreview, expectedDigest, preview.Digest)
	}
	if !preview.Ready {
		return Result{}, fmt.Errorf("%w: preview has at least one blocking finding; creation is refused", ErrUnresolved)
	}

	collision, err := specIdentityCollision(ctx, root, preview.BaseCommit, slug)
	if err != nil {
		return Result{}, err
	}
	if collision {
		return Result{}, fmt.Errorf("%w: an active or archive spec already exists at slug %q", ErrTargetExists, slug)
	}

	identity, err := buildActorIdentity(ctx, root, preview.BaseCommit, slug)
	if err != nil {
		return Result{}, err
	}
	policy, typed := draftmutation.AuthorizeMutationPolicy(ctx, root, identity, actor, s.Policy)
	if typed != nil {
		return Result{}, translateDraftmutationError(typed)
	}

	return s.publishNew(ctx, root, request, plan, preview, actor, policy, branch, slug)
}

// specIdentityCollision reports whether slug already names an active or
// archive spec at baseCommit's tree (spec-import-contract.md: "Reject any
// active/archive spec identity or target branch collision").
func specIdentityCollision(ctx context.Context, root, baseCommit, slug string) (bool, error) {
	for _, zone := range []string{store.ZoneActive, store.ZoneArchive} {
		_, found, err := gitx.BlobAt(ctx, root, baseCommit, store.SpecRelPath(zone, slug))
		if err != nil {
			return false, fmt.Errorf("%w: checking existing spec identity: %v", ErrIOFailure, err)
		}
		if found {
			return true, nil
		}
	}
	return false, nil
}

// writeFile is one blob this package writes into the new commit's tree.
type writeFile struct {
	path string
	data []byte
}

// publishNew builds the complete write set in sorted path order onto
// preview.BaseCommit's tree, creates one child commit, and create-only
// publishes refs/heads/design/<slug> (spec-import-contract.md: "Use shared
// Git plumbing to build the base tree with the complete write set in
// sorted path order, create one child commit, then create-only publish").
// No checkout/index is ever touched — every write goes through gitx's
// scratch-index plumbing. A lost create-only CAS re-enters reconciliation
// exactly once before returning target-exists.
func (s *Service) publishNew(ctx context.Context, root string, request Request, plan Plan, preview PreviewResult, actor draftmutation.Actor, policy designprovenance.Policy, branch, slug string) (Result, error) {
	recordSources := make([]RecordSource, 0, len(plan.Sources))
	files := make([]writeFile, 0, len(plan.Sources)+2)
	files = append(files, writeFile{path: store.ActiveSpecRelPath(slug), data: preview.Candidate})
	for _, snap := range plan.Sources {
		files = append(files, writeFile{path: store.ImportSourceRelPath(slug, preview.Digest, snap.ID), data: snap.Data})
		recordSources = append(recordSources, RecordSource{
			ID: snap.ID, Label: snap.Label, OriginalDigest: snap.OriginalDigest, Digest: snap.Digest,
			StartLine: snap.StartLine, EndLine: snap.EndLine,
		})
	}

	record := Record{
		Schema: RecordSchema, PreviewDigest: preview.Digest, BaseCommit: preview.BaseCommit,
		ModelDigest: preview.ModelDigest, ConfigDigest: preview.ConfigDigest, EngineDigest: preview.EngineDigest,
		RequestDigest: preview.RequestDigest, CandidateDigest: sha256Hex(preview.Candidate), SpecRef: preview.SpecRef,
		Sources: recordSources, Fields: append([]Field{}, preview.Fields...), Mappings: append([]Mapping{}, request.Mappings...),
		Coverage: append([]Coverage{}, preview.Coverage...),
		Actor:    RecordActor{Attribution: actor.Attribution(), Harness: actor.Harness(), Session: actor.Session()},
		Policy:   policy,
	}
	recordBytes, err := encodeRecord(record)
	if err != nil {
		return Result{}, err
	}
	files = append(files, writeFile{path: store.ImportRecordRelPath(slug, preview.Digest), data: recordBytes})

	sort.Slice(files, func(i, j int) bool { return files[i].path < files[j].path })

	tree := preview.BaseCommit + "^{tree}"
	for _, f := range files {
		blobSHA, err := gitx.WriteBlob(ctx, root, f.data)
		if err != nil {
			return Result{}, fmt.Errorf("%w: writing blob for %s: %v", ErrIOFailure, f.path, err)
		}
		newTree, err := gitx.BuildTreeWithFile(ctx, root, tree, f.path, blobSHA)
		if err != nil {
			return Result{}, fmt.Errorf("%w: building tree for %s: %v", ErrIOFailure, f.path, err)
		}
		tree = newTree
	}
	commit, err := gitx.CommitTree(ctx, root, tree, preview.BaseCommit, fmt.Sprintf("Import %s via spec-import", slug))
	if err != nil {
		return Result{}, fmt.Errorf("%w: creating import commit: %v", ErrIOFailure, err)
	}

	if s.Faults.BeforeRefUpdate != nil {
		if err := s.Faults.BeforeRefUpdate(); err != nil {
			return Result{}, err
		}
	}

	if err := s.recheckPublicationContext(ctx, root, preview); err != nil {
		return Result{}, err
	}

	if err := gitx.UpdateRef(ctx, root, designBranchRef(slug), commit); err != nil {
		result, ok, reconcileErr := s.reconcileExisting(ctx, root, request, plan, preview.Digest, actor, branch, slug)
		if reconcileErr != nil {
			return Result{}, reconcileErr
		}
		if ok {
			return result, nil
		}
		return Result{}, fmt.Errorf("%w: design/%s already exists: %v", ErrTargetExists, slug, err)
	}

	if s.Faults.AfterRefUpdate != nil {
		if err := s.Faults.AfterRefUpdate(); err != nil {
			return Result{}, err
		}
	}

	return Result{
		Schema: ResultSchema, Status: StatusCreated, Branch: branch, Commit: commit,
		SpecRef: preview.SpecRef, PreviewDigest: preview.Digest, StatementsDeferred: request.DeferStatements,
		Disclosures: nonBlockingFindings(preview.Findings), BoardPath: boardPath(slug),
	}, nil
}

// recheckPublicationContext re-verifies, immediately before the create-only
// ref update, the two context facts the prepared candidate is bound to
// (spec-import-contract.md: "Recheck HEAD/context before publication"):
// the tracked checkout/index is still clean, and HEAD is still exactly the
// preview's recorded base commit. Neither check subsumes the other — an
// uncommitted window change leaves HEAD alone, and a committed one leaves
// the checkout clean — so both run here, after publishNew's fault seam so
// hermetic tests can drive either one.
//
// It refuses; it never re-parents, resets or overwrites. Re-parenting the
// already-built candidate onto a new HEAD would publish a tree no preview
// digest binds (and could publish over a spec identity that appeared in
// the window), so the operator is told to re-preview against the context
// that actually exists now.
func (s *Service) recheckPublicationContext(ctx context.Context, root string, preview PreviewResult) error {
	if err := s.checkCleanContext(ctx, root); err != nil {
		return err
	}
	head, err := gitx.RevParse(ctx, root, "HEAD")
	if err != nil {
		return fmt.Errorf("%w: re-reading HEAD before publication: %v", ErrIdentityUnavailable, err)
	}
	if head != preview.BaseCommit {
		return fmt.Errorf("%w: HEAD moved from the preview's base commit %s to %s before publication; the prepared candidate is bound to the old context and is never re-parented — re-run preview against the current HEAD and apply that digest", ErrStalePreview, preview.BaseCommit, head)
	}
	return nil
}

// reconcileExisting attempts the contract's already-created reconciliation
// (spec-import-contract.md: "an existing target can be reported already-
// created only if its tip is exactly the recorded base plus the expected
// import write set, the recorded preview/request/actor match this call,
// and every candidate/source/record binding verifies under the same engine
// identity"). It reads ONLY committed Git bytes — never the caller's
// working tree/model/config — so it remains safe even over a since-dirtied
// checkout. ok is true only on a full match; a non-nil err is reserved for
// a genuine operational failure or an engine-identity provenance-mismatch,
// which must never be silently downgraded to an ordinary target-exists.
func (s *Service) reconcileExisting(ctx context.Context, root string, request Request, plan Plan, expectedDigest string, actor draftmutation.Actor, branch, slug string) (Result, bool, error) {
	record, importCommit, err := verifyCommittedImport(ctx, root, branch, slug, expectedDigest)
	if err != nil {
		if isProvenanceMismatch(err) {
			return Result{}, false, err
		}
		return Result{}, false, nil
	}

	canonicalRequest, err := canonicalRequestBytes(request)
	if err != nil {
		return Result{}, false, err
	}
	if record.RequestDigest != sha256Hex(canonicalRequest) {
		return Result{}, false, nil
	}
	if record.Actor.Attribution != actor.Attribution() || record.Actor.Harness != actor.Harness() || record.Actor.Session != actor.Session() {
		return Result{}, false, nil
	}
	if s.Engine == nil {
		return Result{}, false, fmt.Errorf("%w: service has no engine identity configured", ErrIOFailure)
	}
	engineDigest, err := s.Engine.Digest(ctx)
	if err != nil {
		return Result{}, false, fmt.Errorf("%w: resolving engine identity: %v", ErrIOFailure, err)
	}
	if record.EngineDigest != engineDigest {
		return Result{}, false, fmt.Errorf("%w: the committed import was produced under a different engine identity; inspect it read-only via ReadRecord rather than retrying creation", ErrProvenanceMismatch)
	}

	tip, err := gitx.RevParse(ctx, root, branch)
	if err != nil {
		return Result{}, false, fmt.Errorf("%w: resolving %s tip: %v", ErrIOFailure, branch, err)
	}
	if tip != importCommit {
		// The branch has moved past the pristine import (ordinary
		// descendant edits): this is not a matching retry of the SAME
		// creation, so it falls through to the ordinary ErrTargetExists
		// refusal rather than a false already-created claim.
		return Result{}, false, nil
	}

	_, fieldFindings := prepareCandidateFields(request, plan)
	disclosures := nonBlockingFindings(fieldFindings)

	return Result{
		Schema: ResultSchema, Status: StatusAlreadyCreated, Branch: branch, Commit: tip,
		SpecRef: specRef(slug), PreviewDigest: expectedDigest, StatementsDeferred: request.DeferStatements,
		Disclosures: disclosures, BoardPath: boardPath(slug),
	}, true, nil
}

// canonicalRequestBytes returns request's canonical JSON, recomputed fresh
// rather than trusted from any caller-supplied claim (reconcileExisting
// never trusts the committed record's own request_digest claim alone: it
// recomputes this call's own request digest and compares).
func canonicalRequestBytes(request Request) ([]byte, error) {
	data, err := canonjson.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("%w: canonicalizing request: %v", ErrInvalidRequest, err)
	}
	return data, nil
}

// isProvenanceMismatch reports whether err wraps ErrProvenanceMismatch —
// the one reconciliation failure that must never be silently downgraded to
// an ordinary target-exists refusal.
func isProvenanceMismatch(err error) bool { return errors.Is(err, ErrProvenanceMismatch) }
