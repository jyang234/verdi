package main

// Built-binary tests for `verdi design import apply|record` (Task 4 CLI,
// spec-import-contract.md), continuing designimport_test.go's helpers.
// Apply scenarios cover delegated-actor policy refusal/success, explicit
// deferral disclosure, already-created retry and malformed flags/request
// refusals; record scenarios cover a truthful read immediately after
// apply, a truthful disclosure after a later ordinary edit, a missing
// record and an invalid --spec.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/specimport"
	"github.com/jyang234/verdi/internal/store"
)

// designImportPreviewDigest runs `verdi design import preview` against
// reqBytes and returns its ready digest, failing the test if the preview
// is not ready — the shared setup step every apply test needs.
func designImportPreviewDigest(t *testing.T, bin, root string, reqBytes []byte) string {
	t.Helper()
	run := runDesignImportBinary(t, bin, root, reqBytes, nil, "preview", "--request", "-")
	if run.code != 0 || run.stderr != "" {
		t.Fatalf("preview for digest = %+v", run)
	}
	var result specimport.PreviewResult
	if err := json.Unmarshal([]byte(run.stdout), &result); err != nil {
		t.Fatalf("decoding preview result: %v\n%s", err, run.stdout)
	}
	if !result.Ready {
		t.Fatalf("preview not ready, cannot proceed to apply: %+v", result.Findings)
	}
	return result.Digest
}

// designImportEditActiveSpec checks branch out, applies mutate to slug's
// active spec.md, commits it, and returns to main — driving git directly
// to simulate "a later ordinary edit" on the imported branch, exactly as
// the task's own instructions describe.
func designImportEditActiveSpec(t *testing.T, root, branch, slug string, mutate func([]byte) []byte) {
	t.Helper()
	designImportGitOutput(t, root, "checkout", branch)
	path := store.ActiveSpecPath(filepath.FromSlash(root), slug)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, mutate(data), 0o644); err != nil {
		t.Fatal(err)
	}
	designImportGitOutput(t, root, "add", "-A")
	designImportGitOutput(t, root, "commit", "-m", "later edit after import")
	designImportGitOutput(t, root, "checkout", "main")
}

func assertDesignImportDeferredDisclosure(t *testing.T, result specimport.Result) {
	t.Helper()
	if !result.StatementsDeferred {
		t.Fatalf("result.StatementsDeferred = false, want true: %+v", result)
	}
	for _, f := range result.Disclosures {
		if f.Code == specimport.FindingStatementsDeferred && !f.Blocking {
			return
		}
	}
	t.Fatalf("disclosures missing a nonblocking statements-deferred finding: %+v", result.Disclosures)
}

