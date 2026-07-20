// verdi sync --produce-runtime (spec/runtime-evidence dc-1, dc-3): the
// scheduled-probe producer entrypoint. It writes one kind: runtime
// artifact.Evidence record — built by internal/runtimeprobe.Emit — into
// derived/<RefSlug(spec.ID)>/<commit>/runtime.json, the sibling of
// verdicts.json dc-2 describes, so `verdi sync`'s ordinary forge-pull path
// (writeDerivedTree, unchanged) and internal/evidence.LoadRecords (extended
// to also read runtime.json — records.go's RecordFileNames) carry it
// through to the fold exactly like every other evidence kind.
//
// TWO invocation shapes:
//
//   - Bare (`verdi sync --produce-runtime`, no --story/--ac/--verdict/
//     --witness): the scheduled, HONEST no-op. verdi itself has no live
//     service to probe (dc-3) — nothing in this repo's own verdi.bindings.yaml
//     or CI config ever supplies a genuine runtime verdict for a verdi spec,
//     and none should, since that would be exactly the fabricated "passing"
//     record dc-3 forbids. .github/workflows/runtime-probe.yml's scheduled
//     job invokes this bare form: it proves the cron trigger and this
//     entrypoint are real and wired (dc-1) without inventing a check result
//     that does not exist.
//   - Full (`verdi sync --produce-runtime --story <ref> --ac <id> --verdict
//     <pass|fail|abstain> --witness "<text>"`): the real emission path. A
//     real service's own scheduled job runs ITS OWN check (out of scope for
//     verdi — 03 §Pluggable evidence's precedent: "the producer side is out
//     of scope for this contract") and invokes this form with the genuine
//     outcome; verdi's job is only to stamp provenance honestly and place
//     the record where every fold reader already looks.
//
// The same source: ci / source: local discipline as --produce's runProduce
// governs both shapes: a stamp of source: ci requires a genuine, detected CI
// environment (internal/lint.ReadCIEnv) and no --force-local override;
// anything else stamps source: local, folded only under --preview.
//
// TRANSCRIPTION semantic (spec/fail-loud ac-2): verdi STAMPS an externally
// computed verdict here; it does not compute one. So emission SUCCESS is
// exit 0 REGARDLESS of the stamped verdict's value — a --verdict fail run
// that successfully writes its record is exit 0, exactly like a --verdict
// pass run; only a failure to write the record (a dangling AC, an I/O
// error, refusing to run outside CI) is exit 2. Contrast sync.go's
// --produce path: runProduce's evaluateBundle computes its OWN verdicts
// from the bundle it just assembled and surfaces THAT verdict as exit 1 on
// failure — a genuinely different contract for a genuinely different
// producer. Here the fail verdict itself is real signal, but it is
// consumed downstream by the fold (evidence.Fold reading runtime.json),
// never by this producer's own exit code.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/lint"
	"github.com/jyang234/verdi/internal/runtimeprobe"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/storyresolve"
)

