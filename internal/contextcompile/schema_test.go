package contextcompile

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/jyang234/verdi/internal/execworkspace"
)

// --- fixture / mutation helpers shared by Request/Manifest/DataItem tests --

func mustReadFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	return data
}

// withTopLevelField returns a copy of data with key set to the given raw
// JSON value (a full JSON literal, e.g. `"bogus"` or `null` or `{"a":1}`).
// Used to construct explicit-null, unknown-enum, and structurally-invalid
// top-level field variants precisely (no substring surgery).
func withTopLevelField(t *testing.T, data []byte, key, rawValue string) []byte {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("test setup: unmarshal fixture: %v", err)
	}
	m[key] = json.RawMessage(rawValue)
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("test setup: remarshal fixture: %v", err)
	}
	return append(out, '\n')
}

// withoutTopLevelField returns a copy of data with key removed entirely
// (simulating an absent field, distinct from an explicit null).
func withoutTopLevelField(t *testing.T, data []byte, key string) []byte {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("test setup: unmarshal fixture: %v", err)
	}
	delete(m, key)
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("test setup: remarshal fixture: %v", err)
	}
	return append(out, '\n')
}

// reorderedNoncanonically returns data's top-level object re-emitted with
// keys in descending order — always different from canonjson's ascending
// canonical order for a document with 2+ keys, so it is guaranteed
// noncanonical without needing to know the real canonical order in advance.
func reorderedNoncanonically(t *testing.T, data []byte) []byte {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("test setup: unmarshal fixture: %v", err)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(keys)))
	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		kb, err := json.Marshal(k)
		if err != nil {
			t.Fatalf("test setup: marshal key: %v", err)
		}
		b.Write(kb)
		b.WriteByte(':')
		b.Write(m[k])
	}
	b.WriteByte('}')
	b.WriteByte('\n')
	return b.Bytes()
}

func withExtraWhitespace(data []byte) []byte {
	return bytes.Replace(data, []byte(":"), []byte(": "), 1)
}

func withoutTrailingNewline(data []byte) []byte {
	return bytes.TrimSuffix(data, []byte("\n"))
}

func withTrailingData(data []byte) []byte {
	out := append([]byte{}, data...)
	return append(out, []byte(`"x"`)...)
}

// withDuplicateKey duplicates the first occurrence of the exact `"key":`
// substring's enclosing key:value pair by simply repeating the whole
// document's schema key:value pair (present exactly once in every
// fixture), producing syntactically valid JSON with a genuine duplicate
// top-level key — something a map-based round-trip cannot express, since
// Go maps cannot hold two entries with the same key.
func withDuplicateSchemaKey(data []byte, schemaKV string) []byte {
	return bytes.Replace(data, []byte(schemaKV), []byte(schemaKV+","+schemaKV), 1)
}

// withInvalidUTF8 flips one byte inside marker (which must occur exactly
// once in data) to an invalid UTF-8 lead byte.
func withInvalidUTF8(t *testing.T, data []byte, marker string) []byte {
	t.Helper()
	idx := bytes.Index(data, []byte(marker))
	if idx < 0 {
		t.Fatalf("test setup: marker %q not found in fixture", marker)
	}
	out := append([]byte{}, data...)
	out[idx] = 0xff
	return out
}

// === Request (authority design §3) =========================================

