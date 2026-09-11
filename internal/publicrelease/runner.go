package publicrelease

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
)

type releaseReport struct {
	Baselines        []baselineRecord  `json:"baseline_source_builds"`
	Inputs           inputs            `json:"inputs"`
	Workflow         workflowContext   `json:"workflow"`
	SourceChecks     []SourceCheck     `json:"source_checks"`
	Executions       []execution       `json:"executions"`
	HistoricalSkips  []historicalSkip  `json:"historical_skips"`
	RuntimeArtifacts map[string]string `json:"runtime_artifacts"`
	Runtime          runtimeOutcomes   `json:"runtime_outcomes"`
	Limits           []string          `json:"limits"`
}
type producerResult struct {
	Producer       string                `json:"producer"`
	AC             string                `json:"ac"`
	Kind           artifact.EvidenceKind `json:"kind"`
	ReleaseSHA     string                `json:"release_sha256"`
	RequiredChecks []string              `json:"required_checks"`
	ExecutionNames []string              `json:"execution_names"`
	SourceChecks   []SourceCheck         `json:"source_checks"`
}

// Run executes every release prerequisite and all ten closed producer scopes.
// OutputDir must not exist; failed runs retain private logs but publish no passes.
func Run(ctx context.Context, c Config) (runErr error) {
	for _, p := range []string{c.VerdiDir, c.ATCDir, c.VerdiBinary, c.ATCBinary, c.BaselineSource, c.BaselineVerdi, c.BaselineATC, c.OutputDir} {
		if !filepath.IsAbs(p) {
			return fmt.Errorf("all release paths must be absolute")
		}
	}
	if err := os.Mkdir(c.OutputDir, 0700); err != nil {
		return fmt.Errorf("fresh output directory: %w", err)
	}
	private := filepath.Join(c.OutputDir, "private")
	if err := os.Mkdir(private, 0700); err != nil {
		return err
	}
	boundary := filepath.Join(private, "boundary")
	if err := os.Mkdir(boundary, 0700); err != nil {
		return err
	}
	defer func() {
		if runErr != nil {
			body, err := canonjson.Marshal(map[string]string{"status": "refused", "error": runErr.Error()})
			if err == nil {
				err = os.WriteFile(filepath.Join(private, "refusal.json"), body, 0600)
			}
			if err != nil {
				fmt.Fprintln(os.Stderr, "cannot retain refusal record:", err)
			}
		}
	}()
	x := processExecutor{root: private}
	workflow, err := provenance(c.WorkflowRun, c.VerdiCommit, os.Getenv)
	if err != nil {
		return err
	}
	before, err := measure(ctx, c)
	if err != nil {
		return err
	}
	declarations, err := readDeclarations(checkBytes)
	if err != nil {
		return err
	}
	sourceChecks, err := SourceChecks(ctx, c.VerdiDir, c.ATCDir)
	if err != nil {
		return fmt.Errorf("%w: source checks: %v", ErrVerdict, err)
	}
	report := releaseReport{Inputs: before, Workflow: workflow, SourceChecks: sourceChecks, HistoricalSkips: []historicalSkip{}, Limits: []string{
		"Five process cuts preserve measured authority outcomes, not seamless resumption or exactly-once execution.",
		"Launch traces cover successful verdiproto starts, not arbitrary descendant processes; no latency claim.",
		"Only quiescent matched-pair upgrade and rollback are supported; interrupted flights resolve under their original pair.",
		"Consolidation metadata is a fixed reviewed source/mapping witness plus freshly executed replay, not a general nested-metadata schema validator.",
		"Runtime source, archives, overlays, binaries and raw logs remain private. Published inventories bind their hashes without publishing source.",
		"Copied-store raw-byte and exact persisted-inventory proofs are the freshly executed boundary assertions; typed report comparisons supplement them by comparing emitted handoff strings and unmodified serialized record fields, without treating JSON reserialization as stored-byte proof.",
	}}
	env := []string{"PUBLIC_RELEASE_SOURCECHECK_ATC=" + c.ATCDir, "GOPROXY=off", "GOWORK=off", "GOFLAGS=", "VERDI_CONSOLIDATION_REQUIRE_ATC=1", "VERDI_CONSOLIDATION_ATC_SOURCE=" + c.ATCDir, "VATC_REAL_VERDI=" + c.VerdiBinary, "VATC_REAL_VERDI_SHA256=" + before.Binaries["verdi"], "VATC_REAL_VERDI_SOURCE=" + c.VerdiDir, "VATC_BOUNDARY_BASELINE_SOURCE=" + c.BaselineSource, "VATC_BOUNDARY_BASELINE_VATC=" + c.BaselineATC, "VATC_BOUNDARY_BASELINE_VERDI=" + c.BaselineVerdi, "VATC_BOUNDARY_EVIDENCE=" + boundary}
	// Both candidate binaries and the actual executing checker are independently
	// rebuilt from the claimed exact source before any producer can pass.
	checkerPath, err := os.Executable()
	if err != nil {
		return err
	}
	for _, row := range []struct{ name, dir, path, pkg string }{{"verdi", c.VerdiDir, c.VerdiBinary, "./cmd/verdi"}, {"atc", c.ATCDir, c.ATCBinary, "./cmd/vatc"}, {"checker", c.VerdiDir, checkerPath, "./cmd/public-execution-contract-release"}} {
		binary := filepath.Join(private, "rebuilt-"+row.name)
		out, e := x.Run(ctx, command{Dir: row.dir, Name: "build-" + row.name, Args: []string{"go", "build", "-trimpath", "-o", binary, row.pkg}, Env: env})
		if e != nil {
			return e
		}
		report.Executions = append(report.Executions, out)
		if out.Exit != 0 {
			return fmt.Errorf("%w: %s build", ErrVerdict, row.name)
		}
		h, e := fileDigest(binary)
		if e != nil {
			return e
		}
		want := before.Binaries[row.name]
		if row.name == "checker" {
			want = before.CheckerSHA
		}
		if h != want {
			return fmt.Errorf("candidate %s executable differs from exact-source rebuild", row.name)
		}
	}
	for _, g := range groups(declarations) {
		dir := c.VerdiDir
		module := "github.com/jyang234/verdi"
		if g.Repo == "A" {
			dir = c.ATCDir
			b, e := commandBytes(ctx, dir, "go", "list", "-m")
			if e != nil {
				return e
			}
			module = strings.TrimSpace(string(b))
		}
		name := g.Repo + "-" + strings.ReplaceAll(g.Package, "/", "-")
		out, e := runTests(ctx, x, command{Dir: dir, Name: name, Args: []string{"go", "test", "-json", "-count=1", "-timeout=30m", "./" + g.Package, "-run", "^(" + strings.Join(g.Roots, "|") + ")$"}, Env: env}, module+"/"+g.Package, g.Required)
		report.Executions = append(report.Executions, out)
		if e != nil {
			return e
		}
	}
	// These are the ordinary gates, unchanged. The boundary target separately
	// requires its exact nine names and zero skips, even when a full race suite
	// discloses the historical fixed-pin skips.
	executions, skips, err := fullGates(ctx, x, c, env)
	report.Executions = append(report.Executions, executions...)
	report.HistoricalSkips = append(report.HistoricalSkips, skips...)
	if err != nil {
		return err
	}

	if report.RuntimeArtifacts, err = inventory(boundary); err != nil {
		return err
	}
	if err = runtimeComplete(report.RuntimeArtifacts); err != nil {
		return fmt.Errorf("%w: %v", ErrVerdict, err)
	}
	if report.Runtime, err = readRuntime(boundary, report.RuntimeArtifacts, report.Executions); err != nil {
		return fmt.Errorf("%w: runtime outcomes: %v", ErrVerdict, err)
	}
	if report.Baselines, err = baselineRecords(boundary, report.RuntimeArtifacts, before); err != nil {
		return err
	}
	after, err := measure(ctx, c)
	if err != nil {
		return err
	}
	if err = unchanged(before, after); err != nil {
		return err
	}
	return publish(c.OutputDir, report, declarations)
}

