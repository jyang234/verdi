package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/countersign"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/forge"
	forgefake "github.com/jyang234/verdi/internal/forge/fake"
	"github.com/jyang234/verdi/internal/instructionprojection"
	"github.com/jyang234/verdi/internal/lifecyclecountersign"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/policyconflict"
	"github.com/jyang234/verdi/internal/specstate"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/storyresolve"
)

const countersignConflictJudgeEnv = "VERDI_TEST_COUNTERSIGN_CONFLICT_JUDGE"

// TestCountersignLifecycleConflictJudgeProcess is the hermetic subprocess
// behind the production align.judge_cmd path in the lifecycle contract.
func TestCountersignLifecycleConflictJudgeProcess(t *testing.T) {
	if os.Getenv(countersignConflictJudgeEnv) != "1" {
		return
	}
	raw, err := policyconflict.EncodeJudgeResult(policyconflict.JudgeResult{
		Schema:         policyconflict.JudgeResultSchema,
		Recommendation: policyconflict.RecommendationNoConflict,
		Findings:       []policyconflict.JudgeFinding{},
	})
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "encode hermetic judge result: %v\n", err)
		os.Exit(2)
	}
	if _, err := os.Stdout.Write(raw); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "write hermetic judge result: %v\n", err)
		os.Exit(2)
	}
	os.Exit(0)
}

