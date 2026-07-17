// D6-23 (round6-divergences.md, folded into Wave T's fix/accept-lints-quartet):
// `verdi accept` ran no lint over the quartet it was about to freeze — the
// round-6 witness was a dangling layout.json positions key (VL-018 class)
// that sailed through accept and was only caught by CI's spec-gate, after
// push. lintQuartetOrRefuse closes that gap: called from runAccept (accept.go)
// before it flips status or writes the frozen stamp, it runs the SAME store
// lint `verdi lint` runs (internal/lint.NewEngine().Run — no forked rule
// logic, per this file's own mandate) and refuses (exit 1, naming every
// violation verbatim) if any SeverityViolation finding falls within the
// spec's own quartet scope. A SeverityDisclosure finding (e.g. the standing
// VL-017 "mutable zone absent" notice) never refuses — the same severity
// split `verdi lint` itself already draws (lint.go: "a run whose only
// findings are disclosures still exits 0").
//
// Scoping cost, disclosed (per this task's own escalation clause): internal/
// lint's Engine has no per-directory scan mode — BuildSnapshot always walks
// the whole .verdi/ store, because several rules are inherently store-wide
// (VL-007's top-level-entries check, VL-016's cross-directory duplicate-name
// check, VL-003's cross-spec link resolution, and others a single spec's
// own files cannot answer in isolation). lintQuartetOrRefuse therefore still
// runs a whole-store lint pass; what is scoped to the spec being accepted is
// which FINDINGS gate the accept, not the underlying computation. A
// pre-existing violation in some OTHER spec elsewhere in the store is
// deliberately not surfaced here — it is not part of the quartet being
// frozen, exactly the posture a separate `verdi lint` run would report on
// its own.
package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/lint"
	"github.com/jyang234/verdi/internal/store"
)

// lintQuartetOrRefuse runs the store lint and refuses acceptance of ref
// (exit 1, verdict discipline — a lint refusal is a verdict, not an
// operational error) if any violation's path falls within ref's own quartet
// scope (quartetPathPrefixes). Returns 0 when the quartet is clean, whether
// or not the whole store carries unrelated findings elsewhere; returns 2 on
// the engine's own operational failure (root unreadable, service discovery
// failed — never a per-artifact content problem, which is always a finding
// instead, per internal/lint.Engine.Run's own contract).
func lintQuartetOrRefuse(ctx context.Context, root string, ref artifact.Ref, spec *artifact.SpecFrontmatter, stderr io.Writer) int {
	lctx := lint.BuildContext(ctx, root)
	findings, err := lint.NewEngine().Run(ctx, root, lctx, lint.Options{})
	if err != nil {
		fmt.Fprintf(stderr, "accept: running the store lint: %v\n", err)
		return 2
	}

	prefixes := quartetPathPrefixes(ref, spec)

	var violations []lint.Finding
	for _, f := range findings {
		if f.Severity == lint.SeverityDisclosure {
			continue
		}
		if inQuartetScope(f.Path, prefixes) {
			violations = append(violations, f)
		}
	}
	if len(violations) == 0 {
		return 0
	}

	fmt.Fprintf(stderr, "accept: refusing to freeze %s — the store lint finds %d violation(s) in its quartet:\n", ref.String(), len(violations))
	for _, f := range violations {
		fmt.Fprintln(stderr, f.String())
	}
	return 1
}

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
