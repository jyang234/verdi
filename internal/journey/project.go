package journey

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/policyauthority"
	"github.com/jyang234/verdi/internal/store"
)

// Project resolves arg to a target spec (GatherFacts) and derives the
// complete journey Record over its facts plus cfg.Model — the operating-
// model catalog that is the ONLY source of candidate transitions and
// actions (DC-3). Project takes no locks and writes no files: every field
// is pure derivation over already-gathered facts (facts.go/derive.go); the
// returned Record's Digest is unset — call Canonical to validate, stamp,
// and canonically encode it.
func (p Projector) Project(ctx context.Context, cfg *store.Config, arg string) (Record, error) {
	facts, err := p.GatherFacts(ctx, cfg, arg)
	if err != nil {
		return Record{}, err
	}
	profile, profileErr := p.profiles.Load(ctx, cfg.Root)
	profileAdopted := profileErr == nil
	if profileErr != nil && !errors.Is(profileErr, policyauthority.ErrNotAdopted) {
		return Record{}, profileErr
	}

	owner := Owner{
		Declared:    strings.Join(facts.Owners, ","),
		Attribution: governanceprincipal.NewUnauthenticatedAttribution(),
	}
	evaluationCommit := ""
	if facts.Repository.Head.Known {
		evaluationCommit = facts.Repository.Head.Value
	}
	specLandingCommit := ""
	if facts.Lifecycle.AcceptedBaseline != nil {
		specLandingCommit = facts.Lifecycle.AcceptedBaseline.LandingCommit
	}
	targetCommit := evaluationCommit
	if facts.Repository.Source == "remote-ref" && facts.Repository.DefaultBranch.Known {
		targetCommit = facts.Repository.DefaultBranch.Head
	}
	qualityFacts, err := p.obligations.Assess(ctx, cfg.Root, facts.Target.Path, facts.Target.Class, targetCommit, evaluationCommit, specLandingCommit)
	if err != nil {
		return Record{}, fmt.Errorf("journey: assessing obligation quality: %w", err)
	}

	candidates, classDeclared := candidateTransitions(cfg.Model, facts.Target.Class, facts.LifecycleResult)
	current := deriveBlockers(facts.Repository.DefaultBranch.Known, profileAdopted, facts.LifecycleResult, candidates, owner)
	current = mergeObligationQualityBlockers(current, deriveObligationQualityBlockers(qualityFacts, owner))
	principals := derivePrincipals(candidates)
	if profileAdopted {
		principals.ProfileAdopted = true
		principals.SelectedProfileID = profile.ID
		principals.SelectedProfileDigest = profile.Digest
		disclosures := make([]string, 0, len(principals.Disclosures))
		for _, disclosure := range principals.Disclosures {
			if disclosure != profileNotAdoptedDisclosure {
				disclosures = append(disclosures, disclosure)
			}
		}
		principals.Disclosures = sortDedupStrings(append(disclosures, profileResolutionUnprovenDisclosure))
	}

	targetRef := "spec/" + specNameFromRelPath(facts.Target.Path)
	actions := deriveActions(facts.Target.Class, facts.LifecycleResult, candidates, classDeclared, targetRef)

	recordLevelDisclosures := facts.RepositoryDisclosures
	if !classDeclared {
		// F4: the SAME message as Actions.NeededFacts's own entry
		// (classUndeclaredMessage) — a structural fact about the operating
		// model belongs at the record level too, not only buried inside
		// Actions.
		recordLevelDisclosures = append(append([]string(nil), recordLevelDisclosures...), classUndeclaredMessage(facts.Target.Class))
	}

	rec := Record{
		Schema:     SchemaID,
		Target:     facts.Target,
		Repository: facts.Repository,
		Lifecycle:  facts.Lifecycle,
		Evidence:   facts.Evidence,
		Blockers: Blockers{
			Current:  current,
			Eventual: deriveEventual(),
		},
		Principals:  principals,
		Actions:     actions,
		Disclosures: recordDisclosures(recordLevelDisclosures),
	}
	if err := rec.Validate(); err != nil {
		return Record{}, fmt.Errorf("journey: project: assembled record failed validation: %w", err)
	}
	return rec, nil
}

// specNameFromRelPath extracts a spec's bare directory name from its
// store-relative path (".verdi/specs/<zone>/<name>/spec.md" ->
// "<name>") — the canonical "spec/<name>" ref Safe actions bind as their
// argument (DC-3: "bind arguments to the current ref"), regardless of
// which of I-30's two forms (a direct spec/<name> ref, or a scheme-
// prefixed story ref that may itself carry a ':' — not a legal Action
// argument character, argumentRe) the caller originally supplied.
func specNameFromRelPath(relPath string) string {
	return path.Base(path.Dir(relPath))
}

// recordDisclosures assembles the record's top-level Disclosures: every
// repository-fact disclosure GatherFacts collected (RepositoryFacts itself
// carries no disclosures field of its own — CO-1's silence-is-never-a-pass
// posture routes those here instead) plus the three disclosures this
// projection version always carries, sorted and deduplicated together.
func recordDisclosures(repositoryDisclosures []string) []string {
	all := append([]string(nil), repositoryDisclosures...)
	all = append(all,
		"forge facts are not consulted by this projection; forge-dependent proofs remain unproven",
		"preparation and preflight closure contributors are not yet journey contributors; their blocker coverage is unknown",
		"source posture receipt-bound is never produced by this projection version",
	)
	return sortDedupStrings(all)
}