// TestCountersignLifecycleContract_Behavioral is the exact behavioral AC-3
// evidence producer. It drives the built binary for production entry/exit
// wiring and I-55's honest conflict blocks, then uses the existing injected
// conflict-verdict seams for positive resolver and closure-writer evidence.
func TestCountersignLifecycleContract_Behavioral(t *testing.T) {
	binary := buildCountersignContractBinary(t)

	t.Run("build gate blocks disclosed-unproven missing countersign config without mutation", func(t *testing.T) {
		repo := buildGateRepo(t, "accepted-pending-build")
		writeGateReport(t, repo.Dir, repo.Head, dispositionedFindingYAML)
		before := countersignCandidateSnapshot(t, repo.Dir)

		result := runCountersignContractBinary(t, binary, repo.Dir, nil, "gate")
		if result.code != 1 {
			t.Fatalf("verdi gate exit = %d, want 1; stdout=%s stderr=%s", result.code, result.stdout, result.stderr)
		}
		for _, want := range []string{"countersign", "unproven", "config"} {
			if !strings.Contains(result.stdout+result.stderr, want) {
				t.Fatalf("verdi gate output = %q, want %q", result.stdout+result.stderr, want)
			}
		}
		after := countersignCandidateSnapshot(t, repo.Dir)
		if before != after {
			t.Fatalf("verdi gate mutated candidate repository:\nbefore=%s\nafter=%s", before, after)
		}
		assertNoCountersignArtifact(t, repo.Dir)
	})

	t.Run("built-binary unconfigured gate preserves legacy conflict-first order", func(t *testing.T) {
		repo := buildCountersignGateRepo(t)
		head := installCountersignContractAuthority(t, repo.Dir, true, true)
		removeCountersignConfig(t, repo.Dir)
		writeCountersignGateReport(t, repo.Dir, head)
		requestPath := contextLifecycleRequestFile(t, repo.Dir, "lifecycle-unconfigured-context.json", "spec/enum-spike", contextcompile.PhaseBuild, nil)
		before := countersignCandidateSnapshot(t, repo.Dir)

		result := runCountersignContractBinary(t, binary, repo.Dir, nil, "gate", "--context-request", requestPath)
		if result.code != 1 {
			t.Fatalf("unconfigured built-binary gate = %+v, want blocking exit 1", result)
		}
		conflictAt := strings.Index(result.stdout, "constitutional conflict")
		countersignAt := strings.Index(result.stdout, "forge countersign")
		if conflictAt < 0 || countersignAt < conflictAt {
			t.Fatalf("unconfigured gate changed legacy conflict-first order: stdout=%s stderr=%s", result.stdout, result.stderr)
		}
		if before != countersignCandidateSnapshot(t, repo.Dir) {
			t.Fatalf("unconfigured gate mutated candidate: stdout=%s stderr=%s", result.stdout, result.stderr)
		}
	})

	t.Run("built-binary configured gate proves countersign before unchanged conflict block without mutation", func(t *testing.T) {
		repo := buildCountersignGateRepo(t)
		head := installCountersignContractAuthority(t, repo.Dir, true, true)
		writeCountersignGateReport(t, repo.Dir, head)
		server, requests := newCountersignGitLabServer(t, countersignGitLabScenario{
			CandidateSHA: head, SourceBranch: currentCountersignBranch(t, repo.Dir), AuthorID: 900,
			Approvers: []int64{101}, ApprovedAt: time.Now().Add(-time.Minute), OpenMR: true,
		})
		defer server.Close()
		requestPath := contextLifecycleRequestFile(t, repo.Dir, "lifecycle-gate-context.json", "spec/enum-spike", contextcompile.PhaseBuild, nil)

		before := countersignCandidateSnapshot(t, repo.Dir)
		result := runCountersignContractBinary(t, binary, repo.Dir, countersignGitLabEnv(server.URL), "gate", "--context-request", requestPath)
		if result.code != 1 {
			t.Fatalf("built-binary gate result = %+v, want unchanged conflict verdict exit 1", result)
		}
		assertCountersignBeforeConflictBlock(t, result, "built-binary gate")
		assertCountersignReadOnly(t, repo.Dir, before, requests)
	})

	for _, tc := range []struct {
		name     string
		class    string
		mode     string
		storyArg string
	}{
		{name: "story close preflight", class: "story", mode: "preflight", storyArg: "spec/close-fixture"},
		{name: "story close prepare delegated preflight", class: "story", mode: "prepare", storyArg: "spec/close-fixture"},
		{name: "feature close preflight", class: "feature", mode: "preflight", storyArg: "spec/close-feature-fixture"},
	} {
		t.Run("built-binary configured "+tc.name+" proves countersign before unchanged conflict block without mutation", func(t *testing.T) {
			root, head := readyCountersignCloseRepo(t, tc.class)
			approvers := []int64{101}
			if tc.class == "feature" {
				approvers = []int64{201, 202}
			}
			server, requests := newCountersignGitLabServer(t, countersignGitLabScenario{
				CandidateSHA: head, SourceBranch: currentCountersignBranch(t, root), AuthorID: 900,
				Approvers: approvers, ApprovedAt: time.Now().Add(-time.Minute), OpenMR: true,
			})
			defer server.Close()
			requestPath := contextLifecycleRequestFile(t, root, "lifecycle-close-context.json", tc.storyArg, contextcompile.PhaseReview, nil)

			before := countersignCandidateSnapshot(t, root)
			args := []string{"close", tc.storyArg, "--" + tc.mode, "--force-local", "--context-request", requestPath}
			result := runCountersignContractBinary(t, binary, root, countersignGitLabEnv(server.URL), args...)
			if result.code != 1 {
				t.Fatalf("built-binary %s result = %+v, want unchanged conflict verdict exit 1", tc.name, result)
			}
			assertCountersignBeforeConflictBlock(t, result, "built-binary "+tc.name)
			assertCountersignReadOnly(t, root, before, requests)
		})
	}

	for _, tc := range []struct {
		class     string
		storyArg  string
		approvers []int64
	}{
		{class: "story", storyArg: "spec/close-fixture", approvers: []int64{101}},
		{class: "feature", storyArg: "spec/close-feature-fixture", approvers: []int64{201, 202}},
	} {
		t.Run("built-binary "+tc.class+" close proves countersign before unchanged conflict block", func(t *testing.T) {
			root, head := readyCountersignCloseRepo(t, tc.class)
			server, requests := newCountersignGitLabServer(t, countersignGitLabScenario{
				CandidateSHA: head, SourceBranch: currentCountersignBranch(t, root), AuthorID: 900,
				Approvers: tc.approvers, ApprovedAt: time.Now().Add(-time.Minute), OpenMR: true,
			})
			defer server.Close()
			requestPath := contextLifecycleRequestFile(t, root, "lifecycle-production-block-context.json", tc.storyArg, contextcompile.PhaseReview, nil)
			beforeHead := strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD"))
			beforeBranch := currentCountersignBranch(t, root)

			result := runCountersignContractBinary(t, binary, root, countersignGitLabEnv(server.URL), "close", tc.storyArg, "--force-local", "--context-request", requestPath)
			if result.code != 1 {
				t.Fatalf("built-binary %s close = %+v, want conflict verdict exit 1", tc.class, result)
			}
			counterAt := strings.Index(result.stdout, "forge countersign proven")
			conflictAt := strings.Index(result.stdout, "constitutional conflict: state: blocked-unproven")
			if counterAt < 0 || conflictAt < 0 || counterAt >= conflictAt {
				t.Fatalf("built-binary %s close output does not prove countersign before conflict block: %s", tc.class, result.stdout)
			}
			if got := strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD")); got != beforeHead {
				t.Fatalf("blocked %s close HEAD = %s, want %s", tc.class, got, beforeHead)
			}
			if got := currentCountersignBranch(t, root); got != beforeBranch {
				t.Fatalf("blocked %s close branch = %s, want %s", tc.class, got, beforeBranch)
			}
			if requests.mutating() != 0 {
				t.Fatalf("forge approval-request count = %d, want 0", requests.mutating())
			}
			assertNoCountersignArtifact(t, root)
		})
	}

	t.Run("proven real feature close through injected conflict verdict preserves exact record digest in strict archived rollup", func(t *testing.T) {
		root, head := readyCountersignCloseRepo(t, "feature")
		server, requests := newCountersignGitLabServer(t, countersignGitLabScenario{
			CandidateSHA: head, SourceBranch: currentCountersignBranch(t, root), AuthorID: 900,
			Approvers: []int64{201, 202}, ApprovedAt: time.Now().Add(-time.Minute), OpenMR: true,
		})
		defer server.Close()
		requestPath := contextLifecycleRequestFile(t, root, "lifecycle-feature-close-context.json", "spec/close-feature-fixture", contextcompile.PhaseReview, nil)
		setCountersignGitLabEnv(t, server.URL)

		result := runInjectedCountersignLifecycle(t, root, requestPath, "close", "spec/close-feature-fixture")
		if result.code != 0 {
			t.Fatalf("verdi real feature close exit = %d, want 0; stdout=%s stderr=%s", result.code, result.stdout, result.stderr)
		}
		wantDigest := countersignRecordDigestFromOutput(t, result.stdout)
		raw, err := os.ReadFile(filepath.Join(root, ".verdi", "specs", "archive", "close-feature-fixture", "rollup.json"))
		if err != nil {
			t.Fatalf("read archived feature rollup: %v", err)
		}
		rollup, err := artifact.DecodeRollup(raw)
		if err != nil {
			t.Fatalf("strict-decode archived feature rollup: %v", err)
		}
		if rollup.Countersign == nil || rollup.Countersign.RecordDigest != wantDigest || rollup.Countersign.Verdict != "proven" {
			t.Fatalf("rollup countersign = %+v, want exact proven digest %s", rollup.Countersign, wantDigest)
		}
		if len(rollup.Countersign.Approvals) < 2 || len(rollup.Countersign.EligibleApprovalIDs) < 2 || len(rollup.Countersign.DistinctPrincipalIDs) < 2 || len(rollup.Countersign.Witnesses) == 0 {
			t.Fatalf("rollup countersign projection lost approval/reduction/witness identities: %+v", rollup.Countersign)
		}
		for _, approval := range rollup.Countersign.Approvals {
			if approval.ApprovalID == "" || approval.ApprovalRef != approval.ApprovalID || approval.PrincipalID == "" || approval.PrincipalState != "authenticated" {
				t.Fatalf("rollup approval row lost strict identity: %+v", approval)
			}
		}
		if requests.mutating() != 0 {
			t.Fatalf("forge approval-request count = %d, want 0 (port is read-only)", requests.mutating())
		}
		assertNoCountersignArtifact(t, root)
	})

	t.Run("proven real story close through injected conflict verdict publishes and preserves exact record digest in strict archived rollup", func(t *testing.T) {
		repo := readyCloseFixtureRepo(t)
		prepareCountersignStoryContext(t, repo.Dir)
		jiraServer, publications := newCountersignJiraServer(t)
		defer jiraServer.Close()
		head := installCountersignContractAuthority(t, repo.Dir, true, true, jiraServer.URL)
		prov := artifact.EvidenceProvenance{Source: artifact.SourceCI, Pipeline: "2", Job: "1", Commit: head}
		if err := produceSelfHostedEvidence(repo.Dir, head, prov); err != nil {
			t.Fatalf("produce countersign story evidence: %v", err)
		}
		writeCloseGateReport(t, repo.Dir, head, dispositionedFindingYAML)
		forgeServer, requests := newCountersignGitLabServer(t, countersignGitLabScenario{
			CandidateSHA: head, SourceBranch: currentCountersignBranch(t, repo.Dir), AuthorID: 900,
			Approvers: []int64{101}, ApprovedAt: time.Now().Add(-time.Minute), OpenMR: true,
		})
		defer forgeServer.Close()
		requestPath := contextLifecycleRequestFile(t, repo.Dir, "lifecycle-story-close-context.json", "spec/close-fixture", contextcompile.PhaseReview, nil)
		setCountersignGitLabEnv(t, forgeServer.URL)

		result := runInjectedCountersignLifecycle(t, repo.Dir, requestPath, "close", "spec/close-fixture")
		if result.code != 0 || !strings.Contains(result.stdout, "rollup published to jira:CLOSE-1") {
			t.Fatalf("verdi real story close result = %+v, want proven published close", result)
		}
		wantDigest := countersignRecordDigestFromOutput(t, result.stdout)
		raw, err := os.ReadFile(filepath.Join(repo.Dir, ".verdi", "specs", "archive", "close-fixture", "rollup.json"))
		if err != nil {
			t.Fatalf("read archived story rollup: %v", err)
		}
		rollup, err := artifact.DecodeRollup(raw)
		if err != nil {
			t.Fatalf("strict-decode archived story rollup: %v", err)
		}
		if rollup.Countersign == nil || rollup.Countersign.RecordDigest != wantDigest || rollup.Countersign.Verdict != "proven" || len(rollup.Countersign.Approvals) != 1 || len(rollup.Countersign.Witnesses) == 0 {
			t.Fatalf("story rollup countersign = %+v, want exact proven digest %s with identities", rollup.Countersign, wantDigest)
		}
		if publications.counts() != (countersignJiraCounts{Reads: 1, Writes: 1, Comments: 1}) {
			t.Fatalf("Jira publication requests = %+v, want one read/write/comment", publications.counts())
		}
		if requests.mutating() != 0 {
			t.Fatalf("forge approval-request count = %d, want 0 (port is read-only)", requests.mutating())
		}
		assertNoCountersignArtifact(t, repo.Dir)
	})

	t.Run("real story close blocks on the same missing operand before mutation", func(t *testing.T) {
		repo := readyCloseFixtureRepo(t)
		before := countersignCandidateSnapshot(t, repo.Dir)
		result := runCountersignContractBinary(t, binary, repo.Dir, nil, "close", "spec/close-fixture", "--force-local")
		if result.code != 1 || !strings.Contains(result.stdout+result.stderr, "countersign") {
			t.Fatalf("verdi real story close result = %+v, want countersign block", result)
		}
		after := countersignCandidateSnapshot(t, repo.Dir)
		if before != after {
			t.Fatalf("blocked real story close mutated candidate:\nbefore=%s\nafter=%s", before, after)
		}
		assertNoCountersignArtifact(t, repo.Dir)
	})

	t.Run("removed GitLab approval is absent from the newer active snapshot", func(t *testing.T) {
		repo := buildCountersignGateRepo(t)
		head := installCountersignContractAuthority(t, repo.Dir, true, true)
		writeCountersignGateReport(t, repo.Dir, head)
		server, requests := newCountersignGitLabServer(t, countersignGitLabScenario{
			CandidateSHA: head, SourceBranch: currentCountersignBranch(t, repo.Dir), AuthorID: 900,
			ApproverSets: [][]int64{{101}, {}}, ApprovedAt: time.Now().Add(-time.Minute), OpenMR: true,
		})
		defer server.Close()
		requestPath := contextLifecycleRequestFile(t, repo.Dir, "lifecycle-removed-context.json", "spec/enum-spike", contextcompile.PhaseBuild, nil)
		before := countersignCandidateSnapshot(t, repo.Dir)

		first := runCountersignContractBinary(t, binary, repo.Dir, countersignGitLabEnv(server.URL), "gate", "--context-request", requestPath)
		if first.code != 1 {
			t.Fatalf("first built-binary gate = %+v, want conflict verdict exit 1", first)
		}
		assertCountersignBeforeConflictBlock(t, first, "first active GitLab snapshot")

		second := runCountersignContractBinary(t, binary, repo.Dir, countersignGitLabEnv(server.URL), "gate", "--context-request", requestPath)
		output := second.stdout + second.stderr
		if second.code != 1 || !strings.Contains(output, "[FAIL] 5. forge countersign") || !strings.Contains(output, "countersign verdict is violated-with-witness") {
			t.Fatalf("newer snapshot without approver = %+v, want non-proven countersign verdict", second)
		}
		if strings.Contains(output, "dismissed") || strings.Contains(output, "revoked") {
			t.Fatalf("newer GitLab snapshot fabricated historical state: %s", output)
		}
		if requests.approvalReadCount() != 2 {
			t.Fatalf("approval snapshot reads = %d, want older and newer observations", requests.approvalReadCount())
		}
		assertCountersignReadOnly(t, repo.Dir, before, requests)
	})

	t.Run("explicit dismissed approval row is retained through the lifecycle reducer", func(t *testing.T) {
		repo := buildCountersignGateRepo(t)
		head := installCountersignContractAuthority(t, repo.Dir, true, true)
		cfg, err := store.Open(repo.Dir)
		if err != nil {
			t.Fatalf("open countersign store: %v", err)
		}
		branch := currentCountersignBranch(t, repo.Dir)
		f := forgefake.New()
		f.SeedOpenMR("main", forge.OpenMR{ID: "17", SourceBranch: branch})
		stamp := time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano)
		snapshot, err := forge.NewApprovalSnapshot(
			"github", "acme/widgets", "17", head,
			forge.ProviderActor{Scheme: "github-user-id", Subject: "900"}, time.Now().Add(-30*time.Second),
			[]forge.Approval{{
				ApprovalID: "dismissed-101", ApprovalRef: "review/dismissed-101", State: forge.ApprovalDismissed,
				ApprovedAt: stamp, UpdatedAt: stamp, CandidateSHA: head,
				Actor:             forge.ProviderActor{Scheme: "github-user-id", Subject: "101"},
				ProviderWitnesses: []forge.ProviderWitness{{Name: "review_id", Value: "dismissed-101"}},
			}},
		)
		if err != nil {
			t.Fatalf("build dismissed snapshot: %v", err)
		}
		f.SeedApprovalSnapshot("17", snapshot)

		result, err := (lifecyclecountersign.Resolver{Forge: f}).Resolve(context.Background(), lifecyclecountersign.Request{
			Root: repo.Dir, Manifest: cfg.Manifest, Model: cfg.Model, TargetClass: "story",
			DefaultBranch: "main", SourceBranch: branch, LocalCandidateSHA: head,
		})
		if err != nil {
			t.Fatalf("resolve dismissed approval: %v", err)
		}
		if result.Verdict == countersign.VerdictProven || result.Record == nil {
			t.Fatalf("dismissed result = %+v, want retained adverse record", result)
		}
		if len(result.Record.Approvals) != 1 || result.Record.Approvals[0].State != forge.ApprovalDismissed {
			t.Fatalf("dismissed approval was not retained: %+v", result.Record.Approvals)
		}
	})

	t.Run("adverse forge and authority facts never pass", func(t *testing.T) {
		tests := []struct {
			name                  string
			scenario              countersignGitLabScenario
			withProfile           bool
			withForge             bool
			unreachable           bool
			absentTrustSource     bool
			wantCode              int
			wantUnproven          bool
			wantUnprovenPrincipal bool
			wantRoleRefused       bool
			wantMalformedActor    bool
		}{
			{name: "zero current GitLab approvals", scenario: countersignGitLabScenario{AuthorID: 900, OpenMR: true}, withProfile: true, withForge: true, wantCode: 1},
			{name: "stale approval", scenario: countersignGitLabScenario{AuthorID: 900, Approvers: []int64{101}, ApprovedAt: time.Now().Add(-2 * time.Hour), OpenMR: true}, withProfile: true, withForge: true, wantCode: 1},
			{name: "future approval", scenario: countersignGitLabScenario{AuthorID: 900, Approvers: []int64{101}, ApprovedAt: time.Now().Add(5 * time.Minute), OpenMR: true}, withProfile: true, withForge: true, wantCode: 1},
			{name: "wrong head", scenario: countersignGitLabScenario{CandidateSHA: strings.Repeat("b", 40), AuthorID: 900, Approvers: []int64{101}, ApprovedAt: time.Now().Add(-time.Minute), OpenMR: true}, withProfile: true, withForge: true, wantCode: 1},
			{name: "duplicate stable approver", scenario: countersignGitLabScenario{AuthorID: 900, Approvers: []int64{101, 101}, ApprovedAt: time.Now().Add(-time.Minute), OpenMR: true}, withProfile: true, withForge: true, wantCode: 2},
			{name: "self approved", scenario: countersignGitLabScenario{AuthorID: 101, Approvers: []int64{101}, ApprovedAt: time.Now().Add(-time.Minute), OpenMR: true}, withProfile: true, withForge: true, wantCode: 1},
			{name: "role refused under present trust source", scenario: countersignGitLabScenario{AuthorID: 900, Approvers: []int64{301}, ApprovedAt: time.Now().Add(-time.Minute), OpenMR: true}, withProfile: true, withForge: true, wantCode: 1, wantRoleRefused: true},
			{name: "stable provider actor with absent configured trust source", scenario: countersignGitLabScenario{AuthorID: 900, Approvers: []int64{302}, ApprovedAt: time.Now().Add(-time.Minute), OpenMR: true}, withProfile: true, withForge: true, absentTrustSource: true, wantCode: 1, wantUnproven: true, wantUnprovenPrincipal: true},
			{name: "malformed zero provider actor", scenario: countersignGitLabScenario{AuthorID: 900, Approvers: []int64{0}, ApprovedAt: time.Now().Add(-time.Minute), OpenMR: true}, withProfile: true, withForge: true, wantCode: 2, wantMalformedActor: true},
			{name: "absent selected profile", scenario: countersignGitLabScenario{AuthorID: 900, Approvers: []int64{101}, ApprovedAt: time.Now().Add(-time.Minute), OpenMR: true}, withForge: true, wantCode: 1, wantUnproven: true},
			{name: "absent forge", withProfile: true, wantCode: 1, wantUnproven: true},
			{name: "absent merge request", scenario: countersignGitLabScenario{AuthorID: 900, Approvers: []int64{101}, ApprovedAt: time.Now().Add(-time.Minute)}, withProfile: true, withForge: true, wantCode: 1, wantUnproven: true},
			{name: "unreachable forge", scenario: countersignGitLabScenario{AuthorID: 900, Approvers: []int64{101}, ApprovedAt: time.Now().Add(-time.Minute), OpenMR: true}, withProfile: true, withForge: true, unreachable: true, wantCode: 1, wantUnproven: true},
			{name: "malformed provider JSON", scenario: countersignGitLabScenario{AuthorID: 900, Approvers: []int64{101}, ApprovedAt: time.Now().Add(-time.Minute), OpenMR: true, MalformedApprovals: true}, withProfile: true, withForge: true, wantCode: 2},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				repo := buildCountersignGateRepo(t)
				head := installCountersignContractAuthority(t, repo.Dir, tc.withForge, tc.withProfile)
				if tc.absentTrustSource {
					configureCountersignTrustSource(t, repo.Dir, "forge-unselected")
				}
				writeCountersignGateReport(t, repo.Dir, head)
				tc.scenario.SourceBranch = currentCountersignBranch(t, repo.Dir)
				if tc.scenario.CandidateSHA == "" {
					tc.scenario.CandidateSHA = head
				}
				server, requests := newCountersignGitLabServer(t, tc.scenario)
				if tc.withForge {
					setCountersignGitLabEnv(t, server.URL)
				}
				if tc.unreachable {
					server.Close()
				} else {
					defer server.Close()
				}
				requestPath := ""
				if tc.withProfile {
					requestPath = contextLifecycleRequestFile(t, repo.Dir, "lifecycle-adverse-context.json", "spec/enum-spike", contextcompile.PhaseBuild, nil)
				}

				before := countersignCandidateSnapshot(t, repo.Dir)
				result := runInjectedCountersignLifecycle(t, repo.Dir, requestPath, "gate", "")
				if result.code != tc.wantCode {
					t.Fatalf("verdi gate exit = %d, want %d; stdout=%s stderr=%s", result.code, tc.wantCode, result.stdout, result.stderr)
				}
				if tc.wantUnproven && !strings.Contains(result.stdout+result.stderr, "unproven") {
					t.Fatalf("verdi gate output = %q, want unproven disclosure", result.stdout+result.stderr)
				}
				if tc.wantUnprovenPrincipal {
					output := result.stdout + result.stderr
					if !strings.Contains(output, "lifecycle-countersign:principal-authentication:unproven:") {
						t.Fatalf("verdi gate output = %q, want canonical unproven principal-authentication evidence", output)
					}
					if strings.Contains(output, "role-membership:") {
						t.Fatalf("verdi gate output = %q, absent trust source must not be relabeled as role refusal", output)
					}
				}
				if tc.wantRoleRefused {
					output := result.stdout + result.stderr
					if !strings.Contains(output, "role-membership:") || !strings.Contains(output, `verdict="violated-with-witness"`) {
						t.Fatalf("verdi gate output = %q, want role-membership refusal under present trust source", output)
					}
				}
				if tc.wantMalformedActor && !strings.Contains(result.stderr, "approval carries no stable actor user id") {
					t.Fatalf("verdi gate stderr = %q, want malformed stable-actor error", result.stderr)
				}
				assertCountersignReadOnly(t, repo.Dir, before, requests)
			})
		}
	})
}

