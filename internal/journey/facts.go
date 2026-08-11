package journey

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/repositoryfacts"
	"github.com/jyang234/verdi/internal/specstate"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/storyresolve"
)

// Searched phrases name, per ref form, EXACTLY what resolution looked in
// — the two forms search genuinely different things, and a refusal that
// claims otherwise is a false statement about the evidence (CO-1's
// honesty applies to a refusal's own account of itself, not only to a
// record's fields). Both are fixed and machine-independent: no path, no
// branch name, nothing that varies by checkout.
const (
	// searchedSpecRef is the direct spec/<name> form's reach:
	// resolveDirectSpecRef.
	searchedSpecRef = "checked the working tree's active and archive zones and both zones at the configured default branch"
	// searchedStoryRef is the scheme-prefixed story-ref form's reach:
	// only the working tree's active zone is scanned, for BOTH classes'
	// story: fields (resolveStoryRef). The default branch is never
	// consulted for this form. The two classes are deliberately not
	// enumerated in the text: "every active spec's story: field" says the
	// same thing without spelling class words a store may have renamed
	// (L-M13a(6)'s enumeration rule; no *model.Model is in scope at a
	// package-level const).
	searchedStoryRef = "checked every active spec's story: field in the working tree's active zone"
)

// NotFoundError is GatherFacts's typed refusal when arg resolves to no
// spec this projection can see. Searched names what was actually scanned
// for arg's own form (searchedSpecRef / searchedStoryRef); the zero value
// falls back to a form-agnostic phrase that claims nothing specific. A
// caller (the CLI lane) maps this to exit 2 — an argument that resolves to
// nothing is operational, not a lifecycle verdict of its own.
type NotFoundError struct {
	Ref      string
	Searched string
}

func (e *NotFoundError) Error() string {
	searched := e.Searched
	if searched == "" {
		searched = "no spec with this ref was found"
	}
	return fmt.Sprintf("journey: %s resolves to no spec (%s)", e.Ref, searched)
}

// Facts is the complete repository- and lifecycle-fact basis for a journey
// Record, gathered by GatherFacts but not yet assembled into one — that
// join (deriving blockers, principals, and safe actions from these facts
// plus the operating-model catalog) is Project's job. LifecycleResult
// carries the raw specstate.Result gathering already resolved, retained so
// the derivation stage can call its own Result.ArtifactStatus() join
// (DC-15: journey never reinterprets or re-derives a lifecycle verdict)
// without a second git/specstate round trip. RepositoryDisclosures and
// Owners are inputs the derivation stage needs but that have no field of
// their own on RepositoryFacts (which carries no disclosures list) or on
// the Record itself (Owner attribution, not schema).
type Facts struct {
	Target                Target
	Repository            RepositoryFacts
	RepositoryDisclosures []string
	Lifecycle             LifecycleFacts
	LifecycleResult       specstate.Result
	Evidence              EvidenceFacts
	Owners                []string
}

// GatherFacts resolves arg to a target spec (I-30's two-form contract),
// reads and strict-decodes its evaluated bytes, and gathers every
// repository and lifecycle fact AC-1 names — but does not yet derive
// blockers, principals, or safe actions (Project's job, layered on top).
func (p Projector) GatherFacts(ctx context.Context, cfg *store.Config, arg string) (Facts, error) {
	root := cfg.Root

	name, relPath, content, foundOnDisk, err := p.resolveTargetBytes(ctx, root, arg)
	if err != nil {
		return Facts{}, err
	}

	spec, err := decodeTargetSpec(arg, content)
	if err != nil {
		return Facts{}, err
	}

	repoFacts, repoDisclosures, err := p.gatherRepositoryFacts(ctx, root, relPath, content, foundOnDisk)
	if err != nil {
		return Facts{}, err
	}

	lifeFacts, result, err := p.gatherLifecycleFacts(ctx, root, relPath, name, content, spec)
	if err != nil {
		return Facts{}, err
	}

	return Facts{
		Target:                Target{Ref: arg, Class: string(spec.Class), Path: relPath},
		Repository:            repoFacts,
		RepositoryDisclosures: repoDisclosures,
		Lifecycle:             lifeFacts,
		LifecycleResult:       result,
		Evidence:              gatherEvidenceFacts(spec),
		Owners:                spec.Owners,
	}, nil
}

// --- target resolution --------------------------------------------------

