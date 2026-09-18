# Spec Documents — Wave 1 Implementation Plan (ac-1 document core, ac-2 CLI)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the shared document core (`internal/specdoc`) that renders a spec as a deterministic, stamped Markdown/HTML document in three kinds, and the `verdi spec doc` verb that exposes it.

**Architecture:** One pure package turns a decoded spec, its body, a stamp, and caller-supplied facts into a `Document` model, then renders it to canonical Markdown (and HTML through the existing goldmark path). Facts are never computed inside the package: coverage and claims come from the spec's own stubs (`FactsFromSpec`), evidence from the existing matrix projection (`WithMatrix`). The CLI verb is a thin consumer that loads bytes from the default branch, a pinned commit, or the working tree, resolves status through `specstate`, and writes the render.

**Tech Stack:** Go 1.25 (module `github.com/jyang234/verdi`), `gopkg.in/yaml.v3` via `internal/artifact`, goldmark via `internal/render`, `internal/canonjson` for digests, `internal/fixturegit` for hermetic repos, golden files under `testdata/` with a `-update` flag.

**Spec:** `.verdi/specs/active/spec-documents/spec.md` on branch `design/spec-documents` (PR #327). This plan implements ac-1 and ac-2 under co-1, co-2, co-3, co-5, co-6 and decisions dc-1, dc-3, dc-8, dc-9. Waves 2–4 get their own plans.

## Global Constraints

- No network in any test (co-1). CLI paths are exercised through the built binary over `internal/fixturegit` repos with `CI_DEFAULT_BRANCH=main` in the environment.
- The document is never authority (co-2): every render carries the not-authority stamp; nothing in this wave writes to the store.
- Every new production string literal that contains a class word (`feature`, `story`, `component`, `spike`) or a lifecycle state word (`proposed`, `accepted-pending-build`, `accepted`, `superseded`, `closed`) either routes it through `*model.Model` (`DisplayClass`, `DisplayState`, `DisplayClassPlural`) or is classified at the producing site with `// vocab:identity — <why>` on the literal's line or the line above (co-5; `internal/specalign/vocabprose_test.go:393` fails the gate otherwise).
- Three-valued honesty (co-6): a section whose facts were not supplied says so in fixed wording; it is never omitted.
- Determinism: identical `Input` renders byte-identical output; no wall clock, no map iteration order in output (sort every slice you build from a map).
- Exit codes: `verdi spec doc` exits 0 on a render and 2 on an unresolvable ref, commit, or store; it never exits 1 (ac-2).
- gofmt-clean, `golangci-lint` clean (`.golangci.yml`, v2 standard), `go vet` clean. Commit subjects imperative. Every commit ends with `Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>`.
- Work happens in `/Users/johnyang/code/verdi-system/verdi-wt/spec-documents-w1` on branch `agent/spec-documents-wave-1` (base `af4a07f3`). Never use bare `git stash`. Read `/Users/johnyang/code/verdi-system/CLAUDE.md` first.

---

## File structure

| File | Responsibility |
|---|---|
| `internal/specdoc/doc.go` | Package doc comment: what the core is, what it never does (co-2). |
| `internal/specdoc/kind.go` | `Kind` type, constants, `ParseKind`, the section set per kind. |
| `internal/specdoc/facts.go` | `Facts`, `ACEvidence`, `KindEvidence`; `FactsFromSpec`; `WithMatrix`. |
| `internal/specdoc/stamp.go` | `Stamp`; `EngineDigest()`. |
| `internal/specdoc/model.go` | `Document` and its item types; `Words`. |
| `internal/specdoc/body.go` | `bodySections(body []byte) map[string]string` — the `## <id>` section extractor. |
| `internal/specdoc/build.go` | `Input`; `Build(in Input) (Document, error)`. |
| `internal/specdoc/markdown.go` | `RenderMarkdown(doc Document) string` — the canonical text form. |
| `internal/specdoc/html.go` | `RenderHTML(doc Document) (string, error)`. |
| `internal/specdoc/*_test.go`, `internal/specdoc/testdata/` | Unit tests, fixture spec, golden outputs. |
| `cmd/verdi/specdoc.go` | `cmdSpecDoc` — the verb; byte loading for default-branch, `--at`, `--proposed`. |
| `cmd/verdi/specdoc_test.go` | Built-binary tests over fixturegit repos. |
| `cmd/verdi/specstate.go:41-48` | `runSpecVerb` grows the `doc` subverb. |
| `cmd/verdi/help.go:142` | `verbUsage["spec"]` grows the `doc` line. |

Package boundary: `internal/specdoc` imports `internal/artifact`, `internal/model`, `internal/matrixprojection`, `internal/render`, `internal/canonjson`. It never imports `internal/store`, `internal/gitx`, `internal/specstate`, or anything under `cmd/`.

---

### Task 1: Package skeleton — kinds, stamp, engine digest

**Files:**
- Create: `internal/specdoc/doc.go`
- Create: `internal/specdoc/kind.go`
- Create: `internal/specdoc/stamp.go`
- Test: `internal/specdoc/kind_test.go`, `internal/specdoc/stamp_test.go`

**Interfaces:**
- Produces: `type Kind string`; `KindSpec`, `KindPlan`, `KindTasks`; `func ParseKind(s string) (Kind, error)`; `type Stamp struct{ Ref, Commit, Engine string; Proposed bool }`; `func EngineDigest() string`; `type SectionID string` with the constants listed below; `func (k Kind) Sections() []SectionID`.

- [ ] **Step 1: Write the failing kind test**

```go
// internal/specdoc/kind_test.go
package specdoc

import "testing"

func TestParseKind(t *testing.T) {
	cases := []struct {
		in      string
		want    Kind
		wantErr bool
	}{
		{"spec", KindSpec, false},
		{"plan", KindPlan, false},
		{"tasks", KindTasks, false},
		{"", "", true},
		{"SPEC", "", true},
		{"task", "", true},
	}
	for _, c := range cases {
		got, err := ParseKind(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("ParseKind(%q) err = %v, wantErr %v", c.in, err, c.wantErr)
			continue
		}
		if got != c.want {
			t.Errorf("ParseKind(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestKindSections(t *testing.T) {
	all := KindSpec.Sections()
	wantAll := []SectionID{SectionIdentity, SectionProblem, SectionOutcome, SectionDecisions, SectionConstraints, SectionCriteria, SectionQuestions, SectionPlan, SectionEvidence}
	if len(all) != len(wantAll) {
		t.Fatalf("spec sections = %v, want %v", all, wantAll)
	}
	for i := range all {
		if all[i] != wantAll[i] {
			t.Fatalf("spec sections = %v, want %v", all, wantAll)
		}
	}
	plan := KindPlan.Sections()
	wantPlan := []SectionID{SectionIdentity, SectionDecisions, SectionConstraints, SectionPlan}
	if len(plan) != len(wantPlan) {
		t.Fatalf("plan sections = %v, want %v", plan, wantPlan)
	}
	tasks := KindTasks.Sections()
	wantTasks := []SectionID{SectionIdentity, SectionPlan, SectionEvidence}
	if len(tasks) != len(wantTasks) {
		t.Fatalf("tasks sections = %v, want %v", tasks, wantTasks)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/specdoc/ -run 'TestParseKind|TestKindSections'`
Expected: FAIL with `undefined: ParseKind` (build failure).

- [ ] **Step 3: Write the minimal implementation**

```go
// internal/specdoc/doc.go
// Package specdoc renders a spec as a human-readable document from its
// decoded objects, its body, a stamp, and facts supplied by the caller.
//
// The package computes nothing it can receive: criteria coverage and
// open-question claims come from the spec's own stubs (FactsFromSpec),
// evidence state from the matrix projection (WithMatrix), readiness from a
// snapshot the caller owns. It never reads the store, never runs git, and
// never writes anything. A rendered document is a projection of the
// objects and carries a stamp saying so; it is never authority
// (spec/spec-documents co-2).
package specdoc
```

```go
// internal/specdoc/kind.go
package specdoc

import "fmt"

// Kind selects which sections a document carries (spec/spec-documents dc-3).
type Kind string

const (
	KindSpec  Kind = "spec"
	KindPlan  Kind = "plan"
	KindTasks Kind = "tasks"
)

// SectionID names one document section in its fixed order.
type SectionID string

const (
	SectionIdentity    SectionID = "identity"
	SectionProblem     SectionID = "problem"
	SectionOutcome     SectionID = "outcome"
	SectionDecisions   SectionID = "decisions"
	SectionConstraints SectionID = "constraints"
	SectionCriteria    SectionID = "criteria"
	SectionQuestions   SectionID = "questions"
	SectionPlan        SectionID = "plan"
	SectionEvidence    SectionID = "evidence"
)

// ParseKind accepts exactly the three lowercase kind names.
func ParseKind(s string) (Kind, error) {
	switch Kind(s) {
	case KindSpec, KindPlan, KindTasks:
		return Kind(s), nil
	}
	return "", fmt.Errorf("specdoc: unknown document kind %q (want spec, plan, or tasks)", s)
}

// Sections returns the ordered section set for the kind. The order is
// part of the engine digest (stamp.go): changing it changes every stamp.
func (k Kind) Sections() []SectionID {
	switch k {
	case KindPlan:
		return []SectionID{SectionIdentity, SectionDecisions, SectionConstraints, SectionPlan}
	case KindTasks:
		return []SectionID{SectionIdentity, SectionPlan, SectionEvidence}
	default:
		return []SectionID{SectionIdentity, SectionProblem, SectionOutcome, SectionDecisions, SectionConstraints, SectionCriteria, SectionQuestions, SectionPlan, SectionEvidence}
	}
}
```

- [ ] **Step 4: Run the kind tests**

Run: `go test ./internal/specdoc/ -run 'TestParseKind|TestKindSections'`
Expected: PASS.

- [ ] **Step 5: Write the failing stamp test**

```go
// internal/specdoc/stamp_test.go
package specdoc

import (
	"strings"
	"testing"
)

func TestEngineDigestIsStableAndPrefixed(t *testing.T) {
	a := EngineDigest()
	b := EngineDigest()
	if a != b {
		t.Fatalf("EngineDigest not stable: %q vs %q", a, b)
	}
	if !strings.HasPrefix(a, "sha256:") || len(a) != len("sha256:")+64 {
		t.Fatalf("EngineDigest = %q, want sha256:<64 hex>", a)
	}
}

func TestEngineDigestCoversSectionOrder(t *testing.T) {
	// The descriptor must include every kind's section order so that a
	// reorder changes the digest. Prove by digesting a descriptor with one
	// kind's order reversed and comparing.
	got := engineDescriptorDigest(engineDescriptor{ID: engineID, Version: engineVersion, Sections: map[Kind][]SectionID{KindSpec: {SectionOutcome, SectionProblem}}})
	if got == EngineDigest() {
		t.Fatalf("a different section order produced the same digest")
	}
}
```

- [ ] **Step 6: Run it to verify it fails**

Run: `go test ./internal/specdoc/ -run TestEngineDigest`
Expected: FAIL with `undefined: EngineDigest`.

- [ ] **Step 7: Write the stamp implementation**

```go
// internal/specdoc/stamp.go
package specdoc

import "github.com/jyang234/verdi/internal/canonjson"

// Stamp identifies exactly which bytes a document was rendered from.
type Stamp struct {
	// Ref is the canonical spec ref, e.g. "spec/uat-round-1".
	Ref string
	// Commit is the full commit the spec bytes were read at.
	Commit string
	// Engine is EngineDigest() at render time.
	Engine string
	// Proposed is true when the bytes are not the accepted bytes on the
	// default branch (a design-branch render). The header says so.
	Proposed bool
}

const (
	engineID      = "verdi.specdoc"
	engineVersion = 1
)

type engineDescriptor struct {
	ID       string               `json:"id"`
	Version  int                  `json:"version"`
	Sections map[Kind][]SectionID `json:"sections"`
}

func engineDescriptorDigest(d engineDescriptor) string {
	digest, err := canonjson.Digest(d)
	if err != nil {
		// The descriptor is a fixed literal; a marshal failure is a
		// programming error, never an input error.
		panic("specdoc: engine descriptor is not canonical-JSON encodable: " + err.Error())
	}
	return digest
}

// EngineDigest is the canonical digest of the renderer's identity: its
// id, version, and every kind's section order. It changes whenever the
// document shape changes, so two renders with equal stamps have equal
// shape.
func EngineDigest() string {
	return engineDescriptorDigest(engineDescriptor{
		ID:      engineID,
		Version: engineVersion,
		Sections: map[Kind][]SectionID{
			KindSpec:  KindSpec.Sections(),
			KindPlan:  KindPlan.Sections(),
			KindTasks: KindTasks.Sections(),
		},
	})
}
```

- [ ] **Step 8: Run the stamp tests**

Run: `go test -race ./internal/specdoc/`
Expected: PASS (four tests).

- [ ] **Step 9: Commit**

```bash
git add internal/specdoc/doc.go internal/specdoc/kind.go internal/specdoc/stamp.go internal/specdoc/kind_test.go internal/specdoc/stamp_test.go
git commit -m "Add internal/specdoc kinds, sections, and engine digest"
```

---

### Task 2: Facts — coverage and claims from stubs, evidence from the matrix

**Files:**
- Create: `internal/specdoc/facts.go`
- Test: `internal/specdoc/facts_test.go`

**Interfaces:**
- Consumes: `artifact.SpecFrontmatter` (`internal/artifact/spec.go:192`), `artifact.Stub{Slug, Spike, AcceptanceCriteria, Resolves}` (`object.go:134`), `matrixprojection.Record{Story *StoryBody, Feature *FeatureBody}` (`internal/matrixprojection/record.go:50`), `FeatureAC{ID, Text, Status, Summary, ImplementingStories []string}` (`record.go:105`), `StoryAC{ID, Text, Status, Summary, Kinds []KindProjection}` (`record.go:74`), `KindProjection{Kind string, Satisfied bool, ...}` (`record.go:83`).
- Produces: `type Facts struct{ Coverage map[string][]string; Claims map[string][]string; Evidence map[string]ACEvidence; EvidenceSource string }`; `type ACEvidence struct{ Status, Summary string; Stories []string; Kinds []KindEvidence }`; `type KindEvidence struct{ Kind string; Satisfied bool }`; `func FactsFromSpec(fm *artifact.SpecFrontmatter) Facts`; `func WithMatrix(f Facts, rec matrixprojection.Record, source string) Facts`.

- [ ] **Step 1: Write the failing facts test**

```go
// internal/specdoc/facts_test.go
package specdoc

import (
	"reflect"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/matrixprojection"
)

func TestFactsFromSpec(t *testing.T) {
	fm := &artifact.SpecFrontmatter{
		AcceptanceCriteria: []artifact.AcceptanceCriterion{{ID: "ac-1"}, {ID: "ac-2"}, {ID: "ac-3"}},
		OpenQuestions:      []artifact.OpenQuestion{{ID: "oq-1"}, {ID: "oq-2"}},
		Stubs: []artifact.Stub{
			{Slug: "zeta", AcceptanceCriteria: []string{"ac-2", "ac-1"}},
			{Slug: "alpha", AcceptanceCriteria: []string{"ac-1"}},
			{Slug: "probe", Spike: true, Resolves: []string{"oq-2"}},
			{Slug: "again", Spike: true, Resolves: []string{"oq-2"}},
		},
	}
	got := FactsFromSpec(fm)
	wantCoverage := map[string][]string{"ac-1": {"alpha", "zeta"}, "ac-2": {"zeta"}, "ac-3": {}}
	if !reflect.DeepEqual(got.Coverage, wantCoverage) {
		t.Errorf("Coverage = %v, want %v", got.Coverage, wantCoverage)
	}
	wantClaims := map[string][]string{"oq-1": {}, "oq-2": {"again", "probe"}}
	if !reflect.DeepEqual(got.Claims, wantClaims) {
		t.Errorf("Claims = %v, want %v", got.Claims, wantClaims)
	}
	if got.Evidence != nil || got.EvidenceSource != "" {
		t.Errorf("Evidence must be unavailable from the spec alone, got %v / %q", got.Evidence, got.EvidenceSource)
	}
}

func TestFactsFromSpecNilAndEmpty(t *testing.T) {
	if got := FactsFromSpec(nil); got.Coverage != nil || got.Claims != nil {
		t.Fatalf("nil spec must yield unavailable facts, got %+v", got)
	}
	got := FactsFromSpec(&artifact.SpecFrontmatter{})
	if got.Coverage == nil || len(got.Coverage) != 0 || got.Claims == nil || len(got.Claims) != 0 {
		t.Fatalf("empty spec must yield empty (known) maps, got %+v", got)
	}
}

func TestWithMatrixFeature(t *testing.T) {
	base := FactsFromSpec(&artifact.SpecFrontmatter{AcceptanceCriteria: []artifact.AcceptanceCriterion{{ID: "ac-1"}}})
	rec := matrixprojection.Record{Feature: &matrixprojection.FeatureBody{ACs: []matrixprojection.FeatureAC{
		{ID: "ac-1", Status: "violated", Summary: "no implementing story", ImplementingStories: []string{"spec/b", "spec/a"}},
	}}}
	got := WithMatrix(base, rec, "matrix at abc123")
	want := map[string]ACEvidence{"ac-1": {Status: "violated", Summary: "no implementing story", Stories: []string{"spec/a", "spec/b"}}}
	if !reflect.DeepEqual(got.Evidence, want) {
		t.Errorf("Evidence = %+v, want %+v", got.Evidence, want)
	}
	if got.EvidenceSource != "matrix at abc123" {
		t.Errorf("EvidenceSource = %q", got.EvidenceSource)
	}
	if !reflect.DeepEqual(got.Coverage, base.Coverage) {
		t.Errorf("WithMatrix must not touch Coverage")
	}
}

func TestWithMatrixStory(t *testing.T) {
	base := Facts{}
	rec := matrixprojection.Record{Story: &matrixprojection.StoryBody{ACs: []matrixprojection.StoryAC{
		{ID: "ac-1", Status: "satisfied", Summary: "all kinds proven", Kinds: []matrixprojection.KindProjection{{Kind: "behavioral", Satisfied: true}, {Kind: "attestation", Satisfied: false}}},
	}}}
	got := WithMatrix(base, rec, "s")
	want := map[string]ACEvidence{"ac-1": {Status: "satisfied", Summary: "all kinds proven", Kinds: []KindEvidence{{Kind: "behavioral", Satisfied: true}, {Kind: "attestation", Satisfied: false}}}}
	if !reflect.DeepEqual(got.Evidence, want) {
		t.Errorf("Evidence = %+v, want %+v", got.Evidence, want)
	}
}

func TestWithMatrixEmptyRecordKeepsUnavailable(t *testing.T) {
	got := WithMatrix(Facts{}, matrixprojection.Record{}, "s")
	if got.Evidence != nil || got.EvidenceSource != "" {
		t.Fatalf("a record with neither body must leave evidence unavailable, got %+v", got)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/specdoc/ -run 'TestFactsFromSpec|TestWithMatrix'`
Expected: FAIL with `undefined: FactsFromSpec`.

- [ ] **Step 3: Write the facts implementation**

```go
// internal/specdoc/facts.go
package specdoc

import (
	"sort"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/matrixprojection"
)

// Facts are the computed inputs a document reports but never derives.
// A nil map means "not supplied for this render" and the section says so
// (spec/spec-documents co-6); an empty, non-nil map means "supplied, and
// there is nothing".
type Facts struct {
	// Coverage maps a criterion id to the sorted slugs of the stubs that
	// name it in acceptance_criteria.
	Coverage map[string][]string
	// Claims maps an open-question id to the sorted slugs of the spike
	// stubs that name it in resolves.
	Claims map[string][]string
	// Evidence maps a criterion id to its state in the matrix projection.
	Evidence map[string]ACEvidence
	// EvidenceSource names where Evidence came from, e.g. "matrix at
	// <commit>"; empty when Evidence is nil.
	EvidenceSource string
}

// ACEvidence is one criterion's evidence state as the matrix reports it.
type ACEvidence struct {
	Status  string
	Summary string
	// Stories is the sorted list of implementing story refs (feature
	// records only).
	Stories []string
	// Kinds is the per-evidence-kind state in the record's own order
	// (story records only).
	Kinds []KindEvidence
}

// KindEvidence is one evidence kind's satisfaction for a story criterion.
type KindEvidence struct {
	Kind      string
	Satisfied bool
}

// FactsFromSpec derives coverage and claims from the spec's own stubs.
// Every declared criterion and question gets an entry, so an uncovered
// criterion reads as "known: nothing covers it", not as "unknown".
// Evidence stays unavailable: the spec alone cannot know it.
func FactsFromSpec(fm *artifact.SpecFrontmatter) Facts {
	if fm == nil {
		return Facts{}
	}
	coverage := make(map[string][]string, len(fm.AcceptanceCriteria))
	for _, ac := range fm.AcceptanceCriteria {
		coverage[ac.ID] = []string{}
	}
	claims := make(map[string][]string, len(fm.OpenQuestions))
	for _, oq := range fm.OpenQuestions {
		claims[oq.ID] = []string{}
	}
	for _, st := range fm.Stubs {
		if st.Spike {
			for _, id := range st.Resolves {
				if _, declared := claims[id]; declared {
					claims[id] = append(claims[id], st.Slug)
				}
			}
			continue
		}
		for _, id := range st.AcceptanceCriteria {
			if _, declared := coverage[id]; declared {
				coverage[id] = append(coverage[id], st.Slug)
			}
		}
	}
	for id := range coverage {
		sort.Strings(coverage[id])
	}
	for id := range claims {
		sort.Strings(claims[id])
	}
	return Facts{Coverage: coverage, Claims: claims}
}

// WithMatrix copies f and fills Evidence from a matrix projection record.
// A record with neither a feature nor a story body leaves Evidence
// unavailable. source is recorded verbatim as EvidenceSource.
func WithMatrix(f Facts, rec matrixprojection.Record, source string) Facts {
	out := f
	switch {
	case rec.Feature != nil:
		out.Evidence = make(map[string]ACEvidence, len(rec.Feature.ACs))
		for _, ac := range rec.Feature.ACs {
			stories := append([]string(nil), ac.ImplementingStories...)
			sort.Strings(stories)
			out.Evidence[ac.ID] = ACEvidence{Status: ac.Status, Summary: ac.Summary, Stories: stories}
		}
	case rec.Story != nil:
		out.Evidence = make(map[string]ACEvidence, len(rec.Story.ACs))
		for _, ac := range rec.Story.ACs {
			kinds := make([]KindEvidence, 0, len(ac.Kinds))
			for _, k := range ac.Kinds {
				kinds = append(kinds, KindEvidence{Kind: k.Kind, Satisfied: k.Satisfied})
			}
			out.Evidence[ac.ID] = ACEvidence{Status: ac.Status, Summary: ac.Summary, Kinds: kinds}
		}
	default:
		return out
	}
	out.EvidenceSource = source
	return out
}
```

- [ ] **Step 4: Run the tests**

Run: `go test -race ./internal/specdoc/ -run 'TestFactsFromSpec|TestWithMatrix'`
Expected: PASS (the feature case carries sorted `Stories` and nil `Kinds`; the story case carries nil `Stories`; both literals in the test already match that).

- [ ] **Step 5: Commit**

```bash
git add internal/specdoc/facts.go internal/specdoc/facts_test.go
git commit -m "Add specdoc facts: coverage and claims from stubs, evidence from the matrix"
```

---

### Task 3: Body section extractor

**Files:**
- Create: `internal/specdoc/body.go`
- Test: `internal/specdoc/body_test.go`

**Interfaces:**
- Produces: `func bodySections(body []byte) map[string]string` — maps an object id (`ac-1`, `dc-2`, `co-3`, `oq-1`) to the trimmed Markdown text under its `## <id>` heading, up to the next `## ` heading. Headings that are not an object id (`## Problem`, `## Outcome`) are keyed by their lowercased text (`problem`, `outcome`).

- [ ] **Step 1: Write the failing test**

```go
// internal/specdoc/body_test.go
package specdoc

import "testing"

func TestBodySections(t *testing.T) {
	body := []byte("# Title\n\nintro\n\n## Problem\n\nthe problem\nspans lines\n\n## ac-1\n\nrationale one\n\n### ac-1 sub\n\nkept inside\n\n## dc-2\n\n## oq-1\ntight\n")
	got := bodySections(body)
	cases := map[string]string{
		"problem": "the problem\nspans lines",
		"ac-1":    "rationale one\n\n### ac-1 sub\n\nkept inside",
		"dc-2":    "",
		"oq-1":    "tight",
	}
	for k, want := range cases {
		if got[k] != want {
			t.Errorf("section %q = %q, want %q", k, got[k], want)
		}
	}
	if _, ok := got["title"]; ok {
		t.Errorf("an H1 must not become a section")
	}
	if len(got) != len(cases) {
		t.Errorf("sections = %v, want exactly %d entries", got, len(cases))
	}
}

func TestBodySectionsEmptyAndNoHeadings(t *testing.T) {
	if got := bodySections(nil); len(got) != 0 {
		t.Fatalf("nil body → %v", got)
	}
	if got := bodySections([]byte("just prose\n")); len(got) != 0 {
		t.Fatalf("no headings → %v", got)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/specdoc/ -run TestBodySections`
Expected: FAIL with `undefined: bodySections`.

- [ ] **Step 3: Write the implementation**

```go
// internal/specdoc/body.go
package specdoc

import (
	"strings"
)

// bodySections splits a spec body on its level-two headings. The scaffold
// and every authored spec key object bodies as "## <id>" (02 §Object
// model anchors), so the id under a heading is the map key; a prose
// heading such as "## Problem" is keyed by its lowercased text. Text is
// trimmed of surrounding blank lines; level-three headings and below stay
// inside their section.
func bodySections(body []byte) map[string]string {
	out := map[string]string{}
	if len(body) == 0 {
		return out
	}
	lines := strings.Split(strings.ReplaceAll(string(body), "\r\n", "\n"), "\n")
	key := ""
	var buf []string
	flush := func() {
		if key != "" {
			out[key] = strings.Trim(strings.Join(buf, "\n"), "\n")
		}
		buf = buf[:0]
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			flush()
			key = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(line, "## ")))
			continue
		}
		if key != "" {
			buf = append(buf, line)
		}
	}
	flush()
	return out
}
```

- [ ] **Step 4: Run the tests**

Run: `go test -race ./internal/specdoc/ -run TestBodySections`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/specdoc/body.go internal/specdoc/body_test.go
git commit -m "Add specdoc body section extractor keyed by object id"
```

---

### Task 4: Document model and Build

**Files:**
- Create: `internal/specdoc/model.go`
- Create: `internal/specdoc/build.go`
- Test: `internal/specdoc/build_test.go`

**Interfaces:**
- Consumes: Task 1 (`Kind`, `Stamp`, `SectionID`), Task 2 (`Facts`), Task 3 (`bodySections`), `artifact.SpecFrontmatter` fields `Base.ID`, `Base.Title`, `Base.Links`, `Class`, `Problem`, `Outcome` (`*artifact.Attribute{Text, Anchor}`), `AcceptanceCriteria`, `Constraints`, `Decisions`, `OpenQuestions`, `Stubs`, `Supersession`; `artifact.WholeSpecSupersedesRefs(links []artifact.Link) []artifact.Ref` (`supersession.go:80`); `(*model.Model).DisplayClass(id string) string`, `DisplayState(class, id string) string` (nil-receiver safe).
- Produces: the types below and `func Build(in Input) (Document, error)`.

- [ ] **Step 1: Write the model**

```go
// internal/specdoc/model.go
package specdoc

// Words carries every class word the renderer speaks, resolved through the
// model display chain at Build time so no renderer literal names a class.
type Words struct {
	Feature string
	Story   string
	Spike   string
}

// KV is one identity row.
type KV struct {
	Label string
	Value string
}

// Item is a decision or constraint: its id, its declared text, and the
// rationale found under its body heading (empty when the body has none).
type Item struct {
	ID     string
	Text   string
	Detail string
}

// Criterion is one acceptance criterion with its evidence kinds (as
// declared, verbatim) and its computed coverage.
type Criterion struct {
	ID       string
	Text     string
	Evidence []string
	// Coverage lists covering stub slugs; CoverageKnown is false when the
	// facts did not supply coverage.
	Coverage      []string
	CoverageKnown bool
	Detail        string
}

// Question is one open question with its claiming spike stubs.
type Question struct {
	ID          string
	Text        string
	Claims      []string
	ClaimsKnown bool
	Detail      string
}

// PlanItem is one stub: a planned story covering criteria, or a spike
// resolving questions.
type PlanItem struct {
	Slug     string
	Spike    bool
	Criteria []string
	Resolves []string
}

// EvidenceRow is one criterion's matrix state.
type EvidenceRow struct {
	ID       string
	Status   string
	Summary  string
	Stories  []string
	Kinds    []KindEvidence
}

// Document is the fully resolved model a renderer walks. Every slice is in
// declaration order; nothing is sorted at render time.
type Document struct {
	Kind     Kind
	Stamp    Stamp
	Sections []SectionID
	Words    Words

	Title    string
	Identity []KV

	// ProblemText/OutcomeText are the declared attribute texts; the
	// *Known flags are false when the spec declares none (a story spec).
	ProblemText  string
	ProblemKnown bool
	OutcomeText  string
	OutcomeKnown bool

	Decisions   []Item
	Constraints []Item
	Criteria    []Criterion
	Questions   []Question
	Plan        []PlanItem

	Evidence       []EvidenceRow
	EvidenceKnown  bool
	EvidenceSource string
}
```

- [ ] **Step 2: Write the failing build test**

```go
// internal/specdoc/build_test.go
package specdoc

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

func fixtureSpec(t *testing.T) (*artifact.SpecFrontmatter, []byte) {
	t.Helper()
	doc := []byte(`---
id: spec/lockbox
kind: spec
title: "Lockbox"
owners: [platform-team]
class: feature
links:
  - { type: supersedes, ref: "spec/lockbox-v0" }
problem: { text: "Keys are shared.", anchor: problem }
outcome: { text: "Each key has one holder.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "A key opens one box.", evidence: [behavioral, attestation], anchor: ac-1 }
  - { id: ac-2, text: "A lost key is revoked.", evidence: [static], anchor: ac-2 }
constraints:
  - { id: co-1, text: "No network.", anchor: co-1 }
decisions:
  - { id: dc-1, text: "One holder per key.", anchor: dc-1 }
open_questions:
  - { id: oq-1, text: "Who audits holders?", anchor: oq-1 }
  - { id: oq-2, text: "How long is a revocation valid?", anchor: oq-2 }
stubs:
  - { slug: key-holder, acceptance_criteria: [ac-1] }
  - { slug: audit-probe, spike: true, resolves: [oq-1] }
supersession:
  carried: [ac-1]
  amended: [ { id: ac-2, note: "tightened" } ]
  amended_advisory: []
  removed: []
  added: [co-1, dc-1, oq-1, oq-2]
---
# Lockbox

## Problem

Keys are shared today.

## Outcome

One holder.

## ac-1

Proven by opening.

## ac-2

## co-1

## dc-1

Because two holders means no holder.

## oq-1

## oq-2
`)
	fmBytes, body, err := artifact.SplitFrontmatter(doc)
	if err != nil {
		t.Fatal(err)
	}
	_ = fmBytes
	fm, err := artifact.DecodeSpec(doc)
	if err != nil {
		t.Fatal(err)
	}
	return fm, body
}

func TestBuildSpecKind(t *testing.T) {
	fm, body := fixtureSpec(t)
	in := Input{Spec: fm, Body: body, Status: "accepted-pending-build", Stamp: Stamp{Ref: "spec/lockbox", Commit: strings.Repeat("a", 40)}, Facts: FactsFromSpec(fm), Kind: KindSpec}
	doc, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Title != "Lockbox" || doc.Kind != KindSpec {
		t.Fatalf("title/kind = %q/%q", doc.Title, doc.Kind)
	}
	if doc.Stamp.Engine != EngineDigest() {
		t.Fatalf("Build must fill Stamp.Engine")
	}
	if !doc.ProblemKnown || doc.ProblemText != "Keys are shared." || !doc.OutcomeKnown {
		t.Fatalf("problem/outcome = %+v", doc)
	}
	if len(doc.Decisions) != 1 || doc.Decisions[0].Detail != "Because two holders means no holder." {
		t.Fatalf("decisions = %+v", doc.Decisions)
	}
	if len(doc.Criteria) != 2 || !doc.Criteria[0].CoverageKnown || len(doc.Criteria[0].Coverage) != 1 || doc.Criteria[0].Coverage[0] != "key-holder" || doc.Criteria[0].Detail != "Proven by opening." {
		t.Fatalf("criteria = %+v", doc.Criteria)
	}
	if len(doc.Criteria[1].Coverage) != 0 || !doc.Criteria[1].CoverageKnown {
		t.Fatalf("ac-2 must be known-uncovered, got %+v", doc.Criteria[1])
	}
	if len(doc.Questions) != 2 || doc.Questions[0].Claims[0] != "audit-probe" || len(doc.Questions[1].Claims) != 0 {
		t.Fatalf("questions = %+v", doc.Questions)
	}
	if len(doc.Plan) != 2 || doc.Plan[0].Slug != "key-holder" || !doc.Plan[1].Spike {
		t.Fatalf("plan = %+v", doc.Plan)
	}
	if doc.EvidenceKnown {
		t.Fatalf("evidence must be unknown when facts carry none")
	}
	var supersedes string
	for _, kv := range doc.Identity {
		if kv.Label == "Supersedes" {
			supersedes = kv.Value
		}
	}
	if supersedes != "spec/lockbox-v0" {
		t.Fatalf("identity rows = %+v", doc.Identity)
	}
	if doc.Words.Story == "" || doc.Words.Spike == "" || doc.Words.Feature == "" {
		t.Fatalf("words must resolve through a nil model to the bare ids: %+v", doc.Words)
	}
}

func TestBuildKindsSelectSections(t *testing.T) {
	fm, body := fixtureSpec(t)
	for _, k := range []Kind{KindSpec, KindPlan, KindTasks} {
		doc, err := Build(Input{Spec: fm, Body: body, Kind: k, Stamp: Stamp{Ref: "spec/lockbox", Commit: strings.Repeat("b", 40)}, Facts: FactsFromSpec(fm)})
		if err != nil {
			t.Fatal(err)
		}
		if len(doc.Sections) != len(k.Sections()) {
			t.Errorf("%s sections = %v", k, doc.Sections)
		}
	}
}

func TestBuildUnknownFactsAndEvidence(t *testing.T) {
	fm, body := fixtureSpec(t)
	doc, err := Build(Input{Spec: fm, Body: body, Kind: KindSpec, Stamp: Stamp{Ref: "spec/lockbox", Commit: strings.Repeat("c", 40)}})
	if err != nil {
		t.Fatal(err)
	}
	if doc.Criteria[0].CoverageKnown || doc.Questions[0].ClaimsKnown {
		t.Fatalf("nil facts must render as unknown coverage/claims: %+v", doc.Criteria[0])
	}
	withEvidence := Facts{Evidence: map[string]ACEvidence{"ac-2": {Status: "violated", Summary: "nothing implements it"}}, EvidenceSource: "matrix at c"}
	doc, err = Build(Input{Spec: fm, Body: body, Kind: KindTasks, Stamp: Stamp{Ref: "spec/lockbox", Commit: strings.Repeat("c", 40)}, Facts: withEvidence})
	if err != nil {
		t.Fatal(err)
	}
	if !doc.EvidenceKnown || len(doc.Evidence) != 2 || doc.Evidence[0].ID != "ac-1" || doc.Evidence[0].Status != "" || doc.Evidence[1].Status != "violated" {
		t.Fatalf("evidence rows must follow criteria order and carry blanks for absent ids: %+v", doc.Evidence)
	}
}

func TestBuildRejectsBadInput(t *testing.T) {
	fm, body := fixtureSpec(t)
	cases := []struct {
		name string
		in   Input
	}{
		{"nil spec", Input{Body: body, Kind: KindSpec, Stamp: Stamp{Ref: "spec/x", Commit: strings.Repeat("a", 40)}}},
		{"bad kind", Input{Spec: fm, Body: body, Kind: "chapter", Stamp: Stamp{Ref: "spec/x", Commit: strings.Repeat("a", 40)}}},
		{"empty ref", Input{Spec: fm, Body: body, Kind: KindSpec, Stamp: Stamp{Commit: strings.Repeat("a", 40)}}},
		{"short commit", Input{Spec: fm, Body: body, Kind: KindSpec, Stamp: Stamp{Ref: "spec/x", Commit: "abc"}}},
	}
	for _, c := range cases {
		if _, err := Build(c.in); err == nil {
			t.Errorf("%s: want error", c.name)
		}
	}
}
```

- [ ] **Step 3: Run it to verify it fails**

Run: `go test ./internal/specdoc/ -run TestBuild`
Expected: FAIL with `undefined: Input` / `undefined: Build`.

- [ ] **Step 4: Write Build**

```go
// internal/specdoc/build.go
package specdoc

import (
	"fmt"
	"regexp"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/model"
)

// Input is everything Build needs. Spec and Body are the decoded halves of
// one spec.md; Status is the effective lifecycle status id the caller
// resolved (empty when unresolved); Facts may be the zero value.
type Input struct {
	Spec   *artifact.SpecFrontmatter
	Body   []byte
	Status string
	Stamp  Stamp
	Facts  Facts
	Model  *model.Model
	Kind   Kind
}

var fullShaRe = regexp.MustCompile(`^[0-9a-f]{40}$`)

// Build resolves an Input into a Document. It validates the stamp, fills
// Stamp.Engine, resolves display words through the model, and copies
// facts onto each object in declaration order.
func Build(in Input) (Document, error) {
	if in.Spec == nil {
		return Document{}, fmt.Errorf("specdoc: nil spec")
	}
	if _, err := ParseKind(string(in.Kind)); err != nil {
		return Document{}, err
	}
	if in.Stamp.Ref == "" {
		return Document{}, fmt.Errorf("specdoc: stamp has no ref")
	}
	if !fullShaRe.MatchString(in.Stamp.Commit) {
		return Document{}, fmt.Errorf("specdoc: stamp commit %q is not a full sha", in.Stamp.Commit)
	}

	sections := bodySections(in.Body)
	doc := Document{
		Kind:     in.Kind,
		Stamp:    in.Stamp,
		Sections: in.Kind.Sections(),
		Words: Words{
			Feature: in.Model.DisplayClass(string(artifact.ClassFeature)),
			Story:   in.Model.DisplayClass(string(artifact.ClassStory)),
			Spike:   in.Model.DisplayClass("spike"),
		},
		Title: in.Spec.Title,
	}
	doc.Stamp.Engine = EngineDigest()

	doc.Identity = identityRows(in)

	if in.Spec.Problem != nil {
		doc.ProblemKnown = true
		doc.ProblemText = in.Spec.Problem.Text
	}
	if in.Spec.Outcome != nil {
		doc.OutcomeKnown = true
		doc.OutcomeText = in.Spec.Outcome.Text
	}
	for _, d := range in.Spec.Decisions {
		doc.Decisions = append(doc.Decisions, Item{ID: d.ID, Text: d.Text, Detail: sections[d.ID]})
	}
	for _, c := range in.Spec.Constraints {
		doc.Constraints = append(doc.Constraints, Item{ID: c.ID, Text: c.Text, Detail: sections[c.ID]})
	}
	for _, ac := range in.Spec.AcceptanceCriteria {
		cr := Criterion{ID: ac.ID, Text: ac.Text, Detail: sections[ac.ID]}
		for _, e := range ac.Evidence {
			cr.Evidence = append(cr.Evidence, string(e))
		}
		if in.Facts.Coverage != nil {
			cr.CoverageKnown = true
			cr.Coverage = append([]string{}, in.Facts.Coverage[ac.ID]...)
		}
		doc.Criteria = append(doc.Criteria, cr)
	}
	for _, oq := range in.Spec.OpenQuestions {
		q := Question{ID: oq.ID, Text: oq.Text, Detail: sections[oq.ID]}
		if in.Facts.Claims != nil {
			q.ClaimsKnown = true
			q.Claims = append([]string{}, in.Facts.Claims[oq.ID]...)
		}
		doc.Questions = append(doc.Questions, q)
	}
	for _, st := range in.Spec.Stubs {
		doc.Plan = append(doc.Plan, PlanItem{
			Slug:     st.Slug,
			Spike:    st.Spike,
			Criteria: append([]string(nil), st.AcceptanceCriteria...),
			Resolves: append([]string(nil), st.Resolves...),
		})
	}
	if in.Facts.Evidence != nil {
		doc.EvidenceKnown = true
		doc.EvidenceSource = in.Facts.EvidenceSource
		for _, ac := range in.Spec.AcceptanceCriteria {
			ev := in.Facts.Evidence[ac.ID]
			doc.Evidence = append(doc.Evidence, EvidenceRow{ID: ac.ID, Status: ev.Status, Summary: ev.Summary, Stories: ev.Stories, Kinds: ev.Kinds})
		}
	}
	return doc, nil
}

// identityRows builds the identity table. Labels are fixed English; the
// class and status values go through the display chain.
func identityRows(in Input) []KV {
	rows := []KV{
		{Label: "Ref", Value: in.Stamp.Ref},
		{Label: "Class", Value: in.Model.DisplayClass(string(in.Spec.Class))},
	}
	if in.Status != "" {
		rows = append(rows, KV{Label: "Status", Value: in.Model.DisplayState(string(in.Spec.Class), in.Status)})
	} else {
		rows = append(rows, KV{Label: "Status", Value: "not resolved for this render"})
	}
	rows = append(rows, KV{Label: "Commit", Value: in.Stamp.Commit})
	for _, ref := range artifact.WholeSpecSupersedesRefs(in.Spec.Links) {
		rows = append(rows, KV{Label: "Supersedes", Value: ref.String()})
	}
	if in.Spec.Supersession != nil {
		s := in.Spec.Supersession
		rows = append(rows, KV{Label: "Revision", Value: fmt.Sprintf("%d carried, %d amended, %d amended (advisory), %d removed, %d added", len(s.Carried), len(s.Amended), len(s.AmendedAdvisory), len(s.Removed), len(s.Added))})
	}
	return rows
}
```

- [ ] **Step 5: Run the build tests**

Run: `go test -race ./internal/specdoc/ -run TestBuild`
Expected: PASS. If `TestBuildSpecKind` fails on `Words` being empty, confirm `(*model.Model).DisplayClass` on a nil receiver returns the bare id (`internal/model/model.go:207`); it does, so the failure would be in the test fixture.

- [ ] **Step 6: Commit**

```bash
git add internal/specdoc/model.go internal/specdoc/build.go internal/specdoc/build_test.go
git commit -m "Add specdoc document model and Build"
```

---

### Task 5: Markdown renderer with golden files

**Files:**
- Create: `internal/specdoc/markdown.go`
- Create: `internal/specdoc/testdata/lockbox.md` (the fixture spec from Task 4, verbatim)
- Create: `internal/specdoc/testdata/golden-spec.md`, `golden-plan.md`, `golden-tasks.md`, `golden-spec-proposed.md`, `golden-spec-nofacts.md` (generated with `-update`)
- Test: `internal/specdoc/markdown_test.go`

**Interfaces:**
- Consumes: Task 4 `Document`.
- Produces: `func RenderMarkdown(doc Document) string`. The exact layout below is the contract every other consumer (board tab, docs site, MCP) will compare against in Wave 2's parity test, so it is fixed here.

- [ ] **Step 1: Write the failing golden test**

```go
// internal/specdoc/markdown_test.go
package specdoc

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/matrixprojection"
)

