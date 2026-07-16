// verdi sync (05 §CLI, PLAN.md Phase 5): materializes a derived evidence
// bundle for the current (ref, commit) at
// .verdi/data/derived/<ref-slug>/<commit>/ — preferring the CI-pulled
// bundle through the configured forge port (I-8, I-22) whenever one
// exists, and falling back to local regeneration (source: local) only
// when --or-regen is passed and no CI bundle is available yet (05 §CLI:
// "regenerates locally when no bundle exists (fresh clone, no pipeline
// yet)"). Kept in its own file per PLAN.md's instruction so dispatch.go's
// diff for wiring this verb in stays a one-line handler change.
//
// --produce (spec/remote-and-ci dc-1) is the CI-provenance producer: it
// assembles the same four-file bundle but stamps provenance.source: ci
// instead of local, for the verdi-evidence workflow to upload. See
// runProduce below for the trust-boundary discipline this flag observes.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/forge"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/lint"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/upstream"
)

// derivedFileNames are the four files a materialized bundle must contain
// (01 §Directory layout's derived tree). toolchain.json
// (derivedToolchainFile) is deliberately NOT in this list: it is the
// bundle's OPTIONAL recorded-tool provenance carrier (spec/forge-transport
// ac-4/dc-4), written by bundle.Assemble only when an upstream tool
// actually ran (a non-empty Graph.Tool) — e.g. the self-hosted producer
// (selfevidence.go) runs no upstream tool and honestly carries none — so
// requiring it here would spuriously fail every honest tool-less bundle.
var derivedFileNames = []string{"verdicts.json", "tests.json", "review.json", "boundary-diff.json"}

// derivedToolchainFile is the bundle's recorded tool provenance carrier
// (upstream.ToolProvenance), checked against verdi.yaml's toolchain.commit
// at fetched-bundle intake — the I-4 secondary defense (ac-4/dc-4).
const derivedToolchainFile = "toolchain.json"

// syncDeps bundles sync's injectable dependencies so runSync can be driven
// hermetically in tests (CLAUDE.md: no network, no exec in any test);
// cmdSync wires the real ones.
type syncDeps struct {
	Runner upstream.Runner
	Forge  forge.Forge
	GoTest goTestRunner
	Stdout io.Writer
	Stderr io.Writer
}

