// Local-regeneration path for `verdi sync --or-regen` (PLAN.md Phase 5):
// exec the pinned toolchain against every discovered service and assemble
// a bundle with provenance source: local. Split from sync.go per SRP —
// sync.go owns the verb's entry point and the CI-pull path; this file owns
// only the "no CI bundle yet" fallback.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/bundle"
	"github.com/OWNER/verdi/internal/store"
	"github.com/OWNER/verdi/internal/upstream"
)

// goTestRunner abstracts `go test -json` execution so tests can supply
// canned output instead of really building and running a Go module
// (CLAUDE.md: no exec in any test).
type goTestRunner interface {
	// RunGoTest runs the test suite for the Go module rooted at dir and
	// returns its `go test -json` stdout. A failing test suite is not an
	// error here — go test's own nonzero exit on test failure is
	// expected and the JSON stream itself (which SummarizeGoTestJSON
	// reads) is the useful signal; only a truly broken invocation (no
	// output at all) is an error.
	RunGoTest(ctx context.Context, dir string) ([]byte, error)
}

// realGoTestRunner execs the real `go test -json ./...` — never used by
// this module's own tests.
type realGoTestRunner struct{}

func (realGoTestRunner) RunGoTest(ctx context.Context, dir string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "go", "test", "-json", "./...")
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	_ = cmd.Run() // a failing suite exits nonzero; the JSON stream is what matters
	if stdout.Len() == 0 {
		return nil, fmt.Errorf("go test -json produced no output in %s: %s", dir, stderr.String())
	}
	return stdout.Bytes(), nil
}

// regenerate execs the pinned toolchain against every discovered service
// (PLAN.md Phase 5 stub: "impacted-service scoping is exact-match on
// impacts: (widen-on-demand later)" — v0 has no story context to scope
// sync's regeneration to, so every discovered service is regenerated) and
// assembles the four bundle files into derivedDir.
func regenerate(ctx context.Context, root, commit, derivedDir string, deps syncDeps) error {
	services, err := store.DiscoverServices(root)
	if err != nil {
		return fmt.Errorf("discovering services: %w", err)
	}
	if len(services) == 0 {
		return fmt.Errorf("no services discovered under %s (nothing to regenerate)", root)
	}

	prov := artifact.EvidenceProvenance{Source: artifact.SourceLocal, Commit: commit}

	var serviceBundles []bundle.ServiceBundle
	var testSummaries []*bundle.TestSummary
	for _, svc := range services {
		sb, ts, err := regenerateService(ctx, root, svc, commit, prov, deps)
		if err != nil {
			return fmt.Errorf("service %s: %w", svc.Name, err)
		}
		serviceBundles = append(serviceBundles, sb)
		if ts != nil {
			testSummaries = append(testSummaries, ts)
		}
	}

	merged := bundle.MergeTestSummaries(testSummaries)
	if err := bundle.Assemble(derivedDir, serviceBundles, merged); err != nil {
		return fmt.Errorf("assembling bundle: %w", err)
	}
	return nil
}

// regenerateService runs the toolchain against one service and returns
// its ServiceBundle contribution plus its test summary, if any (nil if
// the service has no bindings — nothing behavioral to summarize for it).
//
// review.json's base and branch graphs are the same freshly-generated
// graph: `sync --or-regen` has no story/baseline context to draw a
// meaningful base graph from (that comes from `design start`/`feature
// start`, PLAN.md phase 7, which establish a real baseline at a branch
// point) — a disclosed phase-5 scoping choice, not a silent guess. The
// resulting review verdict is still real toolchain output, exercising the
// full exec + strict-decode path even though it is trivially clean.
func regenerateService(ctx context.Context, root string, svc store.Service, commit string, prov artifact.EvidenceProvenance, deps syncDeps) (bundle.ServiceBundle, *bundle.TestSummary, error) {
	sb := bundle.ServiceBundle{ServiceName: svc.Name}

	graph, err := upstream.RunGraph(ctx, deps.Runner, svc.Dir, commit)
	if err != nil {
		return sb, nil, fmt.Errorf("flowmap graph: %w", err)
	}

	if svc.BoundaryContractPath != "" {
		baseRaw, err := os.ReadFile(svc.BoundaryContractPath)
		if err != nil {
			return sb, nil, fmt.Errorf("reading boundary contract %s: %w", svc.BoundaryContractPath, err)
		}
		baseContract, err := upstream.DecodeBoundaryContract(baseRaw)
		if err != nil {
			return sb, nil, fmt.Errorf("decoding boundary contract %s: %w", svc.BoundaryContractPath, err)
		}
		// Preserve the pre-regeneration bytes on disk under their own
		// path: BoundaryGenerate overwrites svc.BoundaryContractPath in
		// place (spike S1: flowmap boundary always writes there), and
		// CrossCheckDiff needs both a base and a branch file path to hand
		// to `groundwork diff`.
		baseTmp, baseCleanup, err := writeTempFile(baseRaw, "verdi-sync-base-contract-*.json")
		if err != nil {
			return sb, nil, fmt.Errorf("writing scratch base contract: %w", err)
		}
		defer baseCleanup()

		if err := upstream.BoundaryGenerate(ctx, deps.Runner, svc.Dir); err != nil {
			return sb, nil, fmt.Errorf("flowmap boundary: %w", err)
		}
		branchContract, err := readBoundaryContract(svc.BoundaryContractPath)
		if err != nil {
			return sb, nil, err
		}
		sb.BoundaryDiff = upstream.ComputeBoundaryDiff(baseContract, branchContract)

		// I-3: cross-check verdi's own computed breaking verdict against
		// `groundwork diff`'s exit code — a disagreement is a hard error,
		// never silently ignored.
		if err := upstream.CrossCheckDiff(ctx, deps.Runner, baseTmp, svc.BoundaryContractPath, upstream.HasBreaking(sb.BoundaryDiff)); err != nil {
			return sb, nil, fmt.Errorf("cross-checking boundary diff: %w", err)
		}
	}

	policyPath := filepath.Join(svc.Dir, "policy.json")
	if fileExists(policyPath) {
		graphPath, cleanup, err := writeTempGraph(graph)
		if err != nil {
			return sb, nil, fmt.Errorf("writing scratch graph for review: %w", err)
		}
		defer cleanup()

		review, err := upstream.RunReview(ctx, deps.Runner, policyPath, graphPath, graphPath, commit)
		if err != nil {
			return sb, nil, fmt.Errorf("groundwork review: %w", err)
		}
		sb.Review = review
	}

	var testSummary *bundle.TestSummary
	if svc.Bindings != nil {
		specACs, err := loadSpecACs(root, svc.Bindings.Spec)
		if err != nil {
			return sb, nil, err
		}
		goldenFlows, err := listGoldenFlows(svc.Dir)
		if err != nil {
			return sb, nil, err
		}
		out, err := deps.GoTest.RunGoTest(ctx, svc.Dir)
		if err != nil {
			return sb, nil, fmt.Errorf("go test -json: %w", err)
		}
		testSummary, err = bundle.SummarizeGoTestJSON(bytes.NewReader(out))
		if err != nil {
			return sb, nil, fmt.Errorf("summarizing go test -json: %w", err)
		}

		recs, err := bundle.BuildVerdicts(bundle.JoinInput{
			ServiceName:      svc.Name,
			Graph:            graph,
			Bindings:         svc.Bindings,
			KnownGoldenFlows: goldenFlows,
			SpecACs:          specACs,
			TestSummary:      testSummary,
			Provenance:       prov,
		})
		if err != nil {
			return sb, nil, fmt.Errorf("joining evidence: %w", err)
		}
		sb.Verdicts = recs
	}

	return sb, testSummary, nil
}

