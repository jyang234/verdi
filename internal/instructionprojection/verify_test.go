package instructionprojection

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

// TestGenerateThenPublicVerify_RoundTripClean is the public-API
// round-trip proof: Generate writes projections and manifests, the
// grammar admits .verdi/policy/projections/ as a generated-output
// directory (policyartifact's projection-manifest row; policyauthority
// skips it as authority input per DC-1), and a fresh public Verify —
// its own Load+Resolve included — reports clean. A second Generate on
// the same root must also succeed and change nothing.
func TestGenerateThenPublicVerify_RoundTripClean(t *testing.T) {
	root := newFixtureRoot(t)

	first, err := Generate(root)
	if err != nil {
		t.Fatalf("Generate() unexpected error: %v", err)
	}

	report, err := Verify(root)
	if err != nil {
		t.Fatalf("Verify() after Generate(): %v", err)
	}
	if !report.Clean() {
		t.Fatalf("Verify() after Generate() not clean: %+v", report)
	}

	second, err := Generate(root)
	if err != nil {
		t.Fatalf("second Generate() on the same root: %v", err)
	}
	a, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("marshaling first result: %v", err)
	}
	b, err := json.Marshal(second)
	if err != nil {
		t.Fatalf("marshaling second result: %v", err)
	}
	if !bytes.Equal(a, b) {
		t.Fatalf("second Generate() differed from the first:\n%s\nvs\n%s", a, b)
	}

	// The public path also witnesses drift end to end: edit a managed
	// file and the next Verify names it.
	adapters := first.Adapters
	if len(adapters) == 0 || len(adapters[0].Files) == 0 {
		t.Fatalf("fixture generated no files: %+v", first)
	}
	managed := filepath.Join(root, filepath.FromSlash(adapters[0].Files[0].Path))
	if err := os.WriteFile(managed, []byte("edited projection\n"), 0o644); err != nil {
		t.Fatalf("editing managed file: %v", err)
	}
	report, err = Verify(root)
	if err != nil {
		t.Fatalf("Verify() after edit: %v", err)
	}
	if report.Clean() {
		t.Fatal("Verify() clean after a managed-file edit, want a drift finding")
	}
	found := false
	for _, f := range report.Findings {
		if f.Code == ReasonDrift && filepath.ToSlash(f.Path) == adapters[0].Files[0].Path {
			found = true
		}
	}
	if !found {
		t.Fatalf("no drift finding for %s in %+v", adapters[0].Files[0].Path, report.Findings)
	}
}

