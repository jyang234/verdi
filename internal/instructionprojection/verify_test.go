package instructionprojection

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jyang234/verdi/internal/policyauthority"
)

// TestVerify_ErrNotAdopted mirrors Generate's own legacy-store behavior:
// no .verdi/policy/ means ErrNotAdopted, unchanged, and Verify claims
// nothing.
func TestVerify_ErrNotAdopted(t *testing.T) {
	root := t.TempDir()
	report, err := Verify(root)
	if !errors.Is(err, policyauthority.ErrNotAdopted) {
		t.Fatalf("Verify() error = %v, want errors.Is(err, ErrNotAdopted)", err)
	}
	if report != nil {
		t.Fatalf("Verify() report = %+v, want nil on ErrNotAdopted", report)
	}
}

// TestVerify_ZeroAdapters_Clean is the explicit "a constitution with
// zero adapters ... verifies clean" case.
func TestVerify_ZeroAdapters_Clean(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, zeroAdapterStoreFiles())

	report, err := Verify(root)
	if err != nil {
		t.Fatalf("Verify() unexpected error: %v", err)
	}
	if !report.Clean() {
		t.Fatalf("Verify() report = %+v, want Clean()", report.Findings)
	}
}

// --- The confirmed policyauthority conflict --------------------------
//
// generate.go's projectionsDirRel doc explains why: writing
// .verdi/policy/projections/<id>.json is this package's OWN contracted
// path, but it also extends .verdi/policy/'s directory grammar beyond
// what internal/policyauthority.Load currently recognizes. The
// following test is the mechanical witness of that conflict — proven by
// experiment, not asserted from a hunch — kept in the suite so CI itself
// carries the disclosure rather than only this build's report.

// TestGenerateThenPublicVerify_BlockedByPolicyAuthorityWalker documents,
// as a passing (GREEN) test of ACTUAL current behavior, that the public
// Verify(root) cannot complete after Generate(root) has run on the same
// root: policyauthority.Load rejects the now-existing
// .verdi/policy/projections/ directory as unrecognized. This is a real,
// reachable production limitation of the contracted manifest path, not
// a testing artifact — see this lane's final report for the disclosed
// conflict and the fix it needs (extending internal/policyauthority's
// directory grammar, which is out of this package's write set).
func TestGenerateThenPublicVerify_BlockedByPolicyAuthorityWalker(t *testing.T) {
	root := newFixtureRoot(t)

	if _, err := Generate(root); err != nil {
		t.Fatalf("Generate() unexpected error: %v", err)
	}

	_, err := Verify(root)
	if err == nil {
		t.Fatal("Verify() after Generate() unexpectedly succeeded; the policyauthority walker conflict this test documents may have been fixed — replace this test with a real round-trip-clean assertion if so")
	}
	if errors.Is(err, policyauthority.ErrNotAdopted) {
		t.Fatalf("Verify() error = %v, want the walker's unrecognized-directory error, not ErrNotAdopted", err)
	}
}

// TestGenerateThenVerifyCore_RoundTripClean is the REAL round-trip-clean
// proof the contract requires, exercised at the layer this package
// actually controls: verify's store-agnostic core, fed the SAME
// EffectivePolicy Generate itself resolved (captured before Generate
// wrote anything, so it is byte-identical to what Generate used — nothing
// about the authority changed by writing its own projection). This is
// the only way to prove "Generate then Verify is clean" without a second
// policyauthority.Load call on a root Generate has already touched (see
// the conflict test above).
func TestGenerateThenVerifyCore_RoundTripClean(t *testing.T) {
	root := newFixtureRoot(t)
	store, ep := loadResolve(t, root)

	if _, err := Generate(root); err != nil {
		t.Fatalf("Generate() unexpected error: %v", err)
	}

	report, err := verify(root, store.Constitution, store.Policies, ep)
	if err != nil {
		t.Fatalf("verify() unexpected error: %v", err)
	}
	if !report.Clean() {
		t.Fatalf("verify() after Generate() = %+v, want Clean()", report.Findings)
	}
}

