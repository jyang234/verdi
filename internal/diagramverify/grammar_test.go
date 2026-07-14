package diagramverify

import "testing"

// pristineTruth is the FQN set behind the pristine fixture below — no two
// entries share a ShortName, so every node resolves unambiguously.
var pristineTruth = []string{
	"(*example.com/svcfix/internal/app.Service).GetRefund",
	"(*example.com/svcfix/internal/app.Service).PublishRefund",
	"(*example.com/svcfix/internal/audit.Store).Write",
	"(*example.com/svcfix/internal/bus.Bus).Publish",
}

// TestParse_Pristine_FullCoverage is obligation ac-1--behavioral case (1):
// a fixture using every one of the four edge forms, both node-declaration
// forms, a %% comment line, and a classDef/:::classname node-class
// assignment — proving a pristine, flowmap-generated-style diagram (which
// always carries at least a header comment and its classDefs, dc-1) can
// reach full coverage.
func TestParse_Pristine_FullCoverage(t *testing.T) {
	src := `flowchart LR
    %% static call graph — scope: whole graph; algo: rta
    classDef fallible stroke:#c44,stroke-width:2px
    A["GetRefund"]:::fallible
    B["PublishRefund"]
    C
    A --> B
    B -->|via outbox| C
    B -.-> D
    D -. async .-> E
`
	ext := Parse(src, pristineTruth)
	if ext.Coverage != CoverageFull {
		t.Fatalf("Coverage = %q, want full", ext.Coverage)
	}

	wantNodes := map[string]bool{"A": true, "B": true, "C": true, "D": true, "E": true}
	if len(ext.Nodes) != len(wantNodes) {
		t.Fatalf("Nodes = %+v, want exactly %v", ext.Nodes, wantNodes)
	}
	for _, n := range ext.Nodes {
		if !wantNodes[n.RawID] {
			t.Errorf("unexpected node %q", n.RawID)
		}
		if n.Ambiguous {
			t.Errorf("node %q: Ambiguous = true, want false", n.RawID)
		}
	}

	wantEdges := []Edge{
		{From: "A", To: "B"},
		{From: "B", To: "C"},
		{From: "B", To: "D"},
		{From: "D", To: "E"},
	}
	if len(ext.Edges) != len(wantEdges) {
		t.Fatalf("Edges = %+v, want %+v", ext.Edges, wantEdges)
	}
	for i, e := range wantEdges {
		if ext.Edges[i] != e {
			t.Errorf("Edges[%d] = %+v, want %+v", i, ext.Edges[i], e)
		}
	}
}

// TestParse_OutOfGrammarConstruct_DowngradesWholeArtifact is obligation
// ac-1--behavioral case (2): a fixture containing one construct outside
// the declared grammar (a subgraph block, dc-1's named example) downgrades
// the WHOLE artifact to partial while the recognized lines still extract.
func TestParse_OutOfGrammarConstruct_DowngradesWholeArtifact(t *testing.T) {
	src := `flowchart LR
    A["GetRefund"]
    B["PublishRefund"]
    subgraph loansvc
    A --> B
    end
`
	ext := Parse(src, pristineTruth)
	if ext.Coverage != CoveragePartial {
		t.Fatalf("Coverage = %q, want partial", ext.Coverage)
	}
	// The recognized lines (both node declarations and the edge) still
	// extract — best-effort disclosure, dc-1.
	if len(ext.Nodes) != 2 {
		t.Fatalf("Nodes = %+v, want 2 (A, B)", ext.Nodes)
	}
	if len(ext.Edges) != 1 || ext.Edges[0] != (Edge{From: "A", To: "B"}) {
		t.Fatalf("Edges = %+v, want [{A B}]", ext.Edges)
	}
}

// TestParse_IdentityCollision_DowngradesWholeArtifact is obligation
// ac-1--behavioral case (3): two distinct truth FQNs normalize to the same
// ShortName; the affected proposal node is excluded from full
// classification (Ambiguous) and the artifact downgrades to partial
// rather than guessing which truth node was meant.
func TestParse_IdentityCollision_DowngradesWholeArtifact(t *testing.T) {
	collidingTruth := []string{
		"(*example.com/svcfix/internal/app.Service).GetRefund",
		"(*example.com/svcfix/internal/handler.Server).GetRefund",
	}
	src := `flowchart LR
    GetRefund["GetRefund"]
`
	ext := Parse(src, collidingTruth)
	if ext.Coverage != CoveragePartial {
		t.Fatalf("Coverage = %q, want partial (identity collision)", ext.Coverage)
	}
	if len(ext.Nodes) != 1 || !ext.Nodes[0].Ambiguous {
		t.Fatalf("Nodes = %+v, want one Ambiguous node", ext.Nodes)
	}
}

// TestParse_AllRecognizedForms_Table exercises each declared form in
// isolation (the static evidence's "recognized token forms enumerated in
// code" claim, proven per-form here rather than only in combination).
func TestParse_AllRecognizedForms_Table(t *testing.T) {
	cases := []struct {
		name string
		line string
	}{
		{"direction flowchart", "flowchart LR"},
		{"direction graph", "graph TD"},
		{"comment", "%% a header comment"},
		{"classDef", "classDef fallible stroke:#c44,stroke-width:2px"},
		{"bare id", "A"},
		{"rectangle label", `A["a label"]`},
		{"rectangle label + classname", `A["a label"]:::fallible`},
		{"cylinder label (db)", `A[("a label")]:::db`},
		{"hexagon label (bus)", `A{{"a label"}}:::bus`},
		{"stadium label (external)", `A(["a label"]):::external`},
		{"edge plain", "A --> B"},
		{"edge labeled", "A -->|via outbox| B"},
		{"edge dashed", "A -.-> B"},
		{"edge dashed labeled", "A -. async .-> B"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ext := Parse(tc.line, nil)
			if ext.Coverage != CoverageFull {
				t.Errorf("Parse(%q).Coverage = %q, want full (line should be recognized)", tc.line, ext.Coverage)
			}
		})
	}
}

// TestParse_UnrecognizedLine_Negative proves an arbitrary unrecognized
// line (not merely the subgraph example) downgrades coverage — the
// negative-path complement to the recognized-forms table above.
func TestParse_UnrecognizedLine_Negative(t *testing.T) {
	cases := map[string]string{
		"click binding":      `click A "https://example.com"`,
		"style directive":    "style A fill:#f9f,stroke:#333",
		"malformed edge":     "A ==> B",
		"unterminated label": `A["unterminated`,
	}
	for name, line := range cases {
		t.Run(name, func(t *testing.T) {
			ext := Parse(line, nil)
			if ext.Coverage != CoveragePartial {
				t.Errorf("Parse(%q).Coverage = %q, want partial", line, ext.Coverage)
			}
		})
	}
}

// TestParse_Empty proves the zero-content case (an empty proposal body)
// is full coverage with no nodes/edges rather than an error — Parse never
// returns an error at all (dc-4 of the parent feature: verification never
// blocks).
func TestParse_Empty(t *testing.T) {
	ext := Parse("", nil)
	if ext.Coverage != CoverageFull {
		t.Errorf("Coverage = %q, want full", ext.Coverage)
	}
	if len(ext.Nodes) != 0 || len(ext.Edges) != 0 {
		t.Errorf("Nodes/Edges = %+v/%+v, want both empty", ext.Nodes, ext.Edges)
	}
}