func TestRequest_DecodeGoldenRoundTrip(t *testing.T) {
	golden := mustReadFixture(t, "request-build.json")

	req, err := DecodeRequest(golden)
	if err != nil {
		t.Fatalf("DecodeRequest: unexpected error: %v", err)
	}
	if req.Schema != RequestSchema {
		t.Errorf("Schema = %q, want %q", req.Schema, RequestSchema)
	}
	if req.Phase != PhaseBuild {
		t.Errorf("Phase = %q, want %q", req.Phase, PhaseBuild)
	}
	if req.Adapter.ID != "codex" || req.Adapter.Version != "1" {
		t.Errorf("Adapter = %+v, want {codex 1}", req.Adapter)
	}
	if req.Expected == nil || req.Expected.Branch != "main" {
		t.Errorf("Expected = %+v, want branch main", req.Expected)
	}
	if req.Spec != "spec/example-story" {
		t.Errorf("Spec = %q, want spec/example-story", req.Spec)
	}

	again, err := EncodeRequest(req)
	if err != nil {
		t.Fatalf("EncodeRequest: unexpected error: %v", err)
	}
	if !bytes.Equal(again, golden) {
		t.Errorf("EncodeRequest(DecodeRequest(golden)) != golden\ngot:  %s\nwant: %s", again, golden)
	}
}

func TestRequest_DecodeRejectsMalformed(t *testing.T) {
	golden := mustReadFixture(t, "request-build.json")
	const schemaKV = `"schema":"` + RequestSchema + `"`
	if !bytes.Contains(golden, []byte(schemaKV)) {
		t.Fatalf("test setup: fixture does not contain expected schema key:value %q", schemaKV)
	}

	cases := map[string][]byte{
		"unknown field":            withTopLevelField(t, golden, "bogus", "true"),
		"duplicate key":            withDuplicateSchemaKey(golden, schemaKV),
		"trailing data":            withTrailingData(golden),
		"invalid utf8":             withInvalidUTF8(t, golden, "example-story"),
		"noncanonical whitespace":  withExtraWhitespace(golden),
		"noncanonical key order":   reorderedNoncanonically(t, golden),
		"missing trailing newline": withoutTrailingNewline(golden),
		"explicit null schema":     withTopLevelField(t, golden, "schema", "null"),
		"absent schema":            withoutTopLevelField(t, golden, "schema"),
		"unknown phase enum":       withTopLevelField(t, golden, "phase", `"bogus-phase"`),
		"invalid spec ref grammar": withTopLevelField(t, golden, "spec", `"not-a-spec-ref"`),
		"unknown grant kind": withTopLevelField(t, golden, "grants",
			`{"grants":[{"kind":"filesystem"}],"schema":"verdi.execution-grants/v1"}`),
		"expected missing head":   withTopLevelField(t, golden, "expected", `{"branch":"main"}`),
		"expected explicit null":  withTopLevelField(t, golden, "expected", "null"),
		"scope duplicate path":    withTopLevelField(t, golden, "scope", `{"environments":[],"paths":["a","a"],"phases":["build"],"refs":[]}`),
		"scope unsorted paths":    withTopLevelField(t, golden, "scope", `{"environments":[],"paths":["b","a"],"phases":["build"],"refs":[]}`),
		"scope missing dimension": withTopLevelField(t, golden, "scope", `{"environments":[],"paths":[],"refs":[]}`),
	}

	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeRequest(data); err == nil {
				t.Fatalf("DecodeRequest(%s): want error, got nil", name)
			}
		})
	}
}

func TestRequest_PhaseScopeRefusalIsTypedNotMessageMatched(t *testing.T) {
	golden := mustReadFixture(t, "request-build.json")

	// phase build, scope.phases [build]: applicable, no refusal.
	if _, err := DecodeRequest(golden); err != nil {
		t.Fatalf("golden request: unexpected error: %v", err)
	}

	// phase design, scope.phases [build]: outside declared scope, refused.
	mismatched := withTopLevelField(t, golden, "phase", `"design"`)
	_, err := DecodeRequest(mismatched)
	if err == nil {
		t.Fatalf("phase outside scope: want *PhaseScopeRefusal, got nil error")
	}
	var refusal *PhaseScopeRefusal
	if !errors.As(err, &refusal) {
		t.Fatalf("phase outside scope: want errors.As match for *PhaseScopeRefusal, got %T: %v", err, err)
	}
	if refusal.Phase != PhaseDesign {
		t.Errorf("refusal.Phase = %q, want design", refusal.Phase)
	}

	// phase design, scope.phases []: universal, no refusal.
	universal := withTopLevelField(t, golden, "phase", `"design"`)
	universal = withTopLevelField(t, universal, "scope", `{"environments":[],"paths":[],"phases":[],"refs":[]}`)
	if _, err := DecodeRequest(universal); err != nil {
		t.Fatalf("universal scope.phases: unexpected error: %v", err)
	}
}

