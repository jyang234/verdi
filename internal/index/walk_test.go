package index

import "testing"

// syntheticReaffirmation is a valid `kind: reaffirmation` frontmatter
// fixture (02 §Kind registry: "(none — existence is the record)"; 03 §The
// amendment ladder rung 4, R4-I-4), modeled on
// testdata/corpus/.verdi/reaffirmations/jira-loan-1483/ac-1.md.
const syntheticReaffirmation = `---
id: reaffirmation/witness--ac-1
kind: reaffirmation
title: "witness reaffirmation"
owners: [platform-team]
frozen: { at: 2026-07-13, commit: 06a3f4cabb226fe9344e1645e27c344493b6b62b }
object: spec/my-spec@06a3f4cabb226fe9344e1645e27c344493b6b62b#ac-1
hash: { old: sha256:cba06b5736faf67e54b07b561eae94395e774c517a7d910a54369e1263ccfbd4, new: sha256:11507a0e2f5e69d5dfa40a62a1bd7b6ee57e6bcd85c67c9b8431b36fff21c437 }
---
# Witness reaffirmation

Fixture proving spec/shared-homes ac-4: index's classify table used to omit
"reaffirmations/" entirely, so this file was silently skipped by
walkCommittedZone (no error, no entry) rather than indexed.
`

// TestBuild_IndexesReaffirmation is spec/shared-homes ac-4's witness test:
// a fixture store containing a reaffirmation file must produce an indexed
// Entry for it. Before this story's fix, internal/index/walk.go's
// classifyArtifactPath table lacked the "reaffirmations/" case (unlike
// lint's copy), so ClassifyPath returned ok=false for this file and
// walkCommittedZone silently skipped it — no error, no entry, the exact
// divergence spec/shared-homes's problem statement names. Written first
// against the pre-fix code to capture the red (entry absent); now green
// against internal/artifact.ClassifyPath (shared table) and
// decodeEntry's new "reaffirmation" arm.
func TestBuild_IndexesReaffirmation(t *testing.T) {
	root := buildSyntheticStore(t)
	writeIndexFile(t, root, ".verdi/reaffirmations/witness/ac-1.md", syntheticReaffirmation)

	ix, err := Build(root)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	entry, ok := ix.Get("reaffirmation/witness--ac-1")
	if !ok {
		t.Fatal("Build: reaffirmation/witness--ac-1 not indexed — reaffirmation file was silently skipped")
	}
	if entry.Kind != "reaffirmation" {
		t.Errorf("entry.Kind = %q, want %q", entry.Kind, "reaffirmation")
	}
	if entry.Title != "witness reaffirmation" {
		t.Errorf("entry.Title = %q, want %q", entry.Title, "witness reaffirmation")
	}
}
