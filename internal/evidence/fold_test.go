package evidence

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

func ac(id string, kinds ...artifact.EvidenceKind) artifact.AcceptanceCriterion {
	return artifact.AcceptanceCriterion{ID: id, Text: "text for " + id, Evidence: kinds}
}

func testSpec(story string, acs ...artifact.AcceptanceCriterion) *artifact.SpecFrontmatter {
	return &artifact.SpecFrontmatter{
		Base: artifact.Base{
			ID:     "spec/test-story",
			Kind:   artifact.KindSpec,
			Title:  "Test story",
			Owners: []string{"platform-team"},
		},
		Class:              artifact.ClassFeature,
		Status:             "accepted-pending-build",
		Story:              story,
		AcceptanceCriteria: acs,
	}
}

// foldOneAC runs Fold on a single-AC spec with the given records and
// returns that AC's result, for tests that only care about one AC's
// status in isolation.
func foldOneAC(t *testing.T, storeRoot string, theAC artifact.AcceptanceCriterion, records []artifact.Evidence) ACResult {
	t.Helper()
	spec := testSpec("jira:TEST-1", theAC)
	result, err := Fold(Input{
		Spec:      spec,
		Records:   records,
		StoreRoot: storeRoot,
		StorySlug: "test-1",
	})
	if err != nil {
		t.Fatalf("Fold: %v", err)
	}
	if len(result.ACs) != 1 {
		t.Fatalf("Fold produced %d AC results, want 1", len(result.ACs))
	}
	return result.ACs[0]
}

// TestFold_EveryStatus proves each of the five fold statuses is reachable
// (03 §The fold's full status set).
//
// guide-claim: 7.2-fold
func TestFold_EveryStatus(t *testing.T) {
	root := t.TempDir()
	writeWaiver(t, root, "test-1", "ac-waived", testActiveWaiver)

	cases := []struct {
		name    string
		ac      artifact.AcceptanceCriterion
		records []artifact.Evidence
		want    Status
	}{
		{
			name:    "waived",
			ac:      ac("ac-waived", artifact.EvidenceStatic),
			records: nil,
			want:    StatusWaived,
		},
		{
			name: "violated",
			ac:   ac("ac-1", artifact.EvidenceStatic),
			records: []artifact.Evidence{
				testEvidence(artifact.EvidenceStatic, artifact.VerdictFail, "ac-1"),
			},
			want: StatusViolated,
		},
		{
			name: "evidenced",
			ac:   ac("ac-1", artifact.EvidenceStatic),
			records: []artifact.Evidence{
				testEvidence(artifact.EvidenceStatic, artifact.VerdictPass, "ac-1"),
			},
			want: StatusEvidenced,
		},
		{
			name: "pending (abstain record, no pass yet)",
			ac:   ac("ac-1", artifact.EvidenceBehavioral),
			records: []artifact.Evidence{
				testEvidence(artifact.EvidenceBehavioral, artifact.VerdictAbstain, "ac-1"),
			},
			want: StatusPending,
		},
		{
			name:    "no-signal",
			ac:      ac("ac-1", artifact.EvidenceBehavioral),
			records: nil,
			want:    StatusNoSignal,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := foldOneAC(t, root, c.ac, c.records)
			if got.Status != c.want {
				t.Fatalf("status = %q, want %q", got.Status, c.want)
			}
		})
	}
}

