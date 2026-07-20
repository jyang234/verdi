package main

// This file covers spec/close-preflight's feature-scope obligations (dc-3:
// --preflight covers both story and feature scope, ADJ-33-ratified): one
// subtest per feature-specific defect class named in the ac-1--behavioral
// obligation (a feature AC not evidenced — including the outcome floor at
// the FeatureSlug path, dc-6 — an unreconciled stub, and an implementing
// story still open), plus the ready-then-close pair. Reuses
// closefeature_test.go's own fixture family (buildCloseFeatureRepo,
// defaultCloseFeatureFixtureOpts, seedCloseFeatureEvidence, closeFeatureDeps)
// directly — same package, same fixture shapes the feature closure gate's
// own tests already exercise, so the preflight and the real gate are
// proven against IDENTICAL fixtures throughout.

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	forgefake "github.com/jyang234/verdi/internal/forge/fake"
	"github.com/jyang234/verdi/internal/provider/fake"
	"github.com/jyang234/verdi/internal/store"
)

// TestRunPreflight_FeatureScope_DefectClasses is the feature-scope
// counterpart of TestRunPreflight_StoryScope_DefectClasses: each subtest
// builds a fixture with exactly one feature-closure-gate defect, runs
// --preflight (asserting the exact condition/kind/path disclosure), then a
// real, unmodified verdi close on the byte-identical fixture (asserting its
// refusal reason matches).
func TestRunPreflight_FeatureScope_DefectClasses(t *testing.T) {
	ctx := context.Background()

	t.Run("feature AC not evidenced: outcome floor unmet", func(t *testing.T) {
		opts := defaultCloseFeatureFixtureOpts()
		opts.FeatureAC2FloorSatisfied = false
		repo := buildCloseFeatureRepo(t, opts)
		seedCloseFeatureEvidence(t, repo.Dir, repo.Head, opts)
		before := snapshotRepo(t, repo.Dir)

		var pstdout, pstderr bytes.Buffer
		rc := runPreflight(ctx, repo.Dir, "spec/close-feature-fixture", &store.Manifest{}, nil, forgefake.New(), true, &pstdout, &pstderr)
		if rc != 1 {
			t.Fatalf("runPreflight = %d, want 1; stdout=%s stderr=%s", rc, pstdout.String(), pstderr.String())
		}
		if !strings.Contains(pstdout.String(), "[FAIL] closure(feature): 1. every feature AC evidenced") {
			t.Fatalf("preflight stdout missing feature-AC-evidenced FAIL:\n%s", pstdout.String())
		}
		if !strings.Contains(pstdout.String(), "ac-2=pending") {
			t.Fatalf("preflight stdout missing the per-AC status the gate already itemizes:\n%s", pstdout.String())
		}

		// ADJ-56 finding 2: the feature outcome floor is an OR across an AC's
		// signals (attested OR any passing outcome record — 03 §The feature
		// fold), NOT the story fold's AND-across-declared-kinds. An unsatisfied
		// floor renders ONE disjunctive remedy naming both ways to clear it
		// (the FeatureSlug-keyed attestation path, dc-6, OR a passing outcome
		// record), never a separate per-kind attestation/behavioral remedy.
		derivedRoot := filepath.ToSlash(filepath.Join(".verdi", "data", "derived", store.RefSlug("spec/close-feature-fixture"))) + "/"
		featureSlugPath := filepath.ToSlash(filepath.Join(".verdi", "attestations", "close-feature-fixture", "ac-2.md"))
		wantFloor := "ac-2 outcome floor unsatisfied: needs an authored outcome attestation at " + featureSlugPath + ", or any passing outcome record under " + derivedRoot
		if !strings.Contains(pstdout.String(), wantFloor) {
			t.Fatalf("preflight stdout missing the OR-floor outcome-floor disclosure %q:\n%s", wantFloor, pstdout.String())
		}
		if strings.Contains(pstdout.String(), "ac-2 attestation: no file") || strings.Contains(pstdout.String(), "ac-2 behavioral: no current passing record") {
			t.Fatalf("preflight stdout must not render the feature floor as story-style AND-across-kinds remedies (finding 2):\n%s", pstdout.String())
		}
		if strings.Contains(pstdout.String(), "ac-1 behavioral:") || strings.Contains(pstdout.String(), "ac-1 outcome floor") {
			t.Fatalf("preflight stdout should not name ac-1 (only ac-2 is unmet):\n%s", pstdout.String())
		}

		after := snapshotRepo(t, repo.Dir)
		if before != after {
			t.Fatalf("--preflight mutated the repo:\nbefore: %s\nafter:  %s", before, after)
		}

		var cstdout, cstderr bytes.Buffer
		gotClose := runClose(ctx, repo.Dir, "spec/close-feature-fixture", &store.Manifest{}, closeFeatureDeps(fake.New()), &cstdout, &cstderr)
		if gotClose != 1 {
			t.Fatalf("runClose = %d, want 1; stdout=%s stderr=%s", gotClose, cstdout.String(), cstderr.String())
		}
		if !strings.Contains(cstdout.String(), "ac-2=pending") {
			t.Fatalf("close stdout missing the SAME per-AC status preflight showed: %s", cstdout.String())
		}
	})

	t.Run("unreconciled stub and implementing story still open", func(t *testing.T) {
		opts := defaultCloseFeatureFixtureOpts()
		opts.Story2Status = "accepted-pending-build"
		repo := buildCloseFeatureRepo(t, opts)
		seedCloseFeatureEvidence(t, repo.Dir, repo.Head, opts)
		before := snapshotRepo(t, repo.Dir)

		var pstdout, pstderr bytes.Buffer
		rc := runPreflight(ctx, repo.Dir, "spec/close-feature-fixture", &store.Manifest{}, nil, forgefake.New(), true, &pstdout, &pstderr)
		if rc != 1 {
			t.Fatalf("runPreflight = %d, want 1; stdout=%s stderr=%s", rc, pstdout.String(), pstderr.String())
		}
		if !strings.Contains(pstdout.String(), "[FAIL] closure(feature): 2. stub reconciliation not blocked") {
			t.Fatalf("preflight stdout missing stub-reconciliation FAIL:\n%s", pstdout.String())
		}
		if !strings.Contains(pstdout.String(), "unreconciled stub(s): [fixture-story-two]") {
			t.Fatalf("preflight stdout missing the unreconciled stub slug:\n%s", pstdout.String())
		}
		if !strings.Contains(pstdout.String(), "[FAIL] closure(feature): 3. every implementing story closed") {
			t.Fatalf("preflight stdout missing implementing-stories FAIL:\n%s", pstdout.String())
		}
		if !strings.Contains(pstdout.String(), "not yet closed") || !strings.Contains(pstdout.String(), "fixture-story-two") {
			t.Fatalf("preflight stdout missing the still-open story ref:\n%s", pstdout.String())
		}

		after := snapshotRepo(t, repo.Dir)
		if before != after {
			t.Fatalf("--preflight mutated the repo:\nbefore: %s\nafter:  %s", before, after)
		}

		var cstdout, cstderr bytes.Buffer
		gotClose := runClose(ctx, repo.Dir, "spec/close-feature-fixture", &store.Manifest{}, closeFeatureDeps(fake.New()), &cstdout, &cstderr)
		if gotClose != 1 {
			t.Fatalf("runClose = %d, want 1; stdout=%s stderr=%s", gotClose, cstdout.String(), cstderr.String())
		}
		if !strings.Contains(cstdout.String(), "unreconciled stub(s): [fixture-story-two]") {
			t.Fatalf("close stdout missing the SAME unreconciled-stub reason preflight showed: %s", cstdout.String())
		}
		if !strings.Contains(cstdout.String(), "not yet closed") || !strings.Contains(cstdout.String(), "fixture-story-two") {
			t.Fatalf("close stdout missing the SAME still-open story ref preflight showed: %s", cstdout.String())
		}
	})

	// ADJ-56 finding 2 (0.55): a feature AC whose outcome floor is SATISFIED
	// via one disjunct (a passing behavioral outcome record) but which is
	// still unmet only because an implementing story is open+ineligible
	// (pending) must NOT print an attestation remedy — the feature gate does
	// not require an attestation once the floor is cleared, so instructing the
	// operator to author one is a false remedy from re-derived (story-AND)
	// requirement semantics.
	t.Run("floor satisfied via one disjunct prints no false remedy (finding 2)", func(t *testing.T) {
		opts := defaultCloseFeatureFixtureOpts()
		opts.Story2Status = "accepted-pending-build" // story-two open
		opts.Story2OwnVerdict = "abstain"            // ...and NOT self-eligible, so ac-2 folds pending, not evidenced
		opts.FeatureAC2FloorSatisfied = true         // floor cleared by a passing behavioral outcome record
		repo := buildCloseFeatureRepo(t, opts)
		seedCloseFeatureEvidence(t, repo.Dir, repo.Head, opts)
		before := snapshotRepo(t, repo.Dir)

		var pstdout, pstderr bytes.Buffer
		rc := runPreflight(ctx, repo.Dir, "spec/close-feature-fixture", &store.Manifest{}, nil, forgefake.New(), true, &pstdout, &pstderr)
		if rc != 1 {
			t.Fatalf("runPreflight = %d, want 1; stdout=%s stderr=%s", rc, pstdout.String(), pstderr.String())
		}
		// ac-2 IS unmet (pending — its open implementing story), and the gate
		// itemizes that; but its outcome floor is already satisfied.
		if !strings.Contains(pstdout.String(), "ac-2=pending") {
			t.Fatalf("preflight stdout should still itemize ac-2 as pending:\n%s", pstdout.String())
		}
		if strings.Contains(pstdout.String(), "close-feature-fixture/ac-2.md") {
			t.Fatalf("finding 2: preflight printed a FALSE attestation remedy for an already-satisfied outcome floor:\n%s", pstdout.String())
		}
		if strings.Contains(pstdout.String(), "ac-2 attestation:") || strings.Contains(pstdout.String(), "ac-2 outcome floor") {
			t.Fatalf("finding 2: preflight printed a floor remedy for a satisfied floor:\n%s", pstdout.String())
		}

		after := snapshotRepo(t, repo.Dir)
		if before != after {
			t.Fatalf("--preflight mutated the repo:\nbefore: %s\nafter:  %s", before, after)
		}

		// Agreement (ac-3): a real close on the byte-identical fixture refuses
		// for exactly the reasons the preflight named (ac-2 pending + the open
		// implementing story), never on the fabricated attestation.
		var cstdout, cstderr bytes.Buffer
		gotClose := runClose(ctx, repo.Dir, "spec/close-feature-fixture", &store.Manifest{}, closeFeatureDeps(fake.New()), &cstdout, &cstderr)
		if gotClose != 1 {
			t.Fatalf("runClose = %d, want 1; stdout=%s stderr=%s", gotClose, cstdout.String(), cstderr.String())
		}
		if !strings.Contains(cstdout.String(), "ac-2=pending") {
			t.Fatalf("close stdout missing the SAME per-AC status preflight showed: %s", cstdout.String())
		}
	})

	// dc-4's "found but excluded as non-ancestor" stale rendering on the
	// FEATURE side: an outcome record present only at a commit head does not
	// descend from is excluded by the feature fold, but --preflight discloses
	// it was found-and-excluded (naming the sha) on the outcome-floor line. This
	// is the only test that drives renderFeatureFloorGap's excluded-commit
	// branch (closepreflightfeature.go:118-120), doubly unfired before ADJ-72
	// (th-4's feature-side extension).
	t.Run("outcome record only on a non-ancestor sibling commit reads as found-but-excluded", func(t *testing.T) {
		opts := defaultCloseFeatureFixtureOpts()
		opts.FeatureAC2FloorSatisfied = false
		repo := buildCloseFeatureRepo(t, opts)
		// A real fork off the scaffold parent (repo.Heads[0]) — a genuine
		// non-ancestor of head, exactly as internal/evidence's own
		// ExcludedCommitDirs fixtures build it. Created before any derived
		// records are seeded, and staging only its own file, so the head/sibling
		// derived trees below are left untouched.
		sibling := preflightSiblingCommit(t, repo.Dir, repo.Heads[0])
		seedCloseFeatureEvidence(t, repo.Dir, repo.Head, opts)
		// ac-2's ONLY outcome record lives on the excluded sibling; the fold
		// discounts it, so the floor stays unsatisfied AND the disclosure names
		// the excluded sha rather than reporting the record simply absent.
		writeFixtureVerdicts(t, repo.Dir, "spec/close-feature-fixture", sibling,
			featureFixtureEvidenceJSON("ac-2", "behavioral", "pass", sibling))
		before := snapshotRepo(t, repo.Dir)

		var pstdout, pstderr bytes.Buffer
		rc := runPreflight(ctx, repo.Dir, "spec/close-feature-fixture", &store.Manifest{}, nil, forgefake.New(), true, &pstdout, &pstderr)
		if rc != 1 {
			t.Fatalf("runPreflight = %d, want 1; stdout=%s stderr=%s", rc, pstdout.String(), pstderr.String())
		}
		derivedRoot := filepath.ToSlash(filepath.Join(".verdi", "data", "derived", store.RefSlug("spec/close-feature-fixture"))) + "/"
		featureSlugPath := filepath.ToSlash(filepath.Join(".verdi", "attestations", "close-feature-fixture", "ac-2.md"))
		// The exact line, INCLUDING the excluded-sha suffix: the prefix alone
		// would pass against a deleted excluded-commit branch, so the full-line
		// assertion is what makes this a genuine witness for that branch (th-4).
		wantExcluded := "ac-2 outcome floor unsatisfied: needs an authored outcome attestation at " + featureSlugPath +
			", or any passing outcome record under " + derivedRoot +
			" (found but excluded as non-ancestor: [" + sibling + "])"
		if !strings.Contains(pstdout.String(), wantExcluded) {
			t.Fatalf("preflight stdout missing the feature found-but-excluded disclosure %q:\n%s", wantExcluded, pstdout.String())
		}

		after := snapshotRepo(t, repo.Dir)
		if before != after {
			t.Fatalf("--preflight mutated the repo:\nbefore: %s\nafter:  %s", before, after)
		}
	})
}

