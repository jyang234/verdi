package forge_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/forge"
)

// mrFactsDigest re-derives MergeRecordFacts' provider snapshot digest the way
// the package documents it — the canonical digest of every field except
// ObservedAt and ProviderSnapshotID — so a test can re-seal facts it
// reshaped and reach a rule other than the digest check.
func mrFactsDigest(t *testing.T, facts forge.MergeRecordFacts) string {
	t.Helper()
	facts.ObservedAt = ""
	facts.ProviderSnapshotID = ""
	digest, err := canonjson.Digest(facts)
	if err != nil {
		t.Fatalf("digest merge-record facts: %v", err)
	}
	return digest
}

// Merge-record facts fixtures (SI-249; plan R-PB-2). mrCommit is the commit
// the forge is asked about; mrOtherCommit is any other commit.
const (
	mrCommit       = "6dcb09b5b57875f334f61aebed695e2e4193db5e"
	mrOtherCommit  = "e5bd3914e2e596debea16f433f57875b5b90bcd6"
	mrCommit256    = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	mrMergedAt     = "2026-09-20T10:15:30Z"
	mrDefault      = "main"
	mrRepository   = "octocat/Hello-World"
	mrUnsupportedR = "fake: merge records are not supported by this adapter"
)

func mrOpen(id string) forge.ChangeRequestMerge {
	return forge.ChangeRequestMerge{ChangeID: id, State: forge.ChangeRequestOpen, TargetBranch: mrDefault}
}

func mrClosed(id string) forge.ChangeRequestMerge {
	return forge.ChangeRequestMerge{ChangeID: id, State: forge.ChangeRequestClosed, TargetBranch: mrDefault}
}

func mrMerged(id, target, mergeSHA string) forge.ChangeRequestMerge {
	return forge.ChangeRequestMerge{
		ChangeID: id, State: forge.ChangeRequestMerged, TargetBranch: target,
		MergeCommitSHA: mergeSHA, MergedAt: mrMergedAt,
	}
}

// mrDraft is a supported observation of one change merged into the default
// branch by the forge with mrCommit as its merge commit.
func mrDraft() forge.MergeRecordFacts {
	return forge.MergeRecordFacts{
		Supported: true, Repository: mrRepository, Commit: mrCommit, DefaultBranch: mrDefault,
		Changes: []forge.ChangeRequestMerge{mrMerged("1347", mrDefault, mrCommit)},
	}
}

func mrUnsupportedDraft() forge.MergeRecordFacts {
	return forge.MergeRecordFacts{Supported: false, UnsupportedReason: mrUnsupportedR, Repository: mrRepository, Commit: mrCommit}
}

func mustMergeRecordFacts(t *testing.T, draft forge.MergeRecordFacts) forge.MergeRecordFacts {
	t.Helper()
	facts, err := forge.NewMergeRecordFacts(draft, fixedClock())
	if err != nil {
		t.Fatalf("NewMergeRecordFacts: %v", err)
	}
	return facts
}

func changeIDs(changes []forge.ChangeRequestMerge) []string {
	ids := make([]string, 0, len(changes))
	for _, change := range changes {
		ids = append(ids, change.ChangeID)
	}
	return ids
}

