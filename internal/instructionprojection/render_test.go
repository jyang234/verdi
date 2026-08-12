package instructionprojection

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/policyauthority"
)

// twoPolicyAdapterStoreFiles is a store whose two nonzero-instruction
// policies (policy/alpha, policy/bravo) let a test distinguish a full
// selection from a partial one: testdata/store carries only one
// nonzero-instruction policy (go-toolchain) plus one zero-instruction
// policy (silent), which cannot witness "the unselected policy's own
// instructions are omitted" — omitting the only nonzero policy would
// just reproduce the already-covered "zero policies" shape.
func twoPolicyAdapterStoreFiles() map[string]string {
	return map[string]string{
		".verdi/policy/constitution.md": `---
schema: verdi.policy-constitution/v1
id: policy-constitution/constitution
kind: policy-constitution
title: "Render selection fixture constitution"
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
    managed: [AGENTS.md]
    discovery_filenames: [AGENTS.md]
---
Two nonzero-instruction policies so a partial selection has something to
omit.
`,
		".verdi/policy/policies/alpha.md": `---
schema: verdi.policy/v1
id: policy/alpha
kind: policy
title: "Alpha policy"
owners: [platform-team]
scope: {phases: [], environments: [], paths: [], refs: []}
claims: []
instructions:
  - "Alpha instruction one."
payloads: {}
template: {identity: "embedded:policy.md", digest: "sha256:0e1b83a8e41d5ecfe9f14cb4973b7a584bfcb471247fa064b5fe273e4d322561"}
---
Alpha.
`,
		".verdi/policy/policies/bravo.md": `---
schema: verdi.policy/v1
id: policy/bravo
kind: policy
title: "Bravo policy"
owners: [platform-team]
scope: {phases: [], environments: [], paths: [], refs: []}
claims: []
instructions:
  - "Bravo instruction one."
payloads: {}
template: {identity: "embedded:policy.md", digest: "sha256:0e1b83a8e41d5ecfe9f14cb4973b7a584bfcb471247fa064b5fe273e4d322561"}
---
Bravo.
`,
		".verdi/policy/profiles/solo-default.md": soloDefaultProfileDoc,
	}
}

// newTwoPolicyFixtureRoot writes twoPolicyAdapterStoreFiles into a fresh
// temp dir and returns it, never calling Generate.
func newTwoPolicyFixtureRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeTree(t, root, twoPolicyAdapterStoreFiles())
	return root
}

// listAllEntries returns the sorted, root-relative slash paths of every
// file under root, used to prove a call wrote nothing.
func listAllEntries(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root || d.IsDir() {
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("listAllEntries(%s): %v", root, err)
	}
	sort.Strings(out)
	return out
}

// fullSelectionOf returns a Selection naming every one of ep's effective
// policy ids, matching this lane's contract clause "Generate invokes
// Render for every adapter with all effective policy IDs".
func fullSelectionOf(ep *policyauthority.EffectivePolicy) Selection {
	ids := make([]string, 0, len(ep.Policies))
	for _, e := range ep.Policies {
		ids = append(ids, e.PolicyID)
	}
	return Selection{PolicyIDs: ids}
}

// TestRender_NoWrites proves Render performs no filesystem I/O at all:
// its signature carries no root parameter, and this test additionally
// witnesses that a fixture tree Render never called Generate against is
// byte-for-byte unchanged after the call (SI-87(c): "compile never writes
// managed projections"; the same purity is Render's own contract).
func TestRender_NoWrites(t *testing.T) {
	root := newTwoPolicyFixtureRoot(t)
	before := listAllEntries(t, root)

	store, ep := loadResolve(t, root)
	adapter := store.Constitution.Adapters[0]

	if _, err := Render(store, ep, adapter, fullSelectionOf(ep)); err != nil {
		t.Fatalf("Render() unexpected error: %v", err)
	}

	after := listAllEntries(t, root)
	if len(before) != len(after) {
		t.Fatalf("Render() changed the fixture tree's entry count: before %v, after %v", before, after)
	}
	for i := range before {
		if before[i] != after[i] {
			t.Fatalf("Render() changed the fixture tree: before %v, after %v", before, after)
		}
	}
}

