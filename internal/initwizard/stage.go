package initwizard

import (
	"fmt"
	"path/filepath"

	"github.com/jyang234/verdi/internal/atomicfile"
	"github.com/jyang234/verdi/internal/designscaffold"
	"github.com/jyang234/verdi/internal/model"
)

// canonicalTemplateNames is the fixed, closed set of class template
// filenames the wizard's template-set choice copies — the two entries
// model.Canonical().Classes itself declares (Class.Template values
// "feature.md"/"story.md", internal/model/canonical.yaml). Named here
// rather than derived from cfg.Model.Classes at call time because
// CopyCanonicalTemplates runs during staging, before any store.Config
// exists for the not-yet-promoted root — the canonical model is the
// only one in scope.
var canonicalTemplateNames = []string{"feature.md", "story.md"}

// WriteVerdiYAML stages VerdiYAMLContent at root/.verdi/verdi.yaml,
// creating .verdi/ as needed. Used by both verdi init paths (bare and
// --wizard) — root is always a staged, not-yet-promoted root
// (cmd/verdi/init.go's sibling temp directory), never the real store
// root; no write here is ever visible at the real location until the
// caller's own single os.Rename promotes the whole staged tree.
func WriteVerdiYAML(root string) error {
	path := filepath.Join(root, ".verdi", "verdi.yaml")
	if err := atomicfile.Write(path, []byte(VerdiYAMLContent), 0o644); err != nil {
		return fmt.Errorf("initwizard: writing %s: %w", path, err)
	}
	return nil
}

// WriteModelYAML stages RenderModelYAML(vocab)'s bytes at
// root/.verdi/model.yaml. Callers only invoke this when
// !VocabularyEmpty(vocab) (the "model.yaml only on divergence from
// canonical" contract, spec/init-wizard outcome) — WriteModelYAML itself
// does not enforce that, since the empty case is a perfectly valid
// (if pointless) render, and keeping the write-or-not decision at the
// call site keeps this function a simple, unconditional stage-what-I'm-
// given primitive.
func WriteModelYAML(root string, vocab model.Vocabulary) error {
	path := filepath.Join(root, ".verdi", "model.yaml")
	if err := atomicfile.Write(path, RenderModelYAML(vocab), 0o644); err != nil {
		return fmt.Errorf("initwizard: writing %s: %w", path, err)
	}
	return nil
}

// CopyCanonicalTemplates stages local, editable override copies of both
// canonical class templates at root/.verdi/templates/{feature,story}.md
// — the wizard's "template-set selection" choice (spec/init-wizard
// ac-2): byte-identical to the embedded default at the moment they are
// copied (designscaffold.LoadTemplate(root, name), called before either
// file exists under root/.verdi/templates/ so it resolves to the
// embedded canonical bytes, never a stale prior override), changing
// nothing about what a class's resolved template renders until the
// operator hand-edits a copy — LoadTemplate already prefers a store
// override of the same filename over the embedded default the moment
// one exists.
func CopyCanonicalTemplates(root string) error {
	for _, name := range canonicalTemplateNames {
		data, err := designscaffold.LoadTemplate(root, name)
		if err != nil {
			return fmt.Errorf("initwizard: loading canonical template %s: %w", name, err)
		}
		path := filepath.Join(root, ".verdi", "templates", name)
		if err := atomicfile.Write(path, data, 0o644); err != nil {
			return fmt.Errorf("initwizard: writing %s: %w", path, err)
		}
	}
	return nil
}
