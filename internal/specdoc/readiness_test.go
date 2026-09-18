package specdoc

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/readinesspilot"
)

func readinessFixture() readinesspilot.Snapshot {
	return readinesspilot.Snapshot{
		TargetRef: "spec/lockbox", TargetTitle: "Lockbox", TargetClass: "feature", Branch: "main",
		Head: strings.Repeat("a", 40), RequestDigest: "sha256:" + strings.Repeat("0", 64),
		Areas: []readinesspilot.Area{
			{ID: readinesspilot.AreaShape, Label: "Define the work", State: readinesspilot.StateUnproven},
			{ID: readinesspilot.AreaSuccess, Label: "Define success", State: readinesspilot.StateProven},
		},
		CurrentFocus: readinesspilot.AreaShape,
		Attention: []readinesspilot.Concern{
			{ID: "shape/question/oq-2", Area: readinesspilot.AreaShape, State: readinesspilot.StateUnproven, Blocking: true, Timing: readinesspilot.TimingCurrent, Summary: "Declared open question remains unresolved", Witnesses: []string{"oq-2"}},
			{ID: "shape/question/oq-1", Area: readinesspilot.AreaShape, State: readinesspilot.StateUnproven, Blocking: false, Timing: readinesspilot.TimingEventual, Summary: "Declared open question is claimed by spike stubs and remains unresolved", Witnesses: []string{"oq-1", "audit-probe"}},
		},
		StaleNotice: "Startup snapshot; restart verdi serve after an edit.",
	}
}

func TestWithReadinessGatesOnTargetRef(t *testing.T) {
	snap := readinessFixture()
	got := WithReadiness(Facts{}, snap, "spec/lockbox")
	if got.Readiness == nil {
		t.Fatal("matching ref must supply readiness")
	}
	if got.Readiness.TargetRef != "spec/lockbox" || got.Readiness.CurrentFocus != "shape-proposal" || len(got.Readiness.Areas) != 2 || len(got.Readiness.Attention) != 2 {
		t.Fatalf("readiness facts = %+v", got.Readiness)
	}
	if got.Readiness.Attention[0].ID != "shape/question/oq-2" || !got.Readiness.Attention[0].Blocking || got.Readiness.Attention[1].Timing != "eventual" {
		t.Fatalf("attention order/fields = %+v", got.Readiness.Attention)
	}
	other := WithReadiness(Facts{}, snap, "spec/other")
	if other.Readiness != nil {
		t.Fatal("a snapshot for another spec must not be supplied")
	}
}

func TestWithReadinessDoesNotTouchOtherFacts(t *testing.T) {
	base := Facts{Coverage: map[string][]string{"ac-1": {"x"}}}
	got := WithReadiness(base, readinessFixture(), "spec/lockbox")
	if len(got.Coverage) != 1 || got.Coverage["ac-1"][0] != "x" {
		t.Fatal("WithReadiness must copy the other facts unchanged")
	}
}

func TestKindSectionsIncludeReadiness(t *testing.T) {
	spec := KindSpec.Sections()
	if spec[len(spec)-1] != SectionReadiness || spec[len(spec)-2] != SectionEvidence {
		t.Fatalf("spec sections = %v", spec)
	}
	tasks := KindTasks.Sections()
	if tasks[len(tasks)-1] != SectionReadiness {
		t.Fatalf("tasks sections = %v", tasks)
	}
	for _, s := range KindPlan.Sections() {
		if s == SectionReadiness {
			t.Fatal("plan kind must not carry readiness")
		}
	}
}

func TestBuildAndRenderReadiness(t *testing.T) {
	fm, body := loadFixture(t)
	commit := strings.Repeat("0", 39) + "1"
	with := WithReadiness(FactsFromSpec(fm), readinessFixture(), "spec/lockbox")
	doc, err := Build(Input{Spec: fm, Body: body, Status: "accepted-pending-build", Stamp: Stamp{Ref: "spec/lockbox", Commit: commit}, Facts: with, Kind: KindSpec})
	if err != nil {
		t.Fatal(err)
	}
	if !doc.ReadinessKnown || doc.Readiness == nil {
		t.Fatal("Build must carry supplied readiness")
	}
	md := RenderMarkdown(doc)
	for _, want := range []string{
		"## Readiness",
		"Source: readiness snapshot for `spec/lockbox` at `aaaaaaaaaaaa`. Current focus: Define the work.",
		"| Define the work | unproven |",
		"| Define success | proven |",
		"1. Declared open question remains unresolved — Define the work; blocking; current; unproven; witnesses: oq-2",
		"2. Declared open question is claimed by spike stubs and remains unresolved — Define the work; advisory; eventual; unproven; witnesses: oq-1, audit-probe",
		"Startup snapshot; restart verdi serve after an edit.",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("missing %q in:\n%s", want, md)
		}
	}
	without, _ := Build(Input{Spec: fm, Body: body, Stamp: Stamp{Ref: "spec/lockbox", Commit: commit}, Facts: FactsFromSpec(fm), Kind: KindSpec})
	if !strings.Contains(RenderMarkdown(without), "## Readiness\n\nReadiness was not supplied for this render.") {
		t.Errorf("unsupplied readiness must be stated:\n%s", RenderMarkdown(without))
	}
}

