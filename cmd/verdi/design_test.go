package main

import (
	"bytes"
	"context"
	"testing"

	"github.com/OWNER/verdi/internal/gitx"
	"github.com/OWNER/verdi/internal/provider"
	providerfake "github.com/OWNER/verdi/internal/provider/fake"
)

func seedFakeProvider(t *testing.T) *providerfake.Provider {
	t.Helper()
	p := providerfake.New()
	p.SeedStory(provider.Story{Ref: "jira:LOAN-1482", Title: "Stale decline handling", Status: "In Progress", URL: "https://example.atlassian.net/browse/LOAN-1482"})
	return p
}

// TestRunDesignStart_Happy proves the whole scaffold ritual: branch cut,
// draft spec written with the provider-resolved title, scaffold committed,
// board placeholder printed.
func TestRunDesignStart_Happy(t *testing.T) {
	repo := buildPhase7Repo(t)
	ctx := context.Background()
	manifest := phase7Manifest(t)
	deps := designDeps{Provider: seedFakeProvider(t), Runner: nil, GoTest: fakeGoTest{}}

	var stdout, stderr bytes.Buffer
	got := runDesignStart(ctx, repo.Dir, "jira:LOAN-1482", "stale-decline", manifest, deps, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runDesignStart = %d, want 0; stderr=%s", got, stderr.String())
	}

	branch, err := gitx.CurrentBranch(ctx, repo.Dir)
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if branch != "design/stale-decline" {
		t.Fatalf("CurrentBranch = %q, want design/stale-decline", branch)
	}

	spec, _ := readSpec(t, repo.Dir, "stale-decline")
	if spec.Status != "draft" {
		t.Fatalf("spec.Status = %q, want draft", spec.Status)
	}
	if spec.Story != "jira:LOAN-1482" {
		t.Fatalf("spec.Story = %q, want jira:LOAN-1482", spec.Story)
	}
	if spec.Title != "Stale decline handling" {
		t.Fatalf("spec.Title = %q, want the provider-resolved title", spec.Title)
	}
	if spec.Class != "feature" {
		t.Fatalf("spec.Class = %q, want feature", spec.Class)
	}

	head, err := gitx.RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatalf("RevParse(HEAD): %v", err)
	}
	if head == repo.Head {
		t.Fatal("design start did not create a new commit")
	}

	if !contains(stdout.String(), "board:") {
		t.Fatalf("stdout = %q, want a board URL placeholder line", stdout.String())
	}
}

// TestRunDesignStart_ProviderResolveFails_DegradesToRawRef proves 04
// §Semantics's degrade-to-raw-ref path: a provider that cannot resolve the
// story never blocks the scaffold, and the disclosed degrade is visible on
// stderr.
func TestRunDesignStart_ProviderResolveFails_DegradesToRawRef(t *testing.T) {
	repo := buildPhase7Repo(t)
	ctx := context.Background()
	manifest := phase7Manifest(t)

	p := providerfake.New()
	p.FailResolve("jira:LOAN-9999", provider.ErrNotFound)
	deps := designDeps{Provider: p, Runner: nil, GoTest: fakeGoTest{}}

	var stdout, stderr bytes.Buffer
	got := runDesignStart(ctx, repo.Dir, "jira:LOAN-9999", "some-feature", manifest, deps, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runDesignStart = %d, want 0; stderr=%s", got, stderr.String())
	}

	spec, _ := readSpec(t, repo.Dir, "some-feature")
	if spec.Title != "jira:LOAN-9999" {
		t.Fatalf("spec.Title = %q, want the degraded raw ref", spec.Title)
	}
	if !contains(stderr.String(), "degraded") {
		t.Fatalf("stderr = %q, want a disclosed degrade message", stderr.String())
	}
}

