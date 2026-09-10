package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/policyconflict"
)

// --- fixture constants, verified against cmd/verdi/testdata/disposition-record/report.json ---

const (
	dispositionRecordFixtureInputID         = "sha256:2222222222222222222222222222222222222222222222222222222222222222"
	dispositionRecordFixtureTargetRef       = "spec/example-story"
	dispositionRecordFixtureAuthorityDigest = "sha256:7777777777777777777777777777777777777777777777777777777777777777"
	dispositionRecordFixtureClaim1ID        = "policy/example-policy#instruction-1"
	dispositionRecordFixtureClaim1Digest    = "sha256:9999999999999999999999999999999999999999999999999999999999999999"
	dispositionRecordFixtureClaim2ID        = "spec/example-story#ac-1"
	dispositionRecordFixtureClaim2Digest    = "sha256:8888888888888888888888888888888888888888888888888888888888888888"
	dispositionRecordFixtureApproverArg     = "policy-owner=principal/github-org/YWxpY2U"
	dispositionRecordFixtureApproverRole    = "policy-owner"
	dispositionRecordFixtureApproverID      = "principal/github-org/YWxpY2U"
)

// dispositionRecordFixtureReport returns the committed base fixture's raw
// bytes — a real verdi.policy-conflict-report/v1 document (copied from
// internal/policyconflict/testdata/report.json, this package's own
// hermetic copy per CLAUDE.md's "testdata/ is the only home for fixtures")
// whose one semantic row carries two claims sharing one authority digest,
// no primary/challenger judge exchange (a genuine human-fallback shape),
// and whose one mechanical row names no exemption.
func dispositionRecordFixtureReport(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "disposition-record", "report.json"))
	if err != nil {
		t.Fatalf("reading fixture report: %v", err)
	}
	// Self-check: the fixture is real, canonical policyconflict content —
	// never a hand-typed stand-in that merely looks like one.
	if _, err := policyconflict.DecodeReport(data); err != nil {
		t.Fatalf("test setup: fixture report does not strict-decode: %v", err)
	}
	return data
}

// mutateFixtureReport decodes the committed base fixture, applies mutate
// to the live Go value, and re-encodes it through policyconflict's own
// EncodeReport — which recomputes the report's self digest — so every
// variant this file needs (a governing-parent-feature claim added, an
// exemption present on a mechanical row, a duplicated semantic row, ...)
// is genuine, canonical policyconflict content, never a hand-edited JSON
// string that would fail DecodeReport's own canonical round-trip check.
func mutateFixtureReport(t *testing.T, mutate func(*policyconflict.Report)) []byte {
	t.Helper()
	report, err := policyconflict.DecodeReport(dispositionRecordFixtureReport(t))
	if err != nil {
		t.Fatalf("test setup: decoding base fixture: %v", err)
	}
	mutate(&report)
	data, err := policyconflict.EncodeReport(report)
	if err != nil {
		t.Fatalf("test setup: re-encoding mutated fixture: %v", err)
	}
	return data
}

// dispositionRecordParentFeatureDigest and dispositionRecordParentFeatureID
// are the governing-parent-feature claim mutateFixtureReport's
// "add a parent claim" mutation inserts — a claim sharing the base
// fixture's own acceptance-criterion CATEGORY shape (spec-outcome, in
// internal/contextcompile's own vocabulary, is equally a "spec-shaped"
// category a governing parent contributes) but a DIFFERENT authority
// digest, simulating internal/contextcompile's buildFragmentProse
// (conflict.go:1000) — structurally indistinguishable from the target's
// own claim by category alone (review finding I-1).
const (
	dispositionRecordParentFeatureID     = "spec/example-feature#outcome"
	dispositionRecordParentFeatureDigest = "sha256:6666666666666666666666666666666666666666666666666666666666666666"
)

// withParentFeatureClaim mutates report to add a governing-parent-feature
// claim to its one semantic row, sorted into place (claims must arrive
// strictly sorted by id with no duplicates).
func withParentFeatureClaim(report *policyconflict.Report) {
	parent := policyartifact.SemanticClaimWitness{
		ID: dispositionRecordParentFeatureID, Digest: "sha256:" + strings.Repeat("5", 64),
		Category: "spec-outcome", AuthorityDigest: dispositionRecordParentFeatureDigest,
		Scope:  policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{dispositionRecordParentFeatureID}},
		Values: []string{},
	}
	claims := append([]policyartifact.SemanticClaimWitness{}, report.Semantic[0].Claims...)
	claims = append(claims, parent)
	sort.Slice(claims, func(i, j int) bool { return claims[i].ID < claims[j].ID })
	report.Semantic[0].Claims = claims
}

// dispositionRecordDistinctPolicyDigest is a digest distinct from the base
// fixture's target claim digest (dispositionRecordFixtureAuthorityDigest) —
// the base fixture's own policy-instruction claim happens to coincidentally
// share the target's digest, which would make a "policy digest is refused"
// test meaningless (indistinguishable from "wrong digest, not found
// anywhere"). withDistinctPolicyDigest gives the policy claim ITS OWN,
// different digest so the refusal is provably about CATEGORY exclusion,
// not mere absence.
const dispositionRecordDistinctPolicyDigest = "sha256:3333333333333333333333333333333333333333333333333333333333333333"

func withDistinctPolicyDigest(report *policyconflict.Report) {
	claims := report.Semantic[0].Claims
	for i := range claims {
		if claims[i].Category == "policy-instruction" {
			claims[i].AuthorityDigest = dispositionRecordDistinctPolicyDigest
		}
	}
}

// withOnlyPolicyInstructionClaim mutates report so its one semantic row
// carries no claim the target specification itself could have
// contributed — every remaining claim is policy-instruction — simulating
// "report whose target contributes no claims" (review finding I-1).
func withOnlyPolicyInstructionClaim(report *policyconflict.Report) {
	report.Semantic[0].Claims = []policyartifact.SemanticClaimWitness{
		{
			ID: dispositionRecordFixtureClaim1ID, Digest: dispositionRecordFixtureClaim1Digest,
			Category: "policy-instruction", AuthorityDigest: dispositionRecordFixtureAuthorityDigest,
			Scope:  policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{}},
			Values: []string{},
		},
	}
}

// withExemption mutates report so its one mechanical row names one
// applicable exemption — M-5, review finding.
const (
	dispositionRecordExemptionID     = "policy-exemption/example-exemption"
	dispositionRecordExemptionDigest = "sha256:4444444444444444444444444444444444444444444444444444444444444444"
)