var updateGolden = flag.Bool("update", false, "rewrite golden files under testdata/")

func loadFixture(t *testing.T) (*artifact.SpecFrontmatter, []byte) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "lockbox.md"))
	if err != nil {
		t.Fatal(err)
	}
	_, body, err := artifact.SplitFrontmatter(raw)
	if err != nil {
		t.Fatal(err)
	}
	fm, err := artifact.DecodeSpec(raw)
	if err != nil {
		t.Fatal(err)
	}
	return fm, body
}

func checkGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *updateGolden {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing golden %s (run with -update): %v", path, err)
	}
	if string(want) != got {
		t.Errorf("%s differs from golden; run `go test ./internal/specdoc/ -update` and review the diff\n--- got ---\n%s", name, got)
	}
}

func TestRenderMarkdownGoldens(t *testing.T) {
	fm, body := loadFixture(t)
	commit := strings.Repeat("0", 39) + "1"
	facts := WithMatrix(FactsFromSpec(fm), matrixFixture(), "matrix at "+commit[:8])
	for _, k := range []Kind{KindSpec, KindPlan, KindTasks} {
		doc, err := Build(Input{Spec: fm, Body: body, Status: "accepted-pending-build", Stamp: Stamp{Ref: "spec/lockbox", Commit: commit}, Facts: facts, Kind: k})
		if err != nil {
			t.Fatal(err)
		}
		checkGolden(t, "golden-"+string(k)+".md", RenderMarkdown(doc))
	}
	proposed, err := Build(Input{Spec: fm, Body: body, Status: "proposed", Stamp: Stamp{Ref: "spec/lockbox", Commit: commit, Proposed: true}, Facts: FactsFromSpec(fm), Kind: KindSpec})
	if err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "golden-spec-proposed.md", RenderMarkdown(proposed))
	nofacts, err := Build(Input{Spec: fm, Body: body, Stamp: Stamp{Ref: "spec/lockbox", Commit: commit}, Kind: KindSpec})
	if err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "golden-spec-nofacts.md", RenderMarkdown(nofacts))
}

