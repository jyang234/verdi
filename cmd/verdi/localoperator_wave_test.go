// localoperator_wave_test.go is the whole-wave, CLI-level integration proof
// for Task 5a (FABLE controller, owner-ratified local-operator disposition
// plan): it drives the BUILT verdi binary (not in-process cmd* calls) end to
// end over the committed cmd/verdi/testdata/localoperator/ hermetic fixture,
// provisioned WITHOUT its recorded disposition, through every one of the
// four integrated lanes (context project's instruction-projection wrapper,
// the local-operator actor resolver wired into context conflict/build
// start, and disposition record) in one continuous story: regenerate ->
// blocked-unproven -> record a disposition -> compare it to the committed
// one -> commit it (and the projection it ripples) -> pass -> cut the build
// branch -> an unmapped identity stops passing again. 2026-09-05
// local-operator disposition design §3; ledger SI-178, SI-180; Task 3 I-2
// deferral.
//
// Every other local-operator proof in this package (context_conflict_
// localoperator_e2e_test.go, context_conflict_localoperator_test.go) drives
// cmdContextConflict/cmdBuildStart in-process against a store that ALREADY
// carries the committed disposition from its very first commit. This file
// is the one place the four lanes are exercised together, as a human
// operator would actually use them from a shell, against a store that
// starts without a disposition and gets one during the run.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/instructionprojection"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/policyconflict"
)

// localOperatorDispositionRelPath is the fixture disposition's store-
// relative path, both in the committed testdata tree and inside a
// provisioned store (store.PolicyDispositionPath("", "localop-story-no-
// conflict") without the ".verdi/" root join duplicated here).
const localOperatorDispositionRelPath = ".verdi/policy/dispositions/localop-story-no-conflict.md"

// localOperatorStoryInputID is the committed fixture disposition's own
// witness.input_id — the canonical semantic-input digest context compile
// evaluation derives from the fixture's UNCHANGED specs/obligations/policy
// content. Stripping only the disposition file never changes this: the
// input a disposition witnesses is independent of whether one is on file
// yet. Asserted against the freshly generated report below as a
// determinism cross-check, not trusted blindly.
const localOperatorStoryInputID = "sha256:929cf7e6506f57a60e4b0aad8f5467651cfac8fb159103d5f05b3d0d90a50f48"

// localOperatorStoryTargetDigest is the same fixture's witness.target_digest
// (== rawContentDigest of specs/localop-story.md's exact bytes, the
// disposition_record.go header's own "sha256:"+hex convention). Computed
// independently in TestLocalOperatorWave from the provisioned store's own
// checked-out file rather than pasted in twice; kept here only as the
// value the test's own computation is checked against.
const localOperatorStoryTargetDigest = "sha256:6ad39873f652dcbf3bb51232abb9905d9d12d0efb15d94b598194f1dc5304a99"

// localOperatorWaveManifestDigest and localOperatorWaveAGENTSDigest are a
// digest ratchet (CLAUDE.md "fixturegit stable SHAs, digest ratchets"):
// instructionprojection.Generate is a pure function of the fixture's own
// byte-stable authority content (fixturegit's fixed identity/date make
// every commit SHA, and therefore every rendered digest, reproducible
// across machines and runs), so `verdi context project` regenerating the
// DISPOSITION-LESS store always reproduces exactly these two digests.
// Captured from a real run of this exact fixture content through this
// exact binary (see the task report for the command).
const (
	localOperatorWaveManifestDigest = "sha256:7cfd0f0465d3cf90127a9f3db32a7b267bf85aedc72160e3636337a5ec26c5c3"
	localOperatorWaveAGENTSDigest   = "sha256:8d3687c6d30934efac07210eddc48b9216351c95028d2670c55a269369f8a177"
)