// cmdSync is `verdi sync`'s real entry point, invoked by dispatch.go. It
// resolves the store root, manifest, current ref/commit, and forge from
// real git plumbing and verdi.yaml, then delegates to runSync.
func cmdSync(args []string, stdout, stderr io.Writer) int {
	ctx := context.Background()

	orRegen := false
	produce := false
	produceRuntime := false
	forceLocal := false
	var storyArg, acID, witnessArg string
	var verdictArg artifact.EvidenceVerdict

	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--or-regen":
			orRegen = true
		case "--produce":
			produce = true
		case "--produce-runtime":
			produceRuntime = true
		case "--force-local":
			forceLocal = true
		case "--story", "--ac", "--verdict", "--witness":
			i++
			if i >= len(args) {
				fmt.Fprintf(stderr, "sync: %s requires a value\n", a)
				return 2
			}
			switch a {
			case "--story":
				storyArg = args[i]
			case "--ac":
				acID = args[i]
			case "--verdict":
				verdictArg = artifact.EvidenceVerdict(args[i])
			case "--witness":
				witnessArg = args[i]
			}
		default:
			fmt.Fprintf(stderr, "sync: unknown argument %q\n", a)
			return 2
		}
	}

	modes := 0
	for _, m := range []bool{orRegen, produce, produceRuntime} {
		if m {
			modes++
		}
	}
	if modes > 1 {
		fmt.Fprintln(stderr, "sync: --or-regen, --produce, and --produce-runtime are mutually exclusive (--or-regen falls back to source: local; --produce always stamps source: ci for the verdi-evidence workflow, spec/remote-and-ci dc-1; --produce-runtime emits one kind: runtime record, spec/runtime-evidence dc-1)")
		return 2
	}
	if forceLocal && modes == 0 {
		fmt.Fprintln(stderr, "sync: --force-local only applies to --produce or --produce-runtime")
		return 2
	}
	if !produceRuntime && (storyArg != "" || acID != "" || witnessArg != "" || verdictArg != "") {
		fmt.Fprintln(stderr, "sync: --story/--ac/--verdict/--witness only apply to --produce-runtime")
		return 2
	}
	if produceRuntime && verdictArg != "" && verdictArg != artifact.VerdictPass && verdictArg != artifact.VerdictFail && verdictArg != artifact.VerdictAbstain {
		fmt.Fprintf(stderr, "sync: --verdict %q is not pass, fail, or abstain\n", verdictArg)
		return 2
	}

	root, err := store.FindRoot(".")
	if err != nil {
		fmt.Fprintln(stderr, "sync:", err)
		return 2
	}
	manifest, err := loadManifest(root)
	if err != nil {
		fmt.Fprintln(stderr, "sync:", err)
		return 2
	}
	ref, commit, err := resolveRefCommit(ctx, root)
	if err != nil {
		fmt.Fprintln(stderr, "sync:", err)
		return 2
	}

	remoteURL, _ := gitx.RemoteURL(ctx, root, "origin") // best-effort: only used for auto-detect
	forgeKind, err := forge.DetectKind(manifest.Forge, remoteURL)
	if err != nil {
		fmt.Fprintln(stderr, "sync:", err)
		return 2
	}
	// ADJ-43: the ac-1 identifier refusal belongs ONLY to invocations that
	// actually DIAL the forge by (owner, repo) — the fetch and --or-regen
	// paths (fetchAncestorBundle → FetchEvidenceBundle). --produce and
	// --produce-runtime never dial; they only read the CI environment
	// (CIContext, a pure env read that uses no repo identifier), so they
	// build an identifier-tolerant forge and run in an env-less, origin-less
	// checkout exactly as they did before ac-1 (co-3 byte-identity restored).
	// Dispatching the construction here — rather than after the toolchain
	// check below — keeps the identifier refusal ahead of that check for the
	// dialing path, unchanged (TestCmdSync_LocalCheckout_RefusesNamingSources).
	var fg forge.Forge
	if produce || produceRuntime {
		fg, err = buildForgeForCI(forgeKind, remoteURL)
	} else {
		fg, err = buildForge(forgeKind, remoteURL)
	}
	if err != nil {
		fmt.Fprintln(stderr, "sync:", err)
		return 2
	}

	if produceRuntime {
		deps := syncDeps{Forge: fg, Stdout: stdout, Stderr: stderr}
		return runProduceRuntime(ctx, root, commit, storyArg, acID, witnessArg, verdictArg, forceLocal, deps)
	}

	if manifest.Toolchain == nil {
		fmt.Fprintln(stderr, "sync: verdi.yaml has no toolchain: block (I-4)")
		return 2
	}
	runner := upstream.RealRunner{Module: manifest.Toolchain.Module, Commit: manifest.Toolchain.Commit, Dir: root}

	deps := syncDeps{Runner: runner, Forge: fg, GoTest: realGoTestRunner{}, Stdout: stdout, Stderr: stderr}
	return runSync(ctx, root, ref, commit, orRegen, produce, forceLocal, deps)
}

