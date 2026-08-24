package experiment

import (
	"os"
	"path/filepath"
	"testing"
)

type definitionFixtureDigest struct {
	Bytes      string `json:"bytes"`
	Definition string `json:"definition"`
}

func TestDefinitionV2FixtureDigestRatchet(t *testing.T) {
	directory := "testdata"
	ratchetBytes, err := os.ReadFile(filepath.Join(directory, "definition-digests.json"))
	if err != nil {
		t.Fatalf("ReadFile(definition-digests.json): %v", err)
	}
	var ratchets map[string]definitionFixtureDigest
	if err := decodeStrictJSON(ratchetBytes, &ratchets); err != nil {
		t.Fatalf("decode definition digest ratchets: %v", err)
	}
	if err := requireCanonicalJSON(ratchetBytes, ratchets); err != nil {
		t.Fatalf("definition digest ratchets are not canonical: %v", err)
	}

	tests := []struct {
		name       string
		wantSchema string
		wantClass  string
	}{
		{name: "definition-v1.yaml", wantSchema: DefinitionSchemaV1},
		{name: "definition-v2.yaml", wantSchema: DefinitionSchemaV2, wantClass: "request-path-performance"},
	}
	if len(ratchets) != len(tests) {
		t.Fatalf("definition digest ratchet count = %d, want %d", len(ratchets), len(tests))
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(directory, tt.name))
			if err != nil {
				t.Fatalf("ReadFile(%s): %v", tt.name, err)
			}
			def, err := DecodeDefinition(raw)
			if err != nil {
				t.Fatalf("DecodeDefinition(%s): %v", tt.name, err)
			}
			if def.Schema != tt.wantSchema || def.Class != tt.wantClass {
				t.Fatalf("fixture schema/class = %q/%q, want %q/%q", def.Schema, def.Class, tt.wantSchema, tt.wantClass)
			}
			definitionDigest, err := DefinitionDigest(def)
			if err != nil {
				t.Fatalf("DefinitionDigest(%s): %v", tt.name, err)
			}
			want, ok := ratchets[tt.name]
			if !ok {
				t.Fatalf("missing digest ratchet for %s", tt.name)
			}
			if got := sha256Digest(raw); got != want.Bytes {
				t.Errorf("raw bytes digest = %q, want %q", got, want.Bytes)
			}
			if definitionDigest != want.Definition {
				t.Errorf("definition digest = %q, want %q", definitionDigest, want.Definition)
			}
		})
	}
}
