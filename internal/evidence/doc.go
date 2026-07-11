// Package evidence implements 03 §The fold: the deterministic reduction of
// a story's acceptance-criteria evidence records, waivers, and attestations
// into a per-AC status and a story-level eligibility verdict.
//
// It is deliberately separate from internal/bundle (which assembles
// verdicts.json in the first place, on the producing side) and from
// cmd/verdi (which resolves a story/spec ref, finds the store root, and
// prints the result — see cmd/verdi/matrix.go). This package only folds an
// already-loaded record set; it does the loading itself (LoadRecords) but
// leaves ref/story resolution to its caller.
//
// The fold, verbatim (03 §The fold):
//
//	for each AC:
//	    records := current authoritative records bound to this AC at C
//	               ("current" = latest record per (kind, producer) whose commit
//	                is an ancestor of C; runtime records attach by timestamp
//	                after merge)
//	    if waiver(story, AC) active:            status = waived
//	    else if any record.verdict == fail:     status = violated
//	    else if every expected kind has ≥1 pass
//	            (attestation kind: file exists): status = evidenced
//	    else if some expected kind has records
//	            or is awaited post-merge:        status = pending
//	    else:                                    status = no-signal
//
//	story.violated  = any AC violated
//	story.eligible  = every AC in {evidenced, waived}
//
// Precedence is total: waived > violated > evidenced > pending > no-signal.
//
// Two v0-specific readings, both disclosed here rather than invented
// silently elsewhere:
//
//   - Producer identity (03's "producer = the declared artifact id") is not
//     always recoverable from an on-disk record: witness text is free-form
//     and, for a static record whose obligation has a call site, usually
//     shows "fn @ site" rather than the binding's producer id. See
//     artifact.Evidence's Producer field doc for the resolution (an
//     optional explicit field, stamped by internal/bundle, falling back to
//     grouping by witness text when absent).
//   - Runtime evidence has no v0 producer (OQ-2): a declared runtime kind
//     is therefore always "awaited post-merge" regardless of whether any
//     record exists, which the fold reads as unconditionally contributing
//     to the pending branch (never no-signal) for that kind — see foldAC.
package evidence