type countersignGitLabScenario struct {
	CandidateSHA       string
	SourceBranch       string
	AuthorID           int64
	Approvers          []int64
	ApproverSets       [][]int64
	ApprovedAt         time.Time
	OpenMR             bool
	MalformedApprovals bool
}

type countersignForgeRequests struct {
	mu               sync.Mutex
	mutatingRequests int
	approvalReads    int
}

type countersignJiraCounts struct {
	Reads    int
	Writes   int
	Comments int
}

type countersignJiraRequests struct {
	mu sync.Mutex
	countersignJiraCounts
}

func (r *countersignJiraRequests) counts() countersignJiraCounts {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.countersignJiraCounts
}

func newCountersignJiraServer(t *testing.T) (*httptest.Server, *countersignJiraRequests) {
	t.Helper()
	requests := &countersignJiraRequests{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/3/issue/CLOSE-1" && r.URL.Query().Get("fields") == "customfield_00000":
			requests.mu.Lock()
			requests.Reads++
			requests.mu.Unlock()
			_, _ = w.Write([]byte(`{"fields":{"customfield_00000":null}}`))
		case r.Method == http.MethodPut && r.URL.Path == "/rest/api/3/issue/CLOSE-1":
			requests.mu.Lock()
			requests.Writes++
			requests.mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && r.URL.Path == "/rest/api/3/issue/CLOSE-1/comment":
			requests.mu.Lock()
			requests.Comments++
			requests.mu.Unlock()
			w.WriteHeader(http.StatusCreated)
		default:
			http.NotFound(w, r)
		}
	})
	return httptest.NewServer(handler), requests
}

