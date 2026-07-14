package diagramedit

import (
	"errors"
	"strings"
	"testing"
)

// adversarialSrc is a renderer-legal flowchart with deliberately unusual
// formatting (obligation ac-3--static: mixed indentation, %% comments,
// blank lines, trailing spaces, no uniform style) — every op below must
// leave its untouched bytes bit-identical. A graph round-trip would
// reorder or reformat this (mixed indents, the comment between edges, the
// trailing spaces); surviving bit-for-bit is the no-round-trip witness.
const adversarialSrc = "flowchart TD\n" +
	"  a[\"Loan service\"]   \n" +
	"\tb\n" +
	"\n" +
	"  %% a comment that must survive verbatim   \n" +
	"    a --> b\n" +
	"  c[\"Notification\"]\n" +
	"  b-->c\n"

func TestParse_Happy_SubsetModel(t *testing.T) {
	d, err := Parse([]byte(adversarialSrc))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	nodes := d.Nodes()
	wantNodes := []Node{{ID: "a", Label: "Loan service"}, {ID: "b"}, {ID: "c", Label: "Notification"}}
	if len(nodes) != len(wantNodes) {
		t.Fatalf("Nodes = %+v, want %+v", nodes, wantNodes)
	}
	for i := range wantNodes {
		if nodes[i] != wantNodes[i] {
			t.Errorf("Nodes[%d] = %+v, want %+v", i, nodes[i], wantNodes[i])
		}
	}
	edges := d.Edges()
	wantEdges := []Edge{{From: "a", To: "b"}, {From: "b", To: "c"}}
	if len(edges) != len(wantEdges) {
		t.Fatalf("Edges = %+v, want %+v", edges, wantEdges)
	}
	for i := range wantEdges {
		if edges[i] != wantEdges[i] {
			t.Errorf("Edges[%d] = %+v, want %+v", i, edges[i], wantEdges[i])
		}
	}
}

// TestParse_OutsideSubset pins the disclosed-unavailable boundary (ac-2,
// dc-2): every construct the grammar does not name returns a typed
// *OutsideSubsetError — never a partial model, never a rewrite.
func TestParse_OutsideSubset(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"sequence diagram", "sequenceDiagram\n  Alice->>Bob: hi\n"},
		{"state diagram", "stateDiagram-v2\n  [*] --> Still\n"},
		{"labeled edge", "flowchart TD\n  a -->|label| b\n"},
		{"dashed edge", "flowchart TD\n  a -.-> b\n"},
		{"hexagon node", "flowchart TD\n  a{{\"bus\"}}\n"},
		{"stadium node", "flowchart TD\n  a([\"ext\"])\n"},
		{"unquoted label", "flowchart TD\n  a[plain]\n"},
		{"subgraph block", "flowchart TD\n  subgraph s1\n  a\n  end\n"},
		{"bare reserved word", "flowchart TD\n  end\n"},
		{"classDef", "flowchart TD\n  classDef k fill:#fff\n"},
		{"second header", "flowchart TD\ngraph LR\n"},
		{"node before header", "a[\"x\"]\nflowchart TD\n"},
		{"no header at all", "  a --> b\n"},
		{"empty source", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(tc.src))
			var outside *OutsideSubsetError
			if !errors.As(err, &outside) {
				t.Fatalf("Parse(%q) err = %v, want *OutsideSubsetError", tc.src, err)
			}
			if !strings.Contains(outside.Error(), "code pane stays live") {
				t.Errorf("error %q does not carry the pane-stays-live disclosure", outside.Error())
			}
		})
	}
}

// TestOps_OutsideSubset_TypedRefusal: every op against out-of-subset
// source returns the typed ops-unavailable result and NO rewritten source
// (obligation ac-2--static's negative path).
func TestOps_OutsideSubset_TypedRefusal(t *testing.T) {
	src := []byte("sequenceDiagram\n  Alice->>Bob: hi\n")
	checks := []struct {
		name string
		run  func() ([]byte, error)
	}{
		{"add-node", func() ([]byte, error) { out, _, err := AddNode(src, "x"); return out, err }},
		{"connect", func() ([]byte, error) { return Connect(src, "a", "b") }},
		{"rename", func() ([]byte, error) { return Rename(src, "a", "x") }},
		{"delete-node", func() ([]byte, error) { return DeleteNode(src, "a") }},
		{"delete-edge", func() ([]byte, error) { return DeleteEdge(src, "a", "b") }},
	}
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			out, err := c.run()
			var outside *OutsideSubsetError
			if !errors.As(err, &outside) {
				t.Fatalf("err = %v, want *OutsideSubsetError", err)
			}
			if out != nil {
				t.Fatalf("op returned source bytes %q alongside the refusal; must return none", out)
			}
		})
	}
}

