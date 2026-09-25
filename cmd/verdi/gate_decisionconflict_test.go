package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/align"
	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/objsupersede/scenario"
	"github.com/jyang234/verdi/internal/policyconflict"
	"github.com/jyang234/verdi/internal/store"
)

// writeDecisionConflictReport writes decision-conflict-report.md directly
// to the working tree, mirroring gate_test.go's writeGateReport for the
// build-branch deviation report.
func writeDecisionConflictReport(t *testing.T, root, covers, findingsYAML string) {
	t.Helper()
	dir := filepath.Join(root, ".verdi", "specs", "active", "stale-decline")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	content := "---\nschema: verdi.decisionconflict/v1\ncovers: " + covers + "\nfindings:\n" + findingsYAML + "digest: sha256:" + repeatZero(64) + "\n---\n# Decision-conflict report\n"
	if err := os.WriteFile(filepath.Join(dir, "decision-conflict-report.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("writing decision-conflict-report.md: %v", err)
	}
}

func repeatZero(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = '0'
	}
	return string(b)
}

const gdcHeadCommit = "0000000000000000000000000000000000000c"

func TestCheckDeclaredDecisionConflicts_NoReport(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cond, err := checkDeclaredDecisionConflicts(context.Background(), root, "stale-decline", gdcHeadCommit)
	if err != nil {
		t.Fatalf("checkDeclaredDecisionConflicts: %v", err)
	}
	if cond.OK {
		t.Fatal("OK = true, want false (no report at all)")
	}
}

func TestCheckDeclaredDecisionConflicts_StaleCovers(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeDecisionConflictReport(t, root, "0000000000000000000000000000000000000b",
		"  - { id: f-1, kind: computed, text: t, disposition: exempt, note: n }\n")
	cond, err := checkDeclaredDecisionConflicts(context.Background(), root, "stale-decline", gdcHeadCommit)
	if err != nil {
		t.Fatalf("checkDeclaredDecisionConflicts: %v", err)
	}
	if cond.OK {
		t.Fatal("OK = true, want false (stale covers)")
	}
}

func TestCheckDeclaredDecisionConflicts_UndispositionedFails(t *testing.T) {
	t.Parallel()
	repo := buildDesignGateRepo(t)
	writeDecisionConflictReport(t, repo.Dir, repo.Head,
		"  - { id: f-1, kind: judged, text: t }\n")
	cond, err := checkDeclaredDecisionConflicts(context.Background(), repo.Dir, "stale-decline", repo.Head)
	if err != nil {
		t.Fatalf("checkDeclaredDecisionConflicts: %v", err)
	}
	if cond.OK {
		t.Fatal("OK = true, want false (undispositioned judged finding)")
	}
}

// buildDesignGateRepo builds a fixturegit repo whose spec (a draft feature
// spec — a legitimate design-branch spec, ResolveDesignSpec accepts feature
// and story class alike) lives at specs/active/stale-decline, then checks
// out the design/stale-decline branch `verdi design start` would have cut.
func buildDesignGateRepo(t *testing.T) *fixturegit.Repo {
	return buildDesignGateRepoWith(t, gateSpecMD("draft"), nil)
}

// buildDesignGateRepoWith is buildDesignGateRepo with the given design spec
// and extra files (an ADR a declared edge targets).
func buildDesignGateRepoWith(t *testing.T, spec string, extra map[string]string) *fixturegit.Repo {
	t.Helper()
	files := map[string]string{
		".verdi/verdi.yaml":                         "schema: verdi.layout/v1\nforge: gitlab\n",
		".verdi/specs/active/stale-decline/spec.md": spec,
	}
	for p, c := range extra {
		files[p] = c
	}
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: files, Message: "scaffold + draft design spec"}})
	checkoutBranch(t, repo.Dir, "design/stale-decline")
	return repo
}