// runSync is the testable core: given an already-resolved root/ref/commit
// and injected deps, materialize the bundle and return the exit code.
func runSync(ctx context.Context, root, ref, commit string, orRegen, produce, forceLocal bool, deps syncDeps) int {
	derivedRoot := filepath.Join(root, ".verdi", "data", "derived")
	// --produce and --or-regen's local regeneration both assemble ONE
	// whole-branch per-service bundle keyed by the git ref (the transport
	// and gc unit, 01 §gc); the per-spec fold records CI ultimately serves
	// come from the self-hosted producer (selfevidence.go, keyed by spec
	// ref) written alongside it. The forge-PULL path, by contrast, writes
	// whatever keyed subdirs the fetched artifact carries verbatim (see
	// writeDerivedTree) — that is where a real CI run's per-spec records
	// land so the fold's readers reach them.
	derivedDir := filepath.Join(derivedRoot, store.RefSlug(ref), commit)

	// --produce never fetches — it IS the producer the verdi-evidence
	// workflow invokes to build the artifact a later `sync` fetches back
	// (spec/remote-and-ci dc-1); there is nothing to pull yet for the
	// commit that is producing it.
	if produce {
		if err := os.MkdirAll(derivedDir, 0o755); err != nil {
			fmt.Fprintln(deps.Stderr, "sync:", err)
			return 2
		}
		return runProduce(ctx, root, commit, derivedDir, forceLocal, deps)
	}

	// The CI bundle is authoritative and always preferred when available,
	// with or without --or-regen (05 §CLI: "--or-regen regenerates
	// locally when no bundle exists"). The fetch walks the current
	// commit's ancestry, nearest first, applying the fold's own ancestor
	// rule verbatim (spec/sync-local-flow ac-2/dc-1, sync_ancestor.go) —
	// a bundle at commit itself still wins first, never a stricter,
	// HEAD-exact-only demand the fold itself would not require.
	tree, acceptedCommit, distance, err := fetchAncestorBundle(ctx, root, deps.Forge, ref, commit)
	switch {
	case err == nil:
		// The I-4 secondary defense (spec/forge-transport ac-4/dc-4):
		// check the fetched bundle's recorded tool provenance against the
		// manifest pin BEFORE any of its records are accepted onto disk. A
		// mismatch is an operational refusal; an absent carrier is a
		// disclosed-unproven notice (checkFetchedToolPin prints it), never
		// a silent skip and never a spurious refusal of a pre-carrier or
		// tool-less (self-hosted producer) bundle.
		if pinErr := checkFetchedToolPin(root, tree, deps.Stdout); pinErr != nil {
			fmt.Fprintln(deps.Stderr, "sync:", pinErr)
			return 2
		}
		if writeErr := writeDerivedTree(derivedRoot, tree); writeErr != nil {
			fmt.Fprintln(deps.Stderr, "sync:", writeErr)
			return 2
		}
		// distance is the accepted commit's index in gitx.Log's own walk
		// order (ADJ-41 fix 2, disclosure only): dc-1 blesses gitx.Log as
		// the enumeration primitive, and across parallel branches of a
		// merged history that order is committer-date order, not graph
		// distance — so name it "in log order" rather than let the count
		// read as a graph-distance a user would verify against first-parent
		// intuition. No walk-semantics change.
		fmt.Fprintf(deps.Stdout, "sync: pulled CI evidence bundle (%d files) — accepted at commit %s, %d commit(s) back in log order from %s, into %s\n",
			len(tree), acceptedCommit, distance, commit, derivedRoot)
		return evaluateTree(deps, tree)

	case errors.Is(err, forge.ErrNoBundle) && orRegen:
		// ADJ-37 fix 1: an ancestry-enumeration failure reaches here
		// wrapped as no-bundle-shaped (so the routing is unchanged), but it
		// is NOT absence-evidence — the nearest-ancestor walk never ran, so
		// a bundle at a real ancestor may exist and was never consulted.
		// Disclose that (and why) before regenerating, rather than letting
		// --or-regen silently treat an unwalkable history as a genuine miss.
		if errors.Is(err, errAncestryUnwalkable) {
			fmt.Fprintf(deps.Stderr, "sync: %v — the nearest-ancestor bundle walk never ran, so --or-regen is regenerating locally without having consulted any ancestor's bundle (one may exist at a real ancestor this run never reached)\n", err)
		} else if errors.Is(err, errShallowTruncatedExhaustion) {
			// ADJ-41 fix 3: the walk ran but exhausted only a shallow clone's
			// truncated graph — not genuine absence. Disclose the truncation
			// before regenerating; a bundle may exist at a deeper true
			// ancestor this clone never contained. A full clone's plain
			// absence takes neither branch and stays byte-quiet as today.
			fmt.Fprintf(deps.Stderr, "sync: %v — the walk exhausted only this shallow clone's truncated history, so --or-regen is regenerating locally without having consulted any bundle that may exist at a deeper true ancestor absent from this clone\n", err)
		}
		if err := os.MkdirAll(derivedDir, 0o755); err != nil {
			fmt.Fprintln(deps.Stderr, "sync:", err)
			return 2
		}
		prov := artifact.EvidenceProvenance{Source: artifact.SourceLocal, Commit: commit}
		if regenErr := regenerate(ctx, root, commit, derivedDir, prov, deps); regenErr != nil {
			fmt.Fprintln(deps.Stderr, "sync:", regenErr)
			return 2
		}
		fmt.Fprintf(deps.Stdout, "sync: regenerated evidence bundle locally at %s\n", derivedDir)
		return evaluateBundle(deps, derivedDir)

	case errors.Is(err, forge.ErrNoBundle):
		fmt.Fprintf(deps.Stderr, "sync: %v; pass --or-regen to regenerate locally\n", err)
		return 2

	default:
		fmt.Fprintln(deps.Stderr, "sync:", err)
		return 2
	}
}

