package artifact

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSplitFrontmatter_Happy(t *testing.T) {
	doc := []byte("---\nid: spec/foo\ntitle: Foo\n---\n# Body\n\ntext\n")
	fm, body, err := SplitFrontmatter(doc)
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	if string(fm) != "id: spec/foo\ntitle: Foo" {
		t.Fatalf("frontmatter = %q", fm)
	}
	if string(body) != "# Body\n\ntext\n" {
		t.Fatalf("body = %q", body)
	}
}

func TestSplitFrontmatter_Negative(t *testing.T) {
	cases := map[string][]byte{
		"no leading delimiter": []byte("id: spec/foo\n---\nbody\n"),
		"no closing delimiter": []byte("---\nid: spec/foo\nbody with no close\n"),
		"empty document":       []byte(""),
	}
	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, _, err := SplitFrontmatter(doc); err == nil {
				t.Fatalf("SplitFrontmatter(%q): want error, got nil", doc)
			}
		})
	}
}

type decodeTarget struct {
	ID    string   `yaml:"id"`
	Title string   `yaml:"title"`
	Tags  []string `yaml:"tags"`
}

func TestDecodeStrict_Happy(t *testing.T) {
	data := []byte("id: spec/foo\ntitle: Foo\ntags: [a, b]\n")
	var out decodeTarget
	if err := DecodeStrict(data, &out); err != nil {
		t.Fatalf("DecodeStrict: %v", err)
	}
	if out.ID != "spec/foo" || out.Title != "Foo" || len(out.Tags) != 2 {
		t.Fatalf("DecodeStrict decoded = %+v", out)
	}
}

func TestDecodeStrict_UnknownField(t *testing.T) {
	data := []byte("id: spec/foo\ntitle: Foo\nbogus_field: surprise\n")
	var out decodeTarget
	err := DecodeStrict(data, &out)
	if err == nil {
		t.Fatal("DecodeStrict: want error for unknown field, got nil")
	}
	if !strings.Contains(err.Error(), "bogus_field") {
		t.Fatalf("DecodeStrict error = %q, want it to name the unknown field", err)
	}
}

func TestDecodeStrict_TypeMismatch(t *testing.T) {
	data := []byte("id: spec/foo\ntitle: [not, a, scalar]\n")
	var out decodeTarget
	if err := DecodeStrict(data, &out); err == nil {
		t.Fatal("DecodeStrict: want error for type mismatch (title is a sequence, not a scalar), got nil")
	}
}

func TestDecodeStrict_RejectsAnchor(t *testing.T) {
	data := []byte("id: &anchor spec/foo\ntitle: Foo\n")
	var out decodeTarget
	err := DecodeStrict(data, &out)
	if err == nil {
		t.Fatal("DecodeStrict: want dialect error for anchor, got nil")
	}
	if !strings.Contains(err.Error(), "anchor") {
		t.Fatalf("DecodeStrict error = %q, want it to name the anchor offense", err)
	}
}

func TestDecodeStrict_RejectsAlias(t *testing.T) {
	data := []byte("defaults: &d\n  title: Foo\nid: spec/foo\ntitle: Foo\nsame: *d\n")
	var out map[string]interface{}
	err := DecodeStrict(data, &out)
	if err == nil {
		t.Fatal("DecodeStrict: want dialect error for alias, got nil")
	}
	if !strings.Contains(err.Error(), "anchor") && !strings.Contains(err.Error(), "alias") {
		t.Fatalf("DecodeStrict error = %q, want it to name the anchor/alias offense", err)
	}
}

func TestDecodeStrict_RejectsCustomTag(t *testing.T) {
	data := []byte("id: !mytag spec/foo\ntitle: Foo\n")
	var out decodeTarget
	err := DecodeStrict(data, &out)
	if err == nil {
		t.Fatal("DecodeStrict: want dialect error for custom tag, got nil")
	}
	if !strings.Contains(err.Error(), "custom tag") {
		t.Fatalf("DecodeStrict error = %q, want it to name the custom-tag offense", err)
	}
}