func TestRenderMarkdownIsDeterministic(t *testing.T) {
	fm, body := loadFixture(t)
	in := Input{Spec: fm, Body: body, Stamp: Stamp{Ref: "spec/lockbox", Commit: strings.Repeat("d", 40)}, Facts: FactsFromSpec(fm), Kind: KindSpec}
	a, _ := Build(in)
	b, _ := Build(in)
	if RenderMarkdown(a) != RenderMarkdown(b) {
		t.Fatal("two renders of the same input differ")
	}
}

func TestRenderMarkdownHonestyLines(t *testing.T) {
	fm, body := loadFixture(t)
	doc, _ := Build(Input{Spec: fm, Body: body, Stamp: Stamp{Ref: "spec/lockbox", Commit: strings.Repeat("e", 40)}, Kind: KindSpec})
	md := RenderMarkdown(doc)
	for _, want := range []string{
		"Coverage: not computed for this render.",
		"Claims: not computed for this render.",
		"Evidence was not supplied for this render.",
		"Status | not resolved for this render",
		"Derived from the spec's objects; not authority.",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("missing honesty line %q in:\n%s", want, md)
		}
	}
	if strings.Contains(md, "Proposed, not") {
		t.Errorf("an accepted render must not carry the proposed header")
	}
}
```

Also add the matrix fixture helper in the same test file:

```go
func matrixFixture() matrixprojection.Record {
	return matrixprojection.Record{Feature: &matrixprojection.FeatureBody{ACs: []matrixprojection.FeatureAC{
		{ID: "ac-1", Status: "eligible", Summary: "one implementing story, not yet closed", ImplementingStories: []string{"spec/key-holder"}},
		{ID: "ac-2", Status: "violated", Summary: "no implementing story"},
	}}}
}
```


- [ ] **Step 2: Copy the fixture spec to testdata**

Create `internal/specdoc/testdata/lockbox.md` with exactly the bytes of the string literal in Task 4's `fixtureSpec` (frontmatter through the final `## oq-2` line, trailing newline).