// TestRunPreflight_FeatureScope_OutcomeFloorAttestation_UsesFeatureSlug is
// dc-6's dedicated proof: the outcome-floor attestation's disclosed path
// keys by the feature spec's own Name (FeatureSlug), never
// store.RefSlug(spec.Story) — the "single easiest correctness mistake"
// dc-6 names explicitly. This fixture's feature carries no story: ref at
// all (defaultCloseFeatureFixtureOpts' own choice, mirroring
// spec/true-closure's real shape), so store.RefSlug("") would silently
// collapse the story-slug path to a bare, wrong ".verdi/attestations/ac-2.md"
// — a maximally visible witness if the build ever used the wrong helper.
//
// The floor stays unmet by withholding BOTH signals (no behavioral record,
// no attestation), so the OR-floor disclosure (ADJ-56 finding 2) names both
// ways to clear it — including the FeatureSlug-keyed attestation path this
// test polices. That the floor is a disjunction (a single passing behavioral
// record alone clears it) is now proven separately by
// TestRunPreflight_FeatureScope_DefectClasses' finding-2 subtest; here the
// floor is genuinely unsatisfied so the attestation path is legitimately named.
func TestRunPreflight_FeatureScope_OutcomeFloorAttestation_UsesFeatureSlug(t *testing.T) {
	opts := defaultCloseFeatureFixtureOpts()
	opts.FeatureAC2FloorSatisfied = false // neither a behavioral record nor an attestation exists for ac-2
	repo := buildCloseFeatureRepo(t, opts)
	seedCloseFeatureEvidence(t, repo.Dir, repo.Head, opts)
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	rc := runPreflight(ctx, repo.Dir, "spec/close-feature-fixture", &store.Manifest{}, nil, forgefake.New(), true, &stdout, &stderr)
	if rc != 1 {
		t.Fatalf("runPreflight = %d, want 1; stdout=%s stderr=%s", rc, stdout.String(), stderr.String())
	}

	derivedRoot := filepath.ToSlash(filepath.Join(".verdi", "data", "derived", store.RefSlug("spec/close-feature-fixture"))) + "/"
	featureSlugPath := filepath.ToSlash(filepath.Join(".verdi", "attestations", "close-feature-fixture", "ac-2.md"))
	want := "ac-2 outcome floor unsatisfied: needs an authored outcome attestation at " + featureSlugPath + ", or any passing outcome record under " + derivedRoot
	if !strings.Contains(stdout.String(), want) {
		t.Fatalf("stdout missing the FeatureSlug-keyed OR-floor disclosure %q:\n%s", want, stdout.String())
	}

	wrongStorySlugPath := filepath.ToSlash(filepath.Join(".verdi", "attestations", "ac-2.md")) // store.RefSlug("") collapses away entirely
	if strings.Contains(stdout.String(), "attestation at "+wrongStorySlugPath) {
		t.Fatalf("stdout must never use the story-slug helper (store.RefSlug(spec.Story)) for the feature outcome floor: %s", stdout.String())
	}

	var cstdout, cstderr bytes.Buffer
	gotClose := runClose(ctx, repo.Dir, "spec/close-feature-fixture", &store.Manifest{}, closeFeatureDeps(fake.New()), &cstdout, &cstderr)
	if gotClose != 1 {
		t.Fatalf("runClose = %d, want 1; stdout=%s stderr=%s", gotClose, cstdout.String(), cstderr.String())
	}
}