// buildLocalOperatorRepoNoDisposition is buildLocalOperatorRepo's (context_
// conflict_localoperator_e2e_test.go) own twin, stripping the recorded
// disposition from the fixture file map before the first commit — the
// design §3 / Task 3 I-2 deferred starting state this whole-wave proof
// begins from. Mirrors buildLocalOperatorRepo's own generate-then-commit
// shape exactly (a hermetic store's very first instruction projection is
// generated and committed once here, in Go, the same way any adopting
// store's first `verdi context project` run would be) so what
// TestLocalOperatorWave's own step 1 regenerates through the BUILT BINARY
// has a real, committed baseline to reproduce.
func buildLocalOperatorRepoNoDisposition(t *testing.T) *fixturegit.Repo {
	t.Helper()
	files := localOperatorFixtureFiles(t)
	if _, ok := files[localOperatorDispositionRelPath]; !ok {
		t.Fatalf("fixture file map has no %s to strip; the fixture layout changed", localOperatorDispositionRelPath)
	}
	delete(files, localOperatorDispositionRelPath)
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: files, Message: "adopt the local-operator fixture store without its recorded disposition"}})
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	if _, err := instructionprojection.Generate(repo.Dir); err != nil {
		t.Fatalf("instructionprojection.Generate: %v", err)
	}
	repo.Head = commitAllOnCurrentBranch(t, repo.Dir, "generate instruction projection")
	return repo
}

// sha256Digest is the "sha256:"+hex form used throughout this fixture
// (mirrors internal/contextcompile's and internal/policyconflict's own
// unexported rawContentDigest, unreachable from this package).
func sha256Digest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// parseContextProjectLines parses `context project`'s stdout contract
// (context_project.go's contextProjectFormatResult): sorted
// "<digest>  <path>\n" lines, into a path->digest map, failing the test on
// any line not matching that exact two-space-separated shape.
func parseContextProjectLines(t *testing.T, stdout string) map[string]string {
	t.Helper()
	out := make(map[string]string)
	trimmed := strings.TrimSuffix(stdout, "\n")
	if trimmed == "" {
		return out
	}
	for _, line := range strings.Split(trimmed, "\n") {
		parts := strings.SplitN(line, "  ", 2)
		if len(parts) != 2 {
			t.Fatalf("context project line %q does not match \"<digest>  <path>\"", line)
		}
		out[parts[1]] = parts[0]
	}
	return out
}