func withExemption(report *policyconflict.Report) {
	report.Mechanical[0].Exemptions = []policyconflict.ExemptionResolution{
		{
			ID: dispositionRecordExemptionID, Digest: dispositionRecordExemptionDigest,
			// Resolution/RemovedClaims states are irrelevant to
			// reconstructApplicableExemptions (it reads only ID/Digest,
			// mirroring policyconflict's own unexported
			// applicableExemptionWitnesses) — ProofUnproven throughout
			// avoids Report.Validate's "an all-proven resolution must
			// name at least one removed claim" rule without asserting a
			// real, actually-applied exemption this test does not need.
			Resolution: policyconflict.AuthorityResolution{
				Match: policyconflict.ProofUnproven, Freshness: policyconflict.ProofUnproven,
				Scope: policyconflict.ProofUnproven, Bound: policyconflict.ProofUnproven, Authorization: policyconflict.ProofUnproven,
			},
			RemovedClaims: []policyconflict.MechanicalClaimWitness{},
		},
	}
}

// withAcceptanceCandidateTarget mutates report's Input.Target from
// accepted-context to acceptance-candidate, naming
// dispositionRecordFixtureTargetRef as the candidate's own ref — the one
// report arm that DOES carry the target ref directly
// (CandidateIdentity.Ref), letting --target be cross-checked against it
// (controller round 2, "candidate arm mismatch ⇒ refused"). Every other
// CandidateIdentity field is a well-formed placeholder Report.Validate
// requires but this test does not otherwise exercise.
func withAcceptanceCandidateTarget(report *policyconflict.Report) {
	report.Input.Target = policyconflict.TargetIdentity{
		Kind: policyconflict.TargetAcceptanceCandidate,
		Candidate: &policyconflict.CandidateIdentity{
			Ref:           dispositionRecordFixtureTargetRef,
			Path:          "story-alpha/spec.md",
			Branch:        "feature/story-alpha",
			Head:          strings.Repeat("a", 40),
			Blob:          strings.Repeat("b", 40),
			ContentDigest: dispositionRecordFixtureAuthorityDigest,
			Scope:         policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{}},
			Adapter:       contextcompile.AdapterRef{ID: "codex", Version: "1"},
			GrantDigest:   "sha256:" + strings.Repeat("d", 64),
		},
	}
}

// withDuplicateSemanticRow mutates report so its Semantic slice carries
// two rows sharing the same input_id — M-3, review finding. The two rows
// get DIFFERENT ID values (report.semantic[].id, distinct from
// report.semantic[].input_id): Report.Validate's own
// requireSortedUnique("report.semantic", ...) enforces uniqueness on ID,
// never on InputID (the real kernel always sets them equal,
// policyconflict/service.go's evaluateSemantic, but the two fields are
// independent on the wire), so a canonical, Validate-passing report CAN
// carry two distinct-ID rows that happen to share one input_id — exactly
// the internally-ambiguous shape this verb must refuse rather than
// silently pick the first of.
func withDuplicateSemanticRow(report *policyconflict.Report) {
	dup := report.Semantic[0]
	dup.ID = dup.ID + "-duplicate"
	report.Semantic = append(report.Semantic, dup)
	sort.Slice(report.Semantic, func(i, j int) bool { return report.Semantic[i].ID < report.Semantic[j].ID })
}

// writeDispositionRecordStoreRoot builds a minimal, real store root (the
// same bare "schema: verdi.layout/v1" manifest cmd/verdi/disposition_test.go's
// own writeDispositionStoreRoot uses) — `disposition record` touches only
// .verdi/policy/dispositions/, never git, never the target spec.
func writeDispositionRecordStoreRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, ".verdi", "verdi.yaml"), []byte("schema: verdi.layout/v1\n"))
	return root
}

// runDispositionRecordBinary execs the built verdi binary's "disposition
// record" verb with args, capturing stdout/stderr separately.
func runDispositionRecordBinary(t *testing.T, bin string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command(bin, append([]string{"disposition", "record"}, args...)...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return outBuf.String(), errBuf.String(), ee.ExitCode()
		}
		t.Fatalf("running verdi disposition record %v: %v", args, err)
	}
	return outBuf.String(), errBuf.String(), 0
}

// dispositionRecordBaseArgs returns a complete, valid argument set against
// root and the fixture report at reportPath — every refusal test in the
// table below starts from a copy of this and mutates exactly one thing, so
// a refusal is provably attributable to that one change.
func dispositionRecordBaseArgs(root, reportPath, id string) []string {
	return []string{
		"--report", reportPath,
		"--row", dispositionRecordFixtureInputID,
		"--target", dispositionRecordFixtureTargetRef,
		"--target-digest", dispositionRecordFixtureAuthorityDigest,
		"--conclusion", "no-conflict",
		"--compensating-control", "Human reviewed manually; no automated judge is configured.",
		"--expiry", "2099-12-31",
		"--approver", dispositionRecordFixtureApproverArg,
		"--id", id,
		"--title", "story-alpha claims coexist without conflict",
		"--owner", "platform-team",
		"--root", root,
	}
}

