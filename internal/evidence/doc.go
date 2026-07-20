// Package evidence implements 03 §The fold: the deterministic reduction of
// a story's acceptance-criteria evidence records, waivers, and attestations
// into a per-AC status and a story-level eligibility verdict.
//
// It is deliberately separate from internal/bundle (which assembles
// verdicts.json in the first place, on the producing side), from
// internal/runtime (which emits runtime.json's records, spec/runtime-
// evidence dc-1/dc-2), and from cmd/verdi (which resolves a story/spec ref,
// finds the store root, and prints the result — see cmd/verdi/matrix.go).
// This package only folds an already-loaded record set; it does the loading
// itself (LoadRecords, which reads both verdicts.json and runtime.json —
// records.go's RecordFileNames) but leaves ref/story resolution to its
// caller.
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
//   - A declared runtime kind is always "awaited post-merge" regardless of
//     whether a record exists yet (03 §The fold's own text — runtime
//     records "attach by timestamp after merge"), which the fold reads as
//     unconditionally contributing to the pending branch (never no-signal)
//     for that kind — see foldAC. This predates and is unchanged by
//     spec/runtime-evidence's producer (internal/runtime, OQ-2/true-closure
//     ac-3 resolved): even with a real producer wired up, a story can be
//     merged before its first scheduled probe run ever fires, so "no
//     record yet" must still read as pending, never no-signal, exactly as
//     it did when runtime had no producer at all.
package evidence