func publish(root string, report releaseReport, d declarations) error {
	b, err := canonjson.Marshal(report)
	if err != nil {
		return err
	}
	releaseSHA := digest(b)
	results, err := produce(report, d, releaseSHA)
	if err != nil {
		return err
	}
	// This new directory is the entire explicit upload allowlist. No copy/glob
	// from private evidence enters it. Only typed source-free report values do.
	dir := filepath.Join(root, "publish")
	if err = os.Mkdir(dir, 0700); err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(dir, "release.json"), b, 0600); err != nil {
		return err
	}
	records := make([]artifact.Evidence, 0, len(results))
	for _, r := range results {
		body, e := canonjson.Marshal(r)
		if e != nil {
			return e
		}
		sha := digest(body)
		name := strings.ReplaceAll(r.Producer, ":", "--") + ".json"
		if e = os.WriteFile(filepath.Join(dir, name), body, 0600); e != nil {
			return e
		}
		record := artifact.Evidence{Schema: "verdi.evidence/v1", EvidenceFor: []string{r.AC}, Kind: r.Kind, Verdict: artifact.VerdictPass, Producer: r.Producer, Witness: name + " sha256:" + sha, Digest: "sha256:" + sha, Provenance: artifact.EvidenceProvenance{Source: report.Workflow.Source, Pipeline: report.Workflow.Run, Job: Job, Commit: report.Inputs.Verdi.Commit}}
		if e = record.Validate(); e != nil {
			return e
		}
		records = append(records, record)
	}
	evidence, e := canonjson.Marshal(records)
	if e != nil {
		return e
	}
	return os.WriteFile(filepath.Join(dir, "verdicts.json"), evidence, 0600)
}
func produce(report releaseReport, d declarations, releaseSHA string) ([]producerResult, error) {
	for _, name := range []string{"build-verdi", "build-atc", "build-checker", "verdi-verify", "verdi-race", "atc-verify", "atc-race", "boundary-check"} {
		count := 0
		for _, e := range report.Executions {
			if e.Name == name {
				count++
				if e.Exit != 0 {
					return nil, fmt.Errorf("failed release prerequisite %s", name)
				}
			}
		}
		if count != 1 {
			return nil, fmt.Errorf("missing or duplicate release prerequisite %s", name)
		}
	}
	results := make([]producerResult, 0, 10)
	for _, p := range d.Producers {
		r := producerResult{Producer: p.ID, AC: p.AC, Kind: p.Kind, ReleaseSHA: releaseSHA, RequiredChecks: p.Checks, ExecutionNames: []string{}, SourceChecks: []SourceCheck{}}
		for _, s := range report.SourceChecks {
			if s.AC == p.AC {
				r.SourceChecks = append(r.SourceChecks, s)
			}
		}
		if p.Kind == "static" && len(r.SourceChecks) == 0 {
			return nil, fmt.Errorf("missing %s source inventory", p.ID)
		}
		used := map[string]bool{}
		for _, key := range p.Checks {
			required := d.Tests[key]
			name := required.Repo + "-" + strings.ReplaceAll(required.Package, "/", "-")
			matched := false
			for _, out := range report.Executions {
				if out.Name != name {
					continue
				}
				if out.Exit != 0 || out.Tests == nil {
					return nil, fmt.Errorf("unproven check %s", key)
				}
				passed := map[string]bool{}
				for _, n := range out.Tests.Passed {
					passed[n] = true
				}
				for _, n := range required.Required {
					if !passed[n] {
						return nil, fmt.Errorf("missing check outcome %s", n)
					}
				}
				matched = true
			}
			if !matched {
				return nil, fmt.Errorf("missing executed check %s", key)
			}
			if !used[name] {
				r.ExecutionNames = append(r.ExecutionNames, name)
				used[name] = true
			}
		}
		results = append(results, r)
	}
	if len(results) != 10 {
		return nil, fmt.Errorf("missing producer")
	}
	return results, nil
}
func runtimeComplete(files map[string]string) error {
	required := []string{"matrix.json", "baseline-authentication.json", "opposite-shipped-replay.json", "opposite-shipped-dirty-git.json", "verdi-source.tar", "endpoint_test.go.txt", "verdi-endpoint.test", "atc-endpoint.test"}
	for _, suffix := range required {
		found := false
		for name := range files {
			if strings.HasSuffix(name, suffix) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("missing freshly executed runtime artifact %s", suffix)
		}
	}
	return nil
}
