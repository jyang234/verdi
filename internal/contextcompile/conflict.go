// Sealed policy-conflict operands (docs/superpowers/specs/2026-08-12-
// policy-conflict-gate-authority-design.md §§2-3, 6, 12; ledger SI-93,
// SI-94, SI-99). This file adds a runtime-only, non-wire projection over
// the existing compiler: ConflictOperands binds one accepted-context
// compile or one exact acceptance-candidate resolution to its typed
// policy claims, normalized authority prose, applicable exemptions, and
// governing profile/actor facts, sealed against post-construction mutation
// and cross-snapshot substitution. It carries no verdict semantics — that
// is internal/policyconflict's job over the operands this file produces.
//
// Accepted-context construction (CompileConflict) reuses compilePipeline's
// single compile/policy-resolution pass unchanged: it never calls
// AuthorityLoader.Load/Resolve or RepositoryFactsGatherer.Gather a second
// time. Acceptance-candidate construction (resolveConflictCandidate/
// ResolveConflictCandidate) resolves its own one authority/repository pass
// and reads the exact HEAD-tree candidate spec blob directly — it never
// verifies merge-signaled acceptance state for its own target, never reads
// worktree-changed paths, never verifies the instruction projection, and
// never calls the accepted manifest encoder.
package contextcompile

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"unicode/utf8"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/policyauthority"
	"github.com/jyang234/verdi/internal/repositoryfacts"
	"github.com/jyang234/verdi/internal/specstate"
)

// The two closed SnapshotIdentity.TargetKind values (authority design §3).
const (
	snapshotTargetAcceptedContext     = "accepted-context"
	snapshotTargetAcceptanceCandidate = "acceptance-candidate"
)

// The closed §6 source-category vocabulary this package derives from the
// data compilePipeline/resolveConflictCandidate already resolve. The exact
// string values mirror policyartifact.knownWitnessCategories so a
// ProseClaim.Category always matches a legal disposition-witness category.
//
// adr-decision is produced on both arms from the target's effective
// declared context (SI-92), pinning each declared ADR's exact bytes and
// content digest. An ADR's whole normalized authored body IS its decision
// authority — artifact.ADRFrontmatter carries no structured decision field
// to project instead. Candidate construction resolves the target and its
// parents from req.Expected.Head, then ResolveDeclaredContext follows each
// declared ref's own exact pinned commit; no accepted-target resolution or
// candidate manifest is involved.
const (
	categoryPolicyInstruction     = "policy-instruction"
	categorySpecProblem           = "spec-problem"
	categorySpecOutcome           = "spec-outcome"
	categoryAcceptanceCriterion   = "acceptance-criterion"
	categoryOpenQuestion          = "open-question"
	categoryConstraint            = "constraint"
	categoryDecision              = "decision"
	categoryADRDecision           = "adr-decision"
	categoryObligationDeclaration = "obligation-declaration"
)

// conflictADRDecisionObject is the canonical object id every adr-decision
// ProseClaim carries. An ADR has exactly one decision authority — its
// authored body — and no structured object list to name, so this one fixed
// id keeps ADR claim identity (ref#object) shaped exactly like every
// neighboring spec/fragment claim's.
const conflictADRDecisionObject = "decision"

// TypedClaim is one applicable policy's claim, exactly as sealed for
// mechanical conflict evaluation: the base (unrefined) operand plus its
// governing policy identity and the exact witness digest an exemption
// binds to. ConflictView.EffectivePolicy carries the same claim's scoped
// overlay refinements; this flattened form exists for the mechanical
// solver's (family, subject) grouping.
type TypedClaim struct {
	PolicyID, PolicyDigest, ClaimDigest string
	Claim                               policyartifact.Claim
}

// ProseClaim is one normalized human-authored authority-prose claim
// (authority design §6): its canonical source id and category, normalized
// single text value and digest, source artifact identity, inherited
// scope, governing authority digest, and the exact object/line identity
// it came from.
type ProseClaim struct {
	ID, Category, Text, TextDigest        string
	SourceRef, SourcePath, SourceDigest   string
	Scope                                 policyartifact.Scope
	AuthorityDigest, Object, LineIdentity string
}

// ConflictSourceIdentity is one exact contributing artifact's identity
// inside a conflict snapshot.
type ConflictSourceIdentity struct {
	Ref, Path, ContentDigest string
}

// SnapshotIdentity binds every fact a conflict evaluation's sealed
// operands were resolved from (authority design §3). Exactly one of
// ManifestDigest (accepted-context) or CandidateDigest (acceptance-
// candidate) is set, matching TargetKind.
type SnapshotIdentity struct {
	TargetKind                                string
	Repository                                repositoryfacts.Facts
	ManifestDigest, CandidateDigest           string
	EffectivePolicyDigest, ConstitutionDigest string
	ProfileID, ProfileDigest                  string
	Adapter                                   AdapterRef
	Phase                                     Phase
	Scope                                     policyartifact.Scope
	GrantDigest                               string
	Sources                                   []ConflictSourceIdentity
}

// CandidateRequest is the exact operand set an acceptance-candidate
// resolution requires: the same strict adapter/grant/scope shape as an
// accepted compile Request, plus the unpinned whole spec/<name> ref the
// candidate proposes and a mandatory (not optional) expected branch/HEAD.
type CandidateRequest struct {
	Adapter  AdapterRef
	Expected Expected
	Grants   execworkspace.GrantSet
	Scope    policyartifact.Scope
	Spec     string
}

// ConflictFacts carries the caller-supplied sealed principal resolutions a
// conflict evaluation needs (authority design §9). A managed caller
// injects authenticated resolutions; the local CLI supplies none.
type ConflictFacts struct {
	Actors []governanceprincipal.PrincipalResolution
}

// ConflictView is ConflictOperands' complete exported content: exactly the
// semantic groups authority design §3 names. It is never marshaled
// directly and carries no self-digest of its own — ConflictOperands owns
// sealing.
type ConflictView struct {
	Snapshot        SnapshotIdentity
	EffectivePolicy policyauthority.EffectivePolicy
	TypedClaims     []TypedClaim
	ProseClaims     []ProseClaim
	Exemptions      []policyartifact.Exemption
	Profile         governanceprincipal.Profile
	Actors          []governanceprincipal.PrincipalResolution
}

// ConflictOperands is a sealed, clone-on-read projection: view is the
// package's own private deep copy, seal is the canonical digest minted
// over view plus a per-construction nonce (see sealConflictOperands), and
// every field is unexported so only this package's constructors can
// produce a value whose View() succeeds. The zero value, a nil pointer,
// and any hand-built literal (even one that copies an authentic view) fail
// closed.
//
// The guard is two-layered, and honest about which layer covers what.
// cloneConflictView PREVENTS mutation for everything this package can
// deep-copy; for the one value it cannot — a pointer-implemented
// policyartifact.Payload interface value, which stays shared across clones
// — the seal DETECTS it, because the next View() reseals the private view
// and refuses on any digest change. Detection, not prevention, is the
// contract for that field.
type ConflictOperands struct {
	view  ConflictView
	seal  string
	nonce string
}