// TestGenerateThenVerifyCore_RoundTripClean exercises verify's store-
// agnostic core directly, fed a Store and EffectivePolicy the CALLER
// resolved (captured before Generate wrote anything). The public
// round-trip above already proves the end-to-end verdict; what this adds
// is the core's own contract — it never performs a Load of its own, so a
// caller that already holds a resolved store gets the same verdict from
// the same authority without re-reading the store. Writing a projection
// must not change that authority, and this is where that stays pinned.
func TestGenerateThenVerifyCore_RoundTripClean(t *testing.T) {
	root := newFixtureRoot(t)
	store, ep := loadResolve(t, root)

	if _, err := Generate(root); err != nil {
		t.Fatalf("Generate() unexpected error: %v", err)
	}

	report, err := verify(root, store, ep)
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
	if _, err := Generate(root); err != nil {
		t.Fatalf("Generate(): %v", err)
	}

	agentsPath := filepath.Join(root, "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte("this is not the generated content at all\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := Verify(root)
	if err != nil {
		t.Fatalf("Verify(): %v", err)
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

	report, err := Verify(root)
	if err != nil {
		t.Fatalf("Verify(): %v", err)
	}
	findOne(t, report, "codex", ReasonTruncated, "AGENTS.md")
	if hasFinding(report, "codex", ReasonDrift, "AGENTS.md") {
		t.Fatalf("a proper-prefix truncation must classify as truncated, not drift: %+v", report.Findings)
	}
}

// TestVerify_MissingManagedFile deletes a managed file after Generate.
func TestVerify_MissingManagedFile(t *testing.T) {
	root := newFixtureRoot(t)
	if _, err := Generate(root); err != nil {
		t.Fatalf("Generate(): %v", err)
	}
	if err := os.Remove(filepath.Join(root, "docs", "AGENTS.md")); err != nil {
		t.Fatal(err)
	}

	report, err := Verify(root)
	if err != nil {
		t.Fatalf("Verify(): %v", err)
	}
	findOne(t, report, "codex", ReasonMissing, "docs/AGENTS.md")
}

// TestVerify_MissingManifest deletes the generated manifest directory
// after Generate: a managed file set that still matches authority is not
// clean while its manifest is absent.
func TestVerify_MissingManifest(t *testing.T) {
	root := newFixtureRoot(t)
	if _, err := Generate(root); err != nil {
		t.Fatalf("Generate(): %v", err)
	}
	if err := os.RemoveAll(filepath.Join(root, ".verdi", "policy", "projections")); err != nil {
		t.Fatal(err)
	}

	report, err := Verify(root)
	if err != nil {
		t.Fatalf("Verify(): %v", err)
	}
	findOne(t, report, "codex", ReasonMissing, ".verdi/policy/projections/codex.json")
}

// TestVerify_ManifestDrift unit-tests verifyManifestFile directly. The
// end-to-end manifest-drift verdict is proven through the public Verify
// by TestVerify_ManifestDrift_StaleAfterAuthorityChange below; what this
// adds is the classifier's own contract in isolation — arbitrary
// non-matching bytes are manifest-drift, and the finding carries both
// digests — with no store, adapter, or walk involved.
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

	report, err := Verify(root)
	if err != nil {
		t.Fatalf("Verify(): %v", err)
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
	writeTree(t, root, map[string]string{"AGENTS.md": "hand-authored, never generated\n"})

	report, err := Verify(root)
	if err != nil {
		t.Fatalf("Verify(): %v", err)
	}
	findOne(t, report, "codex", ReasonUnmanaged, "AGENTS.md")
}

// TestVerify_NestedShadowingInstructionFile plants an AGENTS.md nested
// under a subdirectory the adapter never declared as managed.
func TestVerify_NestedShadowingInstructionFile(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, unmanagedAdapterStoreFiles())
	writeTree(t, root, map[string]string{"services/legacy/AGENTS.md": "a nested instruction file the harness would also discover\n"})

	report, err := Verify(root)
	if err != nil {
		t.Fatalf("Verify(): %v", err)
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
	writeTree(t, root, map[string]string{"target.txt": "irrelevant\n"})
	if err := os.MkdirAll(filepath.Join(root, "extra"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "target.txt"), filepath.Join(root, "extra", "AGENTS.md")); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	report, err := Verify(root)
	if err != nil {
		t.Fatalf("Verify(): %v", err)
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

	report, err := Verify(root)
	if err != nil {
		t.Fatalf("Verify(): %v", err)
	}
	// AGENTS.md's basename ("AGENTS.md") IS a declared discovery
	// filename here, so the discovery walk itself classifies the
	// symlink fail-closed (unmanaged/shadowing rule), independent of
	// the managed-file integrity pass.
	if !hasFinding(report, "codex", ReasonUnmanaged, "AGENTS.md") && !hasFinding(report, "codex", ReasonMissing, "AGENTS.md") {
		t.Fatalf("a symlinked managed file must fail closed (unmanaged or missing), got: %+v", report.Findings)
	}
}

// TestVerify_SymlinkedManagedFile_OutsideTheWalk_FailsClosed isolates
// the MANAGED-FILE integrity check's own symlink branch. The test above
// leaves that branch unpinned: its managed path sits where the discovery
// walk would have caught the symlink anyway. Here the adapter's managed
// path lives inside node_modules — a subtree the walk NEVER descends
// into (discovery.go's skipDirBasenames, disclosed by every report) —
// and the symlink's target holds BYTE-CORRECT generated content, so the
// only thing that can produce a finding is the refusal to treat a
// symlink as the managed file.
func TestVerify_SymlinkedManagedFile_OutsideTheWalk_FailsClosed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink semantics differ on windows")
	}
	root := t.TempDir()
	writeTree(t, root, excludedSubtreeManagedStoreFiles())
	if _, err := Generate(root); err != nil {
		t.Fatalf("Generate(): %v", err)
	}

	const rel = "node_modules/AGENTS.md"
	managed := filepath.Join(root, filepath.FromSlash(rel))
	generated, err := os.ReadFile(managed)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "generated-elsewhere.md")
	if err := os.WriteFile(target, generated, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(managed); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, managed); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	report, err := Verify(root)
	if err != nil {
		t.Fatalf("Verify(): %v", err)
	}
	f := findOne(t, report, "codex", ReasonMissing, rel)
	if !strings.Contains(f.Detail, "symlink") {
		t.Fatalf("missing finding must disclose WHY the managed file is absent: %+v", f)
	}
	if hasFinding(report, "codex", ReasonUnmanaged, rel) || hasFinding(report, "codex", ReasonShadowing, rel) {
		t.Fatalf("%s lives in a subtree the walk never enters; only the managed-file check may report it: %+v", rel, report.Findings)
	}
}

// excludedSubtreeManagedStoreFiles is a store whose one adapter's only
// managed projection lives inside node_modules — a subtree the discovery
// walk never descends into — so the managed-file integrity check is the
// ONLY pass that can ever report on it.
func excludedSubtreeManagedStoreFiles() map[string]string {
	files := unmanagedAdapterStoreFiles()
	files[".verdi/policy/constitution.md"] = strings.Replace(
		files[".verdi/policy/constitution.md"],
		"    managed: [PROJECTION.md]\n    discovery_filenames: [AGENTS.md, PROJECTION.md]\n",
		"    managed: [\"node_modules/AGENTS.md\"]\n    discovery_filenames: [AGENTS.md]\n", 1)
	return files
}

