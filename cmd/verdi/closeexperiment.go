// Spike closure on ratified CSE experiment evidence (Wave 5C Task 10;
// design §9's final paragraph, §10; CSE AC-5/AC-7, DC-1/DC-7/DC-9/DC-16,
// CO-1; SI-146 option c). The existing spike-close service (runClose,
// close.go) receives an ADDITIVE evidence provider: a comparison-backed
// close target — one that carries at least one comparative-spike-
// experiment under `.verdi/specs/active/<name>/experiments/` (worktree or
// accepted tree, either zone) — must additionally clear this gate before
// the ritual is allowed to mutate anything. A target that carries no
// experiment evidence at all is untouched: the gate reports zero
// experiments and runClose proceeds exactly as it always has.
//
// Design §9's singular rule is: a comparison-backed close requires an
// accepted ratification; SELECTING that ratification (select-recommended
// or select-other) additionally requires a valid capsule whose
// definition, result, candidate, and ratification identities all match
// the accepted proof; the ratified answer then flows through the spike's
// EXISTING `resolves` edge (no new edge is ever written, no parent-
// feature spec is ever touched — SI-146 option c). A non-selecting
// disposition (reject-all, misframed, request-new-revision) is an honest
// terminal response and does NOT satisfy closure by itself.
//
// This file composes that rule per experiment (the brief's disclosed
// composition, adjudicated by the controller, not re-decided here): a
// comparison-backed target may close only when EVERY discovered
// experiment's evidence is clean (a genuine accepted ratification, and —
// when it selects — a matching capsule) AND at least one experiment
// supplies a valid selecting ratification. CO-1 fail-closed: any
// experiment whose evidence is a verdict, or a broken/mismatched
// selecting proof, blocks closure outright even if another experiment in
// the same target already satisfies it; a non-selecting experiment merely
// fails to CONTRIBUTE the satisfying selection, it does not itself poison
// an otherwise-satisfied target.
//
// design §10 / CO-4's exit floor governs every path here exactly as it
// governs the rest of close.go: 0 clean, 1 a well-formed but unsatisfied
// authority/lifecycle/evidence verdict, 2 an uninterpretable or unsafe
// operational condition. No missing fact is ever interpreted favorably
// (a provider error, or any per-experiment operational Outcome, is 2 —
// never silently read as "no experiments" or as a verdict); no
// operational result is ever softened into a verdict.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/experiment"
	"github.com/jyang234/verdi/internal/experimentapp"
	"github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/policyauthority"
)

// closeExperimentEvidence is the consumer-owned typed evidence this gate
// judges, one per experiment discovered under the closure target — never
// a decoded CSE artifact passed through verbatim (the port returns
// close's own shape, not experimentapp's). Outcome alone is meaningful
// for a non-clean experiment; the remaining fields are populated only
// when Outcome.Classification is ClassificationClean (the accepted
// ratification proof held).
//
// DefinitionID/DefinitionDigest/SelectedCandidate/RatificationDigest are
// gathered independently of Ratification/Capsule so the core judgment
// below can compare identities without re-deriving anything itself
// (CLAUDE.md: no algorithm re-implementation outside its one owning
// package) — every derivation (SelectedCapsuleCandidate, EncodeRatification,
// DefinitionDigest, ResultDigest) already lives in internal/experiment and
// runs exactly once, in the production adapter, never in the pure core.
type closeExperimentEvidence struct {
	ExperimentID string
	Outcome      experimentapp.Outcome
	Ratification experiment.Ratification
	// Capsule is nil for an absent or non-selecting capsule; never a zero
	// CapsuleManifest standing in for absence (a zero value would decode-
	// validate as false but silently carry no signal of WHY).
	Capsule *experiment.CapsuleManifest
	// DefinitionID and DefinitionDigest are the accepted experiment.yaml's
	// own id and content digest — never the closure target's spec id.
	DefinitionID     string
	DefinitionDigest string
	// SelectedCandidate is experiment.SelectedCapsuleCandidate's derivation
	// for a selecting disposition, "" for a non-selecting one.
	SelectedCandidate string
	// RatificationDigest is "sha256:<hex>" of the accepted ratification's
	// own canonical bytes (EncodeRatification(Ratification)) — the exact
	// identity the capsule manifest's own "ratification" artifact entry
	// must reproduce for a valid selecting capsule.
	RatificationDigest string
	// RequiredCapsuleArtifactIDs is the complete CLOSED required inventory
	// design §9's retained set obliges a selecting capsule to carry (DC-8),
	// derived once by the production adapter from the ACCEPTED definition
	// and sorted. It is empty for a non-selecting disposition, which binds
	// no capsule at all (SI-146). The pure core compares this set against
	// the manifest's own artifact ids; it never derives the set itself.
	RequiredCapsuleArtifactIDs []string
}

