// Package designscaffold is the shared spec-scaffolding core, promoted
// out of cmd/verdi/design.go (CLAUDE.md: "anything used by two or more
// packages lives in a shared internal/ package ... never copy-paste
// across packages; ... keep cmd thin"). It has two consumers: `verdi
// design start` (cmd/verdi/design.go) and the workbench's stub-instantiate
// board action (internal/workbench, spec/scoping-canvas ac-6), which
// scaffolds a story (or spike) spec from a declared stub's own real
// AC/open-question ids rather than design start's single hardcoded
// placeholder edge.
//
// Rendering is template-driven (spec/scaffold-templates ac-1, render.go):
// Render instantiates a text/template source against a ScaffoldData value;
// LoadTemplate resolves that source per class, a store's own
// .verdi/templates/<name>.md override winning over the embedded canonical
// default (templates/feature.md, templates/story.md) of the same name.
// Feature and Story below are this package's own convenience delegates to
// Render for the two classes' standard scaffold shapes — both call sites
// resolve their class's Class.Template via LoadTemplate themselves and
// pass the result in, rather than this package hardcoding a class-to-
// filename mapping.
package designscaffold

import (
	"strings"
	"unicode"

	"github.com/jyang234/verdi/internal/artifact"
)

// HumanizeName renders a kebab-case name as a Title Case placeholder
// title — moved verbatim from cmd/verdi/design.go's humanizeName. Used
// only where no real title source exists at all (05 §CLI's own I-10: "no
// magic, no tracker-derived naming" rules out inventing one from anything
// but the caller's own name).
func HumanizeName(name string) string {
	parts := strings.Split(name, "-")
	for i, p := range parts {
		if p == "" {
			continue
		}
		r := []rune(p)
		r[0] = unicode.ToUpper(r[0])
		parts[i] = string(r)
	}
	return strings.Join(parts, " ")
}

// defaultOwnersLiteral, defaultProblemText, and defaultOutcomeText are the
// scaffold's own fixed placeholder values — today's hardcoded content,
// moved verbatim from the retired string builders into ScaffoldData
// inputs rather than a second copy baked into the template text, so a
// store override template that wants real values instead of these
// placeholders has a real, data-driven field to reference
// ({{.Owners}}/{{.Problem}}/{{.Outcome}}).
const (
	defaultOwnersLiteral = "[unassigned]"
	defaultProblemText   = "TODO: replace with the real problem statement before accept"
	defaultOutcomeText   = "TODO: replace with the real outcome statement before accept"
)

// Feature renders a draft feature spec's markdown content by instantiating
// tmpl — the class's resolved template, LoadTemplate's embedded canonical
// templates/feature.md or a store's own .verdi/templates/feature.md
// override — against the standard feature scaffold's inputs (spec/
// scaffold-templates ac-1: designscaffold stops building strings and
// starts rendering templates; this function becomes Render's delegate
// rather than its own fmt.Sprintf body). storyRef is "" when the feature
// carries no tracker ref at all (optional for the feature class).
func Feature(tmpl []byte, specRef, storyRef, title string) (string, error) {
	return Render(tmpl, ScaffoldData{
		Ref:      specRef,
		Title:    title,
		StoryRef: storyRef,
		Owners:   defaultOwnersLiteral,
		Problem:  defaultProblemText,
		Outcome:  defaultOutcomeText,
	})
}

// StoryLink is one document-level edge a scaffolded story spec's `links:`
// block carries — an `implements` edge to a feature AC fragment, or (the
// spike variant) a `resolves` edge to an open-question fragment.
type StoryLink struct {
	Type artifact.LinkType
	Ref  string
}

// Story renders a draft story spec's markdown content (02 §Kind registry:
// story (NEW), including the spike variant) by instantiating tmpl — the
// class's resolved template, LoadTemplate's embedded canonical
// templates/story.md or a store's own .verdi/templates/story.md override
// — against the caller's inputs (spec/scaffold-templates ac-1: this
// function becomes Render's delegate rather than its own strings.Builder
// body). Generalized beyond design start's single hardcoded placeholder
// edge to any set of document-level links: design start
// (cmd/verdi/design.go) passes exactly one placeholder implements edge
// with no AC of its own to bind to; the workbench's stub-instantiate board
// action passes the stub's REAL implements/resolves edges (derived from
// the stub's own acceptance_criteria/resolves list). storyRef is the
// required `story:` tracker scalar (validateStory requires one
// unconditionally, even for the spike variant) — a caller with no real
// tracker ref of its own (stub-instantiate has none: ac-6 binds by slug,
// "with no new provenance record") passes an explicit placeholder value
// shaped like a real one (e.g. "todo:REPLACE-ME") rather than leaving the
// field empty and failing self-validation. spike selects the 02 spike
// variant: `spike: true`, no acceptance_criteria placeholder (spikes are
// evidence-model-exempt, 02 §Kind registry: "Spikes are exempt from the
// evidence model"), links become its resolves edges. Story does not
// itself enforce validateStory's edge-count/type grammar — an empty or
// wrongly-typed links list renders content that will fail DecodeSpec, by
// design (callers self-validate before writing, exactly like design
// start already does; the canonical story.md template's own "links:\n"
// followed by zero entries decodes as a nil Links slice, which
// validateStory then rejects for a non-spike story — TestStory_Negative_
// NoLinks pins this).
func Story(tmpl []byte, specRef, storyRef, title string, spike bool, links []StoryLink) (string, error) {
	return Render(tmpl, ScaffoldData{
		Ref:      specRef,
		Title:    title,
		StoryRef: storyRef,
		Spike:    spike,
		Links:    links,
		Owners:   defaultOwnersLiteral,
		Problem:  defaultProblemText,
		Outcome:  defaultOutcomeText,
	})
}
