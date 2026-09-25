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
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

// workflowTriggers is the subset of a GitHub Actions workflow's `on:` block
// this file cares about. pull_request, push, workflow_dispatch, and
// workflow_call are modeled — anything else in a real workflow's `on:`
// block is simply not extracted, which is fine: this package asserts
// presence/absence and filter shape, not an exhaustive schema of every
// trigger GitHub supports. A nil field means the trigger was absent from
// the document; a non-nil *triggerFilter with empty Branches/Paths means
// the trigger fired with no filter narrowing it (a bare `pull_request:` or
// the explicit empty-mapping `pull_request: {}` form).
//
// WorkflowCall reuses triggerFilter purely for its Keys whitelist net
// (on.workflow_call's body is `inputs:`/`outputs:`/`secrets:`, never
// branches/paths — those two fields are simply unused for this trigger,
// left zero).
//
// Keys is the `on:` mapping's COMPLETE raw key set, sorted: the whitelist
// net one level above triggerFilter.Keys. The four modeled fields can only
// prove a trigger present or absent; a trigger this struct does not model
// (`schedule:`, `pull_request_target:`, `workflow_run:`, ...) decodes to
// nothing at all, so "workflow_dispatch is the ONLY trigger" is provable only
// by asserting the key set itself.
type workflowTriggers struct {
	PullRequest      *triggerFilter
	Push             *triggerFilter
	WorkflowDispatch *workflowDispatchTrigger
	WorkflowCall     *triggerFilter
	Keys             []string
}

// workflowDispatchInput models one on.workflow_dispatch.inputs.<id> entry:
// the fields the close workflow's own input-validation contract cares
// about (required, type, description).
type workflowDispatchInput struct {
	Required    bool
	Type        string
	Description string
}