func TestRequest_EncodeRejectsCallerSuppliedGrantSemantics(t *testing.T) {
	golden := mustReadFixture(t, "request-build.json")
	req, err := DecodeRequest(golden)
	if err != nil {
		t.Fatalf("DecodeRequest: %v", err)
	}
	// A GrantSet with a duplicate kind is invalid per the one grant seam
	// (execworkspace.GrantSet.Validate); EncodeRequest must not silently
	// accept it.
	req.Grants.Grants = append(req.Grants.Grants,
		grantNetwork(), grantNetwork(),
	)
	if _, err := EncodeRequest(req); err == nil {
		t.Fatalf("EncodeRequest: want error for duplicate grant kind, got nil")
	}
}

// === Manifest (authority design §8.2) =======================================

func decodeGoldenManifest(t *testing.T) Manifest {
	t.Helper()
	m, err := DecodeManifest(mustReadFixture(t, "manifest-build.json"))
	if err != nil {
		t.Fatalf("DecodeManifest(golden): unexpected error: %v", err)
	}
	return m
}

func TestManifest_DecodeGoldenRoundTrip(t *testing.T) {
	golden := mustReadFixture(t, "manifest-build.json")
	m := decodeGoldenManifest(t)

	if m.Schema != ManifestSchema {
		t.Errorf("Schema = %q, want %q", m.Schema, ManifestSchema)
	}
	if m.Phase != PhaseBuild {
		t.Errorf("Phase = %q, want build", m.Phase)
	}
	if m.Revisions.Context != 1 {
		t.Errorf("Revisions.Context = %d, want 1", m.Revisions.Context)
	}
	if len(m.RequiredInputs) != 0 {
		t.Errorf("RequiredInputs = %v, want [] for a build-phase manifest", m.RequiredInputs)
	}

	again, err := EncodeManifest(m)
	if err != nil {
		t.Fatalf("EncodeManifest: unexpected error: %v", err)
	}
	if !bytes.Equal(again, golden) {
		t.Errorf("EncodeManifest(DecodeManifest(golden)) != golden\ngot:  %s\nwant: %s", again, golden)
	}
}

func TestManifest_DecodeRejectsMalformed(t *testing.T) {
	golden := mustReadFixture(t, "manifest-build.json")
	const schemaKV = `"schema":"` + ManifestSchema + `"`
	if !bytes.Contains(golden, []byte(schemaKV)) {
		t.Fatalf("test setup: fixture does not contain expected schema key:value %q", schemaKV)
	}

	cases := map[string][]byte{
		"unknown field":            withTopLevelField(t, golden, "bogus", "true"),
		"duplicate key":            withDuplicateSchemaKey(golden, schemaKV),
		"trailing data":            withTrailingData(golden),
		"noncanonical whitespace":  withExtraWhitespace(golden),
		"noncanonical key order":   reorderedNoncanonically(t, golden),
		"missing trailing newline": withoutTrailingNewline(golden),
		"explicit null schema":     withTopLevelField(t, golden, "schema", "null"),
		"absent schema":            withoutTopLevelField(t, golden, "schema"),
		"unknown phase enum":       withTopLevelField(t, golden, "phase", `"bogus-phase"`),
		"invalid top-level digest": withTopLevelField(t, golden, "digest", `"not-a-digest"`),
		"nonempty dispositions":    withTopLevelField(t, golden, "dispositions", `[{}]`),
		"nonempty expansions":      withTopLevelField(t, golden, "expansions", `[{}]`),
		"unknown grant kind in capabilities": withTopLevelField(t, golden, "capabilities",
			`{"grants":[{"kind":"filesystem"}],"schema":"verdi.execution-grants/v1"}`),
	}

	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeManifest(data); err == nil {
				t.Fatalf("DecodeManifest(%s): want error, got nil", name)
			}
		})
	}
}