// conflictOperandsSeq is a process-local monotonic counter minting each
// ConflictOperands construction's nonce. Its only purpose is to make two
// separately constructed operands whose exported content happens to be
// byte-identical still bind to numerically distinct seals, so splicing one
// construction's view under another's seal (or vice versa) always fails
// integrity verification — content equality alone must never be mistaken
// for the same sealed construction. It carries no wall-clock or random
// input and is never part of any wire artifact.
var conflictOperandsSeq uint64

// conflictOperandsSealDoc is the exact value canonjson.Digest seals: the
// view plus its construction nonce. Unexported: this shape never crosses
// the package boundary.
type conflictOperandsSealDoc struct {
	View  ConflictView
	Nonce string
}

// sealConflictOperands takes ownership of a freshly built ConflictView,
// deep-clones it into private storage so no caller-held reference can
// reach it, and mints its one seal.
func sealConflictOperands(view ConflictView) (*ConflictOperands, error) {
	clean := cloneConflictView(view)
	nonce := strconv.FormatUint(atomic.AddUint64(&conflictOperandsSeq, 1), 10)
	seal, err := canonjson.Digest(conflictOperandsSealDoc{View: clean, Nonce: nonce})
	if err != nil {
		return nil, fmt.Errorf("contextcompile: seal conflict operands: %w", err)
	}
	return &ConflictOperands{view: clean, seal: seal, nonce: nonce}, nil
}

// View verifies o's integrity — a zero value, a nil pointer, an unsealed
// literal, a forged seal, and a cross-snapshot splice (a view spliced
// under a foreign seal/nonce pair) all fail closed — and returns a deep
// clone of the sealed content so the caller can never mutate o's private
// state through the returned value.
func (o *ConflictOperands) View() (ConflictView, error) {
	if o == nil {
		return ConflictView{}, fmt.Errorf("contextcompile: conflict operands: nil receiver")
	}
	if o.seal == "" {
		return ConflictView{}, fmt.Errorf("contextcompile: conflict operands are unsealed (not produced by CompileConflict/resolveConflictCandidate)")
	}
	got, err := canonjson.Digest(conflictOperandsSealDoc{View: o.view, Nonce: o.nonce})
	if err != nil {
		return ConflictView{}, fmt.Errorf("contextcompile: reseal conflict operands: %w", err)
	}
	if got != o.seal {
		return ConflictView{}, fmt.Errorf("contextcompile: conflict operands failed integrity verification (mutated, forged, or cross-snapshot substituted)")
	}
	return cloneConflictView(o.view), nil
}

// --- accepted-context construction (authority design §3) -------------------

// CompileConflict compiles request exactly once (the same compilePipeline
// Compile itself runs) and seals the resulting sealed conflict operands
// from that single compile/policy-resolution pass — it never reloads
// policy or re-gathers repository facts.
func (c Compiler) CompileConflict(ctx context.Context, root string, request Request, facts ConflictFacts) (*ConflictOperands, error) {
	outcome, err := c.compilePipeline(ctx, root, request)
	if err != nil {
		return nil, err
	}

	snapshot, err := buildSnapshotIdentity(snapshotBuildInput{
		targetKind:     snapshotTargetAcceptedContext,
		repository:     outcome.snapshot.Facts,
		manifestDigest: outcome.result.Manifest.Digest,
		authority:      outcome.authority,
		adapter:        AdapterRef{ID: outcome.authority.Adapter.ID, Version: outcome.authority.Adapter.Version},
		phase:          request.Phase,
		scope:          request.Scope,
		grants:         request.Grants,
		target:         outcome.target,
		fragments:      outcome.fragments,
		obligations:    outcome.obligations,
		declared:       outcome.declared.Items,
		selection:      outcome.selection,
	})
	if err != nil {
		return nil, fmt.Errorf("contextcompile: build accepted conflict snapshot: %w", err)
	}

	view, err := buildConflictView(outcome.authority, outcome.selection, outcome.target, outcome.fragments, outcome.obligations, outcome.declared.Items, request.Scope, snapshot, facts)
	if err != nil {
		return nil, err
	}
	return sealConflictOperands(view)
}

// --- acceptance-candidate construction (authority design §2-3) -------------

// validate checks r's complete grammar: a mandatory (never optional)
// expected branch and full 40-hex HEAD, the same adapter/grant/scope
// grammar as an accepted request, and an unpinned whole spec/<name> ref.
func (r CandidateRequest) validate() error {
	if err := r.Adapter.validate("candidate.adapter"); err != nil {
		return err
	}
	if err := validateNonEmpty("candidate.expected.branch", r.Expected.Branch); err != nil {
		return err
	}
	if err := validateGitHash("candidate.expected.head", r.Expected.Head); err != nil {
		return err
	}
	if len(r.Expected.Head) != 40 {
		return fmt.Errorf("contextcompile: candidate.expected.head: %q must be a full 40-lowercase-hex commit", r.Expected.Head)
	}
	if err := r.Grants.Validate(); err != nil {
		return fmt.Errorf("contextcompile: candidate.grants: %w", err)
	}
	if err := validateScope("candidate.scope", r.Scope); err != nil {
		return err
	}
	if err := validateSpecWholeRef("candidate.spec", r.Spec); err != nil {
		return err
	}
	return nil
}

// ResolveConflictCandidate resolves req's sealed conflict operands using
// production trusted ports (NewCompiler): the one package-level entry
// point internal/policyconflict's acceptance-candidate arm dispatches to.
func ResolveConflictCandidate(ctx context.Context, root string, req CandidateRequest, facts ConflictFacts) (*ConflictOperands, error) {
	return NewCompiler().resolveConflictCandidate(ctx, root, req, facts)
}

