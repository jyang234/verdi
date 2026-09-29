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
// local-operator disposition design §3; ledger SI-183, SI-185; Task 3 I-2
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
// exact binary (see the task report for the command). Re-ratcheted at the
// 2026-09-09 origin/main integration: the fixture's policy prose cites the
// local-operator disposition ledger row, and that row was renumbered
// SI-178 -> SI-183 when wave6's own SI-176..SI-180 landed on main, so the
// projection of that content necessarily moved. Both values were observed
// identical across repeated runs, and reverting only that two-word
// citation edit reproduces the prior pair exactly.
const (
	localOperatorWaveManifestDigest = "sha256:a400beb8837afd27ed7fbb712f00c9cd41b28816237a2de4b285b5d769c70b9a"
	localOperatorWaveAGENTSDigest   = "sha256:4a86cf8d36396bca9273ba9fe3056eb8a96a8aae42bb90fe185cfc17b53f7e58"
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
	pinFixtureDefaultBranch(t, repo.Dir)
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
	binary := buildVerdiBinary(t)
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
	// I-1 (whole-wave review): on success the verb names design §2.3's own
	// ratified ordering in one stderr line — commit the artifact, run
	// `verdi context project` (recording a disposition moves the
	// effective-policy digest the projections embed), commit the
	// regenerated projections, then run the gate — so an operator learns
	// the recipe from the verb itself, not from a later projection-drift
	// refusal at the gate.
	for _, want := range []string{"next steps", "commit", "verdi context project", "effective-policy digest", "gate"} {
		if !strings.Contains(record.stderr, want) {
			t.Fatalf("disposition record stderr = %q, want it to contain %q (the next-steps line)", record.stderr, want)
		}
	}
	dispositionPath := filepath.Join(repo.Dir, filepath.FromSlash(localOperatorDispositionRelPath))
	writtenBytes, err := os.ReadFile(dispositionPath)
	if err != nil {
		t.Fatalf("reading written disposition: %v", err)
	}

	// --- Step 4: the written artifact equals the committed fixture
	// disposition in every member `disposition record` can possibly
	// reproduce (I-2, M-3, whole-wave review). The committed fixture's
	// own template.digest was refreshed (this task) to the CURRENT
	// embedded internal/designscaffold/templates/policy-disposition.md's
	// digest — computed two independent ways for the task report,
	// humanartifact.ResolveScaffold and a direct sha256 of the embedded
	// file, in agreement — so Template.Digest is now asserted equal
	// below, not merely logged. ---
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
	if written.Template == nil || committed.Template == nil {
		t.Fatalf("written.Template = %+v, committed.Template = %+v, want both resolved", written.Template, committed.Template)
	}
	if written.Template.Digest != committed.Template.Digest {
		t.Fatalf("written.Template.Digest = %s, want %s (the committed fixture's digest tracks the current embedded template — I-2, whole-wave review)", written.Template.Digest, committed.Template.Digest)
	}

	// Whole-struct equality (M-3, review finding), replacing the former
	// per-field checks: DeepEqual after zeroing EXACTLY the two members
	// that cannot agree byte-for-byte, both named here, nothing else —
	// see dispositionForWholeStructComparison's own doc comment for why
	// both must go together.
	gotCmp, wantCmp := dispositionForWholeStructComparison(written), dispositionForWholeStructComparison(committed)
	if !reflect.DeepEqual(gotCmp, wantCmp) {
		t.Fatalf("written and committed dispositions differ outside Rationale/body prose:\nwritten=%+v\nwant   =%+v", gotCmp, wantCmp)
	}
	if byteEqual := bytes.Equal(writtenBytes, committedBytes); byteEqual {
		t.Logf("bonus: written disposition is byte-identical to the committed fixture")
	} else {
		t.Logf("written disposition is NOT byte-identical to the committed fixture (expected: the embedded scaffold always renders the fixed body placeholder \"TODO: replace with the real rationale before accept.\", never the fixture's own hand-authored rationale prose; every decoded field checked above — including Template.Digest — matches, so this is the ONLY source of the byte difference)")
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

// dispositionForWholeStructComparison returns a value copy of d for Step
// 4's whole-struct equality check (M-3, review finding), with EXACTLY
// two members left at their zero value — both necessary, both named
// here, nothing else:
//
//   - Rationale (the artifact's body prose): `disposition record` never
//     accepts rationale text as an operand at all — RenderDisposition
//     always renders the disposition scaffold's own fixed placeholder
//     body, "TODO: replace with the real rationale before accept." (the
//     last line of internal/designscaffold/templates/policy-
//     disposition.md). The committed fixture instead carries real,
//     hand-authored rationale prose (Task 2). A byte-for-byte match is
//     therefore structurally impossible; TestLocalOperatorWave's own
//     bytes.Equal log after this check discloses that, rather than
//     silently degrading the proof to something weaker.
//   - policyartifact.Disposition's own unexported "seal" field:
//     DecodeDisposition computes it once at decode time
//     (canonjson.Digest(d) in internal/policyartifact/disposition.go),
//     over the WHOLE decoded struct INCLUDING Rationale — so it
//     necessarily differs whenever Rationale does. It is a derived
//     value, never itself an operand of any verb, and this package
//     cannot assign it directly (unexported, different package): a
//     keyed struct literal naming every OTHER field — exactly what this
//     function returns — leaves it at its zero value on both sides,
//     which is the only way from here to hold it out of the comparison
//     without reimplementing canonjson.Digest.
//
// Every other field — Schema, ID, Kind, Title, Owners, Scope, Witness,
// Conclusion, Origin, Judgment, CompensatingControls, Approvals, Expiry,
// ReviewCondition, and Template (identity AND digest) — stays IN the
// comparison: each is either a real operand this task's story chose to
// match the fixture, or (Origin, Judgment, ReviewCondition, Scope) a
// value this verb always derives identically regardless of operand, so
// including them only strengthens the proof.
func dispositionForWholeStructComparison(d *policyartifact.Disposition) policyartifact.Disposition {
	return policyartifact.Disposition{
		Schema:               d.Schema,
		ID:                   d.ID,
		Kind:                 d.Kind,
		Title:                d.Title,
		Owners:               d.Owners,
		Scope:                d.Scope,
		Witness:              d.Witness,
		Conclusion:           d.Conclusion,
		Origin:               d.Origin,
		Judgment:             d.Judgment,
		CompensatingControls: d.CompensatingControls,
		Approvals:            d.Approvals,
		Expiry:               d.Expiry,
		ReviewCondition:      d.ReviewCondition,
		Template:             d.Template,
	}
}