// TestRender_FullSelectionMatchesGenerateAndVerify proves the one-pure-
// call contract: rendering with every effective policy id reproduces
// exactly the bytes Generate wrote and exactly the bytes Verify's own
// (pre-refactor) rendering path expects, for both the managed file
// content and the canonical manifest.
func TestRender_FullSelectionMatchesGenerateAndVerify(t *testing.T) {
	root := newFixtureRoot(t)

	genRes, err := Generate(root)
	if err != nil {
		t.Fatalf("Generate(): %v", err)
	}
	if len(genRes.Adapters) != 1 {
		t.Fatalf("Generate() adapters = %d, want 1", len(genRes.Adapters))
	}
	genAdapter := genRes.Adapters[0]

	store, ep := loadResolve(t, root)
	adapter := store.Constitution.Adapters[0]

	rendered, err := Render(store, ep, adapter, fullSelectionOf(ep))
	if err != nil {
		t.Fatalf("Render() unexpected error: %v", err)
	}

	if rendered.AdapterID != genAdapter.AdapterID || rendered.AdapterVersion != genAdapter.AdapterVersion {
		t.Fatalf("Render() adapter identity = %s/%s, want %s/%s", rendered.AdapterID, rendered.AdapterVersion, genAdapter.AdapterID, genAdapter.AdapterVersion)
	}
	if len(rendered.Files) != len(genAdapter.Files) {
		t.Fatalf("Render() files = %d, want %d", len(rendered.Files), len(genAdapter.Files))
	}
	for i, gf := range genAdapter.Files {
		rf := rendered.Files[i]
		if rf.Path != gf.Path {
			t.Fatalf("Render() file[%d] path = %q, want %q", i, rf.Path, gf.Path)
		}
		if rf.Digest != gf.Digest {
			t.Fatalf("Render() file[%d] digest = %s, want %s", i, rf.Digest, gf.Digest)
		}
		onDisk, rerr := os.ReadFile(filepath.Join(root, filepath.FromSlash(gf.Path)))
		if rerr != nil {
			t.Fatalf("reading generated %s: %v", gf.Path, rerr)
		}
		if !bytes.Equal(rf.Content, onDisk) {
			t.Fatalf("Render() file[%d] content differs from what Generate wrote to %s", i, gf.Path)
		}
	}

	if rendered.ManifestDigest != genAdapter.ManifestDigest {
		t.Fatalf("Render() manifest digest = %s, want %s", rendered.ManifestDigest, genAdapter.ManifestDigest)
	}
	onDiskManifest, rerr := os.ReadFile(filepath.Join(root, filepath.FromSlash(genAdapter.ManifestPath)))
	if rerr != nil {
		t.Fatalf("reading generated manifest %s: %v", genAdapter.ManifestPath, rerr)
	}
	if !bytes.Equal(rendered.Manifest, onDiskManifest) {
		t.Fatalf("Render() manifest bytes differ from what Generate wrote to %s", genAdapter.ManifestPath)
	}

	// Cross-check against the package's own renderProjection/buildManifest
	// path directly (the same one Verify's core uses) so this test also
	// pins "the same projection manifest bytes/facts Generate and Verify
	// use today" independent of Generate's own write.
	in, err := buildProjectionInput(store.Policies, ep)
	if err != nil {
		t.Fatalf("buildProjectionInput: %v", err)
	}
	wantContent := renderProjection(adapter, in)
	wantDigest := contentDigest(wantContent)
	wantFiles := make([]FileDigest, 0, len(adapter.Managed))
	for _, rel := range adapter.Managed {
		wantFiles = append(wantFiles, FileDigest{Path: rel, Digest: wantDigest})
	}
	wantManifest := buildManifest(adapter, in, wantFiles)
	wantManifestBytes, err := manifestBytes(wantManifest)
	if err != nil {
		t.Fatalf("manifestBytes: %v", err)
	}
	if !bytes.Equal(rendered.Manifest, wantManifestBytes) {
		t.Fatalf("Render() manifest differs from the shared buildManifest/manifestBytes path")
	}
	for _, rf := range rendered.Files {
		if !bytes.Equal(rf.Content, wantContent) {
			t.Fatalf("Render() file %s content differs from the shared renderProjection path", rf.Path)
		}
	}

	if rendered.AuthorityDigest != in.AuthorityDigest || rendered.ProfileID != in.ProfileID || rendered.ProfileDigest != in.ProfileDigest {
		t.Fatalf("Render() authority facts = %+v, want authority=%s profile=%s/%s", rendered, in.AuthorityDigest, in.ProfileID, in.ProfileDigest)
	}
}