func TestNewMergeRecordFacts(t *testing.T) {
	t.Run("sorts changes numerically by change id", func(t *testing.T) {
		draft := mrDraft()
		draft.Changes = []forge.ChangeRequestMerge{mrOpen("100"), mrClosed("9"), mrMerged("20", mrDefault, mrCommit), mrOpen("3")}
		facts := mustMergeRecordFacts(t, draft)
		if got, want := changeIDs(facts.Changes), []string{"3", "9", "20", "100"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("change ids = %v, want numeric order %v", got, want)
		}
		if got := changeIDs(draft.Changes); !reflect.DeepEqual(got, []string{"100", "9", "20", "3"}) {
			t.Fatalf("NewMergeRecordFacts reordered the caller's draft: %v", got)
		}
	})

	t.Run("a nil change list becomes an explicit empty list", func(t *testing.T) {
		draft := mrDraft()
		draft.Changes = nil
		facts := mustMergeRecordFacts(t, draft)
		if facts.Changes == nil || len(facts.Changes) != 0 {
			t.Fatalf("Changes = %#v, want a non-nil empty list", facts.Changes)
		}
	})

	t.Run("observation time is normalized to UTC", func(t *testing.T) {
		observed := time.Date(2026, 9, 23, 9, 30, 0, 500, time.FixedZone("PDT", -7*3600))
		facts, err := forge.NewMergeRecordFacts(mrDraft(), observed)
		if err != nil {
			t.Fatalf("NewMergeRecordFacts: %v", err)
		}
		if facts.ObservedAt != "2026-09-23T16:30:00.0000005Z" {
			t.Fatalf("ObservedAt = %q, want normalized UTC RFC3339Nano", facts.ObservedAt)
		}
	})

	t.Run("the provider snapshot digest excludes observation time", func(t *testing.T) {
		first, err := forge.NewMergeRecordFacts(mrDraft(), fixedClock())
		if err != nil {
			t.Fatalf("first: %v", err)
		}
		second, err := forge.NewMergeRecordFacts(mrDraft(), fixedClock().Add(72*time.Hour))
		if err != nil {
			t.Fatalf("second: %v", err)
		}
		if first.ObservedAt == second.ObservedAt {
			t.Fatalf("fixture: both observations carry %q", first.ObservedAt)
		}
		if first.ProviderSnapshotID != second.ProviderSnapshotID {
			t.Fatalf("ProviderSnapshotID %q != %q: the digest must not depend on observation time", first.ProviderSnapshotID, second.ProviderSnapshotID)
		}
		if !strings.HasPrefix(first.ProviderSnapshotID, "sha256:") {
			t.Fatalf("ProviderSnapshotID = %q, want a sha256 digest", first.ProviderSnapshotID)
		}
	})

	t.Run("the provider snapshot digest covers every provider fact", func(t *testing.T) {
		base := mustMergeRecordFacts(t, mrDraft())
		for name, mutate := range map[string]func(*forge.MergeRecordFacts){
			"repository":       func(f *forge.MergeRecordFacts) { f.Repository = "octocat/Spoon-Knife" },
			"commit":           func(f *forge.MergeRecordFacts) { f.Commit = mrOtherCommit },
			"default branch":   func(f *forge.MergeRecordFacts) { f.DefaultBranch = "trunk" },
			"change state":     func(f *forge.MergeRecordFacts) { f.Changes[0] = mrClosed("1347") },
			"target branch":    func(f *forge.MergeRecordFacts) { f.Changes[0].TargetBranch = "release" },
			"merge commit sha": func(f *forge.MergeRecordFacts) { f.Changes[0].MergeCommitSHA = mrOtherCommit },
			"merged at":        func(f *forge.MergeRecordFacts) { f.Changes[0].MergedAt = "2026-09-21T00:00:00Z" },
			"another change":   func(f *forge.MergeRecordFacts) { f.Changes = append(f.Changes, mrOpen("1348")) },
		} {
			t.Run(name, func(t *testing.T) {
				draft := mrDraft()
				mutate(&draft)
				changed := mustMergeRecordFacts(t, draft)
				if changed.ProviderSnapshotID == base.ProviderSnapshotID {
					t.Fatalf("changing the %s left the provider snapshot digest unchanged", name)
				}
			})
		}
	})

	t.Run("accepts 64-character commits and unsupported facts", func(t *testing.T) {
		draft := mrDraft()
		draft.Commit = mrCommit256
		draft.Changes = []forge.ChangeRequestMerge{mrMerged("1", mrDefault, mrCommit256)}
		mustMergeRecordFacts(t, draft)
		unsupported := mustMergeRecordFacts(t, mrUnsupportedDraft())
		if unsupported.Changes == nil || len(unsupported.Changes) != 0 {
			t.Fatalf("unsupported Changes = %#v, want a non-nil empty list", unsupported.Changes)
		}
	})

	t.Run("round-trips through JSON and still validates", func(t *testing.T) {
		facts := mustMergeRecordFacts(t, mrDraft())
		data, err := json.Marshal(facts)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		var decoded forge.MergeRecordFacts
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if err := decoded.Validate(); err != nil {
			t.Fatalf("Validate after round trip: %v", err)
		}
		if !reflect.DeepEqual(decoded, facts) {
			t.Fatalf("round trip = %+v, want %+v", decoded, facts)
		}
	})
}

