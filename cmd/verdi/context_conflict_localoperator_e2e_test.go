// context_conflict_localoperator_e2e_test.go proves the local-operator
// lifecycle-gate wiring end-to-end (2026-09-05 local-operator disposition
// design, ledger SI-178, Task 2 §3) against the committed hermetic fixture
// under testdata/localoperator/: a genuine `context conflict` pass through
// the real kernel carrying the local-operator-asserted disclosure with
// every disposition resolution proven, `build start` cutting the build
// branch through the same evaluation, a subject-mismatch identity
// resolving violated-with-witness (verdict not pass), and an absent
// identity resolving unproven (verdict not pass) — never a favorable
// default in either failure shape.
//
// Fixture provenance: testdata/localoperator/policy/dispositions/
// localop-story-no-conflict.md's witness (input_id, target_digest, and the
// seven claims) was copied verbatim from a real `context conflict`
// evaluation run against this exact fixture content BEFORE the disposition
// existed (a temporary test, deleted before this file's commit; see the
// task report for the exact commands and captured output). The bound
// approval principal "principal/local/Zml4dHVyZUB2ZXJkaS5pbnZhbGlk" is
// governanceprincipal.CanonicalPrincipalID("local", "fixture@verdi.invalid")
// — exactly the repo-local Git identity internal/fixturegit.Build
// configures, so the committed fixture authenticates with no test-time git
// config override at all.
package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/instructionprojection"
	"github.com/jyang234/verdi/internal/policyconflict"
)

// localOperatorTestdataDir is testdata/localoperator/'s path relative to
// this package's own directory (cmd/verdi/testdata/... — never a sibling
// package's fixture).
const localOperatorTestdataDir = "testdata/localoperator"

// localOperatorFixtureFiles reads the committed hermetic fixture tree into
// a fixturegit.Layer file map keyed by real store-relative paths.
func localOperatorFixtureFiles(t *testing.T) map[string]string {
	t.Helper()
	read := func(rel string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(localOperatorTestdataDir, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("read fixture %s: %v", rel, err)
		}
		return string(data)
	}
	return map[string]string{
		".verdi/verdi.yaml":                                       read("verdi.yaml"),
		".verdi/specs/active/localop-feature/spec.md":             read("specs/localop-feature.md"),
		".verdi/specs/active/localop-story/spec.md":               read("specs/localop-story.md"),
		".verdi/obligations/localop-story/ac-1--behavioral.md":    read("obligations/localop-story-ac-1-behavioral.md"),
		".verdi/policy/constitution.md":                           read("policy/constitution.md"),
		".verdi/policy/profiles/local-operator.md":                read("policy/profiles/local-operator.md"),
		".verdi/policy/dispositions/localop-story-no-conflict.md": read("policy/dispositions/localop-story-no-conflict.md"),
	}
}

// buildLocalOperatorRepo builds a fresh, real fixturegit repository from
// the committed testdata/localoperator/ tree and generates the one real
// managed instruction projection this store's adapter declares (mirroring
// buildContextCompileRepo's own pattern in context_test.go), so a
// conflict evaluation's compiler stage finds a clean, non-drifted
// projection.
func buildLocalOperatorRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: localOperatorFixtureFiles(t), Message: "adopt the local-operator fixture store"}})
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	if _, err := instructionprojection.Generate(repo.Dir); err != nil {
		t.Fatalf("instructionprojection.Generate: %v", err)
	}
	repo.Head = commitAllOnCurrentBranch(t, repo.Dir, "generate instruction projection")
	return repo
}

// setLocalOperatorGitIdentity overrides the fixture's repo-local Git
// identity after the fact — fixturegit.Build always configures
// "fixture@verdi.invalid" (the fixture's own bound subject); a test
// proving a DIFFERENT resolution outcome must explicitly ask for a
// different (or absent) one.
func setLocalOperatorGitIdentity(t *testing.T, dir, email string) {
	t.Helper()
	if out, err := exec.Command("git", "-C", dir, "config", "user.email", email).CombinedOutput(); err != nil {
		t.Fatalf("git config user.email %s: %v\n%s", email, err, out)
	}
}

