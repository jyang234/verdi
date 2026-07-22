package initwizard

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/designscaffold"
	"github.com/jyang234/verdi/internal/model"
)

// loadEmbeddedTemplateForTest resolves name's canonical bytes against
// root (which must carry no .verdi/templates/ override of its own) —
// the same designscaffold.LoadTemplate call CopyCanonicalTemplates
// itself makes, used here only as an independent reference for the
// byte-identity assertion.
func loadEmbeddedTemplateForTest(t *testing.T, root, name string) []byte {
	t.Helper()
	data, err := designscaffold.LoadTemplate(root, name)
	if err != nil {
		t.Fatalf("designscaffold.LoadTemplate(%s): %v", name, err)
	}
	return data
}

// TestWriteVerdiYAML_WritesExactContent proves WriteVerdiYAML stages
// exactly VerdiYAMLContent at .verdi/verdi.yaml under the given root,
// creating the .verdi/ directory as needed.
func TestWriteVerdiYAML_WritesExactContent(t *testing.T) {
	root := t.TempDir()
	if err := WriteVerdiYAML(root); err != nil {
		t.Fatalf("WriteVerdiYAML: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(root, ".verdi", "verdi.yaml"))
	if err != nil {
		t.Fatalf("reading staged verdi.yaml: %v", err)
	}
	if string(got) != VerdiYAMLContent {
		t.Fatalf("staged verdi.yaml = %q, want %q", got, VerdiYAMLContent)
	}
}

// TestWriteModelYAML_WritesRenderedContent proves WriteModelYAML stages
// exactly RenderModelYAML(vocab)'s own bytes at .verdi/model.yaml.
func TestWriteModelYAML_WritesRenderedContent(t *testing.T) {
	root := t.TempDir()
	vocab := model.Vocabulary{Classes: map[string]string{"story": "Task"}}
	if err := WriteModelYAML(root, vocab); err != nil {
		t.Fatalf("WriteModelYAML: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(root, ".verdi", "model.yaml"))
	if err != nil {
		t.Fatalf("reading staged model.yaml: %v", err)
	}
	want := RenderModelYAML(vocab)
	if string(got) != string(want) {
		t.Fatalf("staged model.yaml does not match RenderModelYAML's own bytes:\ngot:  %s\nwant: %s", got, want)
	}
	decoded, err := model.DecodeModel(got)
	if err != nil {
		t.Fatalf("staged model.yaml failed to decode: %v", err)
	}
	if decoded.Vocabulary.Classes["story"] != "Task" {
		t.Fatalf("staged model.yaml decoded Vocabulary.Classes[story] = %q, want %q", decoded.Vocabulary.Classes["story"], "Task")
	}
}

// TestCopyCanonicalTemplates_CopiesBothFiles proves the template-set
// selection materializes local, editable override copies of BOTH
// canonical templates, byte-identical to the embedded default (spec/
// init-wizard ac-2's "copying the canonical templates into
// .verdi/templates/ for local customization").
func TestCopyCanonicalTemplates_CopiesBothFiles(t *testing.T) {
	root := t.TempDir()
	if err := CopyCanonicalTemplates(root); err != nil {
		t.Fatalf("CopyCanonicalTemplates: %v", err)
	}
	for _, name := range []string{"feature.md", "story.md"} {
		path := filepath.Join(root, ".verdi", "templates", name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading copied template %s: %v", name, err)
		}
		if len(data) == 0 {
			t.Fatalf("copied template %s is empty", name)
		}
	}
}

// TestCopyCanonicalTemplates_MatchesEmbeddedDefault proves the copies
// are byte-identical to what designscaffold.LoadTemplate resolves absent
// any override — CopyCanonicalTemplates must never invent or alter
// content, only relocate the embedded default into an editable home.
func TestCopyCanonicalTemplates_MatchesEmbeddedDefault(t *testing.T) {
	// A fresh root with no override present anywhere resolves
	// LoadTemplate straight to the embedded canonical bytes — the same
	// property CopyCanonicalTemplates itself depends on when it reads
	// FROM tempRoot before ever writing to it.
	referenceRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(referenceRoot, ".verdi"), 0o755); err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	if err := CopyCanonicalTemplates(root); err != nil {
		t.Fatalf("CopyCanonicalTemplates: %v", err)
	}
	for _, name := range []string{"feature.md", "story.md"} {
		want := loadEmbeddedTemplateForTest(t, referenceRoot, name)
		got, err := os.ReadFile(filepath.Join(root, ".verdi", "templates", name))
		if err != nil {
			t.Fatalf("reading copied template %s: %v", name, err)
		}
		if string(got) != string(want) {
			t.Fatalf("copied template %s does not match the embedded canonical default", name)
		}
	}
}
