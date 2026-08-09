package journey

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/governanceprincipal"
)

func TestRecordValidateHappyPath(t *testing.T) {
	r := validRecord(t)
	if err := r.Validate(); err != nil {
		t.Fatalf("Validate() on well-formed fixture: unexpected error: %v", err)
	}
}

func TestRecordValidateHappyPathWithAuthenticatedOwner(t *testing.T) {
	r := validRecord(t)
	id, err := governanceprincipal.CanonicalPrincipalID("github", "user-123")
	if err != nil {
		t.Fatalf("CanonicalPrincipalID: %v", err)
	}
	attr, err := governanceprincipal.NewPrincipalAttribution(id)
	if err != nil {
		t.Fatalf("NewPrincipalAttribution: %v", err)
	}
	r.Blockers.Current[0].Owner = Owner{Declared: "Jane Doe", Attribution: attr}
	if err := r.Validate(); err != nil {
		t.Fatalf("Validate() with authenticated owner: unexpected error: %v", err)
	}
}

func TestRecordValidateNegative(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Record)
		wantSub string
	}{
		{
			name:    "unknown schema id",
			mutate:  func(r *Record) { r.Schema = "verdi.journey/v2" },
			wantSub: "schema",
		},
		{
			name:    "unknown target class",
			mutate:  func(r *Record) { r.Target.Class = "epic" },
			wantSub: "class",
		},
		{
			name:    "empty target ref",
			mutate:  func(r *Record) { r.Target.Ref = "" },
			wantSub: "ref",
		},
		{
			name:    "empty target path",
			mutate:  func(r *Record) { r.Target.Path = "" },
			wantSub: "path",
		},
		{
			name:    "lifecycle class does not match target class",
			mutate:  func(r *Record) { r.Lifecycle.Class = "story" },
			wantSub: "class",
		},
		{
			name:    "unknown lifecycle state",
			mutate:  func(r *Record) { r.Lifecycle.State = "in-review" },
			wantSub: "state",
		},
		{
			name:    "unknown lifecycle relation",
			mutate:  func(r *Record) { r.Lifecycle.Relation = "similar" },
			wantSub: "relation",
		},
		{
			name: "accepted baseline partially empty",
			mutate: func(r *Record) {
				r.Lifecycle.AcceptedBaseline = &Baseline{Path: "", Blob: "x", LandingCommit: "y"}
			},
			wantSub: "baseline",
		},
		{
			name: "frozen revision partially empty",
			mutate: func(r *Record) {
				r.Lifecycle.Frozen = &FrozenRevision{At: "", Commit: "y"}
			},
			wantSub: "frozen",
		},
		{
			name:    "nil lifecycle disclosures",
			mutate:  func(r *Record) { r.Lifecycle.Disclosures = nil },
			wantSub: "disclosures",
		},
		{
			name:    "unknown repository relationship",
			mutate:  func(r *Record) { r.Repository.Relationship = "parallel" },
			wantSub: "relationship",
		},
		{
			name:    "unknown repository source",
			mutate:  func(r *Record) { r.Repository.Source = "cache" },
			wantSub: "source",
		},
		{
			name:    "string fact known false with nonempty value",
			mutate:  func(r *Record) { r.Repository.Branch = StringFact{Known: false, Value: "x"} },
			wantSub: "known",
		},
		{
			name:    "bool fact known false with true value",
			mutate:  func(r *Record) { r.Repository.Dirty = BoolFact{Known: false, Value: true} },
			wantSub: "known",
		},
		{
			name: "default branch fact known false with nonempty fields",
			mutate: func(r *Record) {
				r.Repository.DefaultBranch = DefaultBranchFact{Known: false, Name: "main"}
			},
			wantSub: "known",
		},
		{
			name:    "worktree fact name set without managed",
			mutate:  func(r *Record) { r.Repository.Worktree = WorktreeFact{Managed: false, Name: "wt1"} },
			wantSub: "managed",
		},
		{
			name:    "unsorted blocker witnesses",
			mutate:  func(r *Record) { r.Blockers.Current[0].Witnesses = []string{"b", "a"} },
			wantSub: "sorted",
		},
		{
			name:    "duplicated blocker witnesses",
			mutate:  func(r *Record) { r.Blockers.Current[0].Witnesses = []string{"a", "a"} },
			wantSub: "sorted",
		},
		{
			name:    "nil blocker witnesses",
			mutate:  func(r *Record) { r.Blockers.Current[0].Witnesses = nil },
			wantSub: "witnesses",
		},
		{
			name:    "blocker class contradicts reason's fixed class",
			mutate:  func(r *Record) { r.Blockers.Current[0].Class = ClassMechanical },
			wantSub: "class",
		},
		{
			name:    "blocker reason code unknown",
			mutate:  func(r *Record) { r.Blockers.Current[0].Reason = "not-a-real-reason" },
			wantSub: "reason",
		},
		{
			name:    "blocker id empty",
			mutate:  func(r *Record) { r.Blockers.Current[0].ID = "" },
			wantSub: "id",
		},
		{
			name:    "blocker clearing condition empty",
			mutate:  func(r *Record) { r.Blockers.Current[0].ClearingCondition = "" },
			wantSub: "clearing_condition",
		},
		{
			name:    "blocker transition free text with spaces",
			mutate:  func(r *Record) { r.Blockers.Current[0].Transition = "close the thing" },
			wantSub: "transition",
		},
		{
			name:    "blocker transition empty",
			mutate:  func(r *Record) { r.Blockers.Current[0].Transition = "" },
			wantSub: "transition",
		},
		{
			name: "blocker owner attribution both principal and unauthenticated",
			mutate: func(r *Record) {
				id, err := governanceprincipal.CanonicalPrincipalID("github", "user-123")
				if err != nil {
					t.Fatalf("CanonicalPrincipalID: %v", err)
				}
				r.Blockers.Current[0].Owner.Attribution = governanceprincipal.Attribution{
					PrincipalID:     id,
					Unauthenticated: true,
				}
			},
			wantSub: "attribution",
		},
		{
			name:    "nil blockers.current",
			mutate:  func(r *Record) { r.Blockers.Current = nil },
			wantSub: "current",
		},
		{
			name: "eventual blockers not derived but items present",
			mutate: func(r *Record) {
				r.Blockers.Eventual.Derived = false
				r.Blockers.Eventual.Items = []Blocker{r.Blockers.Current[0]}
			},
			wantSub: "derived",
		},
		{
			name: "eventual blockers not derived and no disclosure",
			mutate: func(r *Record) {
				r.Blockers.Eventual.Derived = false
				r.Blockers.Eventual.Disclosures = []string{}
			},
			wantSub: "disclos",
		},
		{
			name: "duplicate blocker id across current and eventual",
			mutate: func(r *Record) {
				dup := r.Blockers.Current[0]
				dup.Class = ClassGovernance
				dup.Reason = ReasonPrincipalResolutionUnproven
				r.Blockers.Eventual.Derived = true
				r.Blockers.Eventual.Items = []Blocker{dup}
			},
			wantSub: "duplicate",
		},
		{
			name:    "unknown required role resolution",
			mutate:  func(r *Record) { r.Principals.Required[0].Resolution = "certified" },
			wantSub: "resolution",
		},
		{
			name:    "required role obligation malformed",
			mutate:  func(r *Record) { r.Principals.Required[0].Obligation = "countersign" },
			wantSub: "obligation",
		},
		{
			name:    "required role count zero",
			mutate:  func(r *Record) { r.Principals.Required[0].Count = 0 },
			wantSub: "count",
		},
		{
			name:    "required role transition empty",
			mutate:  func(r *Record) { r.Principals.Required[0].Transition = "" },
			wantSub: "transition",
		},
		{
			name:    "nil principals.required",
			mutate:  func(r *Record) { r.Principals.Required = nil },
			wantSub: "required",
		},
		{
			name:    "action id empty",
			mutate:  func(r *Record) { r.Actions.Safe[0].ID = "" },
			wantSub: "id",
		},
		{
			name:    "duplicate action ids",
			mutate:  func(r *Record) { r.Actions.Safe = append(r.Actions.Safe, r.Actions.Safe[0]) },
			wantSub: "duplicate",
		},
		{
			name:    "action verb malformed",
			mutate:  func(r *Record) { r.Actions.Safe[0].Verb = "Close!" },
			wantSub: "verb",
		},
		{
			name:    "action to_state malformed",
			mutate:  func(r *Record) { r.Actions.Safe[0].ToState = "Closed State" },
			wantSub: "to_state",
		},
		{
			name:    "action from_state empty",
			mutate:  func(r *Record) { r.Actions.Safe[0].FromState = "" },
			wantSub: "from_state",
		},
		{
			name:    "unknown action confirmation",
			mutate:  func(r *Record) { r.Actions.Safe[0].Confirmation = "maybe" },
			wantSub: "confirmation",
		},
		{
			name:    "action argument contains shell metacharacters",
			mutate:  func(r *Record) { r.Actions.Safe[0].Arguments = []string{"foo; rm -rf"} },
			wantSub: "argument",
		},
		{
			name:    "nil action arguments",
			mutate:  func(r *Record) { r.Actions.Safe[0].Arguments = nil },
			wantSub: "arguments",
		},
		{
			name:    "action with empty preconditions",
			mutate:  func(r *Record) { r.Actions.Safe[0].Preconditions = []Precondition{} },
			wantSub: "precondition",
		},
		{
			name:    "nil action preconditions",
			mutate:  func(r *Record) { r.Actions.Safe[0].Preconditions = nil },
			wantSub: "precondition",
		},
		{
			name: "precondition id malformed",
			mutate: func(r *Record) {
				r.Actions.Safe[0].Preconditions[0].ID = "Fold-Green"
			},
			wantSub: "precondition",
		},
		{
			name: "precondition witness empty",
			mutate: func(r *Record) {
				r.Actions.Safe[0].Preconditions[0].Witness = ""
			},
			wantSub: "witness",
		},
		{
			name:    "nil actions.safe",
			mutate:  func(r *Record) { r.Actions.Safe = nil },
			wantSub: "safe",
		},
		{
			name:    "unsorted needed facts",
			mutate:  func(r *Record) { r.Actions.NeededFacts = []string{"default-branch", "another-fact"} },
			wantSub: "sorted",
		},
		{
			name:    "nil needed facts",
			mutate:  func(r *Record) { r.Actions.NeededFacts = nil },
			wantSub: "needed_facts",
		},
		{
			name:    "nil top-level disclosures",
			mutate:  func(r *Record) { r.Disclosures = nil },
			wantSub: "disclosures",
		},
		{
			name:    "unsorted top-level disclosures",
			mutate:  func(r *Record) { r.Disclosures = []string{"b", "a"} },
			wantSub: "sorted",
		},
		{
			name:    "malformed digest",
			mutate:  func(r *Record) { r.Digest = "sha256:not-hex" },
			wantSub: "digest",
		},
		{
			name:    "digest missing prefix",
			mutate:  func(r *Record) { r.Digest = strings.Repeat("a", 64) },
			wantSub: "digest",
		},

		// --- grammar symmetry ---
		{
			name:    "action id malformed",
			mutate:  func(r *Record) { r.Actions.Safe[0].ID = "Close Issue" },
			wantSub: "id",
		},
		{
			name:    "blocker id malformed (double slash)",
			mutate:  func(r *Record) { r.Blockers.Current[0].ID = "forge-facts-unavailable//close" },
			wantSub: "id",
		},
		{
			name:    "blocker id malformed (trailing slash)",
			mutate:  func(r *Record) { r.Blockers.Current[0].ID = "forge-facts-unavailable/" },
			wantSub: "id",
		},
		{
			name:    "required role transition malformed",
			mutate:  func(r *Record) { r.Principals.Required[0].Transition = "Close Now" },
			wantSub: "transition",
		},
		{
			name:    "action from_state malformed",
			mutate:  func(r *Record) { r.Actions.Safe[0].FromState = "Accepted Pending" },
			wantSub: "from_state",
		},
		{
			name: "precondition duplicate id within action",
			mutate: func(r *Record) {
				r.Actions.Safe[0].Preconditions = []Precondition{
					{ID: "fold-green", Witness: "a"},
					{ID: "fold-green", Witness: "b"},
				}
			},
			wantSub: "preconditions",
		},
		{
			name: "precondition swapped order",
			mutate: func(r *Record) {
				r.Actions.Safe[0].Preconditions = []Precondition{
					{ID: "z-check", Witness: "a"},
					{ID: "a-check", Witness: "b"},
				}
			},
			wantSub: "preconditions",
		},

		// --- deterministic ordering (CO-4) ---
		{
			name: "blockers.current swapped order",
			mutate: func(r *Record) {
				owner := r.Blockers.Current[0].Owner
				r.Blockers.Current = []Blocker{
					{ID: "zzz-blocker", Reason: ReasonForgeFactsUnavailable, Class: ClassExternalWait, Witnesses: []string{}, Owner: owner, ClearingCondition: "x", Transition: "unknown"},
					{ID: "aaa-blocker", Reason: ReasonForgeFactsUnavailable, Class: ClassExternalWait, Witnesses: []string{}, Owner: owner, ClearingCondition: "x", Transition: "unknown"},
				}
			},
			wantSub: "blockers.current",
		},
		{
			name: "blockers.eventual.items swapped order",
			mutate: func(r *Record) {
				owner := r.Blockers.Current[0].Owner
				r.Blockers.Eventual.Derived = true
				r.Blockers.Eventual.Items = []Blocker{
					{ID: "zzz-eventual", Reason: ReasonForgeFactsUnavailable, Class: ClassExternalWait, Witnesses: []string{}, Owner: owner, ClearingCondition: "x", Transition: "unknown"},
					{ID: "aaa-eventual", Reason: ReasonForgeFactsUnavailable, Class: ClassExternalWait, Witnesses: []string{}, Owner: owner, ClearingCondition: "x", Transition: "unknown"},
				}
			},
			wantSub: "blockers.eventual.items",
		},
		{
			name: "actions.safe swapped order",
			mutate: func(r *Record) {
				a1, a2 := r.Actions.Safe[0], r.Actions.Safe[0]
				a1.ID, a2.ID = "zzz-action", "aaa-action"
				r.Actions.Safe = []Action{a1, a2}
			},
			wantSub: "actions.safe",
		},
		{
			name: "principals.required swapped order",
			mutate: func(r *Record) {
				r.Principals.Required = []RequiredRole{
					{Transition: "merge", Obligation: "attestation/countersign", Count: 1, Resolution: "unproven"},
					{Transition: "close", Obligation: "attestation/countersign", Count: 1, Resolution: "unproven"},
				}
			},
			wantSub: "principals.required",
		},
		{
			name: "action.authority swapped order",
			mutate: func(r *Record) {
				r.Actions.Safe[0].Authority = []RequiredRole{
					{Transition: "close", Obligation: "attestation/zzz-check", Count: 1, Resolution: "unproven"},
					{Transition: "close", Obligation: "attestation/aaa-check", Count: 1, Resolution: "unproven"},
				}
			},
			wantSub: "authority",
		},
		{
			name: "action.authority transition mismatches verb",
			mutate: func(r *Record) {
				r.Actions.Safe[0].Authority = []RequiredRole{
					{Transition: "merge", Obligation: "attestation/countersign", Count: 1, Resolution: "unproven"},
				}
			},
			wantSub: "authority",
		},
		{
			name:    "action.authority nil",
			mutate:  func(r *Record) { r.Actions.Safe[0].Authority = nil },
			wantSub: "authority",
		},

		// --- known/value converse arms ---
		{
			name:    "string fact known true with empty value",
			mutate:  func(r *Record) { r.Repository.Branch = StringFact{Known: true, Value: ""} },
			wantSub: "known",
		},
		{
			name: "default branch fact known true with missing field",
			mutate: func(r *Record) {
				r.Repository.DefaultBranch = DefaultBranchFact{Known: true, Name: "main", Ref: "", Head: "abc123"}
			},
			wantSub: "known",
		},
		{
			name:    "worktree fact managed true with empty name",
			mutate:  func(r *Record) { r.Repository.Worktree = WorktreeFact{Managed: true, Name: ""} },
			wantSub: "managed",
		},

		// --- schema completion (lifecycle posture / active branch) ---
		{
			name:    "unknown lifecycle posture",
			mutate:  func(r *Record) { r.Lifecycle.Posture = "mystery" },
			wantSub: "posture",
		},
		{
			name:    "lifecycle active branch known false with nonempty value",
			mutate:  func(r *Record) { r.Lifecycle.ActiveBranch = StringFact{Known: false, Value: "x"} },
			wantSub: "known",
		},

		// --- PrincipalFacts CO-1 mirror ---
		{
			name: "principals profile not adopted without disclosure",
			mutate: func(r *Record) {
				r.Principals.ProfileAdopted = false
				r.Principals.Disclosures = []string{}
			},
			wantSub: "disclos",
		},

		// --- reason/blocker-class cleanup and coverage gaps ---
		{
			name:    "unknown blocker class value entirely",
			mutate:  func(r *Record) { r.Blockers.Current[0].Class = "urgent" },
			wantSub: "unknown class",
		},
		{
			name: "nil blockers.eventual.items",
			mutate: func(r *Record) {
				r.Blockers.Eventual.Items = nil
			},
			wantSub: "items",
		},
		{
			name:    "nil blockers.eventual.disclosures",
			mutate:  func(r *Record) { r.Blockers.Eventual.Disclosures = nil },
			wantSub: "disclosures",
		},
		{
			name:    "unsorted blockers.eventual.disclosures",
			mutate:  func(r *Record) { r.Blockers.Eventual.Disclosures = []string{"b", "a"} },
			wantSub: "sorted",
		},
		{
			name:    "nil principals.disclosures",
			mutate:  func(r *Record) { r.Principals.Disclosures = nil },
			wantSub: "disclosures",
		},
		{
			name:    "unsorted principals.disclosures",
			mutate:  func(r *Record) { r.Principals.Disclosures = []string{"b", "a"} },
			wantSub: "sorted",
		},
		{
			name:    "unsorted lifecycle.disclosures",
			mutate:  func(r *Record) { r.Lifecycle.Disclosures = []string{"b", "a"} },
			wantSub: "sorted",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := validRecord(t)
			tt.mutate(&r)
			err := r.Validate()
			if err == nil {
				t.Fatalf("Validate(): expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Errorf("Validate() error = %q, want substring %q", err.Error(), tt.wantSub)
			}
		})
	}
}