// resolveTargetBytes resolves arg per I-30's strict two-form contract: a
// spec/<name> ref is read directly, active-then-archive-then-default-
// branch (mirroring cmd/verdi/specstate.go's readSpecBytesEitherZone, with
// the default-branch fallback this projection additionally needs); a
// scheme-prefixed story ref is resolved by resolveStoryRef against BOTH
// spec classes' story: fields and then re-read from the active zone.
// foundOnDisk is false only when the direct-ref form fell back to the
// default branch (Source == "remote-ref"); it is always true for the
// story-ref form, which only ever matches specs already present in the
// working tree's active zone.
func (p Projector) resolveTargetBytes(ctx context.Context, root, arg string) (name, relPath string, content []byte, foundOnDisk bool, err error) {
	if ref, perr := artifact.ParseRef(arg); perr == nil && ref.Kind == artifact.KindSpec {
		relPath, content, foundOnDisk, err = p.resolveDirectSpecRef(ctx, root, ref.Name)
		return ref.Name, relPath, content, foundOnDisk, err
	}

	if scheme, key, ok := strings.Cut(arg, ":"); ok && scheme != "" && key != "" {
		spec, rerr := p.resolveStoryRef(root, arg)
		if rerr != nil {
			return "", "", nil, false, rerr
		}
		specRef, perr := artifact.ParseRef(spec.ID)
		if perr != nil {
			return "", "", nil, false, fmt.Errorf("journey: resolved spec id %q: %w", spec.ID, perr)
		}
		activePath := store.ActiveSpecPath(root, specRef.Name)
		data, rerr2 := os.ReadFile(activePath)
		if rerr2 != nil {
			return "", "", nil, false, fmt.Errorf("journey: re-reading resolved spec %s: %w", spec.ID, rerr2)
		}
		return specRef.Name, store.ActiveSpecRelPath(specRef.Name), data, true, nil
	}

	// vocab:identity — the "story ref" FIELD-form grammar (I-30; mirrors storyresolve.Resolve's own refusal text)
	return "", "", nil, false, fmt.Errorf("journey: %q is neither a scheme-prefixed story ref (e.g. jira:LOAN-1482) nor a spec ref (e.g. spec/stale-decline); this verb accepts exactly those two forms", arg)
}

// resolveStoryRef resolves a scheme-prefixed story ref against BOTH spec
// classes' story: fields, over the working tree's active zone.
//
// AC-1's contract is "any feature OR story", so both classes must be
// reachable by the ref they carry. The two halves come from two different
// storyresolve entry points, deliberately: Resolve's shared story-ref
// scan is feature-class only (its own doc comment explains why widening
// it would silently change which spec several already-shipped consumers
// find), and MatchStoryClassRef answers the story-class half. Joining
// them here — rather than in either of them — keeps every other consumer
// byte-for-byte unchanged.
//
// Both classes legitimately carry the SAME tracker ref: a feature's
// OPTIONAL epic/objective story: field and a story's REQUIRED own story:
// field are different refs that may coincide, with no uniqueness rule
// between them (this module's own examples/showcase does exactly that:
// stale-decline, class: feature, and borrower-update-api, class: story,
// both carry story: jira:LOAN-1482). When that happens the ref names two
// different targets, and silently projecting either one is a wrong
// answer, so resolution fails closed naming every match and how to
// disambiguate. That refusal is a plain operational error, never a
// *NotFoundError: the ref resolves to too much, not to nothing.
func (p Projector) resolveStoryRef(root, arg string) (*artifact.SpecFrontmatter, error) {
	var matches []*artifact.SpecFrontmatter

	featureMatch, rerr := storyresolve.Resolve(root, arg)
	switch {
	case rerr == nil:
		matches = append(matches, featureMatch)
	case errors.As(rerr, new(*storyresolve.UnmatchedStoryRefError)):
		// Zero feature-class matches — an ordinary outcome here, not a
		// failure: the story-class half may still answer.
	default:
		// Anything else (a corrupt store walked into mid-scan, a
		// feature-versus-feature collision storyresolve refuses on its own)
		// is surfaced as-is.
		return nil, rerr
	}

	storyMatches, serr := storyresolve.MatchStoryClassRef(root, arg)
	if serr != nil {
		return nil, serr
	}
	matches = append(matches, storyMatches...)

	switch len(matches) {
	case 0:
		return nil, &NotFoundError{Ref: arg, Searched: searchedStoryRef}
	case 1:
		return matches[0], nil
	default:
		ids := make([]string, len(matches))
		for i, m := range matches {
			ids[i] = m.ID
		}
		sort.Strings(ids)
		// vocab:identity — "story ref" names the scheme-prefixed story: FIELD's ref form, and spec/<name> is the ref grammar the operator must retype (I-30); both identity, like storyresolve's own twin refusal
		return nil, fmt.Errorf("journey: story ref %q matches more than one active spec: %s; name the one you mean as spec/<name>", arg, strings.Join(ids, ", "))
	}
}