- [ ] **Step 3: Run to verify it fails**

Run: `go test ./internal/specdoc/ -run TestRenderMarkdown`
Expected: FAIL with `undefined: RenderMarkdown`.

- [ ] **Step 4: Write the renderer**

```go
// internal/specdoc/markdown.go
package specdoc

import (
	"fmt"
	"strings"
)

// RenderMarkdown writes the canonical text form. The layout is the
// cross-consumer contract (spec/spec-documents ac-6): CLI, board, docs
// site, and MCP compare on these bytes.
func RenderMarkdown(doc Document) string {
	var b strings.Builder
	w := func(format string, args ...any) { fmt.Fprintf(&b, format, args...) }

	w("# %s\n\n", doc.Title)
	if doc.Stamp.Proposed {
		// vocab:identity — "proposed"/"accepted" here name the merge event of the spec MR, not a lifecycle state label
		w("> Proposed, not accepted: rendered from design-branch bytes at `%s`.\n\n", doc.Stamp.Commit)
	}

	for _, s := range doc.Sections {
		switch s {
		case SectionIdentity:
			w("## Identity\n\n| Field | Value |\n|---|---|\n")
			for _, kv := range doc.Identity {
				w("| %s | %s |\n", kv.Label, kv.Value)
			}
			w("\n")
		case SectionProblem:
			w("## Problem\n\n")
			if doc.ProblemKnown {
				w("%s\n\n", doc.ProblemText)
			} else {
				w("No problem statement is declared on this spec.\n\n")
			}
		case SectionOutcome:
			w("## Outcome\n\n")
			if doc.OutcomeKnown {
				w("%s\n\n", doc.OutcomeText)
			} else {
				w("No outcome statement is declared on this spec.\n\n")
			}
		case SectionDecisions:
			w("## Decisions\n\n")
			if len(doc.Decisions) == 0 {
				w("No decisions are declared.\n\n")
			}
			for _, d := range doc.Decisions {
				w("### %s\n\n%s\n\n", d.ID, d.Text)
				if d.Detail != "" {
					w("%s\n\n", d.Detail)
				}
			}
		case SectionConstraints:
			w("## Constraints\n\n")
			if len(doc.Constraints) == 0 {
				w("No constraints are declared.\n\n")
			}
			for _, c := range doc.Constraints {
				w("- **%s** %s\n", c.ID, c.Text)
				if c.Detail != "" {
					w("  %s\n", strings.ReplaceAll(c.Detail, "\n", "\n  "))
				}
			}
			if len(doc.Constraints) > 0 {
				w("\n")
			}
		case SectionCriteria:
			w("## Acceptance criteria\n\n")
			for i, c := range doc.Criteria {
				w("%d. **%s** %s <a id=\"%s\"></a>\n", i+1, c.ID, c.Text, c.ID)
				w("   Evidence: %s.\n", joinOr(c.Evidence, "none declared"))
				w("   Coverage: %s\n", coverageLine(c, doc.Words))
				if c.Detail != "" {
					w("\n   %s\n", strings.ReplaceAll(c.Detail, "\n", "\n   "))
				}
				w("\n")
			}
		case SectionQuestions:
			w("## Open questions\n\n")
			if len(doc.Questions) == 0 {
				w("No open questions are declared.\n\n")
			}
			for _, q := range doc.Questions {
				w("- **%s** %s — %s\n", q.ID, q.Text, claimsLine(q, doc.Words))
			}
			if len(doc.Questions) > 0 {
				w("\n")
			}
		case SectionPlan:
			w("## Plan\n\n")
			if len(doc.Plan) == 0 {
				w("No planned %s or %s are declared.\n\n", pluralWord(doc.Words.Story), pluralWord(doc.Words.Spike))
			}
			for i, p := range doc.Plan {
				if p.Spike {
					w("%d. Research %s `%s` answers %s.\n", i+1, doc.Words.Spike, p.Slug, joinOr(p.Resolves, "no declared question"))
				} else {
					w("%d. Planned %s `%s` covers %s.\n", i+1, doc.Words.Story, p.Slug, joinOr(p.Criteria, "no declared criterion"))
				}
			}
			if len(doc.Plan) > 0 {
				w("\n")
			}
		case SectionEvidence:
			w("## Evidence\n\n")
			if !doc.EvidenceKnown {
				w("Evidence was not supplied for this render.\n\n")
				break
			}
			w("Source: %s.\n\n| Criterion | State | Summary | Detail |\n|---|---|---|---|\n", doc.EvidenceSource)
			for _, e := range doc.Evidence {
				w("| %s | %s | %s | %s |\n", e.ID, orDash(e.Status), orDash(e.Summary), evidenceDetail(e, doc.Words))
			}
			w("\n")
		}
	}

	w("---\n\n")
	// vocab:identity — the stamp names the artifact's objects, not a lifecycle state label
	w("Derived from the spec's objects; not authority. Ref `%s` · commit `%s` · kind `%s` · engine `%s`\n", doc.Stamp.Ref, doc.Stamp.Commit, doc.Kind, doc.Stamp.Engine)
	return b.String()
}

func coverageLine(c Criterion, words Words) string {
	if !c.CoverageKnown {
		return "not computed for this render."
	}
	if len(c.Coverage) == 0 {
		return "not yet planned."
	}
	return fmt.Sprintf("planned in %s %s.", pluralIf(words.Story, len(c.Coverage)), joinCode(c.Coverage))
}

func claimsLine(q Question, words Words) string {
	if !q.ClaimsKnown {
		return "Claims: not computed for this render."
	}
	if len(q.Claims) == 0 {
		return "unclaimed; blocks acceptance until a research " + words.Spike + " claims it or a decision answers it."
	}
	return fmt.Sprintf("claimed by research %s %s; answered after acceptance.", pluralIf(words.Spike, len(q.Claims)), joinCode(q.Claims))
}

func evidenceDetail(e EvidenceRow, words Words) string {
	if len(e.Stories) > 0 {
		return fmt.Sprintf("implementing %s: %s", pluralIf(words.Story, len(e.Stories)), joinCode(e.Stories))
	}
	if len(e.Kinds) > 0 {
		parts := make([]string, 0, len(e.Kinds))
		for _, k := range e.Kinds {
			state := "unsatisfied"
			if k.Satisfied {
				state = "satisfied"
			}
			parts = append(parts, k.Kind+" "+state)
		}
		return strings.Join(parts, ", ")
	}
	return "—"
}

func joinOr(items []string, empty string) string {
	if len(items) == 0 {
		return empty
	}
	return strings.Join(items, ", ")
}

func joinCode(items []string) string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, "`"+it+"`")
	}
	return strings.Join(out, ", ")
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

// pluralWord appends "s" unless the word already ends in "s"; the display
// chain's own plural (DisplayClassPlural) is model-aware but needs the
// model, which Build has already consumed into Words, so the renderer
// applies the same rule the chain applies for plain words.
func pluralWord(word string) string {
	if strings.HasSuffix(word, "s") {
		return word
	}
	return word + "s"
}

func pluralIf(word string, n int) string {
	if n == 1 {
		return word
	}
	return pluralWord(word)
}
```

