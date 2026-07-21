package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/forge"
	forgefake "github.com/jyang234/verdi/internal/forge/fake"
	"github.com/jyang234/verdi/internal/provider/fake"
	"github.com/jyang234/verdi/internal/store"
)

const closureGateStorySpecMD = `---
id: spec/stale-decline
kind: spec
class: story
title: "Stale decline story"
status: accepted-pending-build
owners: [platform-team]
story: jira:LOAN-1482
problem: { text: "x", anchor: problem }
outcome: { text: "y", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "static obligation holds", evidence: [attestation] }
links:
  - { type: implements, ref: "spec/loan-mgmt#ac-1" }
frozen: { at: 2024-01-01, commit: ` + gateFakeFrozenCommit + `}
---
# body
## Problem
x
## Outcome
y
`

// closureGateQuarantineStorySpecMD is a story spec whose sole AC declares
// [static] evidence (rather than closureGateStorySpecMD's [attestation]) —
// spec/evidence-resilience ac-2's own test needs a real derived-evidence-
// record AC, since an attestation-only AC never consults
// LoadRecords/QuarantinedRecords at all. No feature link: condition 3
// (pending-supersession) is then trivially satisfied ("no feature
// implemented"), keeping this fixture focused on condition 1 alone.
const closureGateQuarantineStorySpecMD = `---
id: spec/quarantine-story
kind: spec
class: story
title: "Quarantine story"
status: accepted-pending-build
owners: [platform-team]
story: jira:QUAR-1
problem: { text: "x", anchor: problem }
outcome: { text: "y", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "static obligation holds", evidence: [static] }
links:
  - { type: implements, ref: "spec/loan-mgmt#ac-1" }
frozen: { at: 2024-01-01, commit: ` + gateFakeFrozenCommit + `}
---
# body
## Problem
x
## Outcome
y
`

func buildClosureGateQuarantineRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/verdi.yaml":                            "schema: verdi.layout/v1\nforge: gitlab\n",
			".verdi/specs/active/quarantine-story/spec.md": closureGateQuarantineStorySpecMD,
		},
		Message: "closure gate quarantine fixture",
	}})
	checkoutBranch(t, repo.Dir, "feature/quarantine-story")
	return repo
}

// writeClosureGateDerivedRecord writes one verdi.evidence/v1 record array
// under root's derived tree for specID at the given commit-named
// subdirectory (which need not be a real commit at all — the exact shape
// a bundle referencing a deleted branch's tip produces).
func writeClosureGateDerivedRecord(t *testing.T, root, specID, commitDir, recordJSON string) {
	t.Helper()
	dir := filepath.Join(store.DerivedSpecDir(root, store.RefSlug(specID)), commitDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "verdicts.json"), []byte(recordJSON), 0o644); err != nil {
		t.Fatalf("writing verdicts.json: %v", err)
	}
}

// closureGateQuarantineRecordJSON is one static-pass record for
// spec/quarantine-story's ac-1, at provenance.commit commit.
func closureGateQuarantineRecordJSON(commit string) string {
	return `[{"schema":"verdi.evidence/v1","evidence_for":["ac-1"],"kind":"static","verdict":"pass",` +
		`"witness":"someFunc @ site","provenance":{"source":"ci","pipeline":"1","commit":"` + commit + `"},` +
		`"digest":"sha256:` + strings.Repeat("ab", 32) + `"}]`
}

// TestRunClosureGate_UnreachableCommitRecord_NeverOperational is
// spec/evidence-resilience ac-2's core regression pin, X-15's exact shape
// reproduced directly against the closure gate: a derived verdicts.json
// sits under a commit-named directory that resolves to no real commit at
// all (the branch that produced it has since been deleted) — the literal
// shape that used to hard-fail runClosureGate operationally (git's own
// "fatal: Not a valid commit name", surfaced as a returned error). It must
// now evaluate cleanly (no error), read the story as NOT eligible (the
// excluded record never silently counts as evidence), and disclose WHY —
// a per-AC disclosed-unproven line naming the excluded record — rather
// than leaving the gap looking like no evidence was ever produced.
func TestRunClosureGate_UnreachableCommitRecord_NeverOperational(t *testing.T) {
	repo := buildClosureGateQuarantineRepo(t)
	spec, _ := readSpec(t, repo.Dir, "quarantine-story")
	ctx := context.Background()
	const unreachable = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"

	writeClosureGateDerivedRecord(t, repo.Dir, spec.ID, unreachable, closureGateQuarantineRecordJSON(unreachable))

	var stdout bytes.Buffer
	ok, err := runClosureGate(ctx, repo.Dir, spec, nil, "main", nil, nil, repo.Head, &stdout)
	if err != nil {
		t.Fatalf("runClosureGate: want no error (X-15 must never brick this), got %v; stdout=%s", err, stdout.String())
	}
	if ok {
		t.Fatalf("runClosureGate() = true, want false (ac-1's only record is excluded, so it is not evidenced); stdout=%s", stdout.String())
	}
	if !contains(stdout.String(), "[FAIL] closure: 1.") {
		t.Fatalf("stdout = %q, want condition 1 to FAIL (never silently proven from an excluded record)", stdout.String())
	}
	if !contains(stdout.String(), "disclosed-unproven [gate:evidence-quarantine]") {
		t.Fatalf("stdout = %q, want a per-record disclosed-unproven line naming the excluded record (ac-2)", stdout.String())
	}
	if !contains(stdout.String(), "ac-1") {
		t.Fatalf("stdout = %q, want the disclosure to name ac-1, the AC the excluded record would have evidenced", stdout.String())
	}
	if !contains(stdout.String(), unreachable) {
		t.Fatalf("stdout = %q, want the disclosure to name the unreachable commit", stdout.String())
	}
}