// TestVerify_Drift edits one managed file after Generate and expects
// exactly a drift finding on that path, with digests witnessing the
// mismatch, and the other managed file for the same adapter staying
// clean.
func TestVerify_Drift(t *testing.T) {
	root := newFixtureRoot(t)
	store, ep := loadResolve(t, root)
	if _, err := Generate(root); err != nil {
		t.Fatalf("Generate(): %v", err)
	}

	agentsPath := filepath.Join(root, "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte("this is not the generated content at all\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := verify(root, store.Constitution, store.Policies, ep)
	if err != nil {
		t.Fatalf("verify(): %v", err)
	}
	f := findOne(t, report, "codex", ReasonDrift, "AGENTS.md")
	if f.Expected == "" || f.Actual == "" || f.Expected == f.Actual {
		t.Fatalf("drift finding missing usable digests: %+v", f)
	}
	if hasFinding(report, "codex", ReasonDrift, "docs/AGENTS.md") || hasFinding(report, "codex", ReasonTruncated, "docs/AGENTS.md") {
		t.Fatalf("the untouched managed file must stay clean: %+v", report.Findings)
	}
}

// TestVerify_Truncated writes a PROPER PREFIX of the regenerated content
// and expects the truncated subclass, not plain drift.
func TestVerify_Truncated(t *testing.T) {
	root := newFixtureRoot(t)
	store, ep := loadResolve(t, root)
	if _, err := Generate(root); err != nil {
		t.Fatalf("Generate(): %v", err)
	}

	full := filepath.Join(root, "AGENTS.md")
	original, err := os.ReadFile(full)
	if err != nil {
		t.Fatal(err)
	}
	if len(original) < 10 {
		t.Fatalf("fixture-generated AGENTS.md too short to truncate meaningfully: %d bytes", len(original))
	}
	if err := os.WriteFile(full, original[:len(original)/2], 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := verify(root, store.Constitution, store.Policies, ep)
	if err != nil {
		t.Fatalf("verify(): %v", err)
	}
	findOne(t, report, "codex", ReasonTruncated, "AGENTS.md")
	if hasFinding(report, "codex", ReasonDrift, "AGENTS.md") {
		t.Fatalf("a proper-prefix truncation must classify as truncated, not drift: %+v", report.Findings)
	}
}

// TestVerify_MissingManagedFile deletes a managed file after Generate.
func TestVerify_MissingManagedFile(t *testing.T) {
	root := newFixtureRoot(t)
	store, ep := loadResolve(t, root)
	if _, err := Generate(root); err != nil {
		t.Fatalf("Generate(): %v", err)
	}
	if err := os.Remove(filepath.Join(root, "docs", "AGENTS.md")); err != nil {
		t.Fatal(err)
	}

	report, err := verify(root, store.Constitution, store.Policies, ep)
	if err != nil {
		t.Fatalf("verify(): %v", err)
	}
	findOne(t, report, "codex", ReasonMissing, "docs/AGENTS.md")
}

// TestVerify_MissingManifest deletes the manifest entirely (no
// .verdi/policy/projections/ directory left at all — so the missing-
// manifest scenario is exercised WITHOUT recreating the policyauthority
// conflict for THIS test's own purposes; the conflict-witness test above
// covers the case where the directory does exist).
func TestVerify_MissingManifest(t *testing.T) {
	root := newFixtureRoot(t)
	store, ep := loadResolve(t, root)
	if _, err := Generate(root); err != nil {
		t.Fatalf("Generate(): %v", err)
	}
	if err := os.RemoveAll(filepath.Join(root, ".verdi", "policy", "projections")); err != nil {
		t.Fatal(err)
	}

	report, err := verify(root, store.Constitution, store.Policies, ep)
	if err != nil {
		t.Fatalf("verify(): %v", err)
	}
	findOne(t, report, "codex", ReasonMissing, ".verdi/policy/projections/codex.json")
}

// TestVerify_ManifestDrift proves the manifest-drift classification
// directly at verifyManifestFile (never through Verify's public Load
// path, which the conflict makes structurally unreachable once a
// manifest exists — see the conflict-witness test above): a manifest
// whose bytes do not match the freshly recomputed canonical manifest is
// manifest-drift, independent of the managed files' own state.
func TestVerify_ManifestDrift(t *testing.T) {
	root := newFixtureRoot(t)
	manifestPath := filepath.Join(root, ".verdi", "policy", "projections", "codex.json")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, []byte(`{"schema":"stale"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	want := []byte(`{"schema":"verdi.instruction-projection/v1"}`)
	f := verifyManifestFile(manifestPath, ".verdi/policy/projections/codex.json", want)
	if f == nil || f.Code != ReasonManifestDrift {
		t.Fatalf("verifyManifestFile() = %+v, want ReasonManifestDrift", f)
	}
	if f.Expected == "" || f.Actual == "" || f.Expected == f.Actual {
		t.Fatalf("manifest-drift finding missing usable digests: %+v", f)
	}
}

// TestVerify_ManifestDrift_StaleAfterAuthorityChange proves the
// contract's own explicit rule: "a stale manifest is manifest-drift
// even when the files match the new authority" — Verify never trusts
// the stored manifest as ground truth for what should exist; it always
// recomputes.
func TestVerify_ManifestDrift_StaleAfterAuthorityChange(t *testing.T) {
	root := newFixtureRoot(t)
	store, ep := loadResolve(t, root)

	// A manifest whose bytes happen to equal an OLDER authority's
	// canonical manifest, but the files on disk match the CURRENT
	// authority: still manifest-drift, because Verify recomputes rather
	// than trusting what is stored.
	in, err := buildProjectionInput(store.Policies, ep)
	if err != nil {
		t.Fatal(err)
	}
	adapter := store.Constitution.Adapters[0]
	content := renderProjection(adapter, in)
	for _, rel := range adapter.Managed {
		writeTree(t, root, map[string]string{rel: string(content)})
	}
	staleManifest := manifest{Schema: ManifestSchema, Adapter: manifestAdapter{ID: "codex", Version: "1"}, AuthorityDigest: "sha256:stale"}
	staleBytes, err := manifestBytes(staleManifest)
	if err != nil {
		t.Fatal(err)
	}
	writeTree(t, root, map[string]string{".verdi/policy/projections/codex.json": string(staleBytes)})

	report, err := verify(root, store.Constitution, store.Policies, ep)
	if err != nil {
		t.Fatalf("verify(): %v", err)
	}
	findOne(t, report, "codex", ReasonManifestDrift, ".verdi/policy/projections/codex.json")
	if hasFinding(report, "codex", ReasonDrift, "AGENTS.md") || hasFinding(report, "codex", ReasonMissing, "AGENTS.md") {
		t.Fatalf("managed files matching current authority must stay clean even with a stale manifest: %+v", report.Findings)
	}
}

// TestVerify_RootUnmanagedInstructionFile plants an AGENTS.md the
// adapter never declared as managed, at repo root level.
func TestVerify_RootUnmanagedInstructionFile(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, unmanagedAdapterStoreFiles())
	store, ep := loadResolve(t, root)
	writeTree(t, root, map[string]string{"AGENTS.md": "hand-authored, never generated\n"})

	report, err := verify(root, store.Constitution, store.Policies, ep)
	if err != nil {
		t.Fatalf("verify(): %v", err)
	}
	findOne(t, report, "codex", ReasonUnmanaged, "AGENTS.md")
}

// TestVerify_NestedShadowingInstructionFile plants an AGENTS.md nested
// under a subdirectory the adapter never declared as managed.
func TestVerify_NestedShadowingInstructionFile(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, unmanagedAdapterStoreFiles())
	store, ep := loadResolve(t, root)
	writeTree(t, root, map[string]string{"services/legacy/AGENTS.md": "a nested instruction file the harness would also discover\n"})

	report, err := verify(root, store.Constitution, store.Policies, ep)
	if err != nil {
		t.Fatalf("verify(): %v", err)
	}
	findOne(t, report, "codex", ReasonShadowing, "services/legacy/AGENTS.md")
}

// TestVerify_SymlinkedDiscoveryFile_FailsClosed plants a symlink named
// AGENTS.md (not the managed path) pointing at an unrelated file; even
// though its basename matches the discovery filename, its target is
// outside proof, so it must fail closed as unmanaged, never silently
// ignored and never treated as satisfying anything.
func TestVerify_SymlinkedDiscoveryFile_FailsClosed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink semantics differ on windows")
	}
	root := t.TempDir()
	writeTree(t, root, unmanagedAdapterStoreFiles())
	store, ep := loadResolve(t, root)
	writeTree(t, root, map[string]string{"target.txt": "irrelevant\n"})
	if err := os.MkdirAll(filepath.Join(root, "extra"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "target.txt"), filepath.Join(root, "extra", "AGENTS.md")); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	report, err := verify(root, store.Constitution, store.Policies, ep)
	if err != nil {
		t.Fatalf("verify(): %v", err)
	}
	findOne(t, report, "codex", ReasonShadowing, "extra/AGENTS.md")
}

// TestVerify_SymlinkedManagedFile_FailsClosed replaces the MANAGED
// AGENTS.md with a symlink after Generate: even at the exact managed
// path, a symlink's target is outside proof and can never be treated as
// the generated file, so it fails closed rather than silently passing.
func TestVerify_SymlinkedManagedFile_FailsClosed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink semantics differ on windows")
	}
	root := newFixtureRoot(t)
	store, ep := loadResolve(t, root)
	if _, err := Generate(root); err != nil {
		t.Fatalf("Generate(): %v", err)
	}

	agentsPath := filepath.Join(root, "AGENTS.md")
	if err := os.Remove(agentsPath); err != nil {
		t.Fatal(err)
	}
	targetPath := filepath.Join(root, "docs", "AGENTS.md")
	if err := os.Symlink(targetPath, agentsPath); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	report, err := verify(root, store.Constitution, store.Policies, ep)
	if err != nil {
		t.Fatalf("verify(): %v", err)
	}
	// AGENTS.md's basename ("AGENTS.md") IS a declared discovery
	// filename here, so the discovery walk itself classifies the
	// symlink fail-closed (unmanaged/shadowing rule), independent of
	// the managed-file integrity pass.
	if !hasFinding(report, "codex", ReasonUnmanaged, "AGENTS.md") && !hasFinding(report, "codex", ReasonMissing, "AGENTS.md") {
		t.Fatalf("a symlinked managed file must fail closed (unmanaged or missing), got: %+v", report.Findings)
	}
}

// TestVerify_UnreadableDirectory_IncompleteDiscovery proves a directory
// the walk cannot open is reported as incomplete-discovery, never
// silently treated as absent (CO-1).
func TestVerify_UnreadableDirectory_IncompleteDiscovery(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod semantics differ on windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permission bits")
	}
	root := t.TempDir()
	writeTree(t, root, unmanagedAdapterStoreFiles())
	store, ep := loadResolve(t, root)

	blocked := filepath.Join(root, "blocked")
	if err := os.MkdirAll(blocked, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(blocked, "AGENTS.md"), []byte("hidden\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(blocked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o755) })

	report, err := verify(root, store.Constitution, store.Policies, ep)
	if err != nil {
		t.Fatalf("verify(): %v", err)
	}
	found := false
	for _, f := range report.Findings {
		if f.Code == ReasonIncompleteDiscovery {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected an incomplete-discovery finding for the unreadable directory, got: %+v", report.Findings)
	}
	if report.Clean() {
		t.Fatal("a report carrying incomplete-discovery must never report Clean()")
	}
}

// TestVerify_DanglingSymlink_FailsClosedNotError proves a symlink whose
// TARGET does not exist is still classified (never followed, so its
// dangling target never surfaces as a read error).
func TestVerify_DanglingSymlink_FailsClosedNotError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink semantics differ on windows")
	}
	root := t.TempDir()
	writeTree(t, root, unmanagedAdapterStoreFiles())
	store, ep := loadResolve(t, root)
	if err := os.Symlink(filepath.Join(root, "does-not-exist"), filepath.Join(root, "AGENTS.md")); err != nil {
		t.Fatalf("creating dangling symlink: %v", err)
	}

	report, err := verify(root, store.Constitution, store.Policies, ep)
	if err != nil {
		t.Fatalf("verify(): %v", err)
	}
	findOne(t, report, "codex", ReasonUnmanaged, "AGENTS.md")
}

// TestVerify_FindingsSortedByAdapterCodePath proves the deterministic
// ordering contract.
func TestVerify_FindingsSortedByAdapterCodePath(t *testing.T) {
	root := newFixtureRoot(t)
	store, ep := loadResolve(t, root)
	if _, err := Generate(root); err != nil {
		t.Fatalf("Generate(): %v", err)
	}
	if err := os.Remove(filepath.Join(root, "docs", "AGENTS.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("drifted\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := verify(root, store.Constitution, store.Policies, ep)
	if err != nil {
		t.Fatalf("verify(): %v", err)
	}
	for i := 1; i < len(report.Findings); i++ {
		a, b := report.Findings[i-1], report.Findings[i]
		if a.Adapter > b.Adapter {
			t.Fatalf("findings not sorted by adapter: %+v then %+v", a, b)
		}
		if a.Adapter == b.Adapter && a.Code > b.Code {
			t.Fatalf("findings not sorted by code within adapter: %+v then %+v", a, b)
		}
		if a.Adapter == b.Adapter && a.Code == b.Code && a.Path > b.Path {
			t.Fatalf("findings not sorted by path within adapter/code: %+v then %+v", a, b)
		}
	}
}

// unmanagedAdapterStoreFiles is a minimal store whose adapter manages a
// DIFFERENT path (PROJECTION.md) than the filename it discovers
// (AGENTS.md), so a test can plant a real AGENTS.md anywhere in the
// tree and know it can never coincide with the adapter's own managed
// path — every AGENTS.md this fixture's tests see is necessarily
// unmanaged or shadowing, never the generated file.
func unmanagedAdapterStoreFiles() map[string]string {
	return map[string]string{
		".verdi/policy/constitution.md": `---
schema: verdi.policy-constitution/v1
id: policy-constitution/constitution
kind: policy-constitution
title: "Discovery fixture constitution"
owners: [platform-team]
selected_profile: solo-default
environments: []
catalog:
  roles: [author, reviewer, policy-owner]
  transitions: [accept, close]
  evidence_sources: [ci]
  escalation_metrics: [age-days]
subjects:
  action: []
  configuration: []
  capability: []
  resource: []
  identity: []
  evidence: []
adapters:
  - id: codex
    version: "1"
    managed: [PROJECTION.md]
    discovery_filenames: [AGENTS.md]
---
Discovery fixture: one adapter managing PROJECTION.md while the harness
discovers AGENTS.md — deliberately disjoint so a planted AGENTS.md is
always unmanaged/shadowing, never the generated file.
`,
		".verdi/policy/policies/go-toolchain.md": `---
schema: verdi.policy/v1
id: policy/go-toolchain
kind: policy
title: "Go toolchain policy"
owners: [platform-team]
scope: {phases: [], environments: [], paths: [], refs: []}
claims: []
instructions:
  - "Run make verify before claiming completion."
payloads: {}
---
Pin the toolchain.
`,
		".verdi/policy/profiles/solo-default.md": `---
schema: verdi.governance-profile/v1
id: solo-default
class: solo
applicable_transitions: [accept]
identity_trust_sources:
  - {id: github-org, kind: forge}
role_mappings:
  - {role: author, trust_source: github-org, subjects: [alice]}
  - {role: policy-owner, trust_source: github-org, subjects: [alice]}
ownership_sources: []
signature_requirements: []
required_approvers: []
distinctness_rules: []
evidence_source_restrictions: []
escalation_thresholds: []
---
The solo operator profile.
`,
	}
}

func findOne(t *testing.T, report *Report, adapter string, code Reason, path string) Finding {
	t.Helper()
	var matches []Finding
	for _, f := range report.Findings {
		if f.Adapter == adapter && f.Code == code && f.Path == path {
			matches = append(matches, f)
		}
	}
	if len(matches) != 1 {
		t.Fatalf("findings matching adapter=%s code=%s path=%s = %d, want 1; all findings: %+v", adapter, code, path, len(matches), report.Findings)
	}
	return matches[0]
}

func hasFinding(report *Report, adapter string, code Reason, path string) bool {
	for _, f := range report.Findings {
		if f.Adapter == adapter && f.Code == code && f.Path == path {
			return true
		}
	}
	return false
}
