// D6-23 (round6-divergences.md, folded into Wave T's fix/accept-lints-quartet):
// `verdi accept` used to run no lint over the quartet it was about to
// freeze — the round-6 witness was a dangling layout.json positions key
// (VL-018 class) that sailed through accept and was only caught by CI's
// spec-gate, after push. The gate this file used to close
// (lintQuartetOrRefuse, called from runAccept before it flipped status or
// wrote the frozen stamp) is retired along with the rest of accept's
// mutation by Task 7 (docs/superpowers/specs/2026-08-01-merge-signals-
// spec-acceptance-design.md) — `verdi accept` no longer lints, flips
// status, or writes a frozen stamp at all, so there is nothing left for a
// quartet gate to run before. The design's own "Pull-request gates"
// section names the pre-merge required check (out of this task's scope)
// as this predicate's next home.
//
// The pure quartet-scoping predicates below (quartetPathPrefixes,
// inQuartetScope) are kept, with their own direct unit tests, as the
// reusable building block a future pre-merge-gate wiring would reach for
// — they carry no dependency on accept's own retired ritual.
package main

import (
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/store"
)

// quartetPathPrefixes lists the store-relative path prefixes making up
// ref's own quartet (D6-23): its spec directory under specs/active/ (which
// holds spec.md, layout.json, and decision-conflict-report.md — all three
// sit directly under the one directory, per 01 §Directory layout) and, when
// spec carries a story: tracker ref, its attestations directory. The
// attestations directory is keyed by store.RefSlug(spec.Story) — the SAME
// derivation cmd/verdi/foldload.go's evidence fold already uses for
// waivers/attestations, not a new one invented here — never by the spec's
// own name, since a spec's directory name and its tracker ref's slug are
// independent (02 §Kind registry). A feature spec with no story: ref (05
// §CLI: "features may carry no story: at all") gets no attestations prefix
// at all, rather than one that would wrongly match every attestation in the
// store.
func quartetPathPrefixes(ref artifact.Ref, spec *artifact.SpecFrontmatter) []string {
	prefixes := []string{store.SpecDirRelPath(store.ZoneActive, ref.Name)}
	if spec.Story != "" {
		prefixes = append(prefixes, store.AttestationDirRelPath(store.RefSlug(spec.Story)))
	}
	return prefixes
}

// inQuartetScope reports whether path equals, or sits under, any of
// prefixes — a slash-boundary-aware prefix match, so
// "specs/active/quartet-lint-2" never matches the "specs/active/quartet-lint"
// prefix.
func inQuartetScope(path string, prefixes []string) bool {
	for _, p := range prefixes {
		if path == p || strings.HasPrefix(path, p+"/") {
			return true
		}
	}
	return false
}