- [ ] **Step 5: Generate the goldens, then run the tests**

Run: `go test ./internal/specdoc/ -run TestRenderMarkdownGoldens -update`
Then open each `testdata/golden-*.md` and read it top to bottom as a reviewer would. Check by eye: the Identity table has Ref, Class, Status, Commit, Supersedes, Revision rows; criteria are numbered with Evidence and Coverage lines; `ac-2` reads "not yet planned"; `oq-2` reads "unclaimed; blocks acceptance…"; the plan lists `key-holder` then `audit-probe`; the Evidence table (spec and tasks kinds) has two rows; the footer carries ref, commit, kind, engine. Fix the renderer, not the golden, if anything reads wrong; regenerate.

Run: `go test -race ./internal/specdoc/`
Expected: PASS.

- [ ] **Step 6: Run the vocabulary witness**

Run: `go test -count=1 ./internal/specalign/ -run TestVocabProseWitness`
Expected: PASS. If it reports a bare word in `markdown.go`, the offending literal is one that names a class or state without going through `doc.Words` or without the `// vocab:identity` marker; route it or mark it, never weaken the witness.

- [ ] **Step 7: Commit**

```bash
git add internal/specdoc/markdown.go internal/specdoc/markdown_test.go internal/specdoc/testdata/
git commit -m "Add specdoc Markdown renderer with golden documents"
```