func TestMergeRecordFactsRefusals(t *testing.T) {
	t.Run("supported drafts that break the facts contract", func(t *testing.T) {
		tests := []struct {
			name   string
			mutate func(*forge.MergeRecordFacts)
		}{
			{"missing repository", func(f *forge.MergeRecordFacts) { f.Repository = "" }},
			{"repository with surrounding whitespace", func(f *forge.MergeRecordFacts) { f.Repository = " octocat/Hello-World" }},
			{"missing commit", func(f *forge.MergeRecordFacts) { f.Commit = "" }},
			{"abbreviated commit", func(f *forge.MergeRecordFacts) { f.Commit = mrCommit[:12] }},
			{"uppercase commit", func(f *forge.MergeRecordFacts) { f.Commit = strings.ToUpper(mrCommit) }},
			{"non-hex commit", func(f *forge.MergeRecordFacts) { f.Commit = strings.Repeat("g", 40) }},
			{"missing default branch", func(f *forge.MergeRecordFacts) { f.DefaultBranch = "" }},
			{"default branch with surrounding whitespace", func(f *forge.MergeRecordFacts) { f.DefaultBranch = "main " }},
			{"unsupported reason on supported facts", func(f *forge.MergeRecordFacts) { f.UnsupportedReason = "why" }},
			{"missing change id", func(f *forge.MergeRecordFacts) { f.Changes[0].ChangeID = "" }},
			{"zero change id", func(f *forge.MergeRecordFacts) { f.Changes[0].ChangeID = "0" }},
			{"negative change id", func(f *forge.MergeRecordFacts) { f.Changes[0].ChangeID = "-7" }},
			{"signed change id", func(f *forge.MergeRecordFacts) { f.Changes[0].ChangeID = "+7" }},
			{"zero-padded change id", func(f *forge.MergeRecordFacts) { f.Changes[0].ChangeID = "07" }},
			{"non-numeric change id", func(f *forge.MergeRecordFacts) { f.Changes[0].ChangeID = "!7" }},
			{"duplicate change id", func(f *forge.MergeRecordFacts) { f.Changes = append(f.Changes, mrOpen("1347")) }},
			{"unknown change state", func(f *forge.MergeRecordFacts) { f.Changes[0].State = "reopened" }},
			{"missing change state", func(f *forge.MergeRecordFacts) { f.Changes[0].State = "" }},
			{"missing target branch", func(f *forge.MergeRecordFacts) { f.Changes[0].TargetBranch = "" }},
			{"merge commit on an open change", func(f *forge.MergeRecordFacts) {
				f.Changes[0] = mrOpen("1347")
				f.Changes[0].MergeCommitSHA = mrCommit
			}},
			{"merge commit on a closed change", func(f *forge.MergeRecordFacts) {
				f.Changes[0] = mrClosed("1347")
				f.Changes[0].MergeCommitSHA = mrCommit
			}},
			{"merge time on an open change", func(f *forge.MergeRecordFacts) {
				f.Changes[0] = mrOpen("1347")
				f.Changes[0].MergedAt = mrMergedAt
			}},
			{"merge time on a closed change", func(f *forge.MergeRecordFacts) {
				f.Changes[0] = mrClosed("1347")
				f.Changes[0].MergedAt = mrMergedAt
			}},
			{"abbreviated merge commit", func(f *forge.MergeRecordFacts) { f.Changes[0].MergeCommitSHA = mrCommit[:7] }},
			{"uppercase merge commit", func(f *forge.MergeRecordFacts) { f.Changes[0].MergeCommitSHA = strings.ToUpper(mrCommit) }},
			{"merged without a merge time", func(f *forge.MergeRecordFacts) { f.Changes[0].MergedAt = "" }},
			{"merge time not in UTC", func(f *forge.MergeRecordFacts) { f.Changes[0].MergedAt = "2026-09-20T12:15:30+02:00" }},
			{"merge time not a timestamp", func(f *forge.MergeRecordFacts) { f.Changes[0].MergedAt = "yesterday" }},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				draft := mrDraft()
				tt.mutate(&draft)
				if facts, err := forge.NewMergeRecordFacts(draft, fixedClock()); err == nil {
					t.Fatalf("NewMergeRecordFacts(%s) = %+v, want error", tt.name, facts)
				}
			})
		}
	})

	t.Run("unsupported drafts carry a reason, the repository, and the commit, and nothing else", func(t *testing.T) {
		tests := []struct {
			name   string
			mutate func(*forge.MergeRecordFacts)
		}{
			{"no reason", func(f *forge.MergeRecordFacts) { f.UnsupportedReason = "" }},
			{"reason with surrounding whitespace", func(f *forge.MergeRecordFacts) { f.UnsupportedReason = mrUnsupportedR + "\n" }},
			{"missing repository", func(f *forge.MergeRecordFacts) { f.Repository = "" }},
			{"missing commit", func(f *forge.MergeRecordFacts) { f.Commit = "" }},
			{"abbreviated commit", func(f *forge.MergeRecordFacts) { f.Commit = mrCommit[:10] }},
			{"a default branch", func(f *forge.MergeRecordFacts) { f.DefaultBranch = mrDefault }},
			{"changes", func(f *forge.MergeRecordFacts) { f.Changes = []forge.ChangeRequestMerge{mrOpen("1")} }},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				draft := mrUnsupportedDraft()
				tt.mutate(&draft)
				if facts, err := forge.NewMergeRecordFacts(draft, fixedClock()); err == nil {
					t.Fatalf("NewMergeRecordFacts(%s) = %+v, want error", tt.name, facts)
				}
			})
		}
	})

	t.Run("validate refuses values changed after construction", func(t *testing.T) {
		valid := mustMergeRecordFacts(t, mrDraft())
		twoChanges := mrDraft()
		twoChanges.Changes = append(twoChanges.Changes, mrOpen("1348"))
		sorted := mustMergeRecordFacts(t, twoChanges)
		tests := []struct {
			name   string
			mutate func(*forge.MergeRecordFacts)
		}{
			{"null changes", func(f *forge.MergeRecordFacts) { f.Changes = nil }},
			{"tampered after its digest", func(f *forge.MergeRecordFacts) { f.DefaultBranch = "trunk" }},
			{"tampered change after its digest", func(f *forge.MergeRecordFacts) { f.Changes[0].TargetBranch = "release" }},
			{"missing digest", func(f *forge.MergeRecordFacts) { f.ProviderSnapshotID = "" }},
			{"malformed digest", func(f *forge.MergeRecordFacts) { f.ProviderSnapshotID = "sha256:xyz" }},
			{"uppercase digest", func(f *forge.MergeRecordFacts) { f.ProviderSnapshotID = strings.ToUpper(f.ProviderSnapshotID) }},
			{"missing observation time", func(f *forge.MergeRecordFacts) { f.ObservedAt = "" }},
			{"observation time not in UTC", func(f *forge.MergeRecordFacts) { f.ObservedAt = "2026-08-26T09:30:00-07:00" }},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				facts := valid
				facts.Changes = append([]forge.ChangeRequestMerge(nil), valid.Changes...)
				tt.mutate(&facts)
				if err := facts.Validate(); err == nil {
					t.Fatalf("Validate(%s): want error, got nil", tt.name)
				}
			})
		}
		// The unsorted row re-derives the digest for the swapped order, so the
		// digest check passes and only the sort rule can refuse it (lane EF
		// review F3: swapping alone also broke the digest, so the row passed
		// with the sort rule disabled).
		t.Run("unsorted changes", func(t *testing.T) {
			if got := mrFactsDigest(t, sorted); got != sorted.ProviderSnapshotID {
				t.Fatalf("mrFactsDigest(sorted) = %s, want the constructor's %s; the re-derivation below would prove nothing", got, sorted.ProviderSnapshotID)
			}
			facts := sorted
			facts.Changes = []forge.ChangeRequestMerge{sorted.Changes[1], sorted.Changes[0]}
			facts.ProviderSnapshotID = mrFactsDigest(t, facts)
			err := facts.Validate()
			if err == nil {
				t.Fatal("Validate(unsorted changes): want error, got nil")
			}
			if want := "not sorted by change_id"; !strings.Contains(err.Error(), want) {
				t.Fatalf("Validate(unsorted changes) = %q, want the sort rule's error naming %q", err, want)
			}
		})
		t.Run("zero value", func(t *testing.T) {
			if err := (forge.MergeRecordFacts{}).Validate(); err == nil {
				t.Fatal("Validate(zero value): want error, got nil")
			}
		})
	})

	t.Run("unknown change states fail closed at decode", func(t *testing.T) {
		for _, raw := range []string{`"reopened"`, `""`, `"OPEN"`, `7`, `null`} {
			var state forge.ChangeRequestState
			if err := json.Unmarshal([]byte(raw), &state); err == nil {
				t.Fatalf("decoding change state %s = %q, want error", raw, state)
			}
		}
		for _, want := range []forge.ChangeRequestState{forge.ChangeRequestOpen, forge.ChangeRequestClosed, forge.ChangeRequestMerged} {
			var state forge.ChangeRequestState
			if err := json.Unmarshal([]byte(`"`+string(want)+`"`), &state); err != nil || state != want {
				t.Fatalf("decoding change state %q = %q, %v", want, state, err)
			}
		}
	})
}