// TestFold_PrecedencePairs proves each adjacent pair in 03's total
// precedence order (waived > violated > evidenced > pending > no-signal):
// the higher-precedence condition wins even when a lower one's condition
// also holds.
func TestFold_PrecedencePairs(t *testing.T) {
	root := t.TempDir()
	writeWaiver(t, root, "test-1", "ac-1", testActiveWaiver)

	t.Run("waived beats violated", func(t *testing.T) {
		got := foldOneAC(t, root, ac("ac-1", artifact.EvidenceStatic), []artifact.Evidence{
			testEvidence(artifact.EvidenceStatic, artifact.VerdictFail, "ac-1"),
		})
		if got.Status != StatusWaived {
			t.Fatalf("status = %q, want waived (a fail record must not override an active waiver)", got.Status)
		}
	})

	t.Run("violated beats evidenced", func(t *testing.T) {
		got := foldOneAC(t, root, ac("ac-99", artifact.EvidenceStatic, artifact.EvidenceBehavioral), []artifact.Evidence{
			testEvidence(artifact.EvidenceStatic, artifact.VerdictPass, "ac-99"),
			testEvidence(artifact.EvidenceBehavioral, artifact.VerdictFail, "ac-99"),
		})
		if got.Status != StatusViolated {
			t.Fatalf("status = %q, want violated (one failing kind outweighs another kind's pass)", got.Status)
		}
	})

	t.Run("evidenced beats pending", func(t *testing.T) {
		// Both kinds satisfied: must be evidenced, not merely pending
		// because a fully-satisfied AC "also" has records.
		got := foldOneAC(t, root, ac("ac-99", artifact.EvidenceStatic, artifact.EvidenceBehavioral), []artifact.Evidence{
			testEvidence(artifact.EvidenceStatic, artifact.VerdictPass, "ac-99"),
			testEvidence(artifact.EvidenceBehavioral, artifact.VerdictPass, "ac-99"),
		})
		if got.Status != StatusEvidenced {
			t.Fatalf("status = %q, want evidenced (every expected kind has a pass)", got.Status)
		}
	})

	t.Run("pending beats no-signal", func(t *testing.T) {
		// static has a record (abstain — signal, not satisfied); behavioral
		// has none at all. Overall must be pending, not no-signal, because
		// at least one expected kind has records.
		got := foldOneAC(t, root, ac("ac-99", artifact.EvidenceStatic, artifact.EvidenceBehavioral), []artifact.Evidence{
			testEvidence(artifact.EvidenceStatic, artifact.VerdictAbstain, "ac-99"),
		})
		if got.Status != StatusPending {
			t.Fatalf("status = %q, want pending (static has signal even though behavioral has none)", got.Status)
		}
	})
}

// TestFold_Flake proves the flake case end to end through Fold (not just
// Current in isolation): a same-commit retry that passes after an earlier
// failing job resolves the AC to evidenced, not violated.
func TestFold_Flake(t *testing.T) {
	root := t.TempDir()
	records := []artifact.Evidence{
		testEvidence(artifact.EvidenceStatic, artifact.VerdictFail, "ac-1",
			withProducer("retryWorker"), withPipeline("913"), withJob("1"), withCommit("7f3c2a1")),
		testEvidence(artifact.EvidenceStatic, artifact.VerdictPass, "ac-1",
			withProducer("retryWorker"), withPipeline("913"), withJob("2"), withCommit("7f3c2a1")),
	}
	got := foldOneAC(t, root, ac("ac-1", artifact.EvidenceStatic), records)
	if got.Status != StatusEvidenced {
		t.Fatalf("status = %q, want evidenced (the later, passing retry must win)", got.Status)
	}
}

// TestFold_RuntimeAlwaysPendingPostMerge proves a declared runtime kind
// with zero records is pending, never no-signal (OQ-2: no v0 runtime
// producer, but the AC is still "awaited post-merge").
func TestFold_RuntimeAlwaysPendingPostMerge(t *testing.T) {
	root := t.TempDir()
	got := foldOneAC(t, root, ac("ac-1", artifact.EvidenceRuntime), nil)
	if got.Status != StatusPending {
		t.Fatalf("status = %q, want pending (runtime is always awaited pre-close, OQ-2)", got.Status)
	}
}

// TestFold_NoSignalForUndeclaredKind proves an AC whose declared kind has
// no records at all (and is not runtime) reads as no-signal, distinct
// from pending.
func TestFold_NoSignalForUndeclaredKind(t *testing.T) {
	root := t.TempDir()
	got := foldOneAC(t, root, ac("ac-1", artifact.EvidenceBehavioral), nil)
	if got.Status != StatusNoSignal {
		t.Fatalf("status = %q, want no-signal", got.Status)
	}
}

// TestFold_AttestationByExistence proves the attestation kind is
// satisfied purely by file existence, both directions.
func TestFold_AttestationByExistence(t *testing.T) {
	t.Run("present", func(t *testing.T) {
		root := t.TempDir()
		writeAttestation(t, root, "test-1", "ac-1", testAttestation)
		got := foldOneAC(t, root, ac("ac-1", artifact.EvidenceAttestation), nil)
		if got.Status != StatusEvidenced {
			t.Fatalf("status = %q, want evidenced (attestation file exists)", got.Status)
		}
		if !strings.Contains(got.Summary, "attestation:present") {
			t.Fatalf("Summary = %q, want it to mention attestation:present", got.Summary)
		}
	})

	t.Run("absent", func(t *testing.T) {
		root := t.TempDir()
		got := foldOneAC(t, root, ac("ac-1", artifact.EvidenceAttestation), nil)
		if got.Status != StatusNoSignal {
			t.Fatalf("status = %q, want no-signal (no attestation file, no other signal)", got.Status)
		}
		if !strings.Contains(got.Summary, "attestation:absent") {
			t.Fatalf("Summary = %q, want it to mention attestation:absent", got.Summary)
		}
	})
}

