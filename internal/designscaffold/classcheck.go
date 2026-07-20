package designscaffold

import (
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
)

// CheckClass asserts that a rendered-and-decoded scaffold's own declared
// class (spec.Class) agrees with want — the class its resolved template
// was looked up UNDER: a model.yaml classes.<id> entry (model check's
// checkTemplates), a --kind request (design start), or a declared stub's
// own class (the workbench's stub-instantiate, always story).
//
// A class's Template filename is DATA, not code (model.Class.Template):
// a misconfigured model.yaml can bind one class's Template to another
// class's template file, and a store's own .verdi/templates/ override can
// simply hardcode the wrong `class:` literal. Neither SplitFrontmatter nor
// DecodeSpec alone catches this — both accept any well-formed spec of ANY
// legal class, so a story class bound to feature.md still strict-decodes
// clean, just as a feature. Every scaffold consumer re-asserts this
// identity after decoding, before trusting the render belongs to the
// class it was resolved for (K1): model check's checkTemplates (a broken
// binding must fail closed at check time, exit 2, never surface first at
// a real design start or stub-instantiate); design start (never write a
// scaffold whose own class: line disagrees with the --kind it was asked
// for, even though stdout and the commit message echo the REQUESTED
// kind); and the workbench's stub-instantiate (same guard, story class
// only, before any git plumbing runs).
//
// Returns nil when spec.Class == want. The error names both classes so a
// caller's own wrapping (template file, declared/requested class) gives
// an operator every fact needed in one message.
func CheckClass(spec *artifact.SpecFrontmatter, want artifact.SpecClass) error {
	if spec.Class != want {
		return fmt.Errorf("rendered content declares class %q, want %q", spec.Class, want)
	}
	return nil
}