// TestRunClosureGate_QuarantinedRecord_SurfacesSyncReason proves the
// disclosure prefers the ACTUAL reason `verdi sync` recorded
// (artifact.Evidence.Quarantine, ac-1) over a generic fallback, when the
// derived record already carries one — the realistic end state after a
// real sync run quarantined it.
func TestRunClosureGate_QuarantinedRecord_SurfacesSyncReason(t *testing.T) {
	repo := buildClosureGateQuarantineRepo(t)
	spec, _ := readSpec(t, repo.Dir, "quarantine-story")
	ctx := context.Background()
	const unreachable = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"

	recordJSON := `[{"schema":"verdi.evidence/v1","evidence_for":["ac-1"],"kind":"static","verdict":"pass",` +
		`"witness":"someFunc @ site","provenance":{"source":"ci","pipeline":"1","commit":"` + unreachable + `"},` +
		`"quarantine":{"reason":"custom sync-time reason naming the deleted branch"},` +
		`"digest":"sha256:` + strings.Repeat("ab", 32) + `"}]`
	writeClosureGateDerivedRecord(t, repo.Dir, spec.ID, unreachable, recordJSON)

	var stdout bytes.Buffer
	ok, err := runClosureGate(ctx, repo.Dir, spec, nil, "main", nil, nil, repo.Head, &stdout)
	if err != nil {
		t.Fatalf("runClosureGate: %v", err)
	}
	if ok {
		t.Fatalf("runClosureGate() = true, want false; stdout=%s", stdout.String())
	}
	if !contains(stdout.String(), "custom sync-time reason naming the deleted branch") {
		t.Fatalf("stdout = %q, want the sync-recorded quarantine reason surfaced verbatim", stdout.String())
	}
}

// TestRunClosureGate_AnnotatedRecordUnderReachableDir_ExcludedAndDisclosed is
// spec/evidence-resilience finding-1's behavioral pin: a record `verdi sync`
// ANNOTATED as quarantined that sits under a REACHABLE commit directory (its
// subdir key differs from its own provenance.commit — hand-placed derived
// data, or a fetched artifact keyed differently from the record's commit)
// must NOT silently count as authoritative evidence. Before the fix, the
// fold's exclusion rested entirely on directory reachability, so this record
// was loaded and silently marked ac-1 proven — the exact false green ac-2's
// honesty clause forbids. The gate must read condition 1 as FAILing (ac-1's
// sole record is excluded on the annotation signal) AND disclose the excluded
// record even though its directory is reachable.
func TestRunClosureGate_AnnotatedRecordUnderReachableDir_ExcludedAndDisclosed(t *testing.T) {
	repo := buildClosureGateQuarantineRepo(t)
	spec, _ := readSpec(t, repo.Dir, "quarantine-story")
	ctx := context.Background()
	const gone = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"

	// The record sits under repo.Head (REACHABLE) but carries a sync-written
	// quarantine annotation naming a since-deleted source commit.
	annotated := `[{"schema":"verdi.evidence/v1","evidence_for":["ac-1"],"kind":"static","verdict":"pass",` +
		`"witness":"someFunc @ site","provenance":{"source":"ci","pipeline":"1","commit":"` + gone + `"},` +
		`"quarantine":{"reason":"provenance.commit ` + gone + ` not reachable from HEAD at sync time"},` +
		`"digest":"sha256:` + strings.Repeat("ab", 32) + `"}]`
	writeClosureGateDerivedRecord(t, repo.Dir, spec.ID, repo.Head, annotated)

	var stdout bytes.Buffer
	ok, err := runClosureGate(ctx, repo.Dir, spec, nil, "main", nil, nil, repo.Head, &stdout)
	if err != nil {
		t.Fatalf("runClosureGate: %v; stdout=%s", err, stdout.String())
	}
	if ok {
		t.Fatalf("runClosureGate() = true, want false (ac-1's sole record is annotated-quarantined and must be excluded); stdout=%s", stdout.String())
	}
	if !contains(stdout.String(), "[FAIL] closure: 1.") {
		t.Fatalf("stdout = %q, want condition 1 to FAIL (an annotated record must never silently prove an AC — finding 1)", stdout.String())
	}
	if !contains(stdout.String(), "disclosed-unproven [gate:evidence-quarantine]") {
		t.Fatalf("stdout = %q, want the annotated record disclosed even though its directory is reachable (finding 1)", stdout.String())
	}
}