// TestFold_AttestationUnauthored_CollapsesToNoSignal proves spec/
// attest-helper dc-3 at this package's own story-fold call site
// (fold.go): an unauthored `verdi attest` scaffold is NOT foldable
// (parent spec/closure-ergonomics dc-2) — it folds exactly as absence
// would, same status, same "attestation:absent" summary wording (dc-3:
// "this story does not itself change what any of those three callers
// RENDER") — until the operator removes the marker and authors their
// claim.
func TestFold_AttestationUnauthored_CollapsesToNoSignal(t *testing.T) {
	root := t.TempDir()
	writeAttestation(t, root, "test-1", "ac-1", unauthoredScaffoldFixture)
	got := foldOneAC(t, root, ac("ac-1", artifact.EvidenceAttestation), nil)
	if got.Status != StatusNoSignal {
		t.Fatalf("status = %q, want no-signal (an unauthored scaffold is not yet evidence, dc-3)", got.Status)
	}
	if !strings.Contains(got.Summary, "attestation:absent") {
		t.Fatalf("Summary = %q, want it to mention attestation:absent (dc-3: unauthored renders identically to absent)", got.Summary)
	}
}

// TestFold_ExpiredWaiverDoesNotWaive is the fold-level complement to
// TestWaiverActive_Expired: an expired waiver's AC is folded as if no
// waiver existed at all.
func TestFold_ExpiredWaiverDoesNotWaive(t *testing.T) {
	root := t.TempDir()
	writeWaiver(t, root, "test-1", "ac-3", testExpiredWaiver)

	got := foldOneAC(t, root, ac("ac-3", artifact.EvidenceBehavioral), nil)
	if got.Status != StatusNoSignal {
		t.Fatalf("status = %q, want no-signal (expired waiver must not waive, and there is no other signal)", got.Status)
	}
}

// TestFold_DanglingBinding proves a record whose evidence_for names an AC
// the spec does not declare is a loud error, never a silently-dropped
// record (03 §Declarations).
func TestFold_DanglingBinding(t *testing.T) {
	root := t.TempDir()
	spec := testSpec("jira:TEST-1", ac("ac-1", artifact.EvidenceStatic))
	_, err := Fold(Input{
		Spec: spec,
		Records: []artifact.Evidence{
			testEvidence(artifact.EvidenceStatic, artifact.VerdictPass, "ac-99"),
		},
		StoreRoot: root,
		StorySlug: "test-1",
	})
	if err == nil {
		t.Fatal("Fold with a dangling binding: want error, got nil")
	}
	if !strings.Contains(err.Error(), "ac-99") {
		t.Errorf("error = %v, want it to name the dangling ac-99", err)
	}
}

// TestFold_Negative covers Fold's other operational-error paths: a nil
// spec and a spec with no acceptance criteria.
func TestFold_Negative(t *testing.T) {
	root := t.TempDir()

	t.Run("nil spec", func(t *testing.T) {
		if _, err := Fold(Input{StoreRoot: root}); err == nil {
			t.Fatal("Fold(nil Spec): want error, got nil")
		}
	})

	t.Run("no acceptance criteria", func(t *testing.T) {
		spec := testSpec("jira:TEST-1")
		if _, err := Fold(Input{Spec: spec, StoreRoot: root}); err == nil {
			t.Fatal("Fold(spec with no ACs): want error, got nil")
		}
	})
}

