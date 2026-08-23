package experiment

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
)

func validCapsuleManifestJSON() string {
	return `{
  "schema": "verdi.experiment-capsule/v1",
  "experiment": "cache-placement-v1",
  "definition_digest": "` + digestOf("a") + `",
  "result_digest": "` + digestOf("b") + `",
  "selected": "facts-cache",
  "artifacts": [
    {"id": "candidate-patch", "digest": "` + digestOf("c") + `"},
    {"id": "observations", "digest": "` + digestOf("d") + `"}
  ]
}`
}

func TestCapsuleProtocolCanonicalDigestRatchet(t *testing.T) {
	manifest, err := DecodeCapsuleManifest([]byte(validCapsuleManifestJSON()))
	if err != nil {
		t.Fatal(err)
	}
	digest, err := canonjson.Digest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	const want = "sha256:8588e115ebfe5756223d59c3188daf00f10d43f1654e9751bf66a6b9f72806fd"
	if digest != want {
		t.Fatalf("capsule manifest canonical digest = %q, want %q", digest, want)
	}
}

func mutateCapsule(t *testing.T, old, replacement string) string {
	t.Helper()
	doc := validCapsuleManifestJSON()
	if !strings.Contains(doc, old) {
		t.Fatalf("fixture does not contain %q", old)
	}
	return strings.Replace(doc, old, replacement, 1)
}

func TestDecodeCapsuleManifestHappyPath(t *testing.T) {
	m, err := DecodeCapsuleManifest([]byte(validCapsuleManifestJSON()))
	if err != nil {
		t.Fatalf("DecodeCapsuleManifest() unexpected error: %v", err)
	}
	if m.Selected != "facts-cache" {
		t.Errorf("m.Selected = %q, want facts-cache", m.Selected)
	}
	if len(m.Artifacts) != 2 {
		t.Fatalf("len(m.Artifacts) = %d, want 2", len(m.Artifacts))
	}
}

func TestDecodeCapsuleManifestRejects(t *testing.T) {
	tests := []struct {
		name string
		doc  string
	}{
		{"unknown schema", mutateCapsule(t, `"schema": "verdi.experiment-capsule/v1"`, `"schema": "verdi.experiment-capsule/v2"`)},
		{"unknown field", strings.TrimSuffix(validCapsuleManifestJSON(), "}") + `,"unknown_field": true}`},
		{"trailing data", validCapsuleManifestJSON() + "\n{}"},
		{"bad experiment id", mutateCapsule(t, `"experiment": "cache-placement-v1"`, `"experiment": "Cache_Placement"`)},
		{"bad definition digest", mutateCapsule(t, `"definition_digest": "`+digestOf("a")+`"`, `"definition_digest": "not-a-digest"`)},
		{"bad result digest", mutateCapsule(t, `"result_digest": "`+digestOf("b")+`"`, `"result_digest": "not-a-digest"`)},
		{"bad selected id", mutateCapsule(t, `"selected": "facts-cache"`, `"selected": "Facts_Cache"`)},
		{"empty artifacts", mutateCapsule(t,
			`"artifacts": [
    {"id": "candidate-patch", "digest": "`+digestOf("c")+`"},
    {"id": "observations", "digest": "`+digestOf("d")+`"}
  ]`,
			`"artifacts": []`)},
		{"duplicate artifact ids", mutateCapsule(t, `{"id": "observations", "digest": "`+digestOf("d")+`"}`, `{"id": "candidate-patch", "digest": "`+digestOf("d")+`"}`)},
		{"bad artifact digest", mutateCapsule(t, `"digest": "`+digestOf("c")+`"`, `"digest": "not-a-digest"`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := DecodeCapsuleManifest([]byte(tt.doc)); err == nil {
				t.Errorf("DecodeCapsuleManifest(%s) = nil error, want error", tt.name)
			}
		})
	}
}
