package mcpserve

import (
	"context"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

// TestGetLinks_SurfacesExemptedBy proves D-7 end-to-end through the tool: an
// `exempts` edge now inverts (internal/index's inverseOf gained
// exempts→exempted-by), and get_links's generic backlink rendering surfaces
// that computed inverse verbatim — no per-type wiring, the same path
// implemented-by already travels. The `resolves→resolved-by` inverse is
// proven at the index level (internal/index.TestBuildBacklinks_ResolvesAndExempts);
// a spike's resolves edge targets an object FRAGMENT, whose whole-ref
// get_links query is the separate D-8 key-mismatch gap left out of scope.
func TestGetLinks_SurfacesExemptedBy(t *testing.T) {
	const adr = `---
id: adr/0007-policy
kind: adr
title: "Policy ADR"
status: accepted
owners: [platform-team]
decided: 2026-01-01
frozen: { at: 2026-01-01, commit: 0000000000000000000000000000000000000a }
---
# Policy ADR

An ADR a downstream spec exempts itself from.
`
	const spec = `---
id: spec/scoped-notes
kind: spec
class: component
title: "Scoped notes"
status: active
owners: [platform-team]
links:
  - { type: exempts, ref: adr/0007-policy }
---
# Scoped notes

A component spec carrying a top-level exempts edge to the policy ADR.
`
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/adr/0007-policy.md":                adr,
			".verdi/specs/active/scoped-notes/spec.md": spec,
		},
		Message: "adr + exempting spec",
	}})

	b := &Backend{Root: repo.Dir}
	result := b.GetLinks(context.Background(), mustArgs(t, map[string]any{"ref": "adr/0007-policy"}))
	var out struct {
		Backlinks []backlinkItem `json:"backlinks"`
	}
	toolResultJSON(t, result, &out)

	found := false
	for _, bl := range out.Backlinks {
		if bl.From == "spec/scoped-notes" && bl.Type == "exempted-by" {
			found = true
		}
	}
	if !found {
		t.Fatalf("get_links(adr/0007-policy) did not surface an exempted-by backlink: %+v", out.Backlinks)
	}
}