// TestRunDesignStart_Negative covers runDesignStart's own operational
// error paths.
func TestRunDesignStart_Negative(t *testing.T) {
	manifest := phase7Manifest(t)
	deps := designDeps{Provider: seedFakeProvider(t), Runner: nil, GoTest: fakeGoTest{}}
	ctx := context.Background()

	t.Run("invalid name", func(t *testing.T) {
		repo := buildPhase7Repo(t)
		var stdout, stderr bytes.Buffer
		got := runDesignStart(ctx, repo.Dir, "jira:LOAN-1482", "Not_A_Valid_Name", manifest, deps, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("runDesignStart(invalid name) = %d, want 2", got)
		}
		if stderr.Len() == 0 {
			t.Fatal("expected an explanatory stderr message")
		}
	})

	t.Run("malformed story ref", func(t *testing.T) {
		repo := buildPhase7Repo(t)
		var stdout, stderr bytes.Buffer
		got := runDesignStart(ctx, repo.Dir, "not-a-story-ref", "some-name", manifest, deps, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("runDesignStart(malformed story ref) = %d, want 2", got)
		}
	})

	t.Run("unconfigured scheme", func(t *testing.T) {
		repo := buildPhase7Repo(t)
		var stdout, stderr bytes.Buffer
		got := runDesignStart(ctx, repo.Dir, "confluence:PAGE-1", "some-name", manifest, deps, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("runDesignStart(unconfigured scheme) = %d, want 2", got)
		}
		if !contains(stderr.String(), "confluence") {
			t.Fatalf("stderr = %q, want it to name the unconfigured scheme", stderr.String())
		}
	})

	t.Run("spec already exists", func(t *testing.T) {
		repo := buildPhase7Repo(t)
		var stdout, stderr bytes.Buffer
		if got := runDesignStart(ctx, repo.Dir, "jira:LOAN-1482", "stale-decline", manifest, deps, &stdout, &stderr); got != 0 {
			t.Fatalf("first runDesignStart = %d, want 0; stderr=%s", got, stderr.String())
		}
		stdout.Reset()
		stderr.Reset()
		got := runDesignStart(ctx, repo.Dir, "jira:LOAN-1482", "stale-decline", manifest, deps, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("second runDesignStart(same name) = %d, want 2", got)
		}
	})
}

// TestCmdDesignStart_NameFlagOrdering proves --name parses correctly
// whether it comes before or after the positional story-ref — in
// particular the "<story-ref> --name <name>" ordering PLAN.md Phase 7's
// own exit criteria and 05 §CLI's example both use, which the stdlib flag
// package cannot parse (it stops consuming flags at the first non-flag
// token), hence extractNameFlag's hand-rolled parse.
func TestCmdDesignStart_NameFlagOrdering(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"flag after positional", []string{"jira:LOAN-1482", "--name", "stale-decline"}},
		{"flag before positional", []string{"--name", "stale-decline", "jira:LOAN-1482"}},
		{"flag=value after positional", []string{"jira:LOAN-1482", "--name=stale-decline"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := buildPhase7Repo(t)
			t.Chdir(repo.Dir)

			var stdout, stderr bytes.Buffer
			got := cmdDesignStart(tc.args, &stdout, &stderr)
			if got != 0 {
				t.Fatalf("cmdDesignStart(%v) = %d, want 0; stderr=%s", tc.args, got, stderr.String())
			}
			readSpec(t, repo.Dir, "stale-decline") // fails the test if not found/decodable
		})
	}
}

// TestCmdDesignStart_NameFlagMissing proves --name is required at the
// flag-parsing layer (I-10), exiting 2 before touching the store at all.
func TestCmdDesignStart_NameFlagMissing(t *testing.T) {
	repo := buildPhase7Repo(t)
	t.Chdir(repo.Dir)

	var stdout, stderr bytes.Buffer
	got := cmdDesignStart([]string{"jira:LOAN-1482"}, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("cmdDesignStart(no --name) = %d, want 2", got)
	}
	if !contains(stderr.String(), "--name") {
		t.Fatalf("stderr = %q, want it to mention --name", stderr.String())
	}
}

// TestRunDesignVerb_UnknownSubcommand proves the design/start subcommand
// dispatch is a usage error for anything but "start".
func TestRunDesignVerb_UnknownSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := runDesignVerb([]string{"bogus"}, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("runDesignVerb(bogus) = %d, want 2", got)
	}

	stdout.Reset()
	stderr.Reset()
	got = runDesignVerb(nil, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("runDesignVerb(no args) = %d, want 2", got)
	}
}

// TestRun_DesignDispatchesToRealVerb proves dispatch.go routes "design" to
// the real implementation, matching the equivalent lint/sync/matrix tests.
func TestRun_DesignDispatchesToRealVerb(t *testing.T) {
	t.Chdir(t.TempDir())
	var stderr bytes.Buffer
	got := run([]string{"design", "start", "jira:LOAN-1", "--name", "x"}, &stderr)
	if got != 2 {
		t.Fatalf("run([design start ...]) outside a store = %d, want 2 (operational)", got)
	}
	if contains(stderr.String(), "usage") || contains(stderr.String(), "not implemented") {
		t.Fatalf("stderr = %q, want a real store-root error, not the generic stub message", stderr.String())
	}
}