// closeExperimentEvidenceProvider is the port runClose calls between the
// closure gate's ok check and foldStory (close.go). nil/empty evidence
// means the target is not comparison-backed: ordinary closure, zero
// behavior change. A returned error is always operational (2) — a
// provider that cannot answer this question honestly must never be read
// as "no experiments".
type closeExperimentEvidenceProvider interface {
	CloseEvidence(ctx context.Context, root string, spec *artifact.SpecFrontmatter) ([]closeExperimentEvidence, error)
}

// closeExperimentSelecting reports whether d is one of the two selecting
// dispositions (AC-5's closed vocabulary; release.go:140 precedent for the
// identical two-way split at the identical decision point).
func closeExperimentSelecting(d experiment.Disposition) bool {
	return d == experiment.DispositionSelectRecommended || d == experiment.DispositionSelectOther
}

// closeExperimentCapsuleReason validates a selecting experiment's capsule
// against its own typed evidence — design §9's four identity matches plus
// the capsule's own "ratification" artifact digest — and returns "" when
// every check holds. It never re-derives an identity itself: every value
// compared here was already computed once, by the production adapter, from
// the same accepted proof AcceptedRatification sealed.
func closeExperimentCapsuleReason(e closeExperimentEvidence) string {
	if e.Capsule == nil {
		return fmt.Sprintf("disposition %q selects a candidate but the accepted tree carries no capsule manifest for it", e.Ratification.Disposition)
	}
	if err := e.Capsule.Validate(); err != nil {
		return fmt.Sprintf("the accepted capsule manifest is invalid: %v", err)
	}
	switch {
	case e.Capsule.Experiment != e.DefinitionID:
		return fmt.Sprintf("capsule experiment %q does not match the accepted definition id %q", e.Capsule.Experiment, e.DefinitionID)
	case e.Capsule.DefinitionDigest != e.DefinitionDigest:
		return fmt.Sprintf("capsule definition digest %q does not match the accepted definition digest %q", e.Capsule.DefinitionDigest, e.DefinitionDigest)
	case e.Capsule.ResultDigest != e.Ratification.ResultDigest:
		return fmt.Sprintf("capsule result digest %q does not match the ratified result digest %q", e.Capsule.ResultDigest, e.Ratification.ResultDigest)
	case e.Capsule.Selected != e.SelectedCandidate:
		return fmt.Sprintf("capsule selected candidate %q does not match the ratified selection %q", e.Capsule.Selected, e.SelectedCandidate)
	}
	if reason := closeExperimentInventoryReason(e); reason != "" {
		return reason
	}
	for _, a := range e.Capsule.Artifacts {
		if a.ID == experiment.CapsuleArtifactRatification {
			if a.Digest != e.RatificationDigest {
				return fmt.Sprintf("capsule ratification-artifact digest %q does not match the accepted ratification digest %q", a.Digest, e.RatificationDigest)
			}
			return ""
		}
	}
	return fmt.Sprintf("capsule manifest carries no %q artifact entry", experiment.CapsuleArtifactRatification)
}