func TestValidateMergeRecordCommit(t *testing.T) {
	for _, commit := range []string{mrCommit, mrCommit256} {
		if err := forge.ValidateMergeRecordCommit(commit); err != nil {
			t.Fatalf("ValidateMergeRecordCommit(%q): %v", commit, err)
		}
	}
	for _, commit := range []string{"", "main", "HEAD", mrCommit[:12], strings.ToUpper(mrCommit), mrCommit + "0", strings.Repeat("z", 40), " " + mrCommit[1:]} {
		if err := forge.ValidateMergeRecordCommit(commit); err == nil {
			t.Fatalf("ValidateMergeRecordCommit(%q): want error, got nil", commit)
		}
	}
}

func TestProveMergedIntoDefault(t *testing.T) {
	with := func(changes ...forge.ChangeRequestMerge) forge.MergeRecordFacts {
		draft := mrDraft()
		draft.Changes = changes
		return draft
	}
	tests := []struct {
		name         string
		draft        forge.MergeRecordFacts
		wantState    forge.MergeProofState
		wantChange   string
		wantReason   string
		detailSubstr []string
	}{
		{
			name: "merged into the default branch with the exact merge commit", draft: mrDraft(),
			wantState: forge.MergeProofProven, wantChange: "1347",
		},
		{
			name: "one qualifying change among others", draft: with(mrOpen("5"), mrMerged("9", "release", mrCommit), mrMerged("12", mrDefault, mrCommit), mrClosed("30")),
			wantState: forge.MergeProofProven, wantChange: "12",
		},
		{
			name: "several qualifying changes report the lowest change id", draft: with(mrMerged("10", mrDefault, mrCommit), mrMerged("9", mrDefault, mrCommit), mrMerged("100", mrDefault, mrCommit)),
			wantState: forge.MergeProofProven, wantChange: "9",
			detailSubstr: []string{"changes 9, 10, 100", "lowest change id"},
		},
		{
			name: "unsupported adapter", draft: mrUnsupportedDraft(),
			wantState: forge.MergeProofUnproven, wantReason: forge.MergeProofReasonUnsupported,
			detailSubstr: []string{mrUnsupportedR},
		},
		{
			name: "no associated change", draft: with(),
			wantState: forge.MergeProofUnproven, wantReason: forge.MergeProofReasonNoMergeIntoDefault,
			detailSubstr: []string{"no change request", mrCommit},
		},
		{
			// The adapter never carries an open change's provider test-merge
			// sha, even when it equals the commit; the facts say only "open".
			name: "open change whose provider test merge equals the commit", draft: with(mrOpen("1347")),
			wantState: forge.MergeProofUnproven, wantReason: forge.MergeProofReasonNoMergeIntoDefault,
			detailSubstr: []string{"change 1347", "state=open", "test merge"},
		},
		{
			name: "closed without merging", draft: with(mrClosed("1347")),
			wantState: forge.MergeProofUnproven, wantReason: forge.MergeProofReasonNoMergeIntoDefault,
			detailSubstr: []string{"change 1347", "closed without merging"},
		},
		{
			name: "merged into another branch with the commit as its merge commit", draft: with(mrMerged("1347", "release", mrCommit)),
			wantState: forge.MergeProofUnproven, wantReason: forge.MergeProofReasonNoMergeIntoDefault,
			detailSubstr: []string{"change 1347", "merged into release", "not the default branch main"},
		},
		{
			name: "merged into the default branch with a different merge commit", draft: with(mrMerged("1347", mrDefault, mrOtherCommit)),
			wantState: forge.MergeProofUnproven, wantReason: forge.MergeProofReasonNoMergeIntoDefault,
			detailSubstr: []string{"change 1347", "merge commit " + mrOtherCommit, "not " + mrCommit},
		},
		{
			name: "merged with no merge commit (a GitLab fast-forward merge)", draft: with(mrMerged("1347", mrDefault, "")),
			wantState: forge.MergeProofUnproven, wantReason: forge.MergeProofReasonNoMergeIntoDefault,
			detailSubstr: []string{"change 1347", "no forge-reported merge commit", "fast-forward"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			facts := mustMergeRecordFacts(t, tt.draft)
			proof, err := forge.ProveMergedIntoDefault(facts)
			if err != nil {
				t.Fatalf("ProveMergedIntoDefault: %v", err)
			}
			if proof.State != tt.wantState {
				t.Fatalf("State = %q, want %q (proof %+v)", proof.State, tt.wantState, proof)
			}
			if proof.Commit != facts.Commit || proof.ProviderSnapshotID != facts.ProviderSnapshotID {
				t.Fatalf("proof %+v does not bind the facts' commit %q and snapshot %q", proof, facts.Commit, facts.ProviderSnapshotID)
			}
			if proof.Reason != tt.wantReason {
				t.Fatalf("Reason = %q, want %q", proof.Reason, tt.wantReason)
			}
			for _, substr := range tt.detailSubstr {
				if !strings.Contains(proof.Detail, substr) {
					t.Fatalf("Detail = %q, want it to name %q", proof.Detail, substr)
				}
			}
			if tt.wantState == forge.MergeProofProven {
				if proof.ChangeID != tt.wantChange || proof.TargetBranch != facts.DefaultBranch || proof.MergedAt != mrMergedAt {
					t.Fatalf("proven proof = %+v, want change %q into %q merged at %q", proof, tt.wantChange, facts.DefaultBranch, mrMergedAt)
				}
				return
			}
			if proof.ChangeID != "" || proof.TargetBranch != "" || proof.MergedAt != "" {
				t.Fatalf("unproven proof %+v carries proven-only fields", proof)
			}
			if proof.Detail == "" {
				t.Fatal("unproven proof carries no detail")
			}
		})
	}

	t.Run("proofs are deterministic", func(t *testing.T) {
		facts := mustMergeRecordFacts(t, with(mrOpen("2"), mrMerged("1", "release", mrCommit)))
		first, err := forge.ProveMergedIntoDefault(facts)
		if err != nil {
			t.Fatalf("first: %v", err)
		}
		second, err := forge.ProveMergedIntoDefault(facts)
		if err != nil {
			t.Fatalf("second: %v", err)
		}
		if first != second {
			t.Fatalf("proofs differ: %+v vs %+v", first, second)
		}
	})

	t.Run("a proven proof encodes no unproven members", func(t *testing.T) {
		proof, err := forge.ProveMergedIntoDefault(mustMergeRecordFacts(t, mrDraft()))
		if err != nil {
			t.Fatalf("ProveMergedIntoDefault: %v", err)
		}
		data, err := json.Marshal(proof)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if strings.Contains(string(data), `"reason"`) || !strings.Contains(string(data), `"change_id":"1347"`) {
			t.Fatalf("proven proof JSON = %s", data)
		}
	})

	t.Run("invalid facts are an error, never a proof", func(t *testing.T) {
		valid := mustMergeRecordFacts(t, mrDraft())
		tampered := valid
		tampered.Changes = []forge.ChangeRequestMerge{mrMerged("1347", mrDefault, mrOtherCommit)}
		retargeted := valid
		retargeted.DefaultBranch = "release"
		for name, facts := range map[string]forge.MergeRecordFacts{
			"zero value":                    {},
			"hand-built and never digested": mrDraft(),
			"changed after its digest":      tampered,
			"default branch changed":        retargeted,
		} {
			t.Run(name, func(t *testing.T) {
				proof, err := forge.ProveMergedIntoDefault(facts)
				if err == nil {
					t.Fatalf("ProveMergedIntoDefault(%s) = %+v, want error", name, proof)
				}
				if proof != (forge.MergeProof{}) {
					t.Fatalf("ProveMergedIntoDefault(%s) returned proof %+v alongside its error", name, proof)
				}
			})
		}
	})
}