// resolveDirectSpecRef reads name's spec.md active-then-archive from the
// working tree (mirroring cmd/verdi/specstate.go's readSpecBytesEitherZone
// — that function is package main; this is the pattern, not the code).
// When neither zone has it locally, it additionally checks both zones at
// the configured default branch (foundOnDisk == false, Source ==
// "remote-ref") before returning a *NotFoundError.
func (p Projector) resolveDirectSpecRef(ctx context.Context, root, name string) (relPath string, content []byte, foundOnDisk bool, err error) {
	activePath := store.ActiveSpecPath(root, name)
	data, rerr := os.ReadFile(activePath)
	if rerr == nil {
		return store.ActiveSpecRelPath(name), data, true, nil
	}
	if !os.IsNotExist(rerr) {
		return "", nil, false, fmt.Errorf("journey: reading %s: %w", activePath, rerr)
	}

	archivePath := store.ArchiveSpecPath(root, name)
	data, rerr = os.ReadFile(archivePath)
	if rerr == nil {
		return store.SpecRelPath(store.ZoneArchive, name), data, true, nil
	}
	if !os.IsNotExist(rerr) {
		return "", nil, false, fmt.Errorf("journey: reading %s: %w", archivePath, rerr)
	}

	branch, ok := p.resolveDefaultBranch(ctx, root)
	if !ok {
		return "", nil, false, &NotFoundError{Ref: "spec/" + name, Searched: searchedSpecRef}
	}
	for _, zone := range []string{store.ZoneActive, store.ZoneArchive} {
		zoneRelPath := store.SpecRelPath(zone, name)
		shown, serr := p.git.Show(ctx, root, branch.Ref, zoneRelPath)
		if serr == nil {
			return zoneRelPath, shown, false, nil
		}
	}
	return "", nil, false, &NotFoundError{Ref: "spec/" + name, Searched: searchedSpecRef}
}

// decodeTargetSpec strict-decodes content's frontmatter and refuses any
// class other than feature or story (a component spec carries no story
// and no acceptance criteria — reusing storyresolve.ComponentSpecError's
// prose semantics; a plain error is sufficient here since this path is
// never reached through storyresolve.Resolve itself, whose own
// ComponentSpecError already covers its spec-ref branch).
func decodeTargetSpec(ref string, content []byte) (*artifact.SpecFrontmatter, error) {
	fm, _, err := artifact.SplitFrontmatter(content)
	if err != nil {
		return nil, fmt.Errorf("journey: %s: %w", ref, err)
	}
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		return nil, fmt.Errorf("journey: %s: %w", ref, err)
	}
	if spec.Class != artifact.ClassFeature && spec.Class != artifact.ClassStory {
		// vocab:identity — strict-decode/schema diagnostic naming class ids directly (mirrors internal/artifact/spec.go's own "vocab:identity — strict-decode/schema diagnostic" markers); no *model.Model is in scope at this call site to route the class words through
		return nil, fmt.Errorf("journey: %s is a %s spec (no story: field, no acceptance_criteria:); journey requires class: feature or class: story", ref, spec.Class)
	}
	return spec, nil
}

// --- repository facts -----------------------------------------------------