// runProduce implements `verdi sync --produce` (spec/remote-and-ci dc-1):
// the CI-provenance producer, intended to be invoked only by CI, strictly
// after `make verify` has already succeeded in the same job (round 6,
// spec/close-verb ac-3/dc-1: `.github/workflows/verify.yml` — see its own
// header comment for why the formerly-separate verdi-evidence.yml workflow
// was folded into this one job rather than left standalone). It assembles
// the derived bundle exactly like --or-regen's regeneration
// path (regenerate/regenerateServices, internal/bundle), pulling
// Pipeline/Job identifiers from the forge's CIContext when present (03
// §Evidence records). Whether the records are stamped source:ci or
// source:local is decided below by whether this is a genuine CI run.
//
// This invocation itself never makes a bundle authoritative: dc-1 —
// "its output is authoritative solely because it is fetched back from
// the forge artifact store by (ref, commit) — not because the flag was
// passed." Every gate (cmd/verdi/gate.go, matrix.go, closuregate.go,
// rollup.go, via internal/evidence.LoadRecords/Fold) reads whatever
// provenance.source a record on disk claims, from whatever wrote it —
// the fold does not and structurally cannot verify provenance
// cryptographically. So the trust boundary is enforced at the STAMP, not
// the read: --produce refuses to run outside a detected CI environment
// (internal/lint.ReadCIEnv) unless --force-local overrides it, and a
// --force-local (or otherwise non-CI) run stamps source:local, NOT
// source:ci (true-closure) — mirroring rollup's --force-local precedent
// (I-32) and printing a disclosed, non-authoritative warning. A source:ci
// record therefore can only originate in a genuine CI run, and the only
// path to one on a developer's disk is exactly what dc-1 describes: a real
// verdi-evidence run, fetched back through the forge port by `verdi sync`.
func runProduce(ctx context.Context, root, commit, derivedDir string, forceLocal bool, deps syncDeps) int {
	inCI := lint.ReadCIEnv().InCI
	if !inCI && !forceLocal {
		fmt.Fprintln(deps.Stderr, "sync: --produce refuses to run outside CI (spec/remote-and-ci dc-1: this producer is authoritative only once the verdi-evidence workflow's uploaded artifact is fetched back through the forge); pass --force-local for local testing only")
		return 2
	}

	// A source:ci stamp is now applied ONLY in a genuine, non-overridden CI
	// run (03 §Provenance classes; true-closure). --force-local (or any
	// non-CI run this line is reachable from) stamps source:local instead —
	// structurally non-authoritative, folded only under --preview — so no
	// local invocation can emit a source:ci record that a gate would trust.
	// The ONLY path to an authoritative source:ci record on a developer's
	// disk is therefore a real verdi-evidence run's artifact fetched back
	// through the forge port by `verdi sync`.
	authoritative := inCI && !forceLocal
	if !authoritative {
		fmt.Fprintln(deps.Stderr, "sync: --force-local: producing a NON-AUTHORITATIVE evidence bundle stamped source:local (only a genuine CI run, without --force-local, may stamp source:ci; a gate folds source:local records only under --preview)")
	}

	ciInfo, err := deps.Forge.CIContext(ctx)
	if err != nil {
		fmt.Fprintln(deps.Stderr, "sync:", err)
		return 2
	}
	source := artifact.SourceLocal
	if authoritative {
		source = artifact.SourceCI
	}
	prov := artifact.EvidenceProvenance{Source: source, Commit: commit, Pipeline: ciInfo.Pipeline, Job: ciInfo.Job}

	if regenErr := regenerate(ctx, root, commit, derivedDir, prov, deps); regenErr != nil {
		fmt.Fprintln(deps.Stderr, "sync:", regenErr)
		return 2
	}

	// The self-hosted evidence producer (spec/close-verb ac-3, dc-1;
	// selfevidence.go): verdi is not a flowmap service of itself (D6-4), so
	// regenerate() above always assembles an empty bundle for THIS repo.
	// Reaching this line means make verify already succeeded earlier in
	// THIS SAME CI job (see verify.yml's wiring comment) — the honest basis
	// for the pass records this step binds, never a divergent re-run. A
	// store with no root verdi.bindings.yaml yet is a silent no-op.
	if err := produceSelfHostedEvidence(root, commit, prov); err != nil {
		fmt.Fprintln(deps.Stderr, "sync:", err)
		return 2
	}

	fmt.Fprintf(deps.Stdout, "sync: produced CI evidence bundle at %s\n", derivedDir)
	return evaluateBundle(deps, derivedDir)
}