// --- evidence section (DC-2's always-visible operands) -------------------

func TestEvidenceFactsValidateHappy(t *testing.T) {
	tests := []struct {
		name string
		ef   EvidenceFacts
	}{
		{
			"this delivery unit's shape: unknown/unknown, disclosed, no contributors",
			EvidenceFacts{
				Authority:    "unknown",
				Freshness:    "unknown",
				Contributors: []EvidenceContributor{},
				Disclosures:  []string{"evidence operands are not consumed by this delivery unit"},
			},
		},
		{
			"contributors ascending by id, every kind and resolution legal",
			EvidenceFacts{
				Authority: "authoritative",
				Freshness: "fresh",
				Contributors: []EvidenceContributor{
					{ID: "attestation", Kind: "attestation", Resolution: "proven", Witness: "w1"},
					{ID: "behavioral", Kind: "behavioral", Resolution: "violated-with-witness", Witness: "w2"},
					{ID: "runtime", Kind: "runtime", Resolution: "unproven", Witness: "w3"},
					{ID: "static", Kind: "static", Resolution: "unproven", Witness: "w4"},
				},
				Disclosures: []string{},
			},
		},
		{
			"advisory/stale needs no disclosure: both are PROVEN postures",
			EvidenceFacts{
				Authority:    "advisory",
				Freshness:    "stale",
				Contributors: []EvidenceContributor{},
				Disclosures:  []string{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.ef.validate(); err != nil {
				t.Fatalf("EvidenceFacts.validate() = %v, want nil", err)
			}
			r := validRecord(t)
			r.Evidence = tt.ef
			if err := r.Validate(); err != nil {
				t.Fatalf("Record.Validate() with this evidence section = %v, want nil", err)
			}
		})
	}
}

func TestEvidenceFactsValidateNegative(t *testing.T) {
	base := func() EvidenceFacts {
		return EvidenceFacts{
			Authority:    "authoritative",
			Freshness:    "fresh",
			Contributors: []EvidenceContributor{},
			Disclosures:  []string{},
		}
	}
	tests := []struct {
		name    string
		mutate  func(*EvidenceFacts)
		wantSub string
	}{
		{"unknown authority", func(e *EvidenceFacts) { e.Authority = "trusted" }, "authority"},
		{"empty authority", func(e *EvidenceFacts) { e.Authority = "" }, "authority"},
		{"unknown freshness", func(e *EvidenceFacts) { e.Freshness = "recent" }, "freshness"},
		{"empty freshness", func(e *EvidenceFacts) { e.Freshness = "" }, "freshness"},
		{"nil contributors", func(e *EvidenceFacts) { e.Contributors = nil }, "contributors"},
		{"nil disclosures", func(e *EvidenceFacts) { e.Disclosures = nil }, "disclosures"},
		{
			"unsorted disclosures",
			func(e *EvidenceFacts) { e.Disclosures = []string{"b", "a"} },
			"sorted",
		},
		{
			"duplicate disclosures",
			func(e *EvidenceFacts) { e.Disclosures = []string{"a", "a"} },
			"sorted",
		},
		{
			"malformed contributor id",
			func(e *EvidenceFacts) {
				e.Contributors = []EvidenceContributor{{ID: "Static", Kind: "static", Resolution: "unproven", Witness: "w"}}
			},
			"id",
		},
		{
			"unknown contributor kind",
			func(e *EvidenceFacts) {
				e.Contributors = []EvidenceContributor{{ID: "static", Kind: "vibes", Resolution: "unproven", Witness: "w"}}
			},
			"kind",
		},
		{
			"unknown contributor resolution",
			func(e *EvidenceFacts) {
				e.Contributors = []EvidenceContributor{{ID: "static", Kind: "static", Resolution: "authenticated", Witness: "w"}}
			},
			"resolution",
		},
		{
			"contributor with no witness",
			func(e *EvidenceFacts) {
				e.Contributors = []EvidenceContributor{{ID: "static", Kind: "static", Resolution: "unproven"}}
			},
			"witness",
		},
		{
			"contributors out of order",
			func(e *EvidenceFacts) {
				e.Contributors = []EvidenceContributor{
					{ID: "static", Kind: "static", Resolution: "unproven", Witness: "w"},
					{ID: "behavioral", Kind: "behavioral", Resolution: "unproven", Witness: "w"},
				}
			},
			"ascending",
		},
		{
			"duplicate contributor ids",
			func(e *EvidenceFacts) {
				e.Contributors = []EvidenceContributor{
					{ID: "static", Kind: "static", Resolution: "unproven", Witness: "w"},
					{ID: "static", Kind: "static", Resolution: "unproven", Witness: "w"},
				}
			},
			"ascending",
		},
		{
			// CO-1: an unknown operand must disclose itself.
			"unknown authority with no disclosure",
			func(e *EvidenceFacts) { e.Authority = "unknown" },
			"disclos",
		},
		{
			"unknown freshness with no disclosure",
			func(e *EvidenceFacts) { e.Freshness = "unknown" },
			"disclos",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ef := base()
			tt.mutate(&ef)
			err := ef.validate()
			if err == nil {
				t.Fatalf("EvidenceFacts.validate() = nil, want an error mentioning %q", tt.wantSub)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Fatalf("EvidenceFacts.validate() = %q, want it to mention %q", err.Error(), tt.wantSub)
			}

			// The same rule must be reachable through the whole record —
			// a section validated only in isolation is not fail-closed.
			r := validRecord(t)
			r.Evidence = ef
			if rerr := r.Validate(); rerr == nil {
				t.Fatalf("Record.Validate() = nil for an invalid evidence section (%q)", tt.name)
			}
		})
	}
}

// TestEvidenceKindParityWithArtifact proves journey's closed evidence-kind
// vocabulary is exactly internal/artifact.EvidenceKind's constant set: a
// spec's acceptance criteria may declare ANY of those kinds, and a
// contributor derived from one must never fail this schema's own
// fail-closed enum and abort the projection over a legal spec. record.go
// never imports internal/artifact in production; only this test does.
func TestEvidenceKindParityWithArtifact(t *testing.T) {
	want := []string{
		string(artifact.EvidenceStatic),
		string(artifact.EvidenceBehavioral),
		string(artifact.EvidenceRuntime),
		string(artifact.EvidenceAttestation),
	}
	sort.Strings(want)

	got := make([]string, 0, len(validEvidenceKind))
	for k := range validEvidenceKind {
		got = append(got, k)
	}
	sort.Strings(got)

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("journey evidence kinds = %v, internal/artifact evidence kinds = %v", got, want)
	}
}
