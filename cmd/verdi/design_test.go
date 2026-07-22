package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/provider"
	providerfake "github.com/jyang234/verdi/internal/provider/fake"
)

func seedFakeProvider(t *testing.T) *providerfake.Provider {
	t.Helper()
	p := providerfake.New()
	p.SeedStory(provider.Story{Ref: "jira:LOAN-1482", Title: "Stale decline handling", Status: "In Progress", URL: "https://example.atlassian.net/browse/LOAN-1482"})
	return p
}

// TestRunDesignStart_Happy proves the whole scaffold ritual for a
// --kind feature spec: branch cut, draft spec written with the
// provider-resolved title, scaffold committed (carrying attributes, ACs,
// and stubs per 05 §CLI's own exit criterion), board placeholder printed.
func TestRunDesignStart_Happy(t *testing.T) {
	repo := buildPhase7Repo(t)
	ctx := context.Background()
	manifest := phase7Manifest(t)
	deps := designDeps{Provider: seedFakeProvider(t), Runner: nil, GoTest: fakeGoTest{}, DeferStatements: true}

	var stdout, stderr bytes.Buffer
	got := runDesignStart(ctx, repo.Dir, artifact.ClassFeature, "jira:LOAN-1482", "stale-decline", manifest, phase7Model(t), deps, &stdout, &stderr)
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
	if spec.Problem == nil || spec.Outcome == nil {
		t.Fatal("scaffolded feature spec must carry problem/outcome attributes (05 §CLI exit criterion)")
	}
	if len(spec.AcceptanceCriteria) == 0 {
		t.Fatal("scaffolded feature spec must carry at least one acceptance criterion")
	}
	if len(spec.Stubs) == 0 {
		t.Fatal("scaffolded feature spec must carry at least one stub (05 §CLI exit criterion)")
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

// TestRunDesignStart_FeatureWithNoRef proves a feature's tracker ref is
// optional (05 §CLI, 02 §Kind registry's okr:LOAN-Q3 example): design start
// --kind feature with no ref at all scaffolds a draft feature spec with an
// empty story: field.
func TestRunDesignStart_FeatureWithNoRef(t *testing.T) {
	repo := buildPhase7Repo(t)
	ctx := context.Background()
	manifest := phase7Manifest(t)
	deps := designDeps{Provider: seedFakeProvider(t), Runner: nil, GoTest: fakeGoTest{}, DeferStatements: true}

	var stdout, stderr bytes.Buffer
	got := runDesignStart(ctx, repo.Dir, artifact.ClassFeature, "", "loan-mgmt", manifest, phase7Model(t), deps, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runDesignStart(no ref) = %d, want 0; stderr=%s", got, stderr.String())
	}

	spec, _ := readSpec(t, repo.Dir, "loan-mgmt")
	if spec.Story != "" {
		t.Fatalf("spec.Story = %q, want empty (a feature ref is optional)", spec.Story)
	}
	if spec.Class != artifact.ClassFeature {
		t.Fatalf("spec.Class = %q, want feature", spec.Class)
	}
}

// TestRunDesignStart_FeatureWithEpicRef proves a feature MAY carry an
// epic/objective tracker ref (02 §Kind registry's own okr:LOAN-Q3 example)
// when the store configures that scheme.
func TestRunDesignStart_FeatureWithEpicRef(t *testing.T) {
	repo := buildPhase7Repo(t)
	ctx := context.Background()
	manifest := phase7Manifest(t)
	deps := designDeps{Provider: seedFakeProvider(t), Runner: nil, GoTest: fakeGoTest{}, DeferStatements: true}

	var stdout, stderr bytes.Buffer
	got := runDesignStart(ctx, repo.Dir, artifact.ClassFeature, "jira:LOAN-1482", "loan-mgmt", manifest, phase7Model(t), deps, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runDesignStart(epic ref) = %d, want 0; stderr=%s", got, stderr.String())
	}
	spec, _ := readSpec(t, repo.Dir, "loan-mgmt")
	if spec.Story != "jira:LOAN-1482" {
		t.Fatalf("spec.Story = %q, want jira:LOAN-1482", spec.Story)
	}
}

// TestRunDesignStart_Story proves --kind story scaffolds a class: story
// spec, requires its ref, and carries the object-model fields validateStory
// requires (problem/outcome/an implements edge).
func TestRunDesignStart_Story(t *testing.T) {
	repo := buildPhase7Repo(t)
	ctx := context.Background()
	manifest := phase7Manifest(t)
	deps := designDeps{Provider: seedFakeProvider(t), Runner: nil, GoTest: fakeGoTest{}, DeferStatements: true}

	var stdout, stderr bytes.Buffer
	got := runDesignStart(ctx, repo.Dir, artifact.ClassStory, "jira:LOAN-1482", "stale-decline-story", manifest, phase7Model(t), deps, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runDesignStart(story) = %d, want 0; stderr=%s", got, stderr.String())
	}

	spec, _ := readSpec(t, repo.Dir, "stale-decline-story")
	if spec.Class != artifact.ClassStory {
		t.Fatalf("spec.Class = %q, want story", spec.Class)
	}
	if spec.Story != "jira:LOAN-1482" {
		t.Fatalf("spec.Story = %q, want jira:LOAN-1482", spec.Story)
	}
	if spec.Problem == nil || spec.Outcome == nil {
		t.Fatal("scaffolded story spec must carry problem/outcome attributes")
	}
}

// TestRunDesignStart_StoryRequiresRef proves --kind story refuses (exit 2,
// operational: a usage precondition, not a business verdict) with no ref at
// all — the story class REQUIRES the scheme-prefixed story ref (05 §CLI).
func TestRunDesignStart_StoryRequiresRef(t *testing.T) {
	repo := buildPhase7Repo(t)
	ctx := context.Background()
	manifest := phase7Manifest(t)
	deps := designDeps{Provider: seedFakeProvider(t), Runner: nil, GoTest: fakeGoTest{}, DeferStatements: true}

	var stdout, stderr bytes.Buffer
	got := runDesignStart(ctx, repo.Dir, artifact.ClassStory, "", "some-story", manifest, phase7Model(t), deps, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("runDesignStart(story, no ref) = %d, want 2", got)
	}
	if !contains(stderr.String(), "requires") {
		t.Fatalf("stderr = %q, want it to name the required-ref refusal", stderr.String())
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
	deps := designDeps{Provider: p, Runner: nil, GoTest: fakeGoTest{}, DeferStatements: true}

	var stdout, stderr bytes.Buffer
	got := runDesignStart(ctx, repo.Dir, artifact.ClassFeature, "jira:LOAN-9999", "some-feature", manifest, phase7Model(t), deps, &stdout, &stderr)
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

// TestCmdDesignStart_WiresConfiguredProvider proves D-3's fix: design start
// builds its provider registry from verdi.yaml's providers: map (the same
// buildProviderRegistry construction rollup/sync use), so a configured
// scheme (jira) resolves through a real adapter instead of always degrading
// with ErrUnknownScheme, while an unconfigured scheme still honestly misses.
func TestCmdDesignStart_WiresConfiguredProvider(t *testing.T) {
	reg := buildProviderRegistry(phase7Manifest(t))
	if _, err := reg.Provider("jira"); err != nil {
		t.Fatalf("Provider(jira) = %v, want a real adapter (design start must attempt real resolution for a configured scheme, not ErrUnknownScheme)", err)
	}
	if _, err := reg.Provider("confluence"); err == nil {
		t.Fatal("Provider(confluence) = nil error, want a miss for an unconfigured scheme")
	}
}

// TestRunDesignStart_ConfiguredProviderUnreachable_DegradesForTrueReason
// proves that once the real registry is wired, a configured-but-unreachable
// ref degrades for the TRUE reason (Unavailable/NotFound), never the generic
// ErrUnknownScheme that reads as "this scheme isn't configured" (D-3).
func TestRunDesignStart_ConfiguredProviderUnreachable_DegradesForTrueReason(t *testing.T) {
	repo := buildPhase7Repo(t)
	ctx := context.Background()
	manifest := phase7Manifest(t)

	p := providerfake.New()
	p.FailResolve("jira:LOAN-9999", provider.ErrUnavailable)
	deps := designDeps{Provider: p, Runner: nil, GoTest: fakeGoTest{}, DeferStatements: true}

	var stdout, stderr bytes.Buffer
	got := runDesignStart(ctx, repo.Dir, artifact.ClassFeature, "jira:LOAN-9999", "some-feature", manifest, phase7Model(t), deps, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runDesignStart = %d, want 0; stderr=%s", got, stderr.String())
	}
	if !contains(stderr.String(), "degraded") || !contains(stderr.String(), "unavailable") {
		t.Fatalf("stderr = %q, want the true resolution-failure reason (unavailable)", stderr.String())
	}
	if contains(stderr.String(), "unknown scheme") {
		t.Fatalf("stderr = %q, must NOT degrade with ErrUnknownScheme for a configured, resolvable-scheme ref", stderr.String())
	}
}

// TestRunDesignStart_Negative covers runDesignStart's own operational
// error paths.
func TestRunDesignStart_Negative(t *testing.T) {
	manifest := phase7Manifest(t)
	deps := designDeps{Provider: seedFakeProvider(t), Runner: nil, GoTest: fakeGoTest{}, DeferStatements: true}
	ctx := context.Background()

	t.Run("invalid name", func(t *testing.T) {
		repo := buildPhase7Repo(t)
		var stdout, stderr bytes.Buffer
		got := runDesignStart(ctx, repo.Dir, artifact.ClassFeature, "jira:LOAN-1482", "Not_A_Valid_Name", manifest, phase7Model(t), deps, &stdout, &stderr)
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
		got := runDesignStart(ctx, repo.Dir, artifact.ClassFeature, "not-a-story-ref", "some-name", manifest, phase7Model(t), deps, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("runDesignStart(malformed story ref) = %d, want 2", got)
		}
	})

	t.Run("unconfigured scheme", func(t *testing.T) {
		repo := buildPhase7Repo(t)
		var stdout, stderr bytes.Buffer
		got := runDesignStart(ctx, repo.Dir, artifact.ClassFeature, "confluence:PAGE-1", "some-name", manifest, phase7Model(t), deps, &stdout, &stderr)
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
		if got := runDesignStart(ctx, repo.Dir, artifact.ClassFeature, "jira:LOAN-1482", "stale-decline", manifest, phase7Model(t), deps, &stdout, &stderr); got != 0 {
			t.Fatalf("first runDesignStart = %d, want 0; stderr=%s", got, stderr.String())
		}
		stdout.Reset()
		stderr.Reset()
		got := runDesignStart(ctx, repo.Dir, artifact.ClassFeature, "jira:LOAN-1482", "stale-decline", manifest, phase7Model(t), deps, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("second runDesignStart(same name) = %d, want 2", got)
		}
	})
}

// TestCmdDesignStart_NameFlagOrdering proves --name/--kind parse correctly
// in every position relative to the positional story-ref — in particular
// the "<story-ref> --kind feature --name <name>" ordering 05 §CLI's own
// example uses, which the stdlib flag package cannot parse (it stops
// consuming flags at the first non-flag token), hence extractFlags's
// hand-rolled parse.
func TestCmdDesignStart_NameFlagOrdering(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"flags after positional", []string{"jira:LOAN-1482", "--kind", "feature", "--name", "stale-decline", "--defer-statements"}},
		{"flags before positional", []string{"--kind", "feature", "--name", "stale-decline", "--defer-statements", "jira:LOAN-1482"}},
		{"flag=value form", []string{"jira:LOAN-1482", "--kind=feature", "--name=stale-decline", "--defer-statements"}},
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
	got := cmdDesignStart([]string{"jira:LOAN-1482", "--kind", "feature"}, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("cmdDesignStart(no --name) = %d, want 2", got)
	}
	if !contains(stderr.String(), "--name") {
		t.Fatalf("stderr = %q, want it to mention --name", stderr.String())
	}
}

// TestCmdDesignStart_KindFlagMissingOrInvalid proves --kind is required and
// closed to feature|story.
func TestCmdDesignStart_KindFlagMissingOrInvalid(t *testing.T) {
	repo := buildPhase7Repo(t)
	t.Chdir(repo.Dir)

	t.Run("missing", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		got := cmdDesignStart([]string{"jira:LOAN-1482", "--name", "x"}, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("cmdDesignStart(no --kind) = %d, want 2", got)
		}
		if !contains(stderr.String(), "--kind") {
			t.Fatalf("stderr = %q, want it to mention --kind", stderr.String())
		}
	})

	t.Run("invalid", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		got := cmdDesignStart([]string{"jira:LOAN-1482", "--kind", "epic", "--name", "x"}, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("cmdDesignStart(--kind epic) = %d, want 2", got)
		}
	})
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
// the real implementation.
func TestRun_DesignDispatchesToRealVerb(t *testing.T) {
	t.Chdir(t.TempDir())
	var stderr bytes.Buffer
	got := run([]string{"design", "start", "jira:LOAN-1", "--kind", "feature", "--name", "x"}, &stderr)
	if got != 2 {
		t.Fatalf("run([design start ...]) outside a store = %d, want 2 (operational)", got)
	}
	if contains(stderr.String(), "usage") || contains(stderr.String(), "not implemented") {
		t.Fatalf("stderr = %q, want a real store-root error, not the generic stub message", stderr.String())
	}
}

// TestRunDesignStart_ScaffoldUsesAtomicWrite is Task 1 of the
// extensibility-phase1 plan (audit CLEANUP-BEFORE #1): the scaffold's
// spec.md write was a plain os.WriteFile — truncate-then-write, no
// crash-durability guarantee and no fsync. This proves the fixed write
// leaves no temp sibling in the spec directory, across both spec classes.
func TestRunDesignStart_ScaffoldUsesAtomicWrite(t *testing.T) {
	tests := []struct {
		name     string
		kind     artifact.SpecClass
		specName string
	}{
		{"feature scaffold", artifact.ClassFeature, "atomic-feature"},
		{"story scaffold", artifact.ClassStory, "atomic-story"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := buildPhase7Repo(t)
			ctx := context.Background()
			manifest := phase7Manifest(t)
			deps := designDeps{Provider: seedFakeProvider(t), Runner: nil, GoTest: fakeGoTest{}, DeferStatements: true}

			var stdout, stderr bytes.Buffer
			got := runDesignStart(ctx, repo.Dir, tc.kind, "jira:LOAN-1482", tc.specName, manifest, phase7Model(t), deps, &stdout, &stderr)
			if got != 0 {
				t.Fatalf("runDesignStart = %d, want 0; stderr=%s", got, stderr.String())
			}

			specDir := filepath.Join(repo.Dir, ".verdi", "specs", "active", tc.specName)
			entries, err := os.ReadDir(specDir)
			if err != nil {
				t.Fatalf("ReadDir(%s): %v", specDir, err)
			}
			for _, e := range entries {
				if strings.Contains(e.Name(), ".tmp") {
					t.Fatalf("leftover temp file %s", e.Name())
				}
			}
			if len(entries) != 1 || entries[0].Name() != "spec.md" {
				names := make([]string, len(entries))
				for i, e := range entries {
					names[i] = e.Name()
				}
				t.Fatalf("specDir entries = %v, want exactly [spec.md]", names)
			}
		})
	}
}

// TestDesignGo_AtomicWrite_NoDirectWriteFile is a source-text witness:
// design.go's scaffold write must route through atomicfile.Write, never a
// plain os.WriteFile CALL (CLEANUP-BEFORE #1 — the same crash-durability
// gap atomicfile.Write already closed for boardio/boardlayout/
// disposition.go). Matches the call form "os.WriteFile(" rather than the
// bare identifier so a doc comment merely naming the old API in prose
// (as this very fix's own comment does, contrasting the two) can never
// false-positive the check.
func TestDesignGo_AtomicWrite_NoDirectWriteFile(t *testing.T) {
	data, err := os.ReadFile("design.go")
	if err != nil {
		t.Fatalf("reading design.go: %v", err)
	}
	if strings.Contains(string(data), "os.WriteFile(") {
		t.Error("design.go calls os.WriteFile directly — the scaffold write must route through internal/atomicfile.Write instead (CLEANUP-BEFORE #1)")
	}
}