// resolveConflictCandidate resolves the exact HEAD-tree candidate spec
// blob (authority design §2): it verifies req's declared branch/HEAD
// against computed repository facts, requires the candidate spec to exist
// as a regular blob at the fixed active path (never the archive zone, and
// never a symlink), requires its declared id to match, and records its
// exact content digest. It never verifies merge-signaled acceptance state
// for its own target, never reads worktree-changed paths (a dirty
// worktree can never substitute for the exact HEAD-tree blob), never
// verifies the instruction projection, and never calls the accepted
// manifest encoder — ManifestDigest is always empty on this arm.
func (c Compiler) resolveConflictCandidate(ctx context.Context, root string, req CandidateRequest, facts ConflictFacts) (*ConflictOperands, error) {
	if !c.constructed {
		return nil, fmt.Errorf("contextcompile: zero-value Compiler cannot resolve a conflict candidate; use NewCompiler")
	}
	if root == "" {
		return nil, fmt.Errorf("contextcompile: root must not be empty")
	}
	if err := req.validate(); err != nil {
		return nil, fmt.Errorf("contextcompile: validate acceptance-candidate request: %w", err)
	}

	authority, err := ResolvePolicyAuthority(c.authority, root, req.Adapter)
	if err != nil {
		return nil, fmt.Errorf("contextcompile: resolve candidate policy authority: %w", err)
	}

	snapshot, err := c.repoFacts.Gather(ctx, repositoryfacts.GatherInput{Root: root})
	if err != nil {
		return nil, fmt.Errorf("contextcompile: gather candidate repository facts: %w", err)
	}
	if err := ResolveExpectedRepository(&req.Expected, snapshot.Facts); err != nil {
		return nil, fmt.Errorf("contextcompile: compare candidate expected repository: %w", err)
	}

	parsed, err := artifact.ParseRef(req.Spec)
	if err != nil {
		return nil, fmt.Errorf("contextcompile: parse candidate spec ref: %w", err)
	}
	activePath := ".verdi/specs/active/" + parsed.Name + "/spec.md"

	entries, err := c.git.LsTreeEntries(ctx, root, req.Expected.Head)
	if err != nil {
		return nil, fmt.Errorf("contextcompile: list candidate HEAD tree: %w", err)
	}
	var matches []gitx.TreeEntry
	for _, entry := range entries {
		if entry.Path == activePath {
			matches = append(matches, entry)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("contextcompile: candidate spec %s is absent from the exact HEAD active tree at %s", req.Spec, activePath)
	}
	if len(matches) != 1 {
		return nil, fmt.Errorf("contextcompile: candidate spec %s appears more than once at the exact HEAD active path %s", req.Spec, activePath)
	}
	entry := matches[0]
	if entry.Type != "blob" || (entry.Mode != "100644" && entry.Mode != "100755") || entry.Object == "" {
		return nil, fmt.Errorf("contextcompile: candidate spec %s is not a regular HEAD-tree blob: %+v", req.Spec, entry)
	}
	if err := validateGitHash("candidate spec blob", entry.Object); err != nil {
		return nil, err
	}

	content, err := c.git.Show(ctx, root, req.Expected.Head, entry.Path)
	if err != nil {
		return nil, fmt.Errorf("contextcompile: read candidate HEAD spec %s: %w", entry.Path, err)
	}
	fmBytes, _, err := artifact.SplitFrontmatter(content)
	if err != nil {
		return nil, fmt.Errorf("contextcompile: decode candidate spec %s: %w", req.Spec, err)
	}
	spec, err := artifact.DecodeSpec(fmBytes)
	if err != nil {
		return nil, fmt.Errorf("contextcompile: decode candidate spec %s: %w", req.Spec, err)
	}
	if spec.ID != req.Spec {
		return nil, fmt.Errorf("contextcompile: candidate spec path for %s declares id %q", req.Spec, spec.ID)
	}

	candidateDigest := rawContentDigest(content)
	target := ResolvedSpec{
		Ref: req.Spec, Path: entry.Path, Blob: entry.Object, ContentDigest: candidateDigest,
		Content: append([]byte(nil), content...), Spec: spec,
		// State is synthetically fixed so ResolveFeatureFragments' own
		// internal "target must already be accepted" gate is satisfied
		// when this candidate is itself a story: that gate keeps a
		// STORY's OWN acceptance honest for capsule compilation, but a
		// proposed acceptance candidate is by definition not yet
		// accepted — what actually needs proving here is only that its
		// DECLARED PARENTS are accepted, which ResolveFeatureFragments
		// proves independently via ResolveAcceptedSpec on each parent
		// ref. No caller ever observes this synthetic State; it exists
		// only to satisfy that one internal precondition.
		State: specstate.AcceptedPendingBuild,
	}

	var fragments []FeatureFragment
	switch spec.Class {
	case artifact.ClassFeature:
		fragments = []FeatureFragment{}
	case artifact.ClassStory:
		fragments, err = ResolveFeatureFragments(ctx, c.git, c.states, root, req.Expected.Head, target)
		if err != nil {
			// The marker must sit on the literal's own line or exactly the
			// line above it (scanVocabProse's lineHasMarker rule), so this
			// classification stays one line.
			// vocab:identity — "feature" names the fixed ResolveFeatureFragments stage and its governing-parent artifact-class identity this candidate-arm operational diagnostic reports (SI-84/SI-93)
			return nil, fmt.Errorf("contextcompile: resolve candidate feature fragments: %w", err)
		}
	default:
		return nil, &DeclaredScopeRefusal{
			Phase: PhaseDesign, Ref: target.Ref,
			Reason: fmt.Sprintf("target class %q is not a legal acceptance-candidate target", spec.Class),
		}
	}

	obligations, err := ResolveBoundObligations(ctx, c.git, root, req.Expected.Head, target)
	if err != nil {
		return nil, fmt.Errorf("contextcompile: resolve candidate bound obligations: %w", err)
	}
	declared, err := ResolveDeclaredContext(ctx, c.git, root, req.Expected.Head, target, fragments)
	if err != nil {
		return nil, fmt.Errorf("contextcompile: resolve declared context for candidate: %w", err)
	}

	candidateAsRequest := Request{Schema: RequestSchema, Adapter: req.Adapter, Phase: PhaseDesign, Scope: req.Scope, Spec: req.Spec}
	operandCandidates, err := authorityOperandCandidates(authority)
	if err != nil {
		return nil, fmt.Errorf("contextcompile: list candidate authority operand candidates: %w", err)
	}
	selection, err := selectAuthorityOperands(operandCandidates, candidateAsRequest, target)
	if err != nil {
		return nil, fmt.Errorf("contextcompile: select candidate authority operands: %w", err)
	}

	snapshotIdentity, err := buildSnapshotIdentity(snapshotBuildInput{
		targetKind:      snapshotTargetAcceptanceCandidate,
		repository:      snapshot.Facts,
		candidateDigest: candidateDigest,
		authority:       authority,
		adapter:         AdapterRef{ID: authority.Adapter.ID, Version: authority.Adapter.Version},
		phase:           PhaseDesign,
		scope:           req.Scope,
		grants:          req.Grants,
		target:          target,
		fragments:       fragments,
		obligations:     obligations,
		declared:        declared.Items,
		selection:       selection,
	})
	if err != nil {
		return nil, fmt.Errorf("contextcompile: build candidate conflict snapshot: %w", err)
	}

	view, err := buildConflictView(authority, selection, target, fragments, obligations, declared.Items, req.Scope, snapshotIdentity, facts)
	if err != nil {
		return nil, err
	}
	return sealConflictOperands(view)
}

// --- shared snapshot/view assembly ------------------------------------------

// snapshotBuildInput bundles buildSnapshotIdentity's inputs, shared by both
// the accepted-context and acceptance-candidate arms.
type snapshotBuildInput struct {
	targetKind                      string
	repository                      repositoryfacts.Facts
	manifestDigest, candidateDigest string
	authority                       PolicyAuthority
	adapter                         AdapterRef
	phase                           Phase
	scope                           policyartifact.Scope
	grants                          execworkspace.GrantSet
	target                          ResolvedSpec
	fragments                       []FeatureFragment
	obligations                     []BoundObligation
	// declared is the arm's effective declared-context resolution.
	declared  []DeclaredContextItem
	selection authoritySelection
}

// validateSnapshotIdentityDigests enforces the mutually exclusive target
// identity: an accepted-context snapshot carries a manifest digest and no
// candidate digest, an acceptance-candidate snapshot exactly the reverse,
// and no other target kind is assembled at all. A snapshot carrying both
// (or neither) would let an evaluation be derived against a target its
// operands were never resolved from, so every violation is operational.
func validateSnapshotIdentityDigests(targetKind, manifestDigest, candidateDigest string) error {
	switch targetKind {
	case snapshotTargetAcceptedContext:
		if manifestDigest == "" || candidateDigest != "" {
			return fmt.Errorf("contextcompile: conflict snapshot: target kind %q requires exactly the manifest identity digest (manifest=%q candidate=%q)", targetKind, manifestDigest, candidateDigest)
		}
	case snapshotTargetAcceptanceCandidate:
		if candidateDigest == "" || manifestDigest != "" {
			return fmt.Errorf("contextcompile: conflict snapshot: target kind %q requires exactly the candidate identity digest (manifest=%q candidate=%q)", targetKind, manifestDigest, candidateDigest)
		}
	default:
		return fmt.Errorf("contextcompile: conflict snapshot: unknown target kind %q carries no legal identity digest", targetKind)
	}
	return nil
}

func buildSnapshotIdentity(in snapshotBuildInput) (SnapshotIdentity, error) {
	if err := in.repository.Validate(); err != nil {
		return SnapshotIdentity{}, fmt.Errorf("contextcompile: conflict snapshot: repository facts: %w", err)
	}
	if in.authority.Effective == nil {
		return SnapshotIdentity{}, fmt.Errorf("contextcompile: conflict snapshot: policy authority is not resolved")
	}
	if err := validateSnapshotIdentityDigests(in.targetKind, in.manifestDigest, in.candidateDigest); err != nil {
		return SnapshotIdentity{}, err
	}
	grantBytes, err := execworkspace.EncodeGrantSet(in.grants)
	if err != nil {
		return SnapshotIdentity{}, fmt.Errorf("contextcompile: conflict snapshot: encode grants: %w", err)
	}
	sources, err := buildConflictSources(in.target, in.fragments, in.obligations, in.declared, in.selection, in.authority)
	if err != nil {
		return SnapshotIdentity{}, err
	}

	return SnapshotIdentity{
		TargetKind:            in.targetKind,
		Repository:            in.repository,
		ManifestDigest:        in.manifestDigest,
		CandidateDigest:       in.candidateDigest,
		EffectivePolicyDigest: in.authority.EffectiveDigest,
		ConstitutionDigest:    in.authority.Effective.ConstitutionDigest,
		ProfileID:             in.authority.Effective.ProfileID,
		ProfileDigest:         in.authority.Effective.ProfileDigest,
		Adapter:               in.adapter,
		Phase:                 in.phase,
		Scope:                 cloneScope(in.scope),
		GrantDigest:           rawContentDigest(grantBytes),
		Sources:               sources,
	}, nil
}

// buildConflictSources returns the unique, sorted-by-(ref,path,digest) set
// of every exact artifact contributing to this snapshot: the accepted or
// candidate target, every governing parent-feature fragment, every bound
// obligation, every declared-context ADR whose decision this snapshot
// binds, every applicable policy/overlay/exemption operand, and the
// resolved constitution and selected profile.
func buildConflictSources(target ResolvedSpec, fragments []FeatureFragment, obligations []BoundObligation, declared []DeclaredContextItem, selection authoritySelection, authority PolicyAuthority) ([]ConflictSourceIdentity, error) {
	authorityArtifacts, err := resolvedAuthorityArtifacts(authority)
	if err != nil {
		return nil, fmt.Errorf("contextcompile: conflict snapshot: resolved authority artifacts: %w", err)
	}

	seen := make(map[ConflictSourceIdentity]bool)
	var out []ConflictSourceIdentity
	add := func(ref, path, digest string) error {
		if ref == "" || path == "" || digest == "" {
			return fmt.Errorf("contextcompile: conflict snapshot: source with empty ref/path/digest (ref=%q path=%q digest=%q)", ref, path, digest)
		}
		key := ConflictSourceIdentity{Ref: ref, Path: path, ContentDigest: digest}
		if seen[key] {
			return nil
		}
		seen[key] = true
		out = append(out, key)
		return nil
	}

	if err := add(target.Ref, target.Path, target.ContentDigest); err != nil {
		return nil, err
	}
	for _, f := range fragments {
		if err := add(f.Feature.Ref, f.Feature.Path, f.Feature.SourceDigest); err != nil {
			return nil, err
		}
	}
	for _, o := range obligations {
		if err := add(o.Ref, o.Path, o.ContentDigest); err != nil {
			return nil, err
		}
	}
	for _, item := range declaredADRItems(declared) {
		if err := add(item.Ref, item.Path, item.ContentDigest); err != nil {
			return nil, err
		}
	}
	for _, op := range selection.Operands {
		if err := add(op.ID, op.Path, op.Digest); err != nil {
			return nil, err
		}
	}
	for _, a := range authorityArtifacts {
		if err := add(a.Ref, a.Path, a.Digest); err != nil {
			return nil, err
		}
	}

	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Ref != b.Ref {
			return a.Ref < b.Ref
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.ContentDigest < b.ContentDigest
	})
	return out, nil
}

