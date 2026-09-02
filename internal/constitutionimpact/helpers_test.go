package constitutionimpact

import (
	"context"
	"io/fs"
	"os"
	"testing"
	"testing/fstest"

	"github.com/jyang234/verdi/internal/contextcompile"
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

func resultForConsumer(t *testing.T, plan Plan, consumer Consumer, pass bool) (*policyconflict.Result, []byte) {
	t.Helper()
	manifestBytes, err := os.ReadFile("../contextcompile/testdata/manifest-build.json")
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := contextcompile.DecodeManifest(manifestBytes)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../policyconflict/testdata/report.json")
	if err != nil {
		t.Fatal(err)
	}
	report, err := policyconflict.DecodeReport(raw)
	if err != nil {
		t.Fatal(err)
	}
	manifest.Adapter = consumer.Request.Adapter
	manifest.Phase = consumer.Request.Phase
	manifest.AcceptedSpec.Ref = consumer.Request.Spec
	manifest.Repository.RemoteOrigin.Known = report.Input.Repository.RemoteOrigin.Known
	manifest.Repository.RemoteOrigin.Value = report.Input.Repository.RemoteOrigin.Value
	manifest.Repository.Branch.Known = report.Input.Repository.Branch.Known
	manifest.Repository.Branch.Value = report.Input.Repository.Branch.Value
	manifest.Repository.Head.Known = true
	manifest.Repository.Head.Value = plan.proposed.Commit
	manifest.Repository.DefaultBranch.Known = report.Input.Repository.DefaultBranch.Known
	manifest.Repository.DefaultBranch.Name = report.Input.Repository.DefaultBranch.Name
	manifest.Repository.DefaultBranch.Ref = report.Input.Repository.DefaultBranch.Ref
	manifest.Repository.DefaultBranch.Head = report.Input.Repository.DefaultBranch.Head
	manifest.Repository.Relationship = string(report.Input.Repository.Relationship)
	manifest.Repository.Dirty.Known = report.Input.Repository.Dirty.Known
	manifest.Repository.Dirty.Value = report.Input.Repository.Dirty.Value
	manifest.Repository.Staged.Known = report.Input.Repository.Staged.Known
	manifest.Repository.Staged.Value = report.Input.Repository.Staged.Value
	manifest.Repository.Worktree.Managed = report.Input.Repository.Worktree.Managed
	manifest.Repository.Worktree.Name = report.Input.Repository.Worktree.Name
	manifest.Repository.Source = string(report.Input.Repository.Source)
	manifest.Policy.ConstitutionDigest = plan.proposed.ConstitutionDigest
	manifest.Policy.EffectiveDigest = report.Input.EffectivePolicyDigest
	manifest.Policy.ProfileID = report.Input.Profile.ID
	manifest.Policy.ProfileDigest = report.Input.Profile.Digest
	manifest.Policy.Entries = make([]contextcompile.PolicyEntry, len(report.Input.PolicyEntries))
	for i, entry := range report.Input.PolicyEntries {
		manifest.Policy.Entries[i] = contextcompile.PolicyEntry{
			Kind: entry.Kind, ID: entry.ID, Digest: entry.Digest,
			Applicability: contextcompile.ApplicabilityApplicable,
		}
	}
	manifest.GovernanceProfile.ID = report.Input.Profile.ID
	manifest.GovernanceProfile.Digest = report.Input.Profile.Digest
	manifest.Scope = consumer.Request.Scope
	manifest.Capabilities = consumer.Request.Grants
	manifest.Digest = ""
	manifestBytes, err = contextcompile.EncodeManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err = contextcompile.DecodeManifest(manifestBytes)
	if err != nil {
		t.Fatal(err)
	}

	report.Input.Repository.Head.Known = true
	report.Input.Repository.Head.Value = plan.proposed.Commit
	report.Input.ConstitutionDigest = plan.proposed.ConstitutionDigest
	report.Input.Target.Accepted.ManifestDigest = manifest.Digest
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
	return &policyconflict.Result{Report: canonical, ReportBytes: encoded}, manifestBytes
}

func testEvaluation(consumer Consumer, result *policyconflict.Result, manifestBytes []byte) Evaluation {
	identity, _ := consumer.Identity()
	return Evaluation{
		ConsumerIdentity: identity, Consumer: consumer,
		AcceptedManifestBytes: manifestBytes,
		Result:                result,
	}
}

func testSupplemental(consumer Consumer, result *policyconflict.Result, manifestBytes []byte) SupplementalTarget {
	return SupplementalTarget{
		Request: consumer.Request, AcceptedManifestBytes: manifestBytes, Result: result,
	}
}