// TestRunClosureGate_UndecodableUnderUnreachableDir_NeverOperational is
// spec/evidence-resilience finding-2's regression pin: a verdicts.json that
// FAILS strict decode (a truncated partial write / older-schema record — the
// debris a stale poisoned bundle left behind by a deleted branch) sitting
// under an UNREACHABLE commit directory must NOT brick the closure gate
// operationally. Before the fix, checkClosureEligible's disclosure pass
// (QuarantinedRecords) strict-decoded every file under unreachable dirs and
// surfaced the decode failure as a returned error (operational exit 2) — on
// exactly the degraded-evidence shape this story exists to make non-fatal.
// The gate must instead evaluate cleanly, disclose the undecodable debris,
// and exit per verdict discipline.
func TestRunClosureGate_UndecodableUnderUnreachableDir_NeverOperational(t *testing.T) {
	repo := buildClosureGateQuarantineRepo(t)
	spec, _ := readSpec(t, repo.Dir, "quarantine-story")
	ctx := context.Background()
	const unreachable = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"

	// A truncated / malformed verdicts.json under the unreachable dir.
	writeClosureGateDerivedRecord(t, repo.Dir, spec.ID, unreachable, `[{"schema":"verdi.evidence/v1","evidence_for":["ac-1"`)

	var stdout bytes.Buffer
	ok, err := runClosureGate(ctx, repo.Dir, spec, nil, "main", nil, nil, repo.Head, &stdout)
	if err != nil {
		t.Fatalf("runClosureGate: want no error (undecodable debris inside quarantined data must never brick closure — finding 2), got %v; stdout=%s", err, stdout.String())
	}
	if ok {
		t.Fatalf("runClosureGate() = true, want false; stdout=%s", stdout.String())
	}
	if !contains(stdout.String(), "undecodable") {
		t.Fatalf("stdout = %q, want the undecodable quarantined file disclosed (finding 2)", stdout.String())
	}
}

// TestRunClosureGate_UndecodableUnderReachableDir_NeverOperational is
// spec/evidence-resilience finding-1's (FIX) core regression pin — the judge's
// own scenario reproduced directly against the closure gate: a truncated
// verdicts.json (the bundle's own per-spec record file, keyed by the ACCEPTED
// commit, which is self-or-ancestor of sync's commit and therefore REACHABLE at
// closure) must NOT brick the gate operationally. Before the fix,
// checkClosureEligible's own fold reader (foldStoryEvidence -> LoadRecords)
// strict-decoded every file under a reachable dir and returned the decode
// failure as a closure-gate error (exit 2) — sync had already written the
// known-undecodable bytes and exited 0 claiming "excluded from the fold and
// disclosed at closure", yet the closure surface then hard-failed on exactly
// that shape, deferring ac-2's removed brick from sync time to closure time.
// The gate must now evaluate cleanly (no error), read the story as NOT eligible
// (the excluded file never silently counts as evidence), and disclose the
// undecodable debris — identical to the unreachable-dir case above, degradation
// now being reachability-independent.
func TestRunClosureGate_UndecodableUnderReachableDir_NeverOperational(t *testing.T) {
	repo := buildClosureGateQuarantineRepo(t)
	spec, _ := readSpec(t, repo.Dir, "quarantine-story")
	ctx := context.Background()

	// A truncated / malformed verdicts.json under repo.Head — trivially
	// reachable from itself (the accepted commit's own dir), the exact shape a
	// truncated write of the bundle's own record file leaves behind.
	writeClosureGateDerivedRecord(t, repo.Dir, spec.ID, repo.Head, `[{"schema":"verdi.evidence/v1","evidence_for":["ac-1"`)

	var stdout bytes.Buffer
	ok, err := runClosureGate(ctx, repo.Dir, spec, nil, "main", nil, nil, repo.Head, &stdout)
	if err != nil {
		t.Fatalf("runClosureGate: want no error (an undecodable record file under a REACHABLE dir must never brick closure — finding 1), got %v; stdout=%s", err, stdout.String())
	}
	if ok {
		t.Fatalf("runClosureGate() = true, want false (ac-1's only record is undecodable and excluded, so it is not evidenced); stdout=%s", stdout.String())
	}
	if !contains(stdout.String(), "[FAIL] closure: 1.") {
		t.Fatalf("stdout = %q, want condition 1 to FAIL (an undecodable file never silently proves an AC)", stdout.String())
	}
	if !contains(stdout.String(), "undecodable") {
		t.Fatalf("stdout = %q, want the undecodable file under the reachable dir disclosed (finding 1)", stdout.String())
	}
}