func (r *countersignForgeRequests) mutating() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.mutatingRequests
}

func (r *countersignForgeRequests) approvalReadCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.approvalReads
}

func newCountersignGitLabServer(t *testing.T, scenario countersignGitLabScenario) (*httptest.Server, *countersignForgeRequests) {
	t.Helper()
	requests := &countersignForgeRequests{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			requests.mu.Lock()
			requests.mutatingRequests++
			requests.mu.Unlock()
			http.Error(w, "read-only fixture", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/projects/42/merge_requests":
			if !scenario.OpenMR {
				_, _ = w.Write([]byte("[]"))
				return
			}
			_, _ = fmt.Fprintf(w, `[{"iid":17,"source_branch":%q,"title":"candidate"}]`, scenario.SourceBranch)
		case "/projects/42/merge_requests/17":
			_, _ = fmt.Fprintf(w, `{"sha":%q,"project_id":42,"author":{"id":%d}}`, scenario.CandidateSHA, scenario.AuthorID)
		case "/projects/42/merge_requests/17/approvals":
			if scenario.MalformedApprovals {
				_, _ = w.Write([]byte(`{"approved_by":`))
				return
			}
			approvers := scenario.Approvers
			requests.mu.Lock()
			readIndex := requests.approvalReads
			requests.approvalReads++
			requests.mu.Unlock()
			if len(scenario.ApproverSets) > 0 {
				if readIndex >= len(scenario.ApproverSets) {
					readIndex = len(scenario.ApproverSets) - 1
				}
				approvers = scenario.ApproverSets[readIndex]
			}
			stamp := scenario.ApprovedAt.UTC().Format(time.RFC3339Nano)
			var rows []string
			for _, id := range approvers {
				rows = append(rows, fmt.Sprintf(`{"user":{"id":%d,"username":%q},"approved_at":%q}`, id, fmt.Sprintf("u%d", id), stamp))
			}
			_, _ = fmt.Fprintf(w, `{"approved_by":[%s]}`, strings.Join(rows, ","))
		default:
			http.NotFound(w, r)
		}
	})
	return httptest.NewServer(handler), requests
}