// TestVerify_OrphanManifest proves a manifest left behind by an adapter
// the constitution no longer declares is a named finding. Verify's
// per-adapter passes only ever look for the manifests CURRENT adapters
// should have, so without this enumeration a stale manifest — an
// authority-shaped record of a projection nothing regenerates — would
// verify clean forever (CO-1).
func TestVerify_OrphanManifest(t *testing.T) {
	root := newFixtureRoot(t)
	if _, err := Generate(root); err != nil {
		t.Fatalf("Generate(): %v", err)
	}

	// Rename the adapter in the constitution: codex.json is now nobody's
	// manifest.
	consPath := filepath.Join(root, ".verdi", "policy", "constitution.md")
	data, err := os.ReadFile(consPath)
	if err != nil {
		t.Fatal(err)
	}
	renamed := strings.Replace(string(data), "- id: codex", "- id: codex-next", 1)
	if renamed == string(data) {
		t.Fatal("fixture constitution did not carry the expected adapter id line")
	}
	if err := os.WriteFile(consPath, []byte(renamed), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := Verify(root)
	if err != nil {
		t.Fatalf("Verify(): %v", err)
	}
	f := findOne(t, report, "", ReasonOrphanManifest, ".verdi/policy/projections/codex.json")
	if f.Adapter != "" {
		t.Fatalf("an orphan manifest belongs to no current adapter; Adapter = %q, want empty", f.Adapter)
	}
	// The renamed adapter's own manifest is separately missing — the
	// orphan finding never substitutes for the current adapter's state.
	findOne(t, report, "codex-next", ReasonMissing, ".verdi/policy/projections/codex-next.json")
}

// TestVerify_NoOrphanManifest_WhenEveryManifestIsCurrent is the
// enumeration's negative arm: the generated manifests of currently
// declared adapters must never be reported as orphans.
func TestVerify_NoOrphanManifest_WhenEveryManifestIsCurrent(t *testing.T) {
	root := newMultiFixtureRoot(t)
	if _, err := Generate(root); err != nil {
		t.Fatalf("Generate(): %v", err)
	}

	report, err := Verify(root)
	if err != nil {
		t.Fatalf("Verify(): %v", err)
	}
	for _, f := range report.Findings {
		if f.Code == ReasonOrphanManifest {
			t.Fatalf("a current adapter's own manifest was reported as an orphan: %+v", f)
		}
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

	report, err := Verify(root)
	if err != nil {
		t.Fatalf("Verify(): %v", err)
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
	if err := os.Symlink(filepath.Join(root, "does-not-exist"), filepath.Join(root, "AGENTS.md")); err != nil {
		t.Fatalf("creating dangling symlink: %v", err)
	}

	report, err := Verify(root)
	if err != nil {
		t.Fatalf("Verify(): %v", err)
	}
	findOne(t, report, "codex", ReasonUnmanaged, "AGENTS.md")
}

// TestVerify_FindingsSortedByAdapterCodePath proves the deterministic
// ordering contract.
func TestVerify_FindingsSortedByAdapterCodePath(t *testing.T) {
	root := newFixtureRoot(t)
	if _, err := Generate(root); err != nil {
		t.Fatalf("Generate(): %v", err)
	}
	if err := os.Remove(filepath.Join(root, "docs", "AGENTS.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("drifted\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := Verify(root)
	if err != nil {
		t.Fatalf("Verify(): %v", err)
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
// DIFFERENT path (PROJECTION.md) than the AGENTS.md its tests plant, so
// a test can plant a real AGENTS.md anywhere in the tree and know it can
// never coincide with the adapter's own managed path — every AGENTS.md
// this fixture's tests see is necessarily unmanaged or shadowing, never
// the generated file. PROJECTION.md is itself a declared discovery
// filename because policyartifact's managed-target grammar requires
// every managed projection to be a filename some adapter discovers (a
// managed path no harness ever reads is not a projection target); the
// separation the fixture needs is between MANAGED PATHS, not between the
// declared filename sets.
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
    discovery_filenames: [AGENTS.md, PROJECTION.md]
---
Discovery fixture: one adapter managing PROJECTION.md while the harness
also discovers AGENTS.md — no AGENTS.md is ever managed, so a planted
AGENTS.md is always unmanaged/shadowing, never the generated file.
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
template: {identity: "embedded:policy.md", digest: "sha256:0e1b83a8e41d5ecfe9f14cb4973b7a584bfcb471247fa064b5fe273e4d322561"}
---
Pin the toolchain.
`,
		".verdi/policy/profiles/solo-default.md": soloDefaultProfileDoc,
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
