package journey

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/specstate"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/storyresolve"
	"github.com/jyang234/verdi/internal/wtmanager"
)

// NotFoundError is GatherFacts's typed refusal when arg resolves to no
// spec anywhere this projection can see: neither the working tree's
// active or archive zone, nor the configured default branch's active or
// archive zone (the direct spec/<name> form), nor any active feature
// spec's story: field (the scheme-prefixed story-ref form). A caller (the
// CLI lane) maps this to exit 2 — an argument that resolves to nothing is
// operational, not a lifecycle verdict of its own.
type NotFoundError struct {
	Ref string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("journey: %s resolves to no spec (checked the working tree and the configured default branch)", e.Ref)
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
		Owners:                spec.Owners,
	}, nil
}

// --- target resolution --------------------------------------------------

// resolveTargetBytes resolves arg per I-30's strict two-form contract: a
// spec/<name> ref is read directly, active-then-archive-then-default-
// branch (mirroring cmd/verdi/specstate.go's readSpecBytesEitherZone, with
// the default-branch fallback this projection additionally needs); a
// scheme-prefixed story ref is resolved via internal/storyresolve.Resolve
// and then re-read from the active zone. foundOnDisk is false only when
// the direct-ref form fell back to the default branch (Source ==
// "remote-ref"); it is always true for the story-ref form, since
// storyresolve.Resolve only ever matches specs already present in the
// working tree's active zone.
func (p Projector) resolveTargetBytes(ctx context.Context, root, arg string) (name, relPath string, content []byte, foundOnDisk bool, err error) {
	if ref, perr := artifact.ParseRef(arg); perr == nil && ref.Kind == artifact.KindSpec {
		relPath, content, foundOnDisk, err = p.resolveDirectSpecRef(ctx, root, ref.Name)
		return ref.Name, relPath, content, foundOnDisk, err
	}

	if scheme, key, ok := strings.Cut(arg, ":"); ok && scheme != "" && key != "" {
		spec, rerr := storyresolve.Resolve(root, arg)
		if rerr != nil {
			var unmatched *storyresolve.UnmatchedStoryRefError
			if errors.As(rerr, &unmatched) {
				return "", "", nil, false, &NotFoundError{Ref: arg}
			}
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
		return "", nil, false, &NotFoundError{Ref: "spec/" + name}
	}
	for _, zone := range []string{store.ZoneActive, store.ZoneArchive} {
		zoneRelPath := store.SpecRelPath(zone, name)
		shown, serr := p.git.Show(ctx, root, branch.Ref, zoneRelPath)
		if serr == nil {
			return zoneRelPath, shown, false, nil
		}
	}
	return "", nil, false, &NotFoundError{Ref: "spec/" + name}
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
// section). Every fact that goes Known == false because a git call itself
// failed (as opposed to a legitimately unprovable state like "no origin
// remote is configured" or "unresolved default branch") is paired with a
// deterministic disclosure appended to the returned list — never an
// invented value. Disclosures are returned unsorted; Project sorts and
// dedupes them once, together with every other source, at final assembly.
func (p Projector) gatherRepositoryFacts(ctx context.Context, root, relPath string, content []byte, foundOnDisk bool) (RepositoryFacts, []string, error) {
	var disclosures []string
	var rf RepositoryFacts

	switch url, rerr := p.git.RemoteURL(ctx, root, "origin"); {
	case rerr == nil:
		// AC-1's repository section reports a CANONICAL repository
		// identity, never the raw origin URL: the raw URL may carry
		// credentials (GLG v3's security decision — a journey projection
		// contains no credentials or secrets), and its ssh and https
		// spellings of one repository differ, which would make identity
		// and every digest over it checkout-dependent. Canonicalization
		// is gitx's (CanonicalRemoteIdentity); gitx.RemoteURL itself still
		// returns the raw URL for the callers that need it.
		if identity, ok := gitx.CanonicalRemoteIdentity(url); ok {
			rf.RemoteOrigin = StringFact{Known: true, Value: identity}
		} else {
			// Same F1(a) posture as a read failure: the raw URL is never
			// routed into the record OR into the disclosure (it is exactly
			// the string that may carry the credential), so the disclosure
			// names only the cause class, fixed and machine-independent.
			rf.RemoteOrigin = StringFact{Known: false}
			disclosures = append(disclosures, "the origin remote URL could not be canonicalized to a repository identity")
		}
	case errors.Is(rerr, gitx.ErrNoSuchRemote):
		rf.RemoteOrigin = StringFact{Known: false}
		disclosures = append(disclosures, "no origin remote is configured")
	default:
		// F1(a): the underlying gitx error (which may itself carry an
		// absolute path or raw git stderr text) is never routed into the
		// record — Known == false already carries the honesty; the
		// disclosure names only the cause class, fixed and machine-
		// independent.
		rf.RemoteOrigin = StringFact{Known: false}
		disclosures = append(disclosures, "remote origin could not be read from this checkout")
	}

	branch, berr := p.git.CurrentBranch(ctx, root)
	switch {
	case berr != nil:
		rf.Branch = StringFact{Known: false}
		disclosures = append(disclosures, "the current branch could not be determined from this checkout")
	case branch == "":
		rf.Branch = StringFact{Known: false}
		disclosures = append(disclosures, "the repository is in a detached HEAD state; the current branch is unknown")
	default:
		rf.Branch = StringFact{Known: true, Value: branch}
	}

	head, herr := p.git.RevParse(ctx, root, "HEAD")
	if herr != nil {
		rf.Head = StringFact{Known: false}
		disclosures = append(disclosures, "HEAD could not be resolved from this checkout")
	} else {
		rf.Head = StringFact{Known: true, Value: head}
	}

	var defaultHead string
	var defaultKnown bool
	if db, ok := p.resolveDefaultBranch(ctx, root); ok {
		if dh, derr := p.git.RevParse(ctx, root, db.Ref); derr == nil {
			rf.DefaultBranch = DefaultBranchFact{Known: true, Name: db.Name, Ref: db.Ref, Head: dh}
			defaultHead, defaultKnown = dh, true
		} else {
			// F2: the default branch NAME resolved, but its ref could not
			// be turned into a commit — a distinct, disclosed failure from
			// "no default branch resolves at all" (below/in derive.go),
			// never silently folded into the same unknown.
			disclosures = append(disclosures, "the resolved default branch ref could not be resolved to a commit")
		}
	}
	if !defaultKnown {
		rf.DefaultBranch = DefaultBranchFact{Known: false}
	}

	rf.Relationship = relationship(ctx, p.git, root, rf.Head, defaultHead, defaultKnown)

	dirty, derr := p.git.StatusDirty(ctx, root)
	if derr != nil {
		rf.Dirty = BoolFact{Known: false}
		disclosures = append(disclosures, "working-tree dirty state could not be determined from this checkout")
	} else {
		rf.Dirty = BoolFact{Known: true, Value: dirty}
	}

	staged, serr := p.git.StagedPaths(ctx, root)
	if serr != nil {
		rf.Staged = BoolFact{Known: false}
		disclosures = append(disclosures, "staged paths could not be determined from this checkout")
	} else {
		rf.Staged = BoolFact{Known: true, Value: len(staged) > 0}
	}

	rf.Worktree = worktreeFact(root)

	if !foundOnDisk {
		rf.Source = "remote-ref"
	} else {
		headBytes, serr := p.git.Show(ctx, root, "HEAD", relPath)
		if serr == nil && bytes.Equal(headBytes, content) {
			rf.Source = "head"
		} else {
			rf.Source = "working-tree"
		}
	}

	return rf, disclosures, nil
}

// relationship classifies HEAD against the default branch's HEAD: "equal"
// on identical shas; otherwise IsAncestor(default, HEAD) -> "ahead",
// IsAncestor(HEAD, default) -> "behind", neither -> "diverged"; unknown
// whenever either HEAD is itself unknown or an ancestry check errors.
func relationship(ctx context.Context, git GitReader, root string, head StringFact, defaultHead string, defaultKnown bool) string {
	if !head.Known || !defaultKnown {
		return "unknown"
	}
	if head.Value == defaultHead {
		return "equal"
	}
	ahead, aerr := git.IsAncestor(ctx, root, defaultHead, head.Value)
	if aerr != nil {
		return "unknown"
	}
	if ahead {
		return "ahead"
	}
	behind, berr := git.IsAncestor(ctx, root, head.Value, defaultHead)
	if berr != nil {
		return "unknown"
	}
	if behind {
		return "behind"
	}
	return "diverged"
}

// worktreeMarker is the "/.verdi/data/worktrees/" path segment a store
// root's managed-worktree membership is decided against, derived from
// wtmanager.WorktreesRoot (its own single home for the managed-worktree
// layout — CLAUDE.md: never a second hardcoded copy of that literal)
// rather than a second literal of this package's own.
func worktreeMarker() string {
	return "/" + filepath.ToSlash(wtmanager.WorktreesRoot("")) + "/"
}

// worktreeFact classifies root as a managed worktree iff its slash-
// normalized path contains worktreeMarker's segment; Name is the path
// segment immediately after it (wtmanager's own <name> == a design
// branch's spec name, naming.go's worktreeName).
func worktreeFact(root string) WorktreeFact {
	norm := filepath.ToSlash(root)
	marker := worktreeMarker()
	idx := strings.Index(norm, marker)
	if idx < 0 {
		return WorktreeFact{Managed: false}
	}
	rest := norm[idx+len(marker):]
	name, _, _ := strings.Cut(rest, "/")
	if name == "" {
		return WorktreeFact{Managed: false}
	}
	return WorktreeFact{Managed: true, Name: name}
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