// TestRunClosureGate_UnreachableRecordProvenanceUnderReachableDir_DisclosesUnproven
// is spec/evidence-resilience finding-2's behavioral pin at the closure gate —
// the judge's exact scenario. An UN-annotated record whose OWN
// provenance.commit is unreachable from HEAD, sitting under a REACHABLE
// commit directory (repo.Head — evidence synced to disk before this story
// landed, or hand-placed derived data), must be excluded from the fold (ac-1
// never silently proven) and disclosed with the PRE-STORY-SYNC fallback reason
// ("provenance.commit ... is not reachable from HEAD" — closuregate.go's
// fallback branch, finally reachable for exactly this shape), the closure run
// never exiting operationally. Before the fix, exclusion keyed on the
// directory alone, so this record was loaded and silently marked ac-1 proven —
// X-11b's false-green family surviving at the precise seam ac-2 hardens.
func TestRunClosureGate_UnreachableRecordProvenanceUnderReachableDir_DisclosesUnproven(t *testing.T) {
	repo := buildClosureGateQuarantineRepo(t)
	spec, _ := readSpec(t, repo.Dir, "quarantine-story")
	ctx := context.Background()
	const gone = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"

	// Un-annotated ci static-pass record for ac-1 under repo.Head (REACHABLE)
	// whose OWN provenance.commit is a since-deleted commit.
	writeClosureGateDerivedRecord(t, repo.Dir, spec.ID, repo.Head, closureGateQuarantineRecordJSON(gone))

	var stdout bytes.Buffer
	ok, err := runClosureGate(ctx, repo.Dir, spec, nil, "main", nil, nil, repo.Head, &stdout)
	if err != nil {
		t.Fatalf("runClosureGate: want no error (finding 2: an unreachable-provenance record under a reachable dir must never brick closure), got %v; stdout=%s", err, stdout.String())
	}
	if ok {
		t.Fatalf("runClosureGate() = true, want false (ac-1's sole record has an unreachable provenance.commit and must be excluded, not silently proven); stdout=%s", stdout.String())
	}
	if !contains(stdout.String(), "[FAIL] closure: 1.") {
		t.Fatalf("stdout = %q, want condition 1 to FAIL (an unreachable-provenance record never silently proves an AC — finding 2)", stdout.String())
	}
	if !contains(stdout.String(), "disclosed-unproven [gate:evidence-quarantine]") {
		t.Fatalf("stdout = %q, want the per-record disclosed-unproven line even though the directory is reachable (finding 2)", stdout.String())
	}
	if !contains(stdout.String(), "is not reachable from HEAD") {
		t.Fatalf("stdout = %q, want the pre-story-sync fallback reason (closuregate.go: provenance.commit ... is not reachable from HEAD)", stdout.String())
	}
	if !contains(stdout.String(), gone) {
		t.Fatalf("stdout = %q, want the unreachable provenance.commit named", stdout.String())
	}
}

func buildClosureGateRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/verdi.yaml":                         "schema: verdi.layout/v1\nforge: gitlab\n",
			".verdi/specs/active/stale-decline/spec.md": closureGateStorySpecMD,
			".verdi/specs/active/loan-mgmt/spec.md":     featureV1SpecMD,
		},
		Message: "closure gate fixture",
	}})
	checkoutBranch(t, repo.Dir, "feature/stale-decline")
	return repo
}

func seedAttestation(t *testing.T, root string) {
	t.Helper()
	dir := filepath.Join(root, ".verdi", "attestations", "jira-loan-1482")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nid: attestation/jira-loan-1482--ac-1\nkind: attestation\ntitle: \"ac-1\"\nowners: [platform-team]\nfrozen: { at: 2024-01-01, commit: " + gateFakeFrozenCommit + " }\n---\n# ac-1\n"
	if err := os.WriteFile(filepath.Join(dir, "ac-1.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestRunClosureGate_EligibleCondition proves the closure gate's condition
// 1: not eligible without the attestation, eligible with it.
func TestRunClosureGate_EligibleCondition(t *testing.T) {
	repo := buildClosureGateRepo(t)
	spec, _ := readSpec(t, repo.Dir, "stale-decline")
	ctx := context.Background()

	t.Run("no attestation: not eligible", func(t *testing.T) {
		var stdout bytes.Buffer
		ok, err := runClosureGate(ctx, repo.Dir, spec, nil, "main", nil, nil, repo.Head, &stdout)
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			t.Fatal("runClosureGate() = true, want false (no attestation, not eligible)")
		}
		if !contains(stdout.String(), "[FAIL] closure: 1.") {
			t.Fatalf("stdout = %q, want condition 1 to FAIL", stdout.String())
		}
	})

	t.Run("attestation present: eligible, closure gate passes", func(t *testing.T) {
		seedAttestation(t, repo.Dir)
		// Condition 4 (X-13/X-16/X-17): a living, fully-dispositioned report
		// already covering head, so the gate genuinely holds overall.
		writeGateReport(t, repo.Dir, repo.Head, dispositionedFindingYAML)
		var stdout bytes.Buffer
		ok, err := runClosureGate(ctx, repo.Dir, spec, nil, "main", nil, nil, repo.Head, &stdout)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Fatalf("runClosureGate() = false, want true; stdout=%s", stdout.String())
		}
	})
}

// TestRunClosureGate_SpecStaleCondition proves the closure gate blocks on
// an unresolved spec-stale flag (03 §The amendment ladder's rung-arbitrage
// counter-pressure) and passes once no such flag is raised.
func TestRunClosureGate_SpecStaleCondition(t *testing.T) {
	repo := buildClosureGateRepo(t)
	seedAttestation(t, repo.Dir)
	spec, _ := readSpec(t, repo.Dir, "stale-decline")
	ctx := context.Background()

	// Trigger (a): an accepted-deviation finding whose id equals the
	// story's own declared AC id (R4-I-18's operationalization).
	writeGateReport(t, repo.Dir, repo.Head, `  - { id: ac-1, kind: computed, text: "targets the AC's own declared text", disposition: accepted-deviation, note: "known drift" }
`)

	var stdout bytes.Buffer
	ok, err := runClosureGate(ctx, repo.Dir, spec, nil, "main", nil, nil, repo.Head, &stdout)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("runClosureGate() = true, want false (spec-stale, own-text trigger)")
	}
	if !contains(stdout.String(), "[FAIL] closure: 2.") {
		t.Fatalf("stdout = %q, want condition 2 to FAIL", stdout.String())
	}
}

