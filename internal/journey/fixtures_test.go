package journey

import (
	"testing"

	"github.com/jyang234/verdi/internal/governanceprincipal"
)

// validRecord returns a fully populated, independently-owned Record that
// satisfies every Validate rule. Each call builds fresh slices/structs so
// subtests may mutate their own copy without disturbing others.
func validRecord(t *testing.T) Record {
	t.Helper()
	return Record{
		Schema: SchemaID,
		Target: Target{
			Ref:   "spec/example",
			Class: "feature",
			Path:  "specs/active/example/spec.md",
		},
		Repository: RepositoryFacts{
			RemoteOrigin: StringFact{Known: true, Value: "https://example.invalid/repo.git"},
			Branch:       StringFact{Known: true, Value: "main"},
			Head:         StringFact{Known: true, Value: "abc123"},
			DefaultBranch: DefaultBranchFact{
				Known: true,
				Name:  "main",
				Ref:   "origin/main",
				Head:  "abc123",
			},
			Relationship: "equal",
			Dirty:        BoolFact{Known: true, Value: false},
			Staged:       BoolFact{Known: true, Value: false},
			Worktree:     WorktreeFact{Managed: false, Name: ""},
			Source:       "head",
		},
		Lifecycle: LifecycleFacts{
			Class:    "feature",
			State:    "accepted-pending-build",
			Relation: "exact",
			AcceptedBaseline: &Baseline{
				Path:          "specs/active/example/spec.md",
				Blob:          "deadbeef",
				LandingCommit: "cafebabe",
			},
			Frozen:      nil,
			Disclosures: []string{},
		},
		Blockers: Blockers{
			Current: []Blocker{
				{
					ID:                "forge-unreachable",
					Reason:            ReasonForgeFactsUnavailable,
					Class:             ClassExternalWait,
					Witnesses:         []string{"forge/pr/42"},
					Owner:             Owner{Declared: "Jane Doe", Attribution: governanceprincipal.NewUnauthenticatedAttribution()},
					ClearingCondition: "forge becomes reachable",
					Transition:        "unknown",
				},
			},
			Eventual: EventualBlockers{
				Derived:     false,
				Items:       []Blocker{},
				Disclosures: []string{"eventual blockers not yet derived"},
			},
		},
		Principals: PrincipalFacts{
			ProfileAdopted: false,
			Required: []RequiredRole{
				{Transition: "close", Obligation: "attestation/countersign", Count: 1, Resolution: "unproven"},
			},
			Disclosures: []string{},
		},
		Actions: Actions{
			Safe: []Action{
				{
					ID:            "close-issue",
					Verb:          "close",
					Arguments:     []string{"issue/123"},
					FromState:     "accepted-pending-build",
					ToState:       "closed",
					Confirmation:  "none",
					Preconditions: []Precondition{{ID: "fold-green", Witness: "ci-run/456"}},
				},
			},
			NeededFacts: []string{"default-branch"},
		},
		Disclosures: []string{"principal profile not adopted"},
		Digest:      "",
	}
}