func TestManifest_DecodeRejectsNonemptyRevisionsParent(t *testing.T) {
	golden := mustReadFixture(t, "manifest-build.json")
	var m map[string]json.RawMessage
	if err := json.Unmarshal(golden, &m); err != nil {
		t.Fatalf("test setup: %v", err)
	}
	var revisions map[string]json.RawMessage
	if err := json.Unmarshal(m["revisions"], &revisions); err != nil {
		t.Fatalf("test setup: %v", err)
	}
	revisions["parent"] = json.RawMessage("1")
	revisionsBytes, err := json.Marshal(revisions)
	if err != nil {
		t.Fatalf("test setup: %v", err)
	}
	mutated := withTopLevelField(t, golden, "revisions", string(revisionsBytes))

	if _, err := DecodeManifest(mutated); err == nil {
		t.Fatalf("DecodeManifest: want error for nonempty revisions.parent, got nil")
	}
}

func TestManifest_EncodeRejectsDuplicateIncludedIdentity(t *testing.T) {
	m := decodeGoldenManifest(t)
	if len(m.Included) == 0 {
		t.Fatalf("test setup: golden manifest has no included rows to duplicate")
	}
	m.Included = append(m.Included, m.Included[0])
	if _, err := EncodeManifest(m); err == nil {
		t.Fatalf("EncodeManifest: want error for duplicate included identity, got nil")
	}
}

func TestManifest_EncodeRejectsUnsortedParentFeatures(t *testing.T) {
	m := decodeGoldenManifest(t)
	m.ParentFeatures = append(m.ParentFeatures, ParentFeature{
		Ref:            "spec/aaa-earlier-feature",
		Path:           "specs/active/aaa-earlier-feature/spec.md",
		SourceDigest:   fixedDigest,
		FragmentDigest: fixedDigest,
		PayloadDigest:  fixedDigest,
	})
	// The appended row sorts before the existing row(s), so the slice is
	// now out of order without introducing a duplicate.
	if _, err := EncodeManifest(m); err == nil {
		t.Fatalf("EncodeManifest: want error for unsorted parent_features, got nil")
	}
}

func TestManifest_EncodeIgnoresCallerSuppliedDigest(t *testing.T) {
	m := decodeGoldenManifest(t)
	m.Digest = "sha256:" + hex64('a')
	out, err := EncodeManifest(m)
	if err != nil {
		t.Fatalf("EncodeManifest: unexpected error: %v", err)
	}
	if bytes.Contains(out, []byte(`"digest":"sha256:`+hex64('a')+`"`)) {
		t.Fatalf("EncodeManifest embedded the caller-supplied digest instead of recomputing it")
	}
	again, err := DecodeManifest(out)
	if err != nil {
		t.Fatalf("DecodeManifest(re-encoded): unexpected error: %v", err)
	}
	if again.Digest == m.Digest {
		t.Fatalf("recomputed digest unexpectedly equals the discarded caller-supplied one")
	}
}

// === DataItem (authority design §8.1) =======================================

func decodeGoldenDataItem(t *testing.T) DataItem {
	t.Helper()
	item, err := DecodeDataItem(mustReadFixture(t, "data-item.json"))
	if err != nil {
		t.Fatalf("DecodeDataItem(golden): unexpected error: %v", err)
	}
	return item
}