func countersignGitLabEnv(baseURL string) map[string]string {
	return map[string]string{"CI_API_V4_URL": baseURL, "CI_PROJECT_ID": "42", "CI_JOB_TOKEN": "test-token"}
}

func buildCountersignGateRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/verdi.yaml":                         "schema: verdi.layout/v1\nforge: gitlab\n",
			".verdi/specs/active/enum-spike/spec.md":    strings.Replace(gateSpikeSpecMD(), "spec/some-feature#oq-1", "spec/feature-alpha#oq-1", 1),
			".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
		},
		Message: "scaffold countersign gate story",
	}})
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	checkoutBranch(t, repo.Dir, "feature/enum-spike")
	return repo
}

func writeCountersignGateReport(t *testing.T, root, head string) {
	t.Helper()
	path := filepath.Join(root, ".verdi", "specs", "active", "enum-spike", "deviation-report.md")
	content := fmt.Sprintf(`---
schema: verdi.deviation/v1
covers: %s
findings:
%s
digest: sha256:%s
---
# Alignment report
`, head, dispositionedFindingYAML, strings.Repeat("0", 64))
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write countersign gate report: %v", err)
	}
}

func installCountersignContractAuthority(t *testing.T, root string, withForge, withProfile bool, jiraBaseURL ...string) string {
	t.Helper()
	manifestPath := filepath.Join(root, ".verdi", "verdi.yaml")
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read fixture manifest: %v", err)
	}
	manifest := string(raw)
	manifest = strings.Replace(manifest, "forge: github\n", "", 1)
	manifest = strings.Replace(manifest, "forge: gitlab\n", "", 1)
	if withForge {
		manifest += "forge: gitlab\n"
	}
	if len(jiraBaseURL) > 0 {
		manifest += fmt.Sprintf("providers:\n  jira:\n    base_url: %s\n    rollup_field: customfield_00000\n", jiraBaseURL[0])
	}
	testExecutable, err := os.Executable()
	if err != nil {
		t.Fatalf("locate test executable for hermetic judge: %v", err)
	}
	manifest += fmt.Sprintf("align:\n  judge_cmd: [%q, %q]\n", testExecutable, "-test.run=^TestCountersignLifecycleConflictJudgeProcess$")
	manifest += `countersign:
  trust_source: forge-live
  freshness_policy_id: forge-current
  maximum_observation_age_seconds: 300
  maximum_approval_age_seconds: 3600
`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatalf("write fixture manifest: %v", err)
	}
	if withProfile {
		files := map[string]string{
			".verdi/policy/constitution.md":       countersignContractConstitution,
			".verdi/policy/profiles/lifecycle.md": countersignContractProfile,
		}
		for rel, content := range files {
			path := filepath.Join(root, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatalf("mkdir policy fixture: %v", err)
			}
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				t.Fatalf("write policy fixture %s: %v", rel, err)
			}
		}
		if _, err := instructionprojection.Generate(root); err != nil {
			t.Fatalf("generate canonical instruction projection: %v", err)
		}
	}
	addArgs := []string{"add", "--", ".verdi/verdi.yaml"}
	if withProfile {
		addArgs = append(addArgs, ".verdi/policy", "AGENTS.md")
	}
	for _, rel := range []string{".verdi/specs/active/close-fixture/spec.md", ".verdi/specs/active/feature-alpha/spec.md"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err == nil {
			addArgs = append(addArgs, rel)
		}
	}
	gitOutput(t, root, addArgs...)
	gitOutput(t, root, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "--quiet", "--no-verify", "-m", "configure countersign authority")
	return strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD"))
}