// clearLocalOperatorGitIdentity unsets BOTH keys gitx.ConfigValue falls
// back across, reproducing "no local Git identity configured" regardless
// of what fixturegit.Build set.
func clearLocalOperatorGitIdentity(t *testing.T, dir string) {
	t.Helper()
	for _, key := range []string{"user.email", "user.name"} {
		if out, err := exec.Command("git", "-C", dir, "config", "--unset", key).CombinedOutput(); err != nil {
			t.Fatalf("git config --unset %s: %v\n%s", key, err, out)
		}
	}
}

// localOperatorConflictRequestBytes builds a canonical
// verdi.policy-conflict-request/v1 document for the standalone `context
// conflict` verb — the accepted-context arm at phase, spec
// spec/localop-story, no Expected repository claim. This is a DIFFERENT
// wire shape than the raw contextcompile.Request file `build start
// --context-request` reads (contextLifecycleRequestFile): `context
// conflict --request` decodes the wrapped policyconflict.Request document
// directly (policyconflict.DecodeRequest).
func localOperatorConflictRequestBytes(t *testing.T, phase contextcompile.Phase) []byte {
	t.Helper()
	accepted, err := contextcompile.DecodeRequest(contextRequestBytes(t, "spec/localop-story", phase, nil))
	if err != nil {
		t.Fatalf("DecodeRequest fixture: %v", err)
	}
	data, err := policyconflict.EncodeRequest(policyconflict.Request{
		Schema: policyconflict.RequestSchema,
		Target: policyconflict.Target{Kind: policyconflict.TargetAcceptedContext, AcceptedContext: &accepted},
	})
	if err != nil {
		t.Fatalf("EncodeRequest fixture: %v", err)
	}
	return data
}

// localOperatorHasDisclosure reports whether report carries a disclosure
// of code, returning it for further inspection (e.g. its witnesses).
func localOperatorHasDisclosure(report policyconflict.Report, code policyconflict.DisclosureCode) (policyconflict.Disclosure, bool) {
	for _, d := range report.Disclosures {
		if d.Code == code {
			return d, true
		}
	}
	return policyconflict.Disclosure{}, false
}

