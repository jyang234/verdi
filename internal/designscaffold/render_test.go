package designscaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

// TestRender_Happy proves the basic substitution shapes Render's callers
// depend on: plain scalar interpolation, the %q-equivalent title quoting,
// and an {{if}} branch.
func TestRender_Happy(t *testing.T) {
	const tmpl = `title: {{printf "%q" .Title}}
ref: {{.Ref}}{{if .StoryRef}}
story: {{.StoryRef}}{{end}}
`
	got, err := Render([]byte(tmpl), ScaffoldData{Ref: "spec/x", Title: "A Title", StoryRef: "jira:LOAN-1"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	want := "title: \"A Title\"\nref: spec/x\nstory: jira:LOAN-1\n"
	if got != want {
		t.Fatalf("Render = %q, want %q", got, want)
	}
}

// TestSafeScalar is K4's static register: safeScalar's own table, proving
// the guard passes every current constrained-input SHAPE through bare
// (the byte-parity floor TestByteForByte pins end to end) while quoting
// exactly the three smuggle-risk shapes named in its own doc comment.
func TestSafeScalar(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"bare kebab ref", "spec/foo-bar", "spec/foo-bar"},
		{"owners flow-sequence literal", "[unassigned]", "[unassigned]"},
		{"owners flow-sequence, multiple", "[platform-team, qa-lead]", "[platform-team, qa-lead]"},
		{"scheme-prefixed story ref", "jira:LOAN-1482", "jira:LOAN-1482"},
		{"todo placeholder story ref", "todo:REPLACE-ME", "todo:REPLACE-ME"},
		{"embedded newline quoted", "a\nb", `"a\nb"`},
		{"embedded double quote quoted", `a"b`, `"a\"b"`},
		{"colon-space quoted", "TODO: replace", `"TODO: replace"`},
		{"colon with no following space stays bare", "jira:LOAN-1", "jira:LOAN-1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := safeScalar(tc.in); got != tc.want {
				t.Errorf("safeScalar(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestSafeScalar_OwnersSmugglePathClosed is K4's load-bearing security
// proof, reproducing the exact trap the round's final review flagged as
// latent: with owners: rendered BARE (the pre-K4 shape), a caller-supplied
// Owners value carrying an embedded newline followed by a legitimate-
// looking key ("[unassigned]\nspike: true") renders as TWO lines —
// `owners: [unassigned]` then a SECOND, illegitimate top-level `spike:
// true` frontmatter key smuggled in underneath it, silently turning a
// plain story scaffold into a spike one. Post-K4, safeScalar quotes the
// whole value (it contains a newline), so the rendered YAML holds ONE
// scalar spanning what would-be two lines — which is no longer even
// decodable as the owners: field's own declared type ([]string), so the
// smuggle attempt fails closed at decode rather than silently annexing a
// second key. No real caller can reach this today (Owners is always the
// fixed "[unassigned]" literal, designscaffold.go's defaultOwnersLiteral)
// — this exercises the mechanism directly via Render, the way a future
// caller passing a dynamic Owners value would.
func TestSafeScalar_OwnersSmugglePathClosed(t *testing.T) {
	tmpl := mustCanonicalTemplate(t, "story.md")
	data := ScaffoldData{
		Ref:      "spec/x",
		Title:    "X",
		Owners:   "[unassigned]\nspike: true",
		StoryRef: "jira:LOAN-1",
		Links:    []StoryLink{{Type: artifact.LinkImplements, Ref: "spec/y#ac-1"}},
	}
	content, err := Render(tmpl, data)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	// The smuggle attempt must be VISIBLE as a single quoted scalar, never
	// as a bare second line that a naive reader (or a lenient YAML
	// decoder) would read as its own key.
	if strings.Contains(content, "owners: [unassigned]\nspike: true\n") {
		t.Fatalf("owners: rendered the smuggled spike: true as a bare, separate line — the newline-smuggle path is NOT closed:\n%s", content)
	}
	if !strings.Contains(content, `owners: "[unassigned]\nspike: true"`) {
		t.Fatalf("owners: did not quote the newline-carrying value as expected:\n%s", content)
	}

	// And the smuggle attempt must fail CLOSED at decode (owners: no
	// longer decodes as []string), never silently succeed with Spike
	// smuggled in as true.
	fm, _, err := artifact.SplitFrontmatter([]byte(content))
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	if _, err := artifact.DecodeSpec(fm); err == nil {
		t.Fatal("DecodeSpec succeeded on a smuggled owners: value — want a strict-decode failure (fail closed, never a silently-annexed spike: true)")
	}
}

// TestRender_Negative_MalformedSyntax proves a template with a syntax
// error fails closed at parse time rather than panicking or rendering
// partial content — exactly the failure `verdi model check` (ac-3) is
// built to catch before a scaffold consumer ever would.
func TestRender_Negative_MalformedSyntax(t *testing.T) {
	const tmpl = `title: {{.Title`
	if _, err := Render([]byte(tmpl), ScaffoldData{Title: "x"}); err == nil {
		t.Fatal("Render(malformed template) = nil error, want a parse failure")
	}
}

// TestRender_Negative_UndefinedField proves a template referencing a
// field ScaffoldData does not declare fails closed at execution time
// (struct field access to an unknown name is already a hard template
// execution error; missingkey=error additionally covers any future map
// value Render might be asked to interpolate).
func TestRender_Negative_UndefinedField(t *testing.T) {
	const tmpl = `bogus: {{.NoSuchField}}`
	if _, err := Render([]byte(tmpl), ScaffoldData{}); err == nil {
		t.Fatal("Render(undefined field) = nil error, want an execution failure")
	}
}

// TestLoadTemplate_EmbeddedFallback proves a store with no
// .verdi/templates/ override at all resolves to the embedded canonical
// template of the same name — the "absence changes nothing" posture this
// story's outcome promises, mirroring store.Open's own model.yaml
// resolution.
func TestLoadTemplate_EmbeddedFallback(t *testing.T) {
	root := t.TempDir() // no .verdi/templates/ at all
	got, err := LoadTemplate(root, "feature.md")
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	want, err := embeddedTemplates.ReadFile("templates/feature.md")
	if err != nil {
		t.Fatalf("reading embedded feature.md directly: %v", err)
	}
	if string(got) != string(want) {
		t.Fatal("LoadTemplate with no override did not return the embedded canonical bytes verbatim")
	}
}

// TestLoadTemplate_StoreOverrideWins proves a store's own
// .verdi/templates/<name>.md file wins over the embedded canonical
// default when present (spec/scaffold-templates outcome).
func TestLoadTemplate_StoreOverrideWins(t *testing.T) {
	root := t.TempDir()
	overrideDir := filepath.Join(root, ".verdi", "templates")
	if err := os.MkdirAll(overrideDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	const overrideContent = "custom override content, not the embedded default\n"
	if err := os.WriteFile(filepath.Join(overrideDir, "feature.md"), []byte(overrideContent), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := LoadTemplate(root, "feature.md")
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	if string(got) != overrideContent {
		t.Fatalf("LoadTemplate = %q, want the store override content %q", got, overrideContent)
	}
}

// TestLoadTemplate_Negative_UnreadableOverride proves a genuine read
// failure on an override path (not mere absence) propagates as an error
// rather than silently falling back to the embedded default — a store
// that shipped a broken override should hear about it, never see it
// silently ignored.
func TestLoadTemplate_Negative_UnreadableOverride(t *testing.T) {
	root := t.TempDir()
	overrideDir := filepath.Join(root, ".verdi", "templates")
	// A DIRECTORY at the override's own path (rather than a plain file) is
	// unreadable-as-a-file but not os.IsNotExist — the "real read failure,
	// not mere absence" case LoadTemplate must not confuse with a missing
	// override.
	if err := os.MkdirAll(filepath.Join(overrideDir, "feature.md"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if _, err := LoadTemplate(root, "feature.md"); err == nil {
		t.Fatal("LoadTemplate(directory where a file is expected) = nil error, want a read failure")
	}
}

// TestLoadTemplate_Negative_UnknownName proves a filename with neither a
// store override nor an embedded canonical default fails closed with a
// clear error, never a silent empty template.
func TestLoadTemplate_Negative_UnknownName(t *testing.T) {
	root := t.TempDir()
	if _, err := LoadTemplate(root, "no-such-class.md"); err == nil {
		t.Fatal("LoadTemplate(unknown filename) = nil error, want a not-found failure")
	}
}

// TestLoadTemplate_Negative_PathEscape proves LoadTemplate is a
// defense-in-depth containment guard on the template filename
// (judged-template-filename-escapes-templates-dir): a separator-carrying,
// absolute, or . / .. value — which internal/model's Model.Validate kernel
// rule already rejects — is refused HERE too, with a SPECIFIC bare-filename
// error (never an incidental "not found"/"is a directory" read error), so
// the ".verdi/templates/<class>.md" containment invariant holds even for a
// caller that reaches LoadTemplate without going through Validate. Both
// layers enforce the one shared artifact.IsBareFilename definition.
func TestLoadTemplate_Negative_PathEscape(t *testing.T) {
	root := t.TempDir()
	for _, bad := range []string{"../../evil.md", "sub/dir.md", "/abs/evil.md", ".", ".."} {
		_, err := LoadTemplate(root, bad)
		if err == nil {
			t.Errorf("LoadTemplate(%q) = nil error, want a bare-filename refusal", bad)
			continue
		}
		if !strings.Contains(err.Error(), "bare filename") {
			t.Errorf("LoadTemplate(%q) error = %q, want it to name the bare-filename rule (a specific refusal, not an incidental read/embed error)", bad, err.Error())
		}
	}
}
