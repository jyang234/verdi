package main

// Lane L2c (SI-230; spec/vatc-forge-countersign-v2 ac-4): the solo close
// countersign reads the owner's environment review of the current close
// run. This file pins the environment-review query to
// .github/workflows/close.yml, and proves the whole path through the built
// binary and through the closure writer against a hermetic GitHub server.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/contextcompile"
	forgegithub "github.com/jyang234/verdi/internal/forge/github"
	gp "github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/instructionprojection"
	"github.com/jyang234/verdi/internal/lifecyclecountersign"
	"github.com/jyang234/verdi/internal/store"
)

// closeWorkflowFile is the dispatch-only close workflow, relative to this
// package's directory.
const closeWorkflowFile = "../../.github/workflows/close.yml"

// closeWorkflowPinViolations reports every way raw (a close workflow) stops
// matching the query the lifecycle resolver pins: GitHub names a job by its
// id unless the job declares `name:`, so the gated job must be the job
// whose id is lifecyclecountersign.CloseGatedJobName, must declare no
// job-level `name:`, and must declare the environment
// lifecyclecountersign.CloseEnvironmentName; and no other job may carry the
// gated job's name, which would make the gated job ambiguous.
func closeWorkflowPinViolations(raw []byte) ([]string, error) {
	generic, err := artifact.DecodeYAMLLoose(raw)
	if err != nil {
		return nil, err
	}
	doc, ok := generic.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("close workflow is not a mapping")
	}
	jobs, ok := doc["jobs"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("close workflow has no jobs mapping")
	}
	var violations []string
	gated, ok := jobs[lifecyclecountersign.CloseGatedJobName].(map[string]interface{})
	if !ok {
		violations = append(violations, fmt.Sprintf("no job has the id %q", lifecyclecountersign.CloseGatedJobName))
	} else {
		if _, named := gated["name"]; named {
			violations = append(violations, fmt.Sprintf("job %q declares a job-level name:, so GitHub no longer names it %q", lifecyclecountersign.CloseGatedJobName, lifecyclecountersign.CloseGatedJobName))
		}
		environment := gated["environment"]
		if mapping, isMapping := environment.(map[string]interface{}); isMapping {
			environment = mapping["name"]
		}
		if environment != lifecyclecountersign.CloseEnvironmentName {
			violations = append(violations, fmt.Sprintf("job %q declares environment %v, want %q", lifecyclecountersign.CloseGatedJobName, environment, lifecyclecountersign.CloseEnvironmentName))
		}
	}
	for id, job := range jobs {
		body, isMapping := job.(map[string]interface{})
		if id != lifecyclecountersign.CloseGatedJobName && isMapping && body["name"] == lifecyclecountersign.CloseGatedJobName {
			violations = append(violations, fmt.Sprintf("job %q is also named %q", id, lifecyclecountersign.CloseGatedJobName))
		}
	}
	return violations, nil
}