// TestCmdDispositionRecord_Positive drives the built binary end to end
// (obligation-shaped behavioral proof, mirroring disposition_test.go's own
// runDispositionBinary discipline): given a real policy-conflict report
// and a row selected by input_id, it writes one policy-disposition
// artifact whose witness fields are copied verbatim from the row, whose
// template identity/digest resolve through humanartifact.ResolveScaffold,
// whose origin is human-fallback (the fixture row carries no judgment),
// and whose remaining members come from the operands — then decodes and
// validates the written file through the frozen policyartifact decoder.
func TestCmdDispositionRecord_Positive(t *testing.T) {
	bin := buildVerdiBinary(t)
	root := writeDispositionRecordStoreRoot(t)
	reportPath := filepath.Join(root, "report.json")
	writeTestFile(t, reportPath, dispositionRecordFixtureReport(t))

	args := dispositionRecordBaseArgs(root, reportPath, "story-alpha-no-conflict")
	stdout, stderr, code := runDispositionRecordBinary(t, bin, args...)
	if code != 0 {
		t.Fatalf("verdi disposition record: exit = %d, want 0; stdout=%s stderr=%s", code, stdout, stderr)
	}

	destPath := filepath.Join(root, ".verdi", "policy", "dispositions", "story-alpha-no-conflict.md")
	raw, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("reading written disposition: %v", err)
	}
	d, err := policyartifact.DecodeDisposition(raw)
	if err != nil {
		t.Fatalf("DecodeDisposition on the written artifact: %v", err)
	}
	if err := d.Scope.Validate(); err != nil {
		t.Fatalf("written disposition scope does not validate: %v", err)
	}

	if d.ID != "policy-disposition/story-alpha-no-conflict" {
		t.Fatalf("ID = %q", d.ID)
	}
	if d.Title != "story-alpha claims coexist without conflict" {
		t.Fatalf("Title = %q", d.Title)
	}
	if len(d.Owners) != 1 || d.Owners[0] != "platform-team" {
		t.Fatalf("Owners = %v", d.Owners)
	}
	if d.Witness.InputID != dispositionRecordFixtureInputID {
		t.Fatalf("Witness.InputID = %q, want %q", d.Witness.InputID, dispositionRecordFixtureInputID)
	}
	if d.Witness.TargetDigest != dispositionRecordFixtureAuthorityDigest {
		t.Fatalf("Witness.TargetDigest = %q, want %q", d.Witness.TargetDigest, dispositionRecordFixtureAuthorityDigest)
	}
	if len(d.Witness.Claims) != 2 {
		t.Fatalf("Witness.Claims = %+v, want 2 entries copied verbatim from the report row", d.Witness.Claims)
	}
	if d.Witness.Claims[0].ID != dispositionRecordFixtureClaim1ID || d.Witness.Claims[0].Digest != dispositionRecordFixtureClaim1Digest {
		t.Fatalf("Witness.Claims[0] = %+v, want id/digest matching the report row's first claim", d.Witness.Claims[0])
	}
	if d.Witness.Claims[1].ID != dispositionRecordFixtureClaim2ID || d.Witness.Claims[1].Digest != dispositionRecordFixtureClaim2Digest {
		t.Fatalf("Witness.Claims[1] = %+v, want id/digest matching the report row's second claim", d.Witness.Claims[1])
	}
	if len(d.Witness.Exemptions) != 0 {
		t.Fatalf("Witness.Exemptions = %+v, want empty (the fixture's one mechanical row names none)", d.Witness.Exemptions)
	}
	if d.Conclusion != policyartifact.DispositionNoConflict {
		t.Fatalf("Conclusion = %q, want no-conflict", d.Conclusion)
	}
	if d.Origin != policyartifact.DispositionHumanFallback {
		t.Fatalf("Origin = %q, want human-fallback (the fixture row carries no judgment)", d.Origin)
	}
	if d.Judgment != nil {
		t.Fatalf("Judgment = %+v, want none", d.Judgment)
	}
	if len(d.CompensatingControls) != 1 || d.CompensatingControls[0] != "Human reviewed manually; no automated judge is configured." {
		t.Fatalf("CompensatingControls = %v", d.CompensatingControls)
	}
	wantApproval := policyartifact.Approval{Role: dispositionRecordFixtureApproverRole, Principal: dispositionRecordFixtureApproverID}
	if len(d.Approvals) != 1 || d.Approvals[0] != wantApproval {
		t.Fatalf("Approvals = %+v, want exactly [%+v]", d.Approvals, wantApproval)
	}
	if d.Expiry != "2099-12-31" {
		t.Fatalf("Expiry = %q", d.Expiry)
	}
	if d.Template == nil || d.Template.Identity == "" || d.Template.Digest == "" {
		t.Fatalf("Template = %+v, want a resolved identity/digest", d.Template)
	}
	if !strings.Contains(stdout, "story-alpha-no-conflict") {
		t.Fatalf("stdout = %q, want it to name the written artifact", stdout)
	}
	// I-1 (whole-wave review, design §2.3): on success, one stderr line
	// names the ratified ordering — commit the artifact, run `verdi
	// context project`, commit the regenerated projections, then run the
	// gate — since recording a disposition moves the effective-policy
	// digest the projections embed.
	for _, want := range []string{"next steps", "commit", "verdi context project", "effective-policy digest", "gate"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want it to contain %q (the next-steps line)", stderr, want)
		}
	}
}

// TestCmdDispositionRecord_TargetWithParentClaims is I-1's controller
// round-2 ruling, positive half ("story row with parent claims ⇒ accepted
// with the story's digest"): a row carrying BOTH the target's own claim
// AND a governing parent feature's claim (a DIFFERENT authority digest)
// still succeeds when --target names the target's own ref and
// --target-digest is the target's own digest — exactly the shape round
// 1's dropped category-narrowing heuristic wrongly refused (a story's row
// always carries its own problem/outcome AND its parent's). The written
// witness carries ALL of the row's claims verbatim (the --target/
// --target-digest operands only select and verify which digest counts as
// "the target's own"; they never filter witness.claims itself).
func TestCmdDispositionRecord_TargetWithParentClaims(t *testing.T) {
	bin := buildVerdiBinary(t)
	root := writeDispositionRecordStoreRoot(t)
	reportPath := filepath.Join(root, "report.json")
	writeTestFile(t, reportPath, mutateFixtureReport(t, withParentFeatureClaim))

	args := dispositionRecordBaseArgs(root, reportPath, "target-with-parent")
	stdout, stderr, code := runDispositionRecordBinary(t, bin, args...)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stdout=%s stderr=%s", code, stdout, stderr)
	}

	raw := readFile(t, filepath.Join(root, ".verdi", "policy", "dispositions", "target-with-parent.md"))
	d, err := policyartifact.DecodeDisposition(raw)
	if err != nil {
		t.Fatalf("DecodeDisposition: %v", err)
	}
	if d.Witness.TargetDigest != dispositionRecordFixtureAuthorityDigest {
		t.Fatalf("Witness.TargetDigest = %q, want the target's own %q (never the parent's %q)", d.Witness.TargetDigest, dispositionRecordFixtureAuthorityDigest, dispositionRecordParentFeatureDigest)
	}
	// All three claims (policy + the target's own + the parent's) land in
	// the witness verbatim, sorted — the operand pair only verified WHICH
	// one is the target's, never filtered the witness itself.
	if len(d.Witness.Claims) != 3 {
		t.Fatalf("Witness.Claims = %+v, want all 3 of the row's claims copied verbatim", d.Witness.Claims)
	}
	foundParent := false
	for _, c := range d.Witness.Claims {
		if c.ID == dispositionRecordParentFeatureID {
			foundParent = true
			if c.AuthorityDigest != dispositionRecordParentFeatureDigest {
				t.Fatalf("parent claim AuthorityDigest = %q, want %q (untouched by target selection)", c.AuthorityDigest, dispositionRecordParentFeatureDigest)
			}
		}
	}
	if !foundParent {
		t.Fatalf("Witness.Claims = %+v, want the parent-feature claim %q present verbatim", d.Witness.Claims, dispositionRecordParentFeatureID)
	}
}