// diffLines returns the 0-based indices of lines that differ between a
// and b under an alignment where b is a with `removed` lines gone and
// `added` lines appended — helpers for the exhaustive-diff assertions.
func splitKeepAll(s string) []string { return strings.Split(s, "\n") }

func TestAddNode(t *testing.T) {
	t.Run("appends one line with the lowest unused n<k> id at prevailing indentation", func(t *testing.T) {
		out, id, err := AddNode([]byte(adversarialSrc), "Billing")
		if err != nil {
			t.Fatalf("AddNode: %v", err)
		}
		if id != "n1" {
			t.Errorf("id = %q, want n1", id)
		}
		// Exhaustive: the output is the input plus EXACTLY one appended
		// line — every prior byte bit-identical (obligation ac-3--static).
		want := adversarialSrc + "  n1[\"Billing\"]\n"
		if string(out) != want {
			t.Fatalf("AddNode result:\n%q\nwant:\n%q", out, want)
		}
	})

	t.Run("lowest unused id skips taken n<k>", func(t *testing.T) {
		src := "flowchart TD\n  n1[\"a\"]\n  n2\n"
		out, id, err := AddNode([]byte(src), "x")
		if err != nil {
			t.Fatalf("AddNode: %v", err)
		}
		if id != "n3" {
			t.Errorf("id = %q, want n3 (n1, n2 taken)", id)
		}
		if want := src + "  n3[\"x\"]\n"; string(out) != want {
			t.Fatalf("result %q, want %q", out, want)
		}
	})

	t.Run("fills a gap: n1 free while n2 taken", func(t *testing.T) {
		src := "flowchart TD\n  n2[\"b\"]\n"
		_, id, err := AddNode([]byte(src), "x")
		if err != nil {
			t.Fatalf("AddNode: %v", err)
		}
		if id != "n1" {
			t.Errorf("id = %q, want n1 (the LOWEST unused)", id)
		}
	})

	t.Run("no trailing newline: the final line's bytes survive and the file stays unterminated", func(t *testing.T) {
		src := "flowchart TD\n  a --> b"
		out, _, err := AddNode([]byte(src), "x")
		if err != nil {
			t.Fatalf("AddNode: %v", err)
		}
		if want := src + "\n  n1[\"x\"]"; string(out) != want {
			t.Fatalf("result %q, want %q", out, want)
		}
	})

	t.Run("negative: label a quoted grammar cannot carry", func(t *testing.T) {
		for _, label := range []string{``, `has "quotes"`, "line\nbreak"} {
			_, _, err := AddNode([]byte(adversarialSrc), label)
			var op *OpError
			if !errors.As(err, &op) {
				t.Errorf("AddNode(label=%q) err = %v, want *OpError", label, err)
			}
		}
	})
}

func TestConnect(t *testing.T) {
	t.Run("appends one <from> --> <to> line", func(t *testing.T) {
		out, err := Connect([]byte(adversarialSrc), "c", "a")
		if err != nil {
			t.Fatalf("Connect: %v", err)
		}
		if want := adversarialSrc + "  c --> a\n"; string(out) != want {
			t.Fatalf("result %q, want %q", out, want)
		}
	})
	t.Run("negative: unknown endpoint refused, nothing minted", func(t *testing.T) {
		for _, pair := range [][2]string{{"a", "ghost"}, {"ghost", "a"}} {
			out, err := Connect([]byte(adversarialSrc), pair[0], pair[1])
			var op *OpError
			if !errors.As(err, &op) {
				t.Errorf("Connect(%v) err = %v, want *OpError", pair, err)
			}
			if out != nil {
				t.Errorf("Connect(%v) returned bytes alongside refusal", pair)
			}
		}
	})
}