// workflowDispatchTrigger models on.workflow_dispatch, plus Keys: the
// trigger body's COMPLETE raw key set (same whitelist-net rationale as
// triggerFilter.Keys).
type workflowDispatchTrigger struct {
	Inputs map[string]workflowDispatchInput
	Keys   []string
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
	Branches       []string
	BranchesIgnore []string
	Tags           []string
	Paths          []string
	Keys           []string
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
//
// Uses, Environment, and Permissions are additive fields for the
// close/close-evidence workflows (R-CM-4): a caller job that calls a
// reusable workflow (`uses:`), the close job's protected `environment:`
// declaration, and a job's own `permissions:` map (scope -> "read"/
// "write"/"none"). Environment is decoded from either the bare-string
// form (`environment: close`) or the `{name, url}` mapping form.
//
// Needs, If, and RunsOn are additive fields for merge-gate.yml's parallel
// shape (SI-266): the aggregator job's `needs:` list (decoded from either the
// bare-string or the sequence form), its `if:` expression as raw text, and
// every job's `runs-on:` label.
type workflowJob struct {
	Name        string
	Steps       []workflowStep
	Uses        string
	Environment string
	Permissions map[string]string
	Needs       []string
	If          string
	RunsOn      string
	Keys        []string
}

// workflowStep models one step of a job: either an `uses:` action reference
// (optionally parameterized by `with:`) or a `run:` shell command, plus Keys:
// the step mapping's COMPLETE raw key set, sorted.
//
// Keys is the whitelist net one level below workflowJob.Keys, and it exists
// because a clean job mapping is not enough: the SAME bypasses GitHub offers
// at job level are offered again per step. `continue-on-error: true` on the
// `make verify` step makes the job (and so the required context) green over a
// failing gate; `if: false` (or any always-false expression) skips the step
// while the job still succeeds; `env:`/`shell:`/`working-directory:` can each
// change what the command actually executes. Asserting each step's key set is
// a subset of a tiny per-shape whitelist — {uses, with, name} for an action
// step, {run, name} for a command step — closes all of those and every future
// sibling at once, exactly as the job-level net does.
//
// Env is the step's own `env:` mapping (name -> raw value text), decoded for
// the close workflow, which passes its dispatch input and its committer
// identity to exactly the steps that need them (R-CM-4 fix pass, m-4, I-2).
type workflowStep struct {
	Name string
	Uses string
	With map[string]string
	Env  map[string]string
	Run  string
	Keys []string
}

// workflowDoc is the top-level shape of a GitHub Actions workflow file, as
// far as this package needs it, plus Keys: the document's COMPLETE raw
// top-level key set, sorted.
//
// Keys is the whitelist net one level ABOVE workflowJob.Keys, and it exists
// because a clean job mapping and clean steps are still not enough: GitHub
// offers the same family of bypasses a third time, at document scope, where
// they apply to every job and every step at once. A workflow-level
//
//	env:
//	  MAKEFLAGS: -i
//
// is the sharpest one: `-i` makes GNU make ignore every failed recipe and
// exit 0, so `make verify` reports success over failing unit, race, fixture,
// and e2e verdicts and the required context goes green over a red gate —
// while every assertion about triggers, jobs, steps, and `run:` text stays
// perfectly satisfied, because nothing about them changed. `defaults: run:
// shell:`/`working-directory:` redirect what the gate commands execute,
// `concurrency: cancel-in-progress:` can cancel the required context out of
// existence mid-run, and `permissions:` widens the token every step holds.
// Asserting the top-level key set is exactly {jobs, name, on} closes all of
// those and every future sibling in one assertion.
//
// Concurrency is the document-level `concurrency:` block, nil when absent.
type workflowDoc struct {
	Name        string
	On          workflowTriggers
	Jobs        map[string]workflowJob
	Concurrency *workflowConcurrency
	Keys        []string
}

// workflowConcurrency models a `concurrency:` block in either of its two
// forms: the bare group string (`concurrency: some-group`) or the mapping
// form with `group:` and `cancel-in-progress:`. CancelInProgress is nil when
// the key is absent (GitHub then keeps an in-progress run), and otherwise
// points at the decoded boolean; a non-boolean value (an expression) leaves
// it nil and is visible in Keys.
type workflowConcurrency struct {
	Group            string
	CancelInProgress *bool
	Keys             []string
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
	doc.Keys = sortedKeys(top)
	if name, ok := asStringVal(top["name"]); ok {
		doc.Name = name
	}
	doc.On = decodeTriggers(top["on"])
	doc.Jobs = decodeJobs(top["jobs"])
	if c, present := top["concurrency"]; present {
		conc := decodeConcurrency(c)
		doc.Concurrency = &conc
	}
	return doc
}

// decodeConcurrency handles both `concurrency:` forms (see
// workflowConcurrency).
func decodeConcurrency(v interface{}) workflowConcurrency {
	if group, ok := v.(string); ok {
		return workflowConcurrency{Group: group}
	}
	m, ok := asMap(v)
	if !ok {
		return workflowConcurrency{}
	}
	conc := workflowConcurrency{Keys: sortedKeys(m)}
	if group, ok := asStringVal(m["group"]); ok {
		conc.Group = group
	}
	if cancel, ok := m["cancel-in-progress"].(bool); ok {
		conc.CancelInProgress = &cancel
	}
	return conc
}

func decodeTriggers(v interface{}) workflowTriggers {
	m, ok := asMap(v)
	if !ok {
		return workflowTriggers{}
	}
	triggers := workflowTriggers{Keys: sortedKeys(m)}
	if pr, present := m["pull_request"]; present {
		tf := decodeTriggerFilter(pr)
		triggers.PullRequest = &tf
	}
	if push, present := m["push"]; present {
		tf := decodeTriggerFilter(push)
		triggers.Push = &tf
	}
	if wd, present := m["workflow_dispatch"]; present {
		t := decodeWorkflowDispatchTrigger(wd)
		triggers.WorkflowDispatch = &t
	}
	if wc, present := m["workflow_call"]; present {
		tf := decodeTriggerFilter(wc)
		triggers.WorkflowCall = &tf
	}
	return triggers
}

// decodeWorkflowDispatchTrigger handles on.workflow_dispatch's shape: a
// bare `workflow_dispatch:`/`workflow_dispatch: {}` (no inputs) or a real
// `inputs:` mapping, each entry decoded into a workflowDispatchInput.
func decodeWorkflowDispatchTrigger(v interface{}) workflowDispatchTrigger {
	m, ok := asMap(v)
	if !ok {
		return workflowDispatchTrigger{}
	}
	trigger := workflowDispatchTrigger{Keys: sortedKeys(m)}
	inputsMap, ok := asMap(m["inputs"])
	if !ok {
		return trigger
	}
	trigger.Inputs = make(map[string]workflowDispatchInput, len(inputsMap))
	for id, iv := range inputsMap {
		im, ok := asMap(iv)
		if !ok {
			continue
		}
		var input workflowDispatchInput
		if req, ok := im["required"].(bool); ok {
			input.Required = req
		}
		if typ, ok := asStringVal(im["type"]); ok {
			input.Type = typ
		}
		if desc, ok := asStringVal(im["description"]); ok {
			input.Description = desc
		}
		trigger.Inputs[id] = input
	}
	return trigger
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
		Branches:       asStringSlice(m["branches"]),
		BranchesIgnore: asStringSlice(m["branches-ignore"]),
		Tags:           asStringSlice(m["tags"]),
		Paths:          asStringSlice(m["paths"]),
		Keys:           sortedKeys(m),
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
	if uses, ok := asStringVal(m["uses"]); ok {
		job.Uses = uses
	}
	if needs, ok := m["needs"].(string); ok {
		job.Needs = []string{needs}
	} else {
		job.Needs = asStringSlice(m["needs"])
	}
	if cond, ok := asStringVal(m["if"]); ok {
		job.If = cond
	}
	if runsOn, ok := asStringVal(m["runs-on"]); ok {
		job.RunsOn = runsOn
	}
	// environment: takes either the bare-string form (`environment: close`)
	// or the `{name, url}` mapping form — both name the SAME environment.
	if env, ok := asStringVal(m["environment"]); ok {
		job.Environment = env
	} else if envMap, ok := asMap(m["environment"]); ok {
		if name, ok := asStringVal(envMap["name"]); ok {
			job.Environment = name
		}
	}
	if permMap, ok := asMap(m["permissions"]); ok {
		job.Permissions = make(map[string]string, len(permMap))
		for k, pv := range permMap {
			if s, ok := asStringVal(pv); ok {
				job.Permissions[k] = s
			}
		}
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
	step.Keys = sortedKeys(m)
	if name, ok := asStringVal(m["name"]); ok {
		step.Name = name
	}
	if uses, ok := asStringVal(m["uses"]); ok {
		step.Uses = uses
	}
	if run, ok := asStringVal(m["run"]); ok {
		step.Run = run
	}
	step.With = decodeStringMap(m["with"])
	step.Env = decodeStringMap(m["env"])
	return step
}

// decodeStringMap renders a decoded YAML mapping of scalars (a step's
// `with:` or `env:`) as name -> raw value text, or nil when v is not a
// mapping (the key was absent).
func decodeStringMap(v interface{}) map[string]string {
	raw, ok := asMap(v)
	if !ok {
		return nil
	}
	out := make(map[string]string, len(raw))
	for k, val := range raw {
		if s, ok := asStringVal(val); ok {
			out[k] = s
		}
	}
	return out
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

// findExactRunSteps returns the index of every step whose Run, trimmed of
// surrounding whitespace, EQUALS cmd.
//
// Exact equality, not findRunStep's substring containment, is the point: a
// substring match accepts `make verify || true`, `make verify &`, `#make
// verify`, or `make verify --dry-run` as proof that the gate runs, when each
// of those lets the step (and the required context) succeed without the gate
// having passed. Whitespace is trimmed because a YAML block scalar (`run: |`)
// carries a trailing newline the author never typed; nothing inside the
// command is normalized.
//
// Returns every match, not the first, so callers can also refuse a duplicate:
// two `make verify` steps would make "which one is the gate" ambiguous, and
// the ordering assertions below have no meaning against an ambiguous index.
func findExactRunSteps(steps []workflowStep, cmd string) []int {
	var out []int
	for i := range steps {
		if strings.TrimSpace(steps[i].Run) == cmd {
			out = append(out, i)
		}
	}
	return out
}

// runCommands returns the trimmed command of every `run:` step, for failure
// messages: when an exact-match lookup finds nothing, what the file actually
// says is the one thing a reader needs.
func runCommands(steps []workflowStep) []string {
	var out []string
	for i := range steps {
		if cmd := strings.TrimSpace(steps[i].Run); cmd != "" {
			out = append(out, cmd)
		}
	}
	return out
}

// keysOutside returns the members of keys not present in allowed, sorted —
// i.e. exactly what a whitelist assertion should name in its failure message.
func keysOutside(keys, allowed []string) []string {
	var extra []string
	for _, k := range keys {
		if !slices.Contains(allowed, k) {
			extra = append(extra, k)
		}
	}
	slices.Sort(extra)
	return extra
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
		// merge-gate.yml's lint runs in its static-checks job (SI-266);
		// TestMergeGateGateJobsUsePinnedSetup proves `make lint` runs there.
		{"merge-gate.yml", "merge-gate.yml", mergeGateLintJob},
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

// TestMergeGateTopLevelKeysAreWhitelisted is the outermost of this file's
// three whitelist nets (document → job → step). It proves merge-gate.yml
// declares exactly `name:`, `on:`, and `jobs:` at top level and nothing else.
//
// A whitelist rather than a list of named negatives, for the same reason the
// job- and step-level nets are: the top level is where a single added key
// changes the behaviour of every job and every step at once, invisibly to
// every other assertion in this file. The concrete case that motivated it: a
// workflow-level
//
//	env:
//	  MAKEFLAGS: -i
//
// makes GNU make ignore failed recipes and exit 0, so the `run: make verify`
// step this file pins by exact text still runs exactly `make verify` — and
// still succeeds while unit, race, fixture, and e2e verdicts fail beneath it.
// The required context would go green over a red gate with the workflow's
// triggers, job shape, step shape, and command text all still perfect.
// `defaults:` (run shell/working-directory), `concurrency:` (a
// cancel-in-progress rule can cancel the required context out of existence),
// and `permissions:` are the other siblings this one assertion closes,
// together with whatever GitHub adds next.
//
// Widening this set is a deliberate act that must be argued for right here.
// A future considered addition — `permissions: contents: read`, narrowing the
// job token, is the plausible one — is legitimate, but it must be added to
// this whitelist consciously and under review, not discovered green.
func TestMergeGateTopLevelKeysAreWhitelisted(t *testing.T) {
	doc := decodeWorkflow(t, mergeGatePath(verdiRepoRoot))

	wantTopKeys := []string{"jobs", "name", "on"}
	if !slices.Equal(doc.Keys, wantTopKeys) {
		t.Errorf("merge-gate.yml: the document must declare exactly the top-level keys %v and nothing else, got %v (extra: %v) — a workflow-level `env:` (e.g. MAKEFLAGS=-i, which makes make ignore failed recipes and exit 0), `defaults:`, `concurrency:`, or `permissions:` can each make the required %q context report green over a failing gate, run something other than the gate, or vanish mid-run, with every other assertion in this file still satisfied", wantTopKeys, doc.Keys, keysOutside(doc.Keys, wantTopKeys), "merge-gate")
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

// merge-gate.yml's parallel shape (SI-266, owner directive 2026-09-24). The
// gate runs as parallel jobs — one per group of `make verify` steps — and one
// aggregator job, keyed exactly `merge-gate`, is the required status check.
// The tests below prove, from the workflow source alone:
//
//   - every job's key set is whitelisted, and no job renames its context;
//   - the aggregator needs every gate job, runs `if: always()`, and decides
//     through the committed verdict script with pinned text (item 4c);
//   - the gate jobs together run exactly `make verify`'s steps plus the
//     post-verify self-lint, each once, and nothing else (item 4a);
//   - each gate job carries today's pinned setup;
//   - every step's key set is whitelisted.
//
// Each old single-job bypass is still closed, one level at a time: a
// workflow-level `env:`/`defaults:`/`concurrency:`/`permissions:`
// (TestMergeGateTopLevelKeysAreWhitelisted); a `name:` override, a matrix, a
// job-level `uses:` or `continue-on-error:`, and any `if:` other than the
// aggregator's exact `always()` (TestMergeGateJobsAreWhitelisted); a step's
// `if:`/`continue-on-error:`/`env:` (TestMergeGateStepsAreWhitelisted); and
// `|| true` or any other command beside the gate's own
// (TestMergeGateParity_GateJobsRunExactlyVerifySteps).

// mergeGateAggregatorJob is the required status check. A job with no `name:`
// override reports under its key, so the branch ruleset's "merge-gate"
// context is exactly this job.
const mergeGateAggregatorJob = "merge-gate"

// mergeGateLintJob is the gate job that runs the static checks, `make lint`
// among them, and so carries the pinned golangci-lint install.
const mergeGateLintJob = "static"

// mergeGateVerdictRun is the aggregator's decision step, pinned exactly: one
// `<job>=<result>` argument per gate job, sorted by job key. A job missing
// from `needs:` expands to an empty result, which the script fails.
const mergeGateVerdictRun = "scripts/merge-gate-verdict.sh" +
	" e2e=${{ needs.e2e.result }}" +
	" spec-align=${{ needs.spec-align.result }}" +
	" static=${{ needs.static.result }}" +
	" test-cmd=${{ needs.test-cmd.result }}" +
	" test-cross=${{ needs.test-cross.result }}" +
	" test-rest=${{ needs.test-rest.result }}"

// mergeGatePostVerifyCommands are the steps that ran after `make verify` in
// the single-job gate: build the binary, then lint this repo's store with it
// in the pull-request context. They stay on the required path as the final
// two steps of one gate job.
var mergeGatePostVerifyCommands = []string{
	"go build -o .build/verdi ./cmd/verdi",
	"./.build/verdi lint",
}

// verifyGateFloor is every gate `make verify` ran when the gate went parallel,
// with `test` expanded to its shards. The gate grows and never shrinks, so
// expanded VERIFY_STEPS must keep every one of these; adding a gate needs no
// edit here, and removing one fails.
var verifyGateFloor = []string{
	"build", "fmt-check", "vet", "lint",
	"test-cmd", "test-cross", "test-rest",
	"fixture", "lint-store", "spec-align", "lint-showcase", "showcase-coverage", "e2e",
}

// jobKeys returns the job ids of jobs, sorted so comparisons and failure
// messages are deterministic.
func jobKeys(jobs map[string]workflowJob) []string {
	keys := make([]string, 0, len(jobs))
	for k := range jobs {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// gateJobKeys returns every job id except the aggregator's, sorted.
func gateJobKeys(jobs map[string]workflowJob) []string {
	var keys []string
	for _, k := range jobKeys(jobs) {
		if k != mergeGateAggregatorJob {
			keys = append(keys, k)
		}
	}
	return keys
}

// golangciInstallRun is the pinned golangci-lint install step's exact command.
// Its `||` skips the install on a cache hit; it is the one command a gate job
// may run besides the gate's own.
func golangciInstallRun(pin string) string {
	return "test -x \"$(go env GOPATH)/bin/golangci-lint\" || \\\n" +
		"  go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@" + pin + "\n" +
		"echo \"$(go env GOPATH)/bin\" >> \"$GITHUB_PATH\""
}

// pinnedSetupActions maps each action a gate job may use to its exact
// `with:` inputs — today's pinned setup. A `with:` that differs (a checkout
// `ref:` naming other code, a different Go) changes what the gate proves.
func pinnedSetupActions(pin string) map[string]map[string]string {
	return map[string]map[string]string{
		"actions/checkout@v4": {"fetch-depth": "0"},
		"actions/setup-go@v5": {"go-version": "1.25"},
		"actions/setup-node@v4": {
			"node-version":          "22",
			"cache":                 "npm",
			"cache-dependency-path": "e2e/package-lock.json",
		},
		"actions/cache@v4": {
			"path": "~/go/bin/golangci-lint",
			"key":  "golangci-lint-${{ runner.os }}-" + pin,
		},
	}
}

// TestMergeGateJobsAreWhitelisted is the job-level whitelist net. Every job
// runs on ubuntu-latest with no `name:` override. A gate job declares exactly
// `runs-on` and `steps`; the aggregator adds exactly `needs` and `if`.
//
// A whitelist, not named negatives: `name:` renames a context, `strategy:
// matrix:` renames it to "merge-gate (…)", a job-level `uses:` renames it to
// "merge-gate / <inner-job>", `continue-on-error:` reports green over a
// failure, and an `if:` on a gate job skips it. The aggregator's `if:` is the
// one exception, and TestMergeGateAggregatorDecidesOverEveryGateJob pins it
// to exactly `always()`. Widening either set must be argued for here.
func TestMergeGateJobsAreWhitelisted(t *testing.T) {
	doc := decodeWorkflow(t, mergeGatePath(verdiRepoRoot))
	if _, ok := doc.Jobs[mergeGateAggregatorJob]; !ok {
		t.Fatalf("merge-gate.yml: no %q job — the required context must be a job keyed exactly %q; found %v", mergeGateAggregatorJob, mergeGateAggregatorJob, jobKeys(doc.Jobs))
	}
	if len(gateJobKeys(doc.Jobs)) == 0 {
		t.Fatalf("merge-gate.yml: no gate jobs beside %q; found %v", mergeGateAggregatorJob, jobKeys(doc.Jobs))
	}
	for _, key := range jobKeys(doc.Jobs) {
		job := doc.Jobs[key]
		if job.Name != "" {
			t.Errorf("merge-gate.yml: job %q must not declare a `name:` override (it renames the reported context), got %q", key, job.Name)
		}
		if job.RunsOn != "ubuntu-latest" {
			t.Errorf("merge-gate.yml: job %q runs-on %q, want ubuntu-latest", key, job.RunsOn)
		}
		want := []string{"runs-on", "steps"}
		if key == mergeGateAggregatorJob {
			want = []string{"if", "needs", "runs-on", "steps"}
		}
		if !slices.Equal(job.Keys, want) {
			t.Errorf("merge-gate.yml: job %q must declare exactly the keys %v, got %v (extra: %v) — name/if/strategy/uses/continue-on-error can each make a gate skip, rename, or report green over a failure", key, want, job.Keys, keysOutside(job.Keys, want))
		}
	}
}

// TestMergeGateAggregatorDecidesOverEveryGateJob proves contract item 4c. The
// aggregator `needs:` exactly the set of gate jobs, so no gate can be left
// off the required path. Its `if:` is exactly `always()`: without it, a
// failed dependency would make GitHub skip the aggregator, and a skipped
// required check does not block a merge. Its steps are exactly a checkout
// and the committed verdict script, called with the pinned text, which
// passes one `<job>=<result>` argument per needed job and fails unless every
// result is `success`.
func TestMergeGateAggregatorDecidesOverEveryGateJob(t *testing.T) {
	doc := decodeWorkflow(t, mergeGatePath(verdiRepoRoot))
	agg, ok := doc.Jobs[mergeGateAggregatorJob]
	if !ok {
		t.Fatalf("merge-gate.yml: no %q job found", mergeGateAggregatorJob)
	}
	gates := gateJobKeys(doc.Jobs)

	needs := slices.Clone(agg.Needs)
	slices.Sort(needs)
	if !slices.Equal(needs, gates) {
		t.Errorf("merge-gate.yml: %q needs %v, want exactly the gate jobs %v — a gate job outside `needs:` is off the required path", mergeGateAggregatorJob, agg.Needs, gates)
	}
	if agg.If != "always()" {
		t.Errorf("merge-gate.yml: %q must run with `if: always()` exactly (a failed dependency otherwise skips it, and a skipped required check passes), got %q", mergeGateAggregatorJob, agg.If)
	}

	derived := mergeGateVerdictScript
	for _, g := range gates {
		derived += " " + g + "=${{ needs." + g + ".result }}"
	}
	if derived != mergeGateVerdictRun {
		t.Errorf("the pinned verdict command must pass exactly one argument per gate job:\n got pinned %q\nwant derived %q", mergeGateVerdictRun, derived)
	}

	if len(agg.Steps) != 2 {
		t.Fatalf("merge-gate.yml: %q must have exactly 2 steps (checkout, verdict), got %d: %+v", mergeGateAggregatorJob, len(agg.Steps), agg.Steps)
	}
	if co := agg.Steps[0]; co.Uses != "actions/checkout@v4" || len(co.With) != 0 {
		t.Errorf("merge-gate.yml: %q step 0 must be a plain actions/checkout@v4, got uses %q with %v", mergeGateAggregatorJob, co.Uses, co.With)
	}
	if got := strings.TrimSpace(agg.Steps[1].Run); got != mergeGateVerdictRun {
		t.Errorf("merge-gate.yml: %q decision step must run exactly\n  %q\ngot\n  %q", mergeGateAggregatorJob, mergeGateVerdictRun, got)
	}

	script := filepath.Join(verdiRepoRoot, filepath.FromSlash(mergeGateVerdictScript))
	info, err := os.Stat(script)
	if err != nil {
		t.Fatalf("verdict script %s: %v", mergeGateVerdictScript, err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Errorf("verdict script %s is not executable (mode %v); the workflow runs it directly", mergeGateVerdictScript, info.Mode())
	}
}

// TestMergeGateParity_GateJobsRunExactlyVerifySteps proves contract item 4a,
// the trust-parity reading SI-266 records: CI runs exactly `make verify`'s
// step set, split across jobs. The gate jobs' commands are exactly one
// `make <step>` per expanded VERIFY_STEPS entry, each once across the whole
// workflow, plus the two post-verify commands; the only other command a gate
// job may run is the pinned golangci-lint install. Nothing is dropped and
// nothing is extra: `make verify || true`, a step writing MAKEFLAGS=-i to
// $GITHUB_ENV, or a second run of a step all fail here. Within a job, make
// steps keep VERIFY_STEPS order, and the post-verify commands are that job's
// final two steps, as they were the single job's.
func TestMergeGateParity_GateJobsRunExactlyVerifySteps(t *testing.T) {
	makefile := readMakefile(t)
	steps := expandedVerifySteps(t, makefile)
	for _, gate := range verifyGateFloor {
		if !slices.Contains(steps, gate) {
			t.Errorf("make verify no longer runs %q (expanded VERIFY_STEPS %v) — the gate grows, never shrinks", gate, steps)
		}
	}
	seen := map[string]bool{}
	for _, s := range steps {
		if seen[s] {
			t.Errorf("expanded VERIFY_STEPS runs %q twice: %v", s, steps)
		}
		seen[s] = true
	}

	doc := decodeWorkflow(t, mergeGatePath(verdiRepoRoot))
	install := golangciInstallRun(makefileGolangciPin(t))
	ranIn := map[string][]string{}
	for _, key := range gateJobKeys(doc.Jobs) {
		var order []int
		for i, step := range doc.Jobs[key].Steps {
			if step.Uses != "" {
				continue
			}
			cmd := strings.TrimSpace(step.Run)
			target, isMake := strings.CutPrefix(cmd, "make ")
			switch {
			case cmd == install:
			case slices.Contains(mergeGatePostVerifyCommands, cmd):
				ranIn[cmd] = append(ranIn[cmd], key)
			case isMake && slices.Contains(steps, target):
				ranIn[cmd] = append(ranIn[cmd], key)
				order = append(order, slices.Index(steps, target))
			default:
				t.Errorf("merge-gate.yml: job %q step %d runs %q, which is not a make verify step, a post-verify step, or the pinned golangci-lint install — nothing else may run in a gate job", key, i, cmd)
			}
		}
		if !slices.IsSorted(order) {
			t.Errorf("merge-gate.yml: job %q runs its make steps out of VERIFY_STEPS order %v", key, steps)
		}
	}

	want := make([]string, 0, len(steps)+len(mergeGatePostVerifyCommands))
	for _, s := range steps {
		want = append(want, "make "+s)
	}
	want = append(want, mergeGatePostVerifyCommands...)
	for _, cmd := range want {
		if n := len(ranIn[cmd]); n != 1 {
			t.Errorf("merge-gate.yml: %q runs in %d gate jobs %v, want exactly 1", cmd, n, ranIn[cmd])
		}
	}

	postJob := ranIn[mergeGatePostVerifyCommands[0]]
	if len(postJob) != 1 {
		return
	}
	jobSteps := doc.Jobs[postJob[0]].Steps
	n := len(mergeGatePostVerifyCommands)
	if len(jobSteps) < n {
		t.Fatalf("merge-gate.yml: job %q has %d steps, fewer than the %d post-verify commands", postJob[0], len(jobSteps), n)
	}
	for i, cmd := range mergeGatePostVerifyCommands {
		if got := strings.TrimSpace(jobSteps[len(jobSteps)-n+i].Run); got != cmd {
			t.Errorf("merge-gate.yml: job %q's final %d steps must be %v in order; step %d runs %q", postJob[0], n, mergeGatePostVerifyCommands, len(jobSteps)-n+i, got)
		}
	}
}

// mergeGateReplayFloor pins the one cache replay `make verify` relies on
// today: the fixture gate re-runs fixturegit, corpus, and svcfixcanned after
// test-rest has executed them. If the Makefile stops producing that pair,
// this test fails rather than passing over nothing; update the pin here when
// the change is deliberate.
var mergeGateReplayFloor = map[string]string{"fixture": "test-rest"}

// TestMergeGateParity_CacheReplaysRunAfterTheirExecutorInOneJob keeps the
// SI-266 promise that no test package runs twice once `make verify` is split
// into jobs. In `make verify`, a step that re-runs packages an earlier step
// already executed replays them from the Go test cache
// (TestGateShards_VerifyExecutesEachPackageOnceUnderRace). Each CI job starts
// on a fresh runner with a cold test cache, so that replay holds only if the
// re-running step runs in the same gate job as the executing step, and after
// it. Anywhere else, the packages execute a second time.
func TestMergeGateParity_CacheReplaysRunAfterTheirExecutorInOneJob(t *testing.T) {
	replays := verifyCacheReplays(t, readMakefile(t))
	for step, executor := range mergeGateReplayFloor {
		if !slices.Contains(replays[step], executor) {
			t.Errorf("make verify's %s step no longer replays packages %s executed (replays %v); this test pins that pair, so update mergeGateReplayFloor if the change is deliberate", step, executor, replays)
		}
	}

	doc := decodeWorkflow(t, mergeGatePath(verdiRepoRoot))
	type position struct {
		job   string
		index int
	}
	where := map[string]position{}
	for _, key := range gateJobKeys(doc.Jobs) {
		for i, step := range doc.Jobs[key].Steps {
			if target, ok := strings.CutPrefix(strings.TrimSpace(step.Run), "make "); ok {
				where[target] = position{job: key, index: i}
			}
		}
	}
	for _, step := range slices.Sorted(maps.Keys(replays)) {
		r, ok := where[step]
		if !ok {
			t.Errorf("merge-gate.yml: no gate job runs make %s", step)
			continue
		}
		for _, executor := range replays[step] {
			e, ok := where[executor]
			switch {
			case !ok:
				t.Errorf("merge-gate.yml: no gate job runs make %s", executor)
			case e.job != r.job:
				t.Errorf("merge-gate.yml: make %s runs in job %q but make %s runs in job %q, so the packages make %s re-runs execute a second time on %q's cold runner; run make %s in job %q, after make %s", step, r.job, executor, e.job, step, r.job, step, e.job, executor)
			case r.index < e.index:
				t.Errorf("merge-gate.yml: job %q runs make %s (step %d) before make %s (step %d); it must run after it to replay its packages from the cache", r.job, step, r.index, executor, e.index)
			}
		}
	}
}

// TestMergeGateGateJobsUsePinnedSetup proves each gate job carries today's
// pinned setup: it starts with a full-history checkout and Go 1.25, uses no
// action outside the pinned set, passes each action exactly its pinned
// inputs, finishes setup before its first gate command, installs Node 22
// wherever `make e2e` runs, and caches and installs the pinned golangci-lint
// wherever `make lint` runs — which must be the static job.
func TestMergeGateGateJobsUsePinnedSetup(t *testing.T) {
	doc := decodeWorkflow(t, mergeGatePath(verdiRepoRoot))
	pin := makefileGolangciPin(t)
	actions := pinnedSetupActions(pin)
	install := golangciInstallRun(pin)
	gates := gateJobKeys(doc.Jobs)
	if len(gates) == 0 {
		t.Fatalf("merge-gate.yml: no gate jobs found; jobs %v", jobKeys(doc.Jobs))
	}
	for _, key := range gates {
		steps := doc.Jobs[key].Steps
		if len(steps) < 2 || steps[0].Uses != "actions/checkout@v4" || steps[1].Uses != "actions/setup-go@v5" {
			t.Errorf("merge-gate.yml: job %q must begin with actions/checkout@v4 then actions/setup-go@v5", key)
		}
		lastSetup, firstGate := -1, len(steps)
		for i, step := range steps {
			switch {
			case step.Uses != "":
				want, ok := actions[step.Uses]
				if !ok {
					t.Errorf("merge-gate.yml: job %q step %d uses %q, outside the pinned setup actions", key, i, step.Uses)
				} else if !maps.Equal(step.With, want) {
					t.Errorf("merge-gate.yml: job %q step %d (%s) has with: %v, want exactly %v", key, i, step.Uses, step.With, want)
				}
				lastSetup = i
			case strings.TrimSpace(step.Run) == install:
				lastSetup = i
			case firstGate == len(steps):
				firstGate = i
			}
		}
		if lastSetup > firstGate {
			t.Errorf("merge-gate.yml: job %q has a setup step (index %d) after its first gate command (index %d)", key, lastSetup, firstGate)
		}
		runs := runCommands(steps)
		if slices.Contains(runs, "make e2e") && findStep(steps, "actions/setup-node@v4") == nil {
			t.Errorf("merge-gate.yml: job %q runs make e2e without actions/setup-node@v4 (Node 22)", key)
		}
		if slices.Contains(runs, "make lint") {
			if findCacheStep(steps, "golangci-lint") == nil || !slices.Contains(runs, install) {
				t.Errorf("merge-gate.yml: job %q runs make lint without the pinned golangci-lint cache and install steps", key)
			}
			if key != mergeGateLintJob {
				t.Errorf("merge-gate.yml: make lint runs in job %q, want %q (TestGolangciLintPinIsLockstepWithMakefile reads that job)", key, mergeGateLintJob)
			}
		}
	}
}

// TestMergeGateStepsAreWhitelisted is the step-level whitelist net, over
// every job. An action step may carry {uses, with, name} and a command step
// {run, name}: `continue-on-error: true` reports a step green over a failed
// gate, `if:` skips it, and `env:`/`shell:`/`working-directory:` change what
// it runs, so none may appear. No step may suffix `|| true`, and evidence
// production and upload stay verify.yml's push-only duty.
func TestMergeGateStepsAreWhitelisted(t *testing.T) {
	doc := decodeWorkflow(t, mergeGatePath(verdiRepoRoot))
	usesAllowed := []string{"name", "uses", "with"}
	runAllowed := []string{"name", "run"}
	for _, key := range jobKeys(doc.Jobs) {
		for i, step := range doc.Jobs[key].Steps {
			hasUses := slices.Contains(step.Keys, "uses")
			hasRun := slices.Contains(step.Keys, "run")
			switch {
			case hasUses == hasRun:
				t.Errorf("job %q step %d (name %q): must carry exactly one of `uses:` or `run:`, got keys %v", key, i, step.Name, step.Keys)
			case hasUses:
				if extra := keysOutside(step.Keys, usesAllowed); len(extra) != 0 {
					t.Errorf("job %q step %d (uses %q): key(s) %v are not whitelisted — an action step may declare only %v", key, i, step.Uses, extra, usesAllowed)
				}
				if strings.HasPrefix(step.Uses, "actions/upload-artifact") {
					t.Errorf("job %q step %d uploads an artifact (%q) — evidence upload stays verify.yml's push-only duty", key, i, step.Uses)
				}
			case hasRun:
				if extra := keysOutside(step.Keys, runAllowed); len(extra) != 0 {
					t.Errorf("job %q step %d (run %q): key(s) %v are not whitelisted — a command step may declare only %v", key, i, strings.TrimSpace(step.Run), extra, runAllowed)
				}
				compact := strings.Join(strings.Fields(step.Run), " ")
				if strings.Contains(compact, "|| true") || strings.Contains(compact, "||true") {
					t.Errorf("job %q step %d (run %q) suffixes `|| true`, which reports the step green whatever the gate says", key, i, strings.TrimSpace(step.Run))
				}
				if strings.Contains(step.Run, "verdi sync --produce") {
					t.Errorf("job %q step %d runs evidence production (%q) — that stays verify.yml's push-only duty", key, i, strings.TrimSpace(step.Run))
				}
			}
		}
	}
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