func TestDecodeStrict_RejectsNonStandardBuiltinTag(t *testing.T) {
	// !!binary is a real YAML 1.1 tag but outside this restricted
	// dialect's whitelist (only str/int/bool/float/null/seq/map/timestamp
	// are accepted) — proves the whitelist is closed, not "anything with
	// two bangs".
	data := []byte("id: spec/foo\ntitle: Foo\nblob: !!binary UGxhY2Vob2xkZXI=\n")
	var out map[string]interface{}
	err := DecodeStrict(data, &out)
	if err == nil {
		t.Fatal("DecodeStrict: want dialect error for !!binary, got nil")
	}
	if !strings.Contains(err.Error(), "custom tag") {
		t.Fatalf("DecodeStrict error = %q, want it to name the custom-tag offense", err)
	}
}

func TestDecodeStrictJSON_Happy(t *testing.T) {
	data := []byte(`{"id":"spec/foo","title":"Foo","tags":["a","b"]}`)
	var out struct {
		ID    string   `json:"id"`
		Title string   `json:"title"`
		Tags  []string `json:"tags"`
	}
	if err := DecodeStrictJSON(data, &out); err != nil {
		t.Fatalf("DecodeStrictJSON: %v", err)
	}
	if out.ID != "spec/foo" {
		t.Fatalf("decoded = %+v", out)
	}
}

func TestDecodeStrictJSON_UnknownField(t *testing.T) {
	data := []byte(`{"id":"spec/foo","bogus":true}`)
	var out struct {
		ID string `json:"id"`
	}
	if err := DecodeStrictJSON(data, &out); err == nil {
		t.Fatal("DecodeStrictJSON: want error for unknown field, got nil")
	}
}

func TestDecodeStrictJSON_TrailingData(t *testing.T) {
	data := []byte(`{"id":"spec/foo"}{"id":"spec/bar"}`)
	var out struct {
		ID string `json:"id"`
	}
	if err := DecodeStrictJSON(data, &out); err == nil {
		t.Fatal("DecodeStrictJSON: want error for trailing top-level value, got nil")
	}
}

// --- TASK 0 spike (PLAN.md risk R3 / spike S3) ---
//
// Proves the decode mechanism against real content before it becomes the
// production seam: strict-decode the frontmatter of the six real spec
// files with KnownFields(true), and prove anchors/aliases/custom tags are
// detectable and rejectable via a yaml.Node walk (checkDialect above).

// specDoc is a permissive shape matching the six real specs' own
// frontmatter (they are themselves component specs, each carrying an
// extra `schema:` field identifying the contract it defines — a
// meta-property of these six foundational documents, not part of the
// general artifact contract's common frontmatter modeled in frontmatter.go
// elsewhere in this package). It exists only to drive this spike.
type specDoc struct {
	ID     string     `yaml:"id"`
	Kind   string     `yaml:"kind"`
	Class  string     `yaml:"class"`
	Title  string     `yaml:"title"`
	Status string     `yaml:"status"`
	Owners []string   `yaml:"owners"`
	Links  []specLink `yaml:"links,omitempty"`
	Schema string     `yaml:"schema"`
}

type specLink struct {
	Type string `yaml:"type"`
	Ref  string `yaml:"ref"`
	Note string `yaml:"note,omitempty"`
}

func realSpecsDir(t *testing.T) string {
	t.Helper()
	// verdi/internal/artifact -> verdi-system/docs/design/specs
	dir := filepath.Join("..", "..", "..", "docs", "design", "specs")
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("real specs directory not found at %s (expected when verdi/ is checked out standalone, without its sibling verdi-system workspace): %v", dir, err)
	}
	return dir
}

func TestSpike_RealSpecFrontmatter_DecodesStrictly(t *testing.T) {
	dir := realSpecsDir(t)
	names := []string{
		"00-index.md",
		"01-store-layout.md",
		"02-artifact-contract.md",
		"03-evidence-model.md",
		"04-story-provider.md",
		"05-surfaces.md",
	}

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatalf("reading %s: %v", name, err)
			}
			fm, _, err := SplitFrontmatter(raw)
			if err != nil {
				t.Fatalf("SplitFrontmatter(%s): %v", name, err)
			}

			var doc specDoc
			if err := DecodeStrict(fm, &doc); err != nil {
				t.Fatalf("DecodeStrict(%s): %v", name, err)
			}
			if doc.ID == "" || doc.Title == "" || doc.Kind == "" {
				t.Fatalf("%s: decoded doc missing required fields: %+v", name, doc)
			}
			if doc.Kind != "spec" {
				t.Fatalf("%s: kind = %q, want %q", name, doc.Kind, "spec")
			}
		})
	}
}