func TestRename(t *testing.T) {
	t.Run("rewrites only the label at the defining occurrence; id and every other byte survive", func(t *testing.T) {
		out, err := Rename([]byte(adversarialSrc), "a", "Loan orchestrator")
		if err != nil {
			t.Fatalf("Rename: %v", err)
		}
		want := strings.Replace(adversarialSrc, `a["Loan service"]`, `a["Loan orchestrator"]`, 1)
		if string(out) != want {
			t.Fatalf("result:\n%q\nwant:\n%q", out, want)
		}
		// Line-level exhaustive diff: exactly one line differs.
		got, orig := splitKeepAll(string(out)), splitKeepAll(adversarialSrc)
		if len(got) != len(orig) {
			t.Fatalf("line count changed: %d -> %d", len(orig), len(got))
		}
		changed := 0
		for i := range orig {
			if got[i] != orig[i] {
				changed++
			}
		}
		if changed != 1 {
			t.Errorf("%d lines changed, want exactly 1", changed)
		}
	})

	t.Run("bare node gains brackets, trailing whitespace preserved", func(t *testing.T) {
		src := "flowchart TD\n\tb   \n\ta --> b\n"
		out, err := Rename([]byte(src), "b", "Billing")
		if err != nil {
			t.Fatalf("Rename: %v", err)
		}
		if want := "flowchart TD\n\tb[\"Billing\"]   \n\ta --> b\n"; string(out) != want {
			t.Fatalf("result %q, want %q", out, want)
		}
	})

	t.Run("first defining occurrence only", func(t *testing.T) {
		src := "flowchart TD\n  a[\"one\"]\n  a[\"two\"]\n"
		out, err := Rename([]byte(src), "a", "new")
		if err != nil {
			t.Fatalf("Rename: %v", err)
		}
		if want := "flowchart TD\n  a[\"new\"]\n  a[\"two\"]\n"; string(out) != want {
			t.Fatalf("result %q, want %q", out, want)
		}
	})

	t.Run("negative: unknown node, edge-only node, illegal label", func(t *testing.T) {
		cases := []struct{ id, label string }{
			{"ghost", "x"},
			{"a", `ill"egal`},
		}
		for _, c := range cases {
			if _, err := Rename([]byte(adversarialSrc), c.id, c.label); err == nil {
				t.Errorf("Rename(%q, %q) = nil error, want refusal", c.id, c.label)
			}
		}
		// A node appearing ONLY inside edge lines has no defining
		// occurrence to rewrite: typed refusal, no rewrite.
		src := "flowchart TD\n  x --> y\n"
		_, err := Rename([]byte(src), "x", "label")
		var op *OpError
		if !errors.As(err, &op) {
			t.Fatalf("Rename(edge-only node) err = %v, want *OpError", err)
		}
	})
}

func TestDeleteNode(t *testing.T) {
	t.Run("removes the defining line and every edge line naming it, nothing else", func(t *testing.T) {
		out, err := DeleteNode([]byte(adversarialSrc), "b")
		if err != nil {
			t.Fatalf("DeleteNode: %v", err)
		}
		want := "flowchart TD\n" +
			"  a[\"Loan service\"]   \n" +
			"\n" +
			"  %% a comment that must survive verbatim   \n" +
			"  c[\"Notification\"]\n"
		if string(out) != want {
			t.Fatalf("result:\n%q\nwant:\n%q", out, want)
		}
	})

	t.Run("final unterminated line removal leaves no dangling separator", func(t *testing.T) {
		// Removing both "  b\n" and the final unterminated "  a --> b"
		// (which takes its preceding separator — b's own "\n", already
		// gone) leaves the "\n" terminating "  a" untouched.
		src := "flowchart TD\n  a\n  b\n  a --> b"
		out, err := DeleteNode([]byte(src), "b")
		if err != nil {
			t.Fatalf("DeleteNode: %v", err)
		}
		if want := "flowchart TD\n  a\n"; string(out) != want {
			t.Fatalf("result %q, want %q", out, want)
		}
	})

	t.Run("negative: unknown node refused", func(t *testing.T) {
		_, err := DeleteNode([]byte(adversarialSrc), "ghost")
		var op *OpError
		if !errors.As(err, &op) {
			t.Fatalf("err = %v, want *OpError", err)
		}
	})
}

