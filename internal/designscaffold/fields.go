package designscaffold

// The placeholder-enumeration API (spec/creation-form ac-1, guide 5.3's
// D-1 contract made mechanical): a class template's own {{ .Placeholder }}
// slots ARE the creation surface's field contract, so the board's
// creation form (internal/workbench, spec/creation-form ac-2/ac-3) and
// the CLI interview (spec/creation-surfaces ac-3, plan Task 12) both
// derive their inputs from Fields rather than hardcoding a one-size
// form the template cannot reshape. One field contract, every front end.

import (
	"fmt"
	"text/template"
	"text/template/parse"
)

// FieldKind classifies how a creation surface sources one enumerated
// field's value — presentation-agnostic roles, so the board form and the
// CLI interview read the same descriptor without a second table.
type FieldKind string

const (
	// FieldIdentity is derived from the new spec's own name (Ref):
	// surfaces collect a kebab-case name once and derive the ref, never
	// a free-text ref input.
	FieldIdentity FieldKind = "identity"
	// FieldInput is a single-line free input (Title, Owners, StoryRef).
	FieldInput FieldKind = "input"
	// FieldStatement is a multiline statement position (Problem,
	// Outcome) — the inputs guide 6.1 says a creation surface exists to
	// collect before the artifact exists.
	FieldStatement FieldKind = "statement"
	// FieldStructural is derived from creation context (Spike, Links,
	// ParentRef) — never asked of the author as a text field.
	FieldStructural FieldKind = "structural"
)

// Field is one ordered creation-surface input descriptor: the
// ScaffoldData field a template references, and how a surface sources
// its value. Produced by Fields in first-reference document order.
type Field struct {
	Name string
	Kind FieldKind
}

// fieldKinds is the D-1 role table over ScaffoldData's fields — the one
// classification both consumers share. A template referencing a name
// outside this table cannot render (Render executes against the
// ScaffoldData STRUCT with missingkey=error semantics: an unknown field
// is an execution error by construction), so Fields refuses it by name
// rather than describing a form field whose submission would fail — the
// disclosed v1 boundary on the guide's custom-placeholder aspiration
// (spec/creation-form ac-1).
var fieldKinds = map[string]FieldKind{
	"Ref":       FieldIdentity,
	"Title":     FieldInput,
	"Owners":    FieldInput,
	"StoryRef":  FieldInput,
	"Problem":   FieldStatement,
	"Outcome":   FieldStatement,
	"ParentRef": FieldStructural,
	"Links":     FieldStructural,
	"Spike":     FieldStructural,
	// Pins/Dispositions: commit-to-design's content-carrying fields
	// (spec/creation-form ac-4, ledger L-M12) — board-derived context,
	// never author-typed.
	"Pins":         FieldStructural,
	"Dispositions": FieldStructural,
}

// Fields enumerates tmpl's placeholders as ordered field descriptors:
// first-reference document order, deduplicated, exactly the template
// positions rendered against the top-level ScaffoldData value. A range
// (or with) body's relative fields belong to the iterated element — the
// walk changes context exactly where text/template's dot does — so
// {{range .Links}} contributes Links itself and nothing from its body.
// Fails closed on a syntactically broken template and on any placeholder
// outside the ScaffoldData contract, naming it.
func Fields(tmpl []byte) ([]Field, error) {
	t, err := template.New("scaffold").Funcs(template.FuncMap{"safe": safeScalar}).Option("missingkey=error").Parse(string(tmpl))
	if err != nil {
		return nil, fmt.Errorf("designscaffold: parsing template: %w", err)
	}
	var out []Field
	seen := map[string]bool{}
	visit := func(name string) error {
		if seen[name] {
			return nil
		}
		kind, ok := fieldKinds[name]
		if !ok {
			return fmt.Errorf("designscaffold: template references placeholder .%s outside the scaffold field contract (D-1); custom placeholders are not renderable in v1, so no field can be offered for it", name)
		}
		seen[name] = true
		out = append(out, Field{Name: name, Kind: kind})
		return nil
	}
	if err := walkTopLevelFields(t.Root, visit); err != nil {
		return nil, err
	}
	return out, nil
}

// walkTopLevelFields walks node enumerating every FieldNode evaluated
// against the top-level dot. If branches keep the dot (both arms
// recurse); a range or with body rebinds it (only the pipeline and the
// else arm — executed with the ORIGINAL dot when the pipeline is
// empty/false — recurse).
func walkTopLevelFields(node parse.Node, visit func(string) error) error {
	switch n := node.(type) {
	case *parse.ListNode:
		if n == nil {
			return nil
		}
		for _, c := range n.Nodes {
			if err := walkTopLevelFields(c, visit); err != nil {
				return err
			}
		}
	case *parse.ActionNode:
		return walkPipeFields(n.Pipe, visit)
	case *parse.IfNode:
		if err := walkPipeFields(n.Pipe, visit); err != nil {
			return err
		}
		if err := walkTopLevelFields(n.List, visit); err != nil {
			return err
		}
		if n.ElseList != nil {
			return walkTopLevelFields(n.ElseList, visit)
		}
	case *parse.RangeNode:
		if err := walkPipeFields(n.Pipe, visit); err != nil {
			return err
		}
		if n.ElseList != nil {
			return walkTopLevelFields(n.ElseList, visit)
		}
	case *parse.WithNode:
		if err := walkPipeFields(n.Pipe, visit); err != nil {
			return err
		}
		if n.ElseList != nil {
			return walkTopLevelFields(n.ElseList, visit)
		}
	case *parse.TemplateNode:
		return walkPipeFields(n.Pipe, visit)
	}
	return nil
}

// walkPipeFields enumerates the top-level field references inside one
// pipeline, nested pipes included ({{printf "%q" .Title}} reaches Title
// through the command's argument list).
func walkPipeFields(p *parse.PipeNode, visit func(string) error) error {
	if p == nil {
		return nil
	}
	for _, cmd := range p.Cmds {
		for _, arg := range cmd.Args {
			switch a := arg.(type) {
			case *parse.FieldNode:
				// A chained access (.Custom.runbook) is still one
				// top-level field reference: the head identifier.
				if len(a.Ident) > 0 {
					if err := visit(a.Ident[0]); err != nil {
						return err
					}
				}
			case *parse.PipeNode:
				if err := walkPipeFields(a, visit); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