// runProduceRuntime is the testable core of `verdi sync --produce-runtime`.
// storyArg/acID/witness/verdict are either all empty (the bare, scheduled
// no-op) or all set (the real emission path) — cmdSync's flag parsing
// enforces that split before calling this function.
func runProduceRuntime(ctx context.Context, root, commit, storyArg, acID, witness string, verdict artifact.EvidenceVerdict, forceLocal bool, deps syncDeps) int {
	inCI := lint.ReadCIEnv().InCI
	if !inCI && !forceLocal {
		fmt.Fprintln(deps.Stderr, "sync: --produce-runtime refuses to run outside CI (spec/runtime-evidence dc-3: a source: ci stamp is only ever genuine in a real, detected CI environment); pass --force-local for local testing only")
		return 2
	}
	authoritative := inCI && !forceLocal
	if !authoritative {
		fmt.Fprintln(deps.Stderr, "sync: --force-local: producing a NON-AUTHORITATIVE runtime record stamped source:local (only a genuine CI run, without --force-local, may stamp source:ci; a gate folds source:local records only under --preview)")
	}

	if storyArg == "" && acID == "" && witness == "" && verdict == "" {
		fmt.Fprintln(deps.Stdout, "sync: --produce-runtime: no --story/--ac/--verdict/--witness given; nothing to report (spec/runtime-evidence dc-3: verdi has no live service of its own to probe) — a real consumer passes all four together to emit one check's outcome")
		return 0
	}
	if storyArg == "" || acID == "" || witness == "" || verdict == "" {
		// vocab:identity — CLI flag names (identity)
		fmt.Fprintln(deps.Stderr, "sync: --produce-runtime: --story, --ac, --verdict, and --witness must all be given together (or none at all, for the scheduled no-op)")
		return 2
	}

	spec, err := storyresolve.Resolve(root, storyArg)
	if err != nil {
		fmt.Fprintln(deps.Stderr, "sync:", err)
		return 2
	}
	if !specDeclaresAC(spec, acID) {
		fmt.Fprintf(deps.Stderr, "sync: --produce-runtime: %s does not declare ac %q (dangling reference, 03 §Declarations: \"a misspelled ac-3 must never surface as a silent no-signal\")\n", spec.ID, acID)
		return 2
	}

	var ciInfo struct{ Pipeline, Job string }
	if deps.Forge != nil {
		info, err := deps.Forge.CIContext(ctx)
		if err != nil {
			fmt.Fprintln(deps.Stderr, "sync:", err)
			return 2
		}
		ciInfo.Pipeline, ciInfo.Job = info.Pipeline, info.Job
	}

	rec, err := runtimeprobe.Emit(runtimeprobe.ProbeInput{
		StoryRef:   spec.Story,
		ACID:       acID,
		Verdict:    verdict,
		Witness:    witness,
		Commit:     commit,
		Pipeline:   ciInfo.Pipeline,
		Job:        ciInfo.Job,
		InCI:       inCI,
		ForceLocal: forceLocal,
	})
	if err != nil {
		fmt.Fprintln(deps.Stderr, "sync:", err)
		return 2
	}

	if err := writeRuntimeRecord(root, commit, spec.ID, rec); err != nil {
		fmt.Fprintln(deps.Stderr, "sync:", err)
		return 2
	}

	fmt.Fprintf(deps.Stdout, "sync: produced runtime evidence record for %s %s (verdict=%s, source=%s)\n", spec.ID, acID, rec.Verdict, rec.Provenance.Source)
	return 0
}

// specDeclaresAC reports whether spec declares acID among its acceptance
// criteria — the same dangling-reference check verifySelfHostedACDeclared
// (selfevidence.go) applies, reimplemented here in terms of an
// already-resolved *artifact.SpecFrontmatter rather than a bare ref string,
// since storyresolve.Resolve (unlike selfevidence.go's store-root sidecar
// path) already hands this producer the decoded spec directly.
func specDeclaresAC(spec *artifact.SpecFrontmatter, acID string) bool {
	for _, ac := range spec.AcceptanceCriteria {
		if ac.ID == acID {
			return true
		}
	}
	return false
}

// writeRuntimeRecord merges rec into
// derived/<RefSlug(specRef)>/<commit>/runtime.json — dc-2's sibling file to
// verdicts.json — reusing writeSelfHostedEvidence's exact read-merge-write
// shape (readExistingEvidenceRecords, mergeEvidenceByProducer) rather than a
// copy: both are already fully generic over the target path/records, so one
// producer's helper serves both without duplication.
func writeRuntimeRecord(root, commit, specRef string, rec artifact.Evidence) error {
	dir := filepath.Join(store.DerivedSpecDir(root, store.RefSlug(specRef)), commit)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("runtime probe: mkdir %s: %w", dir, err)
	}
	path := filepath.Join(dir, "runtime.json")
	existing, err := readExistingEvidenceRecords(path)
	if err != nil {
		return fmt.Errorf("runtime probe: %w", err)
	}
	merged := mergeEvidenceByProducer(existing, []artifact.Evidence{rec})
	data, err := canonjson.Marshal(merged)
	if err != nil {
		return fmt.Errorf("runtime probe: marshaling %s: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("runtime probe: writing %s: %w", path, err)
	}
	return nil
}