// TestFold_StoryEligibleAndViolated proves story.eligible and
// story.violated aggregate correctly across a mix of AC statuses.
func TestFold_StoryEligibleAndViolated(t *testing.T) {
	root := t.TempDir()
	writeWaiver(t, root, "test-1", "ac-4", testActiveWaiver)

	spec := testSpec("jira:TEST-1",
		ac("ac-1", artifact.EvidenceStatic),     // evidenced
		ac("ac-2", artifact.EvidenceBehavioral), // pending
		ac("ac-4", artifact.EvidenceRuntime),    // waived
	)
	records := []artifact.Evidence{
		testEvidence(artifact.EvidenceStatic, artifact.VerdictPass, "ac-1"),
		testEvidence(artifact.EvidenceBehavioral, artifact.VerdictAbstain, "ac-2"),
	}

	result, err := Fold(Input{Spec: spec, Records: records, StoreRoot: root, StorySlug: "test-1"})
	if err != nil {
		t.Fatalf("Fold: %v", err)
	}
	if result.Violated {
		t.Fatal("story.Violated = true, want false (no AC is violated)")
	}
	if result.Eligible {
		t.Fatal("story.Eligible = true, want false (ac-2 is pending, not evidenced/waived)")
	}

	t.Run("eligible once pending resolves to evidenced", func(t *testing.T) {
		// A higher pipeline id so this record supersedes the earlier
		// abstain under (kind, witness) grouping — same default witness,
		// so ordering alone must pick the later one.
		withPending := append([]artifact.Evidence(nil), records...)
		withPending = append(withPending, testEvidence(artifact.EvidenceBehavioral, artifact.VerdictPass, "ac-2", withPipeline("2")))
		result, err := Fold(Input{Spec: spec, Records: withPending, StoreRoot: root, StorySlug: "test-1"})
		if err != nil {
			t.Fatalf("Fold: %v", err)
		}
		if !result.Eligible {
			t.Fatalf("story.Eligible = false, want true once every AC is evidenced/waived; ACs=%+v", result.ACs)
		}
	})

	t.Run("violated once any AC fails", func(t *testing.T) {
		withFail := append([]artifact.Evidence(nil), records...)
		withFail = append(withFail, testEvidence(artifact.EvidenceStatic, artifact.VerdictFail, "ac-1", withPipeline("2")))
		result, err := Fold(Input{Spec: spec, Records: withFail, StoreRoot: root, StorySlug: "test-1"})
		if err != nil {
			t.Fatalf("Fold: %v", err)
		}
		if !result.Violated {
			t.Fatal("story.Violated = false, want true (ac-1 now has a fail record)")
		}
		if result.Eligible {
			t.Fatal("story.Eligible = true, want false (a violated story is never eligible)")
		}
	})
}

// TestFold_PreviewIncludesAdvisoryRecords proves --preview's fold differs
// exactly by including source:local records, which the authoritative-only
// fold must ignore.
func TestFold_PreviewIncludesAdvisoryRecords(t *testing.T) {
	root := t.TempDir()
	spec := testSpec("jira:TEST-1", ac("ac-1", artifact.EvidenceBehavioral))
	records := []artifact.Evidence{
		testEvidence(artifact.EvidenceBehavioral, artifact.VerdictAbstain, "ac-1", withSource(artifact.SourceLocal)),
	}

	authoritative, err := Fold(Input{Spec: spec, Records: records, StoreRoot: root, StorySlug: "test-1"})
	if err != nil {
		t.Fatalf("Fold (authoritative): %v", err)
	}
	if authoritative.ACs[0].Status != StatusNoSignal {
		t.Fatalf("authoritative status = %q, want no-signal (advisory record must be excluded)", authoritative.ACs[0].Status)
	}

	preview, err := Fold(Input{Spec: spec, Records: records, Preview: true, StoreRoot: root, StorySlug: "test-1"})
	if err != nil {
		t.Fatalf("Fold (preview): %v", err)
	}
	if preview.ACs[0].Status != StatusPending {
		t.Fatalf("preview status = %q, want pending (advisory abstain record is now in scope)", preview.ACs[0].Status)
	}
}

// TestRank_Happy proves Rank reflects 03's total precedence order.
func TestRank_Happy(t *testing.T) {
	order := []Status{StatusWaived, StatusViolated, StatusEvidenced, StatusPending, StatusNoSignal}
	for i := 0; i < len(order)-1; i++ {
		if Rank(order[i]) >= Rank(order[i+1]) {
			t.Fatalf("Rank(%s)=%d must be < Rank(%s)=%d", order[i], Rank(order[i]), order[i+1], Rank(order[i+1]))
		}
	}
}

// TestRank_Negative proves an unknown status ranks as -1, never
// silently equal to a real status.
func TestRank_Negative(t *testing.T) {
	if got := Rank(Status("bogus")); got != -1 {
		t.Fatalf("Rank(bogus) = %d, want -1", got)
	}
}