// TestRender_UnsealedEffectivePolicy_Rejected proves a hand-built,
// never-Resolved EffectivePolicy is refused rather than silently
// rendered.
func TestRender_UnsealedEffectivePolicy_Rejected(t *testing.T) {
	root := newTwoPolicyFixtureRoot(t)
	store, ep := loadResolve(t, root)
	adapter := store.Constitution.Adapters[0]

	unsealed := &policyauthority.EffectivePolicy{
		Schema:    ep.Schema,
		ProfileID: ep.ProfileID,
		Policies:  ep.Policies,
	}

	if _, err := Render(store, unsealed, adapter, fullSelectionOf(ep)); err == nil {
		t.Fatal("Render() with an unsealed effective policy = nil error, want a rejection")
	}
}

// TestRender_MismatchedStoreEffectivePair_Rejected proves Render never
// substitutes a fresh re-resolution for a store/effective pair that does
// not genuinely match: it must name the mismatch as an error rather than
// silently rendering against whichever one it trusts more.
func TestRender_MismatchedStoreEffectivePair_Rejected(t *testing.T) {
	rootA := newTwoPolicyFixtureRoot(t)
	rootB := newFixtureRoot(t) // a materially different store/policy set

	storeA, _ := loadResolve(t, rootA)
	_, epB := loadResolve(t, rootB)

	adapterA := storeA.Constitution.Adapters[0]

	if _, err := Render(storeA, epB, adapterA, Selection{}); err == nil {
		t.Fatal("Render() with a mismatched store/effective pair = nil error, want a rejection")
	}
}

// TestRender_NilStore_Rejected proves Render never tolerates a store that
// was not produced by policyauthority.Load (the same posture Resolve
// itself already enforces, restated at this seam).
func TestRender_NilStore_Rejected(t *testing.T) {
	root := newTwoPolicyFixtureRoot(t)
	_, ep := loadResolve(t, root)

	// The zero-value adapter's identity does not matter here: the nil
	// store must be refused before any adapter comparison is reachable.
	if _, err := Render(nil, ep, policyartifact.Adapter{}, Selection{}); err == nil {
		t.Fatal("Render() with a nil store = nil error, want a rejection")
	}
}

// TestRender_UnknownSelectedID_Rejected proves a selected policy id that
// does not name any effective policy entry is refused.
func TestRender_UnknownSelectedID_Rejected(t *testing.T) {
	root := newTwoPolicyFixtureRoot(t)
	store, ep := loadResolve(t, root)
	adapter := store.Constitution.Adapters[0]

	_, err := Render(store, ep, adapter, Selection{PolicyIDs: []string{"policy/alpha", "policy/does-not-exist"}})
	if err == nil {
		t.Fatal("Render() with an unknown selected policy id = nil error, want a rejection")
	}
}

// TestRender_DuplicateSelectedID_Rejected proves a selection naming the
// same policy id twice is refused rather than silently deduplicated.
func TestRender_DuplicateSelectedID_Rejected(t *testing.T) {
	root := newTwoPolicyFixtureRoot(t)
	store, ep := loadResolve(t, root)
	adapter := store.Constitution.Adapters[0]

	_, err := Render(store, ep, adapter, Selection{PolicyIDs: []string{"policy/alpha", "policy/alpha"}})
	if err == nil {
		t.Fatal("Render() with a duplicate selected policy id = nil error, want a rejection")
	}
}

// TestRender_MismatchedAdapter_Rejected proves an adapter value that does
// not byte-exactly match a row of the passed store's own constitution is
// refused — a caller cannot render against a constitution the store never
// declared.
func TestRender_MismatchedAdapter_Rejected(t *testing.T) {
	root := newTwoPolicyFixtureRoot(t)
	store, ep := loadResolve(t, root)
	adapter := store.Constitution.Adapters[0]
	mutated := adapter
	mutated.Version = adapter.Version + "-not-the-declared-version"

	_, err := Render(store, ep, mutated, fullSelectionOf(ep))
	if err == nil {
		t.Fatal("Render() with an adapter that does not match the store's constitution = nil error, want a rejection")
	}
}

