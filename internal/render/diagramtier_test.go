package render

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

// The dc-1 badge grammar, asserted verbatim: the figure wrapper's semantic
// marker and the visible figcaption chip. Every table entry below keys off
// these exact strings so a wording drift fails loudly in one place.
const (
	wantIllustrativeMarker = `<figure class="diagram-figure" data-diagram-tier="illustrative">`
	wantIllustrativeChip   = `<figcaption class="diagram-tier-badge">illustrative · not deterministically verifiable</figcaption>`
)

// TestRenderMermaidBlock_IllustrativeFigure pins spec/illustrative-class
// dc-1 at the one shared seam: the mermaid wrapper is a badged figure —
// data-diagram-tier="illustrative" on the figure element, the visible
// figcaption chip, and the diagram source still HTML-escaped inside the
// <pre class="mermaid"> the client-side engine consumes.
func TestRenderMermaidBlock_IllustrativeFigure(t *testing.T) {
	out := RenderMermaidBlock("graph TD\n  a --> b\n")

	for _, want := range []string{
		wantIllustrativeMarker,
		wantIllustrativeChip,
		`<pre class="mermaid">`,
		"a --&gt; b", // escaped, mermaid reads textContent
		"</figure>",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("RenderMermaidBlock missing %q, got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "a --> b") {
		t.Errorf("diagram source must be HTML-escaped inside the pre, got:\n%s", out)
	}
	// The badge sits on the SAME figure as the diagram: pre inside figure.
	if !strings.HasPrefix(out, wantIllustrativeMarker) {
		t.Errorf("figure must open the wrapper, got:\n%s", out)
	}
}

// TestRenderBody_TierDispatch is the ac-2 static table (obligation
// ac-2--static): the badge covers the fenced body-figure path and the
// non-proposal diagram-kind path, and the class: proposal path is NEVER
// painted with the illustrative badge or marker (the negative case).
func TestRenderBody_TierDispatch(t *testing.T) {
	mermaidSrc := "graph TD\n  a --> b\n"
	outOfGrammar := "sequenceDiagram\n  Alice->>Bob: hello\n"

	tests := []struct {
		name        string
		kind, class string
		body        string
		wantSubstr  []string
		banSubstr   []string
	}{
		{
			name:  "fenced mermaid in a markdown body is illustrative by location (dc-2)",
			kind:  string(artifact.KindSpec),
			class: "",
			body:  "# T\n\n```mermaid\n" + mermaidSrc + "```\n",
			wantSubstr: []string{
				wantIllustrativeMarker,
				wantIllustrativeChip,
				`<pre class="mermaid">`,
			},
		},
		{
			name:  "a diagram-kind artifact without class: proposal is illustrative by class (dc-2)",
			kind:  string(artifact.KindDiagram),
			class: "",
			body:  mermaidSrc,
			wantSubstr: []string{
				wantIllustrativeMarker,
				wantIllustrativeChip,
				`<pre class="mermaid">`,
				"a --&gt; b",
			},
			banSubstr: []string{"<p>"},
		},
		{
			name:  "NEGATIVE: the class: proposal path emits no illustrative badge and no illustrative marker (ac-2)",
			kind:  string(artifact.KindDiagram),
			class: artifact.DiagramClassProposal,
			body:  mermaidSrc,
			wantSubstr: []string{
				// The proposal's surfaces carry the extractor-computed tier
				// instead (ac-3): this source is inside the declared grammar.
				`data-diagram-tier="full"`,
				`<pre class="mermaid">`,
			},
			banSubstr: []string{"illustrative"},
		},
		{
			name:  "a proposal beyond the extractor grammar wears the partial tier, still never illustrative",
			kind:  string(artifact.KindDiagram),
			class: artifact.DiagramClassProposal,
			body:  outOfGrammar,
			wantSubstr: []string{
				`data-diagram-tier="partial"`,
				`<pre class="mermaid">`,
			},
			banSubstr: []string{"illustrative"},
		},
		{
			name:       "a non-diagram body with no mermaid carries no tier marker at all",
			kind:       string(artifact.KindSpec),
			class:      "",
			body:       "# T\n\nProse only.\n",
			wantSubstr: []string{`<h1 id="t">T</h1>`},
			banSubstr:  []string{"data-diagram-tier", "diagram-tier-badge"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := RenderBody(tt.kind, tt.class, tt.body)
			if err != nil {
				t.Fatalf("RenderBody: %v", err)
			}
			for _, want := range tt.wantSubstr {
				if !strings.Contains(out, want) {
					t.Errorf("missing %q in:\n%s", want, out)
				}
			}
			for _, ban := range tt.banSubstr {
				if strings.Contains(out, ban) {
					t.Errorf("forbidden substring %q present in:\n%s", ban, out)
				}
			}
		})
	}
}

// TestRenderBody_UnknownDiagramClassFailsClosed: an unrecognized class
// value must not silently pick a tier — strict-decode upstream refuses it,
// and the render seam refuses it too rather than guessing (fail closed:
// painting a proposal illustrative would be ac-2's inverted blending lie).
func TestRenderBody_UnknownDiagramClassFailsClosed(t *testing.T) {
	_, err := RenderBody(string(artifact.KindDiagram), "sketch", "graph TD\n  a --> b\n")
	if err == nil {
		t.Fatal("RenderBody accepted an unknown diagram class; must fail closed")
	}
}

// TestDiagramFigure_Determinism (co-1): the badge markup is a pure
// function of the artifact bytes — byte-identical across repeated calls,
// no clock, no randomness, no store lookup in any input.
func TestDiagramFigure_Determinism(t *testing.T) {
	inputs := []struct {
		kind, class, body string
	}{
		{string(artifact.KindDiagram), "", "graph TD\n  a --> b\n"},
		{string(artifact.KindDiagram), artifact.DiagramClassProposal, "graph TD\n  a --> b\n"},
		{string(artifact.KindSpec), "", "```mermaid\ngraph TD\n  a --> b\n```\n"},
	}
	for _, in := range inputs {
		first, err := RenderBody(in.kind, in.class, in.body)
		if err != nil {
			t.Fatalf("RenderBody(%s/%s): %v", in.kind, in.class, err)
		}
		for i := 0; i < 5; i++ {
			again, err := RenderBody(in.kind, in.class, in.body)
			if err != nil {
				t.Fatalf("RenderBody(%s/%s) run %d: %v", in.kind, in.class, i, err)
			}
			if again != first {
				t.Fatalf("RenderBody(%s/%s) not byte-deterministic:\nfirst: %q\nagain: %q", in.kind, in.class, first, again)
			}
		}
	}
}

// TestProposalFigure_ChipIsVisibleAndDistinct (ac-3): the proposal figure
// carries a visible chip too — the extractor-computed vocabulary — so the
// two tiers are visually AND semantically distinguishable, and the two
// markers differ.
func TestProposalFigure_ChipIsVisibleAndDistinct(t *testing.T) {
	ill := RenderMermaidBlock("graph TD\n  a --> b\n")
	prop, err := RenderBody(string(artifact.KindDiagram), artifact.DiagramClassProposal, "graph TD\n  a --> b\n")
	if err != nil {
		t.Fatalf("RenderBody proposal: %v", err)
	}
	if !strings.Contains(prop, `<figcaption class="diagram-tier-badge diagram-tier-badge--proposal">proposal · full coverage</figcaption>`) {
		t.Errorf("proposal figure missing its visible tier chip, got:\n%s", prop)
	}
	if !strings.Contains(ill, `data-diagram-tier="illustrative"`) || !strings.Contains(prop, `data-diagram-tier="full"`) {
		t.Errorf("tier markers must differ and be non-empty:\nillustrative: %s\nproposal: %s", ill, prop)
	}
}
