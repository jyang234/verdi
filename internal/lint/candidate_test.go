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

// TestCheckCandidate_DisclosesUnrelatedCorpusFindings proves an existing
// corpus document's own, wholly unconnected failure is DISCLOSED separately
// rather than discarded (spec-import-contract: "Unrelated pre-existing
// corpus findings are disclosed separately and cannot silently validate a
// candidate whose dependencies fail to decode or resolve"): it comes back on
// its own path, at SeverityDisclosure so it never blocks, while the
// candidate itself stays usable — no candidate-path finding at all. Unlike
// TestCheckCandidate_SurfacesCorruptDependency's broken-dep, this fixture's
// unrelated-broken spec is never named by any link on the clean candidate.
func TestCheckCandidate_DisclosesUnrelatedCorpusFindings(t *testing.T) {
	root := emptyStoreRoot(t)
	unrelatedRelPath := ".verdi/specs/active/unrelated-broken/spec.md"
	writeTestFile(t, filepath.Join(root, unrelatedRelPath), strings.Replace(corruptDependencySpec, "spec/broken-dep", "spec/unrelated-broken", 1))

	candidateRelPath := ".verdi/specs/active/widget-a/spec.md"
	findings, err := CheckCandidate(context.Background(), root, candidateRelPath, []byte(cleanFeatureCandidate))
	if err != nil {
		t.Fatalf("CheckCandidate: %v", err)
	}

	var disclosed *Finding
	for i, f := range findings {
		if f.Path == candidateRelPath {
			t.Fatalf("an unrelated corpus failure was folded onto the candidate's own path:\n%s", findingsString(findings))
		}
		if f.Path == unrelatedRelPath {
			disclosed = &findings[i]
		}
	}
	if disclosed == nil {
		t.Fatalf("the unrelated corpus decode failure was discarded instead of disclosed:\n%s", findingsString(findings))
	}
	if disclosed.Severity != SeverityDisclosure {
		t.Fatalf("unrelated corpus finding severity = %v, want SeverityDisclosure (it must not block the candidate): %+v", disclosed.Severity, *disclosed)
	}
	if disclosed.Rule != "VL-001" {
		t.Fatalf("unrelated corpus finding lost its originating rule: %+v", *disclosed)
	}
	if !strings.Contains(disclosed.Message, unrelatedRelPath) || !strings.Contains(disclosed.Message, "bogus_field") {
		t.Fatalf("unrelated corpus disclosure does not carry the original path and fact: %+v", *disclosed)
	}
}

// oldNativeFeatureCandidate is a v0-shaped feature spec: no problem, no
// outcome, and one acceptance criterion carrying neither an anchor nor the
// feature outcome floor's attestation kind. vl006.isNewClassSpec classifies
// it as grandfathered, so ORDINARY corpus lint skips requiredness and the
// attestation floor for it — correctly, and unchanged.
const oldNativeFeatureCandidate = `---
id: spec/old-widget
kind: spec
title: "Old Widget"
owners: [team-a]
class: feature
acceptance_criteria:
  - { id: ac-1, text: "The widget works.", evidence: [static] }
---
# Old Widget
`

// TestCheckCandidate_OldNativeFeature_MeetsCurrentRequiredness proves a
// candidate is never grandfathered by shape: "Strict decode, new-spec
// requiredness, anchors and project checks apply even to old native inputs;
// no archive grandfathering" (spec-import-contract). The candidate seam runs
// vl006's OWN requiredness and attestation helpers for a feature
// isNewClassSpec would otherwise skip — no rule copy, no change to the
// corpus rule itself (pinned by TestCheckCandidate_OldFeatureInCorpus_
// StaysGrandfathered).
func TestCheckCandidate_OldNativeFeature_MeetsCurrentRequiredness(t *testing.T) {
	root := emptyStoreRoot(t)
	relPath := ".verdi/specs/active/old-widget/spec.md"
	findings, err := CheckCandidate(context.Background(), root, relPath, []byte(oldNativeFeatureCandidate))
	if err != nil {
		t.Fatalf("CheckCandidate: %v", err)
	}
	for _, want := range []string{
		"new-class spec has no problem attribute",
		"new-class spec has no outcome attribute",
		"acceptance criterion ac-1 has no anchor",
		"does not declare attestation among its expected evidence kinds",
	} {
		var saw bool
		for _, f := range findings {
			if f.Path == relPath && f.Severity == SeverityViolation && strings.Contains(f.Message, want) {
				saw = true
			}
		}
		if !saw {
			t.Fatalf("no blocking candidate finding names %q:\n%s", want, findingsString(findings))
		}
	}
}

