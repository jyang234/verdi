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

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/forge"
	forgegithub "github.com/jyang234/verdi/internal/forge/github"
	forgegitlab "github.com/jyang234/verdi/internal/forge/gitlab"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/lint"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/upstream"
)

// derivedFileNames are the four files a materialized bundle must contain
// (01 §Directory layout's derived tree).
var derivedFileNames = []string{"verdicts.json", "tests.json", "review.json", "boundary-diff.json"}

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
	forceLocal := false
	for _, a := range args {
		switch a {
		case "--or-regen":
			orRegen = true
		case "--produce":
			produce = true
		case "--force-local":
			forceLocal = true
		default:
			fmt.Fprintf(stderr, "sync: unknown argument %q\n", a)
			return 2
		}
	}
	if orRegen && produce {
		fmt.Fprintln(stderr, "sync: --or-regen and --produce are mutually exclusive (--or-regen falls back to source: local; --produce always stamps source: ci for the verdi-evidence workflow, spec/remote-and-ci dc-1)")
		return 2
	}
	if forceLocal && !produce {
		fmt.Fprintln(stderr, "sync: --force-local only applies to --produce")
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
	fg, err := buildForge(forgeKind)
	if err != nil {
		fmt.Fprintln(stderr, "sync:", err)
		return 2
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
	derivedDir := filepath.Join(root, ".verdi", "data", "derived", store.RefSlug(ref), commit)
	if err := os.MkdirAll(derivedDir, 0o755); err != nil {
		fmt.Fprintln(deps.Stderr, "sync:", err)
		return 2
	}

	// --produce never fetches — it IS the producer the verdi-evidence
	// workflow invokes to build the artifact a later `sync` fetches back
	// (spec/remote-and-ci dc-1); there is nothing to pull yet for the
	// commit that is producing it.
	if produce {
		return runProduce(ctx, root, commit, derivedDir, forceLocal, deps)
	}

	// The CI bundle is authoritative and always preferred when available,
	// with or without --or-regen (05 §CLI: "--or-regen regenerates
	// locally when no bundle exists").
	b, err := deps.Forge.FetchEvidenceBundle(ctx, ref, commit)
	switch {
	case err == nil:
		if writeErr := writeRawBundle(derivedDir, b); writeErr != nil {
			fmt.Fprintln(deps.Stderr, "sync:", writeErr)
			return 2
		}
		fmt.Fprintf(deps.Stdout, "sync: pulled CI evidence bundle to %s\n", derivedDir)
		return evaluateBundle(deps, derivedDir)

	case errors.Is(err, forge.ErrNoBundle) && orRegen:
		prov := artifact.EvidenceProvenance{Source: artifact.SourceLocal, Commit: commit}
		if regenErr := regenerate(ctx, root, commit, derivedDir, prov, deps); regenErr != nil {
			fmt.Fprintln(deps.Stderr, "sync:", regenErr)
			return 2
		}
		fmt.Fprintf(deps.Stdout, "sync: regenerated evidence bundle locally at %s\n", derivedDir)
		return evaluateBundle(deps, derivedDir)

	case errors.Is(err, forge.ErrNoBundle):
		fmt.Fprintln(deps.Stderr, "sync: no CI evidence bundle for this ref/commit yet; pass --or-regen to regenerate locally")
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
// path (regenerate/regenerateServices, internal/bundle), but stamps
// provenance.source: ci instead of local, pulling Pipeline/Job
// identifiers from the forge's CIContext when present (03 §Evidence
// records).
//
// This invocation itself never makes a bundle authoritative: dc-1 —
// "its output is authoritative solely because it is fetched back from
// the forge artifact store by (ref, commit) — not because the flag was
// passed." Every gate (cmd/verdi/gate.go, matrix.go, closuregate.go,
// rollup.go, via internal/evidence.LoadRecords/Fold) reads whatever
// provenance.source a record on disk claims, from whatever wrote it —
// the fold does not and structurally cannot verify provenance
// cryptographically. What keeps a locally produced bundle from quietly
// passing as the real thing is discipline, not a lock: --produce refuses
// to run outside a detected CI environment (internal/lint.ReadCIEnv)
// unless --force-local overrides it, mirroring rollup's --force-local
// precedent (I-32) and printing the same class of disclosed,
// non-authoritative warning when it does — so a locally produced
// source:ci bundle is always a deliberate, visible act, and the ordinary
// path to an authoritative bundle stays exactly what dc-1 describes: a
// real verdi-evidence run, fetched back through the forge port.
func runProduce(ctx context.Context, root, commit, derivedDir string, forceLocal bool, deps syncDeps) int {
	inCI := lint.ReadCIEnv().InCI
	if !inCI && !forceLocal {
		fmt.Fprintln(deps.Stderr, "sync: --produce refuses to run outside CI (spec/remote-and-ci dc-1: this producer is authoritative only once the verdi-evidence workflow's uploaded artifact is fetched back through the forge); pass --force-local for local testing only")
		return 2
	}
	if !inCI {
		fmt.Fprintln(deps.Stderr, "sync: --force-local: producing a source:ci-stamped bundle outside CI; this escape hatch is for local testing only and is NON-AUTHORITATIVE (spec/remote-and-ci dc-1: a local --produce bundle is never fetched by a gate)")
	}

	ciInfo, err := deps.Forge.CIContext(ctx)
	if err != nil {
		fmt.Fprintln(deps.Stderr, "sync:", err)
		return 2
	}
	prov := artifact.EvidenceProvenance{Source: artifact.SourceCI, Commit: commit, Pipeline: ciInfo.Pipeline, Job: ciInfo.Job}

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

// writeRawBundle writes a CI-pulled bundle's four files verbatim: the CI
// job that produced them already ran this same assembly (internal/bundle)
// with provenance source: ci, so these bytes are already canonical.
func writeRawBundle(dir string, b *forge.EvidenceBundle) error {
	files := map[string][]byte{
		"verdicts.json":      b.Verdicts,
		"tests.json":         b.Tests,
		"review.json":        b.Review,
		"boundary-diff.json": b.BoundaryDiff,
	}
	for _, name := range derivedFileNames {
		if err := os.WriteFile(filepath.Join(dir, name), files[name], 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", name, err)
		}
	}
	return nil
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

// loadManifest reads and strict-decodes root's verdi.yaml.
func loadManifest(root string) (*store.Manifest, error) {
	data, err := os.ReadFile(filepath.Join(root, ".verdi", "verdi.yaml"))
	if err != nil {
		return nil, fmt.Errorf("reading verdi.yaml: %w", err)
	}
	m, err := store.DecodeManifest(data)
	if err != nil {
		return nil, fmt.Errorf("decoding verdi.yaml: %w", err)
	}
	return m, nil
}

// resolveRefCommit determines the current ref and commit sync operates
// on. Ref resolution prefers forge-provided CI environment variables
// (GitLab's CI_COMMIT_REF_NAME, GitHub's GITHUB_HEAD_REF for a PR run or
// GITHUB_REF_NAME for a push) over `git symbolic-ref`, since CI checkouts
// are usually detached HEAD, where symbolic-ref fails.
func resolveRefCommit(ctx context.Context, root string) (ref, commit string, err error) {
	commit, err = gitx.RevParse(ctx, root, "HEAD")
	if err != nil {
		return "", "", fmt.Errorf("resolving current commit: %w", err)
	}

	for _, envVar := range []string{"CI_COMMIT_REF_NAME", "GITHUB_HEAD_REF", "GITHUB_REF_NAME"} {
		if v := os.Getenv(envVar); v != "" {
			return v, commit, nil
		}
	}
	branch, err := gitx.CurrentBranch(ctx, root)
	if err != nil {
		return "", "", fmt.Errorf("resolving current ref: %w", err)
	}
	// CurrentBranch returns ("", nil) for a detached HEAD (a normal git
	// state lint tolerates — I-14); sync, unlike lint, cannot proceed
	// without a ref name to slug, so absence is an operational error here.
	if branch == "" {
		return "", "", fmt.Errorf("resolving current ref: detached HEAD and no CI ref env var set (CI_COMMIT_REF_NAME / GITHUB_REF_NAME)")
	}
	return branch, commit, nil
}

// buildForge constructs the real adapter for kind ("gitlab" or "github"),
// reading connection details from CI-provided environment variables
// (never verdi.yaml — 01 §Store manifest: "secrets come from env/CI
// vars").
func buildForge(kind string) (forge.Forge, error) {
	switch kind {
	case "gitlab":
		return forgegitlab.New(forgegitlab.Config{
			BaseURL:   os.Getenv("CI_API_V4_URL"),
			ProjectID: os.Getenv("CI_PROJECT_ID"),
			Token:     os.Getenv("CI_JOB_TOKEN"),
		}), nil
	case "github":
		return forgegithub.New(forgegithub.Config{
			Owner: os.Getenv("GITHUB_REPOSITORY_OWNER"),
			Repo:  githubRepoName(),
			Token: os.Getenv("GITHUB_TOKEN"),
		}), nil
	default:
		return nil, fmt.Errorf("unknown forge kind %q", kind)
	}
}

// githubRepoName extracts the repo name from GITHUB_REPOSITORY
// ("owner/repo"), GitHub Actions' own combined env var.
func githubRepoName() string {
	full := os.Getenv("GITHUB_REPOSITORY")
	for i := len(full) - 1; i >= 0; i-- {
		if full[i] == '/' {
			return full[i+1:]
		}
	}
	return full
}