func readBoundaryContract(path string) (*upstream.BoundaryContract, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading boundary contract %s: %w", path, err)
	}
	c, err := upstream.DecodeBoundaryContract(data)
	if err != nil {
		return nil, fmt.Errorf("decoding boundary contract %s: %w", path, err)
	}
	return c, nil
}

// loadSpecACs resolves bindings.yaml's `spec:` ref to a spec.md under
// specs/active/ or specs/archive/ (a closed feature spec's bindings, if
// any, still need their AC set resolvable) and returns the set of AC ids
// it declares.
func loadSpecACs(root, specRef string) (map[string]bool, error) {
	ref, err := artifact.ParseRef(specRef)
	if err != nil {
		return nil, fmt.Errorf("bindings spec ref %q: %w", specRef, err)
	}

	for _, zone := range []string{"active", "archive"} {
		path := filepath.Join(root, ".verdi", "specs", zone, ref.Name, "spec.md")
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		fm, _, err := artifact.SplitFrontmatter(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		spec, err := artifact.DecodeSpec(fm)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		acs := make(map[string]bool, len(spec.AcceptanceCriteria))
		for _, ac := range spec.AcceptanceCriteria {
			acs[ac.ID] = true
		}
		return acs, nil
	}
	return nil, fmt.Errorf("spec %q: no spec.md found under specs/active or specs/archive", specRef)
}

// listGoldenFlows returns the set of golden flow names declared under
// <serviceDir>/testdata/flows/*.golden.json (extension stripped) — the
// behavioral-binding producer existence check (dangling-binding
// detection). A service with no testdata/flows directory yet has an empty
// (not missing) set.
func listGoldenFlows(serviceDir string) (map[string]bool, error) {
	dir := filepath.Join(serviceDir, "testdata", "flows")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]bool{}, nil
		}
		return nil, fmt.Errorf("listing golden flows in %s: %w", dir, err)
	}
	out := make(map[string]bool, len(entries))
	for _, e := range entries {
		if name, ok := strings.CutSuffix(e.Name(), ".golden.json"); ok {
			out[name] = true
		}
	}
	return out, nil
}

// writeTempGraph writes g to a scratch file for feeding to `groundwork
// review`, which takes graph file paths, not stdin. Re-marshals via plain
// encoding/json (determinism does not matter for a file that lives only
// for the duration of one Review call).
func writeTempGraph(g *upstream.Graph) (path string, cleanup func(), err error) {
	data, err := json.Marshal(g)
	if err != nil {
		return "", nil, err
	}
	return writeTempFile(data, "verdi-sync-graph-*.json")
}

// writeTempFile writes data to a fresh scratch file matching pattern
// (os.CreateTemp's glob-with-one-star convention) and returns its path
// plus a cleanup func that removes it. Shared by writeTempGraph and the
// boundary-diff cross-check's scratch base-contract file — both exist
// only to hand a filesystem path to an upstream CLI that takes paths, not
// stdin.
func writeTempFile(data []byte, pattern string) (path string, cleanup func(), err error) {
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", nil, err
	}
	cleanup = func() { os.Remove(f.Name()) }

	if _, err := f.Write(data); err != nil {
		f.Close()
		cleanup()
		return "", nil, err
	}
	if err := f.Close(); err != nil {
		cleanup()
		return "", nil, err
	}
	return f.Name(), cleanup, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