// TestCheckCandidate_OldFeatureInCorpus_StaysGrandfathered is the companion
// containment proof: the SAME old-shaped spec sitting in the corpus as an
// unrelated existing document is still grandfathered by vl006's ordinary
// rule — the candidate seam forces requiredness for the CANDIDATE only, and
// changes no existing corpus behavior.
func TestCheckCandidate_OldFeatureInCorpus_StaysGrandfathered(t *testing.T) {
	root := emptyStoreRoot(t)
	oldRelPath := ".verdi/specs/active/old-widget/spec.md"
	writeTestFile(t, filepath.Join(root, oldRelPath), oldNativeFeatureCandidate)

	findings, err := CheckCandidate(context.Background(), root, ".verdi/specs/active/widget-a/spec.md", []byte(cleanFeatureCandidate))
	if err != nil {
		t.Fatalf("CheckCandidate: %v", err)
	}
	for _, f := range findings {
		if f.Path == oldRelPath {
			t.Fatalf("the old corpus spec lost its ordinary grandfathering:\n%s", findingsString(findings))
		}
	}
}

// TestCheckCandidate_DuplicateIdentity proves a candidate whose id already
// belongs to an existing committed spec is flagged BLOCKING on the
// CANDIDATE's own path by VL-002's existing global-uniqueness check
// (vl002.go's ByRef loop). The existing peer's own mirrored duplicate
// finding is not a candidate-specific violation, so it comes back as a
// nonblocking disclosure on the peer's own path — readiness is determined by
// the candidate's own failures alone.
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
	var sawCandidateViolation, sawPeerDisclosure bool
	for _, f := range findings {
		switch f.Path {
		case candidateRelPath:
			sawCandidateViolation = sawCandidateViolation || f.Severity == SeverityViolation
		case existingRelPath:
			sawPeerDisclosure = sawPeerDisclosure || f.Severity == SeverityDisclosure
		default:
			t.Fatalf("finding on unexpected path %s:\n%s", f.Path, findingsString(findings))
		}
	}
	if !sawCandidateViolation {
		t.Fatalf("want a blocking VL-002 duplicate on the candidate's own path:\n%s", findingsString(findings))
	}
	if !sawPeerDisclosure {
		t.Fatalf("want the existing peer's mirrored duplicate as a nonblocking disclosure:\n%s", findingsString(findings))
	}
}

// validDependencyFeature is a real, cleanly-decoding feature spec a
// candidate story can implement.
const validDependencyFeature = `---
id: spec/widget-feature
kind: spec
title: "Widget Feature"
owners: [unassigned]
class: feature
problem: { text: "Users cannot do X.", anchor: problem }
outcome: { text: "Users can do X.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "Given X, when Y, then Z.", evidence: [static, attestation], anchor: ac-1 }
---
# Widget Feature

## Problem

Users cannot do X.

## Outcome

Users can do X.

## Ac 1

Given X, when Y, then Z.
`

// storyImplementingValidFeature is a candidate story implementing
// validDependencyFeature's own ac-1 fragment.
const storyImplementingValidFeature = `---
id: spec/widget-story
kind: spec
title: "Widget Story"
owners: [unassigned]
class: story
story: jira:LOAN-1482
problem: { text: "Users cannot do X today.", anchor: problem }
outcome: { text: "Users can do X.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "Given X, when Y, then Z.", evidence: [static], anchor: ac-1 }
links:
  - { type: implements, ref: "spec/widget-feature#ac-1" }
---
# Widget Story

## Problem

Users cannot do X today.

## Outcome

Users can do X.

## Ac 1

Given X, when Y, then Z.
`

// TestCheckCandidate_ValidDecodingDependency_NoFinding proves a candidate
// whose implements link names a dependency that exists AND decodes
// cleanly produces no finding about that dependency at all — the
// complement of TestCheckCandidate_SurfacesCorruptDependency: a healthy
// dependency is silent, only a broken one is surfaced.
func TestCheckCandidate_ValidDecodingDependency_NoFinding(t *testing.T) {
	root := emptyStoreRoot(t)
	// The candidate story's own story: jira:LOAN-1482 tracker needs a
	// configured jira provider (VL-005) — unrelated to the dependency
	// resolution this test exercises, but otherwise a spurious finding on
	// the candidate's own path would appear alongside it.
	writeTestFile(t, filepath.Join(root, ".verdi", "verdi.yaml"), setupManifestYAML)
	featureRelPath := ".verdi/specs/active/widget-feature/spec.md"
	writeTestFile(t, filepath.Join(root, featureRelPath), validDependencyFeature)

	relPath := ".verdi/specs/active/widget-story/spec.md"
	findings, err := CheckCandidate(context.Background(), root, relPath, []byte(storyImplementingValidFeature))
	if err != nil {
		t.Fatalf("CheckCandidate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("got %d findings for a candidate whose dependency decodes and resolves cleanly, want 0:\n%s", len(findings), findingsString(findings))
	}
}