// closeExperimentInventoryReason judges a selecting capsule's own artifact
// inventory against design §9's CLOSED retained set (DC-8): a selecting
// capsule that matches every identity but retains only part of the
// reproduction set is not the sealed complete capsule §9 requires, and one
// that retains something outside the set is not a capsule the ratified
// protocol admits at all. It returns "" when the inventory is exact.
//
// This is a SET COMPARISON over internal/experiment's own exported closed
// vocabulary and the required id list the production adapter already
// derived from the ACCEPTED definition (BindCapsuleManifest's identical
// required/optional split is the one owner of that mapping — CLAUDE.md: no
// digest and no binding algorithm is re-implemented here). "recommendation"
// is the set's one OPTIONAL member and is therefore admitted but never
// demanded.
//
// A selecting evidence carrying an EMPTY required set is itself refused by
// the unexpected-id arm below (any real capsule entry then lies outside
// "required ∪ {recommendation}") — CO-1 fail-closed: an inventory the
// adapter could not state is never read as "no constraint".
func closeExperimentInventoryReason(e closeExperimentEvidence) string {
	present := make(map[string]bool, len(e.Capsule.Artifacts))
	for _, a := range e.Capsule.Artifacts {
		present[a.ID] = true
	}
	required := make(map[string]bool, len(e.RequiredCapsuleArtifactIDs))
	missing := make([]string, 0, len(e.RequiredCapsuleArtifactIDs))
	for _, id := range e.RequiredCapsuleArtifactIDs {
		required[id] = true
		if !present[id] {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		// RequiredCapsuleArtifactIDs arrives sorted, so missing is too.
		// vocab:identity — "closed retained set" names the fixed capsule protocol inventory (capsule_binding.go's own SI-149 artifact-ID set), not a model-state display.
		return fmt.Sprintf("capsule manifest is missing required artifact %s of design §9's closed retained set", closeExperimentQuoteIDs(missing))
	}
	unexpected := make([]string, 0, len(e.Capsule.Artifacts))
	for _, a := range e.Capsule.Artifacts {
		if !required[a.ID] && a.ID != experiment.CapsuleArtifactRecommendation {
			unexpected = append(unexpected, a.ID)
		}
	}
	if len(unexpected) > 0 {
		sort.Strings(unexpected)
		// vocab:identity — "closed retained set" names the fixed capsule protocol inventory (capsule_binding.go's own SI-149 artifact-ID set), not a model-state display.
		return fmt.Sprintf("capsule manifest presents artifact %s, which is outside design §9's closed retained set", closeExperimentQuoteIDs(unexpected))
	}
	return ""
}

// closeExperimentQuoteIDs renders a nonempty id list as quoted, comma-
// separated prose so a refusal always NAMES the exact offending ids.
func closeExperimentQuoteIDs(ids []string) string {
	quoted := make([]string, len(ids))
	for i, id := range ids {
		quoted[i] = fmt.Sprintf("%q", id)
	}
	return strings.Join(quoted, ", ")
}

// closeExperimentRequiredCapsuleArtifactIDs mirrors, EXACTLY, the required
// half of BindCapsuleManifest's closed inventory (capsule_binding.go's
// `required` map; design §9's retained set, DC-8) for one accepted
// definition: the nine fixed members plus one namespaced entry per fixture
// the definition registers. "recommendation" is the set's one optional
// member and is deliberately absent here. Sorted, so the evidence's own
// field and every refusal that names it are deterministic.
func closeExperimentRequiredCapsuleArtifactIDs(def experiment.Definition) ([]string, error) {
	ids := []string{
		experiment.CapsuleArtifactDefinition,
		experiment.CapsuleArtifactCandidatePatch,
		experiment.CapsuleArtifactEvaluatorCapabilities,
		experiment.CapsuleArtifactContract,
		experiment.CapsuleArtifactWorkload,
		experiment.CapsuleArtifactExecutionReceipt,
		experiment.CapsuleArtifactObservations,
		experiment.CapsuleArtifactResult,
		experiment.CapsuleArtifactRatification,
	}
	for _, fixture := range def.Fixtures {
		id, err := experiment.CapsuleFixtureArtifactID(fixture.ID)
		if err != nil {
			return nil, fmt.Errorf("experiment evidence: the accepted definition's retained fixture inventory: %w", err)
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}

// closeExperimentCondition is one experiment's rendered per-condition
// judgment (the reportClosureGateConditions [PASS]/[FAIL] idiom, applied
// to CSE evidence rather than AC evidence).
type closeExperimentCondition struct {
	id     string
	ok     bool
	reason string
}

// closeExperimentEvaluate is the pure core judgment (§9's singular rule
// composed per experiment, disclosed to Codex adjudication in the task
// report): it reads evidence only — it never assigns into any element or
// any pointed-to Capsule/Ratification, so the caller's injected slice is
// provably byte-unchanged afterward (the matrix's deep-copy/no-alias
// item). evidence must not contain an operational Outcome; the caller
// filters that case before this runs (a provider/port failure is 2, never
// folded into this judgment's 0/1 verdict).
//
// proceed is true exactly when every experiment's evidence is clean (a
// genuine accepted ratification, and — when it selects — a capsule that
// matches every identity) AND at least one experiment supplies a valid
// selecting ratification. A verdict Outcome, or a selecting-but-mismatched
// capsule, is a HARD failure that blocks closure outright (CO-1
// fail-closed) even alongside another experiment that already satisfies
// it; a non-selecting disposition merely fails to contribute the
// satisfying selection and does not itself block an otherwise-satisfied
// target.
func closeExperimentEvaluate(evidence []closeExperimentEvidence) (proceed bool, conditions []closeExperimentCondition) {
	hardFailure := false
	anySelecting := false
	conditions = make([]closeExperimentCondition, 0, len(evidence))
	for _, e := range evidence {
		switch e.Outcome.Classification {
		case experimentapp.ClassificationVerdict:
			hardFailure = true
			conditions = append(conditions, closeExperimentCondition{
				id: e.ExperimentID, reason: fmt.Sprintf("%s [%s]", e.Outcome.Detail, e.Outcome.Code),
			})
			continue
		case experimentapp.ClassificationClean:
			// fall through to the selecting/capsule judgment below.
		default:
			// An operational Outcome must never reach this pure judgment
			// (the caller filters and exits 2 first); a defensive fail-
			// closed line rather than a silent skip if it ever does.
			hardFailure = true
			conditions = append(conditions, closeExperimentCondition{
				id: e.ExperimentID, reason: fmt.Sprintf("unexpected outcome classification %q", e.Outcome.Classification),
			})
			continue
		}
		if !closeExperimentSelecting(e.Ratification.Disposition) {
			conditions = append(conditions, closeExperimentCondition{
				id: e.ExperimentID, reason: fmt.Sprintf("disposition %q is an honest terminal response and does not select a candidate; it does not satisfy closure by itself (design §9)", e.Ratification.Disposition),
			})
			continue
		}
		if reason := closeExperimentCapsuleReason(e); reason != "" {
			hardFailure = true
			conditions = append(conditions, closeExperimentCondition{id: e.ExperimentID, reason: reason})
			continue
		}
		anySelecting = true
		conditions = append(conditions, closeExperimentCondition{
			id: e.ExperimentID, ok: true,
			reason: fmt.Sprintf("valid ratified selection of candidate %q, capsule identity confirmed", e.SelectedCandidate),
		})
	}
	return !hardFailure && anySelecting, conditions
}

// closeExperimentGate is the seam runClose calls between the closure
// gate's ok check and foldStory (close.go, per the controller's fixed
// ordering: AC-5 — the ratification record joins the normal closure
// review, still strictly pre-effect). It returns 0 to proceed (the caller
// continues the ritual unchanged), or the exact exit code to return
// immediately otherwise.
func closeExperimentGate(evidence []closeExperimentEvidence, stdout, stderr io.Writer) int {
	if len(evidence) == 0 {
		// Not comparison-backed: zero behavior change.
		return 0
	}
	// design §10 / CO-4, fail-closed on an unknown enum value: the pure
	// judgment below is defined ONLY over the two interpretable
	// classifications, so anything that is neither clean nor verdict —
	// operational, or a classification this build does not recognize at
	// all — is an uninterpretable condition and exits 2 here. Reading an
	// unrecognized classification as a verdict would be exactly the
	// operational→verdict collapse §10 forbids.
	for _, e := range evidence {
		switch e.Outcome.Classification {
		case experimentapp.ClassificationClean, experimentapp.ClassificationVerdict:
			continue
		default:
			fmt.Fprintf(stderr, "close: experiment %s: %s [%s]\n", e.ExperimentID, e.Outcome.Detail, e.Outcome.Code)
			return 2
		}
	}
	proceed, conditions := closeExperimentEvaluate(evidence)
	if proceed {
		return 0
	}
	sort.Slice(conditions, func(i, j int) bool { return conditions[i].id < conditions[j].id })
	for _, c := range conditions {
		if c.ok {
			fmt.Fprintf(stdout, "[PASS] experiment %s: %s\n", c.id, c.reason)
		} else {
			fmt.Fprintf(stdout, "[FAIL] experiment %s: %s\n", c.id, c.reason)
		}
	}
	fmt.Fprintln(stdout, "close: FAIL (experiment evidence not satisfied; see conditions above)")
	return 1
}

// productionCloseExperimentEvidence is the real closeExperimentEvidenceProvider
// (closeDeps.Experiments == nil wires this, mirroring the State field's
// nil-is-production precedent). It discovers every experiment id under the
// closure target (worktree ∪ accepted tree, either zone), re-authenticates
// each one's accepted ratification through experimentapp.Service exactly as
// `verdi experiment` does, and gathers the identity facts a selecting
// disposition's capsule must match — ALL from the one already-listed
// accepted-tree enumeration and ONE profile load, per the controller's
// stop-gate audit (no second Git enumeration, no second policy load).
type productionCloseExperimentEvidence struct{}

// closeExperimentTrustFactReader is the close-local
// governanceprincipal.TrustFactReader (the CONSUMER-OWNED port,
// resolve.go:137 — the kernel never reads ambient identity itself): it
// reports whether the accepted profile's own role_mappings attest
// claim.Subject for claim.TrustSource, mirroring experimenthuman's fact
// construction (experimenthuman/verify.go's candidateSubjects/
// staticFactReader) with one difference — that reader answers for a single
// pre-verified subject, this one answers the general claim the kernel
// re-resolves the PERSISTED ratification actor against, so it must consult
// the mapping for whatever claim it is asked about, not a fixed one.
// EvidenceDigest is the accepted profile's own content digest: the
// evidence "observed" for a persisted claim IS the accepted governance
// profile that authorized it, not a separately verified artifact.
type closeExperimentTrustFactReader struct {
	profile governanceprincipal.Profile
}

func (r closeExperimentTrustFactReader) ReadTrustFact(_ context.Context, source governanceprincipal.TrustSource, claim governanceprincipal.PrincipalClaim) (governanceprincipal.TrustFact, error) {
	digest, err := r.profile.Digest()
	if err != nil {
		return governanceprincipal.TrustFact{}, fmt.Errorf("experiment evidence: digesting the accepted governance profile: %w", err)
	}
	for _, mapping := range r.profile.RoleMappings {
		if mapping.TrustSource != claim.TrustSource {
			continue
		}
		for _, subject := range mapping.Subjects {
			if subject == claim.Subject {
				return governanceprincipal.TrustFact{
					SourceID: source.ID, SourceKind: source.Kind,
					Subjects:       []string{claim.Subject},
					EvidenceDigest: digest, Available: true, Valid: true,
				}, nil
			}
		}
	}
	return governanceprincipal.TrustFact{
		SourceID: source.ID, SourceKind: source.Kind,
		Available: false,
		Reason:    fmt.Sprintf("no role_mapping of the accepted governance profile attests subject %q for trust source %q", claim.Subject, claim.TrustSource),
	}, nil
}

// CloseEvidence implements closeExperimentEvidenceProvider.
func (productionCloseExperimentEvidence) CloseEvidence(ctx context.Context, root string, spec *artifact.SpecFrontmatter) ([]closeExperimentEvidence, error) {
	specRef, err := artifact.ParseRef(spec.ID)
	if err != nil {
		return nil, fmt.Errorf("experiment evidence: resolved spec has an invalid id: %w", err)
	}
	name := specRef.Name

	accGit := experimentAcceptedGit{}
	branch, err := accGit.ResolveDefaultBranch(ctx, root)
	if err != nil {
		return nil, fmt.Errorf("experiment evidence: resolving the accepted default branch: %w", err)
	}
	head := branch.Head

	treeEntries, err := accGit.ListTree(ctx, root, head)
	if err != nil {
		return nil, fmt.Errorf("experiment evidence: listing the accepted tree at %s: %w", head, err)
	}

	ids, err := closeExperimentIDUnion(root, name, treeEntries)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}

	service, err := newExperimentService(root)
	if err != nil {
		return nil, fmt.Errorf("experiment evidence: constructing the experiment service: %w", err)
	}
	source, err := experimentAcceptedTreeFS(ctx, root, head)
	if err != nil {
		return nil, fmt.Errorf("experiment evidence: loading the accepted authority tree: %w", err)
	}
	policyStore, err := policyauthority.LoadFromSource(source)
	if err != nil {
		return nil, fmt.Errorf("experiment evidence: loading the accepted policy authority: %w", err)
	}
	profile, err := policyStore.SelectedProfile()
	if err != nil {
		return nil, fmt.Errorf("experiment evidence: selecting the accepted governance profile: %w", err)
	}
	facts := closeExperimentTrustFactReader{profile: profile}

	// vocab:identity — the delegated-agent harness identifier this verb
	// registers itself under (experimentapp.NewDelegatedAgent's own
	// grammar), not display prose.
	actor, err := experimentapp.NewDelegatedAgent("verdi-close", "")
	if err != nil {
		return nil, fmt.Errorf("experiment evidence: constructing the delegated-agent actor: %w", err)
	}

	entriesByPath := make(map[string]experimentapp.GitTreeEntry, len(treeEntries))
	for _, entry := range treeEntries {
		entriesByPath[entry.Path] = entry
	}

	out := make([]closeExperimentEvidence, 0, len(ids))
	for _, id := range ids {
		identity := experimentapp.Identity{
			CheckoutRoot: root, Spike: "spec/" + name, ExperimentID: id,
			ExpectedAcceptedHEAD: head, Actor: actor,
		}
		result := service.AcceptedRatification(ctx, identity, experimentapp.AcceptedRatificationAuthority{Profile: profile, Facts: facts})
		ev := closeExperimentEvidence{ExperimentID: id, Outcome: result.Outcome}
		if result.Outcome.Classification == experimentapp.ClassificationClean {
			gathered, gatherErr := closeExperimentGatherFacts(ctx, root, accGit, head, name, entriesByPath, result)
			if gatherErr != nil {
				// Any read/decode failure at this stage is operational
				// evidence for THIS experiment, never a skip (the
				// controller's stop-gate audit): the accepted ratification
				// proof held, but the identity facts a selecting capsule
				// must match could not be honestly gathered.
				ev.Outcome = experimentapp.Outcome{
					Classification: experimentapp.ClassificationOperational,
					Code:           "experiment-evidence-unreadable",
					Detail:         gatherErr.Error(),
				}
			} else {
				ev.Ratification = result.Ratification
				ev.DefinitionID = gathered.definitionID
				ev.DefinitionDigest = gathered.definitionDigest
				ev.SelectedCandidate = gathered.selectedCandidate
				ev.RatificationDigest = gathered.ratificationDigest
				ev.RequiredCapsuleArtifactIDs = gathered.requiredCapsuleArtifactIDs
				ev.Capsule = gathered.capsule
			}
		}
		out = append(out, ev)
	}
	return out, nil
}

// closeExperimentIDUnion is the union of worktree-side and accepted-tree-
// side experiment ids under the closure target's experiments/ directory
// (the controller's stop-gate audit item 1): worktree ids via os.ReadDir
// (absent dir = none; a non-directory entry is included as an id too —
// a malformed layout must fail closed through Identity validation
// downstream, never be silently skipped), accepted-tree ids via the
// already-listed tree entries under EITHER zone's
// specs/<zone>/<name>/experiments/<id>/ prefix. Sorted, deduplicated.
func closeExperimentIDUnion(root, name string, treeEntries []experimentapp.GitTreeEntry) ([]string, error) {
	ids := map[string]bool{}

	worktreeDir := filepath.Join(root, ".verdi", "specs", "active", name, "experiments")
	dirEntries, err := os.ReadDir(worktreeDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("experiment evidence: reading %s: %w", worktreeDir, err)
	}
	for _, e := range dirEntries {
		ids[e.Name()] = true
	}

	for _, zone := range []string{"active", "archive"} {
		prefix := ".verdi/specs/" + zone + "/" + name + "/experiments/"
		for _, entry := range treeEntries {
			rest, ok := strings.CutPrefix(entry.Path, prefix)
			if !ok {
				continue
			}
			if id, _, _ := strings.Cut(rest, "/"); id != "" {
				ids[id] = true
			}
		}
	}

	out := make([]string, 0, len(ids))
	for id := range ids {
		out = append(out, id)
	}
	sort.Strings(out)
	return out, nil
}

// closeExperimentGathered is the identity facts closeExperimentGatherFacts
// derives from the accepted tree for one clean accepted ratification.
type closeExperimentGathered struct {
	definitionID       string
	definitionDigest   string
	selectedCandidate  string
	ratificationDigest string
	// requiredCapsuleArtifactIDs is design §9's closed required inventory
	// for THIS accepted definition, derived only for a selecting
	// disposition (a non-selecting one binds no capsule at all).
	requiredCapsuleArtifactIDs []string
	capsule                    *experiment.CapsuleManifest
}

// closeExperimentGatherFacts reads every identity fact a selecting
// disposition's capsule must match, entirely from the SAME pinned
// accepted head and the SAME already-listed tree entries the caller's one
// ListTree call produced — every blob read goes through the entry's own
// recorded OID (experimentAcceptedGit.ReadBlob's pinned-head-plus-OID
// contract: no mixed snapshot is possible even if the branch moves
// between calls). It never re-implements DeriveState or any other
// experimentapp algorithm: the caller has already proven result.Ratification
// via AcceptedRatification, and this function only locates the ONE
// bound-result blob AcceptedRatification already proved unique and derives
// SelectedCapsuleCandidate from it (internal/experiment's own function,
// invoked, never re-implemented).
//
// name is the closure target's own spec name (the caller's already-parsed
// artifact ref), needed for the accepted definition's self-consistency
// check below.
func closeExperimentGatherFacts(ctx context.Context, root string, git experimentAcceptedGit, head, name string, entriesByPath map[string]experimentapp.GitTreeEntry, result experimentapp.AcceptedRatificationResult) (closeExperimentGathered, error) {
	defEntry, ok := entriesByPath[path.Join(result.ExperimentPath, "experiment.yaml")]
	if !ok {
		return closeExperimentGathered{}, fmt.Errorf("experiment evidence: %s: experiment.yaml is absent from the already-listed accepted tree", result.ExperimentPath)
	}
	defBytes, err := git.ReadBlob(ctx, root, head, defEntry.Object, defEntry.Path)
	if err != nil {
		return closeExperimentGathered{}, fmt.Errorf("experiment evidence: reading %s: %w", defEntry.Path, err)
	}
	def, err := experiment.DecodeDefinition(defBytes)
	if err != nil {
		return closeExperimentGathered{}, fmt.Errorf("experiment evidence: decoding %s: %w", defEntry.Path, err)
	}
	defDigest, err := experiment.DefinitionDigest(def)
	if err != nil {
		return closeExperimentGathered{}, fmt.Errorf("experiment evidence: digesting %s: %w", defEntry.Path, err)
	}

	// The accepted definition must be the one this directory and this
	// closure target actually name: `experiment.yaml` carries its own `id`
	// and `spike`, and the accepted tree carries it at
	// .../<name>/experiments/<id>/. A definition whose own id or spike
	// disagrees with the location it was accepted into is INTERNALLY
	// INCONSISTENT accepted evidence — the identity facts below would be
	// gathered from an artifact that does not belong to the experiment
	// being judged. That is uninterpretable, not unsatisfied, so it is
	// operational (2) exactly as acceptedRatificationAt classifies its own
	// definition-invalid/state-invalid arms (design §10 / CO-4: never
	// softened into a verdict).
	if dirID := path.Base(result.ExperimentPath); def.ID != dirID {
		return closeExperimentGathered{}, fmt.Errorf("experiment evidence: %s: accepted definition id %q does not name its own experiment directory %q", result.ExperimentPath, def.ID, dirID)
	}
	if wantSpike := "spec/" + name; def.Spike != wantSpike {
		// vocab:identity — spike: the experiment definition's own FIELD name (verdi.experiment/v2 ref grammar, definition.go's spikeRe), not a model-state display.
		return closeExperimentGathered{}, fmt.Errorf("experiment evidence: %s: accepted definition spike %q does not name the closure target %q", result.ExperimentPath, def.Spike, wantSpike)
	}

	ratBytes, err := experiment.EncodeRatification(result.Ratification)
	if err != nil {
		return closeExperimentGathered{}, fmt.Errorf("experiment evidence: re-encoding the accepted ratification: %w", err)
	}
	gathered := closeExperimentGathered{
		definitionID:       def.ID,
		definitionDigest:   defDigest,
		ratificationDigest: closeExperimentRawDigest(ratBytes),
	}

	if !closeExperimentSelecting(result.Ratification.Disposition) {
		return gathered, nil
	}

	requiredIDs, err := closeExperimentRequiredCapsuleArtifactIDs(def)
	if err != nil {
		return closeExperimentGathered{}, err
	}
	gathered.requiredCapsuleArtifactIDs = requiredIDs

	// Map iteration order is randomized, so the candidate run results are
	// collected and SORTED before they are read: the duplicate-result
	// diagnostic below names two exact paths, and that prose must be the
	// same on every run over the same accepted tree (CLAUDE.md:
	// deterministic outputs).
	runsPrefix := path.Join(result.ExperimentPath, "runs") + "/"
	resultPaths := make([]string, 0, len(entriesByPath))
	for p := range entriesByPath {
		rest, ok := strings.CutPrefix(p, runsPrefix)
		if !ok || !strings.HasSuffix(rest, "/result.json") {
			continue
		}
		resultPaths = append(resultPaths, p)
	}
	sort.Strings(resultPaths)

	var matchedResult *experiment.Result
	matchedPath := ""
	for _, p := range resultPaths {
		entry := entriesByPath[p]
		resultBytes, err := git.ReadBlob(ctx, root, head, entry.Object, entry.Path)
		if err != nil {
			return closeExperimentGathered{}, fmt.Errorf("experiment evidence: reading %s: %w", entry.Path, err)
		}
		res, err := experiment.DecodeResult(resultBytes)
		if err != nil {
			return closeExperimentGathered{}, fmt.Errorf("experiment evidence: decoding %s: %w", entry.Path, err)
		}
		digest, err := experiment.ResultDigest(res)
		if err != nil {
			return closeExperimentGathered{}, fmt.Errorf("experiment evidence: digesting %s: %w", entry.Path, err)
		}
		if digest != result.Ratification.ResultDigest {
			continue
		}
		if matchedResult != nil {
			return closeExperimentGathered{}, fmt.Errorf("experiment evidence: %s: more than one run's result matches the ratified result digest (%s and %s)", result.ExperimentPath, matchedPath, entry.Path)
		}
		bound := res
		matchedResult = &bound
		matchedPath = entry.Path
	}
	if matchedResult == nil {
		return closeExperimentGathered{}, fmt.Errorf("experiment evidence: %s: no run result matches the ratified result digest %s", result.ExperimentPath, result.Ratification.ResultDigest)
	}

	selected, err := experiment.SelectedCapsuleCandidate(def, *matchedResult, result.Ratification)
	if err != nil {
		return closeExperimentGathered{}, fmt.Errorf("experiment evidence: deriving the selected candidate: %w", err)
	}
	gathered.selectedCandidate = selected

	if capsuleEntry, ok := entriesByPath[path.Join(result.ExperimentPath, "selected", "capsule-manifest.json")]; ok {
		capsuleBytes, err := git.ReadBlob(ctx, root, head, capsuleEntry.Object, capsuleEntry.Path)
		if err != nil {
			return closeExperimentGathered{}, fmt.Errorf("experiment evidence: reading %s: %w", capsuleEntry.Path, err)
		}
		capsule, err := experiment.DecodeCapsuleManifest(capsuleBytes)
		if err != nil {
			return closeExperimentGathered{}, fmt.Errorf("experiment evidence: decoding %s: %w", capsuleEntry.Path, err)
		}
		gathered.capsule = &capsule
	}
	return gathered, nil
}

// closeExperimentRawDigest is the shared sha256 raw-bytes digest form
// (mirroring experimentRawDigest, the test-only twin already used by
// experiment_test.go, and experimentapp's own private rawDigest) — the
// EncodeRatification wire bytes are YAML, never canonjson, so this is
// deliberately NOT canonjson.Digest.
func closeExperimentRawDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}