func configureCountersignTrustSource(t *testing.T, root, trustSource string) {
	t.Helper()
	manifestPath := filepath.Join(root, ".verdi", "verdi.yaml")
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read fixture manifest: %v", err)
	}
	updated := strings.Replace(string(raw), "trust_source: forge-live", "trust_source: "+trustSource, 1)
	if updated == string(raw) {
		t.Fatal("fixture manifest does not contain forge-live countersign trust source")
	}
	if err := os.WriteFile(manifestPath, []byte(updated), 0o644); err != nil {
		t.Fatalf("write fixture manifest trust source: %v", err)
	}
}

func removeCountersignConfig(t *testing.T, root string) {
	t.Helper()
	manifestPath := filepath.Join(root, ".verdi", "verdi.yaml")
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read fixture manifest: %v", err)
	}
	const block = `countersign:
  trust_source: forge-live
  freshness_policy_id: forge-current
  maximum_observation_age_seconds: 300
  maximum_approval_age_seconds: 3600
`
	updated := strings.Replace(string(raw), block, "", 1)
	if updated == string(raw) {
		t.Fatal("fixture manifest does not contain countersign config")
	}
	if err := os.WriteFile(manifestPath, []byte(updated), 0o644); err != nil {
		t.Fatalf("write fixture manifest without countersign config: %v", err)
	}
}