// TestContextConflict_LocalOperator_PassWithDisclosure is the central
// design §3 proof: against the unmodified committed fixture (whose bound
// subject is exactly fixturegit's own repo-local identity), `context
// conflict` reaches verdict pass with every one of the disposition's five
// resolution states proven and the local-operator-asserted disclosure
// present exactly once, witnessed by the trust source id.
func TestContextConflict_LocalOperator_PassWithDisclosure(t *testing.T) {
	repo := buildLocalOperatorRepo(t)
	requestPath := writeContextRequestFile(t, repo.Dir, "conflict-request.json", localOperatorConflictRequestBytes(t, contextcompile.PhaseBuild))
	t.Chdir(repo.Dir)

	var stdout, stderr bytes.Buffer
	code := cmdContextConflict([]string{"--request", requestPath}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("exit=%d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	report, err := policyconflict.DecodeReport(stdout.Bytes())
	if err != nil {
		t.Fatalf("DecodeReport: %v\n%s", err, stdout.String())
	}
	if report.Verdict != policyconflict.VerdictPass {
		t.Fatalf("verdict = %s, want pass\n%s", report.Verdict, stdout.String())
	}
	if len(report.Semantic) != 1 || len(report.Semantic[0].Dispositions) != 1 {
		t.Fatalf("semantic = %+v, want exactly one row with one disposition", report.Semantic)
	}
	res := report.Semantic[0].Dispositions[0].Resolution
	if res.Match != policyconflict.ProofProven || res.Freshness != policyconflict.ProofProven ||
		res.Scope != policyconflict.ProofProven || res.Bound != policyconflict.ProofProven ||
		res.Authorization != policyconflict.ProofProven {
		t.Fatalf("resolution = %+v, want all five proven", res)
	}
	disclosure, ok := localOperatorHasDisclosure(report, policyconflict.DisclosureLocalOperatorAsserted)
	if !ok {
		t.Fatalf("disclosures = %+v, want local-operator-asserted present", report.Disclosures)
	}
	if len(disclosure.Witnesses) != 1 || disclosure.Witnesses[0] != "local" {
		t.Fatalf("disclosure witnesses = %v, want [local]", disclosure.Witnesses)
	}
}

// TestBuildStart_LocalOperator_CutsBranch is the design §3 lifecycle-gate
// proof: `build start spec/localop-story --context-request PATH` reaches
// exit 0 through the SAME factory and disposition, cutting
// feature/localop-story.
func TestBuildStart_LocalOperator_CutsBranch(t *testing.T) {
	repo := buildLocalOperatorRepo(t)
	requestPath := contextLifecycleRequestFile(t, repo.Dir, "build-start-context.json", "spec/localop-story", contextcompile.PhaseBuild, nil)
	t.Chdir(repo.Dir)

	var stdout, stderr bytes.Buffer
	code := cmdBuildStart([]string{"spec/localop-story", "--context-request", requestPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "constitutional conflict: state: pass") {
		t.Fatalf("stdout = %q, want a passing conflict summary", stdout.String())
	}
	branch, err := gitx.CurrentBranch(context.Background(), repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if branch != "feature/localop-story" {
		t.Fatalf("branch = %q, want feature/localop-story; stdout=%s stderr=%s", branch, stdout.String(), stderr.String())
	}
}

// TestContextConflict_LocalOperator_SubjectMismatch_Violated proves
// design §3's "subject mismatch ⇒ violated-with-witness, never a
// favorable default": the store's Git identity resolves to a real,
// available subject the profile's role_mappings do not bind, so the
// resolution ITSELF is violated-with-witness (governanceprincipal.
// resolve.go's ReasonTrustSubjectMismatch) and — because this test's
// disposition variant names exactly that violated principal as its
// approver — the disposition's own authorization resolution is
// violated-with-witness too, never merely "unproven" (which would make
// this indistinguishable from the absent-identity case below). The
// verdict is not pass either way.
func TestContextConflict_LocalOperator_SubjectMismatch_Violated(t *testing.T) {
	const mismatchedSubject = "unbound@example.com"
	mismatchedID, err := governanceprincipal.CanonicalPrincipalID("local", mismatchedSubject)
	if err != nil {
		t.Fatal(err)
	}
	files := localOperatorFixtureFiles(t)
	dispositionPath := ".verdi/policy/dispositions/localop-story-no-conflict.md"
	const boundPrincipal = "principal/local/Zml4dHVyZUB2ZXJkaS5pbnZhbGlk"
	if !strings.Contains(files[dispositionPath], boundPrincipal) {
		t.Fatalf("fixture disposition does not contain the expected bound principal token %q; the fixture likely changed", boundPrincipal)
	}
	files[dispositionPath] = strings.Replace(files[dispositionPath], boundPrincipal, string(mismatchedID), 1)

	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: files, Message: "adopt the local-operator fixture store (mismatched approver variant)"}})
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	if _, err := instructionprojection.Generate(repo.Dir); err != nil {
		t.Fatalf("instructionprojection.Generate: %v", err)
	}
	repo.Head = commitAllOnCurrentBranch(t, repo.Dir, "generate instruction projection")
	setLocalOperatorGitIdentity(t, repo.Dir, mismatchedSubject)

	requestPath := writeContextRequestFile(t, repo.Dir, "conflict-request.json", localOperatorConflictRequestBytes(t, contextcompile.PhaseBuild))
	t.Chdir(repo.Dir)

	var stdout, stderr bytes.Buffer
	code := cmdContextConflict([]string{"--request", requestPath}, strings.NewReader(""), &stdout, &stderr)
	if code != 1 || stderr.Len() != 0 {
		t.Fatalf("exit=%d stderr=%q stdout=%q, want a completed blocked report", code, stderr.String(), stdout.String())
	}
	report, err := policyconflict.DecodeReport(stdout.Bytes())
	if err != nil {
		t.Fatalf("DecodeReport: %v\n%s", err, stdout.String())
	}
	if report.Verdict == policyconflict.VerdictPass {
		t.Fatalf("verdict = pass, want not-pass for a subject not bound by role_mappings")
	}
	if len(report.Semantic) != 1 || len(report.Semantic[0].Dispositions) != 1 {
		t.Fatalf("semantic = %+v, want exactly one row with one disposition", report.Semantic)
	}
	res := report.Semantic[0].Dispositions[0].Resolution
	if res.Authorization != policyconflict.ProofViolatedWithWitness {
		t.Fatalf("authorization = %q, want violated-with-witness (never a favorable default); resolution=%+v", res.Authorization, res)
	}
	if _, ok := localOperatorHasDisclosure(report, policyconflict.DisclosureLocalOperatorAsserted); !ok {
		t.Fatalf("disclosures = %+v, want local-operator-asserted present even for a violated resolution", report.Disclosures)
	}
}

// TestContextConflict_LocalOperator_AbsentIdentity_Unproven proves design
// §3's "absent identity ⇒ unproven": neither user.email nor user.name is
// configured, so resolveLocalActors still attempts resolution (never
// silently omitting it) and the resolver reports unproven through the
// real port. The verdict is not pass.
func TestContextConflict_LocalOperator_AbsentIdentity_Unproven(t *testing.T) {
	repo := buildLocalOperatorRepo(t)
	clearLocalOperatorGitIdentity(t, repo.Dir)
	requestPath := writeContextRequestFile(t, repo.Dir, "conflict-request.json", localOperatorConflictRequestBytes(t, contextcompile.PhaseBuild))
	t.Chdir(repo.Dir)

	var stdout, stderr bytes.Buffer
	code := cmdContextConflict([]string{"--request", requestPath}, strings.NewReader(""), &stdout, &stderr)
	if code != 1 || stderr.Len() != 0 {
		t.Fatalf("exit=%d stderr=%q stdout=%q, want a completed blocked report", code, stderr.String(), stdout.String())
	}
	report, err := policyconflict.DecodeReport(stdout.Bytes())
	if err != nil {
		t.Fatalf("DecodeReport: %v\n%s", err, stdout.String())
	}
	if report.Verdict == policyconflict.VerdictPass {
		t.Fatalf("verdict = pass, want not-pass for an absent local identity")
	}
	if len(report.Semantic) != 1 || len(report.Semantic[0].Dispositions) != 1 {
		t.Fatalf("semantic = %+v, want exactly one row with one disposition", report.Semantic)
	}
	res := report.Semantic[0].Dispositions[0].Resolution
	if res.Authorization != policyconflict.ProofUnproven {
		t.Fatalf("authorization = %q, want unproven; resolution=%+v", res.Authorization, res)
	}
	if _, ok := localOperatorHasDisclosure(report, policyconflict.DisclosureLocalOperatorAsserted); !ok {
		t.Fatalf("disclosures = %+v, want local-operator-asserted present even for an unproven resolution", report.Disclosures)
	}
}

// --- operational-mapping proofs (review finding: context_conflict.go's
// factory-error -> exit 2 mapping was previously proven only by reading) --

// localOperatorTwoSourceProfileMD declares two local-operator trust
// sources — an ambiguous declaration resolveLocalActors refuses rather
// than guessing which one to trust.
const localOperatorTwoSourceProfileMD = `---
schema: verdi.governance-profile/v1
id: solo-local-ambiguous
class: solo
applicable_transitions: [policy-disposition-approval]
identity_trust_sources:
  - {id: local, kind: local-operator}
  - {id: local2, kind: local-operator}
role_mappings:
  - {role: policy-owner, trust_source: local, subjects: [wiring-fixture@example.com]}
  - {role: policy-owner, trust_source: local2, subjects: [wiring-fixture@example.com]}
ownership_sources: []
signature_requirements: []
required_approvers:
  - {transitions: [policy-disposition-approval], roles: [policy-owner], minimum: 1}
distinctness_rules: []
evidence_source_restrictions: []
escalation_thresholds: []
---
Fixture-only profile: declares two local-operator sources to prove
resolveLocalActors' ambiguity refusal reaches the CLI's exit-2 mapping.
`

// buildLocalOperatorTwoSourceRepo builds a minimal adopted store (no
// specs — the factory fails before Evaluate ever needs one) whose
// selected profile declares two local-operator sources.
func buildLocalOperatorTwoSourceRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	const from, to = "selected_profile: solo-local\n", "selected_profile: solo-local-ambiguous\n"
	constitution := strings.Replace(localOperatorConstitutionMD, from, to, 1)
	if constitution == localOperatorConstitutionMD {
		t.Fatalf("expected substitution of %q to change the constitution fixture", from)
	}
	return fixturegit.Build(t, []fixturegit.Layer{{Files: map[string]string{
		".verdi/verdi.yaml":                              "schema: verdi.layout/v1\n",
		".verdi/policy/constitution.md":                  constitution,
		".verdi/policy/profiles/solo-local-ambiguous.md": localOperatorTwoSourceProfileMD,
	}, Message: "adopt a store whose selected profile declares two local-operator sources"}})
}

