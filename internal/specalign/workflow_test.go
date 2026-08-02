// Task 8 (spec/merge-signaled-acceptance, "install the stable required merge
// gate"): source-level assertions over the .github/workflows/*.yml files
// themselves — never a live GitHub API call (co-1: no network in any test).
// The enforcement plan's step 5 (proving the check actually reports
// "merge-gate" on a real PR) and step 6 (mutating the branch ruleset) are
// deliberately out of this package's reach; this file only proves the
// workflow SOURCE is shaped the way the ratified design requires, so a CI
// checkout of verdi alone (no sibling docs/ workspace) can still run it.
//
// Why this file does NOT `import "gopkg.in/yaml.v3"` directly: CLAUDE.md's
// "single import seam" (internal/artifact is the module's one YAML decode
// seam) is enforced module-wide by
// internal/artifact.TestYAMLImportSeam_TestFiles, which fails any package
// — including a _test.go file outside internal/artifact's own subtree —
// that imports yaml.v3 itself. GitHub Actions workflow files are
// foreign-authored YAML (not a verdi artifact schema), which is exactly
// the case internal/artifact.DecodeYAMLLoose exists for ("verdi doesn't
// own this schema, read it as a guest" — its own doc comment, generalizing
// the same posture DecodeFlowmapLoose established for .flowmap.yaml and
// the dex build's OpenAPI transcoding). So this file decodes through that
// one exported function and then hand-converts the resulting generic
// map[string]interface{}/[]interface{}/scalar value into the small typed
// structs below (workflowDoc, workflowTriggers, ...) — not full
// map[string]any spelunking at every call site, but as close to typed
// struct decoding as the seam allows without duplicating a second yaml.v3
// import path.
//
// The YAML 1.1 `on:` bareword gotcha this file works around: some
// go-yaml versions/decode paths resolve the bare, unquoted mapping key
// `on` (and `off`/`yes`/`no`) to the boolean `true`/`false` per the YAML
// 1.1 core schema, even though these workflow files use it as the literal
// key text "on". Verified empirically against this repo's pinned
// gopkg.in/yaml.v3 v3.0.1: DecodeYAMLLoose's Node-based decode path
// resolves the top-level `on:` key to the STRING "on", not a bool — proven
// directly against the real current files by every test below, not a
// synthetic fixture. asMap's key-normalization below still defends against
// the boolean-key shape anyway (mapping a literal Go `true`/`false` map key
// back to "on"/"off"), in case that decode path's behavior ever differs —
// "handle robustly" per the brief, not "assume today's observed behavior
// forever".
package specalign

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

// workflowTriggers is the subset of a GitHub Actions workflow's `on:` block
// this file cares about. Only pull_request and push are modeled — anything
// else in a real workflow's `on:` block is simply not extracted, which is
// fine: this package asserts presence/absence and filter shape, not an
// exhaustive schema of every trigger GitHub supports. A nil field means
// the trigger was absent from the document; a non-nil *triggerFilter with
// empty Branches/Paths means the trigger fired with no filter narrowing it
// (a bare `pull_request:` or the explicit empty-mapping `pull_request: {}`
// form).
type workflowTriggers struct {
	PullRequest *triggerFilter
	Push        *triggerFilter
}

// triggerFilter models the branches/paths narrowing a single trigger can
// carry, plus Keys: the trigger body's COMPLETE raw key set, sorted.
//
// Keys is the whitelist net. Branches/Paths are two named ways to narrow a
// trigger, but GitHub offers many more (`types:`, `paths-ignore:`,
// `branches-ignore:`, `tags:`, `tags-ignore:`) and every one of them can
// make a required check absent on some PR shape. Rather than growing one
// negative assertion per narrowing keyword — a list that is only ever as
// complete as the last person to read GitHub's docs — the callers below
// assert Keys is EMPTY, which closes all of them plus anything GitHub adds
// later, in one assertion. Keys is nil for the bare `pull_request:` form
// (no body at all) and empty for the explicit `pull_request: {}` form; both
// mean "the trigger fires, nothing narrows it".
type triggerFilter struct {
	Branches []string
	Paths    []string
	Keys     []string
}