---

### Task 6: HTML renderer

**Files:**
- Create: `internal/specdoc/html.go`
- Create: `internal/specdoc/testdata/golden-spec.html` (generated)
- Test: `internal/specdoc/html_test.go`

**Interfaces:**
- Consumes: `render.RenderMarkdown(body string) (string, error)` (`internal/render/markdown.go:65`).
- Produces: `func RenderHTML(doc Document) (string, error)` — the HTML fragment for the canonical Markdown; no `<html>`/`<body>` wrapper (consumers add their shell).

- [ ] **Step 1: Write the failing test**

```go
// internal/specdoc/html_test.go
package specdoc

import (
	"strings"
	"testing"
)

func TestRenderHTMLGolden(t *testing.T) {
	fm, body := loadFixture(t)
	doc, err := Build(Input{Spec: fm, Body: body, Status: "accepted-pending-build", Stamp: Stamp{Ref: "spec/lockbox", Commit: strings.Repeat("0", 39) + "1"}, Facts: WithMatrix(FactsFromSpec(fm), matrixFixture(), "matrix at 00000000"), Kind: KindSpec})
	if err != nil {
		t.Fatal(err)
	}
	html, err := RenderHTML(doc)
	if err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "golden-spec.html", html)
	for _, want := range []string{"<h1", "<h2", "<table", "not authority"} {
		if !strings.Contains(html, want) {
			t.Errorf("html missing %q", want)
		}
	}
	if strings.Contains(html, "<html") || strings.Contains(html, "<body") {
		t.Errorf("RenderHTML must return a fragment, not a page")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/specdoc/ -run TestRenderHTMLGolden`
Expected: FAIL with `undefined: RenderHTML`.

- [ ] **Step 3: Write the implementation**

```go
// internal/specdoc/html.go
package specdoc

import "github.com/jyang234/verdi/internal/render"

// RenderHTML renders the canonical Markdown through the store's own
// Markdown engine (internal/render, goldmark) and returns the fragment.
// Consumers wrap it in their shell; the bytes inside are the same reading
// of the same objects every consumer shows (spec/spec-documents ac-6).
func RenderHTML(doc Document) (string, error) {
	return render.RenderMarkdown(RenderMarkdown(doc))
}
```

- [ ] **Step 4: Generate the golden and run**

Run: `go test ./internal/specdoc/ -run TestRenderHTMLGolden -update && go test -race ./internal/specdoc/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/specdoc/html.go internal/specdoc/html_test.go internal/specdoc/testdata/golden-spec.html
git commit -m "Add specdoc HTML renderer over the shared Markdown engine"
```

---

### Task 7: `verdi spec doc` verb

**Files:**
- Create: `cmd/verdi/specdoc.go`
- Modify: `cmd/verdi/specstate.go:41-48` (`runSpecVerb`)
- Modify: `cmd/verdi/help.go:142` (`verbUsage["spec"]`)
- Test: `cmd/verdi/specdoc_test.go`

**Interfaces:**
- Consumes: `specdoc.Build`, `specdoc.RenderMarkdown`, `specdoc.RenderHTML`, `specdoc.FactsFromSpec`, `specdoc.WithMatrix`, `specdoc.ParseKind`; `store.FindRoot(".")`, `store.Open(root) (*store.Config, error)` with `cfg.Model`; `store.ActiveSpecRelPath(name)`, `store.SpecRelPath(store.ZoneArchive, name)`; `readSpecBytesEitherZone(root, name) (relPath string, content []byte, err error)` (`cmd/verdi/specstate.go:109`); `specstate.ResolveDefaultBranch(ctx, root) (specstate.Branch, bool)`; `specstate.NewProjector().Resolve(ctx, root, specstate.Candidate{Path, Content}) (specstate.Result, error)` and `Result.ArtifactStatus()`; `gitx.RevParse(ctx, root, rev)`, `gitx.Show(ctx, root, commit, path)`; `matrixprojection.Project(ctx, root, ref, preview bool, mdl) (Projection, error)` with `.Record`; `artifact.ParseRef`, `artifact.SplitFrontmatter`, `artifact.DecodeSpec`.
- Produces: `func cmdSpecDoc(args []string, stdout, stderr io.Writer) int`.

- [ ] **Step 1: Write the failing built-binary test**