func readyCountersignCloseRepo(t *testing.T, class string) (string, string) {
	t.Helper()
	if class == "story" {
		repo := readyCloseFixtureRepo(t)
		prepareCountersignStoryContext(t, repo.Dir)
		head := installCountersignContractAuthority(t, repo.Dir, true, true)
		prov := artifact.EvidenceProvenance{Source: artifact.SourceCI, Pipeline: "2", Job: "1", Commit: head}
		if err := produceSelfHostedEvidence(repo.Dir, head, prov); err != nil {
			t.Fatalf("produce countersign story evidence: %v", err)
		}
		writeCloseGateReport(t, repo.Dir, head, dispositionedFindingYAML)
		return repo.Dir, head
	}
	opts := defaultCloseFeatureFixtureOpts()
	repo := buildCloseFeatureRepo(t, opts)
	head := installCountersignContractAuthority(t, repo.Dir, true, true)
	seedCloseFeatureEvidence(t, repo.Dir, head, opts)
	writeCloseFeatureGateReport(t, repo.Dir, head, dispositionedFindingYAML)
	return repo.Dir, head
}

func prepareCountersignStoryContext(t *testing.T, root string) {
	t.Helper()
	storyPath := filepath.Join(root, ".verdi", "specs", "active", "close-fixture", "spec.md")
	raw, err := os.ReadFile(storyPath)
	if err != nil {
		t.Fatalf("read countersign story context: %v", err)
	}
	updated := strings.Replace(string(raw), "spec/loan-mgmt#ac-1", "spec/feature-alpha#ac-1", 1)
	if updated == string(raw) {
		t.Fatal("countersign story context did not contain expected parent feature ref")
	}
	if err := os.WriteFile(storyPath, []byte(updated), 0o644); err != nil {
		t.Fatalf("write countersign story context: %v", err)
	}
	featurePath := filepath.Join(root, ".verdi", "specs", "active", "feature-alpha", "spec.md")
	if err := os.MkdirAll(filepath.Dir(featurePath), 0o755); err != nil {
		t.Fatalf("mkdir countersign feature context: %v", err)
	}
	if err := os.WriteFile(featurePath, []byte(contextFeatureAlphaSpec(t)), 0o644); err != nil {
		t.Fatalf("write countersign feature context: %v", err)
	}
}

func currentCountersignBranch(t *testing.T, root string) string {
	t.Helper()
	return strings.TrimSpace(gitOutput(t, root, "symbolic-ref", "--short", "HEAD"))
}

func assertCountersignReadOnly(t *testing.T, root, before string, requests *countersignForgeRequests) {
	t.Helper()
	after := countersignCandidateSnapshot(t, root)
	if before != after {
		t.Fatalf("read-only lifecycle command mutated candidate:\nbefore=%s\nafter=%s\nstatus including ignored files:\n%s", before, after, gitOutput(t, root, "status", "--short", "--ignored"))
	}
	if requests.mutating() != 0 {
		t.Fatalf("forge approval-request count = %d, want 0 (port is read-only)", requests.mutating())
	}
	assertNoCountersignArtifact(t, root)
}

func countersignRecordDigestFromOutput(t *testing.T, output string) string {
	t.Helper()
	match := regexp.MustCompile(`countersign record: (sha256:[0-9a-f]{64})`).FindStringSubmatch(output)
	if len(match) != 2 {
		t.Fatalf("output has no countersign record digest: %s", output)
	}
	return match[1]
}

func assertCountersignBeforeConflictBlock(t *testing.T, result countersignCommandResult, label string) {
	t.Helper()
	countersignAt := strings.Index(result.stdout, "forge countersign proven")
	conflictAt := strings.Index(result.stdout, "constitutional conflict")
	stateAt := strings.Index(result.stdout, "state: blocked-unproven")
	passOnCountersignLine := false
	if countersignAt >= 0 {
		lineStart := strings.LastIndex(result.stdout[:countersignAt], "\n") + 1
		passOnCountersignLine = strings.Contains(result.stdout[lineStart:countersignAt], "[PASS]")
	}
	if !passOnCountersignLine || conflictAt < 0 || stateAt < conflictAt || countersignAt >= conflictAt {
		t.Fatalf("%s output does not prove countersign before conflict block: stdout=%s stderr=%s", label, result.stdout, result.stderr)
	}
}

const countersignContractConstitution = `---
schema: verdi.policy-constitution/v1
id: policy-constitution/constitution
kind: policy-constitution
title: "Countersign fixture constitution"
owners: [platform-team]
selected_profile: lifecycle
environments: [local, production]
catalog:
  roles: [feature-uat, story-review]
  transitions: [close]
  evidence_sources: []
  escalation_metrics: []
subjects:
  action: []
  configuration: []
  capability: []
  resource: []
  identity: []
  evidence: []
adapters:
  - id: codex
    version: "1"
    managed: [AGENTS.md]
    discovery_filenames: [AGENTS.md]
---
# Countersign fixture
`

const countersignContractProfile = `---
schema: verdi.governance-profile/v1
id: lifecycle
class: team
applicable_transitions: [close]
identity_trust_sources:
  - {id: forge-live, kind: forge}
role_mappings:
  - {role: feature-uat, trust_source: forge-live, subjects: ["201", "202"]}
  - {role: story-review, trust_source: forge-live, subjects: ["101"]}
ownership_sources: []
signature_requirements: []
required_approvers:
  - {transitions: [close], roles: [feature-uat, story-review], minimum: 1}
distinctness_rules:
  - {transitions: [close], left_role: feature-uat, right_role: story-review, relation: different-principal}
evidence_source_restrictions: []
escalation_thresholds: []
---
Hermetic lifecycle governance profile.
`

type countersignCommandResult struct {
	code   int
	stdout string
	stderr string
}

