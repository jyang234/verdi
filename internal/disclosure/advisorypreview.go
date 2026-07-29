package disclosure

// The advisory-preview disclosure lives here, in the seam package itself,
// because it has producers in TWO different trees — cmd/verdi's matrix
// rungs (matrix.go's printMatrix and featurematrix.go's
// printFeatureMatrix) and internal/workbench's advisory preview matrix
// page (matrix.go's renderMatrixHTML) — and CLAUDE.md's rule is that
// anything used by two or more packages lives in one shared internal/
// package. cmd/verdi cannot be that home (nothing under internal/ may
// import a main package), and internal/workbench owning it would put a
// CLI banner's vocabulary inside a UI package. Authoring the text is a
// disclosure-vocabulary concern, which is precisely this package's single
// concern, so the constructor sits next to New/Render — one home, no copy
// to drift (the same placement reasoning as reviewfeed.go).
//
// The migration this file completes: the CLI rungs already rendered
// through the seam (source matrix:advisory-preview), but the workbench
// page still hand-authored a THIRD vocabulary for the SAME state — an
// HTML "PREVIEW — ADVISORY" banner invisible to IsRendered and to every
// disclosure consumer (spec/disclosure-legibility#ac-1 binds every
// surface to one vocabulary; judged-ac-1-vocabulary-coverage).

// SourceAdvisoryPreview is the source id every advisory-preview
// disclosure carries, whichever surface renders it: the state is one
// producing condition — a fold that included advisory (source: local)
// evidence — however it reaches a reader.
const SourceAdvisoryPreview = "matrix:advisory-preview"

// advisoryPreviewText names the observed fact and its consequence, and
// NEVER its own severity: Render supplies the one vocabulary word
// (spec/disclosure-legibility#ac-1). The pre-seam banners opened with
// hand-authored "PREVIEW:"/"PREVIEW — ADVISORY" markers instead — extra
// vocabularies for a disclosed-unproven state, invisible to IsRendered
// and to every disclosure consumer.
const advisoryPreviewText = "this fold included advisory (source: local) evidence alongside authoritative (source: ci) evidence, so the statuses below are not the merge gate's answer — local evidence is never authoritative (04/03)"

// AdvisoryPreview is the structured value every advisory-fold surface
// constructs at its existing decision point, so each printed banner IS
// that value rendered — never a second, hand-aligned copy of it, and
// never per-surface copies that can drift. The scope is empty: the
// disclosure is about the FOLD as a whole, not about any one artifact in
// the table below it (the checkout-wide form).
func AdvisoryPreview() Disclosure {
	return New(SourceAdvisoryPreview, "", advisoryPreviewText)
}