// gatherRepositoryFacts gathers RepositoryFacts (AC-1's repository-identity
// section) by delegating to the shared internal/repositoryfacts leaf
// (SI-85: "internal/journey and the context compiler consume that
// package"). journey's own remaining job here is exactly two things:
// pass through the computed Facts unchanged (RepositoryFacts is a type
// alias for repositoryfacts.Facts, so no conversion exists to omit), and
// map the shared leaf's closed, machine-stable DisclosureCode vocabulary
// to this projection's OWN byte-identical prose
// (repositoryDisclosureProse) — preserving every disclosure string a
// caller of GatherFacts already observes, without journey re-deriving or
// reinterpreting how any fact was established. Disclosures are returned
// unsorted (repositoryDisclosureProse preserves Gather's own sorted
// order, but Project sorts and dedupes every disclosure source together
// once at final assembly regardless).
func (p Projector) gatherRepositoryFacts(ctx context.Context, root, relPath string, content []byte, foundOnDisk bool) (RepositoryFacts, []string, error) {
	snap, err := p.repoFacts.Gather(ctx, repositoryfacts.GatherInput{
		Root:              root,
		TargetPath:        relPath,
		TargetContent:     content,
		TargetFoundOnDisk: foundOnDisk,
	})
	if err != nil {
		return RepositoryFacts{}, nil, fmt.Errorf("journey: gathering repository facts: %w", err)
	}

	disclosures := make([]string, 0, len(snap.Disclosures))
	for _, code := range snap.Disclosures {
		prose, ok := repositoryDisclosureProse(code)
		if !ok {
			// A shared-leaf disclosure code this projection's own mapping
			// table does not recognize is a contract-drift bug between
			// internal/repositoryfacts and this package, not an ordinary
			// unprovable repository fact — fail closed rather than
			// silently dropping the disclosure (CO-1: silence is never a
			// pass).
			return RepositoryFacts{}, nil, fmt.Errorf("journey: unmapped repository disclosure code %q", code)
		}
		disclosures = append(disclosures, prose)
	}

	return snap.Facts, disclosures, nil
}

// repositoryDisclosureProse maps a repositoryfacts.DisclosureCode to this
// projection's own fixed, machine-independent prose — byte-identical to
// what internal/journey's pre-extraction gatherRepositoryFacts produced
// for the same cause, so the SI-85 extraction changes no observable
// journey output. ok is false for a code this table does not recognize.
func repositoryDisclosureProse(code repositoryfacts.DisclosureCode) (string, bool) {
	switch code {
	case repositoryfacts.DisclosureRemoteOriginUncanonicalizable:
		return "the origin remote URL could not be canonicalized to a repository identity", true
	case repositoryfacts.DisclosureRemoteOriginNotConfigured:
		return "no origin remote is configured", true
	case repositoryfacts.DisclosureRemoteOriginReadFailed:
		return "remote origin could not be read from this checkout", true
	case repositoryfacts.DisclosureBranchUnresolved:
		return "the current branch could not be determined from this checkout", true
	case repositoryfacts.DisclosureBranchDetached:
		return "the repository is in a detached HEAD state; the current branch is unknown", true
	case repositoryfacts.DisclosureHeadUnresolved:
		return "HEAD could not be resolved from this checkout", true
	case repositoryfacts.DisclosureDefaultBranchRefUnresolved:
		return "the resolved default branch ref could not be resolved to a commit", true
	case repositoryfacts.DisclosureDirtyUnknown:
		return "working-tree dirty state could not be determined from this checkout", true
	case repositoryfacts.DisclosureStagedUnknown:
		return "staged paths could not be determined from this checkout", true
	default:
		return "", false
	}
}

// --- evidence facts --------------------------------------------------------

// evidenceOperandsUnknownDisclosure is the fixed disclosure this delivery
// unit always carries for the two always-visible evidence operands DC-2
// names. Context Integrity owns the canonical evidence-authority and
// freshness operands; DC-15 is explicit that the journey CONSUMES them
// when available and DISCLOSES their absence otherwise, never recomputing
// or guessing a posture of its own.
const evidenceOperandsUnknownDisclosure = "Context Integrity's canonical evidence-authority and freshness operands are not consumed by this delivery unit; evidence posture and freshness are unknown"

// noEvidenceContributorsDisclosure is the defensive branch's disclosure: a
// target declaring no acceptance-criteria evidence kinds yields no
// contributors, and an empty list must never be read as "this target needs
// no evidence" (CO-1). artifact.DecodeSpec already refuses a feature or
// story spec with no acceptance criteria (and every criterion with no
// evidence kinds), so this is unreachable through GatherFacts today — it
// exists so the section stays honest if that ever changes.
const noEvidenceContributorsDisclosure = "the target declares no acceptance-criteria evidence kinds; this projection derives no evidence contributors for it"

// evidenceContributorWitness is a contributor's fixed, machine-independent
// witness: this delivery unit wires no evidence source, so every derived
// contributor is unproven and says exactly why (CO-1: silence is never a
// pass, and an unproven resolution with no witness IS silence).
func evidenceContributorWitness(kind string) string {
	return fmt.Sprintf("evidence kind %s is declared by the target's acceptance criteria, but no evidence source is wired to this projection, so its resolution is unproven", kind)
}