// buildConflictView assembles ConflictView's full content from one already-
// resolved authority/selection/target/fragments/obligations set: the
// applicable typed claims, the normalized authority-prose universe this
// package can derive, applicable exemptions, the selected governance
// profile, and the caller-supplied actor facts.
func buildConflictView(authority PolicyAuthority, selection authoritySelection, target ResolvedSpec, fragments []FeatureFragment, obligations []BoundObligation, declared []DeclaredContextItem, governingScope policyartifact.Scope, snapshot SnapshotIdentity, facts ConflictFacts) (ConflictView, error) {
	if authority.Store == nil || authority.Effective == nil {
		return ConflictView{}, fmt.Errorf("contextcompile: conflict view: policy authority is not resolved")
	}
	storedProfile, ok := authority.Store.Profiles[authority.Effective.ProfileID]
	if !ok || storedProfile == nil {
		return ConflictView{}, fmt.Errorf("contextcompile: conflict view: resolved effective policy names profile %q, absent from the loaded store", authority.Effective.ProfileID)
	}
	if target.Spec == nil {
		return ConflictView{}, fmt.Errorf("contextcompile: conflict view: target %s has no decoded specification", target.Ref)
	}

	proseClaims, err := buildProseClaims(authority, selection, target, fragments, obligations, declared, governingScope)
	if err != nil {
		return ConflictView{}, err
	}
	exemptions, err := buildConflictExemptions(authority, selection)
	if err != nil {
		return ConflictView{}, err
	}

	return ConflictView{
		Snapshot:        snapshot,
		EffectivePolicy: *authority.Effective,
		TypedClaims:     buildTypedClaims(authority, selection),
		ProseClaims:     proseClaims,
		Exemptions:      exemptions,
		Profile:         storedProfile.Profile,
		Actors:          cloneActorResolutions(facts.Actors),
	}, nil
}