// workflowJob is the subset of a job's fields this file asserts on: whether
// it declares a `name:` override (job.Name != "" means the reported check
// context would be that override, not the job key), its step list, and
// Keys: the job mapping's COMPLETE raw key set, sorted.
//
// Keys is the whitelist net, for the same reason triggerFilter.Keys is.
// A required status check can be made absent, skipped, renamed, or
// non-blocking by any of `name:` (renames the context), `if:` (a skipped
// job does not satisfy a required context), `strategy: matrix:` (renames
// it to "merge-gate (…)"), a job-level `uses:` (reusable workflow —
// renames it to "merge-gate / <inner-job>"), and `continue-on-error:` (the
// context reports green over a failing gate). Asserting the key set is
// exactly {"runs-on", "steps"} closes all of those, and every future
// sibling of them, at once.
type workflowJob struct {
	Name  string
	Steps []workflowStep
	Keys  []string
}

// workflowStep models one step of a job: either an `uses:` action reference
// (optionally parameterized by `with:`) or a `run:` shell command.
type workflowStep struct {
	Name string
	Uses string
	With map[string]string
	Run  string
}

// workflowDoc is the top-level shape of a GitHub Actions workflow file, as
// far as this package needs it.
type workflowDoc struct {
	Name string
	On   workflowTriggers
	Jobs map[string]workflowJob
}

// asMap normalizes a decoded YAML mapping value (from
// artifact.DecodeYAMLLoose's generic tree, which produces
// map[string]interface{} when every key resolved as a string and
// map[interface{}]interface{} when at least one key resolved to a
// non-string scalar) into a plain map[string]interface{}, or reports false
// if v is not a mapping at all (nil, a scalar, or a sequence — e.g. a bare
// trigger with no body, or a key genuinely absent).
//
// The key-normalization defends specifically against the YAML 1.1
// `on`/`off`/`yes`/`no` bareword-boolean gotcha documented in this file's
// package comment: a literal Go bool key is mapped back to its YAML 1.1
// bareword spelling rather than its Go %v rendering ("true"/"false"),
// since a bool key at this document's shape can only plausibly have come
// from one of those four barewords, and the whole point of this function
// existing is to make that gotcha a non-event for every caller below.
func asMap(v interface{}) (map[string]interface{}, bool) {
	switch m := v.(type) {
	case map[string]interface{}:
		return m, true
	case map[interface{}]interface{}:
		out := make(map[string]interface{}, len(m))
		for k, val := range m {
			out[normalizeKey(k)] = val
		}
		return out, true
	default:
		return nil, false
	}
}

func normalizeKey(k interface{}) string {
	if b, ok := k.(bool); ok {
		if b {
			return "on"
		}
		return "off"
	}
	return fmt.Sprintf("%v", k)
}

