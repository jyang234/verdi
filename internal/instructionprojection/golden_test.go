package instructionprojection

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var updateGolden = flag.Bool("update", false, "regenerate testdata/golden-projection.json")

// TestFixtureProjection_Ratchet proves the committed fixture under
// testdata/store/ resolves clean and produces byte-stable generated
// content and manifest digests (mirroring internal/policyauthority's own
// TestFixtureEffectivePolicy_Ratchet discipline): any change to fixture
// content, rendering, normalization, or canonical encoding shows up as
// an explicit golden diff, never silent drift. This reads testdata/
// store/ directly (never writes into it) and never calls Generate, so it
// never touches .verdi/policy/projections/ and is unaffected by the
// confirmed policyauthority walker conflict.
func TestFixtureProjection_Ratchet(t *testing.T) {
	root := filepath.Join("testdata", "store")

	store, ep := loadResolve(t, root)
	in, err := buildProjectionInput(store.Policies, ep)
	if err != nil {
		t.Fatalf("buildProjectionInput: %v", err)
	}
	if len(store.Constitution.Adapters) != 1 {
		t.Fatalf("fixture adapters = %d, want 1", len(store.Constitution.Adapters))
	}
	adapter := store.Constitution.Adapters[0]

	content := renderProjection(adapter, in)
	contentDig := contentDigest(content)
	files := make([]FileDigest, 0, len(adapter.Managed))
	for _, rel := range adapter.Managed {
		files = append(files, FileDigest{Path: rel, Digest: contentDig})
	}
	m := buildManifest(adapter, in, files)
	mBytes, err := manifestBytes(m)
	if err != nil {
		t.Fatalf("manifestBytes: %v", err)
	}

	// Determinism: repeating the same Load+Resolve+render pipeline over
	// the same fixture reproduces byte-identical content, never a value
	// that merely looks equal once.
	store2, ep2 := loadResolve(t, root)
	in2, err := buildProjectionInput(store2.Policies, ep2)
	if err != nil {
		t.Fatalf("second buildProjectionInput: %v", err)
	}
	content2 := renderProjection(store2.Constitution.Adapters[0], in2)
	if string(content) != string(content2) {
		t.Fatalf("repeated rendering of the same fixture produced different bytes")
	}

	got := map[string]string{
		"content":  contentDigest(content),
		"manifest": contentDigest(mBytes),
	}

	goldenPath := filepath.Join("testdata", "golden-projection.json")
	if *updateGolden {
		data, err := json.MarshalIndent(got, "", "  ")
		if err != nil {
			t.Fatalf("marshaling golden: %v", err)
		}
		if err := os.WriteFile(goldenPath, append(data, '\n'), 0o644); err != nil {
			t.Fatalf("writing golden: %v", err)
		}
	}

	goldenData, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("reading golden (run with -update to regenerate): %v", err)
	}
	var want map[string]string
	if err := json.Unmarshal(goldenData, &want); err != nil {
		t.Fatalf("decoding golden: %v", err)
	}
	if want["content"] != got["content"] {
		t.Errorf("fixture projection content digest = %s, golden %s", got["content"], want["content"])
	}
	if want["manifest"] != got["manifest"] {
		t.Errorf("fixture projection manifest digest = %s, golden %s", got["manifest"], want["manifest"])
	}

	// No wall-clock/random content: exact golden equality on the raw
	// bytes themselves (not merely their digest) proves the content
	// never carries hidden entropy that happened to digest the same
	// twice by coincidence.
	contentGoldenPath := filepath.Join("testdata", "golden-agents.md")
	if *updateGolden {
		if err := os.WriteFile(contentGoldenPath, content, 0o644); err != nil {
			t.Fatalf("writing content golden: %v", err)
		}
	}
	wantContent, err := os.ReadFile(contentGoldenPath)
	if err != nil {
		t.Fatalf("reading content golden (run with -update to regenerate): %v", err)
	}
	if string(wantContent) != string(content) {
		t.Errorf("fixture projection content bytes changed:\n--- golden ---\n%s\n--- got ---\n%s", wantContent, content)
	}
}

// TestFixtureProjection_SilentPolicyProjectsNoSection proves the
// contracted rule directly: policy/silent (zero instructions) never
// contributes a section to the rendered content.
func TestFixtureProjection_SilentPolicyProjectsNoSection(t *testing.T) {
	root := filepath.Join("testdata", "store")
	store, ep := loadResolve(t, root)
	if _, ok := store.Policies["policy/silent"]; !ok {
		t.Fatal("fixture must carry policy/silent for this test to be meaningful")
	}
	in, err := buildProjectionInput(store.Policies, ep)
	if err != nil {
		t.Fatalf("buildProjectionInput: %v", err)
	}
	for _, p := range in.Policies {
		if p.ID == "policy/silent" {
			t.Fatalf("policy/silent has zero instructions and must not appear in the rendered policy set: %+v", in.Policies)
		}
	}
	content := renderProjection(store.Constitution.Adapters[0], in)
	if bytes.Contains(content, []byte("Silent policy")) {
		t.Fatalf("rendered content must not mention policy/silent's title at all:\n%s", content)
	}
}
