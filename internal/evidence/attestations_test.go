package evidence

import (
	"os"
	"path/filepath"
	"testing"
)

const testAttestation = `---
id: attestation/story-1--ac-2
kind: attestation
title: "AC-2 attested (test)"
owners: [qa-lead]
frozen: { at: 2026-05-01, commit: c5e360a9ee5e9eb6089e54b772fa16959ada4662 }
---
# Attestation
`

func writeAttestation(t *testing.T, root, storySlug, acID, content string) {
	t.Helper()
	dir := filepath.Join(root, ".verdi", "attestations", storySlug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, acID+".md"), []byte(content), 0o644); err != nil {
		t.Fatalf("writing attestation: %v", err)
	}
}

// TestAttestationExists_Happy proves existence, alone, is the record
// (02 §Kind registry: "(none — existence is the record)").
func TestAttestationExists_Happy(t *testing.T) {
	root := t.TempDir()
	writeAttestation(t, root, "story-1", "ac-2", testAttestation)

	exists, err := AttestationExists(root, "story-1", "ac-2")
	if err != nil {
		t.Fatalf("AttestationExists: %v", err)
	}
	if !exists {
		t.Fatal("AttestationExists(present file) = false, want true")
	}
}

// TestAttestationExists_Negative proves a missing file reads as false, no
// error, and a path that is a directory (not a file) is a real error.
func TestAttestationExists_Negative(t *testing.T) {
	root := t.TempDir()

	exists, err := AttestationExists(root, "story-1", "ac-999")
	if err != nil {
		t.Fatalf("AttestationExists(missing): %v", err)
	}
	if exists {
		t.Fatal("AttestationExists(missing file) = true, want false")
	}

	dirPath := filepath.Join(root, ".verdi", "attestations", "story-1", "ac-2.md")
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dirPath, err)
	}
	if _, err := AttestationExists(root, "story-1", "ac-2"); err == nil {
		t.Fatal("AttestationExists(path is a directory): want error, got nil")
	}
}