```go
// cmd/verdi/specdoc_test.go
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

const specDocFixture = `---
id: spec/lockbox
kind: spec
title: "Lockbox"
owners: [platform-team]
class: feature
problem: { text: "Keys are shared.", anchor: problem }
outcome: { text: "Each key has one holder.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "A key opens one box.", evidence: [behavioral, attestation], anchor: ac-1 }
constraints:
  - { id: co-1, text: "No network.", anchor: co-1 }
decisions:
  - { id: dc-1, text: "One holder per key.", anchor: dc-1 }
open_questions:
  - { id: oq-1, text: "Who audits holders?", anchor: oq-1 }
stubs:
  - { slug: key-holder, acceptance_criteria: [ac-1] }
  - { slug: audit-probe, spike: true, resolves: [oq-1] }
---
# Lockbox

## Problem

Keys are shared today.

## Outcome

One holder.

## ac-1

Proven by opening.

## co-1

## dc-1

Because two holders means no holder.

## oq-1
`

func buildSpecDocRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{{
		Message: "adopt store with one accepted spec",
		Files: map[string]string{
			".verdi/verdi.yaml":                   supersedeManifestYAML, // the package's shared minimal manifest (designsupersede_test.go:17)
			".verdi/specs/active/lockbox/spec.md": specDocFixture,
		},
	}})
}

func TestSpecDoc_RendersAcceptedSpecToStdout(t *testing.T) {
	repo := buildSpecDocRepo(t)
	bin := buildVerdiBinary(t)
	stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, []string{"CI_DEFAULT_BRANCH=main"}, "spec", "doc", "spec/lockbox")
	if code != 0 {
		t.Fatalf("exit %d, stderr:\n%s", code, stderr)
	}
	for _, want := range []string{"# Lockbox", "## Acceptance criteria", "**ac-1** A key opens one box.", "planned in", "`key-holder`", "commit `" + repo.Head + "`", "not authority"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "Proposed, not") {
		t.Errorf("accepted render must not carry the proposed header")
	}
}

func TestSpecDoc_KindsFormatsAndOutputFile(t *testing.T) {
	repo := buildSpecDocRepo(t)
	bin := buildVerdiBinary(t)
	out := filepath.Join(t.TempDir(), "tasks.html")
	stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, []string{"CI_DEFAULT_BRANCH=main"}, "spec", "doc", "spec/lockbox", "--kind", "tasks", "--format", "html", "-o", out)
	if code != 0 {
		t.Fatalf("exit %d, stderr:\n%s", code, stderr)
	}
	if stdout != "" {
		t.Errorf("-o must leave stdout empty, got %q", stdout)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	html := string(data)
	if !strings.Contains(html, "<h2") || strings.Contains(html, "Acceptance criteria") || !strings.Contains(html, "Plan") {
		t.Errorf("tasks html wrong shape:\n%s", html)
	}
	stdout, _, code = runVerdiBinary(t, bin, repo.Dir, []string{"CI_DEFAULT_BRANCH=main"}, "spec", "doc", "spec/lockbox", "--kind=plan")
	if code != 0 || !strings.Contains(stdout, "## Decisions") || strings.Contains(stdout, "## Problem") {
		t.Errorf("plan kind: exit %d, stdout:\n%s", code, stdout)
	}
}

func TestSpecDoc_ProposedAndAt(t *testing.T) {
	repo := buildSpecDocRepo(t)
	bin := buildVerdiBinary(t)
	// A design branch with an edited spec.
	branch := "design/lockbox-edit"
	run := func(args ...string) (string, string, int) {
		return runVerdiBinary(t, bin, repo.Dir, []string{"CI_DEFAULT_BRANCH=main"}, args...)
	}
	runGitCmd(t, repo.Dir, "checkout", "-q", "-b", branch)
	edited := strings.Replace(specDocFixture, "Keys are shared.", "Keys are shared widely.", 1)
	if err := os.WriteFile(filepath.Join(repo.Dir, ".verdi/specs/active/lockbox/spec.md"), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCmd(t, repo.Dir, "commit", "-qam", "edit problem")
	head := strings.TrimSpace(gitOutput(t, repo.Dir, "rev-parse", "HEAD"))

	stdout, stderr, code := run("spec", "doc", "spec/lockbox", "--proposed")
	if code != 0 {
		t.Fatalf("--proposed exit %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "Proposed, not accepted") || !strings.Contains(stdout, "Keys are shared widely.") || !strings.Contains(stdout, "commit `"+head+"`") {
		t.Errorf("--proposed render wrong:\n%s", stdout)
	}

	stdout, stderr, code = run("spec", "doc", "spec/lockbox")
	if code != 0 {
		t.Fatalf("default exit %d: %s", code, stderr)
	}
	if strings.Contains(stdout, "widely") || !strings.Contains(stdout, "commit `"+repo.Head+"`") {
		t.Errorf("default render must read main's bytes, not the branch's:\n%s", stdout)
	}

	stdout, stderr, code = run("spec", "doc", "spec/lockbox", "--at", head)
	if code != 0 || !strings.Contains(stdout, "widely") || !strings.Contains(stdout, "commit `"+head+"`") {
		t.Errorf("--at render wrong (exit %d, %s):\n%s", code, stderr, stdout)
	}
}

func TestSpecDoc_Refusals(t *testing.T) {
	repo := buildSpecDocRepo(t)
	bin := buildVerdiBinary(t)
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"missing ref", []string{"spec", "doc"}, "usage: verdi spec doc"},
		{"not a spec ref", []string{"spec", "doc", "adr/x"}, "is not a valid spec/<name> ref"},
		{"fragment ref", []string{"spec", "doc", "spec/lockbox#ac-1"}, "is not a valid spec/<name> ref"},
		{"unknown spec", []string{"spec", "doc", "spec/nope"}, "spec/nope"},
		{"bad kind", []string{"spec", "doc", "spec/lockbox", "--kind", "chapter"}, "unknown document kind"},
		{"bad format", []string{"spec", "doc", "spec/lockbox", "--format", "pdf"}, "--format must be md or html"},
		{"bad commit", []string{"spec", "doc", "spec/lockbox", "--at", "deadbeef"}, "deadbeef"},
		{"at and proposed", []string{"spec", "doc", "spec/lockbox", "--at", repo.Head, "--proposed"}, "--at and --proposed cannot be combined"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, []string{"CI_DEFAULT_BRANCH=main"}, c.args...)
			if code != 2 {
				t.Fatalf("exit %d, want 2; stdout %q stderr %q", code, stdout, stderr)
			}
			if !strings.Contains(stderr, c.want) {
				t.Errorf("stderr %q does not name %q", stderr, c.want)
			}
		})
	}
}

func TestSpecDoc_NoStoreExitsOperational(t *testing.T) {
	bin := buildVerdiBinary(t)
	_, stderr, code := runVerdiBinary(t, bin, t.TempDir(), nil, "spec", "doc", "spec/lockbox")
	if code != 2 || stderr == "" {
		t.Fatalf("exit %d stderr %q, want 2 with a message", code, stderr)
	}
}
```

`runGitCmd(t, dir, args...)` (cmd/verdi/gc_test.go:53) and `gitOutput(t, dir, args...) string` (cmd/verdi/close_test.go:2054) already exist in this package; use them, do not add new git helpers. If `runGitCmd` refuses `commit -qam` because the fixture identity env is unset, set `GIT_AUTHOR_NAME/EMAIL` and `GIT_COMMITTER_NAME/EMAIL` the way `internal/fixturegit` does (`Verdi Fixture <fixture@verdi.invalid>`) on that one call.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./cmd/verdi/ -run TestSpecDoc`
Expected: every case exits 2 with `usage: verdi spec state <spec-ref>` because the `doc` subverb is not dispatched; `TestSpecDoc_RendersAcceptedSpecToStdout` fails on exit code.

- [ ] **Step 3: Write the verb**

```go
// cmd/verdi/specdoc.go
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/matrixprojection"
	"github.com/jyang234/verdi/internal/specdoc"
	"github.com/jyang234/verdi/internal/specstate"
	"github.com/jyang234/verdi/internal/store"
)

// vocab:identity — CLI usage grammar (identity arg placeholders)
const specDocUsage = "usage: verdi spec doc <spec-ref> [--kind spec|plan|tasks] [--format md|html] [--at <commit>] [--proposed] [-o <path>]"

// cmdSpecDoc renders one spec as a document (spec/spec-documents ac-2).
// Exit 0 on a render, 2 on an unresolvable ref, commit, kind, format, or
// store. Never 1: a document is a projection, not a verdict.
func cmdSpecDoc(args []string, stdout, stderr io.Writer) int {
	ref := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		ref, args = args[0], args[1:]
	}
	fs := flag.NewFlagSet("spec doc", flag.ContinueOnError)
	fs.SetOutput(stderr)
	kindFlag := fs.String("kind", "spec", "document kind: spec, plan, or tasks")
	formatFlag := fs.String("format", "md", "output format: md or html")
	atFlag := fs.String("at", "", "render the spec bytes at this commit")
	proposedFlag := fs.Bool("proposed", false, "render the working tree's bytes (a design-branch draft)")
	outFlag := fs.String("o", "", "write the document to this path instead of stdout")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if ref == "" && fs.NArg() > 0 {
		ref = fs.Arg(0)
	}
	if ref == "" {
		fmt.Fprintln(stderr, specDocUsage)
		return 2
	}
	parsed, err := artifact.ParseRef(ref)
	if err != nil || parsed.Kind != artifact.KindSpec || parsed.Fragment() || parsed.Pinned() {
		fmt.Fprintf(stderr, "spec doc: %q is not a valid spec/<name> ref (use --at for a commit)\n", ref)
		return 2
	}
	kind, err := specdoc.ParseKind(*kindFlag)
	if err != nil {
		fmt.Fprintln(stderr, "spec doc:", err)
		return 2
	}
	if *formatFlag != "md" && *formatFlag != "html" {
		fmt.Fprintf(stderr, "spec doc: --format must be md or html, got %q\n", *formatFlag)
		return 2
	}
	if *atFlag != "" && *proposedFlag {
		fmt.Fprintln(stderr, "spec doc: --at and --proposed cannot be combined")
		return 2
	}

	root, err := store.FindRoot(".")
	if err != nil {
		fmt.Fprintln(stderr, "spec doc:", err)
		return 2
	}
	cfg, err := store.Open(root)
	if err != nil {
		fmt.Fprintln(stderr, "spec doc:", err)
		return 2
	}
	ctx := context.Background()

	src, err := loadSpecDocSource(ctx, root, parsed.Name, *atFlag, *proposedFlag)
	if err != nil {
		fmt.Fprintln(stderr, "spec doc:", err)
		return 2
	}
	fm, err := artifact.DecodeSpec(src.content)
	if err != nil {
		fmt.Fprintf(stderr, "spec doc: %s at %s: %v\n", parsed.String(), src.commit, err)
		return 2
	}
	_, body, err := artifact.SplitFrontmatter(src.content)
	if err != nil {
		fmt.Fprintln(stderr, "spec doc:", err)
		return 2
	}

	status := ""
	if res, rerr := specstate.NewProjector().Resolve(ctx, root, specstate.Candidate{Path: src.relPath, Content: src.content}); rerr == nil {
		status = string(res.ArtifactStatus())
	} else {
		fmt.Fprintf(stderr, "spec doc: status not resolved: %v\n", rerr)
	}

	facts := specdoc.FactsFromSpec(fm)
	if proj, perr := matrixprojection.Project(ctx, root, parsed.String(), *proposedFlag, cfg.Model); perr == nil {
		facts = specdoc.WithMatrix(facts, proj.Record, "matrix at "+src.commit[:12])
	} else {
		fmt.Fprintf(stderr, "spec doc: evidence not computed: %v\n", perr)
	}

	doc, err := specdoc.Build(specdoc.Input{
		Spec:   fm,
		Body:   body,
		Status: status,
		Stamp:  specdoc.Stamp{Ref: parsed.String(), Commit: src.commit, Proposed: *proposedFlag},
		Facts:  facts,
		Model:  cfg.Model,
		Kind:   kind,
	})
	if err != nil {
		fmt.Fprintln(stderr, "spec doc:", err)
		return 2
	}

	var rendered string
	if *formatFlag == "html" {
		rendered, err = specdoc.RenderHTML(doc)
		if err != nil {
			fmt.Fprintln(stderr, "spec doc:", err)
			return 2
		}
	} else {
		rendered = specdoc.RenderMarkdown(doc)
	}
	if *outFlag != "" {
		if err := os.WriteFile(*outFlag, []byte(rendered), 0o644); err != nil {
			fmt.Fprintln(stderr, "spec doc:", err)
			return 2
		}
		return 0
	}
	if _, err := io.WriteString(stdout, rendered); err != nil {
		fmt.Fprintln(stderr, "spec doc:", err)
		return 2
	}
	return 0
}

