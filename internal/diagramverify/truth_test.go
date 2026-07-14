package diagramverify

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/upstream"
)

const cannedDir = "../../testdata/svcfix-canned"

func readCanned(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(cannedDir, name))
	if err != nil {
		t.Fatalf("reading canned fixture %s: %v", name, err)
	}
	return data
}

// TestRegenerateTruth_Unscoped is obligation ac-2--behavioral case (1):
// truth regenerates unscoped and decodes the canned graph capture
// correctly, over internal/upstream's fake Runner seam — never a real
// flowmap binary.
func TestRegenerateTruth_Unscoped(t *testing.T) {
	f := upstream.NewFakeRunner()
	f.Enqueue("flowmap", "graph", upstream.Result{Stdout: readCanned(t, "graph.json"), ExitCode: 0})

	g, err := RegenerateTruth(context.Background(), f, "testdata/svcfix", "deadbeef", "")
	if err != nil {
		t.Fatalf("RegenerateTruth: %v", err)
	}
	if g.Stamp != "deadbeef" {
		t.Fatalf("Stamp = %q, want %q", g.Stamp, "deadbeef")
	}
	if len(g.Nodes) == 0 {
		t.Fatal("Nodes is empty, want the canned graph's node set")
	}
	flags := f.Calls[0].Flags
	for _, tok := range flags {
		if tok == "-entry" {
			t.Fatalf("Flags %v contains -entry for an unscoped call, want it omitted", flags)
		}
	}
}

// TestRegenerateTruth_Scoped is obligation ac-2--behavioral case (2):
// truth regenerates with a non-empty scope and the fake runner observes
// an `-entry <scope>` flag in the request it received.
func TestRegenerateTruth_Scoped(t *testing.T) {
	f := upstream.NewFakeRunner()
	f.Enqueue("flowmap", "graph", upstream.Result{Stdout: readCanned(t, "graph.json"), ExitCode: 0})

	if _, err := RegenerateTruth(context.Background(), f, "testdata/svcfix", "deadbeef", "POST /loan-application"); err != nil {
		t.Fatalf("RegenerateTruth: %v", err)
	}
	flags := f.Calls[0].Flags
	found := false
	for i, tok := range flags {
		if tok == "-entry" && i+1 < len(flags) && flags[i+1] == "POST /loan-application" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Flags = %v, want an -entry POST /loan-application pair", flags)
	}
}

// TestRegenerateTruth_Negative_OperationalError is obligation
// ac-2--behavioral case (3): a non-zero fake exit code surfaces as an
// operational error rather than a silent empty graph.
func TestRegenerateTruth_Negative_OperationalError(t *testing.T) {
	f := upstream.NewFakeRunner()
	f.Enqueue("flowmap", "graph", upstream.Result{Stderr: []byte("bad flag"), ExitCode: 2})

	if _, err := RegenerateTruth(context.Background(), f, "testdata/svcfix", "deadbeef", ""); err == nil {
		t.Fatal("RegenerateTruth with exit 2: want error, got nil")
	}
}

func TestTruthShortNames_ExcludesCollisions(t *testing.T) {
	g := &upstream.Graph{Nodes: []upstream.Node{
		{FQN: "(*example.com/svcfix/internal/app.Service).GetRefund"},
		{FQN: "(*example.com/svcfix/internal/handler.Server).GetRefund"},
		{FQN: "(*example.com/svcfix/internal/app.Service).PublishRefund"},
	}}
	names := TruthShortNames(g)
	if names["GetRefund"] {
		t.Error(`TruthShortNames["GetRefund"] = true, want excluded (ambiguous)`)
	}
	if !names["PublishRefund"] {
		t.Error(`TruthShortNames["PublishRefund"] = false, want true`)
	}
}

func TestTruthEdgeIdentities(t *testing.T) {
	g := &upstream.Graph{Edges: []upstream.Edge{
		{From: "(*example.com/svcfix/internal/app.Service).GetRefund", To: "(*example.com/svcfix/internal/audit.Store).Write"},
	}}
	ids := TruthEdgeIdentities(g)
	if !ids[EdgeIdentity("GetRefund", "Write")] {
		t.Errorf("TruthEdgeIdentities() = %v, want GetRefund->Write present", ids)
	}
}