// TestLocalOperatorWave is the design §3 whole-wave proof (Task 5a): the
// four integrated lanes exercised together through the built binary, in
// the exact order a human operator would use them, against the committed
// hermetic fixture provisioned without its recorded disposition.
func TestLocalOperatorWave(t *testing.T) {
	binary := buildCountersignContractBinary(t)
	env := map[string]string{}
	repo := buildLocalOperatorRepoNoDisposition(t)

	// --- Step 1: `context project --root <store>` regenerates the
	// projections deterministically; digests equal the committed ones. ---
	proj1 := runCountersignContractBinary(t, binary, repo.Dir, env, "context", "project", "--root", repo.Dir)
	if proj1.code != 0 || proj1.stderr != "" {
		t.Fatalf("context project (initial): exit=%d stdout=%q stderr=%q", proj1.code, proj1.stdout, proj1.stderr)
	}
	digests := parseContextProjectLines(t, proj1.stdout)
	wantDigests := map[string]string{
		".verdi/policy/projections/codex.json": localOperatorWaveManifestDigest,
		"AGENTS.md":                            localOperatorWaveAGENTSDigest,
	}
	if !reflect.DeepEqual(digests, wantDigests) {
		t.Fatalf("context project (initial) digests = %v, want %v", digests, wantDigests)
	}
	for path, digest := range digests {
		data, err := os.ReadFile(filepath.Join(repo.Dir, filepath.FromSlash(path)))
		if err != nil {
			t.Fatalf("reading regenerated %s: %v", path, err)
		}
		if got := sha256Digest(data); got != digest {
			t.Fatalf("%s on-disk digest = %s, want the reported %s", path, got, digest)
		}
	}
	if status := gitStatusPorcelain(t, repo.Dir); status != "" {
		t.Fatalf("git status after regenerating projections over an already-projected store is not clean (Generate must be idempotent):\n%s", status)
	}

	// --- Step 2: `context conflict` on the disposition-less store ->
	// blocked-unproven, disposition-required among the row's reasons, the
	// local-operator-asserted disclosure present. ---
	requestPath := writeContextRequestFile(t, repo.Dir, "conflict-request.json", localOperatorConflictRequestBytes(t, contextcompile.PhaseBuild))
	conflict1 := runCountersignContractBinary(t, binary, repo.Dir, env, "context", "conflict", "--request", requestPath)
	if conflict1.code != 1 || conflict1.stderr != "" {
		t.Fatalf("context conflict (no disposition): exit=%d stdout=%q stderr=%q, want a completed blocked report", conflict1.code, conflict1.stdout, conflict1.stderr)
	}
	report1, err := policyconflict.DecodeReport([]byte(conflict1.stdout))
	if err != nil {
		t.Fatalf("DecodeReport (no disposition): %v\n%s", err, conflict1.stdout)
	}
	if report1.Verdict != policyconflict.VerdictBlockedUnproven {
		t.Fatalf("verdict (no disposition) = %s, want blocked-unproven\n%s", report1.Verdict, conflict1.stdout)
	}
	if len(report1.Semantic) != 1 {
		t.Fatalf("semantic (no disposition) = %+v, want exactly one row", report1.Semantic)
	}
	row1 := report1.Semantic[0]
	if len(row1.Dispositions) != 0 {
		t.Fatalf("dispositions (no disposition on disk) = %+v, want none", row1.Dispositions)
	}
	if !containsReason(row1.Reasons, policyconflict.ReasonDispositionRequired) {
		t.Fatalf("reasons (no disposition) = %v, want disposition-required among them", row1.Reasons)
	}
	if _, ok := localOperatorHasDisclosure(report1, policyconflict.DisclosureLocalOperatorAsserted); !ok {
		t.Fatalf("disclosures (no disposition) = %+v, want local-operator-asserted present", report1.Disclosures)
	}
	if row1.InputID != localOperatorStoryInputID {
		t.Fatalf("row input_id (no disposition) = %s, want %s (stripping the disposition must not change the semantic input identity)", row1.InputID, localOperatorStoryInputID)
	}
	reportPath := filepath.Join(repo.Dir, "conflict-report.json")
	if err := os.WriteFile(reportPath, []byte(conflict1.stdout), 0o644); err != nil {
		t.Fatalf("writing conflict report for disposition record: %v", err)
	}

	// --- Step 3: `disposition record` from that report writes the
	// artifact; --target-digest is the sha256 of the committed story spec
	// bytes at HEAD, verified equal to the report's own target-claim
	// authority digest before use. ---
	storyBytes, err := os.ReadFile(filepath.Join(repo.Dir, ".verdi/specs/active/localop-story/spec.md"))
	if err != nil {
		t.Fatalf("reading provisioned story spec: %v", err)
	}
	targetDigest := sha256Digest(storyBytes)
	if targetDigest != localOperatorStoryTargetDigest {
		t.Fatalf("computed target digest = %s, want %s", targetDigest, localOperatorStoryTargetDigest)
	}
	if got := targetClaimAuthorityDigests(row1.Claims, "spec/localop-story"); len(got) != 1 || got[0] != targetDigest {
		t.Fatalf("report's own spec/localop-story claim authority digests = %v, want exactly [%s]", got, targetDigest)
	}
	approverID, err := governanceprincipal.CanonicalPrincipalID("local", "fixture@verdi.invalid")
	if err != nil {
		t.Fatalf("CanonicalPrincipalID: %v", err)
	}
	record := runCountersignContractBinary(t, binary, repo.Dir, env,
		"disposition", "record",
		"--report", reportPath,
		"--row", row1.InputID,
		"--target", "spec/localop-story",
		"--target-digest", targetDigest,
		"--conclusion", "no-conflict",
		"--compensating-control", "Fixture-only human ruling recorded for the Task 2 hermetic fixture; not a real compensating control.",
		"--expiry", "2030-01-01",
		"--approver", "policy-owner="+string(approverID),
		"--id", "localop-story-no-conflict",
		"--title", "Local-operator story claims coexist without conflict",
		"--owner", "platform-team",
	)
	if record.code != 0 {
		t.Fatalf("disposition record: exit=%d stdout=%q stderr=%q", record.code, record.stdout, record.stderr)
	}
	dispositionPath := filepath.Join(repo.Dir, filepath.FromSlash(localOperatorDispositionRelPath))
	writtenBytes, err := os.ReadFile(dispositionPath)
	if err != nil {
		t.Fatalf("reading written disposition: %v", err)
	}

	// --- Step 4: the written artifact equals the committed fixture
	// disposition in witness, conclusion, origin, and approvals (every
	// operand above was chosen to match the fixture exactly, so title/
	// owners/expiry/controls match too). Template.Identity matches;
	// Template.Digest does NOT — disclosed below, not a defect this task's
	// write set may fix. ---
	committedBytes, err := os.ReadFile(filepath.Join(localOperatorTestdataDir, "policy/dispositions/localop-story-no-conflict.md"))
	if err != nil {
		t.Fatalf("reading committed fixture disposition: %v", err)
	}
	written, err := policyartifact.DecodeDisposition(writtenBytes)
	if err != nil {
		t.Fatalf("DecodeDisposition (written): %v", err)
	}
	committed, err := policyartifact.DecodeDisposition(committedBytes)
	if err != nil {
		t.Fatalf("DecodeDisposition (committed fixture): %v", err)
	}
	if !reflect.DeepEqual(written.Witness, committed.Witness) {
		t.Fatalf("written.Witness = %+v,\nwant (committed) %+v", written.Witness, committed.Witness)
	}
	if written.Conclusion != committed.Conclusion {
		t.Fatalf("written.Conclusion = %q, want %q", written.Conclusion, committed.Conclusion)
	}
	if written.Origin != committed.Origin {
		t.Fatalf("written.Origin = %q, want %q", written.Origin, committed.Origin)
	}
	if !reflect.DeepEqual(written.Approvals, committed.Approvals) {
		t.Fatalf("written.Approvals = %+v, want %+v", written.Approvals, committed.Approvals)
	}
	if written.Title != committed.Title || !reflect.DeepEqual(written.Owners, committed.Owners) ||
		written.Expiry != committed.Expiry || !reflect.DeepEqual(written.CompensatingControls, committed.CompensatingControls) {
		t.Fatalf("supplied operands drifted from the fixture: title=%q/%q owners=%v/%v expiry=%q/%q controls=%v/%v",
			written.Title, committed.Title, written.Owners, committed.Owners, written.Expiry, committed.Expiry, written.CompensatingControls, committed.CompensatingControls)
	}
	if written.Template == nil || committed.Template == nil || written.Template.Identity != committed.Template.Identity {
		t.Fatalf("written.Template = %+v, want identity %q to match the committed fixture's %+v", written.Template, "embedded:policy-disposition.md", committed.Template)
	}
	// KNOWN, DISCLOSED, PRE-EXISTING discrepancy (not this task's write set
	// to fix): commit 992ff3a7 ("humanartifact: RenderDisposition gains
	// multi-claim/human-fallback support", Task 3) edited the embedded
	// internal/designscaffold/templates/policy-disposition.md AFTER commit
	// 8f9523c1 ("Add the hermetic local-operator fixture and its end-to-end
	// proofs", Task 2) authored and recorded this fixture's template.digest
	// — 992ff3a7's own commit message documents the digest move as
	// expected ("byte-for-byte unchanged, save for the template's own
	// self-digest"). The committed fixture's template.digest is therefore
	// stale relative to the CURRENT embedded template; this has no bearing
	// on any pass/fail proof below (Disposition.Template is provenance
	// only — never compared during conflict evaluation).
	if written.Template.Digest == committed.Template.Digest {
		t.Logf("NOTE: written and committed template digests now match (%s) — the pre-existing staleness this test disclosed may have been fixed upstream; no action needed here", written.Template.Digest)
	} else {
		t.Logf("DISCLOSED (not a defect): template digest differs — written=%s committed(stale, pre-dates commit 992ff3a7)=%s", written.Template.Digest, committed.Template.Digest)
	}
	if byteEqual := bytes.Equal(writtenBytes, committedBytes); byteEqual {
		t.Logf("bonus: written disposition is byte-identical to the committed fixture")
	} else {
		t.Logf("written disposition is NOT byte-identical to the committed fixture (expected: the embedded scaffold always renders the fixed body placeholder \"TODO: replace with the real rationale before accept.\", never the fixture's own hand-authored rationale prose, and template.digest differs per the note above); every kernel field checked above matches")
	}

	// --- Step 5: commit the artifact (fixture identity/dates, mirroring
	// buildLocalOperatorRepo's own post-Build commit shape), regenerate and
	// commit the instruction projection it ripples (recording a
	// disposition changes the effective-policy digest AGENTS.md/the
	// manifest embed — DC-1; the previously committed projection is now
	// stale), then `context conflict` -> pass, resolution all proven,
	// disclosure present. ---
	repo.Head = commitAllOnCurrentBranch(t, repo.Dir, "record the local-operator disposition")
	proj2 := runCountersignContractBinary(t, binary, repo.Dir, env, "context", "project", "--root", repo.Dir)
	if proj2.code != 0 || proj2.stderr != "" {
		t.Fatalf("context project (post-disposition): exit=%d stdout=%q stderr=%q", proj2.code, proj2.stdout, proj2.stderr)
	}
	repo.Head = commitAllOnCurrentBranch(t, repo.Dir, "regenerate the instruction projection after recording the disposition")

	conflict2 := runCountersignContractBinary(t, binary, repo.Dir, env, "context", "conflict", "--request", requestPath)
	if conflict2.code != 0 || conflict2.stderr != "" {
		t.Fatalf("context conflict (disposition committed): exit=%d stdout=%q stderr=%q, want a passing report", conflict2.code, conflict2.stdout, conflict2.stderr)
	}
	report2, err := policyconflict.DecodeReport([]byte(conflict2.stdout))
	if err != nil {
		t.Fatalf("DecodeReport (disposition committed): %v\n%s", err, conflict2.stdout)
	}
	if report2.Verdict != policyconflict.VerdictPass {
		t.Fatalf("verdict (disposition committed) = %s, want pass\n%s", report2.Verdict, conflict2.stdout)
	}
	if len(report2.Semantic) != 1 || len(report2.Semantic[0].Dispositions) != 1 {
		t.Fatalf("semantic (disposition committed) = %+v, want exactly one row with one disposition", report2.Semantic)
	}
	res2 := report2.Semantic[0].Dispositions[0].Resolution
	if res2.Match != policyconflict.ProofProven || res2.Freshness != policyconflict.ProofProven ||
		res2.Scope != policyconflict.ProofProven || res2.Bound != policyconflict.ProofProven ||
		res2.Authorization != policyconflict.ProofProven {
		t.Fatalf("resolution (disposition committed) = %+v, want all five proven", res2)
	}
	if _, ok := localOperatorHasDisclosure(report2, policyconflict.DisclosureLocalOperatorAsserted); !ok {
		t.Fatalf("disclosures (disposition committed) = %+v, want local-operator-asserted present", report2.Disclosures)
	}

	// --- Step 6: `build start spec/localop-story --context-request
	// <request file>` -> exit 0, feature/localop-story exists. ---
	buildRequestPath := contextLifecycleRequestFile(t, repo.Dir, "build-start-context.json", "spec/localop-story", contextcompile.PhaseBuild, nil)
	buildStart := runCountersignContractBinary(t, binary, repo.Dir, env, "build", "start", "spec/localop-story", "--context-request", buildRequestPath)
	if buildStart.code != 0 {
		t.Fatalf("build start: exit=%d stdout=%q stderr=%q", buildStart.code, buildStart.stdout, buildStart.stderr)
	}
	if !strings.Contains(buildStart.stdout, "constitutional conflict: state: pass") {
		t.Fatalf("build start stdout = %q, want a passing conflict summary", buildStart.stdout)
	}
	branch, err := gitx.CurrentBranch(context.Background(), repo.Dir)
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if branch != "feature/localop-story" {
		t.Fatalf("branch = %q, want feature/localop-story; stdout=%s stderr=%s", branch, buildStart.stdout, buildStart.stderr)
	}

	// --- Step 7: with the disposition present but the store's user.email
	// rewritten to a subject the profile's role_mappings do not bind,
	// `context conflict` -> not pass, disclosure still present. The
	// disposition's own authorization substate resolves UNPROVEN, not
	// violated-with-witness: fillRoles (internal/governanceprincipal/
	// authorize.go) only ever marks an approval violated when its named
	// principal ID is FOUND among the currently resolved actors AND that
	// resolution is itself violated; this disposition's one approval names
	// fixture@verdi.invalid's principal id (unchanged since step 3, so
	// step 4's byte/field comparison above stays meaningful), and the
	// newly observed "unbound@example.com" resolves to a DIFFERENT
	// principal id entirely, so the lookup simply misses
	// (ReasonPrincipalUnproven) — mechanically identical to
	// TestContextConflict_LocalOperator_AbsentIdentity_Unproven's shape,
	// not TestContextConflict_LocalOperator_SubjectMismatch_Violated's
	// (which reaches violated only by rewriting the disposition's approver
	// to name the SAME mismatched principal, which would break step 4's
	// comparison against the unmodified committed fixture). Verified
	// empirically before writing this assertion (see the task report); the
	// design's own invariant this step exists to prove — never a
	// favorable default — holds either way. ---
	setLocalOperatorGitIdentity(t, repo.Dir, "unbound@example.com")
	conflict3 := runCountersignContractBinary(t, binary, repo.Dir, env, "context", "conflict", "--request", requestPath)
	if conflict3.code != 1 || conflict3.stderr != "" {
		t.Fatalf("context conflict (unmapped identity): exit=%d stdout=%q stderr=%q, want a completed blocked report", conflict3.code, conflict3.stdout, conflict3.stderr)
	}
	report3, err := policyconflict.DecodeReport([]byte(conflict3.stdout))
	if err != nil {
		t.Fatalf("DecodeReport (unmapped identity): %v\n%s", err, conflict3.stdout)
	}
	if report3.Verdict == policyconflict.VerdictPass {
		t.Fatalf("verdict (unmapped identity) = pass, want not-pass for a subject not bound by role_mappings")
	}
	if len(report3.Semantic) != 1 || len(report3.Semantic[0].Dispositions) != 1 {
		t.Fatalf("semantic (unmapped identity) = %+v, want exactly one row with one disposition", report3.Semantic)
	}
	res3 := report3.Semantic[0].Dispositions[0].Resolution
	if res3.Authorization == policyconflict.ProofProven {
		t.Fatalf("authorization (unmapped identity) = proven, want a non-favorable result (never a favorable default); resolution=%+v", res3)
	}
	if res3.Authorization != policyconflict.ProofUnproven {
		t.Fatalf("authorization (unmapped identity) = %q, want unproven (see this step's doc comment for why violated-with-witness is unreachable here); resolution=%+v", res3.Authorization, res3)
	}
	if res3.Match != policyconflict.ProofProven || res3.Freshness != policyconflict.ProofProven ||
		res3.Scope != policyconflict.ProofProven || res3.Bound != policyconflict.ProofProven {
		t.Fatalf("resolution (unmapped identity) = %+v, want match/freshness/scope/bound still proven (only authorization is affected)", res3)
	}
	if _, ok := localOperatorHasDisclosure(report3, policyconflict.DisclosureLocalOperatorAsserted); !ok {
		t.Fatalf("disclosures (unmapped identity) = %+v, want local-operator-asserted still present", report3.Disclosures)
	}
}

// containsReason reports whether reasons contains want.
func containsReason(reasons []policyconflict.ReasonCode, want policyconflict.ReasonCode) bool {
	for _, r := range reasons {
		if r == want {
			return true
		}
	}
	return false
}
