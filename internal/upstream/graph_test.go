package upstream

import (
	"os"
	"path/filepath"
	"testing"
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

// TestDecodeGraph_Happy decodes spike S1's real capture (via the S1
// binaries run against testdata/svcfix) and checks the fields that matter
// to the rest of this module: the one obligation, its SATISFIED status,
// and a handful of structural facts.
func TestDecodeGraph_Happy(t *testing.T) {
	g, err := DecodeGraph(readCanned(t, "graph.json"))
	if err != nil {
		t.Fatalf("DecodeGraph: %v", err)
	}
	if g.Stamp != "deadbeef" {
		t.Errorf("Stamp = %q, want %q", g.Stamp, "deadbeef")
	}
	if g.Algo != "rta" {
		t.Errorf("Algo = %q, want %q", g.Algo, "rta")
	}
	if len(g.Nodes) == 0 {
		t.Error("Nodes is empty, want at least one node")
	}
	if len(g.Obligations) != 1 {
		t.Fatalf("Obligations = %d entries, want 1", len(g.Obligations))
	}
	ob := g.Obligations[0]
	if ob.Rule != "audit-before-publish" {
		t.Errorf("Obligations[0].Rule = %q, want %q", ob.Rule, "audit-before-publish")
	}
	if ob.Status != ObligationSatisfied {
		t.Errorf("Obligations[0].Status = %q, want %q", ob.Status, ObligationSatisfied)
	}
	if g.Frontier == nil {
		t.Error("Frontier is nil, want non-nil for an obligation-bearing graph")
	}
}

// TestDecodeGraph_UnknownField proves strict decode fails closed on an
// unmodeled top-level field, using the canned twin (real capture + one
// injected field) rather than a hand-authored one.
func TestDecodeGraph_UnknownField(t *testing.T) {
	_, err := DecodeGraph(readCanned(t, "graph-unknown-field.json"))
	if err == nil {
		t.Fatal("DecodeGraph(unknown-field twin): want error, got nil")
	}
}

// TestDecodeGraph_Negative covers malformed and empty input, plus every
// obligation status enum value (all four, per PLAN.md §3), including the
// UNMATCHED case's hard requirement that verdi's join logic (internal/bundle)
// treats it as a loud error rather than silence — this test only proves the
// decoder itself accepts the enum; internal/bundle's tests prove the error.
func TestDecodeGraph_Negative(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{"empty", []byte("")},
		{"not json", []byte("not json at all")},
		{"trailing data", []byte(`{"stamp":"a"}{"stamp":"b"}`)},
		{"bad obligation status", []byte(`{"obligations":[{"rule":"r","status":"BOGUS"}]}`)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := DecodeGraph(tc.data); err == nil {
				t.Fatalf("DecodeGraph(%s): want error, got nil", tc.name)
			}
		})
	}
}

// TestDecodeGraph_AllObligationStatuses proves every one of the four
// enum values spike S1 observed decodes cleanly (SATISFIED and UNMATCHED
// via testdata/svcfix's own real captures; VIOLATED and CANT-PROVE via
// literal JSON matching the exact shape spike S1 captured from obligsvc
// and svcfix branch-state edits — see testdata/svcfix-canned/README.md).
func TestDecodeGraph_AllObligationStatuses(t *testing.T) {
	cases := []struct {
		name   string
		json   string
		status ObligationStatus
	}{
		{
			name:   "satisfied",
			json:   `{"obligations":[{"rule":"audit-before-publish","kind":"must-precede","fn":"(*example.com/svcfix/internal/app.Service).PublishRefund","site":"internal/app/app.go:36","status":"SATISFIED"}]}`,
			status: ObligationSatisfied,
		},
		{
			name:   "violated",
			json:   `{"obligations":[{"rule":"audit-before-publish","kind":"must-precede","fn":"(*example.com/svcfix/internal/app.Service).PublishRefund","site":"internal/app/app.go:33","status":"VIOLATED","detail":"no call to example.com/svcfix/internal/audit#Write dominates this call to example.com/svcfix/internal/bus#Publish"}]}`,
			status: ObligationViolated,
		},
		{
			name:   "cant-prove",
			json:   `{"obligations":[{"rule":"tx-must-close","kind":"must-release","fn":"example.com/obligsvc/internal/app.TransferOwn","site":"internal/app/app.go:49","status":"CANT-PROVE","detail":"acquired value is returned — its lifecycle leaves the function"}]}`,
			status: ObligationCantProve,
		},
		{
			name:   "unmatched",
			json:   `{"obligations":[{"rule":"audit-before-publish","kind":"must-precede","status":"UNMATCHED","detail":"anchor example.com/svcfix/internal/bus#Publish matches no call site — the rule is inert"}]}`,
			status: ObligationUnmatched,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g, err := DecodeGraph([]byte(tc.json))
			if err != nil {
				t.Fatalf("DecodeGraph: %v", err)
			}
			if len(g.Obligations) != 1 || g.Obligations[0].Status != tc.status {
				t.Fatalf("decoded status = %+v, want %q", g.Obligations, tc.status)
			}
		})
	}
}
