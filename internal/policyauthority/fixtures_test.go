package policyauthority

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var updateGolden = flag.Bool("update", false, "regenerate testdata/golden-effective-policy.json")

// TestFixtureEffectivePolicy_Ratchet proves the committed fixture store
// under testdata/store/ decodes clean, resolves clean, and produces a
// byte-stable effective-policy digest (mirroring internal/policyartifact/
// fixtures_test.go's own digest-ratchet discipline): any change to
// fixture content, narrowing behavior, normalization, or canonical
// encoding shows up as an explicit golden diff, never silent drift.
func TestFixtureEffectivePolicy_Ratchet(t *testing.T) {
	root := filepath.Join("testdata", "store")

	s, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	ep, err := Resolve(s)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	digest, err := ep.Digest()
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}

	// Repeated Load+Resolve over the same fixture is byte-identical
	// (determinism proof): a fresh Load/Resolve pair must reproduce the
	// exact same digest, never a value that merely looks equal once.
	s2, err := Load(root)
	if err != nil {
		t.Fatalf("second Load: %v", err)
	}
	ep2, err := Resolve(s2)
	if err != nil {
		t.Fatalf("second Resolve: %v", err)
	}
	digest2, err := ep2.Digest()
	if err != nil {
		t.Fatalf("second Digest: %v", err)
	}
	if digest != digest2 {
		t.Fatalf("repeated Load+Resolve produced different digests: %s vs %s", digest, digest2)
	}

	got := map[string]string{"effective-policy": digest}

	goldenPath := filepath.Join("testdata", "golden-effective-policy.json")
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
	if want["effective-policy"] != got["effective-policy"] {
		t.Errorf("fixture effective-policy digest = %s, golden %s", got["effective-policy"], want["effective-policy"])
	}
}