// TestRender_SelectionSortedOnACopy proves an out-of-order selection is
// accepted and internally sorted, and that sorting never mutates the
// caller's own slice (the plan's explicit "sorted on a copy" clause).
func TestRender_SelectionSortedOnACopy(t *testing.T) {
	root := newTwoPolicyFixtureRoot(t)
	store, ep := loadResolve(t, root)
	adapter := store.Constitution.Adapters[0]

	ids := []string{"policy/bravo", "policy/alpha"}
	original := append([]string(nil), ids...)
	sel := Selection{PolicyIDs: ids}

	if _, err := Render(store, ep, adapter, sel); err != nil {
		t.Fatalf("Render() unexpected error: %v", err)
	}

	if len(ids) != len(original) {
		t.Fatalf("Render() changed the caller's selection slice length: %v", ids)
	}
	for i := range ids {
		if ids[i] != original[i] {
			t.Fatalf("Render() mutated the caller's selection slice in place: got %v, want unchanged %v", ids, original)
		}
	}
}

// TestRender_PartialSelection_OmitsExactlyUnselectedPolicies proves a
// selection naming only one of two nonzero-instruction policies renders a
// view carrying that policy's own section and instruction, and neither
// the unselected policy's section nor its instruction text, without
// Render itself interpreting scope (SI-87(c)) — the omission here is
// driven purely by which ids the caller selected.
func TestRender_PartialSelection_OmitsExactlyUnselectedPolicies(t *testing.T) {
	root := newTwoPolicyFixtureRoot(t)
	store, ep := loadResolve(t, root)
	adapter := store.Constitution.Adapters[0]

	rendered, err := Render(store, ep, adapter, Selection{PolicyIDs: []string{"policy/bravo"}})
	if err != nil {
		t.Fatalf("Render() unexpected error: %v", err)
	}
	if len(rendered.Files) != 1 {
		t.Fatalf("Render() files = %d, want 1", len(rendered.Files))
	}
	content := rendered.Files[0].Content

	if !bytes.Contains(content, []byte("Bravo policy")) || !bytes.Contains(content, []byte("Bravo instruction one.")) {
		t.Fatalf("partial selection omitted the SELECTED policy's own section:\n%s", content)
	}
	if bytes.Contains(content, []byte("Alpha policy")) || bytes.Contains(content, []byte("Alpha instruction one.")) {
		t.Fatalf("partial selection did not omit the unselected policy's section:\n%s", content)
	}

	// The manifest facts still identify this adapter's real authority
	// digest (unaffected by selection — selection filters WHICH policies
	// render, not what authority they were resolved from).
	if rendered.AuthorityDigest == "" {
		t.Fatal("Render() with a partial selection carries an empty authority digest")
	}
}

// TestRender_FreshCopies_NeverAliasStoreOrInputMemory mutates every byte
// slice a Rendered result returns and proves a SECOND, independent Render
// call over the same genuine inputs is unaffected — the only way that
// can hold is if neither call's bytes alias a shared backing array (the
// store, the effective policy, or each other).
func TestRender_FreshCopies_NeverAliasStoreOrInputMemory(t *testing.T) {
	root := newFixtureRoot(t)
	store, ep := loadResolve(t, root)
	adapter := store.Constitution.Adapters[0]
	sel := fullSelectionOf(ep)

	first, err := Render(store, ep, adapter, sel)
	if err != nil {
		t.Fatalf("Render() first call: %v", err)
	}
	if len(first.Files) == 0 || len(first.Files[0].Content) == 0 || len(first.Manifest) == 0 {
		t.Fatalf("fixture rendered no usable content to mutate: %+v", first)
	}

	wantContent := append([]byte(nil), first.Files[0].Content...)
	wantManifest := append([]byte(nil), first.Manifest...)

	// Mutate every returned byte slice in place.
	for i := range first.Files[0].Content {
		first.Files[0].Content[i] = 'X'
	}
	for i := range first.Manifest {
		first.Manifest[i] = 'X'
	}

	second, err := Render(store, ep, adapter, sel)
	if err != nil {
		t.Fatalf("Render() second call: %v", err)
	}
	if !bytes.Equal(second.Files[0].Content, wantContent) {
		t.Fatal("mutating the first Render() call's file content affected a second, independent Render() call: content is aliased")
	}
	if !bytes.Equal(second.Manifest, wantManifest) {
		t.Fatal("mutating the first Render() call's manifest bytes affected a second, independent Render() call: manifest is aliased")
	}

	// The store's own loaded policy instructions must also be untouched:
	// mutating a rendered file must never reach back into store-owned
	// memory.
	for _, p := range store.Policies {
		for _, ins := range p.Instructions {
			if bytes.Contains([]byte(ins), []byte("XXXXXXXX")) {
				t.Fatalf("mutating Render() output corrupted store-owned policy instruction text: %q", ins)
			}
		}
	}
}