// TestDesignImportApplyPolicyBuiltBinary proves delegated-agent apply is
// refused (with correction guidance) under a proposal-only design-
// assistance policy and no branch created, and succeeds under a
// draft-write policy while ignoring actor/principal environment
// variables — mirroring designmutate_test.go's own spoof-resistance
// proof for `design mutate`.
func TestDesignImportApplyPolicyBuiltBinary(t *testing.T) {
	t.Parallel()
	bin := buildVerdiBinary(t)

	t.Run("proposal-only policy refuses with no branch created", func(t *testing.T) {
		root := designImportPolicyRepo(t, "proposal-only")
		before := designImportRepoStateNow(t, root)
		reqBytes := designImportRequestJSON(t, designImportReadyRequest(t, "sample-feature-refused"))
		digest := designImportPreviewDigest(t, bin, root, reqBytes)

		run := runDesignImportBinary(t, bin, root, reqBytes, nil, "apply", "--request", "-", "--preview", digest, "--harness", "codex")
		if run.code != 1 || run.stdout != "" {
			t.Fatalf("policy refusal = %+v", run)
		}
		if !strings.HasPrefix(run.stderr, "design import apply: policy-forbidden:") {
			t.Fatalf("policy refusal stderr = %q", run.stderr)
		}
		if !strings.Contains(run.stderr, "proposal-only") {
			t.Fatalf("policy refusal stderr does not name the forbidding mode: %q", run.stderr)
		}
		if designImportBranchExists(t, root, "design/sample-feature-refused") {
			t.Fatalf("branch was created despite policy refusal")
		}
		after := designImportRepoStateNow(t, root)
		if before != after {
			t.Fatalf("refused apply changed the repository:\nbefore: %+v\nafter:  %+v", before, after)
		}
	})

	t.Run("draft-write policy succeeds, ignores actor environment, and retries to already-created", func(t *testing.T) {
		root := designImportPolicyRepo(t, "draft-write")
		callerBefore := designImportRepoStateNow(t, root)
		slug := "sample-feature-created"
		reqBytes := designImportRequestJSON(t, designImportReadyRequest(t, slug))
		digest := designImportPreviewDigest(t, bin, root, reqBytes)

		spoofEnv := map[string]string{"VERDI_ACTOR_KIND": "human", "VERDI_PRINCIPAL_ID": "principal/forged", "VERDI_HUMAN": "1"}
		created := runDesignImportBinary(t, bin, root, reqBytes, spoofEnv, "apply", "--request", "-", "--preview", digest, "--harness", "codex", "--session", "session-1")
		if created.code != 0 || created.stderr != "" {
			t.Fatalf("created apply = %+v", created)
		}
		var createdResult specimport.Result
		if err := json.Unmarshal([]byte(created.stdout), &createdResult); err != nil {
			t.Fatalf("decoding created result: %v\n%s", err, created.stdout)
		}
		if createdResult.Status != specimport.StatusCreated || createdResult.Branch != "design/"+slug {
			t.Fatalf("created result = %+v", createdResult)
		}
		if !designImportBranchExists(t, root, "design/"+slug) {
			t.Fatalf("design/%s branch was not created", slug)
		}

		callerAfter := designImportRepoStateNow(t, root)
		if callerBefore.head != callerAfter.head || callerBefore.branch != callerAfter.branch || callerBefore.status != callerAfter.status {
			t.Fatalf("apply changed the caller's own checkout:\nbefore: %+v\nafter:  %+v", callerBefore, callerAfter)
		}

		retry := runDesignImportBinary(t, bin, root, reqBytes, spoofEnv, "apply", "--request", "-", "--preview", digest, "--harness", "codex", "--session", "session-1")
		if retry.code != 0 || retry.stderr != "" {
			t.Fatalf("retry apply = %+v", retry)
		}
		var retryResult specimport.Result
		if err := json.Unmarshal([]byte(retry.stdout), &retryResult); err != nil {
			t.Fatalf("decoding retry result: %v\n%s", err, retry.stdout)
		}
		if retryResult.Status != specimport.StatusAlreadyCreated || retryResult.Commit != createdResult.Commit {
			t.Fatalf("retry result = %+v, want already-created at %s", retryResult, createdResult.Commit)
		}

		// The committed record's actor attribution — read back through the
		// CLI's own read-only `record` verb — must be unauthenticated with
		// the GIVEN harness/session, never the spoofed environment.
		recordRun := runDesignImportBinary(t, bin, root, nil, nil, "record", "--branch", "design/"+slug, "--spec", slug)
		if recordRun.code != 0 || recordRun.stderr != "" {
			t.Fatalf("record after create = %+v", recordRun)
		}
		var view specimport.RecordView
		if err := json.Unmarshal([]byte(recordRun.stdout), &view); err != nil {
			t.Fatalf("decoding record view: %v\n%s", err, recordRun.stdout)
		}
		actor := view.Record.Actor
		if !actor.Attribution.Unauthenticated || actor.Attribution.PrincipalID != "" || actor.Harness != "codex" || actor.Session != "session-1" {
			t.Fatalf("committed actor attribution = %+v, want unauthenticated harness=codex session=session-1 despite spoofed environment", actor)
		}
	})
}

// TestDesignImportApplyDeferralBuiltBinary proves explicit statement
// deferral discloses statements_deferred=true and a nonblocking
// statements-deferred finding on BOTH the created result and an
// already-created retry.
func TestDesignImportApplyDeferralBuiltBinary(t *testing.T) {
	t.Parallel()
	bin := buildVerdiBinary(t)
	root := designImportPolicyRepo(t, "draft-write")
	slug := "sample-feature-deferred"
	req := designImportReadyRequest(t, slug, func(r *specimport.Request) { r.DeferStatements = true })
	reqBytes := designImportRequestJSON(t, req)
	digest := designImportPreviewDigest(t, bin, root, reqBytes)

	first := runDesignImportBinary(t, bin, root, reqBytes, nil, "apply", "--request", "-", "--preview", digest, "--harness", "codex")
	if first.code != 0 || first.stderr != "" {
		t.Fatalf("first deferred apply = %+v", first)
	}
	var firstResult specimport.Result
	if err := json.Unmarshal([]byte(first.stdout), &firstResult); err != nil {
		t.Fatalf("decoding first deferred result: %v\n%s", err, first.stdout)
	}
	if firstResult.Status != specimport.StatusCreated {
		t.Fatalf("first deferred result = %+v", firstResult)
	}
	assertDesignImportDeferredDisclosure(t, firstResult)

	second := runDesignImportBinary(t, bin, root, reqBytes, nil, "apply", "--request", "-", "--preview", digest, "--harness", "codex")
	if second.code != 0 || second.stderr != "" {
		t.Fatalf("retry deferred apply = %+v", second)
	}
	var secondResult specimport.Result
	if err := json.Unmarshal([]byte(second.stdout), &secondResult); err != nil {
		t.Fatalf("decoding retry deferred result: %v\n%s", err, second.stdout)
	}
	if secondResult.Status != specimport.StatusAlreadyCreated || secondResult.Commit != firstResult.Commit {
		t.Fatalf("retry deferred result = %+v, want already-created at %s", secondResult, firstResult.Commit)
	}
	assertDesignImportDeferredDisclosure(t, secondResult)
}

