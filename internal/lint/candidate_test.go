package lint

import (
	"context"
	"os"
	"path/filepath"
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
