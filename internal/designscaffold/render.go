package designscaffold

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/jyang234/verdi/internal/artifact"
)

// embeddedTemplates carries the canonical scaffold templates (spec/
// scaffold-templates ac-1) — the shipped default every class's Class.
// Template filename resolves to when a store carries no override of its
// own, precedent internal/dex/assets.go / internal/model/embed.go's
// Canonical().
//
//go:embed templates/*.md
var embeddedTemplates embed.FS

// ScaffoldData is Render's one input shape (fixed signature, docs/design/
// plans/2026-07-17-extensibility-phase1-plan.md Task 8): every field a
// class's template might reference when scaffolding a fresh spec. Not
// every template uses every field — the canonical feature.md ignores
// Spike/Links/ParentRef entirely, and a plain (non-spike) story ignores
// nothing story.md itself defines — a template failing to reference a
// field is never an error; only referencing an UNDEFINED one is (Render's
// missingkey=error posture, mirroring struct field access semantics: an
// unknown field name is already a template execution error by
// construction, since ScaffoldData is a struct, not a map).
type ScaffoldData struct {
	Ref       string
	Title     string
	Owners    string
	Problem   string
	Outcome   string
	StoryRef  string
	ParentRef string
	Links     []StoryLink
	Spike     bool
}

// Render instantiates tmpl (a text/template source: an embedded canonical
// template, or a store's own .verdi/templates/<name>.md override) against
// data, failing closed on any template syntax error or undefined-field
// access rather than producing a scaffold silently missing content. Every
// real caller self-validates the result afterward (SplitFrontmatter +
// DecodeSpec, exactly as design start and stub-instantiate already did
// before this story); `verdi model check` (ac-3) runs the identical
// instantiate-then-strict-decode round trip proactively, over every
// resolved template, so a broken template is caught at check time rather
// than at a scaffold consumer's first use.
func Render(tmpl []byte, data ScaffoldData) (string, error) {
	t, err := template.New("scaffold").Option("missingkey=error").Parse(string(tmpl))
	if err != nil {
		return "", fmt.Errorf("designscaffold: parsing template: %w", err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("designscaffold: rendering template: %w", err)
	}
	return buf.String(), nil
}

// LoadTemplate resolves filename's template bytes: a store override at
// .verdi/templates/<filename> under root when one exists, else the
// embedded canonical template of the same name (spec/scaffold-templates
// outcome: "a store with no templates/ directory at all changes
// nothing" — the same absence-changes-nothing posture store.Open's own
// model.yaml resolution already established, internal/store/open.go's
// loadModel). filename is a class's own Class.Template value (model.Class,
// kernel-required non-empty) — callers resolve it from the store's
// already-open model, never hardcode a class-to-filename mapping here.
func LoadTemplate(root, filename string) ([]byte, error) {
	// Defense-in-depth on the containment invariant internal/model's
	// Model.Validate kernel rule already enforces (judged-template-filename-
	// escapes-templates-dir): filename must be a BARE filename, so the join
	// below cannot escape .verdi/templates/. Rejecting a separator-carrying,
	// absolute, or . / .. value HERE keeps the invariant even for a caller
	// that reaches LoadTemplate without a Validate-clean model — a specific,
	// fail-closed refusal naming the rule rather than an incidental
	// file-not-found on a resolved-elsewhere path. One shared definition
	// (artifact.IsBareFilename) backs both layers.
	if !artifact.IsBareFilename(filename) {
		return nil, fmt.Errorf("designscaffold: template filename %q must be a bare filename under .verdi/templates/ (no path separator, absolute path, or . / ..)", filename)
	}
	override := filepath.Join(root, ".verdi", "templates", filename)
	data, err := os.ReadFile(override)
	if err == nil {
		return data, nil
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("designscaffold: reading template override %s: %w", override, err)
	}
	data, err = embeddedTemplates.ReadFile("templates/" + filename)
	if err != nil {
		return nil, fmt.Errorf("designscaffold: no embedded canonical template named %q: %w", filename, err)
	}
	return data, nil
}
