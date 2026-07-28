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
// positions rendered against the top-level ScaffoldData value. The walk
// changes context exactly where text/template's dot does: a range (or
// with, or field-invoked sub-template) body's relative fields belong to
// the passed value, never enumerated, while a $-rooted reference names
// the top-level value from ANY context and a sub-template invoked with
// the root dot ({{template "x" .}} at top level, {{template "x" $}}
// anywhere) enumerates its body against the root. Fails closed — naming
// the placeholder or construct, never a silently partial field list
// (judged-placeholder-enumeration-fail-closed) — on a syntactically
// broken template, a placeholder outside the ScaffoldData contract, and
// any construct the walker cannot prove enumerable (a local template
// variable's use, a whole-value {{.}}/{{$}} render against the root, an
// undefined sub-template).
func Fields(tmpl []byte) ([]Field, error) {
	t, err := template.New("scaffold").Funcs(template.FuncMap{"safe": safeScalar}).Option("missingkey=error").Parse(string(tmpl))
	if err != nil {
		return nil, fmt.Errorf("designscaffold: parsing template: %w", err)
	}
	w := &fieldWalker{
		trees:   map[string]*parse.Tree{},
		seen:    map[string]bool{},
		visited: map[string]bool{},
	}
	for _, assoc := range t.Templates() {
		if assoc.Tree != nil {
			w.trees[assoc.Name()] = assoc.Tree
		}
	}
	if err := w.walk(t.Root, true); err != nil {
		return nil, err
	}
	return w.out, nil
}

// fieldWalker carries one enumeration's state: the parse trees of every
// associated (defined) template, the dedupe set, the ordered output, and
// the (template, dot-context) pairs already walked — the recursion guard
// a self-invoking define needs.
type fieldWalker struct {
	trees   map[string]*parse.Tree
	seen    map[string]bool
	visited map[string]bool
	out     []Field
}

// visit records one top-level field reference, refusing a name outside
// the ScaffoldData contract.
func (w *fieldWalker) visit(name string) error {
	if w.seen[name] {
		return nil
	}
	kind, ok := fieldKinds[name]
	if !ok {
		return fmt.Errorf("designscaffold: template references placeholder .%s outside the scaffold field contract (D-1); custom placeholders are not renderable in v1, so no field can be offered for it", name)
	}
	w.seen[name] = true
	w.out = append(w.out, Field{Name: name, Kind: kind})
	return nil
}

// walk enumerates node's references. dotIsRoot reports whether the
// current dot is the top-level ScaffoldData value: if branches keep the
// dot (both arms recurse unchanged); a range or with BODY rebinds it
// (recursed with dotIsRoot=false, so relative fields are skipped while
// $-rooted references and unprovable constructs are still seen); their
// else arms run with the ORIGINAL dot and recurse unchanged.
func (w *fieldWalker) walk(node parse.Node, dotIsRoot bool) error {
	switch n := node.(type) {
	case *parse.ListNode:
		if n == nil {
			return nil
		}
		for _, c := range n.Nodes {
			if err := w.walk(c, dotIsRoot); err != nil {
				return err
			}
		}
	case *parse.ActionNode:
		return w.walkPipe(n.Pipe, dotIsRoot)
	case *parse.IfNode:
		if err := w.walkPipe(n.Pipe, dotIsRoot); err != nil {
			return err
		}
		if err := w.walk(n.List, dotIsRoot); err != nil {
			return err
		}
		if n.ElseList != nil {
			return w.walk(n.ElseList, dotIsRoot)
		}
	case *parse.RangeNode:
		if err := w.walkPipe(n.Pipe, dotIsRoot); err != nil {
			return err
		}
		if err := w.walk(n.List, false); err != nil {
			return err
		}
		if n.ElseList != nil {
			return w.walk(n.ElseList, dotIsRoot)
		}
	case *parse.WithNode:
		if err := w.walkPipe(n.Pipe, dotIsRoot); err != nil {
			return err
		}
		if err := w.walk(n.List, false); err != nil {
			return err
		}
		if n.ElseList != nil {
			return w.walk(n.ElseList, dotIsRoot)
		}
	case *parse.TemplateNode:
		return w.walkTemplateCall(n, dotIsRoot)
	}
	return nil
}