// sortedKeys returns m's keys in sorted order — the raw key set a
// whitelist assertion compares against. Sorted so both the comparison and
// any failure message are deterministic regardless of Go's map iteration
// order.
func sortedKeys(m map[string]interface{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// asSlice normalizes a decoded YAML sequence value into []interface{}, or
// reports false if v is not a sequence.
func asSlice(v interface{}) ([]interface{}, bool) {
	s, ok := v.([]interface{})
	return s, ok
}

// asStringVal renders a decoded YAML scalar as a string, matching the raw
// text a struct-typed yaml.v3 decode would have produced for a string
// field (e.g. the unquoted integer `fetch-depth: 0` renders as "0", not
// Go's `%#v` form) — via fmt.Sprintf's default `%v` verb, which is exactly
// that raw-ish textual rendering for the scalar kinds YAML produces
// (string, bool, int, float). Reports false only for a genuinely absent
// value (Go nil, meaning the YAML key was absent or explicitly `null`).
func asStringVal(v interface{}) (string, bool) {
	if v == nil {
		return "", false
	}
	if s, ok := v.(string); ok {
		return s, true
	}
	return fmt.Sprintf("%v", v), true
}

// asStringSlice normalizes a decoded YAML sequence of scalars (e.g. a
// `paths:`/`branches:` list) into a []string, skipping any element that
// isn't a renderable scalar. Returns nil (not an error) if v isn't a
// sequence at all — the caller's zero-length check is what matters, not
// distinguishing "absent" from "empty".
func asStringSlice(v interface{}) []string {
	items, ok := asSlice(v)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := asStringVal(item); ok {
			out = append(out, s)
		}
	}
	return out
}

// decodeWorkflow reads path, decodes it as foreign-schema YAML through
// internal/artifact's one loose-decode seam (DecodeYAMLLoose — see this
// file's package comment for why not yaml.v3 directly), and hand-converts
// the result into a workflowDoc. Fails the calling test outright on any
// read, parse, or top-level shape error.
func decodeWorkflow(t *testing.T, path string) workflowDoc {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	generic, err := artifact.DecodeYAMLLoose(raw)
	if err != nil {
		t.Fatalf("parsing %s as YAML: %v", path, err)
	}
	top, ok := asMap(generic)
	if !ok {
		t.Fatalf("parsing %s: top-level document is not a mapping (got %T)", path, generic)
	}

	var doc workflowDoc
	if name, ok := asStringVal(top["name"]); ok {
		doc.Name = name
	}
	doc.On = decodeTriggers(top["on"])
	doc.Jobs = decodeJobs(top["jobs"])
	return doc
}

func decodeTriggers(v interface{}) workflowTriggers {
	m, ok := asMap(v)
	if !ok {
		return workflowTriggers{}
	}
	var triggers workflowTriggers
	if pr, present := m["pull_request"]; present {
		tf := decodeTriggerFilter(pr)
		triggers.PullRequest = &tf
	}
	if push, present := m["push"]; present {
		tf := decodeTriggerFilter(push)
		triggers.Push = &tf
	}
	return triggers
}

// decodeTriggerFilter handles all three shapes a trigger body can take: a
// bare `pull_request:` with nothing under it (v is Go nil), the explicit
// empty-mapping form `pull_request: {}` (v is an empty map), and a real
// filter body (`branches:`/`paths:`/`types:`/… ) — the first two decode to
// a zero-value triggerFilter, which is exactly "the trigger fired, no
// filter narrows it". Keys carries the body's complete raw key set so a
// caller can assert emptiness rather than enumerating narrowing keywords
// one at a time (see triggerFilter's doc comment).
func decodeTriggerFilter(v interface{}) triggerFilter {
	m, ok := asMap(v)
	if !ok {
		return triggerFilter{}
	}
	return triggerFilter{
		Branches: asStringSlice(m["branches"]),
		Paths:    asStringSlice(m["paths"]),
		Keys:     sortedKeys(m),
	}
}

func decodeJobs(v interface{}) map[string]workflowJob {
	m, ok := asMap(v)
	if !ok {
		return nil
	}
	jobs := make(map[string]workflowJob, len(m))
	for k, jv := range m {
		jobs[k] = decodeJob(jv)
	}
	return jobs
}

func decodeJob(v interface{}) workflowJob {
	m, ok := asMap(v)
	if !ok {
		return workflowJob{}
	}
	var job workflowJob
	job.Keys = sortedKeys(m)
	if name, ok := asStringVal(m["name"]); ok {
		job.Name = name
	}
	if steps, ok := asSlice(m["steps"]); ok {
		job.Steps = make([]workflowStep, 0, len(steps))
		for _, sv := range steps {
			job.Steps = append(job.Steps, decodeStep(sv))
		}
	}
	return job
}

func decodeStep(v interface{}) workflowStep {
	m, ok := asMap(v)
	if !ok {
		return workflowStep{}
	}
	var step workflowStep
	if name, ok := asStringVal(m["name"]); ok {
		step.Name = name
	}
	if uses, ok := asStringVal(m["uses"]); ok {
		step.Uses = uses
	}
	if run, ok := asStringVal(m["run"]); ok {
		step.Run = run
	}
	if withMap, ok := asMap(m["with"]); ok {
		step.With = make(map[string]string, len(withMap))
		for k, wv := range withMap {
			if s, ok := asStringVal(wv); ok {
				step.With[k] = s
			}
		}
	}
	return step
}

// findStep returns the first step in steps whose Uses has usesPrefix as a
// prefix (e.g. "actions/checkout@v4"), or nil if none matches.
func findStep(steps []workflowStep, usesPrefix string) *workflowStep {
	for i := range steps {
		if strings.HasPrefix(steps[i].Uses, usesPrefix) {
			return &steps[i]
		}
	}
	return nil
}

// findRunStep returns the first step in steps whose Run contains substr, or
// nil if none matches. `run:` steps in these workflows are single- or
// multi-line shell; substring search is enough to prove a specific command
// is invoked without over-fitting to exact formatting.
func findRunStep(steps []workflowStep, substr string) *workflowStep {
	for i := range steps {
		if strings.Contains(steps[i].Run, substr) {
			return &steps[i]
		}
	}
	return nil
}

// findCacheStep returns the first actions/cache step whose `with: path:`
// mentions pathSubstr (e.g. "golangci-lint"), or nil if none matches. The
// path is what disambiguates one cache step from another; matching on
// `uses:` alone would pick whichever cache step happens to come first.
func findCacheStep(steps []workflowStep, pathSubstr string) *workflowStep {
	for i := range steps {
		if !strings.HasPrefix(steps[i].Uses, "actions/cache@") {
			continue
		}
		if strings.Contains(steps[i].With["path"], pathSubstr) {
			return &steps[i]
		}
	}
	return nil
}

// golangciPinRE extracts the Makefile's golangci-lint pin. The Makefile
// spells it `GOLANGCI_LINT_VERSION ?= v2.5.0` (a conditional assignment);
// the pattern also accepts a plain `=` so a future switch to an
// unconditional assignment does not silently turn this gate off.
var golangciPinRE = regexp.MustCompile(`(?m)^GOLANGCI_LINT_VERSION[ \t]*\??=[ \t]*(\S+)[ \t]*$`)

// makefileGolangciPin reads the real Makefile (not a fixture) and returns
// the value of GOLANGCI_LINT_VERSION.
func makefileGolangciPin(t *testing.T) string {
	t.Helper()
	path := filepath.Join(verdiRepoRoot, "Makefile")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	m := golangciPinRE.FindSubmatch(raw)
	if m == nil {
		t.Fatalf("%s: no GOLANGCI_LINT_VERSION assignment found (pattern %q) — if the variable was renamed, this lockstep gate must be renamed with it, not deleted", path, golangciPinRE)
	}
	return string(m[1])
}

func mergeGatePath(root string) string {
	return filepath.Join(root, ".github", "workflows", "merge-gate.yml")
}

func workflowPath(root, file string) string {
	return filepath.Join(root, ".github", "workflows", file)
}

// TestGolangciLintPinIsLockstepWithMakefile closes the drift the Makefile's
// own head comment and verify.yml's head comment both warn about in prose
// and neither enforces: `make verify`'s lint step runs whatever
// golangci-lint the workflow installed, so if the Makefile's pin is bumped
// and the workflows are not, CI silently lints with the OLD linter while
// every other test stays green. The Makefile is read as the single source
// of truth and both the install step's `@<version>` AND the cache key's
// `<version>` are asserted against it, in both workflows that carry the
// pattern.
//
// verify.yml is asserted here but never modified by this task — it uses the
// identical cache/install step pair, so covering it costs one table row.
func TestGolangciLintPinIsLockstepWithMakefile(t *testing.T) {
	pin := makefileGolangciPin(t)

	// The plan's stated pin, asserted literally so a bump cannot happen by
	// accident anywhere. A DELIBERATE Makefile bump updates this literal
	// too — that is the point: the bump becomes one visible, reviewed edit
	// here instead of silent divergence across three files.
	if want := "v2.5.0"; pin != want {
		t.Errorf("Makefile GOLANGCI_LINT_VERSION = %q, want %q (the plan's stated pin); if this bump is deliberate, update this literal in the same commit", pin, want)
	}

	tests := []struct {
		name string
		file string
		job  string
	}{
		{"merge-gate.yml", "merge-gate.yml", "merge-gate"},
		{"verify.yml", "verify.yml", "verify"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := decodeWorkflow(t, workflowPath(verdiRepoRoot, tt.file))
			job, ok := doc.Jobs[tt.job]
			if !ok {
				t.Fatalf("%s: no %q job found", tt.name, tt.job)
			}

			install := findRunStep(job.Steps, "go install github.com/golangci/golangci-lint")
			if install == nil {
				t.Fatalf("%s: no run step installing golangci-lint found", tt.name)
			}
			if !strings.Contains(install.Run, "@"+pin) {
				t.Errorf("%s: golangci-lint install step does not pin @%s (the Makefile's GOLANGCI_LINT_VERSION), got run: %q", tt.name, pin, install.Run)
			}

			cache := findCacheStep(job.Steps, "golangci-lint")
			if cache == nil {
				t.Fatalf("%s: no actions/cache step caching golangci-lint found", tt.name)
			}
			if key := cache.With["key"]; !strings.Contains(key, pin) {
				t.Errorf("%s: golangci-lint cache key %q does not carry the Makefile's pin %s — a stale key would restore the wrong linter binary and make the install step a no-op", tt.name, key, pin)
			}
		})
	}
}

