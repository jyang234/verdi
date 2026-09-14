package lint

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// emptyStoreRoot returns a fresh temp directory containing nothing but an
// empty .verdi/ — BuildSnapshot's bare minimum (TestEngine_Run_Negative_
// NotAStoreRoot already pins that a root with no .verdi/ at all is an
// operational error, not a Finding, so an otherwise-empty one is the
// smallest root BuildSnapshot accepts). CheckCandidate's own corpus never
// needs fixturegit/real git history unless a test's candidate itself
// carries a pinned ref or context[] entry that would drive VL-003 to exec
// git.
func emptyStoreRoot(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".verdi"), 0o755); err != nil {
		t.Fatalf("mkdir .verdi: %v", err)
	}
	return dir
}

// cleanFeatureCandidate is a hand-authored, fully decode-and-lint-clean
// feature spec: no stubs, problem/outcome present with resolving anchors,
// one AC carrying both a resolving anchor and the feature outcome floor's
// required attestation evidence kind (vl006.go's checkFeatureACAttestation),
// no links at all (nothing for VL-003 to chase). It is the smallest content
// CheckCandidate should ever wave through with zero findings.
const cleanFeatureCandidate = `---
id: spec/widget-a
kind: spec
title: "Widget A"
owners: [unassigned]
class: feature
problem: { text: "Users cannot do X.", anchor: problem }
outcome: { text: "Users can do X.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "Given X, when Y, then Z.", evidence: [static, attestation], anchor: ac-1 }
---
# Widget A

## Problem

Users cannot do X.

## Outcome

Users can do X.

## Ac 1

Given X, when Y, then Z.
`

// TestCheckCandidate_Happy_NoFindings is the smallest possible proof
// CheckCandidate compiles and runs at all: a single clean candidate over an
// otherwise-empty corpus produces no findings and no error.
func TestCheckCandidate_Happy_NoFindings(t *testing.T) {
	root := emptyStoreRoot(t)
	findings, err := CheckCandidate(context.Background(), root, ".verdi/specs/active/widget-a/spec.md", []byte(cleanFeatureCandidate))
	if err != nil {
		t.Fatalf("CheckCandidate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("got %d findings, want 0:\n%s", len(findings), findingsString(findings))
	}
}

// corruptDependencySpec has an unknown frontmatter field (bogus_field),
// tripping DecodeStrict's KnownFields(true) — the fixture is otherwise a
// well-formed feature spec, so the ONLY way it fails is strict decode.
const corruptDependencySpec = `---
id: spec/broken-dep
kind: spec
title: "Broken"
owners: [unassigned]
class: feature
bogus_field: true
---
# Broken
`

// dependingFeatureCandidate carries one depends-on link to spec/broken-dep
// — otherwise identical to cleanFeatureCandidate — so VL-003 already red's
// "does not resolve" on the candidate's own path; corruptDependencyFindings
// must ADDITIONALLY surface broken-dep's own decode failure by name, on
// broken-dep's own path, rather than leave the dependency's corruption
// distinguishable only as an anonymous "does not resolve".
const dependingFeatureCandidate = `---
id: spec/widget-b
kind: spec
title: "Widget B"
owners: [unassigned]
class: feature
problem: { text: "Users cannot do X.", anchor: problem }
outcome: { text: "Users can do X.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "Given X, when Y, then Z.", evidence: [static, attestation], anchor: ac-1 }
links:
  - { type: depends-on, ref: "spec/broken-dep" }
---
# Widget B

## Problem

Users cannot do X.

## Outcome

Users can do X.

## Ac 1

Given X, when Y, then Z.
`

// TestCheckCandidate_SurfacesCorruptDependency proves a dependency that
// exists on disk but fails to decode is surfaced BY NAME, on its own path,
// rather than folded silently into the candidate's own generic
// "does not resolve" finding (spec-import-contract: "surface corrupt or
// unresolvable dependencies the candidate references explicitly ... must not
// be silent").
func TestCheckCandidate_SurfacesCorruptDependency(t *testing.T) {
	root := emptyStoreRoot(t)
	depRelPath := ".verdi/specs/active/broken-dep/spec.md"
	writeTestFile(t, filepath.Join(root, depRelPath), corruptDependencySpec)

	relPath := ".verdi/specs/active/widget-b/spec.md"
	findings, err := CheckCandidate(context.Background(), root, relPath, []byte(dependingFeatureCandidate))
	if err != nil {
		t.Fatalf("CheckCandidate: %v", err)
	}

	var sawDependencyFinding bool
	for _, f := range findings {
		if f.Path == depRelPath {
			sawDependencyFinding = true
		}
	}
	if !sawDependencyFinding {
		t.Fatalf("no finding named the corrupt dependency's own path %s; got:\n%s", depRelPath, findingsString(findings))
	}
}

// TestCheckCandidate_ExcludesUnrelatedCorpusFindings proves an existing
// corpus document's own, wholly unconnected decode failure — one the
// candidate's own frontmatter never references at all — is never folded
// into the candidate's result (spec-import-contract: "Unrelated pre-existing
// corpus findings are disclosed separately"). Unlike
// TestCheckCandidate_SurfacesCorruptDependency's broken-dep, this fixture's
// unrelated-broken spec is never named by any link on the clean candidate.
func TestCheckCandidate_ExcludesUnrelatedCorpusFindings(t *testing.T) {
	root := emptyStoreRoot(t)
	unrelatedRelPath := ".verdi/specs/active/unrelated-broken/spec.md"
	writeTestFile(t, filepath.Join(root, unrelatedRelPath), strings.Replace(corruptDependencySpec, "spec/broken-dep", "spec/unrelated-broken", 1))

	findings, err := CheckCandidate(context.Background(), root, ".verdi/specs/active/widget-a/spec.md", []byte(cleanFeatureCandidate))
	if err != nil {
		t.Fatalf("CheckCandidate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("got %d findings, want 0 (the unrelated corpus decode failure must not appear here):\n%s", len(findings), findingsString(findings))
	}
}

// TestCheckCandidate_DuplicateIdentity proves a candidate whose id already
// belongs to an existing committed spec is flagged on the CANDIDATE's own
// path by VL-002's existing global-uniqueness check (vl002.go's ByRef loop)
// — target-specific filtering keeps exactly that one copy, not also the
// existing document's own mirrored duplicate finding (a conflicting peer is
// not a "dependency" this candidate references).
func TestCheckCandidate_DuplicateIdentity(t *testing.T) {
	root := emptyStoreRoot(t)
	existingRelPath := ".verdi/specs/active/widget-a/spec.md"
	writeTestFile(t, filepath.Join(root, existingRelPath), cleanFeatureCandidate)

	// The candidate declares the SAME id (spec/widget-a) but lives at a
	// different path — decode succeeds (VL-002 does not check path/id
	// agreement failure here; it checks GLOBAL ref uniqueness), so this
	// exercises the ByRef duplicate branch, not checkSpecPath.
	candidateRelPath := ".verdi/specs/active/widget-a-dup/spec.md"
	findings, err := CheckCandidate(context.Background(), root, candidateRelPath, []byte(cleanFeatureCandidate))
	if err != nil {
		t.Fatalf("CheckCandidate: %v", err)
	}
	onlyRule(t, findings, "VL-002")
	for _, f := range findings {
		if f.Path != candidateRelPath {
			t.Fatalf("finding on unexpected path %s (want only the candidate's own %s):\n%s", f.Path, candidateRelPath, findingsString(findings))
		}
	}
}