func TestEngineDigestChangedForVersion2(t *testing.T) {
	if engineVersion != 2 {
		t.Fatalf("engineVersion = %d, want 2 (readiness section added)", engineVersion)
	}
}

// TestReadinessDoesNotAliasCallerSlices is the fix-round-1 F1 mutation
// witness: WithReadiness's per-concern Witnesses copy (readiness.go) and
// Build's Areas/Attention copy (build.go) must each own a backing array
// the caller cannot reach — proven by mutating the caller's own slice
// AFTER the call returns and asserting the produced facts/document did
// not move. Mutation witnesses (recorded verbatim in the fix-round
// report): replacing readiness.go's
// `Witnesses: append([]string(nil), c.Witnesses...)` with the bare
// `Witnesses: c.Witnesses` reds the first subtest; replacing either of
// build.go's `append([]ReadinessArea(nil), ...)` /
// `append([]ReadinessConcern(nil), ...)` with a direct field assignment
// (`rf.Areas = in.Facts.Readiness.Areas` / `rf.Attention =
// in.Facts.Readiness.Attention`) reds the third/fourth subtest
// respectively. The second subtest (WithReadiness's Areas copy) cannot be
// killed by an equivalent one-line mutant: ReadinessArea and
// readinesspilot.Area are distinct struct types, so the per-field
// construction in WithReadiness's Areas loop is the only assignment that
// type-checks — it is still asserted here, both to document the
// invariant and as a regression guard should a future refactor ever
// align the two types' fields.
func TestReadinessDoesNotAliasCallerSlices(t *testing.T) {
	fm, body := loadFixture(t)
	commit := strings.Repeat("0", 39) + "1"

	cases := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "WithReadiness copies Attention[].Witnesses",
			run: func(t *testing.T) {
				snap := readinessFixture()
				got := WithReadiness(Facts{}, snap, "spec/lockbox")
				snap.Attention[0].Witnesses[0] = "MUTATED"
				if got.Readiness.Attention[0].Witnesses[0] == "MUTATED" {
					t.Error("WithReadiness must copy Witnesses, not alias the snapshot's slice")
				}
			},
		},
		{
			name: "WithReadiness copies Areas",
			run: func(t *testing.T) {
				snap := readinessFixture()
				got := WithReadiness(Facts{}, snap, "spec/lockbox")
				snap.Areas[0].Label = "MUTATED"
				if got.Readiness.Areas[0].Label == "MUTATED" {
					t.Error("WithReadiness must copy Areas, not alias the snapshot's slice")
				}
			},
		},
		{
			name: "Build copies Facts.Readiness.Areas",
			run: func(t *testing.T) {
				facts := WithReadiness(FactsFromSpec(fm), readinessFixture(), "spec/lockbox")
				doc, err := Build(Input{Spec: fm, Body: body, Stamp: Stamp{Ref: "spec/lockbox", Commit: commit}, Facts: facts, Kind: KindSpec})
				if err != nil {
					t.Fatal(err)
				}
				facts.Readiness.Areas[0].Label = "MUTATED"
				if doc.Readiness.Areas[0].Label == "MUTATED" {
					t.Error("Build must copy Facts.Readiness.Areas, not alias the caller's slice")
				}
			},
		},
		{
			name: "Build copies Facts.Readiness.Attention",
			run: func(t *testing.T) {
				facts := WithReadiness(FactsFromSpec(fm), readinessFixture(), "spec/lockbox")
				doc, err := Build(Input{Spec: fm, Body: body, Stamp: Stamp{Ref: "spec/lockbox", Commit: commit}, Facts: facts, Kind: KindSpec})
				if err != nil {
					t.Fatal(err)
				}
				facts.Readiness.Attention[0].Summary = "MUTATED"
				if doc.Readiness.Attention[0].Summary == "MUTATED" {
					t.Error("Build must copy Facts.Readiness.Attention, not alias the caller's slice")
				}
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, c.run)
	}
}

