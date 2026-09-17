package workbench

import (
	"net/http"
	"strings"
	"testing"
)

// TestSpecImport_PageServesSelectionMappingAffordances pins the served
// markup the selection-mapping path (spec/uat-round-1 ac-8, closing
// UAT-006) relies on: the source step tells the reader that each file's
// text is shown and that a selected passage is mapped with "Map
// selection"; the page-local stylesheet carries the read-only source
// block (monospace, whitespace preserved, selectable) and the mapping
// excerpt the list shows beside each byte range; and the advanced
// free-text target hint still names all four object kinds for power
// users. The browser behavior itself is proven by
// e2e/tests/76-import-selection-mapping.spec.ts.
func TestSpecImport_PageServesSelectionMappingAffordances(t *testing.T) {
	root := newSpecImportStore(t)
	c := newImportClient(t, NewHandlerWith(root, Deps{}))
	status, page := c.get("/design/import")
	if status != http.StatusOK {
		t.Fatalf("GET /design/import = %d", status)
	}
	index := func(marker string) int {
		t.Helper()
		i := strings.Index(page, marker)
		if i < 0 {
			t.Fatalf("import page missing %q", marker)
		}
		return i
	}

	styleEnd := index(`</style>`)
	style := page[:styleEnd]
	for _, want := range []struct{ selector, rule string }{
		{".import-source-text", "white-space: pre-wrap"},
		{".import-source-text", "font-family: var(--mono)"},
		{".import-source-text", "user-select: text"},
		{".import-mapping-excerpt", "white-space: pre-wrap"},
	} {
		i := strings.Index(style, want.selector+" {")
		if i < 0 {
			t.Errorf("page-local stylesheet missing selector %q", want.selector)
			continue
		}
		rule := style[i:]
		rule = rule[:strings.Index(rule, "}")+1]
		if !strings.Contains(rule, want.rule) {
			t.Errorf("%s rule missing %q: %s", want.selector, want.rule, rule)
		}
	}

	step := page[index(`<legend>1. Source files</legend>`):index(`<legend>2. How the primary file is read</legend>`)]
	for _, want := range []string{"Map selection", "select a passage"} {
		if !strings.Contains(step, want) {
			t.Errorf("source step copy missing %q: %s", want, step)
		}
	}

	advanced := page[index(`<details id="import-advanced"`):]
	advanced = advanced[:strings.Index(advanced, `</details>`)]
	if !strings.Contains(advanced, "ac-/co-/dc-/oq- id") {
		t.Errorf("advanced hint no longer names the free-text target grammar: %s", advanced)
	}
	if !strings.Contains(advanced, "Map selection") {
		t.Errorf("advanced hint does not point the reader back to the selection path: %s", advanced)
	}
}