// writeDerivedTree writes each entry of a CI-pulled DerivedTree to disk
// under derivedRoot (.verdi/data/derived), preserving the artifact's
// internal per-spec/per-ref keys verbatim. The CI job that produced these
// bytes already ran the same assembly (internal/bundle / selfevidence.go)
// with provenance source: ci, so the content is already canonical; what
// matters here is WHERE it lands — a per-spec record keyed
// spec--<name>/<commit>/verdicts.json must be written under exactly that
// key so the fold's readers (RefSlug(spec.id)) reach it. Every key is
// re-validated against directory traversal at this disk-write boundary,
// independent of whichever forge adapter produced it.
func writeDerivedTree(derivedRoot string, tree forge.DerivedTree) error {
	keys := make([]string, 0, len(tree))
	for key := range tree {
		keys = append(keys, key)
	}
	sort.Strings(keys) // deterministic write order (aids test/debug legibility)
	for _, key := range keys {
		dest, err := safeJoin(derivedRoot, key)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return fmt.Errorf("creating %s: %w", filepath.Dir(dest), err)
		}
		if err := os.WriteFile(dest, tree[key], 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", dest, err)
		}
	}
	return nil
}

// checkFetchedToolPin is the I-4 secondary defense at fetched-bundle
// intake (spec/forge-transport ac-4/dc-4; adjudicated 2026-07-13: the
// carrier is toolchain.json, since no other bundle artifact records the
// tool): every toolchain.json in the fetched tree is strict-decoded and
// its recorded tool checked with upstream.CheckToolPin against verdi.yaml's
// toolchain.commit. A mismatch (or a malformed carrier, or a carrier
// present with no pin configured) returns an error — the caller's
// operational refusal (exit 2) — with CheckToolPin's own message naming
// both the recorded tool string and the pinned commit. A tree with NO
// carrier anywhere prints a one-line disclosed-unproven notice on stdout
// and returns nil: a pre-carrier bundle, or one whose producer ran no
// upstream tool (the self-hosted producer, selfevidence.go), is honestly
// tool-less — disclosed, never silently passed and never spuriously
// refused. The manifest is read only when a carrier is present, so
// tool-less intake works even in a store without a toolchain: block.
func checkFetchedToolPin(root string, tree forge.DerivedTree, stdout io.Writer) error {
	var keys []string
	for key := range tree {
		if filepath.Base(key) == derivedToolchainFile {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		fmt.Fprintln(stdout, "sync: fetched bundle carries no toolchain.json (a pre-carrier bundle, or its producer ran no upstream tool); the I-4 tool-pin check is disclosed-unproven for this intake")
		return nil
	}
	sort.Strings(keys) // deterministic check/refusal order

	manifest, err := loadManifest(root)
	if err != nil {
		return err
	}
	pinned := ""
	if manifest.Toolchain != nil {
		pinned = manifest.Toolchain.Commit
	}
	for _, key := range keys {
		prov, err := upstream.DecodeToolProvenance(tree[key])
		if err != nil {
			return fmt.Errorf("fetched bundle %s: %w", key, err)
		}
		if err := upstream.CheckToolPin(prov.Tool, pinned); err != nil {
			return fmt.Errorf("refusing fetched evidence bundle (%s): %w", key, err)
		}
	}
	return nil
}

// safeJoin joins a fetched tree key onto root, refusing any key that would
// escape root — defense in depth against a malicious or malformed artifact
// at the real trust boundary (the disk write), even though the zip
// extractor already rejects traversal.
func safeJoin(root, key string) (string, error) {
	dest := filepath.Join(root, filepath.FromSlash(key))
	rel, err := filepath.Rel(root, dest)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("sync: refusing derived tree key %q: escapes %s", key, root)
	}
	return dest, nil
}