// gatherEvidenceFacts derives the record's evidence section (DC-2's
// always-visible evidence authority and freshness operands) from the
// target's own frontmatter.
//
// Authority and Freshness are both "unknown" in this delivery unit, with
// the reason disclosed (DC-15): Context Integrity owns those canonical
// operands and this projection consumes none of them yet. Contributors
// are one per DISTINCT evidence kind the target's acceptance criteria
// declare — the kind is the contributor's id, so the set is inherently
// deduplicated — each resolved "unproven" with its own witness, sorted
// ascending by id for determinism.
func gatherEvidenceFacts(spec *artifact.SpecFrontmatter) EvidenceFacts {
	kinds := map[string]bool{}
	for _, ac := range spec.AcceptanceCriteria {
		for _, k := range ac.Evidence {
			kinds[string(k)] = true
		}
	}

	ids := make([]string, 0, len(kinds))
	for k := range kinds {
		ids = append(ids, k)
	}
	sort.Strings(ids)

	contributors := make([]EvidenceContributor, 0, len(ids))
	for _, id := range ids {
		contributors = append(contributors, EvidenceContributor{
			ID:         id,
			Kind:       id,
			Resolution: "unproven",
			Witness:    evidenceContributorWitness(id),
		})
	}

	disclosures := []string{evidenceOperandsUnknownDisclosure}
	if len(contributors) == 0 {
		disclosures = append(disclosures, noEvidenceContributorsDisclosure)
	}

	return EvidenceFacts{
		Authority:    "unknown",
		Freshness:    "unknown",
		Contributors: contributors,
		Disclosures:  sortDedupStrings(disclosures),
	}
}

// --- lifecycle facts -------------------------------------------------------

// gatherLifecycleFacts gathers LifecycleFacts (AC-1's lifecycle section)
// via the shared StateResolver port — the ONE place lifecycle state is
// derived (specstate's own package doc; R-9's cross-feature constraint:
// "lifecycle state comes ONLY from internal/specstate"). The raw
// specstate.Result is also returned so a later derivation stage can call
// Result.ArtifactStatus() itself rather than this package inventing a
// second literal-to-literal status mapping. content is the SAME evaluated
// bytes GatherFacts already resolved (working-tree, archive, or
// default-branch — whichever Source names), so the candidate this
// resolves lifecycle state for is exactly what the record's repository
// section reports was evaluated.
func (p Projector) gatherLifecycleFacts(ctx context.Context, root, relPath, name string, content []byte, spec *artifact.SpecFrontmatter) (LifecycleFacts, specstate.Result, error) {
	result, err := p.state.Resolve(ctx, root, specstate.Candidate{Path: relPath, Content: content})
	if err != nil {
		return LifecycleFacts{}, specstate.Result{}, fmt.Errorf("journey: resolving lifecycle state for %s: %w", relPath, err)
	}
	// F1(b): specstate's own disclosures may embed this checkout's
	// absolute store-root path (e.g. specstate's own "no default branch
	// could be resolved for <root>" — specstate/resolve.go's
	// unresolvedDefaultBranchResult). Sanitized ONCE, here, before the
	// result is used to build LifecycleFacts.Disclosures below AND before
	// it is returned as the raw specstate.Result a later derivation stage
	// (derive.go's deriveBlockers) reads its own blocker witnesses from —
	// one sanitization point serves both consumers. This is a disclosed
	// transformation for machine independence (CO-2/CO-4: canonical
	// output must not depend on which checkout path evaluated it), never
	// a reinterpretation of what specstate proved (DC-15).
	result.Disclosures = sanitizeDisclosures(root, result.Disclosures)

	lf := LifecycleFacts{
		Class:    string(spec.Class),
		State:    string(result.State),
		Relation: string(result.Relation),
		Posture:  derivePosture(result.State, result.Relation),
	}
	// journey.Baseline is the ACCEPTED-baseline identity: record.go's own
	// Validate requires Path/Blob/LandingCommit all non-empty together.
	// specstate.Result.Baseline is not always that complete: a diverged
	// candidate's own doc comment (specstate/resolve.go resolveOne) is
	// explicit that a diverged candidate deliberately carries a PARTIAL
	// baseline (Path/Blob populated, LandingCommit always "") — a
	// candidate that has never landed has no first-parent landing commit
	// to report, and specstate never computes one for that case. Mapping
	// a partial baseline into AcceptedBaseline would violate this
	// package's own schema (an accepted baseline the schema treats as
	// always-complete), so only a COMPLETE baseline (LandingCommit != "",
	// which specstate only ever sets alongside Path and Blob) becomes an
	// AcceptedBaseline; anything else — nil, or partial — leaves it nil,
	// the honest "no accepted baseline exists yet" reading.
	if result.Baseline != nil && result.Baseline.LandingCommit != "" {
		lf.AcceptedBaseline = &Baseline{
			Path:          result.Baseline.Path,
			Blob:          result.Baseline.Blob,
			LandingCommit: result.Baseline.LandingCommit,
		}
	}
	if spec.Frozen != nil {
		lf.Frozen = &FrozenRevision{At: spec.Frozen.At, Commit: spec.Frozen.Commit}
	}

	branchFact, branchDisclosure, err := p.resolveActiveBranch(ctx, root, name)
	if err != nil {
		return LifecycleFacts{}, specstate.Result{}, err
	}
	lf.ActiveBranch = branchFact

	merged := append([]string(nil), result.Disclosures...)
	if branchDisclosure != "" {
		merged = append(merged, branchDisclosure)
	}
	lf.Disclosures = sortDedupStrings(merged)

	return lf, result, nil
}