// TestMergeGateWorkflowExists is the first, most basic red-phase assertion:
// the file must exist at all before anything else about it can be checked.
func TestMergeGateWorkflowExists(t *testing.T) {
	path := mergeGatePath(verdiRepoRoot)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s to exist (Task 8: the always-present PR gate): %v", path, err)
	}
}

// TestMergeGateTriggersOnEveryPullRequest proves the workflow triggers on
// pull_request with NOTHING narrowing it — the whole point of Task 8's
// design: a path-, branch-, or type-filtered required check can never
// report on a PR outside its filter (spec-gate.yml's own former comment
// named this exact deadlock), so merge-gate.yml must be unconditional.
//
// Two layers, on purpose. The targeted paths/branches assertions come
// first because they name the specific regression in their failure message
// (they are the two filters the brief called out). The trigger-body key-set
// assertion after them is the COMPLETENESS NET: it is a whitelist ("no keys
// at all"), so it also closes `types:`, `paths-ignore:`, `branches-ignore:`,
// `tags:`/`tags-ignore:`, and any future narrowing keyword GitHub invents —
// none of which the targeted checks would ever see.
func TestMergeGateTriggersOnEveryPullRequest(t *testing.T) {
	doc := decodeWorkflow(t, mergeGatePath(verdiRepoRoot))

	if doc.On.PullRequest == nil {
		t.Fatalf("merge-gate.yml: expected an `on: pull_request` trigger, found none (decoded on: %+v)", doc.On)
	}
	if len(doc.On.PullRequest.Paths) != 0 {
		t.Errorf("merge-gate.yml: pull_request trigger must have NO paths filter, got %v", doc.On.PullRequest.Paths)
	}
	if len(doc.On.PullRequest.Branches) != 0 {
		t.Errorf("merge-gate.yml: pull_request trigger must have NO branches filter, got %v", doc.On.PullRequest.Branches)
	}
	if keys := doc.On.PullRequest.Keys; len(keys) != 0 {
		t.Errorf("merge-gate.yml: the pull_request trigger body must be EMPTY — a bare `pull_request:` or `pull_request: {}` — so nothing can narrow when the required context reports; found key(s) %v", keys)
	}
}

