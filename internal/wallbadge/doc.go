// Package wallbadge computes the board wall's badges (spec/badge-computes,
// spec/wall-receipts): VL lint findings partitioned by their own
// wall-locus self-classification (lint.Finding.Locus, dc-3), the
// spec-stale/pending-supersession ladder flags through the EXACT exported
// entry points internal/dex/lens.go's story-lens uses — never a second,
// drifting logic path (co-3) — and the size-smell observation
// (spec/case-file-flags ac-2: dc-1's deterministic viewport proxy, an
// observation and never a rule). Every result is a canonical derivation
// record (dc-2): the rule id, the pinned inputs with their revisions, and
// the firing records, sufficient for the derivation drawer to render
// receipts without recomputing anything.
//
// This package is a pure compute layer: it never touches a BoardProjection
// or renders anything. internal/workbench's loadBoard (boardspec.go) is
// the ONE attachment point (dc-1) — the I/O enrichment tier that calls
// ComputeBadges after buildProjection and folds its result onto the
// projection, exactly the posture attachObligations already established.
// Kept as its own package rather than folded into internal/workbench so
// that workbench (which by established design, see cmd/verdi/
// reviewfeed.go's doc comment, never imports internal/forge) stays free
// of this package's lint/decisionsweep/evidence-heavy compute surface,
// and so the sibling wall-receipts stories (evidence-slot, case-file-
// flags) have one shared home to add their own computes to (dc-1: "never
// a second attachment path or a second record shape") without further
// growing internal/workbench's own concerns.
package wallbadge