func setCountersignGitLabEnv(t *testing.T, baseURL string) {
	t.Helper()
	for key, value := range countersignGitLabEnv(baseURL) {
		t.Setenv(key, value)
	}
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	t.Setenv(countersignConflictJudgeEnv, "1")
}

func countersignPassConflictProvider() policyconflict.VerdictProvider {
	return contextConflictProviderFunc(func(context.Context, policyconflict.Request) (policyconflict.Result, error) {
		return lifecycleConflictResult(policyconflict.VerdictPass), nil
	})
}

// runInjectedCountersignLifecycle exercises the existing conflict-verdict
// seam while retaining production repository/model/forge/profile/countersign
// dependencies. I-55 limits this seam to closure-writer evidence; it is not a
// claim that the production policy-conflict provider can pass.
func runInjectedCountersignLifecycle(t *testing.T, root, requestPath, mode, storyArg string) countersignCommandResult {
	t.Helper()
	ctx := context.Background()
	cfg, err := store.Open(root)
	if err != nil {
		t.Fatalf("open countersign contract store: %v", err)
	}
	f := buildForgeBestEffort(ctx, root)
	provider := countersignPassConflictProvider()
	resolver := lifecyclecountersign.Resolver{Forge: f}
	var stdout, stderr bytes.Buffer
	code := 0
	switch mode {
	case "gate":
		branch := currentCountersignBranch(t, root)
		spec, err := storyresolve.ResolveBuildSpec(root, branch)
		if err != nil {
			t.Fatalf("resolve countersign gate spec: %v", err)
		}
		head := strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD"))
		code = runGateWithConflictAndCountersign(ctx, root, spec, head, specstate.NewProjector(), cfg.Model, requestPath, provider, resolver, &stdout, &stderr)
	case "preflight":
		code = runPreflightWithConflictAndCountersign(ctx, root, storyArg, cfg.Manifest, cfg.Model, f, true, requestPath, provider, resolver, &stdout, &stderr)
	case "prepare":
		deps := countersignContractCloseDeps(cfg.Manifest, cfg.Model, f, requestPath, provider, resolver)
		code = runPrepareWithConflict(ctx, root, storyArg, cfg.Manifest, deps, true, requestPath, provider, &stdout, &stderr)
	case "close":
		deps := countersignContractCloseDeps(cfg.Manifest, cfg.Model, f, requestPath, provider, resolver)
		code = runClose(ctx, root, storyArg, cfg.Manifest, deps, &stdout, &stderr)
	default:
		t.Fatalf("unknown injected lifecycle mode %q", mode)
	}
	return countersignCommandResult{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

func countersignContractCloseDeps(manifest *store.Manifest, mdl *model.Model, f forge.Forge, requestPath string, provider policyconflict.VerdictProvider, resolver lifecycleCountersignResolver) closeDeps {
	deps := closeDeps{
		Forge:               f,
		Countersign:         resolver,
		Registry:            buildProviderRegistry(manifest),
		Model:               mdl,
		ConflictRequestPath: requestPath,
		ConflictProvider:    provider,
	}
	if manifest.Align != nil {
		deps.JudgeCmd = append([]string(nil), manifest.Align.JudgeCmd...)
		deps.JudgeRequired = manifest.Align.JudgeRequired
		if manifest.Align.JudgeTimeoutSeconds > 0 {
			deps.JudgeTimeout = time.Duration(manifest.Align.JudgeTimeoutSeconds) * time.Second
		}
	}
	return deps
}

func buildCountersignContractBinary(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "verdi")
	cmd := exec.Command("go", "build", "-o", binary, ".")
	cmd.Dir = "."
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build countersign contract binary: %v\n%s", err, output)
	}
	return binary
}

func runCountersignContractBinary(t *testing.T, binary, dir string, env map[string]string, args ...string) countersignCommandResult {
	t.Helper()
	cmd := exec.CommandContext(context.Background(), binary, args...)
	cmd.Dir = dir
	cmd.Env = append([]string{}, os.Environ()...)
	cmd.Env = append(cmd.Env,
		"CI_DEFAULT_BRANCH=main",
		countersignConflictJudgeEnv+"=1",
		"CI_API_V4_URL=",
		"CI_PROJECT_ID=",
		"CI_JOB_TOKEN=",
		"GITHUB_TOKEN=",
		"GITHUB_REPOSITORY=",
		"GITHUB_REPOSITORY_OWNER=",
	)
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		cmd.Env = append(cmd.Env, key+"="+env[key])
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("run built verdi %v: %v", args, err)
		}
		code = exitErr.ExitCode()
	}
	return countersignCommandResult{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

func countersignCandidateSnapshot(t *testing.T, root string) string {
	t.Helper()
	hash := sha256.New()
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.IsDir() && (rel == ".git" || filepath.ToSlash(rel) == ".verdi/data") {
			return filepath.SkipDir
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(hash, "%s\x00%s\x00", filepath.ToSlash(rel), info.Mode())
		_, _ = hash.Write(data)
		return nil
	}); err != nil {
		t.Fatalf("snapshot candidate bytes: %v", err)
	}
	for _, args := range [][]string{{"rev-parse", "HEAD"}, {"diff", "--cached", "--binary"}, {"status", "--short", "--", ".", ":(exclude).verdi/data"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
		_, _ = fmt.Fprintf(hash, "git:%s\x00", strings.Join(args, " "))
		_, _ = hash.Write(output)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func assertNoCountersignArtifact(t *testing.T, root string) {
	t.Helper()
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.IsDir() && rel == ".git" {
			return filepath.SkipDir
		}
		if !entry.IsDir() && strings.Contains(strings.ToLower(filepath.Base(path)), "countersign") {
			t.Errorf("forbidden countersign artifact created at %s", filepath.ToSlash(rel))
		}
		return nil
	}); err != nil {
		t.Fatalf("scan countersign artifacts: %v", err)
	}
}