// TestCloseWorkflowPinsTheEnvironmentReviewQuery fails if close.yml's
// close job id, its environment, or the absence of a job-level name:
// changes (L2b closure re-review, L2c integration notes): the lifecycle
// resolver asks GitHub for the review of environment "close" dated by the
// job the jobs API names "close", both constants.
func TestCloseWorkflowPinsTheEnvironmentReviewQuery(t *testing.T) {
	raw, err := os.ReadFile(closeWorkflowFile)
	if err != nil {
		t.Fatalf("read close workflow: %v", err)
	}
	violations, err := closeWorkflowPinViolations(raw)
	if err != nil {
		t.Fatalf("decode close workflow: %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("close.yml no longer matches the environment-review query: %v", violations)
	}

	mutate := func(t *testing.T, old, replacement string) []byte {
		t.Helper()
		if strings.Count(string(raw), old) != 1 {
			t.Fatalf("close.yml fixture anchor %q occurs %d times, want 1", old, strings.Count(string(raw), old))
		}
		return []byte(strings.Replace(string(raw), old, replacement, 1))
	}
	const jobAnchor = "jobs:\n  close:\n"
	const environmentAnchor = "    environment: close\n"
	for _, tc := range []struct {
		name        string
		old, new    string
		wantMessage string
	}{
		{"the job id changes", jobAnchor, "jobs:\n  closer:\n", `no job has the id "close"`},
		{"a job-level name is added", jobAnchor, jobAnchor + "    name: Close the spec\n", "declares a job-level name:"},
		{"the environment changes", environmentAnchor, "    environment: production\n", `declares environment production`},
		{"the environment is removed", environmentAnchor, "", `declares environment <nil>`},
		{"the environment mapping names another environment", environmentAnchor, "    environment:\n      name: production\n", `declares environment production`},
		{"another job carries the gated job's name", jobAnchor, "jobs:\n  notify:\n    name: close\n    runs-on: ubuntu-latest\n    steps: []\n  close:\n", `job "notify" is also named "close"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			violations, err := closeWorkflowPinViolations(mutate(t, tc.old, tc.new))
			if err != nil {
				t.Fatalf("decode mutated close workflow: %v", err)
			}
			if !containsSubstring(violations, tc.wantMessage) {
				t.Fatalf("violations = %v, want one containing %q", violations, tc.wantMessage)
			}
		})
	}

	t.Run("the environment mapping form naming close still matches", func(t *testing.T) {
		violations, err := closeWorkflowPinViolations(mutate(t, environmentAnchor, "    environment:\n      name: close\n      url: https://example.invalid\n"))
		if err != nil || len(violations) != 0 {
			t.Fatalf("violations = %v, err = %v, want none", violations, err)
		}
	})
	t.Run("a document that is not a workflow is refused", func(t *testing.T) {
		for _, raw := range []string{"- a list\n", "name: close\n"} {
			if _, err := closeWorkflowPinViolations([]byte(raw)); err == nil {
				t.Fatalf("closeWorkflowPinViolations(%q) accepted a non-workflow document", raw)
			}
		}
	})
}

func containsSubstring(values []string, want string) bool {
	for _, value := range values {
		if strings.Contains(value, want) {
			return true
		}
	}
	return false
}

// TestGithubConfigAddressesTheRunsAPI: the GitHub adapter addresses the API
// root the CI context names (GitHub Actions' GITHUB_API_URL, symmetric with
// GitLab's CI_API_V4_URL); unset, the adapter keeps its own default.
func TestGithubConfigAddressesTheRunsAPI(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	t.Setenv("GITHUB_API_URL", "http://127.0.0.1:9/api")
	if got := githubConfig("acme", "widgets"); got.BaseURL != "http://127.0.0.1:9/api" || got.Owner != "acme" || got.Repo != "widgets" || got.Token != "test-token" {
		t.Fatalf("githubConfig = %+v, want the CI-named API root, identity, and token", got)
	}
	t.Setenv("GITHUB_API_URL", "")
	if got := githubConfig("acme", "widgets"); got.BaseURL != "" {
		t.Fatalf("githubConfig BaseURL = %q with GITHUB_API_URL unset, want empty so the adapter's own default applies", got.BaseURL)
	}
}

// The hermetic GitHub the solo close reads: pull request 17 authored by the
// owner (GitHub user 900) with no review, and the close workflow's run 5551
// whose first attempt's `close` job the owner approved in environment
// `close`.
const (
	soloCloseRunID      = "5551"
	soloCloseEnvID      = 161088068
	soloCloseJobID      = 7001
	soloCloseOwnerID    = 900
	soloCloseApprovalID = "github-environment-review:acme/widgets:5551:1:161088068:900"
)

type soloCloseGitHub struct {
	server   *httptest.Server
	mu       sync.Mutex
	paths    []string
	mutating int
}

func (g *soloCloseGitHub) requests() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]string{}, g.paths...)
}

func (g *soloCloseGitHub) writes() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.mutating
}

var (
	soloCloseRunJobsPath = regexp.MustCompile(`^/repos/acme/widgets/actions/runs/5551/attempts/\d+/jobs$`)
)

// newSoloCloseGitHub serves the owner's approved review of run 5551, whose
// latest attempt is latestAttempt and whose head, like the pull request's,
// is head. It answers only GET and counts every other request.
func newSoloCloseGitHub(t *testing.T, head, sourceBranch string, latestAttempt int) *soloCloseGitHub {
	t.Helper()
	now := time.Now().UTC()
	created := now.Add(-5 * time.Minute).Format(time.RFC3339)
	started := now.Add(-4 * time.Minute).Format(time.RFC3339)
	g := &soloCloseGitHub{}
	writeJSON := func(w http.ResponseWriter, value any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(value)
	}
	g.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.mu.Lock()
		g.paths = append(g.paths, r.Method+" "+r.URL.Path)
		if r.Method != http.MethodGet {
			g.mutating++
		}
		g.mu.Unlock()
		if r.Method != http.MethodGet {
			http.Error(w, "read-only fixture", http.StatusMethodNotAllowed)
			return
		}
		switch path := r.URL.Path; {
		case path == "/repos/acme/widgets/pulls":
			writeJSON(w, []any{map[string]any{"number": 17, "title": "close candidate", "head": map[string]any{"ref": sourceBranch}}})
		case path == "/repos/acme/widgets/pulls/17":
			writeJSON(w, map[string]any{"number": 17, "head": map[string]any{"sha": head}, "user": map[string]any{"id": soloCloseOwnerID}})
		case path == "/repos/acme/widgets/pulls/17/reviews":
			writeJSON(w, []any{})
		case path == "/repos/acme/widgets/actions/runs/5551/approvals":
			writeJSON(w, []any{map[string]any{
				"state": "approved", "comment": "",
				"environments": []any{map[string]any{"id": soloCloseEnvID, "name": "close"}},
				"user":         map[string]any{"id": soloCloseOwnerID, "login": "owner"},
			}})
		case soloCloseRunJobsPath.MatchString(path):
			writeJSON(w, map[string]any{"total_count": 1, "jobs": []any{map[string]any{
				"id": soloCloseJobID, "name": "close", "status": "in_progress", "conclusion": nil,
				"created_at": created, "started_at": started,
			}}})
		case path == "/repos/acme/widgets/environments/close":
			writeJSON(w, map[string]any{"id": soloCloseEnvID, "name": "close", "protection_rules": []any{
				map[string]any{"id": 3, "type": "required_reviewers", "prevent_self_review": false, "reviewers": []any{}},
			}})
		case path == "/repos/acme/widgets/actions/runs/5551":
			writeJSON(w, map[string]any{
				"id": 5551, "run_attempt": latestAttempt, "head_sha": head,
				"html_url": "https://github.com/acme/widgets/actions/runs/5551",
				"event":    "workflow_dispatch", "path": ".github/workflows/close.yml@refs/heads/close/close-fixture",
				"status": "in_progress", "conclusion": nil,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(g.server.Close)
	return g
}

// soloCloseForgeEnv names the hermetic GitHub API root, the repository,
// and the current run and attempt, as the close job's CI context does.
func soloCloseForgeEnv(apiURL, attempt string) map[string]string {
	return map[string]string{
		"GITHUB_API_URL": apiURL, "GITHUB_REPOSITORY": "acme/widgets", "GITHUB_REPOSITORY_OWNER": "",
		"GITHUB_TOKEN": "test-token", "GITHUB_RUN_ID": soloCloseRunID, "GITHUB_RUN_ATTEMPT": attempt,
		"GITHUB_HEAD_REF": "", "GITHUB_REF_NAME": "", "CI_COMMIT_REF_NAME": "",
	}
}

// soloCloseBinaryEnv is the close job's environment for the built binary:
// GitHub Actions itself, so `verdi close` runs as close.yml runs it, with
// no --force-local. The proxy variables send every host but loopback to a
// closed local port, so a regression that dialed the real GitHub API
// fails here instead of reaching the network.
func soloCloseBinaryEnv(apiURL, attempt string) map[string]string {
	env := soloCloseForgeEnv(apiURL, attempt)
	env["CI"], env["GITHUB_ACTIONS"] = "true", "true"
	env["HTTPS_PROXY"], env["HTTP_PROXY"] = "http://127.0.0.1:9", "http://127.0.0.1:9"
	return env
}

// soloCloseConstitution is the countersign contract constitution with the
// named author role in its catalog.
func soloCloseConstitution(t *testing.T) string {
	t.Helper()
	updated := strings.Replace(countersignContractConstitution, "  roles: [feature-uat, story-review]\n", "  roles: [author, feature-uat, story-review]\n", 1)
	if updated == countersignContractConstitution {
		t.Fatal("countersign contract constitution does not carry the expected role catalog")
	}
	return updated
}

// soloCloseProfile is a solo governance profile that maps the owner's
// forge principal to the author role and both close roles and declares no
// other close rule, so the kernel permits the author/approver collapse
// (SI-233).
const soloCloseProfile = `---
schema: verdi.governance-profile/v1
id: lifecycle
class: solo
applicable_transitions: [close]
identity_trust_sources:
  - {id: forge-live, kind: forge}
role_mappings:
  - {role: author, trust_source: forge-live, subjects: ["900"]}
  - {role: feature-uat, trust_source: forge-live, subjects: ["900"]}
  - {role: story-review, trust_source: forge-live, subjects: ["900"]}
ownership_sources: []
signature_requirements: []
required_approvers:
  - {transitions: [close], roles: [feature-uat, story-review], minimum: 1}
distinctness_rules: []
evidence_source_restrictions: []
escalation_thresholds: []
---
Hermetic solo lifecycle governance profile: the owner's forge principal
fills the author role and both close roles.
`

// readySoloCloseRepo is the close-fixture story ready to close on GitHub
// under the solo profile, adopted on the default branch: evidence for the
// candidate and a dispositioned report covering it. jiraBaseURL, when
// given, is where the close publishes its rollup.
func readySoloCloseRepo(t *testing.T, jiraBaseURL ...string) (string, string) {
	t.Helper()
	repo := readyCloseFixtureRepo(t)
	prepareCountersignStoryContext(t, repo.Dir)
	installCountersignContractAuthority(t, repo.Dir, true, true, jiraBaseURL...)
	manifestPath := filepath.Join(repo.Dir, ".verdi", "verdi.yaml")
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read fixture manifest: %v", err)
	}
	manifest := strings.Replace(string(raw), "forge: gitlab\n", "forge: github\n", 1)
	if manifest == string(raw) {
		t.Fatal("fixture manifest does not name the gitlab forge")
	}
	for rel, content := range map[string]string{
		".verdi/verdi.yaml":                   manifest,
		".verdi/policy/constitution.md":       soloCloseConstitution(t),
		".verdi/policy/profiles/lifecycle.md": soloCloseProfile,
	} {
		if err := os.WriteFile(filepath.Join(repo.Dir, filepath.FromSlash(rel)), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	if _, err := instructionprojection.Generate(repo.Dir); err != nil {
		t.Fatalf("generate canonical instruction projection: %v", err)
	}
	gitOutput(t, repo.Dir, "add", "--", ".verdi/verdi.yaml", ".verdi/policy", "AGENTS.md")
	gitOutput(t, repo.Dir, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "--quiet", "--no-verify", "-m", "adopt the solo profile on github")
	if strings.TrimSpace(gitOutput(t, repo.Dir, "rev-parse", "--abbrev-ref", "HEAD")) != "main" {
		gitOutput(t, repo.Dir, "branch", "-f", "main", "HEAD")
	}
	head := strings.TrimSpace(gitOutput(t, repo.Dir, "rev-parse", "HEAD"))
	prov := artifact.EvidenceProvenance{Source: artifact.SourceCI, Pipeline: "2", Job: "1", JobName: "1", Commit: head}
	if err := produceSelfHostedEvidence(repo.Dir, head, prov); err != nil {
		t.Fatalf("produce solo close evidence: %v", err)
	}
	writeCloseGateReport(t, repo.Dir, head, dispositionedFindingYAML)
	return repo.Dir, head
}

// soloCollapseDisclosure is the kernel's solo role-collapse disclosure for
// the owner's principal, as the countersign record carries it.
func soloCollapseDisclosure(t *testing.T) string {
	t.Helper()
	principal, err := gp.CanonicalPrincipalID("forge-live", "900")
	if err != nil {
		t.Fatalf("owner principal id: %v", err)
	}
	return fmt.Sprintf(`kernel-separation-probe:author-as-approver:collapse-permitted:kernel_disclosure="solo-role-collapse":author_principal_id=%q:roles=["author" "story-review"]`, principal)
}

// assertSoloCloseReadOnly: the countersign only read the forge, and it read
// the current run's environment review.
func assertSoloCloseReadOnly(t *testing.T, github *soloCloseGitHub, attempt string) {
	t.Helper()
	if github.writes() != 0 {
		t.Fatalf("the close wrote to the forge: %v", github.requests())
	}
	for _, want := range []string{
		"GET /repos/acme/widgets/actions/runs/5551/approvals",
		"GET /repos/acme/widgets/actions/runs/5551/attempts/" + attempt + "/jobs",
		"GET /repos/acme/widgets/environments/close",
		"GET /repos/acme/widgets/actions/runs/5551",
	} {
		if !containsSubstring(github.requests(), want) {
			t.Fatalf("forge requests = %v, want %q", github.requests(), want)
		}
	}
}

// TestSoloEnvironmentReviewCloseCountersign proves the solo close end to
// end. The built binary, run as close.yml runs it, proves the close
// countersign from the owner's approved review of the run's first attempt
// alone and stops, unchanged, at the constitutional-conflict block every
// hermetic adopted store reaches (I-55; TestCountersignLifecycleContract_Behavioral
// documents the same limit). The closure writer (runClose), through the
// injected conflict verdict that contract uses, archives a rollup whose
// countersign witnesses carry the kernel's solo role-collapse disclosure
// (L2a review m-7).
func TestSoloEnvironmentReviewCloseCountersign(t *testing.T) {
	binary := buildCountersignContractBinary(t)

	t.Run("built binary: the owner's approved review of the first attempt proves the close countersign", func(t *testing.T) {
		root, head := readySoloCloseRepo(t)
		github := newSoloCloseGitHub(t, head, currentCountersignBranch(t, root), 1)
		requestPath := contextLifecycleRequestFile(t, root, "solo-close-context.json", "spec/close-fixture", contextcompile.PhaseReview, nil)
		before := countersignCandidateSnapshot(t, root)

		result := runCountersignContractBinary(t, binary, root, soloCloseBinaryEnv(github.server.URL, "1"), "close", "spec/close-fixture", "--context-request", requestPath)
		if result.code != 1 {
			t.Fatalf("built-binary solo close = %+v, want the unchanged conflict block, exit 1", result)
		}
		assertCountersignBeforeConflictBlock(t, result, "built-binary solo close")
		countersignRecordDigestFromOutput(t, result.stdout)
		assertSoloCloseReadOnly(t, github, "1")
		if before != countersignCandidateSnapshot(t, root) {
			t.Fatalf("built-binary solo close mutated the candidate: stdout=%s", result.stdout)
		}
		assertNoCountersignArtifact(t, root)
	})

	for _, tc := range []struct {
		name    string
		attempt string
		latest  int
		want    string
	}{
		{"a rerun attempt's review proves nothing", "2", 2, "environment-review:rerun-not-honored:"},
		{"a close run without a run attempt reads no review", "", 1, "environment-review:current-run-unresolved:"},
	} {
		t.Run("built binary: "+tc.name, func(t *testing.T) {
			root, head := readySoloCloseRepo(t)
			github := newSoloCloseGitHub(t, head, currentCountersignBranch(t, root), tc.latest)
			requestPath := contextLifecycleRequestFile(t, root, "solo-close-context.json", "spec/close-fixture", contextcompile.PhaseReview, nil)

			result := runCountersignContractBinary(t, binary, root, soloCloseBinaryEnv(github.server.URL, tc.attempt), "close", "spec/close-fixture", "--context-request", requestPath)
			output := result.stdout + result.stderr
			if result.code != 1 || !strings.Contains(output, "[FAIL] closure: 5. forge countersign") || !strings.Contains(output, tc.want) {
				t.Fatalf("built-binary solo close = %+v, want the countersign refused with %s", result, tc.want)
			}
			if strings.Contains(output, "constitutional conflict") {
				t.Fatalf("the close reached the conflict gate although the countersign failed: %s", output)
			}
			if github.writes() != 0 {
				t.Fatalf("the close wrote to the forge: %v", github.requests())
			}
		})
	}

	t.Run("closure writer: the archived rollup carries the solo role-collapse disclosure", func(t *testing.T) {
		jira, publications := newCountersignJiraServer(t)
		defer jira.Close()
		root, head := readySoloCloseRepo(t, jira.URL)
		github := newSoloCloseGitHub(t, head, currentCountersignBranch(t, root), 1)
		for key, value := range soloCloseForgeEnv(github.server.URL, "1") {
			t.Setenv(key, value)
		}
		t.Setenv("CI_DEFAULT_BRANCH", "main")
		t.Setenv(countersignConflictJudgeEnv, "1")
		requestPath := contextLifecycleRequestFile(t, root, "solo-close-context.json", "spec/close-fixture", contextcompile.PhaseReview, nil)

		// The adapter is bound to the hermetic server directly, so this
		// in-process close can never dial the real GitHub API; the built
		// binary above proves the production forge construction. Its CI
		// context still reads GITHUB_RUN_ID and GITHUB_RUN_ATTEMPT.
		cfg, err := store.Open(root)
		if err != nil {
			t.Fatalf("open solo close store: %v", err)
		}
		f := forgegithub.New(forgegithub.Config{BaseURL: github.server.URL, Owner: "acme", Repo: "widgets", Token: "test-token"})
		deps := countersignContractCloseDeps(cfg.Manifest, cfg.Model, f, requestPath, countersignPassConflictProvider(), lifecyclecountersign.Resolver{Forge: f})
		var stdout, stderr bytes.Buffer
		result := countersignCommandResult{code: runClose(context.Background(), root, "spec/close-fixture", cfg.Manifest, deps, &stdout, &stderr), stdout: stdout.String(), stderr: stderr.String()}
		if result.code != 0 || !strings.Contains(result.stdout, "rollup published to jira:CLOSE-1") {
			t.Fatalf("solo close through the closure writer = %+v, want a proven, published close", result)
		}
		wantDigest := countersignRecordDigestFromOutput(t, result.stdout)
		raw, err := os.ReadFile(filepath.Join(root, ".verdi", "specs", "archive", "close-fixture", "rollup.json"))
		if err != nil {
			t.Fatalf("read archived rollup: %v", err)
		}
		rollup, err := artifact.DecodeRollup(raw)
		if err != nil {
			t.Fatalf("strict-decode archived rollup: %v", err)
		}
		countersign := rollup.Countersign
		if countersign == nil || countersign.RecordDigest != wantDigest || countersign.Verdict != "proven" {
			t.Fatalf("rollup countersign = %+v, want the proven record %s", countersign, wantDigest)
		}
		if len(countersign.Approvals) != 1 || countersign.Approvals[0].ApprovalID != soloCloseApprovalID || countersign.Approvals[0].PrincipalState != "authenticated" {
			t.Fatalf("rollup approvals = %+v, want exactly the owner's environment review %s", countersign.Approvals, soloCloseApprovalID)
		}
		if len(countersign.EligibleApprovalIDs) != 1 || countersign.EligibleApprovalIDs[0] != soloCloseApprovalID {
			t.Fatalf("rollup eligible approvals = %v, want [%s]", countersign.EligibleApprovalIDs, soloCloseApprovalID)
		}
		for _, want := range []string{
			soloCollapseDisclosure(t),
			"environment-review:observation: run_id=5551 run_attempt=1 environment=close gated_job=close ",
			fmt.Sprintf("environment-review:candidate-binding: approval_id=%q run_head_sha=%q change_head_sha=%q local_candidate_sha=%q match=true", soloCloseApprovalID, head, head, head),
		} {
			if !containsSubstring(countersign.Witnesses, want) {
				t.Fatalf("rollup countersign witnesses = %v, want %s", countersign.Witnesses, want)
			}
		}
		if publications.counts() != (countersignJiraCounts{Reads: 1, Writes: 1, Comments: 1}) {
			t.Fatalf("Jira publication requests = %+v, want one read/write/comment", publications.counts())
		}
		assertSoloCloseReadOnly(t, github, "1")
		assertNoCountersignArtifact(t, root)
	})
}