// TestMergeGateSingleUnnamedJob proves the workflow declares exactly one
// job, its key is `merge-gate`, and the job carries no `name:` override —
// together this is what makes GitHub report the required-status-check
// context as exactly "merge-gate" (job key, no override), matching
// verify.yml's own established workflow-name/job-key pattern (`name:
// verify` + job key `verify`).
//
// Same two layers as the trigger test. The `name:`-override check is the
// targeted one (it names the exact regression in its message); the job
// key-set assertion after it is the COMPLETENESS NET. Because it is a
// whitelist — the job may declare `runs-on` and `steps` and nothing else —
// it closes, in one assertion, every other way a required context can be
// made absent, skipped, renamed, or non-blocking: `if:` (a skipped job
// never satisfies a required context), `strategy: matrix:` (context becomes
// "merge-gate (…)"), a job-level `uses:` (context becomes
// "merge-gate / <inner-job>"), `continue-on-error:` (green over a failing
// gate), plus `container:`/`needs:`/`environment:` and any future sibling.
// Widening this set is a deliberate act that must be argued for here.
func TestMergeGateSingleUnnamedJob(t *testing.T) {
	doc := decodeWorkflow(t, mergeGatePath(verdiRepoRoot))

	if len(doc.Jobs) != 1 {
		t.Fatalf("merge-gate.yml: expected exactly one job, found %d: %v", len(doc.Jobs), jobKeys(doc.Jobs))
	}
	job, ok := doc.Jobs["merge-gate"]
	if !ok {
		t.Fatalf("merge-gate.yml: expected the one job's key to be %q, found %v", "merge-gate", jobKeys(doc.Jobs))
	}
	if job.Name != "" {
		t.Errorf("merge-gate.yml: job %q must not declare a `name:` override (it would change the reported check context away from the job key), got %q", "merge-gate", job.Name)
	}
	wantJobKeys := []string{"runs-on", "steps"}
	if !slices.Equal(job.Keys, wantJobKeys) {
		t.Errorf("merge-gate.yml: job %q must declare exactly the keys %v and nothing else (any of name/if/strategy/uses/continue-on-error would make the required %q context absent, skipped, renamed, or non-blocking), got %v", "merge-gate", wantJobKeys, "merge-gate", job.Keys)
	}
}

