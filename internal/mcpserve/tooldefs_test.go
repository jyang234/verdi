package mcpserve

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/specdoc"
)

// toolDefByName returns the one tool definition named name, or fails.
func toolDefByName(t *testing.T, name string) map[string]any {
	t.Helper()
	for _, def := range toolDefs(model.Canonical()) {
		if got, _ := def["name"].(string); got == name {
			return def
		}
	}
	t.Fatalf("no tool named %q in toolDefs", name)
	return nil
}

// TestToolDefs_GetDocumentNamesEveryKindsSections (final-review F4): the
// description is get_document's ONLY reader-facing copy for the kind
// argument, so each kind's parenthetical must name every section that
// kind actually renders beyond the Identity table all three share. The
// tasks parenthetical said "plan and evidence" after R-W2-2 added
// Readiness to KindTasks.Sections(), so a caller reading the tool
// description could not learn the tasks document carries readiness at
// all. Driven off Sections() itself, so a future section addition reds
// this instead of silently drifting.
//
// KindSpec is deliberately exempt: its copy is the honest summary
// "everything" (it renders every declared section), not an enumeration.
func TestToolDefs_GetDocumentNamesEveryKindsSections(t *testing.T) {
	desc, _ := toolDefByName(t, "get_document")["description"].(string)
	if desc == "" {
		t.Fatal("get_document has no description")
	}
	for _, c := range []struct {
		kind  specdoc.Kind
		after string
	}{
		{specdoc.KindPlan, "plan ("},
		{specdoc.KindTasks, "tasks ("},
	} {
		t.Run(string(c.kind), func(t *testing.T) {
			_, rest, ok := strings.Cut(desc, c.after)
			if !ok {
				t.Fatalf("description does not describe the %s kind as %q...: %q", c.kind, c.after, desc)
			}
			parenthetical, _, ok := strings.Cut(rest, ")")
			if !ok {
				t.Fatalf("unterminated %s parenthetical in %q", c.kind, desc)
			}
			lower := strings.ToLower(parenthetical)
			for _, s := range c.kind.Sections() {
				if s == specdoc.SectionIdentity {
					continue // every kind carries it; no parenthetical names it
				}
				if !strings.Contains(lower, string(s)) {
					t.Errorf("%s parenthetical %q does not name the %q section it renders", c.kind, parenthetical, s)
				}
			}
		})
	}
	// Negative path: the parenthetical must not promise a section the
	// kind does not render — the plan document carries no evidence.
	_, planRest, _ := strings.Cut(desc, "plan (")
	planPart, _, _ := strings.Cut(planRest, ")")
	if strings.Contains(strings.ToLower(planPart), string(specdoc.SectionEvidence)) {
		t.Errorf("plan parenthetical %q names a section KindPlan does not render", planPart)
	}
}