// TestCmdDispositionRecord_ExemptionRoundTrips is M-5 (review finding):
// against a fixture variant whose one mechanical row names one applicable
// exemption, the written artifact's witness.exemptions round-trips —
// decodes, is sorted, and equals reconstructApplicableExemptions's own
// aggregation over the same report.
func TestCmdDispositionRecord_ExemptionRoundTrips(t *testing.T) {
	bin := buildVerdiBinary(t)
	root := writeDispositionRecordStoreRoot(t)
	reportPath := filepath.Join(root, "report.json")
	writeTestFile(t, reportPath, mutateFixtureReport(t, withExemption))

	args := dispositionRecordBaseArgs(root, reportPath, "with-exemption")
	stdout, stderr, code := runDispositionRecordBinary(t, bin, args...)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stdout=%s stderr=%s", code, stdout, stderr)
	}

	raw := readFile(t, filepath.Join(root, ".verdi", "policy", "dispositions", "with-exemption.md"))
	d, err := policyartifact.DecodeDisposition(raw)
	if err != nil {
		t.Fatalf("DecodeDisposition: %v", err)
	}
	want := []policyartifact.SemanticExemptionWitness{{ID: dispositionRecordExemptionID, Digest: dispositionRecordExemptionDigest}}
	if len(d.Witness.Exemptions) != len(want) {
		t.Fatalf("Witness.Exemptions = %+v, want %+v", d.Witness.Exemptions, want)
	}
	for i := range want {
		if d.Witness.Exemptions[i] != want[i] {
			t.Fatalf("Witness.Exemptions[%d] = %+v, want %+v", i, d.Witness.Exemptions[i], want[i])
		}
	}
	if !sort.SliceIsSorted(d.Witness.Exemptions, func(i, j int) bool { return d.Witness.Exemptions[i].ID < d.Witness.Exemptions[j].ID }) {
		t.Fatalf("Witness.Exemptions = %+v, not sorted by id", d.Witness.Exemptions)
	}

	// Cross-check against the pure reconstruction function directly, over
	// the SAME report, so this proves the CLI's actual write path used
	// exactly what reconstructApplicableExemptions computes — never a
	// coincidentally-matching hand-typed expectation.
	report, err := policyconflict.DecodeReport(mutateFixtureReport(t, withExemption))
	if err != nil {
		t.Fatalf("re-decoding the fixture variant: %v", err)
	}
	reconstructed, err := reconstructApplicableExemptions(report.Mechanical)
	if err != nil {
		t.Fatalf("reconstructApplicableExemptions: %v", err)
	}
	if len(reconstructed) != len(d.Witness.Exemptions) {
		t.Fatalf("reconstructApplicableExemptions = %+v, want it to match the written witness %+v", reconstructed, d.Witness.Exemptions)
	}
	for i := range reconstructed {
		if reconstructed[i].ID != d.Witness.Exemptions[i].ID || reconstructed[i].Digest != d.Witness.Exemptions[i].Digest {
			t.Fatalf("reconstructApplicableExemptions[%d] = %+v, want it to match the written witness[%d] %+v", i, reconstructed[i], i, d.Witness.Exemptions[i])
		}
	}
}

// TestCmdDispositionRecord_MultipleRepeatables proves --compensating-control,
// --approver, and --owner each accept more than one occurrence and every
// occurrence lands in the written artifact.
func TestCmdDispositionRecord_MultipleRepeatables(t *testing.T) {
	bin := buildVerdiBinary(t)
	root := writeDispositionRecordStoreRoot(t)
	reportPath := filepath.Join(root, "report.json")
	writeTestFile(t, reportPath, dispositionRecordFixtureReport(t))

	args := []string{
		"--report", reportPath,
		"--row", dispositionRecordFixtureInputID,
		"--target", dispositionRecordFixtureTargetRef,
		"--target-digest", dispositionRecordFixtureAuthorityDigest,
		"--conclusion", "no-conflict",
		"--compensating-control", "First control.",
		"--compensating-control", "Second control.",
		"--expiry", "2099-12-31",
		"--approver", "policy-owner=principal/github-org/YWxpY2U",
		"--approver", "security-owner=principal/github-org/Ym9i",
		"--id", "multi-repeatable",
		"--title", "Multi repeatable",
		"--owner", "platform-team",
		"--owner", "security-team",
		"--root", root,
	}
	stdout, stderr, code := runDispositionRecordBinary(t, bin, args...)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stdout=%s stderr=%s", code, stdout, stderr)
	}
	raw := readFile(t, filepath.Join(root, ".verdi", "policy", "dispositions", "multi-repeatable.md"))
	d, err := policyartifact.DecodeDisposition(raw)
	if err != nil {
		t.Fatalf("DecodeDisposition: %v", err)
	}
	if len(d.Owners) != 2 {
		t.Fatalf("Owners = %v, want 2", d.Owners)
	}
	if len(d.CompensatingControls) != 2 {
		t.Fatalf("CompensatingControls = %v, want 2", d.CompensatingControls)
	}
	if len(d.Approvals) != 2 {
		t.Fatalf("Approvals = %+v, want 2", d.Approvals)
	}
}