// alignFakeJudgeDrifted writes a fake judge that emits a DIFFERENT
// judge-side id ("j-2") than alignFakeJudgeOK's "j-1" — a genuine judge
// re-roll that simply does not re-emit the prior finding at all (a drifting
// slug), the exact X-18 laundering shape this test replays.
func alignFakeJudgeDrifted(t *testing.T) []string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fakejudge-drifted.sh")
	script := "#!/bin/sh\ncat <<'EOF'\n{\"is_error\":false,\"subtype\":\"success\",\"result\":\"{\\\"findings\\\":[{\\\"id\\\":\\\"j-2\\\",\\\"text\\\":\\\"a different reading entirely\\\",\\\"confidence\\\":0.6}]}\"}\nEOF\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("writing fake judge: %v", err)
	}
	return []string{path}
}

// TestClosureGate_LaunderingReplay_SpecStaleCountUnchangedAcrossReroll is
// spec/finding-identity ac-3's laundering-replay proof, driven end to end
// through the REAL production path (runAlign -> disk -> runDisposition ->
// disk -> runAlign again -> checkSpecStaleCondition, the exact closure-gate
// function `verdi close` consults) rather than library calls in isolation —
// the true X-18 shape: round 1's judge finds j-1, a human accepts it as a
// deviation; round 2's judge re-rolls and simply does not re-emit j-1 at all
// (a drifting slug). j-1 must land in not-resurfaced:, and
// checkSpecStaleCondition's own accepted-deviation count — the exact input
// the spec-stale threshold gate reads — must be EXACTLY UNCHANGED across the
// reroll: never decremented (the laundering drain this story closes) and
// never inflated.
func TestClosureGate_LaunderingReplay_SpecStaleCountUnchangedAcrossReroll(t *testing.T) {
	repo := buildAlignRepo(t)
	svcDir := filepath.Join(repo.Dir, "loansvc")
	manifest := &store.Manifest{Audit: &store.AuditConfig{DeviationsStaleThreshold: 3}}

	// Round 1: align finds judged-j-1; a human accepts it as a deviation.
	deps1 := alignDeps{Runner: alignRunner(svcDir), JudgeCmd: alignFakeJudgeOK(t), ModelDigest: testResolveModelDigest(t, repo.Dir)}
	var out1, err1 bytes.Buffer
	if rc := runAlign(context.Background(), repo.Dir, false, deps1, &out1, &err1); rc != 0 {
		t.Fatalf("runAlign (round 1) = %d, want 0; stderr=%s", rc, err1.String())
	}
	var dstdout, dstderr bytes.Buffer
	if rc := runDisposition(repo.Dir, "spec/stale-decline", "judged-j-1", "accepted-deviation", "owner-ratified: intentional deviation", false, &dstdout, &dstderr); rc != 0 {
		t.Fatalf("runDisposition (round 1) = %d, want 0; stderr=%s", rc, dstderr.String())
	}

	spec, _ := readSpec(t, repo.Dir, "stale-decline")
	storyACIDs := make(map[string]bool, len(spec.AcceptanceCriteria))
	for _, ac := range spec.AcceptanceCriteria {
		storyACIDs[ac.ID] = true
	}

	round1Cond, err := checkSpecStaleCondition(repo.Dir, spec, manifest)
	if err != nil {
		t.Fatalf("checkSpecStaleCondition (round 1): %v", err)
	}
	round1Report := decodeReportFile(t, reportPathFor(repo.Dir, "stale-decline"))
	round1Count := evidence.SpecStale(evidence.SpecStaleInput{
		Findings:         round1Report.Findings,
		OwnNotResurfaced: round1Report.NotResurfaced,
		StoryACIDs:       storyACIDs,
		Threshold:        manifest.Audit.DeviationsStaleThreshold,
	}).AcceptedDeviationCount

	// Round 2: the judge re-rolls and does NOT re-emit j-1's slug at all.
	deps2 := alignDeps{Runner: alignRunner(svcDir), JudgeCmd: alignFakeJudgeDrifted(t), ModelDigest: testResolveModelDigest(t, repo.Dir)}
	var out2, err2 bytes.Buffer
	if rc := runAlign(context.Background(), repo.Dir, false, deps2, &out2, &err2); rc != 0 {
		t.Fatalf("runAlign (round 2, drifted) = %d, want 0; stderr=%s", rc, err2.String())
	}

	after := decodeReportFile(t, reportPathFor(repo.Dir, "stale-decline"))
	if _, ok := findingByID(after.Findings, "judged-j-1"); ok {
		t.Fatalf("judged-j-1 must not resurface in findings: after a drifted re-roll, got %+v", after.Findings)
	}
	if _, ok := findingByID(after.NotResurfaced, "judged-j-1"); !ok {
		t.Fatalf("judged-j-1 must land in not-resurfaced: after a drifted re-roll, got %+v", after.NotResurfaced)
	}

	round2Cond, err := checkSpecStaleCondition(repo.Dir, spec, manifest)
	if err != nil {
		t.Fatalf("checkSpecStaleCondition (round 2): %v", err)
	}
	round2Count := evidence.SpecStale(evidence.SpecStaleInput{
		Findings:         after.Findings,
		OwnNotResurfaced: after.NotResurfaced,
		StoryACIDs:       storyACIDs,
		Threshold:        manifest.Audit.DeviationsStaleThreshold,
	}).AcceptedDeviationCount

	// The property that actually matters: the RAW accepted-deviation count
	// checkSpecStaleCondition's own evidence.SpecStale call computes is
	// EXACTLY UNCHANGED across the reroll — never decremented (j-1 draining
	// out just because it moved from findings: to not-resurfaced:, the X-18
	// laundering drain) and never inflated. Threshold 3 means both rounds
	// also PASS the gate outright (1 <= 3), asserted too, but the count
	// equality is the load-bearing assertion — a PASS/FAIL comparison alone
	// cannot distinguish "count stayed 1" from "count silently became 0".
	if round1Count != round2Count {
		t.Fatalf("accepted-deviation count = %d (round 1) vs %d (round 2), want EXACTLY unchanged across the re-roll (X-18 laundering drain)", round1Count, round2Count)
	}
	if round1Count != 1 {
		t.Fatalf("round1Count = %d, want 1 (judged-j-1's own accepted-deviation)", round1Count)
	}
	if !round1Cond.OK || !round2Cond.OK {
		t.Fatalf("round1Cond.OK=%v round2Cond.OK=%v, want both true (threshold 3, only 1 accepted-deviation each round)", round1Cond.OK, round2Cond.OK)
	}
}