// derivePosture applies the fixed posture rule: "authoritative" iff
// Relation == exact and State is one of the three landed terminal-or-
// buildable states; "unknown" iff State == unproven; else "advisory".
func derivePosture(state specstate.State, relation specstate.Relation) string {
	if state == specstate.Unproven {
		return "unknown"
	}
	if relation == specstate.RelationExact {
		switch state {
		case specstate.AcceptedPendingBuild, specstate.Closed, specstate.Superseded:
			return "authoritative"
		}
	}
	return "advisory"
}

// resolveActiveBranch resolves LifecycleFacts.ActiveBranch: Known == true
// iff exactly one of the two conventional branches for name
// (design/<name>, `verdi design start`'s own cut — cmd/verdi/design.go;
// feature/<name>, `verdi build start`'s own cut — cmd/verdi/buildstart.go)
// exists, local or remote-tracking. A branch counts once even when it
// exists both locally and as a remote-tracking ref (the ordinary
// already-pushed case is not ambiguity). Zero matches is Known == false
// with no disclosure (nothing has started yet); two or more distinct
// branch names is Known == false with a disclosure naming every match.
func (p Projector) resolveActiveBranch(ctx context.Context, root, name string) (StringFact, string, error) {
	candidates := []string{"design/" + name, "feature/" + name}
	var matched []string
	for _, cand := range candidates {
		local, lerr := p.git.HasLocalBranch(ctx, root, cand)
		if lerr != nil {
			return StringFact{}, "", fmt.Errorf("journey: checking local branch %s: %w", cand, lerr)
		}
		remote, rerr := p.git.HasRemoteTrackingBranch(ctx, root, "origin", cand)
		if rerr != nil {
			return StringFact{}, "", fmt.Errorf("journey: checking remote-tracking branch %s: %w", cand, rerr)
		}
		if local || remote {
			matched = append(matched, cand)
		}
	}
	switch len(matched) {
	case 0:
		return StringFact{Known: false}, "", nil
	case 1:
		return StringFact{Known: true, Value: matched[0]}, "", nil
	default:
		sort.Strings(matched)
		return StringFact{Known: false}, fmt.Sprintf("the active design or build branch is ambiguous for %s: %s", name, strings.Join(matched, ", ")), nil
	}
}

// sanitizeDisclosures replaces every literal occurrence of root inside ds
// with the fixed token "<store-root>" (F1(b)) — a disclosed transformation
// for machine independence (CO-2/CO-4): two evaluations of the same
// semantic inputs on different checkouts (different clone paths, worktree
// locations, or CI runners) must never diverge byte-for-byte in their
// canonical output merely because a disclosure happened to embed this
// process's own absolute filesystem path. An empty root or empty input is
// returned unchanged (a deliberate no-op guard, not a special case).
func sanitizeDisclosures(root string, ds []string) []string {
	if root == "" || len(ds) == 0 {
		return ds
	}
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = strings.ReplaceAll(d, root, "<store-root>")
	}
	return out
}

// sortDedupStrings returns ss sorted and deduplicated, always non-nil
// (Record's schema requires an explicit empty set, never a nil slice).
func sortDedupStrings(ss []string) []string {
	out := append([]string(nil), ss...)
	sort.Strings(out)
	deduped := out[:0]
	var last string
	seen := false
	for _, s := range out {
		if !seen || s != last {
			deduped = append(deduped, s)
			last = s
			seen = true
		}
	}
	if deduped == nil {
		deduped = []string{}
	}
	return deduped
}
