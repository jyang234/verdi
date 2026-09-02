package constitutionimpact

import (
	"context"
	"io/fs"
	"os"
	"testing"
	"testing/fstest"

	"github.com/jyang234/verdi/internal/policyconflict"
)

const (
	testAcceptedCommit = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	testAcceptedTree   = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	testProposedCommit = "0123456789abcdef0123456789abcdef01234567"
	testProposedTree   = "cccccccccccccccccccccccccccccccccccccccc"
)

func testConstitutionBytes(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile("../policyartifact/testdata/store/constitution.md")
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func testTree(inventory []byte, constitution []byte) fs.FS {
	tree := fstest.MapFS{}
	if inventory != nil {
		tree[InventoryPath] = &fstest.MapFile{Data: append([]byte(nil), inventory...), Mode: 0o644}
	}
	if constitution != nil {
		tree[constitutionPath] = &fstest.MapFile{Data: append([]byte(nil), constitution...), Mode: 0o644}
	}
	return tree
}

func testInventoryBytes(t *testing.T, consumers ...Consumer) []byte {
	t.Helper()
	bytes, err := EncodeInventory(Inventory{Schema: InventorySchema, Consumers: consumers})
	if err != nil {
		t.Fatal(err)
	}
	return bytes
}

func testChangedLayer() []LayerChange {
	return []LayerChange{{
		Kind: "policy", ID: "go-toolchain", Change: "changed",
		AcceptedDigest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ProposedDigest: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}}
}

func testPlan(t *testing.T, acceptedConsumers, proposedConsumers []Consumer, layers []LayerChange) Plan {
	t.Helper()
	constitution := testConstitutionBytes(t)
	plan, err := BuildPlan(context.Background(),
		ExactTree{Commit: testAcceptedCommit, Tree: testAcceptedTree, FS: testTree(testInventoryBytes(t, acceptedConsumers...), constitution)},
		ExactTree{Commit: testProposedCommit, Tree: testProposedTree, FS: testTree(testInventoryBytes(t, proposedConsumers...), constitution)},
		layers,
	)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func resultForPlan(t *testing.T, plan Plan, pass bool) *policyconflict.Result {
	t.Helper()
	raw, err := os.ReadFile("../policyconflict/testdata/report.json")
	if err != nil {
		t.Fatal(err)
	}
	report, err := policyconflict.DecodeReport(raw)
	if err != nil {
		t.Fatal(err)
	}
	report.Input.Repository.Head.Known = true
	report.Input.Repository.Head.Value = plan.proposed.Commit
	report.Input.ConstitutionDigest = plan.proposed.ConstitutionDigest
	if pass {
		report.Semantic = []policyconflict.SemanticEvaluation{}
		report.Verdict = policyconflict.VerdictPass
	}
	report.Digest = ""
	encoded, err := policyconflict.EncodeReport(report)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := policyconflict.DecodeReport(encoded)
	if err != nil {
		t.Fatal(err)
	}
	return &policyconflict.Result{Report: canonical, ReportBytes: encoded}
}