// specDocSource is one spec's bytes, where they came from, and the full
// commit the stamp names.
type specDocSource struct {
	relPath string
	content []byte
	commit  string
}

// loadSpecDocSource picks the bytes: the default branch's (the accepted
// reading), a pinned commit's (--at), or the working tree's (--proposed,
// stamped with HEAD). Active zone first, archive second, in every mode.
func loadSpecDocSource(ctx context.Context, root, name, at string, proposed bool) (specDocSource, error) {
	if proposed {
		relPath, content, err := readSpecBytesEitherZone(root, name)
		if err != nil {
			return specDocSource{}, err
		}
		head, err := gitx.RevParse(ctx, root, "HEAD")
		if err != nil {
			return specDocSource{}, fmt.Errorf("resolving HEAD: %w", err)
		}
		return specDocSource{relPath: relPath, content: content, commit: head}, nil
	}
	rev := at
	if rev == "" {
		branch, ok := specstate.ResolveDefaultBranch(ctx, root)
		if !ok {
			return specDocSource{}, fmt.Errorf("the default branch could not be resolved; pass --at <commit> or --proposed")
		}
		rev = branch.Ref
	}
	commit, err := gitx.RevParse(ctx, root, rev)
	if err != nil {
		return specDocSource{}, fmt.Errorf("resolving %q: %w", rev, err)
	}
	for _, relPath := range []string{store.ActiveSpecRelPath(name), store.SpecRelPath(store.ZoneArchive, name)} {
		content, err := gitx.Show(ctx, root, commit, relPath)
		if err == nil {
			return specDocSource{relPath: relPath, content: content, commit: commit}, nil
		}
	}
	return specDocSource{}, fmt.Errorf("spec/%s not found at %s in either zone", name, commit)
}
```

- [ ] **Step 4: Wire the subverb and the usage line**

In `cmd/verdi/specstate.go` replace `runSpecVerb`:

```go
func runSpecVerb(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, specVerbUsage)
		return 2
	}
	switch args[0] {
	case "state":
		return cmdSpecState(args[1:], stdout, stderr)
	case "doc":
		return cmdSpecDoc(args[1:], stdout, stderr)
	}
	fmt.Fprintln(stderr, specVerbUsage)
	return 2
}

// vocab:identity — CLI usage grammar (identity arg placeholders)
const specVerbUsage = "usage: verdi spec state <spec-ref>\n       " + specDocUsage
```

and in `cmd/verdi/help.go` change the `"spec"` entry of `verbUsage` to:

```go
"spec": "usage: verdi spec state <spec-ref>\n       verdi spec doc <spec-ref> [--kind spec|plan|tasks] [--format md|html] [--at <commit>] [--proposed] [-o <path>]",
```

(keep the exact text identical to `specDocUsage`; a help test may pin it.)

- [ ] **Step 5: Run the verb tests**

Run: `go test -race -count=1 ./cmd/verdi/ -run 'TestSpecDoc|Help|Usage|SpecState'`
Expected: PASS. If `TestSpecDoc_ProposedAndAt` fails on `matrixprojection.Project` for a design branch, that call is best-effort and only writes a stderr disclosure; the assertion is on stdout. If the status disclosure line appears where a test expects an empty stderr, note that no test here asserts empty stderr on success.

- [ ] **Step 6: Run the whole cmd/verdi package and the spec-align gate**

Run: `go test -race -count=1 ./cmd/verdi/` (about 8 minutes) and `go test -count=1 ./internal/specalign/` (about 2.5 minutes).
Expected: both PASS. `internal/specalign/verbs_test.go` probes the bare `spec` verb and expects exit 2 on usage; that still holds. If any spec-align test enumerates the `verbUsage["spec"]` text or the CLI verb inventory and fails, update that inventory in the same commit and say so in the report; never edit a witness's assertion to pass.

- [ ] **Step 7: Commit**

```bash
git add cmd/verdi/specdoc.go cmd/verdi/specdoc_test.go cmd/verdi/specstate.go cmd/verdi/help.go
git commit -m "Add verdi spec doc: render a spec as a document from the CLI"
```

---

### Task 8: Wave gate

**Files:** none new.

- [ ] **Step 1: Static gates**

Run: `go build ./... && gofmt -l . && go vet ./internal/specdoc/ ./cmd/verdi/ && golangci-lint run ./internal/specdoc/... ./cmd/...`
Expected: build ok, `gofmt -l` prints nothing, vet clean, `0 issues`.

- [ ] **Step 2: Full gate**

Run from the worktree root: `VERDI_E2E_PORT_BASE=4390 make verify`
Expected: `verify OK`, exit 0. Then `git status --porcelain` is empty and `find e2e -name '*.png' -o -name '*.webm' -o -name 'trace.zip' | grep -v node_modules` prints nothing.

- [ ] **Step 3: Lane report**

Write `docs/superpowers/reports/2026-09-17-spec-documents-wave-1.md` in the evidence format (Status, Risk tier 3 for ac-1, Base..Head, Commits, Files changed, Contract implemented, Explicit exclusions, RED command and observed failure, GREEN commands and results, Reviewer verdict blank, Fix range blank, Residual risks, Integration prerequisites). Commit it:

```bash
git add docs/superpowers/reports/2026-09-17-spec-documents-wave-1.md
git commit -m "Report spec-documents wave 1: document core and spec doc verb"
```

---

## Self-review

**Spec coverage (ac-1, ac-2).** ac-1 core: model from objects, body, computed facts, readiness "when present" — Task 4 accepts facts; readiness is an input consumers supply in Wave 2 (the board and readiness page own the snapshot), and this wave's CLI leaves it unsupplied honestly through the Facts nil-map rule and the fixed section wording. Three kinds: Task 1 and Task 4. Byte-deterministic: Task 5 determinism test and goldens. Stamps with ref, commit, engine, not-authority sentence: Task 1 stamp, Task 5 footer. Proposed header: Task 5 golden-spec-proposed. Sections never omitted: Task 5 honesty-lines test. ac-2 verb: Task 7 covers every flag, default-branch bytes, `--at`, `--proposed`, `-o`, exit 0/2 and no exit 1.

**Placeholders.** None: every step has its code; the goldens are generated by a named command and reviewed by eye in Step 5 of Task 5.

**Type consistency.** `Facts{Coverage, Claims, Evidence, EvidenceSource}` (Task 2) is what `Build` reads (Task 4). `KindEvidence{Kind, Satisfied}` (Task 2) is what `EvidenceRow.Kinds` (Task 4) and `evidenceDetail` (Task 5) consume. `Stamp{Ref, Commit, Engine, Proposed}` (Task 1) is filled by the verb (Task 7) and stamped by `Build`. `readSpecBytesEitherZone(root, name) (relPath, content, err)` matches `cmd/verdi/specstate.go:109`. `specstate.Result.ArtifactStatus()` matches `internal/specstate/state.go:117`. `matrixprojection.Project(ctx, root, ref, preview, mdl)` matches `internal/matrixprojection/project.go:56`.

**Known limits carried to Wave 2.** Readiness facts and the board's live snapshot; the parity test (ac-6) needs the board tab and docs-site consumers to exist first; `--proposed` on the CLI renders the working tree, which on the sealed default branch equals the accepted bytes, so the header logic is keyed on the flag, not on specstate's relation (a design-branch render whose bytes happen to equal main still says proposed, which is honest about provenance).

---

## Amendments during execution

This plan is the argument the wave was built from; the code is the implementation. Review rounds amended it in these places, recorded with rulings in the wave report (docs/superpowers/reports/2026-09-18-spec-documents-wave-1.md): the Markdown layout (nested Evidence/Coverage bullets, decision headings with visible id and anchor, plan lines carrying criterion text, "covered by", plural words from the display chain, empty-state lines for every section, a second renamed-vocabulary fixture); `WithMatrix` takes the resolved HEAD and owns the "matrix over the working tree at <12 hex>" wording; the verb's flag parser peels one positional and refuses a second so flags may appear in any position; `-o` refuses paths inside the store; `bodySections` is fence-aware. Where a code block here disagrees with HEAD, HEAD is right.