// gdcEdgeSpecMD is a draft design spec whose decision dc-1 declares one
// exempts edge to adr/decline-policy, with note (a resolved edge once the
// ADR exists) or without.
func gdcEdgeSpecMD(note string) string {
	link := "{ type: exempts, ref: adr/decline-policy }"
	if note != "" {
		link = "{ type: exempts, ref: adr/decline-policy, note: \"" + note + "\" }"
	}
	return strings.Replace(gateSpecMD("draft"), "\n---\n# body", "\ndecisions:\n  - { id: dc-1, text: \"decline stale requests\", anchor: \"#dc-1\", links: ["+link+"] }\n---\n# body", 1)
}

const gdcDeclinePolicyADR = "---\nid: adr/decline-policy\nkind: adr\ntitle: \"Decline policy\"\nstatus: accepted\nowners: [platform-team]\ndecided: 2026-01-01\nfrozen: { at: 2026-01-01, commit: 3e91ab2 }\n---\nbody\n"

// alignDesignGateRepo runs the in-process design-branch align on repo and
// dispositions the judged absence finding by hand, returning the report path.
func alignDesignGateRepo(t *testing.T, dir string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if got := runDesignAlign(context.Background(), dir, false, alignDeps{ModelDigest: testResolveModelDigest(t, dir)}, &stdout, &stderr); got != 0 {
		t.Fatalf("runDesignAlign = %d; stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
	path := store.DecisionConflictReportPath(dir, store.ZoneActive, "stale-decline")
	dispositionJudged(t, path)
	return path
}

// dispositionJudged dispositions every judged finding of the report at path
// by hand, as a reviewer does.
func dispositionJudged(t *testing.T, path string) {
	t.Helper()
	editDecisionReport(t, path, func(fm *artifact.DecisionConflictFrontmatter) {
		for i := range fm.Findings {
			if fm.Findings[i].Kind == artifact.FindingJudged {
				fm.Findings[i].Disposition, fm.Findings[i].Note = artifact.ConflictNoConflict, "reviewed: the sweep was skipped by configuration"
			}
		}
	})
}

// TestSpecMRGate_DanglingExemptsFails proves the spec-MR path fails the gate
// (exit 1, naming the declared-decision-conflict condition) when the
// decision-conflict report carries a dangling declared edge — an `exempts`
// edge to an ADR that does not exist, whose computed finding align leaves
// undispositioned.
func TestSpecMRGate_DanglingExemptsFails(t *testing.T) {
	t.Parallel()
	repo := buildDesignGateRepoWith(t, gdcEdgeSpecMD("excused, see witness"), nil)
	alignDesignGateRepo(t, repo.Dir)

	var stdout, stderr bytes.Buffer
	got := runSpecMRGate(context.Background(), repo.Dir, "design/stale-decline", nil, "main", &stdout, &stderr)
	if got != 1 {
		t.Fatalf("runSpecMRGate = %d, want 1; stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "undispositioned/unresolved finding(s): [edge-dc-1-exempts-adr--decline-policy]") {
		t.Fatalf("stdout = %q, want it to name the declared-decision-conflict condition's unresolved edge", stdout.String())
	}
	if !strings.Contains(stdout.String(), "gate: FAIL") {
		t.Fatalf("stdout = %q, want a final gate: FAIL line", stdout.String())
	}
}

// TestSpecMRGate_ResolvedPasses proves the same path passes (exit 0) once
// every declared edge is resolved (its computed finding dispositioned by
// computation, and equal to the gate's recompute) and every judged finding
// dispositioned — with a nil forge, the review-thread condition
// (gate_threads.go) discloses unproven (rendered through the shared
// internal/disclosure seam, spec/disclosure-seam-v2 ac-1 — never a silent
// pass, constitution 2/10) rather than either failing the gate or being
// silently skipped.
func TestSpecMRGate_ResolvedPasses(t *testing.T) {
	t.Parallel()
	repo := buildDesignGateRepoWith(t, gdcEdgeSpecMD("excused, see witness"), map[string]string{".verdi/adr/decline-policy.md": gdcDeclinePolicyADR})
	alignDesignGateRepo(t, repo.Dir)

	var stdout, stderr bytes.Buffer
	got := runSpecMRGate(context.Background(), repo.Dir, "design/stale-decline", nil, "main", &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runSpecMRGate = %d, want 0; stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "gate: PASS") {
		t.Fatalf("stdout = %q, want a final gate: PASS line", stdout.String())
	}
	if !strings.Contains(stdout.String(), "disclosed-unproven [gate:review-threads-resolved]") {
		t.Fatalf("stdout = %q, want the shared internal/disclosure rendering disclosing the review-thread condition unproven (nil forge)", stdout.String())
	}
}

// TestCmdGate_SpecMR_EntryPoint drives the real `verdi gate` entry point on a
// design branch, proving cmdGate's branch-prefix dispatch into the spec-MR
// path works end to end (not just runSpecMRGate's core): a resolved report
// exits 0.
func TestCmdGate_SpecMR_EntryPoint(t *testing.T) {
	repo := buildDesignGateRepoWith(t, gdcEdgeSpecMD("n"), map[string]string{".verdi/adr/decline-policy.md": gdcDeclinePolicyADR})
	alignDesignGateRepo(t, repo.Dir)
	t.Chdir(repo.Dir)

	var stderr bytes.Buffer
	got := run([]string{"gate"}, &stderr)
	if got != 0 {
		t.Fatalf("run([gate]) on a design branch = %d, want 0; stderr=%s", got, stderr.String())
	}
}

func TestCheckDeclaredDecisionConflicts_AllResolvedPasses(t *testing.T) {
	t.Parallel()
	repo := buildDesignGateRepoWith(t, gdcEdgeSpecMD("n"), map[string]string{".verdi/adr/decline-policy.md": gdcDeclinePolicyADR})
	writeDecisionConflictReport(t, repo.Dir, repo.Head,
		"  - { id: edge-dc-1-exempts-adr--decline-policy, kind: computed, text: \"decision dc-1 exempts adr/decline-policy: resolved (EXEMPT)\", disposition: exempt, note: n, target_ref: adr/decline-policy, routed_owners: [platform-team] }\n  - { id: f-2, kind: judged, text: t2, disposition: no-conflict, note: n2 }\n")
	cond, err := checkDeclaredDecisionConflicts(context.Background(), repo.Dir, "stale-decline", repo.Head)
	if err != nil {
		t.Fatalf("checkDeclaredDecisionConflicts: %v", err)
	}
	if !cond.OK {
		t.Fatalf("OK = false (%s), want true (every declared edge resolved, every judged finding dispositioned)", cond.Reason)
	}
}

// TestSpecMRGateConflictPreEffect catches the design gate omitting the
// constitutional condition, constructing an accepted arm, or mutating any
// repository/report bytes before returning a block or operational failure.
func TestSpecMRGateConflictPreEffect(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		verdict  policyconflict.Verdict
		provider error
		wantCode int
	}{
		{name: "pass", verdict: policyconflict.VerdictPass, wantCode: 0},
		{name: "blocked violated", verdict: policyconflict.VerdictBlockedViolated, wantCode: 1},
		{name: "blocked unproven", verdict: policyconflict.VerdictBlockedUnproven, wantCode: 1},
		{name: "operational", provider: errors.New("provider unavailable"), wantCode: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := buildDesignGateRepo(t)
			writeDecisionConflictReport(t, repo.Dir, repo.Head,
				"  - { id: f-1, kind: judged, text: t, disposition: no-conflict, note: n }\n")
			installConflictPolicyStore(t, repo.Dir)
			requestPath := contextLifecycleRequestFile(t, repo.Dir, "design-context.json", "spec/stale-decline", contextcompile.PhaseDesign, nil)
			before := takeConflictLifecycleSnapshot(t, repo.Dir,
				".verdi/specs/active/stale-decline/decision-conflict-report.md",
				".verdi/specs/active/stale-decline/deviation-report.md",
			)

			calls := 0
			provider := contextConflictProviderFunc(func(_ context.Context, request policyconflict.Request) (policyconflict.Result, error) {
				calls++
				if request.Target.Kind != policyconflict.TargetAcceptanceCandidate || request.Target.AcceptanceCandidate == nil {
					t.Fatalf("design target = %+v, want acceptance-candidate", request.Target)
				}
				if tt.provider != nil {
					return policyconflict.Result{}, tt.provider
				}
				return lifecycleConflictResult(tt.verdict), nil
			})

			var stdout, stderr bytes.Buffer
			got := runSpecMRGateWithConflict(context.Background(), repo.Dir, "design/stale-decline", nil, "main", requestPath, provider, &stdout, &stderr)
			if got != tt.wantCode {
				t.Fatalf("runSpecMRGateWithConflict = %d, want %d; stdout=%s stderr=%s", got, tt.wantCode, stdout.String(), stderr.String())
			}
			if calls != 1 {
				t.Fatalf("provider calls = %d, want 1", calls)
			}
			assertConflictLifecycleSnapshot(t, repo.Dir, before)
			if tt.provider != nil {
				if stdout.Len() != 0 || !strings.Contains(stderr.String(), "provider unavailable") {
					t.Fatalf("operational stdout=%q stderr=%q", stdout.String(), stderr.String())
				}
				return
			}
			if !strings.Contains(stdout.String(), "3. constitutional conflict verdict") {
				t.Fatalf("stdout = %q, want numbered constitutional condition", stdout.String())
			}
			wantMarker := "[PASS] 3. constitutional conflict verdict"
			if tt.wantCode == 1 {
				wantMarker = "[FAIL] 3. constitutional conflict verdict"
			}
			if !strings.Contains(stdout.String(), wantMarker) {
				t.Fatalf("stdout = %q, want %q", stdout.String(), wantMarker)
			}
		})
	}
}

// Closed-spec object supersession finding ids over the scenario fixture
// (testdata/objsupersede).
const (
	gdcDC1 = "edge-dc-1-supersedes-spec--closed-feature-dc-1"
	gdcDC2 = "edge-dc-2-supersedes-spec--closed-story-ac-1"
)

func gdcGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// alignScenario builds the named objsupersede scenario, checks out branch
// (design/<spec>), runs the design-branch align in process, and
// dispositions the judged sweep's absence finding by hand — a judged
// finding, which humans disposition — so the gate turns on the computed
// section alone. It returns the repository and the report's path.
func alignScenario(t *testing.T, name, branch string) (dir, reportPath string) {
	t.Helper()
	dir = scenario.Build(t, name).Dir
	if branch != "" {
		gdcGit(t, dir, "checkout", "-q", branch)
	}
	var stdout, stderr bytes.Buffer
	if got := runDesignAlign(context.Background(), dir, false, alignDeps{ModelDigest: testResolveModelDigest(t, dir)}, &stdout, &stderr); got != 0 {
		t.Fatalf("runDesignAlign = %d; stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
	spec := strings.TrimPrefix(gdcGit(t, dir, "symbolic-ref", "--short", "HEAD"), "design/")
	reportPath = store.DecisionConflictReportPath(dir, store.ZoneActive, spec)
	dispositionJudged(t, reportPath)
	return dir, reportPath
}

// editDecisionReport strict-decodes the report at path, applies edit, and
// rewrites it as align renders it.
func editDecisionReport(t *testing.T, path string, edit func(*artifact.DecisionConflictFrontmatter)) {
	t.Helper()
	fm := decodeDecisionReportFile(t, path)
	edit(fm)
	if err := os.WriteFile(path, align.RenderDecisionMarkdown(fm, align.RenderDecisionBody(fm.Findings)), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gdcFinding(t *testing.T, fm *artifact.DecisionConflictFrontmatter, id string) *artifact.ConflictFinding {
	t.Helper()
	for i := range fm.Findings {
		if fm.Findings[i].ID == id {
			return &fm.Findings[i]
		}
	}
	t.Fatalf("report has no finding %s: %+v", id, fm.Findings)
	return nil
}

// TestSpecMRGate_RecomputesComputedSection proves design §5's "the gate
// recomputes" (SI-262): the spec-MR condition recomputes the computed
// section from the records at the report's head and fails on any difference
// from the report, naming it — a typed disposition or note, other text, a
// missing or extra finding — so no disposition can supply a missing
// conflict. The report align wrote, with only its judged finding
// dispositioned by hand, passes.
func TestSpecMRGate_RecomputesComputedSection(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, scenario string
		edit           func(*testing.T, *artifact.DecisionConflictFrontmatter)
		wantCode       int
		want           []string
	}{
		{"the report align wrote passes", "proposed", nil, 0, []string{"gate: PASS"}},
		{"a typed disposition cannot supply a missing conflict", "no-conflict", func(t *testing.T, fm *artifact.DecisionConflictFrontmatter) {
			f := gdcFinding(t, fm, gdcDC1)
			f.Disposition, f.Note = artifact.ConflictSuperseded, "typed by hand"
		}, 1, []string{gdcDC1 + `: disposition is "superseded", the records compute none`, gdcDC1 + `: note is "typed by hand", the records compute none`}},
		{"different text", "proposed", func(t *testing.T, fm *artifact.DecisionConflictFrontmatter) {
			gdcFinding(t, fm, gdcDC1).Text = "records match; in force"
		}, 1, []string{gdcDC1 + `: text is "records match; in force", the records compute "records match; takes effect when spec/successor is accepted"`}},
		{"a different note", "proposed", func(t *testing.T, fm *artifact.DecisionConflictFrontmatter) {
			gdcFinding(t, fm, gdcDC2).Note = "signed: owner"
		}, 1, []string{gdcDC2 + `: note is "signed: owner", the records compute "decision dc-2 supersedes spec/closed-story#ac-1"`}},
		{"a missing finding", "proposed", func(t *testing.T, fm *artifact.DecisionConflictFrontmatter) {
			fm.Findings = fm.Findings[1:]
		}, 1, []string{"missing computed finding " + gdcDC1}},
		{"a computed finding moved to the judged section", "proposed", func(t *testing.T, fm *artifact.DecisionConflictFrontmatter) {
			f := gdcFinding(t, fm, gdcDC2)
			f.Kind, f.Disposition, f.Note = artifact.FindingJudged, artifact.ConflictNoConflict, "judged instead"
		}, 1, []string{"missing computed finding " + gdcDC2}},
		{"an extra finding", "proposed", func(t *testing.T, fm *artifact.DecisionConflictFrontmatter) {
			fm.Findings = append(fm.Findings, artifact.ConflictFinding{ID: "edge-dc-9-supersedes-spec--closed-feature-ac-1", Kind: artifact.FindingComputed,
				Text: "records match; takes effect when spec/successor is accepted", Disposition: artifact.ConflictSuperseded, Note: "typed"})
		}, 1, []string{"extra computed finding edge-dc-9-supersedes-spec--closed-feature-ac-1"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir, reportPath := alignScenario(t, tc.scenario, "")
			if tc.edit != nil {
				editDecisionReport(t, reportPath, func(fm *artifact.DecisionConflictFrontmatter) { tc.edit(t, fm) })
			}
			var stdout, stderr bytes.Buffer
			got := runSpecMRGate(context.Background(), dir, "design/successor", nil, "main", &stdout, &stderr)
			if got != tc.wantCode {
				t.Fatalf("runSpecMRGate = %d, want %d; stdout=%s stderr=%s", got, tc.wantCode, stdout.String(), stderr.String())
			}
			for _, w := range tc.want {
				if !strings.Contains(stdout.String(), w) {
					t.Fatalf("stdout = %s\nwant it to contain %q", stdout.String(), w)
				}
			}
		})
	}
}

// TestSpecMRGate_RecomputesAtHeadCommit proves the recompute reads the
// records of the head commit, never the working tree: an uncommitted
// deletion of the successor's conflict leaves the gate passing, since the
// report covers the head, whose records still match.
func TestSpecMRGate_RecomputesAtHeadCommit(t *testing.T) {
	t.Parallel()
	dir, _ := alignScenario(t, "proposed", "")
	if err := os.Remove(filepath.Join(dir, ".verdi", "conflicts", "successor-closed-feature.md")); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if got := runSpecMRGate(context.Background(), dir, "design/successor", nil, "main", &stdout, &stderr); got != 0 {
		t.Fatalf("runSpecMRGate = %d, want 0 (records at the head commit match); stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
}

// TestSpecMRGate_ShallowCloneNeverPassesCarried proves the gate never
// passes on an unproven acceptance (SI-270, L3 review O-2): a carried
// result needs the default branch's first-parent history, so in a shallow
// clone the recompute reads "acceptance unproven" — a report aligned with
// full history then differs and fails naming it, and a report aligned in
// the shallow clone leaves the edges unresolved.
func TestSpecMRGate_ShallowCloneNeverPassesCarried(t *testing.T) {
	t.Parallel()
	full, fullReport := alignScenario(t, "chain", "design/successor-v2")
	if fm := decodeDecisionReportFile(t, fullReport); gdcFinding(t, fm, gdcDC1).Disposition != artifact.ConflictSuperseded {
		t.Fatalf("full-history align did not resolve the carried edge: %+v", fm.Findings)
	}
	shallow := filepath.Join(t.TempDir(), "shallow")
	gdcGit(t, full, "clone", "-q", "--depth=1", "--no-single-branch", "--branch=design/successor-v2", "file://"+full, shallow)
	if gdcGit(t, shallow, "rev-parse", "--is-shallow-repository") != "true" {
		t.Fatal("test setup: the clone is not shallow")
	}
	gdcGit(t, shallow, "remote", "set-head", "origin", "main")
	report := store.DecisionConflictReportPath(shallow, store.ZoneActive, "successor-v2")
	data, err := os.ReadFile(fullReport)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(report, data, 0o644); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name  string
		align bool
		want  string
	}{
		{"a report aligned with full history", false, gdcDC1 + `: text is "carries the replacement established by spec/successor (conflict/successor-closed-feature, since 2024-02-15)", the records compute "acceptance unproven: shallow history`},
		{"a report aligned in the shallow clone", true, "undispositioned/unresolved finding(s): [" + gdcDC1 + " " + gdcDC2 + "]"},
	} {
		if tc.align {
			var stdout, stderr bytes.Buffer
			if got := runDesignAlign(context.Background(), shallow, false, alignDeps{ModelDigest: testResolveModelDigest(t, shallow)}, &stdout, &stderr); got != 0 {
				t.Fatalf("%s: runDesignAlign = %d; stderr=%s", tc.name, got, stderr.String())
			}
			dispositionJudged(t, report)
		}
		var stdout, stderr bytes.Buffer
		if got := runSpecMRGate(context.Background(), shallow, "design/successor-v2", nil, "main", &stdout, &stderr); got != 1 {
			t.Fatalf("%s: runSpecMRGate = %d, want 1; stdout=%s stderr=%s", tc.name, got, stdout.String(), stderr.String())
		}
		if !strings.Contains(stdout.String(), tc.want) {
			t.Fatalf("%s: stdout = %s\nwant it to contain %q", tc.name, stdout.String(), tc.want)
		}
	}
}

// TestCheckDeclaredDecisionConflicts_RecomputeFailures pins the recompute's
// two failure shapes: a spec that is not a decodable spec at the head
// commit (present only in the working tree) fails the condition naming it,
// and a head the repository cannot read is an operational error.
func TestCheckDeclaredDecisionConflicts_RecomputeFailures(t *testing.T) {
	t.Parallel()
	uncommitted := fixturegit.Build(t, []fixturegit.Layer{{Message: "scaffold", Files: map[string]string{".verdi/verdi.yaml": "schema: verdi.layout/v1\n"}}})
	if err := os.MkdirAll(filepath.Join(uncommitted.Dir, ".verdi", "specs", "active", "stale-decline"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(uncommitted.Dir, ".verdi", "specs", "active", "stale-decline", "spec.md"), []byte(gateSpecMD("draft")), 0o644); err != nil {
		t.Fatal(err)
	}
	writeDecisionConflictReport(t, uncommitted.Dir, uncommitted.Head, "  - { id: f-1, kind: judged, text: t, disposition: no-conflict, note: n }\n")
	notGit := t.TempDir()
	writeDecisionConflictReport(t, notGit, gdcHeadCommit, "  - { id: f-1, kind: judged, text: t, disposition: no-conflict, note: n }\n")

	for _, tc := range []struct {
		name, root, head, wantReason string
		wantErr                      bool
	}{
		{"spec absent at the head commit", uncommitted.Dir, uncommitted.Head, "cannot be recomputed from the records at " + uncommitted.Head, false},
		{"unreadable head", notGit, gdcHeadCommit, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cond, err := checkDeclaredDecisionConflicts(context.Background(), tc.root, "stale-decline", tc.head)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, want error: %v", err, tc.wantErr)
			}
			if !tc.wantErr && (cond.OK || !strings.Contains(cond.Reason, tc.wantReason) || !strings.Contains(cond.Reason, "spec/stale-decline")) {
				t.Fatalf("cond = %+v, want a failure containing %q naming spec/stale-decline", cond, tc.wantReason)
			}
		})
	}
}

// TestDiffComputedFindings pins the comparison: equal sections and judged
// findings produce nothing; every other field and a repeated id's count are
// named.
func TestDiffComputedFindings(t *testing.T) {
	t.Parallel()
	base := artifact.ConflictFinding{ID: "e-1", Kind: artifact.FindingComputed, Text: "t", Disposition: artifact.ConflictExempt, Note: "n", TargetRef: "adr/a", RoutedOwners: []string{"o"}}
	with := func(edit func(*artifact.ConflictFinding)) artifact.ConflictFinding {
		f := base
		f.RoutedOwners = append([]string(nil), base.RoutedOwners...)
		edit(&f)
		return f
	}
	judged := artifact.ConflictFinding{ID: "j-1", Kind: artifact.FindingJudged, Text: "j"}
	tests := []struct {
		name     string
		reported []artifact.ConflictFinding
		want     []string
	}{
		{"equal, judged ignored", []artifact.ConflictFinding{base, judged}, nil},
		{"target ref", []artifact.ConflictFinding{with(func(f *artifact.ConflictFinding) { f.TargetRef = "adr/b" })}, []string{`e-1: target_ref is "adr/b", the records compute "adr/a"`}},
		{"routed owners", []artifact.ConflictFinding{with(func(f *artifact.ConflictFinding) { f.RoutedOwners = nil })}, []string{`e-1: routed_owners is none, the records compute "o"`}},
		{"a repeated id", []artifact.ConflictFinding{base, base}, []string{"extra computed finding e-1 (the records do not compute it)"}},
		{"nothing reported", nil, []string{`missing computed finding e-1 (the records compute "t")`}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := diffComputedFindings(tc.reported, []artifact.ConflictFinding{base}); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("diffs = %q, want %q", got, tc.want)
			}
		})
	}
}