// TestRunClosureGate_PendingSupersessionCondition proves the exit
// criterion verbatim: "a pending-supersession flag blocks verdi close but
// not verdi build start/verdi gate while the manifest MR is open." An open
// (unmerged) supersession MR is visible only through the forge port —
// checkPendingSupersessionCondition (closuregate.go) is the only place
// this phase reads it; build start and gate (cascadecheck.go) never
// consult the forge at all, so they cannot be affected by it — that
// asymmetry, not a runtime check, is what proves the second half of this
// exit criterion.
func TestRunClosureGate_PendingSupersessionCondition(t *testing.T) {
	repo := buildClosureGateRepo(t)
	seedAttestation(t, repo.Dir)
	spec, _ := readSpec(t, repo.Dir, "stale-decline")
	writeGateReport(t, repo.Dir, repo.Head, dispositionedFindingYAML)
	ctx := context.Background()

	fakeForge := forgefake.New()
	fakeForge.SeedOpenMR("main", forge.OpenMR{ID: "42", SourceBranch: "supersede-loan-mgmt", Title: "supersede loan-mgmt"})
	fakeForge.SeedFile("supersede-loan-mgmt", ".verdi/specs/active/loan-mgmt-v2/spec.md",
		[]byte(featureV2SpecMD("supersession:\n  amended:\n    - { id: ac-1, note: \"corrected\" }")))

	// 1. The closure gate is blocked while the MR is open.
	var stdout bytes.Buffer
	ok, err := runClosureGate(ctx, repo.Dir, spec, fakeForge, "main", nil, nil, repo.Head, &stdout)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatalf("runClosureGate() = true, want false (pending-supersession, open MR); stdout=%s", stdout.String())
	}
	if !contains(stdout.String(), "[FAIL] closure: 3.") {
		t.Fatalf("stdout = %q, want condition 3 to FAIL", stdout.String())
	}

	// 2. verdi build start is NOT blocked — it never reads the forge, only
	// merged (local) supersessions, and none exists here.
	buildDeps := syncDeps{Runner: nil, GoTest: fakeGoTest{}, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}
	fresh := freshClosureGateRepoForBuildStart(t)
	var bstdout, bstderr bytes.Buffer
	got := runBuildStart(context.Background(), fresh.Dir, "spec/stale-decline", buildDeps, &bstdout, &bstderr)
	if got != 0 {
		t.Fatalf("runBuildStart (pending-supersession only, not merged) = %d, want 0; stderr=%s", got, bstderr.String())
	}

	// 3. verdi gate is NOT blocked either, for the same reason (condition 4
	// only ever consults local, merged specs/active/ — cascadecheck.go).
	var gstdout, gstderr bytes.Buffer
	gotGate := runGate(ctx, repo.Dir, spec, repo.Head, "main", nil, &gstdout, &gstderr)
	if gotGate != 0 {
		t.Fatalf("runGate (pending-supersession only, not merged) = %d, want 0; stdout=%s stderr=%s", gotGate, gstdout.String(), gstderr.String())
	}
}