// TestRunPreflight_FeatureScope_ReadyThenClose is ac-3--behavioral's
// second half at the feature scope: a fully-satisfied feature fixture
// reports ready (exit 0), then a real, unmodified verdi close on the same
// fixture succeeds, actually archiving the feature quartet.
func TestRunPreflight_FeatureScope_ReadyThenClose(t *testing.T) {
	opts := defaultCloseFeatureFixtureOpts()
	repo := buildCloseFeatureRepo(t, opts)
	seedCloseFeatureEvidence(t, repo.Dir, repo.Head, opts)
	// The corrected closure ritual (X-16): align (a living report covering
	// head) -> disposition (working-tree edit) -> close (X-13/X-16/X-17's
	// closure-gate condition 6) — without it "ready" would not actually be
	// ready.
	writeCloseFeatureGateReport(t, repo.Dir, repo.Head, dispositionedFindingYAML)
	ctx := context.Background()

	before := snapshotRepo(t, repo.Dir)
	var pstdout, pstderr bytes.Buffer
	rc := runPreflight(ctx, repo.Dir, "spec/close-feature-fixture", &store.Manifest{}, nil, forgefake.New(), true, &pstdout, &pstderr)
	if rc != 0 {
		t.Fatalf("runPreflight(feature, ready) = %d, want 0; stdout=%s stderr=%s", rc, pstdout.String(), pstderr.String())
	}
	if strings.Contains(pstdout.String(), "[FAIL]") {
		t.Fatalf("ready feature preflight should show no FAIL condition:\n%s", pstdout.String())
	}
	after := snapshotRepo(t, repo.Dir)
	if before != after {
		t.Fatalf("--preflight(feature, ready) mutated the repo:\nbefore: %s\nafter:  %s", before, after)
	}

	var cstdout, cstderr bytes.Buffer
	gotClose := runClose(ctx, repo.Dir, "spec/close-feature-fixture", &store.Manifest{}, closeFeatureDeps(fake.New()), &cstdout, &cstderr)
	if gotClose != 0 {
		t.Fatalf("runClose(feature, ready) = %d, want 0; stdout=%s stderr=%s", gotClose, cstdout.String(), cstderr.String())
	}
	if _, err := os.Stat(filepath.Join(repo.Dir, ".verdi", "specs", "archive", "close-feature-fixture", "spec.md")); err != nil {
		t.Fatalf("real close should have archived the feature quartet after a READY preflight: %v", err)
	}
}