// TestFold_KindsBreakdown proves ACResult.Kinds projects the fold's OWN
// per-declared-kind evaluation over the AUTHORITATIVE candidate set — the
// single source spec/close-preflight's disclosure renders from (ADJ-56). It
// is a projection of the same fold that produced Status, so a source:local
// pass discounted by the verdict reads as unsatisfied here too, a violated
// kind carries its failing witness, and a violated AC still exposes its full
// breakdown (the pre-fix foldAC early-returned before computing any of it).
func TestFold_KindsBreakdown(t *testing.T) {
	root := t.TempDir()

	t.Run("source:local pass does not satisfy the authoritative fold (finding 1)", func(t *testing.T) {
		res := foldOneAC(t, root, ac("ac-1", artifact.EvidenceBehavioral), []artifact.Evidence{
			testEvidence(artifact.EvidenceBehavioral, artifact.VerdictPass, "ac-1", withSource(artifact.SourceLocal)),
		})
		if len(res.Kinds) != 1 || res.Kinds[0].Kind != artifact.EvidenceBehavioral {
			t.Fatalf("Kinds = %+v, want one behavioral entry", res.Kinds)
		}
		if res.Kinds[0].Satisfied {
			t.Fatal("behavioral Satisfied = true over a source:local-only pass; the authoritative fold must discount it")
		}
		if res.Kinds[0].Violating != nil {
			t.Fatalf("Violating = %+v, want nil (a discounted pass is not a violation)", res.Kinds[0].Violating)
		}
	})

	t.Run("source:ci pass satisfies", func(t *testing.T) {
		res := foldOneAC(t, root, ac("ac-1", artifact.EvidenceBehavioral), []artifact.Evidence{
			testEvidence(artifact.EvidenceBehavioral, artifact.VerdictPass, "ac-1"),
		})
		if !res.Kinds[0].Satisfied || res.Kinds[0].Violating != nil {
			t.Fatalf("behavioral KindResult = %+v, want satisfied, no violation", res.Kinds[0])
		}
	})

	t.Run("violated AC still exposes Kinds with the violating witness (finding 3)", func(t *testing.T) {
		res := foldOneAC(t, root, ac("ac-1", artifact.EvidenceStatic, artifact.EvidenceBehavioral), []artifact.Evidence{
			testEvidence(artifact.EvidenceStatic, artifact.VerdictFail, "ac-1", withWitness("bad-static"), withProducer("linter")),
			testEvidence(artifact.EvidenceBehavioral, artifact.VerdictPass, "ac-1", withProducer("e2e")),
		})
		if res.Status != StatusViolated {
			t.Fatalf("Status = %q, want violated", res.Status)
		}
		if len(res.Kinds) != 2 {
			t.Fatalf("Kinds = %+v, want 2 (a violated AC must still expose its per-kind breakdown)", res.Kinds)
		}
		if s := res.Kinds[0]; s.Kind != artifact.EvidenceStatic || s.Violating == nil || s.Violating.Witness != "bad-static" {
			t.Fatalf("static KindResult = %+v, want Violating witness \"bad-static\"", s)
		}
		if b := res.Kinds[1]; !b.Satisfied || b.Violating != nil {
			t.Fatalf("behavioral KindResult = %+v, want satisfied, no violation", b)
		}
	})

	t.Run("coexisting pass+fail of the same kind still names the violation (finding 3)", func(t *testing.T) {
		res := foldOneAC(t, root, ac("ac-1", artifact.EvidenceStatic), []artifact.Evidence{
			testEvidence(artifact.EvidenceStatic, artifact.VerdictFail, "ac-1", withProducer("linter-a"), withWitness("failing")),
			testEvidence(artifact.EvidenceStatic, artifact.VerdictPass, "ac-1", withProducer("linter-b")),
		})
		if res.Status != StatusViolated {
			t.Fatalf("Status = %q, want violated (a coexisting pass never clears a fail)", res.Status)
		}
		if res.Kinds[0].Violating == nil || res.Kinds[0].Violating.Witness != "failing" {
			t.Fatalf("static KindResult = %+v, want the failing record named even though a pass coexists", res.Kinds[0])
		}
	})

	t.Run("attestation states absent/unauthored/authored", func(t *testing.T) {
		absent := foldOneAC(t, root, ac("ac-absent", artifact.EvidenceAttestation), nil)
		if absent.Kinds[0].Attestation != AttestationAbsent || absent.Kinds[0].Satisfied {
			t.Fatalf("absent KindResult = %+v, want Attestation=absent, Satisfied=false", absent.Kinds[0])
		}
		writeAttestation(t, root, "test-1", "ac-unauth", "body\n"+UnauthoredAttestationMarker+"\nmore\n")
		unauth := foldOneAC(t, root, ac("ac-unauth", artifact.EvidenceAttestation), nil)
		if unauth.Kinds[0].Attestation != AttestationUnauthored || unauth.Kinds[0].Satisfied {
			t.Fatalf("unauthored KindResult = %+v, want Attestation=unauthored, Satisfied=false", unauth.Kinds[0])
		}
		writeAttestation(t, root, "test-1", "ac-auth", "an authored claim, no marker\n")
		auth := foldOneAC(t, root, ac("ac-auth", artifact.EvidenceAttestation), nil)
		if auth.Kinds[0].Attestation != AttestationAuthored || !auth.Kinds[0].Satisfied {
			t.Fatalf("authored KindResult = %+v, want Attestation=authored, Satisfied=true", auth.Kinds[0])
		}
	})
}