// TestRenderReadinessEdgeCases is the fix-round-1 F2 proof: rendering
// branches TestBuildAndRenderReadiness's single populated fixture never
// reaches — the zero-attention arm, an empty stale notice, a witness-less
// concern (joinOr's empty arm), a Head under 12 hex characters
// (shortCommit's whole-string return), a CurrentFocus/Area id absent from
// Areas (areaLabel's id fallback) — plus, end to end, F4's escapeAttr
// call site: a concern id carrying an HTML-attribute-breaking character
// renders escaped inside `<a id="...">`. Documents are built directly
// (not through Build), matching this file's existing synthetic-Document
// convention for edge cases (e.g. TestRenderMarkdownEvidenceDetailKinds
// in markdown_test.go).
func TestRenderReadinessEdgeCases(t *testing.T) {
	base := func(rf *ReadinessFacts) Document {
		return Document{
			Kind:           KindSpec,
			Stamp:          Stamp{Ref: "spec/x", Commit: strings.Repeat("1", 40)},
			Sections:       []SectionID{SectionReadiness},
			Words:          Words{Story: "story", StoryPlural: "stories", Spike: "spike", SpikePlural: "spikes"},
			ReadinessKnown: true,
			Readiness:      rf,
		}
	}
	tests := []struct {
		name string
		doc  Document
		want []string
	}{
		{
			name: "zero attention and an empty stale notice",
			doc: base(&ReadinessFacts{
				TargetRef: "spec/x", Head: strings.Repeat("a", 40), CurrentFocus: "shape-proposal",
				Areas: []ReadinessArea{{ID: "shape-proposal", Label: "Define the work", State: "unproven"}},
			}),
			want: []string{
				"## Readiness\n\nSource: readiness snapshot for `spec/x` at `aaaaaaaaaaaa`. Current focus: Define the work.\n\n| Area | State |\n|---|---|\n| Define the work | unproven |\n\nNothing needs attention.\n\n---\n\n",
			},
		},
		{
			name: "a concern with no witnesses",
			doc: base(&ReadinessFacts{
				TargetRef: "spec/x", Head: strings.Repeat("b", 40), CurrentFocus: "shape-proposal",
				Areas:     []ReadinessArea{{ID: "shape-proposal", Label: "Define the work", State: "unproven"}},
				Attention: []ReadinessConcern{{ID: "shape/question/oq-9", Area: "shape-proposal", State: "unproven", Blocking: false, Timing: "current", Summary: "No witnesses here"}},
			}),
			want: []string{
				"1. No witnesses here — Define the work; advisory; current; unproven; witnesses: none <a id=\"shape/question/oq-9\"></a>\n",
			},
		},
		{
			name: "a Head shorter than 12 characters",
			doc: base(&ReadinessFacts{
				TargetRef: "spec/x", Head: "abc", CurrentFocus: "shape-proposal",
				Areas: []ReadinessArea{{ID: "shape-proposal", Label: "Define the work", State: "proven"}},
			}),
			want: []string{
				"Source: readiness snapshot for `spec/x` at `abc`. Current focus: Define the work.\n\n",
			},
		},
		{
			name: "a CurrentFocus and an Area id absent from Areas",
			doc: base(&ReadinessFacts{
				TargetRef: "spec/x", Head: strings.Repeat("d", 40), CurrentFocus: "unknown-area",
				Areas:     []ReadinessArea{{ID: "shape-proposal", Label: "Define the work", State: "proven"}},
				Attention: []ReadinessConcern{{ID: "weird/concern", Area: "another-unknown", State: "unproven", Blocking: true, Timing: "current", Summary: "Orphan concern", Witnesses: []string{"w1"}}},
			}),
			want: []string{
				"Source: readiness snapshot for `spec/x` at `dddddddddddd`. Current focus: unknown-area.\n\n",
				"1. Orphan concern — another-unknown; blocking; current; unproven; witnesses: w1 <a id=\"weird/concern\"></a>\n",
			},
		},
		{
			name: "a concern id with an HTML-attribute-breaking character is escaped",
			doc: base(&ReadinessFacts{
				TargetRef: "spec/x", Head: strings.Repeat("e", 40), CurrentFocus: "shape-proposal",
				Areas:     []ReadinessArea{{ID: "shape-proposal", Label: "Define the work", State: "unproven"}},
				Attention: []ReadinessConcern{{ID: `shape/question/oq-"><script>`, Area: "shape-proposal", State: "unproven", Blocking: true, Timing: "current", Summary: "Adversarial id", Witnesses: []string{"w"}}},
			}),
			want: []string{
				`<a id="shape/question/oq-&quot;&gt;&lt;script&gt;"></a>`,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md := RenderMarkdown(tt.doc)
			for _, want := range tt.want {
				if !strings.Contains(md, want) {
					t.Errorf("missing %q in:\n%s", want, md)
				}
			}
		})
	}
}

// TestEscapeAttr is the fix-round-1 F4 proof: escapeAttr's own table,
// mirroring markdown.go's TestEscapeCell for its sibling escaper.
func TestEscapeAttr(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain slug-shaped id", "shape/question/oq-2", "shape/question/oq-2"},
		{"double quote breaks out of the attribute", `x" onmouseover="alert(1)`, "x&quot; onmouseover=&quot;alert(1)"},
		{"ampersand", "a&b", "a&amp;b"},
		{"angle brackets", "<b>", "&lt;b&gt;"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := escapeAttr(tt.in); got != tt.want {
				t.Errorf("escapeAttr(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
