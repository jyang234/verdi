// Package designscaffold is the shared spec-scaffolding core, promoted
// out of cmd/verdi/design.go (CLAUDE.md: "anything used by two or more
// packages lives in a shared internal/ package ... never copy-paste
// across packages; ... keep cmd thin"). It now has two consumers:
// `verdi design start` (cmd/verdi/design.go, unchanged behavior) and the
// workbench's stub-instantiate board action (internal/workbench,
// spec/scoping-canvas ac-6), which scaffolds a story (or spike) spec from
// a declared stub's own real AC/open-question ids rather than design
// start's single hardcoded placeholder edge.
package designscaffold

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/OWNER/verdi/internal/artifact"
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

// Feature renders a draft feature spec's markdown content (moved verbatim
// from cmd/verdi/design.go's scaffoldDraftFeatureSpec): frontmatter plus
// a minimal body, self-consistently anchored, carrying one attribute,
// AC, and stub of each per 05 §CLI's own exit criterion. storyRef is ""
// when the feature carries no tracker ref at all (optional for the
// feature class).
func Feature(specRef, storyRef, title string) string {
	storyLine := ""
	if storyRef != "" {
		storyLine = fmt.Sprintf("\nstory: %s", storyRef)
	}
	return fmt.Sprintf(`---
id: %s
kind: spec
title: %q
owners: [unassigned]
class: feature%s
status: draft
problem: { text: "TODO: replace with the real problem statement before accept", anchor: problem }
outcome: { text: "TODO: replace with the real outcome statement before accept", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "TODO: replace with real acceptance criteria before accept", evidence: [static, attestation], anchor: ac-1 }
stubs:
  - { slug: todo-replace-stub-slug, acceptance_criteria: [ac-1] }
---
# %s

## Problem

TODO: design notes.

## Outcome

TODO: design notes.

## Ac 1

TODO: design notes.
`, specRef, title, storyLine, title)
}

// StoryLink is one document-level edge a scaffolded story spec's `links:`
// block carries — an `implements` edge to a feature AC fragment, or (the
// spike variant) a `resolves` edge to an open-question fragment.
type StoryLink struct {
	Type artifact.LinkType
	Ref  string
}

// Story renders a draft story spec's markdown content (02 §Kind registry:
// story (NEW), including the spike variant), generalized beyond design
// start's single hardcoded placeholder edge to any set of document-level
// links: design start (cmd/verdi/design.go) passes exactly one
// placeholder implements edge with no AC of its own to bind to; the
// workbench's stub-instantiate board action passes the stub's REAL
// implements/resolves edges (derived from the stub's own acceptance_
// criteria/resolves list). storyRef is the required `story:` tracker
// scalar (validateStory requires one unconditionally, even for the spike
// variant) — a caller with no real tracker ref of its own (stub-
// instantiate has none: ac-6 binds by slug, "with no new provenance
// record") passes an explicit placeholder value shaped like a real one
// (e.g. "todo:REPLACE-ME") rather than leaving the field empty and
// failing self-validation. spike selects the 02 spike variant: `spike:
// true`, no acceptance_criteria placeholder (spikes are evidence-model-
// exempt, 02 §Kind registry: "Spikes are exempt from the evidence
// model"), links become its resolves edges. Story does not itself
// enforce validateStory's edge-count/type grammar — an empty or
// wrongly-typed links list renders content that will fail DecodeSpec, by
// design (callers self-validate before writing, exactly like design
// start already does).
func Story(specRef, storyRef, title string, spike bool, links []StoryLink) string {
	var b strings.Builder
	fmt.Fprintf(&b, `---
id: %s
kind: spec
title: %q
owners: [unassigned]
class: story
status: draft
story: %s
`, specRef, title, storyRef)
	if spike {
		b.WriteString("spike: true\n")
	}
	b.WriteString(`problem: { text: "TODO: replace with the real problem statement before accept", anchor: problem }
outcome: { text: "TODO: replace with the real outcome statement before accept", anchor: outcome }
`)
	if !spike {
		b.WriteString(`acceptance_criteria:
  - { id: ac-1, text: "TODO: replace with real acceptance criteria before accept", evidence: [static], anchor: ac-1 }
`)
	}
	b.WriteString("links:\n")
	for _, l := range links {
		fmt.Fprintf(&b, "  - { type: %s, ref: %q }\n", l.Type, l.Ref)
	}
	fmt.Fprintf(&b, `---
# %s

## Problem

TODO: design notes.

## Outcome

TODO: design notes.
`, title)
	if !spike {
		b.WriteString(`
## Ac 1

TODO: design notes.
`)
	}
	return b.String()
}