// TestContextConflict_LocalOperator_NonTopLevelRoot_Operational proves the
// exit-2 mapping at context_conflict.go's factory call site actually fires
// for verifyGitTopLevel's refusal: the fixture is nested one directory
// below the real Git top level, so store.FindRoot(".") resolves a store
// root that is NOT itself a Git repository top level.
func TestContextConflict_LocalOperator_NonTopLevelRoot_Operational(t *testing.T) {
	files := make(map[string]string)
	for rel, content := range localOperatorFixtureFiles(t) {
		files["nested/"+rel] = content
	}
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: files, Message: "adopt the local-operator fixture nested inside a larger repository"}})
	nestedRoot := filepath.Join(repo.Dir, "nested")

	requestPath := writeContextRequestFile(t, nestedRoot, "conflict-request.json", localOperatorConflictRequestBytes(t, contextcompile.PhaseBuild))
	t.Chdir(nestedRoot)

	var stdout, stderr bytes.Buffer
	code := cmdContextConflict([]string{"--request", requestPath}, strings.NewReader(""), &stdout, &stderr)
	if code != 2 || stdout.Len() != 0 || stderr.Len() == 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q, want an operational refusal", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "top level") {
		t.Fatalf("stderr = %q, want it to name the top-level check", stderr.String())
	}
}