// TestRender_MultiFileAdapter_FilesDoNotAliasEachOther proves that when
// one adapter manages more than one path (this package's own "one
// adapter, one content body, many managed paths" rule), the returned
// RenderedFile entries do not share one backing array: mutating one
// file's Content must never change another file's Content.
func TestRender_MultiFileAdapter_FilesDoNotAliasEachOther(t *testing.T) {
	root := newFixtureRoot(t) // codex: managed [AGENTS.md, docs/AGENTS.md]
	store, ep := loadResolve(t, root)
	adapter := store.Constitution.Adapters[0]

	rendered, err := Render(store, ep, adapter, fullSelectionOf(ep))
	if err != nil {
		t.Fatalf("Render(): %v", err)
	}
	if len(rendered.Files) != 2 {
		t.Fatalf("Render() files = %d, want 2", len(rendered.Files))
	}
	want := append([]byte(nil), rendered.Files[1].Content...)
	for i := range rendered.Files[0].Content {
		rendered.Files[0].Content[i] = 'X'
	}
	if !bytes.Equal(rendered.Files[1].Content, want) {
		t.Fatal("mutating one managed file's Content changed another managed file's Content: they alias the same backing array")
	}
}

// TestRender_DeterministicAcrossIndependentRoots proves two structurally
// identical stores render byte-identical results (CO-3), matching
// Generate/Verify's own determinism tests.
func TestRender_DeterministicAcrossIndependentRoots(t *testing.T) {
	rootA := newTwoPolicyFixtureRoot(t)
	rootB := newTwoPolicyFixtureRoot(t)

	storeA, epA := loadResolve(t, rootA)
	storeB, epB := loadResolve(t, rootB)

	renderedA, err := Render(storeA, epA, storeA.Constitution.Adapters[0], fullSelectionOf(epA))
	if err != nil {
		t.Fatalf("Render(rootA): %v", err)
	}
	renderedB, err := Render(storeB, epB, storeB.Constitution.Adapters[0], fullSelectionOf(epB))
	if err != nil {
		t.Fatalf("Render(rootB): %v", err)
	}

	if renderedA.ManifestDigest != renderedB.ManifestDigest {
		t.Fatalf("two structurally identical stores rendered different manifest digests: %s vs %s", renderedA.ManifestDigest, renderedB.ManifestDigest)
	}
	if !bytes.Equal(renderedA.Manifest, renderedB.Manifest) {
		t.Fatal("two structurally identical stores rendered different manifest bytes")
	}
	if !bytes.Equal(renderedA.Files[0].Content, renderedB.Files[0].Content) {
		t.Fatal("two structurally identical stores rendered different file content")
	}
}

// TestGenerateThenVerify_StillRouteThroughOneSharedRenderer is a
// package-level regression guard: after routing Generate and Verify
// through Render, marshaling their JSON-visible results must still be
// stable (this reuses the existing round-trip fixture, so a divergence
// here would mean the refactor changed observable behavior).
func TestGenerateThenVerify_StillRouteThroughOneSharedRenderer(t *testing.T) {
	root := newFixtureRoot(t)
	res, err := Generate(root)
	if err != nil {
		t.Fatalf("Generate(): %v", err)
	}
	data, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshaling Generate() result: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("Generate() result marshaled to nothing")
	}
	report, err := Verify(root)
	if err != nil {
		t.Fatalf("Verify(): %v", err)
	}
	if !report.Clean() {
		t.Fatalf("Verify() after Generate() not clean: %+v", report.Findings)
	}
}
