package lint

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
)

// vl017 enforces "open-question stickies resolved-or-carried: on a design
// branch targeting the default branch, every open-question annotation
// (§Record schemas) is either status: resolved or explicitly carried as a
// declared open-question object on the spec — the VL-014 successor for new
// specs. Scoped by mutable-zone presence: open-question annotations live
// in the per-checkout mutable zone (data/mutable/annotations/*.jsonl),
// which is never committed. The rule enforces where the mutable zone is
// present — author-local lint and the workbench's review-ready indicator.
// Where the mutable zone is absent (CI clone), lint reports the check
// disclosed-unproven for that spec — never a silent pass (constitution 2);
// a vacuous green is never emitted" (02 §Lint rules, VL-017, in full).
//
// Judgment call (recorded here and in the phase report): 02 gives no
// formal id-based link between an annotation and a spec's declared
// open_questions entry — an annotation only carries a target ref plus a
// free-text body/selector. This rule treats an unresolved open-question
// annotation as "carried" when the target spec declares an open_questions
// entry whose text matches the annotation's body exactly: the same
// question, now formalized as a real object (02 §Object model:
// "graduates into a real object ... the entry is removed in the same
// edit" — an exact-text match is the smallest reversible reading of
// "the same question").
//
// This rule is scoped to new-class specs only, mirroring VL-014's
// complementary grandfather scope (02: VL-017 is "the VL-014 successor for
// new specs" — VL-014 fires only on dispositions:-carrying, i.e.
// grandfathered, specs; VL-017 is the other side of that same split). It
// reuses isNewClassSpec (vl006.go), the discriminator this phase settled
// for R4-I-15: a v0 grandfathered spec (e.g. this corpus's stale-decline,
// new-feature-x) is never subject to it, and a store with only component
// specs (e.g. this repo's own self-hosted .verdi/, which also has no
// object model at all) never has anything for VL-017 to check,
// disclosed-unproven or otherwise.
type vl017 struct{}

func (vl017) ID() string { return "VL-017" }

func (vl017) Check(in *RunInput) []Finding {
	var applicable []*Document
	for _, d := range in.Snapshot.Docs {
		if d.Grandfathered || d.DecodeErr != nil || d.Spec == nil {
			continue
		}
		if !isNewClassSpec(d.Spec) {
			continue
		}
		applicable = append(applicable, d)
	}

	if !mutableZonePresent(in.Root) {
		var findings []Finding
		for _, d := range applicable {
			findings = append(findings, Finding{Rule: "VL-017", Path: d.RelPath, Severity: SeverityDisclosure, Message: "open-question resolved-or-carried check is disclosed-unproven: data/mutable/ is absent (bare clone; the mutable zone is never committed, 01 §Zones) — not a silent pass (constitution 2, three-valued honesty). This is a printed notice, not a verdict failure: a run with no other findings still exits 0 (adjudicated at W2 wave close)"})
		}
		return findings
	}

	annotations, err := readMutableAnnotations(in.Root)
	if err != nil {
		return []Finding{{Rule: "VL-017", Path: ".verdi/data/mutable/annotations", Message: fmt.Sprintf("reading mutable annotations: %v", err)}}
	}

	specByID := make(map[string]*Document, len(applicable))
	for _, d := range applicable {
		specByID[d.Base.ID] = d
	}

	var findings []Finding
	for _, a := range annotations {
		if a.Type != artifact.AnnotationQuestion || a.Status == artifact.AnnotationResolved {
			continue
		}
		if a.Target == nil {
			continue // board-only sticky, no spec target to check against
		}
		ref, err := artifact.ParseRef(a.Target.Ref)
		if err != nil {
			continue // malformed target ref is not this rule's concern
		}
		spec, ok := specByID[artifact.Ref{Kind: ref.Kind, Name: ref.Name}.String()]
		if !ok {
			continue // target isn't a feature/story spec in this snapshot
		}
		if !carriedAsOpenQuestion(spec.Spec, a.Body) {
			// Object-anchored (badge-computes dc-3's "unresolved open
			// question" bucket) to the annotation's own id: an
			// unresolved, non-graduated question annotation renders as its
			// own sticky card on its board (buildProjection's Stickies),
			// so the finding badges that sticky directly rather than the
			// case file.
			findings = append(findings, Finding{Rule: "VL-017", Path: spec.RelPath, Message: fmt.Sprintf("open-question annotation %s (%q) is neither status:resolved nor carried as a declared open_questions object on %s", a.ID, a.Body, spec.Base.ID), Locus: ObjectLocus(a.ID)})
		}
	}
	return findings
}

// carriedAsOpenQuestion reports whether spec declares an open_questions
// entry whose text matches body exactly (see vl017's doc comment for the
// judgment call this implements).
func carriedAsOpenQuestion(spec *artifact.SpecFrontmatter, body string) bool {
	for _, q := range spec.OpenQuestions {
		if q.Text == body {
			return true
		}
	}
	return false
}

// mutableZonePresent reports whether root/.verdi/data/mutable exists at
// all — the per-checkout working area that is never committed (01
// §Zones), and so is invisible to walkDocuments' walk (which explicitly
// skips .verdi/data).
func mutableZonePresent(root string) bool {
	info, err := os.Stat(filepath.Join(root, ".verdi", "data", "mutable"))
	return err == nil && info.IsDir()
}

// readMutableAnnotations reads every *.jsonl file directly under
// root/.verdi/data/mutable/annotations/ (01 §Directory layout) and
// tolerantly decodes each line — a malformed line is not this rule's
// concern (no other rule in this engine reads the mutable zone at all, so
// there is nothing else to attribute a decode failure to; skipping rather
// than failing closed here matches this rule's own "never a silent pass
// only where it can prove something" scope, not a general annotation
// linter). An absent annotations/ subdirectory (mutable zone present, but
// nothing recorded yet) is not an error.
func readMutableAnnotations(root string) ([]artifact.Annotation, error) {
	dir := filepath.Join(root, ".verdi", "data", "mutable", "annotations")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []artifact.Annotation
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			a, err := artifact.DecodeAnnotation([]byte(line))
			if err != nil {
				continue
			}
			out = append(out, *a)
		}
	}
	return out, nil
}
