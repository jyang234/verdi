package designscaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