// TestContextConflict_LocalOperator_AmbiguousConfig_Operational proves the
// same exit-2 mapping for gitx.ConfigValue's ambiguous-value refusal: a
// second user.email value alongside fixturegit's own is a genuine
// operational error, never conflated with "no identity configured".
func TestContextConflict_LocalOperator_AmbiguousConfig_Operational(t *testing.T) {
	repo := buildLocalOperatorRepo(t)
	if out, err := exec.Command("git", "-C", repo.Dir, "config", "--add", "user.email", "second@example.com").CombinedOutput(); err != nil {
		t.Fatalf("git config --add user.email: %v\n%s", err, out)
	}
	requestPath := writeContextRequestFile(t, repo.Dir, "conflict-request.json", localOperatorConflictRequestBytes(t, contextcompile.PhaseBuild))
	t.Chdir(repo.Dir)

	var stdout, stderr bytes.Buffer
	code := cmdContextConflict([]string{"--request", requestPath}, strings.NewReader(""), &stdout, &stderr)
	if code != 2 || stdout.Len() != 0 || stderr.Len() == 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q, want an operational refusal", code, stdout.String(), stderr.String())
	}
}

// TestContextConflict_LocalOperator_TwoSources_Operational proves the same
// exit-2 mapping for resolveLocalActors' own multi-source refusal.
func TestContextConflict_LocalOperator_TwoSources_Operational(t *testing.T) {
	repo := buildLocalOperatorTwoSourceRepo(t)
	requestPath := writeContextRequestFile(t, repo.Dir, "conflict-request.json", localOperatorConflictRequestBytes(t, contextcompile.PhaseBuild))
	t.Chdir(repo.Dir)

	var stdout, stderr bytes.Buffer
	code := cmdContextConflict([]string{"--request", requestPath}, strings.NewReader(""), &stdout, &stderr)
	if code != 2 || stdout.Len() != 0 || stderr.Len() == 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q, want an operational refusal", code, stdout.String(), stderr.String())
	}
}

// TestBuildStart_LocalOperator_AmbiguousConfig_Operational is the one
// build-start-path arm the finding also asks for: the same ambiguous-config
// factory failure, reached through cmdBuildStart's own conflict-gate call
// after the accepted/obligation-quality preconditions already passed.
func TestBuildStart_LocalOperator_AmbiguousConfig_Operational(t *testing.T) {
	repo := buildLocalOperatorRepo(t)
	if out, err := exec.Command("git", "-C", repo.Dir, "config", "--add", "user.email", "second@example.com").CombinedOutput(); err != nil {
		t.Fatalf("git config --add user.email: %v\n%s", err, out)
	}
	requestPath := contextLifecycleRequestFile(t, repo.Dir, "build-start-context.json", "spec/localop-story", contextcompile.PhaseBuild, nil)
	t.Chdir(repo.Dir)

	var stdout, stderr bytes.Buffer
	code := cmdBuildStart([]string{"spec/localop-story", "--context-request", requestPath}, &stdout, &stderr)
	if code != 2 || stderr.Len() == 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q, want an operational refusal", code, stdout.String(), stderr.String())
	}
	branch, err := gitx.CurrentBranch(context.Background(), repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if branch == "feature/localop-story" {
		t.Fatal("an operational factory failure must not cut the build branch")
	}
}