// TestCmdDispositionRecord_Refusals is table-driven over every named
// refusal (Task 3 contract): each is an operational exit (2), names the
// offending operand, and never writes a file.
func TestCmdDispositionRecord_Refusals(t *testing.T) {
	bin := buildVerdiBinary(t)

	// mutateReport returns the fixture bytes with fn applied — used by the
	// non-canonical-report case.
	mutateReport := func(t *testing.T, fn func([]byte) []byte) string {
		t.Helper()
		root := t.TempDir()
		path := filepath.Join(root, "report.json")
		writeTestFile(t, path, fn(dispositionRecordFixtureReport(t)))
		return path
	}

	tests := []struct {
		name       string
		setupRoot  func(t *testing.T) (root, reportPath string)
		mutateArgs func(root, reportPath string) []string
		wantSub    string
	}{
		{
			name: "report unreadable",
			setupRoot: func(t *testing.T) (string, string) {
				return writeDispositionRecordStoreRoot(t), filepath.Join(t.TempDir(), "does-not-exist.json")
			},
			mutateArgs: func(root, reportPath string) []string {
				return dispositionRecordBaseArgs(root, reportPath, "x")
			},
			wantSub: "--report",
		},
		{
			name: "report non-canonical",
			setupRoot: func(t *testing.T) (string, string) {
				root := writeDispositionRecordStoreRoot(t)
				path := mutateReport(t, func(b []byte) []byte { return append(append([]byte(nil), b...), '\n', '\n') })
				return root, path
			},
			mutateArgs: func(root, reportPath string) []string {
				return dispositionRecordBaseArgs(root, reportPath, "x")
			},
			wantSub: "--report",
		},
		{
			name: "row not found",
			setupRoot: func(t *testing.T) (string, string) {
				root := writeDispositionRecordStoreRoot(t)
				path := filepath.Join(root, "report.json")
				writeTestFile(t, path, dispositionRecordFixtureReport(t))
				return root, path
			},
			mutateArgs: func(root, reportPath string) []string {
				args := dispositionRecordBaseArgs(root, reportPath, "x")
				return replaceArgValue(args, "--row", "sha256:"+strings.Repeat("0", 64))
			},
			wantSub: "sha256:" + strings.Repeat("0", 64),
		},
		{
			name: "conclusion outside the closed set",
			setupRoot: func(t *testing.T) (string, string) {
				root := writeDispositionRecordStoreRoot(t)
				path := filepath.Join(root, "report.json")
				writeTestFile(t, path, dispositionRecordFixtureReport(t))
				return root, path
			},
			mutateArgs: func(root, reportPath string) []string {
				args := dispositionRecordBaseArgs(root, reportPath, "x")
				return replaceArgValue(args, "--conclusion", "maybe")
			},
			wantSub: "conclusion",
		},
		{
			name: "no compensating control",
			setupRoot: func(t *testing.T) (string, string) {
				root := writeDispositionRecordStoreRoot(t)
				path := filepath.Join(root, "report.json")
				writeTestFile(t, path, dispositionRecordFixtureReport(t))
				return root, path
			},
			mutateArgs: func(root, reportPath string) []string {
				return removeArg(dispositionRecordBaseArgs(root, reportPath, "x"), "--compensating-control")
			},
			wantSub: "compensating-control",
		},
		{
			name: "no approver",
			setupRoot: func(t *testing.T) (string, string) {
				root := writeDispositionRecordStoreRoot(t)
				path := filepath.Join(root, "report.json")
				writeTestFile(t, path, dispositionRecordFixtureReport(t))
				return root, path
			},
			mutateArgs: func(root, reportPath string) []string {
				return removeArg(dispositionRecordBaseArgs(root, reportPath, "x"), "--approver")
			},
			wantSub: "approver",
		},
		{
			name: "malformed approver: no equals sign",
			setupRoot: func(t *testing.T) (string, string) {
				root := writeDispositionRecordStoreRoot(t)
				path := filepath.Join(root, "report.json")
				writeTestFile(t, path, dispositionRecordFixtureReport(t))
				return root, path
			},
			mutateArgs: func(root, reportPath string) []string {
				args := dispositionRecordBaseArgs(root, reportPath, "x")
				return replaceArgValue(args, "--approver", "policy-owner-no-equals")
			},
			wantSub: "approver",
		},
		{
			name: "malformed approver: invalid principal",
			setupRoot: func(t *testing.T) (string, string) {
				root := writeDispositionRecordStoreRoot(t)
				path := filepath.Join(root, "report.json")
				writeTestFile(t, path, dispositionRecordFixtureReport(t))
				return root, path
			},
			mutateArgs: func(root, reportPath string) []string {
				args := dispositionRecordBaseArgs(root, reportPath, "x")
				return replaceArgValue(args, "--approver", "policy-owner=not-a-principal")
			},
			wantSub: "approver",
		},
		{
			name: "malformed expiry",
			setupRoot: func(t *testing.T) (string, string) {
				root := writeDispositionRecordStoreRoot(t)
				path := filepath.Join(root, "report.json")
				writeTestFile(t, path, dispositionRecordFixtureReport(t))
				return root, path
			},
			mutateArgs: func(root, reportPath string) []string {
				args := dispositionRecordBaseArgs(root, reportPath, "x")
				return replaceArgValue(args, "--expiry", "not-a-date")
			},
			wantSub: "expiry",
		},
		{
			name: "existing artifact at the id refuses to overwrite",
			setupRoot: func(t *testing.T) (string, string) {
				root := writeDispositionRecordStoreRoot(t)
				path := filepath.Join(root, "report.json")
				writeTestFile(t, path, dispositionRecordFixtureReport(t))
				dest := filepath.Join(root, ".verdi", "policy", "dispositions", "already-exists.md")
				writeTestFile(t, dest, []byte("pre-existing content\n"))
				return root, path
			},
			mutateArgs: func(root, reportPath string) []string {
				return dispositionRecordBaseArgs(root, reportPath, "already-exists")
			},
			wantSub: "already-exists",
		},
		{
			name: "operand makes the witness differ from the report: target-digest matches no claim",
			setupRoot: func(t *testing.T) (string, string) {
				root := writeDispositionRecordStoreRoot(t)
				path := filepath.Join(root, "report.json")
				writeTestFile(t, path, dispositionRecordFixtureReport(t))
				return root, path
			},
			mutateArgs: func(root, reportPath string) []string {
				args := dispositionRecordBaseArgs(root, reportPath, "x")
				return replaceArgValue(args, "--target-digest", "sha256:"+strings.Repeat("f", 64))
			},
			wantSub: "target-digest",
		},
		{
			name: "target-digest: a parent-feature claim's digest is refused even with the target explicitly named (I-1, controller round 2)",
			setupRoot: func(t *testing.T) (string, string) {
				root := writeDispositionRecordStoreRoot(t)
				path := filepath.Join(root, "report.json")
				writeTestFile(t, path, mutateFixtureReport(t, withParentFeatureClaim))
				return root, path
			},
			mutateArgs: func(root, reportPath string) []string {
				args := dispositionRecordBaseArgs(root, reportPath, "x")
				return replaceArgValue(args, "--target-digest", dispositionRecordParentFeatureDigest)
			},
			wantSub: "does not match",
		},
		{
			name: "target: an unknown ref (no claim carries it at all) is refused",
			setupRoot: func(t *testing.T) (string, string) {
				root := writeDispositionRecordStoreRoot(t)
				path := filepath.Join(root, "report.json")
				writeTestFile(t, path, dispositionRecordFixtureReport(t))
				return root, path
			},
			mutateArgs: func(root, reportPath string) []string {
				args := dispositionRecordBaseArgs(root, reportPath, "x")
				return replaceArgValue(args, "--target", "spec/no-such-ref-in-this-report")
			},
			wantSub: "--target",
		},
		{
			name: "target-digest: a policy-instruction claim's digest is refused even though it is a real digest in the row (I-1)",
			setupRoot: func(t *testing.T) (string, string) {
				root := writeDispositionRecordStoreRoot(t)
				path := filepath.Join(root, "report.json")
				writeTestFile(t, path, mutateFixtureReport(t, withDistinctPolicyDigest))
				return root, path
			},
			mutateArgs: func(root, reportPath string) []string {
				args := dispositionRecordBaseArgs(root, reportPath, "x")
				return replaceArgValue(args, "--target-digest", dispositionRecordDistinctPolicyDigest)
			},
			wantSub: "does not match",
		},
		{
			name: "target-digest: a report whose target contributes no claim is refused (I-1)",
			setupRoot: func(t *testing.T) (string, string) {
				root := writeDispositionRecordStoreRoot(t)
				path := filepath.Join(root, "report.json")
				writeTestFile(t, path, mutateFixtureReport(t, withOnlyPolicyInstructionClaim))
				return root, path
			},
			mutateArgs: func(root, reportPath string) []string {
				return dispositionRecordBaseArgs(root, reportPath, "x")
			},
			wantSub: "the target contributes no claim to this row",
		},
		{
			name: "target: acceptance-candidate arm mismatch is refused",
			setupRoot: func(t *testing.T) (string, string) {
				root := writeDispositionRecordStoreRoot(t)
				path := filepath.Join(root, "report.json")
				writeTestFile(t, path, mutateFixtureReport(t, withAcceptanceCandidateTarget))
				return root, path
			},
			mutateArgs: func(root, reportPath string) []string {
				args := dispositionRecordBaseArgs(root, reportPath, "x")
				return replaceArgValue(args, "--target", "spec/some-other-ref")
			},
			wantSub: "--target",
		},
		{
			name: "malformed --id: not a single kebab-case path component (M-2)",
			setupRoot: func(t *testing.T) (string, string) {
				root := writeDispositionRecordStoreRoot(t)
				path := filepath.Join(root, "report.json")
				writeTestFile(t, path, dispositionRecordFixtureReport(t))
				return root, path
			},
			mutateArgs: func(root, reportPath string) []string {
				return dispositionRecordBaseArgs(root, reportPath, "../escape")
			},
			wantSub: "--id",
		},
		{
			name: "duplicate semantic rows sharing input_id are refused (M-3)",
			setupRoot: func(t *testing.T) (string, string) {
				root := writeDispositionRecordStoreRoot(t)
				path := filepath.Join(root, "report.json")
				writeTestFile(t, path, mutateFixtureReport(t, withDuplicateSemanticRow))
				return root, path
			},
			mutateArgs: func(root, reportPath string) []string {
				return dispositionRecordBaseArgs(root, reportPath, "x")
			},
			wantSub: "--row",
		},
		{
			name: "unknown flag",
			setupRoot: func(t *testing.T) (string, string) {
				root := writeDispositionRecordStoreRoot(t)
				path := filepath.Join(root, "report.json")
				writeTestFile(t, path, dispositionRecordFixtureReport(t))
				return root, path
			},
			mutateArgs: func(root, reportPath string) []string {
				return append(dispositionRecordBaseArgs(root, reportPath, "x"), "--bogus-flag", "z")
			},
			wantSub: "bogus-flag",
		},
		{
			// I-3, whole-wave review: --root resolves through store.RootAt
			// (exact directory, no ancestor search) exactly like `context
			// project --root`, never store.FindRoot's ancestor walk — a
			// subdirectory of a real store must be refused by name, not
			// silently accepted by walking up to find the store above it.
			name: "a subdirectory of a real store is refused (I-3: --root is exact-directory, no ancestor search)",
			setupRoot: func(t *testing.T) (string, string) {
				root := writeDispositionRecordStoreRoot(t)
				path := filepath.Join(root, "report.json")
				writeTestFile(t, path, dispositionRecordFixtureReport(t))
				sub := filepath.Join(root, "nested")
				if err := os.MkdirAll(sub, 0o755); err != nil {
					t.Fatal(err)
				}
				return root, path
			},
			mutateArgs: func(root, reportPath string) []string {
				args := dispositionRecordBaseArgs(root, reportPath, "x")
				return replaceArgValue(args, "--root", filepath.Join(root, "nested"))
			},
			wantSub: "--root",
		},
		{
			name: "missing --report entirely",
			setupRoot: func(t *testing.T) (string, string) {
				return writeDispositionRecordStoreRoot(t), ""
			},
			mutateArgs: func(root, reportPath string) []string {
				args := dispositionRecordBaseArgs(root, "unused.json", "x")
				return removeArg(args, "--report")
			},
			wantSub: "report",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root, reportPath := tc.setupRoot(t)
			destDir := filepath.Join(root, ".verdi", "policy", "dispositions")
			before := snapshotDir(t, destDir)

			args := tc.mutateArgs(root, reportPath)
			_, stderr, code := runDispositionRecordBinary(t, bin, args...)
			if code != 2 {
				t.Fatalf("exit = %d, want 2 (operational); stderr=%s", code, stderr)
			}
			if !strings.Contains(stderr, tc.wantSub) {
				t.Fatalf("stderr = %q, want it to contain %q", stderr, tc.wantSub)
			}
			after := snapshotDir(t, destDir)
			if strings.Join(before, "\x00") != strings.Join(after, "\x00") {
				t.Fatalf("dispositions directory changed despite a refusal:\nbefore=%v\nafter=%v", before, after)
			}
		})
	}
}