func TestDeleteEdge(t *testing.T) {
	t.Run("removes exactly that one line", func(t *testing.T) {
		out, err := DeleteEdge([]byte(adversarialSrc), "a", "b")
		if err != nil {
			t.Fatalf("DeleteEdge: %v", err)
		}
		want := strings.Replace(adversarialSrc, "    a --> b\n", "", 1)
		if string(out) != want {
			t.Fatalf("result:\n%q\nwant:\n%q", out, want)
		}
	})
	t.Run("duplicate edge lines: one delete removes one line", func(t *testing.T) {
		src := "flowchart TD\n  a\n  b\n  a --> b\n  a --> b\n"
		out, err := DeleteEdge([]byte(src), "a", "b")
		if err != nil {
			t.Fatalf("DeleteEdge: %v", err)
		}
		if want := "flowchart TD\n  a\n  b\n  a --> b\n"; string(out) != want {
			t.Fatalf("result %q, want %q", out, want)
		}
	})
	t.Run("negative: direction matters, absence refused", func(t *testing.T) {
		for _, pair := range [][2]string{{"b", "a"}, {"a", "c"}} {
			_, err := DeleteEdge([]byte(adversarialSrc), pair[0], pair[1])
			var op *OpError
			if !errors.As(err, &op) {
				t.Errorf("DeleteEdge(%v) err = %v, want *OpError", pair, err)
			}
		}
	})
}

// TestOps_Determinism is obligation ac-2--static's determinism property:
// the same source and operation yield identical bytes across repeated
// calls — no clock, no randomness, no hidden state.
func TestOps_Determinism(t *testing.T) {
	src := []byte(adversarialSrc)
	ops := []struct {
		name string
		run  func() ([]byte, error)
	}{
		{"add-node", func() ([]byte, error) { out, _, err := AddNode(src, "Billing"); return out, err }},
		{"connect", func() ([]byte, error) { return Connect(src, "c", "a") }},
		{"rename", func() ([]byte, error) { return Rename(src, "a", "renamed") }},
		{"delete-node", func() ([]byte, error) { return DeleteNode(src, "b") }},
		{"delete-edge", func() ([]byte, error) { return DeleteEdge(src, "a", "b") }},
	}
	for _, op := range ops {
		t.Run(op.name, func(t *testing.T) {
			first, err := op.run()
			if err != nil {
				t.Fatalf("%s: %v", op.name, err)
			}
			for i := 0; i < 5; i++ {
				again, err := op.run()
				if err != nil {
					t.Fatalf("%s (repeat %d): %v", op.name, i, err)
				}
				if string(again) != string(first) {
					t.Fatalf("%s: repeat %d produced different bytes", op.name, i)
				}
			}
			if string(src) != adversarialSrc {
				t.Fatalf("%s mutated its input slice", op.name)
			}
		})
	}
}

// TestOps_PreserveUntouchedBytes is the exhaustive-diff property
// (obligation ac-3--static): applying each op and diffing against the
// input shows changes ONLY on the lines dc-2's grammar names for that op.
func TestOps_PreserveUntouchedBytes(t *testing.T) {
	src := []byte(adversarialSrc)
	origLines := splitKeepAll(adversarialSrc)

	assertOnlyAppended := func(t *testing.T, out []byte, wantAppended string) {
		t.Helper()
		// Byte-level: the output must literally begin with the untouched
		// input bytes (the source ends "\n", so the append adds after it).
		if !strings.HasPrefix(string(out), adversarialSrc) {
			t.Fatalf("output does not begin with the input's own bytes:\n%q", out)
		}
		if got := strings.TrimPrefix(string(out), adversarialSrc); got != wantAppended+"\n" {
			t.Fatalf("appended bytes = %q, want %q", got, wantAppended+"\n")
		}
	}

	t.Run("add-node appends only", func(t *testing.T) {
		out, _, err := AddNode(src, "Billing")
		if err != nil {
			t.Fatal(err)
		}
		assertOnlyAppended(t, out, `  n1["Billing"]`)
	})
	t.Run("connect appends only", func(t *testing.T) {
		out, err := Connect(src, "c", "a")
		if err != nil {
			t.Fatal(err)
		}
		assertOnlyAppended(t, out, "  c --> a")
	})
	t.Run("delete-node removes only its grammar-named lines", func(t *testing.T) {
		out, err := DeleteNode(src, "c")
		if err != nil {
			t.Fatal(err)
		}
		got := splitKeepAll(string(out))
		// c's defining line and the b-->c edge line are gone; every
		// remaining line is bit-identical and in order.
		var wantRemaining []string
		for _, l := range origLines {
			if l == `  c["Notification"]` || l == "  b-->c" {
				continue
			}
			wantRemaining = append(wantRemaining, l)
		}
		if len(got) != len(wantRemaining) {
			t.Fatalf("lines = %q, want %q", got, wantRemaining)
		}
		for i := range wantRemaining {
			if got[i] != wantRemaining[i] {
				t.Fatalf("line %d = %q, want %q", i, got[i], wantRemaining[i])
			}
		}
	})
}