// TestDesignImportApplyMalformedBuiltBinary exercises every malformed
// apply flag/actor refusal named by the task's required outcomes: missing
// --request/--preview/--harness, a blank harness, a malformed or
// uppercase --preview, a duplicate flag, an unknown flag, and a rejected
// --human flag.
func TestDesignImportApplyMalformedBuiltBinary(t *testing.T) {
	t.Parallel()
	bin := buildVerdiBinary(t)
	root := designImportPolicyRepo(t, "draft-write")
	reqBytes := designImportRequestJSON(t, designImportReadyRequest(t, "sample-feature-apply-flags"))
	digest := designImportPreviewDigest(t, bin, root, reqBytes)

	cases := []struct {
		name string
		args []string
	}{
		{"missing --request", []string{"apply", "--preview", digest, "--harness", "codex"}},
		{"missing --preview", []string{"apply", "--request", "-", "--harness", "codex"}},
		{"missing --harness", []string{"apply", "--request", "-", "--preview", digest}},
		{"blank --harness", []string{"apply", "--request", "-", "--preview", digest, "--harness", "   "}},
		{"malformed --preview", []string{"apply", "--request", "-", "--preview", "not-a-digest", "--harness", "codex"}},
		{"uppercase --preview", []string{"apply", "--request", "-", "--preview", strings.ToUpper(digest), "--harness", "codex"}},
		{"duplicate flag", []string{"apply", "--request", "-", "--request", "-", "--preview", digest, "--harness", "codex"}},
		{"unknown flag", []string{"apply", "--request", "-", "--preview", digest, "--harness", "codex", "--foo", "bar"}},
		{"human flag rejected", []string{"apply", "--request", "-", "--preview", digest, "--harness", "codex", "--human", "1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			run := runDesignImportBinary(t, bin, root, reqBytes, nil, tc.args...)
			if run.code != 2 || run.stdout != "" || !strings.HasPrefix(run.stderr, "design import apply: invalid-request:") {
				t.Fatalf("%s = %+v", tc.name, run)
			}
		})
	}
}

