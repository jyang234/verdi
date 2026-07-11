package lint

import "testing"

// TestBuildSnapshot_Happy exercises BuildSnapshot directly (Engine.Run's
// other tests only cover it transitively): the clean corpus+setup repo
// should decode every document, index every ref, and load the manifest
// and .gitattributes without any operational error.
func TestBuildSnapshot_Happy(t *testing.T) {
	repo := buildLintRepo(t)

	snap, err := BuildSnapshot(repo.Dir, Options{})
	if err != nil {
		t.Fatalf("BuildSnapshot: %v", err)
	}
	if len(snap.Docs) == 0 {
		t.Fatal("no documents found")
	}
	if snap.Manifest == nil {
		t.Fatal("manifest not decoded")
	}
	if snap.ManifestErr != nil {
		t.Fatalf("ManifestErr = %v, want nil", snap.ManifestErr)
	}
	if len(snap.GitAttributes) == 0 {
		t.Fatal("GitAttributes not read")
	}
	if len(snap.Services) == 0 {
		t.Fatal("no services discovered (want the loansvc fixture)")
	}
	if _, ok := snap.ByRef["spec/stale-decline"]; !ok {
		t.Fatal(`ByRef["spec/stale-decline"] missing`)
	}
	for _, d := range snap.Docs {
		if d.DecodeErr != nil {
			t.Errorf("unexpected decode error for %s: %v", d.RelPath, d.DecodeErr)
		}
	}
}

// TestBuildSnapshot_Negative_NoVerdiDir proves BuildSnapshot returns an
// operational error (not a panic, not a silently-empty Snapshot) when root
// has no .verdi/ directory at all.
func TestBuildSnapshot_Negative_NoVerdiDir(t *testing.T) {
	dir := t.TempDir()
	if _, err := BuildSnapshot(dir, Options{}); err == nil {
		t.Fatal("BuildSnapshot on a directory with no .verdi/: want error, got nil")
	}
}

// TestBuildSnapshot_MissingOptionalFiles proves an absent manifest and
// absent .gitattributes are not operational errors — both are legal
// (empty) states BuildSnapshot must tolerate; VL-008/VL-012 are the rules
// that turn their absence into findings, not BuildSnapshot itself.
func TestBuildSnapshot_MissingOptionalFiles(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir+"/.verdi/adr/0001-x.md", lintTestMinimalADR)

	snap, err := BuildSnapshot(dir, Options{})
	if err != nil {
		t.Fatalf("BuildSnapshot: %v", err)
	}
	if snap.Manifest != nil || snap.ManifestErr != nil {
		t.Fatalf("Manifest=%v ManifestErr=%v, want both nil (absent manifest is not an error)", snap.Manifest, snap.ManifestErr)
	}
	if snap.GitAttributes != nil || snap.GitAttributesErr != nil {
		t.Fatalf("GitAttributes=%v GitAttributesErr=%v, want both nil (absent .gitattributes is not an error)", snap.GitAttributes, snap.GitAttributesErr)
	}
}

const lintTestMinimalADR = `---
id: adr/0001-x
kind: adr
title: "x"
status: proposed
owners: [platform-team]
---
# x
`