// replaceArgValue returns a copy of args with flag's value (the token
// immediately following the LAST occurrence of flag) replaced by value —
// every base-args table case above uses exactly one occurrence per flag,
// except --approver/--owner/--compensating-control, which this helper is
// never used to target.
func replaceArgValue(args []string, flag, value string) []string {
	out := append([]string(nil), args...)
	for i := len(out) - 2; i >= 0; i-- {
		if out[i] == flag {
			out[i+1] = value
			return out
		}
	}
	return out
}

// removeArg returns a copy of args with the first occurrence of flag and
// its following value removed entirely.
func removeArg(args []string, flag string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		if args[i] == flag {
			i++ // also skip its value
			continue
		}
		out = append(out, args[i])
	}
	return out
}

// snapshotDir returns the sorted names of every file directly under dir
// (which may not exist, yielding nil) — used to prove a refusal writes
// nothing.
func snapshotDir(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("reading dir %s: %v", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// --- I-2 (review finding): kernel round-trip via ResolveDispositionAuthority ---
//
// BLOCKED, reported per the finding's own instruction ("say so and stop
// rather than exporting kernel internals"). Two independent, compounding
// reasons a passing policyconflict.SemanticInput cannot be built from
// report-only data outside internal/policyconflict:
//
//  1. SemanticInput.Claims is typed []contextcompile.ProseClaim
//     (semantic.go:60) — NOT []policyartifact.SemanticClaimWitness, the
//     type Report.Semantic[].Claims actually carries. This is not merely
//     an unexported-symbol problem: ProseClaim carries authored Text and
//     per-source identity (SourceRef/SourcePath/SourceDigest/Object/
//     LineIdentity) that SemanticClaimWitness never retains BY DESIGN
//     (SemanticClaimWitness's own doc comment, policyartifact/
//     disposition.go: "It never carries the claim's authored text").
//     There is no lossless conversion in either direction; fabricating a
//     ProseClaim's Text/SourceRef to satisfy the type would be inventing
//     content the report never recorded, not reconstructing it.
//  2. SemanticInput.Prompt must byte-equal the unexported package constant
//     semanticPrompt (semantic.go:39), enforced by validateSemanticInput
//     (semantic.go:119, unexported), which ResolveDispositionAuthority
//     calls internally (authority.go:569-580). The only exported
//     constructor, BuildSemanticInput (semantic.go:98), requires a live
//     contextcompile.ConflictView — a sealed, in-memory evaluation view,
//     not reconstructable from a persisted Report.
//
// AuthorityInput (authority.go:58) itself has no such blocker — every
// field (EvaluatedOn, TargetDigest, Profile, Actors, Exemptions,
// Dispositions) is an exported type constructible from outside the
// package. The blocker is entirely on the SemanticInput side.
//
// Not worked around: no copy of the private prompt text was hardcoded
// into this test file, and internal/policyconflict was not touched to
// export either seam (both would route around, not use, the kernel's own
// authority — and the second is explicitly outside this fix's write set).
//
// What IS already proven instead (the closest available substitute,
// clearly distinct from the literal ask): resolveDisposition's match/
// freshness/scope predicate (authority.go's resolveDisposition) is exactly
// four equality checks — d.Witness.InputID == currentDigest,
// d.Witness.TargetDigest == in.TargetDigest, normalized claim identities
// equal, normalized exemption identities equal. TestCmdDispositionRecord_
// Positive already proves the first three hold byte-for-byte against the
// same report (Witness.InputID == row.InputID; Witness.TargetDigest ==
// the operand, itself proven equal to the target's own claim digest by
// the I-1 tests below; Witness.Claims == row.Claims verbatim, in order),
// and TestCmdDispositionRecord_ExemptionRoundTrips proves the fourth
// (Witness.Exemptions == reconstructApplicableExemptions(report.
// Mechanical)). This is NOT a call to ResolveDispositionAuthority and does
// not exercise Authorization/Bound (which need a real Profile/Actors this
// investigation never needed to construct) — it is the strongest evidence
// available without the two blocked constructors above.

// --- direct unit tests for disposition_record.go's pure helper functions ---

func TestDispositionOrigin(t *testing.T) {
	tests := []struct {
		name string
		row  policyconflict.SemanticEvaluation
		want string
	}{
		{"no primary judgment: human-fallback", policyconflict.SemanticEvaluation{}, string(policyartifact.DispositionHumanFallback)},
		{"primary judgment present: judge-result", policyconflict.SemanticEvaluation{Primary: &policyconflict.JudgmentExchange{}}, string(policyartifact.DispositionJudgeResult)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := dispositionOrigin(tc.row); got != tc.want {
				t.Fatalf("dispositionOrigin() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFindSemanticRow(t *testing.T) {
	t.Run("found: unique match", func(t *testing.T) {
		report := policyconflict.Report{Semantic: []policyconflict.SemanticEvaluation{
			{InputID: "sha256:aaaa"},
			{InputID: "sha256:bbbb"},
		}}
		row, matches := findSemanticRow(report, "sha256:bbbb")
		if matches != 1 || row.InputID != "sha256:bbbb" {
			t.Fatalf("findSemanticRow = %+v, %d, want 1 match on sha256:bbbb", row, matches)
		}
	})
	t.Run("not found", func(t *testing.T) {
		report := policyconflict.Report{Semantic: []policyconflict.SemanticEvaluation{{InputID: "sha256:aaaa"}}}
		_, matches := findSemanticRow(report, "sha256:cccc")
		if matches != 0 {
			t.Fatalf("findSemanticRow matches = %d, want 0", matches)
		}
	})
	t.Run("duplicate: two rows share one input_id (M-3)", func(t *testing.T) {
		report := policyconflict.Report{Semantic: []policyconflict.SemanticEvaluation{
			{ID: "first", InputID: "sha256:dupe"},
			{ID: "second", InputID: "sha256:dupe"},
		}}
		row, matches := findSemanticRow(report, "sha256:dupe")
		if matches != 2 {
			t.Fatalf("findSemanticRow matches = %d, want 2", matches)
		}
		if row.ID != "first" {
			t.Fatalf("findSemanticRow row = %+v, want the first match (caller must reject on matches>1 before using it)", row)
		}
	})
}

func TestTargetRefFromReport(t *testing.T) {
	tests := []struct {
		name    string
		report  policyconflict.Report
		wantRef string
		wantOK  bool
	}{
		{
			"acceptance-candidate arm: ref is present",
			policyconflict.Report{Input: policyconflict.InputIdentity{Target: policyconflict.TargetIdentity{
				Kind:      policyconflict.TargetAcceptanceCandidate,
				Candidate: &policyconflict.CandidateIdentity{Ref: "spec/story-alpha"},
			}}},
			"spec/story-alpha", true,
		},
		{
			"accepted-context arm: no ref available (only the manifest digest)",
			policyconflict.Report{Input: policyconflict.InputIdentity{Target: policyconflict.TargetIdentity{
				Kind:     policyconflict.TargetAcceptedContext,
				Accepted: &policyconflict.AcceptedIdentity{ManifestDigest: "sha256:" + strings.Repeat("1", 64)},
			}}},
			"", false,
		},
		{
			"candidate identity present but ref empty: still not usable",
			policyconflict.Report{Input: policyconflict.InputIdentity{Target: policyconflict.TargetIdentity{
				Kind:      policyconflict.TargetAcceptanceCandidate,
				Candidate: &policyconflict.CandidateIdentity{Ref: ""},
			}}},
			"", false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ref, ok := targetRefFromReport(tc.report)
			if ref != tc.wantRef || ok != tc.wantOK {
				t.Fatalf("targetRefFromReport() = %q, %v, want %q, %v", ref, ok, tc.wantRef, tc.wantOK)
			}
		})
	}
}

func TestTargetClaimAuthorityDigests(t *testing.T) {
	digestA := "sha256:" + strings.Repeat("1", 64) // the target's own
	digestB := "sha256:" + strings.Repeat("2", 64) // a governing parent feature's
	digestC := "sha256:" + strings.Repeat("3", 64) // a policy's

	tests := []struct {
		name      string
		claims    []policyartifact.SemanticClaimWitness
		targetRef string
		want      []string
	}{
		{
			name: "exact ref match excludes policy/parent claims even when categories overlap",
			claims: []policyartifact.SemanticClaimWitness{
				{ID: "policy/p#instruction-1", Category: "policy-instruction", AuthorityDigest: digestC},
				{ID: "spec/parent-feature#outcome", Category: "spec-outcome", AuthorityDigest: digestB},
				{ID: "spec/story-alpha#ac-1", Category: "acceptance-criterion", AuthorityDigest: digestA},
			},
			targetRef: "spec/story-alpha",
			want:      []string{digestA},
		},
		{
			name: "the target's row ALSO carries its governing parent's claims (the real spike/story shape): still narrows to the target's own digest alone",
			claims: []policyartifact.SemanticClaimWitness{
				{ID: "spec/parent-feature#outcome", Category: "spec-outcome", AuthorityDigest: digestB},
				{ID: "spec/parent-feature#problem", Category: "spec-problem", AuthorityDigest: digestB},
				{ID: "spec/story-alpha#ac-1", Category: "acceptance-criterion", AuthorityDigest: digestA},
				{ID: "spec/story-alpha#outcome", Category: "spec-outcome", AuthorityDigest: digestA},
				{ID: "spec/story-alpha#problem", Category: "spec-problem", AuthorityDigest: digestA},
			},
			targetRef: "spec/story-alpha",
			want:      []string{digestA},
		},
		{
			name: "no claim matches the target ref",
			claims: []policyartifact.SemanticClaimWitness{
				{ID: "spec/parent-feature#outcome", Category: "spec-outcome", AuthorityDigest: digestB},
			},
			targetRef: "spec/story-alpha",
			want:      []string{},
		},
		{
			name: "obligation-declaration id (no '#') never matches a spec ref",
			claims: []policyartifact.SemanticClaimWitness{
				{ID: "obligation/story-alpha--ac-1--behavioral", Category: "obligation-declaration", AuthorityDigest: digestC},
			},
			targetRef: "spec/story-alpha",
			want:      []string{},
		},
		{
			name: "the target's own claims disagreeing with each other is a genuine inconsistency: both digests remain (caller must refuse on len>1)",
			claims: []policyartifact.SemanticClaimWitness{
				{ID: "spec/story-alpha#ac-1", Category: "acceptance-criterion", AuthorityDigest: digestA},
				{ID: "spec/story-alpha#outcome", Category: "spec-outcome", AuthorityDigest: digestB},
			},
			targetRef: "spec/story-alpha",
			want:      []string{digestA, digestB}, // sorted
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := targetClaimAuthorityDigests(tc.claims, tc.targetRef)
			if len(got) != len(tc.want) {
				t.Fatalf("targetClaimAuthorityDigests() = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("targetClaimAuthorityDigests() = %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestValidDispositionID(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{"story-alpha-no-conflict", true},
		{"a", true},
		{"a1-b2", true},
		{"", false},
		{"Story-Alpha", false},
		{"story_alpha", false},
		{"../escape", false},
		{"a/b", false},
		{".", false},
		{"..", false},
		{"-leading-hyphen", false},
		{"trailing-hyphen-", false},
		{"double--hyphen", false},
	}
	for _, tc := range tests {
		t.Run(tc.id, func(t *testing.T) {
			if got := validDispositionID(tc.id); got != tc.want {
				t.Fatalf("validDispositionID(%q) = %v, want %v", tc.id, got, tc.want)
			}
		})
	}
}

func TestReconstructApplicableExemptions(t *testing.T) {
	tests := []struct {
		name       string
		mechanical []policyconflict.MechanicalEvaluation
		want       []dispositionExemption
		wantErr    bool
	}{
		{"no mechanical rows", nil, nil, false},
		{"rows with no exemptions", []policyconflict.MechanicalEvaluation{{ID: "m1"}}, nil, false},
		{
			"one exemption on one row",
			[]policyconflict.MechanicalEvaluation{{ID: "m1", Exemptions: []policyconflict.ExemptionResolution{{ID: "policy-exemption/e1", Digest: "sha256:" + strings.Repeat("1", 64)}}}},
			[]dispositionExemption{{ID: "policy-exemption/e1", Digest: "sha256:" + strings.Repeat("1", 64)}},
			false,
		},
		{
			"same exemption on two rows dedups and sorts by id",
			[]policyconflict.MechanicalEvaluation{
				{ID: "m2", Exemptions: []policyconflict.ExemptionResolution{{ID: "policy-exemption/e2", Digest: "sha256:" + strings.Repeat("2", 64)}}},
				{ID: "m1", Exemptions: []policyconflict.ExemptionResolution{
					{ID: "policy-exemption/e1", Digest: "sha256:" + strings.Repeat("1", 64)},
					{ID: "policy-exemption/e2", Digest: "sha256:" + strings.Repeat("2", 64)},
				}},
			},
			[]dispositionExemption{
				{ID: "policy-exemption/e1", Digest: "sha256:" + strings.Repeat("1", 64)},
				{ID: "policy-exemption/e2", Digest: "sha256:" + strings.Repeat("2", 64)},
			},
			false,
		},
		{
			"same exemption id with two different digests is an inconsistent report",
			[]policyconflict.MechanicalEvaluation{
				{ID: "m1", Exemptions: []policyconflict.ExemptionResolution{{ID: "policy-exemption/e1", Digest: "sha256:" + strings.Repeat("1", 64)}}},
				{ID: "m2", Exemptions: []policyconflict.ExemptionResolution{{ID: "policy-exemption/e1", Digest: "sha256:" + strings.Repeat("2", 64)}}},
			},
			nil,
			true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := reconstructApplicableExemptions(tc.mechanical)
			if tc.wantErr {
				if err == nil {
					t.Fatal("want an error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("reconstructApplicableExemptions() = %+v, want %+v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("reconstructApplicableExemptions()[%d] = %+v, want %+v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestParseApprover(t *testing.T) {
	tests := []struct {
		name       string
		in         string
		wantRole   string
		wantPrinID string
		wantErr    bool
	}{
		{"valid", "policy-owner=principal/github-org/YWxpY2U", "policy-owner", "principal/github-org/YWxpY2U", false},
		{"no equals sign", "policy-owner-no-equals", "", "", true},
		{"empty role", "=principal/github-org/YWxpY2U", "", "", true},
		{"empty principal", "policy-owner=", "", "", true},
		{"malformed principal", "policy-owner=not-a-principal", "", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			role, principal, err := parseApprover(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseApprover(%q): want an error, got role=%q principal=%q", tc.in, role, principal)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseApprover(%q): unexpected error: %v", tc.in, err)
			}
			if role != tc.wantRole || principal != tc.wantPrinID {
				t.Fatalf("parseApprover(%q) = %q, %q, want %q, %q", tc.in, role, principal, tc.wantRole, tc.wantPrinID)
			}
		})
	}
}