// TestRunClosureGate_PendingSupersessionDisclosedUnproven proves the
// three-valued honesty fix (constitution 2/10): when the story implements a
// feature but the open-MR input is unavailable (nil/unreachable forge), the
// pending-supersession condition is reported disclosed-unproven — rendered
// through the shared internal/disclosure seam (spec/disclosure-seam-v2,
// ac-1), never a silent pass — while a reachable forge that finds no open
// supersession MR passes the condition outright.
func TestRunClosureGate_PendingSupersessionDisclosedUnproven(t *testing.T) {
	ctx := context.Background()

	t.Run("nil forge: disclosed-unproven notice, not a silent pass", func(t *testing.T) {
		repo := buildClosureGateRepo(t)
		seedAttestation(t, repo.Dir)
		spec, _ := readSpec(t, repo.Dir, "stale-decline")
		// Condition 4 (X-13/X-16/X-17): so the disclosure on condition 3 is
		// the only thing keeping this gate from a full PASS.
		writeGateReport(t, repo.Dir, repo.Head, dispositionedFindingYAML)

		var stdout bytes.Buffer
		ok, err := runClosureGate(ctx, repo.Dir, spec, nil, "main", nil, nil, repo.Head, &stdout)
		if err != nil {
			t.Fatal(err)
		}
		// Disclosure is not failure: eligible + not spec-stale + disclosed
		// pending-supersession still leaves the gate un-failed.
		if !ok {
			t.Fatalf("runClosureGate() = false, want true (disclosure is not failure); stdout=%s", stdout.String())
		}
		if !contains(stdout.String(), "closure: disclosed-unproven [gate:pending-supersession]:") {
			t.Fatalf("stdout = %q, want condition 3 disclosed through the shared internal/disclosure rendering, never a silent pass", stdout.String())
		}
		if contains(stdout.String(), "[PASS] closure: 3.") {
			t.Fatalf("stdout = %q, condition 3 must NOT silently pass on a nil forge", stdout.String())
		}
	})

	t.Run("reachable forge, no open MR: condition 3 passes", func(t *testing.T) {
		repo := buildClosureGateRepo(t)
		seedAttestation(t, repo.Dir)
		spec, _ := readSpec(t, repo.Dir, "stale-decline")
		writeGateReport(t, repo.Dir, repo.Head, dispositionedFindingYAML)

		fakeForge := forgefake.New() // no seeded open MRs
		var stdout bytes.Buffer
		ok, err := runClosureGate(ctx, repo.Dir, spec, fakeForge, "main", nil, nil, repo.Head, &stdout)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Fatalf("runClosureGate() = false, want true (no open supersession MR); stdout=%s", stdout.String())
		}
		if !contains(stdout.String(), "[PASS] closure: 3.") {
			t.Fatalf("stdout = %q, want condition 3 to PASS with a reachable forge and no open MR", stdout.String())
		}
	})
}

// freshClosureGateRepoForBuildStart builds a repo whose story spec is
// still status: draft-free accepted-pending-build with NO build branch cut
// yet, isolated from buildClosureGateRepo's own repo (which already sits
// on feature/stale-decline) so runBuildStart can cut the branch cleanly.
func freshClosureGateRepoForBuildStart(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/verdi.yaml":                         "schema: verdi.layout/v1\nforge: gitlab\n",
			".verdi/specs/active/stale-decline/spec.md": closureGateStorySpecMD,
			".verdi/specs/active/loan-mgmt/spec.md":     featureV1SpecMD,
		},
		Message: "closure gate fixture, no build branch yet",
	}})
}