// evaluateTree maps a just-materialized fetched tree to sync's exit code: 1
// if any verdicts.json record verdicts fail or any review.json entry BLOCKs
// (surfacing failing evidence sync just materialized, matching the exit-code
// contract's "1 = verdict failure"), 0 otherwise. A file that does not even
// decode is an operational error (2). EVERY verdicts.json/review.json across
// every key in the tree is checked, not just one bundle's — the tree spans
// per-spec subdirs.
func evaluateTree(deps syncDeps, tree forge.DerivedTree) int {
	keys := make([]string, 0, len(tree))
	for key := range tree {
		keys = append(keys, key)
	}
	sort.Strings(keys) // deterministic evaluation/reporting order
	for _, key := range keys {
		switch filepath.Base(key) {
		case "verdicts.json":
			var records []artifact.Evidence
			if err := artifact.DecodeStrictJSON(tree[key], &records); err != nil {
				fmt.Fprintf(deps.Stderr, "sync: decoding materialized %s: %v\n", key, err)
				return 2
			}
			for _, r := range records {
				if r.Verdict == artifact.VerdictFail {
					fmt.Fprintln(deps.Stdout, "sync: materialized bundle contains failing evidence")
					return 1
				}
			}
		case "review.json":
			var reviews []*upstream.Review
			if err := artifact.DecodeStrictJSON(tree[key], &reviews); err != nil {
				fmt.Fprintf(deps.Stderr, "sync: decoding materialized %s: %v\n", key, err)
				return 2
			}
			for _, r := range reviews {
				if r != nil && r.Blocking() {
					fmt.Fprintln(deps.Stdout, "sync: materialized review contains a BLOCK verdict")
					return 1
				}
			}
		}
	}
	return 0
}

// evaluateBundle reads the just-materialized bundle back and maps it to
// sync's own exit code: 1 if any evidence record verdicts to fail or any
// review verdicts BLOCK (sync surfaces failing evidence it just
// materialized, matching the exit-code contract's "1 = verdict failure"
// for the honest case where regeneration or a pulled bundle both
// discovered a real problem), 0 otherwise. Decode failures are operational
// errors (2) — a bundle that doesn't even decode is worse than "failing."
func evaluateBundle(deps syncDeps, dir string) int {
	var records []artifact.Evidence
	if err := decodeBundleFile(dir, "verdicts.json", &records); err != nil {
		fmt.Fprintln(deps.Stderr, "sync:", err)
		return 2
	}
	for _, r := range records {
		if r.Verdict == artifact.VerdictFail {
			fmt.Fprintln(deps.Stdout, "sync: materialized bundle contains failing evidence")
			return 1
		}
	}

	var reviews []*upstream.Review
	if err := decodeBundleFile(dir, "review.json", &reviews); err != nil {
		fmt.Fprintln(deps.Stderr, "sync:", err)
		return 2
	}
	for _, r := range reviews {
		if r != nil && r.Blocking() {
			fmt.Fprintln(deps.Stdout, "sync: materialized review contains a BLOCK verdict")
			return 1
		}
	}
	return 0
}

func decodeBundleFile(dir, name string, out interface{}) error {
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return fmt.Errorf("reading %s: %w", name, err)
	}
	if err := artifact.DecodeStrictJSON(data, out); err != nil {
		return fmt.Errorf("decoding materialized %s: %w", name, err)
	}
	return nil
}