func TestDataItem_DecodeGoldenRoundTrip(t *testing.T) {
	golden := mustReadFixture(t, "data-item.json")
	item := decodeGoldenDataItem(t)

	if item.Schema != DataItemSchema {
		t.Errorf("Schema = %q, want %q", item.Schema, DataItemSchema)
	}
	if item.Classification != DataItemClassification {
		t.Errorf("Classification = %q, want %q", item.Classification, DataItemClassification)
	}
	if item.Source != SourceHeadTree {
		t.Errorf("Source = %q, want head-tree", item.Source)
	}
	if item.Kind != IncludedRepositoryFile {
		t.Errorf("Kind = %q, want repository-file", item.Kind)
	}

	again, err := EncodeDataItem(item)
	if err != nil {
		t.Fatalf("EncodeDataItem: unexpected error: %v", err)
	}
	if !bytes.Equal(again, golden) {
		t.Errorf("EncodeDataItem(DecodeDataItem(golden)) != golden\ngot:  %s\nwant: %s", again, golden)
	}
}

func TestDataItem_DecodeRejectsMalformed(t *testing.T) {
	golden := mustReadFixture(t, "data-item.json")
	const schemaKV = `"schema":"` + DataItemSchema + `"`
	if !bytes.Contains(golden, []byte(schemaKV)) {
		t.Fatalf("test setup: fixture does not contain expected schema key:value %q", schemaKV)
	}

	cases := map[string][]byte{
		"unknown field":            withTopLevelField(t, golden, "bogus", "true"),
		"duplicate key":            withDuplicateSchemaKey(golden, schemaKV),
		"trailing data":            withTrailingData(golden),
		"noncanonical whitespace":  withExtraWhitespace(golden),
		"noncanonical key order":   reorderedNoncanonically(t, golden),
		"missing trailing newline": withoutTrailingNewline(golden),
		"explicit null schema":     withTopLevelField(t, golden, "schema", "null"),
		"absent schema":            withoutTopLevelField(t, golden, "schema"),
		"unknown source enum":      withTopLevelField(t, golden, "source", `"bogus-source"`),
		"unknown kind enum":        withTopLevelField(t, golden, "kind", `"bogus-kind"`),
		"invalid content_digest":   withTopLevelField(t, golden, "content_digest", `"sha256:not-hex"`),
		"invalid self digest":      withTopLevelField(t, golden, "digest", `"sha256:not-hex"`),
		"wrong classification":     withTopLevelField(t, golden, "classification", `"authoritative"`),
		"content_digest mismatch": withTopLevelField(t, golden, "content_digest",
			`"sha256:`+hex64('0')+`"`),
	}

	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeDataItem(data); err == nil {
				t.Fatalf("DecodeDataItem(%s): want error, got nil", name)
			}
		})
	}
}

func TestDataItem_EncodeRejectsInstructionProjectionKind(t *testing.T) {
	item := decodeGoldenDataItem(t)
	item.Kind = IncludedInstructionProjection
	if _, err := EncodeDataItem(item); err == nil {
		t.Fatalf("EncodeDataItem: want error for kind=instruction-projection, got nil")
	}
}

func TestDataItem_EncodeIgnoresCallerSuppliedDigest(t *testing.T) {
	item := decodeGoldenDataItem(t)
	item.Digest = "sha256:" + hex64('a')
	out, err := EncodeDataItem(item)
	if err != nil {
		t.Fatalf("EncodeDataItem: unexpected error: %v", err)
	}
	if bytes.Contains(out, []byte(`"digest":"sha256:`+hex64('a')+`"`)) {
		t.Fatalf("EncodeDataItem embedded the caller-supplied digest instead of recomputing it")
	}
}

// --- small local test fixtures ---------------------------------------------

// hex64 returns 64 copies of r, a cheap way to build a distinct
// grammatically valid 64-hex-character digest tail for negative tests
// without depending on a real digest computation.
func hex64(r byte) string {
	out := make([]byte, 64)
	for i := range out {
		out[i] = r
	}
	return string(out)
}

// fixedDigest is a fixed, grammatically valid placeholder digest used by
// tests that need *a* valid digest string but do not care about its value.
var fixedDigest = "sha256:" + hex64('1')

func grantNetwork() execworkspace.Grant { return execworkspace.Grant{Kind: execworkspace.GrantNetwork} }