// TestSpike_RealSpecFrontmatter_UnknownFieldFailsLoudly mutates a real
// spec's frontmatter with an injected unknown key and proves KnownFields
// rejects it — the negative complement to the happy-path spike above.
func TestSpike_RealSpecFrontmatter_UnknownFieldFailsLoudly(t *testing.T) {
	dir := realSpecsDir(t)
	raw, err := os.ReadFile(filepath.Join(dir, "02-artifact-contract.md"))
	if err != nil {
		t.Fatalf("reading 02-artifact-contract.md: %v", err)
	}
	fm, _, err := SplitFrontmatter(raw)
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	mutated := append([]byte(nil), fm...)
	mutated = append(mutated, []byte("\nunknown_injected_field: surprise\n")...)

	var doc specDoc
	err = DecodeStrict(mutated, &doc)
	if err == nil {
		t.Fatal("DecodeStrict: want error for injected unknown field in real spec frontmatter, got nil")
	}
	if !strings.Contains(err.Error(), "unknown_injected_field") {
		t.Fatalf("DecodeStrict error = %q, want it to name the injected field", err)
	}
}

// TestSpike_RealSpecFrontmatter_TypeMismatchFailsLoudly does the same for
// a type mismatch (owners becomes a scalar instead of a sequence).
func TestSpike_RealSpecFrontmatter_TypeMismatchFailsLoudly(t *testing.T) {
	dir := realSpecsDir(t)
	raw, err := os.ReadFile(filepath.Join(dir, "01-store-layout.md"))
	if err != nil {
		t.Fatalf("reading 01-store-layout.md: %v", err)
	}
	fm, _, err := SplitFrontmatter(raw)
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	mutated := strings.Replace(string(fm), "owners: [platform-team]", "owners: {not: a-list}", 1)
	if mutated == string(fm) {
		t.Fatal("test setup: expected owners: [platform-team] line not found in 01-store-layout.md frontmatter")
	}

	var doc specDoc
	if err := DecodeStrict([]byte(mutated), &doc); err == nil {
		t.Fatal("DecodeStrict: want error for owners type mismatch in real spec frontmatter, got nil")
	}
}

// TestSpike_RealSpecFrontmatter_DialectViolationsRejected proves that if
// a real spec's frontmatter were to gain an anchor, alias, or custom tag,
// checkDialect would catch it — the dialect side of the spike, exercised
// against the same real document text (with a synthetic anchor/alias
// spliced in, since the specs themselves are clean).
func TestSpike_RealSpecFrontmatter_DialectViolationsRejected(t *testing.T) {
	dir := realSpecsDir(t)
	raw, err := os.ReadFile(filepath.Join(dir, "00-index.md"))
	if err != nil {
		t.Fatalf("reading 00-index.md: %v", err)
	}
	fm, _, err := SplitFrontmatter(raw)
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}

	t.Run("anchor", func(t *testing.T) {
		mutated := strings.Replace(string(fm), "owners: [platform-team]", "owners: &o [platform-team]", 1)
		if mutated == string(fm) {
			t.Fatal("test setup: expected owners line not found")
		}
		var doc specDoc
		if err := DecodeStrict([]byte(mutated), &doc); err == nil {
			t.Fatal("DecodeStrict: want dialect error for anchor spliced into real frontmatter, got nil")
		}
	})

	t.Run("alias", func(t *testing.T) {
		mutated := string(fm) + "\ndefaults: &d foo\nsame: *d\n"
		var doc map[string]interface{}
		if err := DecodeStrict([]byte(mutated), &doc); err == nil {
			t.Fatal("DecodeStrict: want dialect error for alias spliced into real frontmatter, got nil")
		}
	})

	t.Run("custom tag", func(t *testing.T) {
		mutated := strings.Replace(string(fm), "kind: spec", "kind: !weird spec", 1)
		if mutated == string(fm) {
			t.Fatal("test setup: expected 'kind: spec' line not found")
		}
		var doc specDoc
		if err := DecodeStrict([]byte(mutated), &doc); err == nil {
			t.Fatal("DecodeStrict: want dialect error for custom tag spliced into real frontmatter, got nil")
		}
	})
}
