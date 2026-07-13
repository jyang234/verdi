// The design/build baseline regeneration shared by `verdi design start`
// (PLAN.md Phase 7, 05 §CLI: "regenerate impacted-service graphs/contracts
// into derived/ at the branch point, provenance: local") and `verdi
// feature start` ("refreshes the baseline into derived/"). Split into its
// own file because it is genuinely shared logic, not either verb's own
// entry point — design.go and feature.go both call regenerateBaseline.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/bundle"
	"github.com/jyang234/verdi/internal/store"
)

// regenerateBaseline computes branch's local baseline for spec's impacted
// services at commit and writes it to
// .verdi/data/derived/<ref-slug(branch)>/<commit>/ via internal/bundle
// (the same four-file shape `verdi sync --or-regen` writes, provenance
// local) — reusing regenerateServices, the exact producer path sync's own
// regeneration uses, scoped down to spec.Impacts instead of every
// discovered service.
//
// This baseline is advisory local convenience, not a gate (03 §Provenance
// classes: "source: local is advisory"), so every failure mode here is a
// graceful, disclosed skip rather than a hard error: no toolchain
// configured (deps.Runner == nil), no discovered service matches any
// declared impact, or the toolchain itself is unreachable (exec/network
// failure, a decode failure in its output — anything regenerateServices
// surfaces). Skipping never writes a partial or synthetic baseline (never
// fake success): either the whole four-file bundle is written, or nothing
// is. verb names the calling verb ("design start" / "feature start") for
// the disclosed message's prefix.
func regenerateBaseline(ctx context.Context, root, branch, commit string, spec *artifact.SpecFrontmatter, deps syncDeps, verb string, stderr io.Writer) {
	if deps.Runner == nil {
		fmt.Fprintf(stderr, "%s: no toolchain configured (verdi.yaml toolchain: block, I-4); skipping baseline regeneration\n", verb)
		return
	}

	services, err := store.DiscoverServices(root)
	if err != nil {
		fmt.Fprintf(stderr, "%s: skipping baseline regeneration: discovering services: %v\n", verb, err)
		return
	}
	impacted := store.FilterImpacted(services, spec.Impacts)
	if len(impacted) == 0 {
		fmt.Fprintf(stderr, "%s: spec %s declares no impacted service discoverable under this store; skipping baseline regeneration\n", verb, spec.ID)
		return
	}

	prov := artifact.EvidenceProvenance{Source: artifact.SourceLocal, Commit: commit}
	serviceBundles, merged, err := regenerateServices(ctx, root, commit, impacted, prov, deps)
	if err != nil {
		fmt.Fprintf(stderr, "%s: skipping baseline regeneration: toolchain unreachable: %v\n", verb, err)
		return
	}

	derivedDir := filepath.Join(root, ".verdi", "data", "derived", store.RefSlug(branch), commit)
	if err := os.MkdirAll(derivedDir, 0o755); err != nil {
		fmt.Fprintf(stderr, "%s: skipping baseline regeneration: %v\n", verb, err)
		return
	}
	if err := bundle.Assemble(derivedDir, serviceBundles, merged); err != nil {
		fmt.Fprintf(stderr, "%s: skipping baseline regeneration: %v\n", verb, err)
		return
	}
	fmt.Fprintf(stderr, "%s: regenerated local baseline for %v at %s\n", verb, impactedNames(impacted), derivedDir)
}

func impactedNames(services []store.Service) []string {
	names := make([]string, len(services))
	for i, svc := range services {
		names[i] = svc.Name
	}
	return names
}