// TestDesignImportRecordBuiltBinary proves `verdi design import record`
// reads truthfully immediately after creation, discloses a later ordinary
// edit without corrupting the original provenance, refuses a branch with
// no import record, and refuses an invalid --spec at the usage layer.
func TestDesignImportRecordBuiltBinary(t *testing.T) {
	t.Parallel()
	bin := buildVerdiBinary(t)

	t.Run("matches after apply, then discloses a later edit truthfully", func(t *testing.T) {
		root := designImportPolicyRepo(t, "draft-write")
		slug := "sample-feature-record"
		branch := "design/" + slug
		reqBytes := designImportRequestJSON(t, designImportReadyRequest(t, slug))
		digest := designImportPreviewDigest(t, bin, root, reqBytes)

		applyRun := runDesignImportBinary(t, bin, root, reqBytes, nil, "apply", "--request", "-", "--preview", digest, "--harness", "codex", "--session", "session-1")
		if applyRun.code != 0 {
			t.Fatalf("apply for record test = %+v", applyRun)
		}

		firstRun := runDesignImportBinary(t, bin, root, nil, nil, "record", "--branch", branch, "--spec", slug)
		if firstRun.code != 0 || firstRun.stderr != "" {
			t.Fatalf("record before edit = %+v", firstRun)
		}
		var firstView specimport.RecordView
		if err := json.Unmarshal([]byte(firstRun.stdout), &firstView); err != nil {
			t.Fatalf("decoding record view: %v\n%s", err, firstRun.stdout)
		}
		if !firstView.CurrentSpecMatches || len(firstView.Disclosures) != 0 {
			t.Fatalf("record before edit = %+v", firstView)
		}
		// uat-round-1 spec ac-4 (closing UAT-005): the record names the
		// profile that produced it. designImportReadyRequest uses
		// markdown-v1, which names no pinned reference profile, so
		// ProfilePrimaryDigest must stay absent for it.
		if firstView.Record.Format != specimport.FormatMarkdownV1 {
			t.Fatalf("record.Format = %q, want %q", firstView.Record.Format, specimport.FormatMarkdownV1)
		}
		if firstView.Record.ProfilePrimaryDigest != "" {
			t.Fatalf("record.ProfilePrimaryDigest = %q, want absent for markdown-v1", firstView.Record.ProfilePrimaryDigest)
		}
		if !strings.Contains(firstRun.stdout, `"format":"markdown-v1"`) {
			t.Fatalf("record command's raw JSON output does not carry the format field: %s", firstRun.stdout)
		}
		if strings.Contains(firstRun.stdout, `"profile_primary_digest"`) {
			t.Fatalf("record command's raw JSON output carries profile_primary_digest for a format with no bound profile: %s", firstRun.stdout)
		}

		designImportEditActiveSpec(t, root, branch, slug, func(data []byte) []byte {
			edited := bytes.Replace(data, []byte("Users get value."), []byte("Users get updated value."), 1)
			if bytes.Equal(edited, data) {
				t.Fatalf("edit did not change spec.md content: %s", data)
			}
			return edited
		})

		secondRun := runDesignImportBinary(t, bin, root, nil, nil, "record", "--branch", branch, "--spec", slug)
		if secondRun.code != 0 || secondRun.stderr != "" {
			t.Fatalf("record after edit = %+v", secondRun)
		}
		var secondView specimport.RecordView
		if err := json.Unmarshal([]byte(secondRun.stdout), &secondView); err != nil {
			t.Fatalf("decoding record view after edit: %v\n%s", err, secondRun.stdout)
		}
		if secondView.CurrentSpecMatches {
			t.Fatalf("record after edit still claims current_spec_matches: %+v", secondView)
		}
		found := false
		for _, f := range secondView.Disclosures {
			if f.Code == specimport.FindingCurrentSpecChanged && !f.Blocking {
				found = true
			}
		}
		if !found {
			t.Fatalf("record after edit missing a nonblocking current-spec-changed disclosure: %+v", secondView.Disclosures)
		}
		if secondView.ImportCommit != firstView.ImportCommit {
			t.Fatalf("import commit changed across record reads: %q vs %q", firstView.ImportCommit, secondView.ImportCommit)
		}
	})

	t.Run("branch with no import record is a provenance-mismatch refusal", func(t *testing.T) {
		root := designImportRepo(t)
		run := runDesignImportBinary(t, bin, root, nil, nil, "record", "--branch", "main", "--spec", "never-imported")
		if run.code != 1 || run.stdout != "" || !strings.HasPrefix(run.stderr, "design import record: provenance-mismatch:") {
			t.Fatalf("missing record = %+v", run)
		}
	})

	t.Run("invalid --spec is a usage refusal", func(t *testing.T) {
		root := designImportRepo(t)
		run := runDesignImportBinary(t, bin, root, nil, nil, "record", "--branch", "main", "--spec", "Not A Slug!")
		if run.code != 2 || run.stdout != "" || !strings.HasPrefix(run.stderr, "design import record: invalid-request:") {
			t.Fatalf("invalid slug = %+v", run)
		}
	})

	t.Run("missing/blank --branch and unknown flag", func(t *testing.T) {
		root := designImportRepo(t)
		cases := []struct {
			name string
			args []string
		}{
			{"missing --branch", []string{"record", "--spec", "sample-feature"}},
			{"missing --spec", []string{"record", "--branch", "main"}},
			{"unknown flag", []string{"record", "--branch", "main", "--spec", "sample-feature", "--bogus", "x"}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				run := runDesignImportBinary(t, bin, root, nil, nil, tc.args...)
				if run.code != 2 || run.stdout != "" || !strings.HasPrefix(run.stderr, "design import record: invalid-request:") {
					t.Fatalf("%s = %+v", tc.name, run)
				}
			})
		}
	})
}