func jobKeys(jobs map[string]workflowJob) []string {
	keys := make([]string, 0, len(jobs))
	for k := range jobs {
		keys = append(keys, k)
	}
	return keys
}

// TestMergeGateStepsProvenSequence proves merge-gate.yml's steps are the
// brief's required, proven sequence copied from verify.yml: checkout with
// full history, pinned Go/Node/golangci-lint, `make verify`, build the
// binary, and self-lint it. Table-driven per assertion so a regression in
// any one step names exactly which.
func TestMergeGateStepsProvenSequence(t *testing.T) {
	doc := decodeWorkflow(t, mergeGatePath(verdiRepoRoot))
	job, ok := doc.Jobs["merge-gate"]
	if !ok {
		t.Fatalf("merge-gate.yml: no %q job found to inspect steps of", "merge-gate")
	}
	steps := job.Steps

	t.Run("checkout@v4 with fetch-depth 0", func(t *testing.T) {
		step := findStep(steps, "actions/checkout@v4")
		if step == nil {
			t.Fatalf("no actions/checkout@v4 step found")
		}
		if got := step.With["fetch-depth"]; got != "0" {
			t.Errorf("actions/checkout@v4 fetch-depth = %q, want \"0\"", got)
		}
	})

	t.Run("setup-go pins 1.25", func(t *testing.T) {
		step := findStep(steps, "actions/setup-go@v5")
		if step == nil {
			t.Fatalf("no actions/setup-go@v5 step found")
		}
		if got := step.With["go-version"]; got != "1.25" {
			t.Errorf("actions/setup-go@v5 go-version = %q, want \"1.25\"", got)
		}
	})

	t.Run("setup-node pins 22", func(t *testing.T) {
		step := findStep(steps, "actions/setup-node@v4")
		if step == nil {
			t.Fatalf("no actions/setup-node@v4 step found")
		}
		if got := step.With["node-version"]; got != "22" {
			t.Errorf("actions/setup-node@v4 node-version = %q, want \"22\"", got)
		}
	})

	// The pin's VALUE is proven against the Makefile (the single source of
	// truth) by TestGolangciLintPinIsLockstepWithMakefile, which also holds
	// the one literal "v2.5.0" assertion; this subtest stays targeted at the
	// step sequence — that an install step exists here at all and carries the
	// pin.
	t.Run("golangci-lint pinned to the Makefile's version", func(t *testing.T) {
		step := findRunStep(steps, "golangci-lint")
		if step == nil {
			t.Fatalf("no run step mentioning golangci-lint found")
		}
		pin := makefileGolangciPin(t)
		if !strings.Contains(step.Run, "@"+pin) {
			t.Errorf("golangci-lint install step does not pin @%s, got run: %q", pin, step.Run)
		}
	})

	t.Run("runs make verify", func(t *testing.T) {
		if findRunStep(steps, "make verify") == nil {
			t.Errorf("no run step invoking `make verify` found")
		}
	})

	t.Run("builds the verdi binary", func(t *testing.T) {
		if findRunStep(steps, "go build -o .build/verdi ./cmd/verdi") == nil {
			t.Errorf("no run step invoking `go build -o .build/verdi ./cmd/verdi` found")
		}
	})

	t.Run("runs the built binary's lint verb", func(t *testing.T) {
		if findRunStep(steps, "./.build/verdi lint") == nil {
			t.Errorf("no run step invoking `./.build/verdi lint` found")
		}
	})

	t.Run("does not upload evidence artifacts (that stays verify.yml's push-only duty)", func(t *testing.T) {
		if step := findStep(steps, "actions/upload-artifact"); step != nil {
			t.Errorf("merge-gate.yml must not produce/upload evidence artifacts, found a step using %q", step.Uses)
		}
		if step := findRunStep(steps, "verdi sync --produce"); step != nil {
			t.Errorf("merge-gate.yml must not run evidence production (`verdi sync --produce`), found: %q", step.Run)
		}
	})
}

// TestOldWorkflowsNoLongerDeclarePullRequest is the negative-path proof:
// verify.yml and spec-gate.yml, which used to gate PRs directly via
// path-filtered pull_request triggers, must no longer declare pull_request
// at all now that merge-gate.yml is the one stable PR gate — leaving a
// stray pull_request trigger behind would mean PRs get gated twice (once
// unconditionally, once path-filtered), reintroducing exactly the ambiguity
// Task 8 exists to remove.
func TestOldWorkflowsNoLongerDeclarePullRequest(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{"verify.yml", workflowPath(verdiRepoRoot, "verify.yml")},
		{"spec-gate.yml", workflowPath(verdiRepoRoot, "spec-gate.yml")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := decodeWorkflow(t, tt.path)
			if doc.On.PullRequest != nil {
				t.Errorf("%s: expected no pull_request trigger (PR gating now lives in merge-gate.yml), found one: %+v", tt.name, doc.On.PullRequest)
			}
			if doc.On.Push == nil {
				t.Errorf("%s: expected the push trigger to remain untouched, found none", tt.name)
			}
		})
	}
}
