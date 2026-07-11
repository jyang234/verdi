package decisionsweep

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/OWNER/verdi/internal/artifact"
)

// renderConflictMarkdown hand-renders a ConflictFrontmatter + body into the
// restricted-dialect flow-mapping style internal/align/render.go and
// cmd/verdi/design.go's scaffoldDraftSpec both use for generated
// frontmatter — this package's own copy of that small, per-producer
// convention (CLAUDE.md's "don't copy-paste across packages" governs
// shared LOGIC; a six-line flow-mapping renderer, re-derived per producer
// file with its own exact field set, is the established precedent here,
// not an exception to it).
func renderConflictMarkdown(fm *artifact.ConflictFrontmatter, body string) []byte {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "id: %s\n", fm.ID)
	fmt.Fprintf(&b, "kind: %s\n", fm.Kind)
	fmt.Fprintf(&b, "title: %s\n", yamlDQ(fm.Title))
	fmt.Fprintf(&b, "owners: [%s]\n", strings.Join(fm.Owners, ", "))
	if len(fm.Links) > 0 {
		b.WriteString("links:\n")
		for _, l := range fm.Links {
			fmt.Fprintf(&b, "  - { type: %s, ref: %s", l.Type, l.Ref)
			if l.Note != "" {
				fmt.Fprintf(&b, ", note: %s", yamlDQ(l.Note))
			}
			b.WriteString(" }\n")
		}
	}
	fmt.Fprintf(&b, "status: %s\n", fm.Status)
	if fm.Provenance != nil {
		fmt.Fprintf(&b, "provenance: { generator: %s, version: %s, inputs: [%s]", fm.Provenance.Generator, fm.Provenance.Version, strings.Join(fm.Provenance.Inputs, ", "))
		if fm.Provenance.Digest != "" {
			fmt.Fprintf(&b, ", digest: %s", fm.Provenance.Digest)
		}
		if fm.Provenance.Integrity != "" {
			fmt.Fprintf(&b, ", integrity: %s", fm.Provenance.Integrity)
		}
		b.WriteString(" }\n")
	}
	b.WriteString("---\n")
	b.WriteString(body)
	return []byte(b.String())
}

// yamlDQ renders s as a YAML double-quoted scalar (align/render.go's own
// doc comment explains why json.Marshal is a safe, well-tested quoter for
// this restricted dialect).
func yamlDQ(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return `""`
	}
	return string(b)
}