// TestRunClosureGate_DispositionCompleteCondition is X-13/X-16/X-17's
// static register for the STORY closure gate's condition 4: every failure
// shape (no report at all — X-17's literal scenario; a stale-covers
// report; an undispositioned finding — X-13's literal scenario) refuses,
// naming the offenders and the closure ritual; a report that covers head
// with every finding dispositioned passes (D6-24: the freeze-in-place
// case must still hold). Mirrors gate_test.go's own
// TestGate_Condition3_FailsAlone in shape — the merge gate's condition 3
// and this closure-gate condition share the same underlying facts, just
// different remedy text.
func TestRunClosureGate_DispositionCompleteCondition(t *testing.T) {
	cases := []struct {
		name       string
		setup      func(t *testing.T, root, head string)
		wantOK     bool
		wantSubstr []string
	}{
		{
			name:       "no report at all (X-17)",
			setup:      func(t *testing.T, root, head string) {},
			wantOK:     false,
			wantSubstr: []string{"no deviation-report.md found at", "the closure ritual is align"},
		},
		{
			name: "stale covers",
			setup: func(t *testing.T, root, head string) {
				writeGateReport(t, root, "0000000000000000000000000000000000000b", dispositionedFindingYAML)
			},
			wantOK:     false,
			wantSubstr: []string{"covers 0000000000000000000000000000000000000b, not head", "the closure ritual is align"},
		},
		{
			name: "undispositioned finding (X-13)",
			setup: func(t *testing.T, root, head string) {
				writeGateReport(t, root, head, undispositionedFindingYAML)
			},
			wantOK:     false,
			wantSubstr: []string{"undispositioned finding(s) [f-1]", "the closure ritual is align"},
		},
		{
			name: "fresh, fully dispositioned (D6-24: freeze-in-place still holds)",
			setup: func(t *testing.T, root, head string) {
				writeGateReport(t, root, head, dispositionedFindingYAML)
			},
			wantOK: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := buildClosureGateRepo(t)
			seedAttestation(t, repo.Dir) // condition 1 holds regardless
			spec, _ := readSpec(t, repo.Dir, "stale-decline")
			tc.setup(t, repo.Dir, repo.Head)

			var stdout bytes.Buffer
			ok, err := runClosureGate(context.Background(), repo.Dir, spec, forgefake.New(), "main", nil, nil, repo.Head, &stdout)
			if err != nil {
				t.Fatal(err)
			}
			if ok != tc.wantOK {
				t.Fatalf("runClosureGate() = %v, want %v; stdout=%s", ok, tc.wantOK, stdout.String())
			}
			if tc.wantOK {
				if !contains(stdout.String(), "[PASS] closure: 4.") {
					t.Fatalf("stdout = %q, want condition 4 to PASS", stdout.String())
				}
				return
			}
			if !contains(stdout.String(), "[FAIL] closure: 4.") {
				t.Fatalf("stdout = %q, want condition 4 to FAIL", stdout.String())
			}
			for _, want := range tc.wantSubstr {
				if !contains(stdout.String(), want) {
					t.Fatalf("stdout = %q, want it to contain %q", stdout.String(), want)
				}
			}
		})
	}
}

// TestRunClosureGate_UnreadableAttestation_OperationalFailure pins ADJ-67 /
// D6-38 on the STORY closure gate. The round replaced the fold's stat-only
// AttestationExists with content-reading LoadAttestationState. On an
// attestation file that exists but cannot be read (mode 000), the old
// stat-only swallow returned (true, nil) — silently counting an unreadable
// file as a satisfied HUMAN attestation, so the gate computed a verdict (exit
// 0/1). The kept behavior propagates the os.ReadFile error out of Fold,
// through foldStoryEvidence's "folding evidence:" wrap and
// checkClosureEligible's "closure gate:" wrap, as an operational failure —
// exit 2 at the cmd level. This test asserts BOTH taxonomy views (the
// gate-function error path, matching this file's other runClosureGate tests,
// AND the cmd-level exit-2) and must FAIL if anyone restores the swallow.
func TestRunClosureGate_UnreadableAttestation_OperationalFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("DISCLOSURE: running as root — os.Chmod(0o000) does not restrict root's own reads, so this permission-based negative test cannot exercise the unreadable-attestation path under this user")
	}
	repo := buildClosureGateRepo(t)
	seedAttestation(t, repo.Dir) // authored attestation at attestations/jira-loan-1482/ac-1.md
	spec, _ := readSpec(t, repo.Dir, "stale-decline")
	ctx := context.Background()

	attPath := filepath.Join(repo.Dir, ".verdi", "attestations", "jira-loan-1482", "ac-1.md")
	if err := os.Chmod(attPath, 0o000); err != nil {
		t.Fatalf("os.Chmod(%s, 0o000): %v", attPath, err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(attPath, 0o644) // restore so t.TempDir()'s own cleanup can remove it
	})

	// Gate-function taxonomy: a non-nil "closure gate:"-wrapped error, ok
	// false — never a swallowed (true, nil) eligible verdict.
	var stdout bytes.Buffer
	ok, err := runClosureGate(ctx, repo.Dir, spec, nil, "main", nil, nil, repo.Head, &stdout)
	if err == nil {
		t.Fatalf("runClosureGate(unreadable attestation) err = nil (ok=%v) — an unreadable attestation must fail closed, never swallow to a satisfied attestation (ADJ-67/D6-38); stdout=%s", ok, stdout.String())
	}
	if ok {
		t.Fatalf("runClosureGate(unreadable attestation) ok = true, want false on an operational failure")
	}
	if !contains(err.Error(), "closure gate:") {
		t.Fatalf("err = %q, want the closure-gate-wrapped error path (closuregate.go's checkClosureEligible)", err.Error())
	}
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("err = %v, want it to wrap os.ErrPermission (the propagated os.ReadFile EACCES)", err)
	}

	// Cmd-level taxonomy: the same input drives `verdi close` to exit 2
	// (operational) — never 0 (clean) or 1 (a business-precondition refusal).
	deps := closeDeps{Forge: forgefake.New(), Registry: fake.New()}
	var cstdout, cstderr bytes.Buffer
	got := runClose(ctx, repo.Dir, "spec/stale-decline", &store.Manifest{}, deps, &cstdout, &cstderr)
	if got != 2 {
		t.Fatalf("runClose(story, unreadable attestation) = %d, want 2 (operational); stdout=%s stderr=%s", got, cstdout.String(), cstderr.String())
	}
	if !contains(cstderr.String(), "loading attestation state") {
		t.Fatalf("stderr = %q, want it to name the propagated attestation read error", cstderr.String())
	}
}