// --- typed claims (authority design §5) -------------------------------------

// buildTypedClaims returns one TypedClaim per applicable policy's claim,
// sorted by (policy id, claim id). Only Kind==PolicyEntryPolicy selection
// operands carry claims; overlays and exemptions never do.
func buildTypedClaims(authority PolicyAuthority, selection authoritySelection) []TypedClaim {
	entryByID := make(map[string]policyauthority.EffectivePolicyEntry, len(authority.Effective.Policies))
	for _, e := range authority.Effective.Policies {
		entryByID[e.PolicyID] = e
	}

	var out []TypedClaim
	for _, op := range selection.Operands {
		if op.Kind != PolicyEntryPolicy {
			continue
		}
		entry, ok := entryByID[op.ID]
		if !ok {
			continue
		}
		for _, ec := range entry.Claims {
			out = append(out, TypedClaim{
				PolicyID:     entry.PolicyID,
				PolicyDigest: entry.PolicyDigest,
				ClaimDigest:  ec.BaseClaimDigest,
				Claim: policyartifact.Claim{
					ID:          ec.ID,
					Family:      ec.Family,
					Operator:    ec.Operator,
					Subject:     ec.Subject,
					Values:      append([]string{}, ec.Values...),
					Bound:       cloneIntPtr(ec.Bound),
					Scope:       cloneScope(ec.Scope),
					Overridable: ec.Overridable,
				},
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].PolicyID != out[j].PolicyID {
			return out[i].PolicyID < out[j].PolicyID
		}
		return out[i].Claim.ID < out[j].Claim.ID
	})
	return out
}

// --- exemptions --------------------------------------------------------------

// buildConflictExemptions returns the applicable committed exemption
// artifacts (Kind==PolicyEntryExemption selection operands), sorted by id.
func buildConflictExemptions(authority PolicyAuthority, selection authoritySelection) ([]policyartifact.Exemption, error) {
	var out []policyartifact.Exemption
	for _, op := range selection.Operands {
		if op.Kind != PolicyEntryExemption {
			continue
		}
		e, ok := authority.Store.Exemptions[op.ID]
		if !ok || e == nil {
			return nil, fmt.Errorf("contextcompile: conflict exemptions: selected exemption %q is absent from the loaded store", op.ID)
		}
		out = append(out, cloneExemption(*e))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// --- normalized authority prose (authority design §6) -----------------------

// normalizeAuthorityProse returns one authored artifact's normalized prose
// body, exactly as authority design §6 fixes normalization: it validates
// UTF-8, converts CRLF to LF BEFORE artifact parsing, trims only the
// structural frontmatter block and the newlines that delimit it from the
// body, and never case-folds, rewrites, summarizes, or reorders the
// authored text (interior text, including indentation, survives byte for
// byte). Trimming is deliberately restricted to "\n" rather than every
// space character: a whole-artifact TrimSpace both leaks frontmatter into
// the claim and rewrites authored leading indentation.
//
// Every failure is OPERATIONAL and wrapped — invalid UTF-8, an artifact
// without a well-formed frontmatter block, and an artifact whose authored
// body is blank. None of them may degrade into a silently skipped claim: a
// semantic universe quietly missing an authority claim is exactly the
// favorable-silence failure the three-valued honesty rule forbids.
func normalizeAuthorityProse(what string, raw []byte) (string, error) {
	if !utf8.Valid(raw) {
		return "", fmt.Errorf("contextcompile: normalize authored prose for %s: content is not valid UTF-8", what)
	}
	_, body, err := artifact.SplitFrontmatter(bytes.ReplaceAll(raw, []byte("\r\n"), []byte("\n")))
	if err != nil {
		return "", fmt.Errorf("contextcompile: normalize authored prose for %s: %w", what, err)
	}
	text := strings.Trim(string(body), "\n")
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("contextcompile: normalize authored prose for %s: the artifact carries no authored body", what)
	}
	return text, nil
}

// buildProseClaims assembles the complete sorted, unique-by-id prose
// universe this package can derive: applicable policy instructions, the
// target's own problem/outcome/AC/open-question/constraint/decision
// prose, the same categories from each governing parent feature, each
// declared-context ADR's decision, and obligation declarations.
func buildProseClaims(authority PolicyAuthority, selection authoritySelection, target ResolvedSpec, fragments []FeatureFragment, obligations []BoundObligation, declared []DeclaredContextItem, governingScope policyartifact.Scope) ([]ProseClaim, error) {
	var out []ProseClaim
	out = append(out, buildPolicyInstructionProse(authority, selection)...)
	out = append(out, buildSpecProse(target.Ref, target.Path, target.ContentDigest, target.Spec, governingScope, authority.EffectiveDigest)...)
	for _, f := range fragments {
		out = append(out, buildFragmentProse(f, governingScope, authority.EffectiveDigest)...)
	}
	adrClaims, err := buildADRDecisionProse(declared, governingScope, authority.EffectiveDigest)
	if err != nil {
		return nil, err
	}
	out = append(out, adrClaims...)
	obligationClaims, err := buildObligationProse(obligations, governingScope, authority.EffectiveDigest)
	if err != nil {
		return nil, err
	}
	out = append(out, obligationClaims...)

	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	for i := 1; i < len(out); i++ {
		if out[i].ID == out[i-1].ID {
			return nil, fmt.Errorf("contextcompile: conflict prose claims: duplicate claim id %q", out[i].ID)
		}
	}
	return out, nil
}

// newProseClaim builds one ProseClaim whose scope inherits the given
// governing (request/candidate) scope's phase/environment/path dimensions
// narrowed to exactly this object's own ref — never a broader claim than
// the object it names.
func newProseClaim(ref, path, sourceDigest, object, category, text string, governingScope policyartifact.Scope, authorityDigest string) ProseClaim {
	lineIdentity := ref + "#" + object
	return ProseClaim{
		ID:           lineIdentity,
		Category:     category,
		Text:         text,
		TextDigest:   rawContentDigest([]byte(text)),
		SourceRef:    ref,
		SourcePath:   path,
		SourceDigest: sourceDigest,
		Scope: policyartifact.Scope{
			Phases:       cloneStrings(governingScope.Phases),
			Environments: cloneStrings(governingScope.Environments),
			Paths:        cloneStrings(governingScope.Paths),
			Refs:         []string{lineIdentity},
		},
		AuthorityDigest: authorityDigest,
		Object:          object,
		LineIdentity:    lineIdentity,
	}
}

// buildPolicyInstructionProse returns one ProseClaim per author-ordered
// instruction line of every applicable policy operand.
func buildPolicyInstructionProse(authority PolicyAuthority, selection authoritySelection) []ProseClaim {
	entryByID := make(map[string]policyauthority.EffectivePolicyEntry, len(authority.Effective.Policies))
	for _, e := range authority.Effective.Policies {
		entryByID[e.PolicyID] = e
	}

	var out []ProseClaim
	for _, op := range selection.Operands {
		if op.Kind != PolicyEntryPolicy {
			continue
		}
		entry, ok := entryByID[op.ID]
		if !ok {
			continue
		}
		for i, instr := range entry.Instructions {
			object := fmt.Sprintf("instruction-%d", i+1)
			out = append(out, ProseClaim{
				ID:              op.ID + "#" + object,
				Category:        categoryPolicyInstruction,
				Text:            instr,
				TextDigest:      rawContentDigest([]byte(instr)),
				SourceRef:       op.ID,
				SourcePath:      op.Path,
				SourceDigest:    op.Digest,
				Scope:           cloneScope(op.Scope),
				AuthorityDigest: authority.EffectiveDigest,
				Object:          object,
				LineIdentity:    op.ID + "#" + object,
			})
		}
	}
	return out
}

// buildSpecProse returns the target's own problem/outcome/AC/open-
// question/constraint/decision prose.
func buildSpecProse(ref, path, contentDigest string, spec *artifact.SpecFrontmatter, governingScope policyartifact.Scope, authorityDigest string) []ProseClaim {
	var out []ProseClaim
	if spec.Problem != nil {
		out = append(out, newProseClaim(ref, path, contentDigest, "problem", categorySpecProblem, spec.Problem.Text, governingScope, authorityDigest))
	}
	if spec.Outcome != nil {
		out = append(out, newProseClaim(ref, path, contentDigest, "outcome", categorySpecOutcome, spec.Outcome.Text, governingScope, authorityDigest))
	}
	for _, ac := range spec.AcceptanceCriteria {
		out = append(out, newProseClaim(ref, path, contentDigest, ac.ID, categoryAcceptanceCriterion, ac.Text, governingScope, authorityDigest))
	}
	for _, oq := range spec.OpenQuestions {
		out = append(out, newProseClaim(ref, path, contentDigest, oq.ID, categoryOpenQuestion, oq.Text, governingScope, authorityDigest))
	}
	for _, co := range spec.Constraints {
		out = append(out, newProseClaim(ref, path, contentDigest, co.ID, categoryConstraint, co.Text, governingScope, authorityDigest))
	}
	for _, dc := range spec.Decisions {
		out = append(out, newProseClaim(ref, path, contentDigest, dc.ID, categoryDecision, dc.Text, governingScope, authorityDigest))
	}
	return out
}

// buildFragmentProse returns the same feature-object categories as
// buildSpecProse, projected from one governing parent-feature fragment.
func buildFragmentProse(f FeatureFragment, governingScope policyartifact.Scope, authorityDigest string) []ProseClaim {
	ref, path, digest := f.Feature.Ref, f.Feature.Path, f.Feature.SourceDigest
	out := []ProseClaim{
		newProseClaim(ref, path, digest, "problem", categorySpecProblem, f.Problem.Text, governingScope, authorityDigest),
		newProseClaim(ref, path, digest, "outcome", categorySpecOutcome, f.Outcome.Text, governingScope, authorityDigest),
	}
	for _, t := range f.Targets {
		category := categoryAcceptanceCriterion
		if strings.HasPrefix(t.ID, "oq-") {
			category = categoryOpenQuestion
		}
		out = append(out, newProseClaim(ref, path, digest, t.ID, category, t.Text, governingScope, authorityDigest))
	}
	for _, co := range f.Constraints {
		out = append(out, newProseClaim(ref, path, digest, co.ID, categoryConstraint, co.Text, governingScope, authorityDigest))
	}
	for _, dc := range f.Decisions {
		out = append(out, newProseClaim(ref, path, digest, dc.ID, categoryDecision, dc.Text, governingScope, authorityDigest))
	}
	return out
}

// declaredADRItems returns the declared-context items that are ADRs — the
// one declared-context kind this package projects into the §6 semantic
// universe. It is the single home of that filter rule, so the prose
// builder and the snapshot's source set can never disagree about which
// declared artifacts this snapshot actually binds.
func declaredADRItems(declared []DeclaredContextItem) []DeclaredContextItem {
	out := make([]DeclaredContextItem, 0, len(declared))
	for _, item := range declared {
		if item.Kind == artifact.KindADR {
			out = append(out, item)
		}
	}
	return out
}

// buildADRDecisionProse returns one adr-decision ProseClaim per declared-
// context ADR: its text the ADR's normalized authored body (the ADR's
// decision authority in full — artifact.ADRFrontmatter has no structured
// decision field), its source identity the exact pinned declared ref, path,
// and RAW content digest. Normalization never changes a source digest: the
// claim's TextDigest covers the normalized text, SourceDigest the exact
// pinned bytes.
func buildADRDecisionProse(declared []DeclaredContextItem, governingScope policyartifact.Scope, authorityDigest string) ([]ProseClaim, error) {
	items := declaredADRItems(declared)
	out := make([]ProseClaim, 0, len(items))
	for _, item := range items {
		text, err := normalizeAuthorityProse(item.Ref, item.Content)
		if err != nil {
			return nil, fmt.Errorf("contextcompile: conflict prose claims: %w", err)
		}
		out = append(out, newProseClaim(item.Ref, item.Path, item.ContentDigest, conflictADRDecisionObject, categoryADRDecision, text, governingScope, authorityDigest))
	}
	return out, nil
}

// buildObligationProse returns one obligation-declaration ProseClaim per
// bound obligation, its text the obligation's normalized authored body —
// never the whole artifact, whose frontmatter is machinery identity, not
// authored authority prose.
func buildObligationProse(obligations []BoundObligation, governingScope policyartifact.Scope, authorityDigest string) ([]ProseClaim, error) {
	out := make([]ProseClaim, 0, len(obligations))
	for _, o := range obligations {
		text, err := normalizeAuthorityProse(o.Ref, o.Content)
		if err != nil {
			return nil, fmt.Errorf("contextcompile: conflict prose claims: %w", err)
		}
		out = append(out, ProseClaim{
			ID:           o.Ref,
			Category:     categoryObligationDeclaration,
			Text:         text,
			TextDigest:   rawContentDigest([]byte(text)),
			SourceRef:    o.Ref,
			SourcePath:   o.Path,
			SourceDigest: o.ContentDigest,
			Scope: policyartifact.Scope{
				Phases:       cloneStrings(governingScope.Phases),
				Environments: cloneStrings(governingScope.Environments),
				Paths:        cloneStrings(governingScope.Paths),
				Refs:         []string{o.Ref},
			},
			AuthorityDigest: authorityDigest,
			Object:          o.AC,
			LineIdentity:    o.Ref,
		})
	}
	return out, nil
}

// --- deep cloning (mutation-safety: authority design §3/§12) ---------------

// cloneIntPtr returns a fresh pointer to a copy of *p, or nil.
func cloneIntPtr(p *int) *int {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

// cloneConflictView returns a deep copy of in: every nested slice, map,
// and typed pointer field this package can allocate is freshly allocated,
// so neither the caller's original value nor the value cloneConflictView
// returns can observe or cause a mutation through the other. Unexported
// seal fields nested inside EffectivePolicy/Profile/PrincipalResolution
// survive: a plain top-level struct assignment (`out := in`) copies every
// field, exported or not, before this function replaces only the mutable
// exported slice/map fields with fresh copies.
//
// ONE deliberate exception, with its own guard: the interface values in
// EffectivePolicyEntry.Payloads (policyartifact.Payload) are copied as
// interface values, so a payload implemented by a pointer type stays
// SHARED between every clone — this package cannot deep-copy an open
// interface without owning its implementations, and adding a Clone method
// to policyartifact is outside this seam. Mutation through a returned view
// is therefore DETECTED rather than prevented: the shared payload is
// inside the sealed canonical digest, so the next View() reseals, sees a
// different digest, and fails integrity verification. No caller ever reads
// a silently mutated payload; a mutating caller loses the operands.
func cloneConflictView(in ConflictView) ConflictView {
	return ConflictView{
		Snapshot:        cloneSnapshotIdentity(in.Snapshot),
		EffectivePolicy: cloneEffectivePolicy(in.EffectivePolicy),
		TypedClaims:     cloneTypedClaims(in.TypedClaims),
		ProseClaims:     cloneProseClaims(in.ProseClaims),
		Exemptions:      cloneExemptionsSlice(in.Exemptions),
		Profile:         cloneProfile(in.Profile),
		Actors:          cloneActorResolutions(in.Actors),
	}
}

func cloneSnapshotIdentity(in SnapshotIdentity) SnapshotIdentity {
	out := in
	out.Scope = cloneScope(in.Scope)
	out.Sources = append([]ConflictSourceIdentity{}, in.Sources...)
	return out
}

func cloneTypedClaims(in []TypedClaim) []TypedClaim {
	out := make([]TypedClaim, len(in))
	for i, c := range in {
		out[i] = TypedClaim{
			PolicyID:     c.PolicyID,
			PolicyDigest: c.PolicyDigest,
			ClaimDigest:  c.ClaimDigest,
			Claim:        cloneClaim(c.Claim),
		}
	}
	return out
}

func cloneClaim(in policyartifact.Claim) policyartifact.Claim {
	out := in
	out.Values = append([]string{}, in.Values...)
	out.Bound = cloneIntPtr(in.Bound)
	out.Scope = cloneScope(in.Scope)
	return out
}

func cloneProseClaims(in []ProseClaim) []ProseClaim {
	out := make([]ProseClaim, len(in))
	for i, c := range in {
		out[i] = c
		out[i].Scope = cloneScope(c.Scope)
	}
	return out
}

func cloneExemptionsSlice(in []policyartifact.Exemption) []policyartifact.Exemption {
	out := make([]policyartifact.Exemption, len(in))
	for i, e := range in {
		out[i] = cloneExemption(e)
	}
	return out
}

func cloneExemption(in policyartifact.Exemption) policyartifact.Exemption {
	out := in // preserves the unexported seal field
	out.Owners = append([]string{}, in.Owners...)
	out.Scope = cloneScope(in.Scope)
	out.Witnesses = append([]policyartifact.Witness{}, in.Witnesses...)
	out.CompensatingControls = append([]string{}, in.CompensatingControls...)
	out.Approvals = append([]policyartifact.Approval{}, in.Approvals...)
	if in.Template != nil {
		t := *in.Template
		out.Template = &t
	}
	return out
}

func cloneEffectivePolicy(in policyauthority.EffectivePolicy) policyauthority.EffectivePolicy {
	out := in // preserves the unexported seal field
	out.Policies = make([]policyauthority.EffectivePolicyEntry, len(in.Policies))
	for i, e := range in.Policies {
		out.Policies[i] = cloneEffectivePolicyEntry(e)
	}
	out.Exemptions = make([]policyauthority.EffectiveExemption, len(in.Exemptions))
	for i, e := range in.Exemptions {
		out.Exemptions[i] = cloneEffectiveExemption(e)
	}
	out.Dispositions = make([]policyauthority.EffectiveDisposition, len(in.Dispositions))
	for i, d := range in.Dispositions {
		out.Dispositions[i] = cloneEffectiveDisposition(d)
	}
	return out
}

func cloneEffectivePolicyEntry(in policyauthority.EffectivePolicyEntry) policyauthority.EffectivePolicyEntry {
	out := in
	out.Claims = make([]policyauthority.EffectiveClaim, len(in.Claims))
	for i, c := range in.Claims {
		out.Claims[i] = cloneEffectiveClaim(c)
	}
	out.Instructions = append([]string{}, in.Instructions...)
	if in.Payloads != nil {
		// A fresh MAP, but the same interface values: a pointer-implemented
		// policyartifact.Payload is shared by every clone (see
		// cloneConflictView's exception note). Adding or removing a key
		// through one view cannot affect another; mutating a shared
		// payload's own fields is caught by the next View()'s seal recheck,
		// not prevented here.
		payloads := make(map[string]policyartifact.Payload, len(in.Payloads))
		for k, v := range in.Payloads {
			payloads[k] = v
		}
		out.Payloads = payloads
	}
	return out
}

func cloneEffectiveClaim(in policyauthority.EffectiveClaim) policyauthority.EffectiveClaim {
	out := in
	out.Scope = cloneScope(in.Scope)
	out.Values = append([]string{}, in.Values...)
	out.Bound = cloneIntPtr(in.Bound)
	out.Refinements = make([]policyauthority.ScopedRefinement, len(in.Refinements))
	for i, r := range in.Refinements {
		out.Refinements[i] = cloneScopedRefinement(r)
	}
	return out
}

func cloneScopedRefinement(in policyauthority.ScopedRefinement) policyauthority.ScopedRefinement {
	out := in
	out.Scope = cloneScope(in.Scope)
	out.Values = append([]string(nil), in.Values...)
	out.Bound = cloneIntPtr(in.Bound)
	out.Overlays = append([]string{}, in.Overlays...)
	return out
}

func cloneEffectiveExemption(in policyauthority.EffectiveExemption) policyauthority.EffectiveExemption {
	out := in
	out.Witnesses = append([]policyartifact.Witness{}, in.Witnesses...)
	out.Scope = cloneScope(in.Scope)
	out.Owners = append([]string{}, in.Owners...)
	out.Approvals = append([]policyartifact.Approval{}, in.Approvals...)
	return out
}

func cloneEffectiveDisposition(in policyauthority.EffectiveDisposition) policyauthority.EffectiveDisposition {
	out := in
	out.Disposition = clonePolicyDisposition(in.Disposition)
	return out
}

func clonePolicyDisposition(in policyartifact.Disposition) policyartifact.Disposition {
	out := in // preserves the unexported seal field
	out.Owners = append([]string{}, in.Owners...)
	out.Scope = cloneScope(in.Scope)
	out.Witness = cloneSemanticWitness(in.Witness)
	out.CompensatingControls = append([]string{}, in.CompensatingControls...)
	out.Approvals = append([]policyartifact.Approval{}, in.Approvals...)
	if in.Judgment != nil {
		j := *in.Judgment
		out.Judgment = &j
	}
	if in.Template != nil {
		t := *in.Template
		out.Template = &t
	}
	return out
}

func cloneSemanticWitness(in policyartifact.SemanticWitness) policyartifact.SemanticWitness {
	claims := make([]policyartifact.SemanticClaimWitness, len(in.Claims))
	for i, c := range in.Claims {
		cc := c
		cc.Scope = cloneScope(c.Scope)
		cc.Values = append([]string{}, c.Values...)
		cc.Bound = cloneIntPtr(c.Bound)
		claims[i] = cc
	}
	return policyartifact.SemanticWitness{
		InputID:      in.InputID,
		TargetDigest: in.TargetDigest,
		Claims:       claims,
		Exemptions:   append([]policyartifact.SemanticExemptionWitness{}, in.Exemptions...),
	}
}

func cloneProfile(in governanceprincipal.Profile) governanceprincipal.Profile {
	out := in // preserves the unexported seal field
	out.ApplicableTransitions = append([]string{}, in.ApplicableTransitions...)
	out.IdentityTrustSources = append([]governanceprincipal.TrustSource{}, in.IdentityTrustSources...)

	out.RoleMappings = make([]governanceprincipal.RoleMapping, len(in.RoleMappings))
	for i, r := range in.RoleMappings {
		out.RoleMappings[i] = governanceprincipal.RoleMapping{Role: r.Role, TrustSource: r.TrustSource, Subjects: append([]string{}, r.Subjects...)}
	}
	out.OwnershipSources = make([]governanceprincipal.OwnershipSource, len(in.OwnershipSources))
	for i, o := range in.OwnershipSources {
		out.OwnershipSources[i] = governanceprincipal.OwnershipSource{ID: o.ID, TrustSource: o.TrustSource, Transitions: append([]string{}, o.Transitions...), Roles: append([]string{}, o.Roles...)}
	}
	out.SignatureRequirements = make([]governanceprincipal.SignatureRequirement, len(in.SignatureRequirements))
	for i, s := range in.SignatureRequirements {
		out.SignatureRequirements[i] = governanceprincipal.SignatureRequirement{Transitions: append([]string{}, s.Transitions...), Roles: append([]string{}, s.Roles...), TrustSources: append([]string{}, s.TrustSources...)}
	}
	out.RequiredApprovers = make([]governanceprincipal.ApproverRequirement, len(in.RequiredApprovers))
	for i, a := range in.RequiredApprovers {
		out.RequiredApprovers[i] = governanceprincipal.ApproverRequirement{Transitions: append([]string{}, a.Transitions...), Roles: append([]string{}, a.Roles...), Minimum: a.Minimum}
	}
	out.DistinctnessRules = make([]governanceprincipal.DistinctnessRule, len(in.DistinctnessRules))
	for i, d := range in.DistinctnessRules {
		out.DistinctnessRules[i] = governanceprincipal.DistinctnessRule{Transitions: append([]string{}, d.Transitions...), LeftRole: d.LeftRole, RightRole: d.RightRole, Relation: d.Relation}
	}
	out.EvidenceSourceRestrictions = make([]governanceprincipal.EvidenceSourceRestriction, len(in.EvidenceSourceRestrictions))
	for i, e := range in.EvidenceSourceRestrictions {
		out.EvidenceSourceRestrictions[i] = governanceprincipal.EvidenceSourceRestriction{Transitions: append([]string{}, e.Transitions...), AllowedSources: append([]string{}, e.AllowedSources...)}
	}
	out.EscalationThresholds = make([]governanceprincipal.EscalationThreshold, len(in.EscalationThresholds))
	for i, t := range in.EscalationThresholds {
		out.EscalationThresholds[i] = governanceprincipal.EscalationThreshold{Transitions: append([]string{}, t.Transitions...), Metric: t.Metric, AtLeast: t.AtLeast, RequiredRoles: append([]string{}, t.RequiredRoles...)}
	}
	return out
}

// cloneActorResolutions mirrors actors.go's own projectActors copy
// discipline: a top-level value copy (preserving each resolution's
// unexported seal field) plus a fresh Witnesses slice per resolution.
func cloneActorResolutions(in []governanceprincipal.PrincipalResolution) []governanceprincipal.PrincipalResolution {
	out := make([]governanceprincipal.PrincipalResolution, len(in))
	for i, r := range in {
		out[i] = r
		out[i].Witnesses = append([]governanceprincipal.Witness(nil), r.Witnesses...)
	}
	return out
}