// walkTemplateCall handles {{template "name" arg}}: the body of a
// sub-template invoked with the root dot enumerates against the root
// (the judge's define/template hole); invoked with a field (or nothing),
// its body's relative references are the passed value's own — skipped,
// exactly like a range body. An undefined name fails closed.
func (w *fieldWalker) walkTemplateCall(n *parse.TemplateNode, dotIsRoot bool) error {
	bodyDotIsRoot := false
	switch arg := singlePipeArg(n.Pipe); a := arg.(type) {
	case *parse.DotNode:
		// {{template "x" .}}: passes the CURRENT dot — root only when
		// the call site's dot is.
		bodyDotIsRoot = dotIsRoot
	case *parse.VariableNode:
		if len(a.Ident) == 1 && a.Ident[0] == "$" {
			// {{template "x" $}}: passes the root, from any context.
			bodyDotIsRoot = true
		} else if err := w.walkPipe(n.Pipe, dotIsRoot); err != nil {
			return err
		}
	default:
		// A field (enumerated normally) or nothing: the body's dot is
		// that value / nil, never the root.
		if err := w.walkPipe(n.Pipe, dotIsRoot); err != nil {
			return err
		}
	}
	tree, ok := w.trees[n.Name]
	if !ok || tree == nil {
		return fmt.Errorf("designscaffold: template invokes undefined sub-template %q — nothing to enumerate; fail closed", n.Name)
	}
	key := fmt.Sprintf("%s|%t", n.Name, bodyDotIsRoot)
	if w.visited[key] {
		return nil
	}
	w.visited[key] = true
	return w.walk(tree.Root, bodyDotIsRoot)
}

// singlePipeArg returns the pipe's sole argument node when the pipe is
// exactly one command with one argument, else nil.
func singlePipeArg(p *parse.PipeNode) parse.Node {
	if p == nil || len(p.Cmds) != 1 || len(p.Cmds[0].Args) != 1 {
		return nil
	}
	return p.Cmds[0].Args[0]
}

// walkPipe enumerates the references inside one pipeline, nested pipes
// included ({{printf "%q" .Title}} reaches Title through the command's
// argument list).
func (w *fieldWalker) walkPipe(p *parse.PipeNode, dotIsRoot bool) error {
	if p == nil {
		return nil
	}
	for _, cmd := range p.Cmds {
		for _, arg := range cmd.Args {
			if err := w.walkArg(arg, dotIsRoot); err != nil {
				return err
			}
		}
	}
	return nil
}

// walkArg classifies one pipeline argument: dot-rooted fields enumerate
// only when the dot IS the root; $-rooted fields enumerate always ($ is
// the root from any context); everything the walker cannot prove
// enumerable fails closed naming the construct.
func (w *fieldWalker) walkArg(arg parse.Node, dotIsRoot bool) error {
	switch a := arg.(type) {
	case *parse.FieldNode:
		// A chained access (.Custom.runbook) is still one top-level
		// field reference: the head identifier.
		if dotIsRoot && len(a.Ident) > 0 {
			return w.visit(a.Ident[0])
		}
	case *parse.VariableNode:
		if len(a.Ident) >= 2 && a.Ident[0] == "$" {
			// {{$.Field}}: the root's own field, from any dot context.
			return w.visit(a.Ident[1])
		}
		if len(a.Ident) == 1 && a.Ident[0] == "$" {
			return fmt.Errorf("designscaffold: template renders the whole scaffold value ($) — not a field; fail closed")
		}
		// A local template variable's use: its fields cannot be proven
		// enumerable without dataflow the walker does not do.
		return fmt.Errorf("designscaffold: template uses local variable %s — its fields cannot be proven enumerable; use direct .Field or $.Field references (D-1); fail closed", a.String())
	case *parse.DotNode:
		if dotIsRoot {
			return fmt.Errorf("designscaffold: template renders the whole scaffold value ({{.}}) — not a field; fail closed")
		}
	case *parse.ChainNode:
		// (expr).Field...: the field tail is relative to the inner
		// expression's result; only the inner expression's own
		// references enumerate.
		return w.walkArg(a.Node, dotIsRoot)
	case *parse.PipeNode:
		return w.walkPipe(a, dotIsRoot)
	}
	return nil
}
