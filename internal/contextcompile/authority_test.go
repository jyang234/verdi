package contextcompile

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/policyauthority"
	"github.com/jyang234/verdi/internal/repositoryfacts"
	"github.com/jyang234/verdi/internal/specstate"
)

const acceptedStoryBytes = `---
id: spec/example-story
kind: spec
title: Example story
owners: [platform-team]
class: story
story: jira:EX-1
problem: {text: "The example is missing.", anchor: problem}
outcome: {text: "The example exists.", anchor: outcome}
acceptance_criteria:
  - {id: ac-1, text: "the example exists", evidence: [static], anchor: ac-1}
links:
  - {type: implements, ref: spec/example-feature#ac-1}
---
# Example story

## Problem

Missing.

## Outcome

Exists.

## AC-1

The example exists.
`

type authorityGit struct {
	show func(context.Context, string, string, string) ([]byte, error)
	tree func(context.Context, string, string) ([]gitx.TreeEntry, error)
}

func (g authorityGit) Show(ctx context.Context, root, ref, path string) ([]byte, error) {
	return g.show(ctx, root, ref, path)
}

func (g authorityGit) LsTreeEntries(ctx context.Context, root, ref string) ([]gitx.TreeEntry, error) {
	return g.tree(ctx, root, ref)
}

func (authorityGit) WorktreeChangedPaths(context.Context, string) ([]string, error) {
	panic("unexpected WorktreeChangedPaths call")
}

type authorityStateResolver struct {
	resolve func(context.Context, string, specstate.Candidate) (specstate.Result, error)
}

func (r authorityStateResolver) Resolve(ctx context.Context, root string, candidate specstate.Candidate) (specstate.Result, error) {
	return r.resolve(ctx, root, candidate)
}

func TestResolveAcceptedSpec(t *testing.T) {
	const (
		root    = "/repo"
		head    = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		path    = ".verdi/specs/active/example-story/spec.md"
		blob    = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		landing = "cccccccccccccccccccccccccccccccccccccccc"
	)
	wantBytes := []byte(acceptedStoryBytes)

	git := authorityGit{
		tree: func(_ context.Context, gotRoot, gotRef string) ([]gitx.TreeEntry, error) {
			if gotRoot != root || gotRef != head {
				t.Fatalf("LsTreeEntries(%q, %q), want (%q, %q)", gotRoot, gotRef, root, head)
			}
			return []gitx.TreeEntry{{Mode: "100644", Type: "blob", Object: blob, Path: path}}, nil
		},
		show: func(_ context.Context, gotRoot, gotRef, gotPath string) ([]byte, error) {
			if gotRoot != root || gotRef != head || gotPath != path {
				t.Fatalf("Show(%q, %q, %q), want (%q, %q, %q)", gotRoot, gotRef, gotPath, root, head, path)
			}
			return append([]byte(nil), wantBytes...), nil
		},
	}
	states := authorityStateResolver{resolve: func(_ context.Context, gotRoot string, candidate specstate.Candidate) (specstate.Result, error) {
		if gotRoot != root || candidate.Path != path || string(candidate.Content) != string(wantBytes) {
			t.Fatalf("Resolve(%q, %+v), want exact HEAD candidate %q", gotRoot, candidate, path)
		}
		return specstate.Result{
			State:    specstate.AcceptedPendingBuild,
			Relation: specstate.RelationExact,
			Baseline: &specstate.Baseline{Path: path, Blob: blob, LandingCommit: landing},
		}, nil
	}}

	got, err := ResolveAcceptedSpec(context.Background(), git, states, root, head, "spec/example-story")
	if err != nil {
		t.Fatalf("ResolveAcceptedSpec: %v", err)
	}
	if got.Ref != "spec/example-story" || got.Path != path || got.Blob != blob || got.Commit != landing {
		t.Fatalf("ResolveAcceptedSpec identity = %+v", got)
	}
	if string(got.Content) != string(wantBytes) {
		t.Fatalf("ResolveAcceptedSpec content = %q, want exact HEAD bytes %q", got.Content, wantBytes)
	}
	if got.ContentDigest != rawContentDigest(wantBytes) {
		t.Fatalf("ResolveAcceptedSpec content digest = %q, want exact-byte digest %q", got.ContentDigest, rawContentDigest(wantBytes))
	}
	if got.Spec == nil || got.Spec.ID != "spec/example-story" {
		t.Fatalf("ResolveAcceptedSpec decoded spec = %+v", got.Spec)
	}
}

func TestResolveAcceptedSpec_StateTable(t *testing.T) {
	const (
		root        = "/repo"
		head        = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		activePath  = ".verdi/specs/active/example-story/spec.md"
		archivePath = ".verdi/specs/archive/example-story/spec.md"
		blob        = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		landing     = "cccccccccccccccccccccccccccccccccccccccc"
	)

	tests := []struct {
		name            string
		entries         []gitx.TreeEntry
		content         []byte
		state           specstate.Result
		wantPath        string
		wantRefusal     bool
		wantOperational bool
	}{
		{
			name:     "accepted active",
			entries:  []gitx.TreeEntry{{Mode: "100644", Type: "blob", Object: blob, Path: activePath}},
			content:  []byte(acceptedStoryBytes),
			state:    specstate.Result{State: specstate.AcceptedPendingBuild, Relation: specstate.RelationExact, Baseline: &specstate.Baseline{Path: activePath, Blob: blob, LandingCommit: landing}},
			wantPath: activePath,
		},
		{
			name:     "archived accepted",
			entries:  []gitx.TreeEntry{{Mode: "100644", Type: "blob", Object: blob, Path: archivePath}},
			content:  []byte(acceptedStoryBytes),
			state:    specstate.Result{State: specstate.Closed, Relation: specstate.RelationExact, Baseline: &specstate.Baseline{Path: archivePath, Blob: blob, LandingCommit: landing}},
			wantPath: archivePath,
		},
		{
			name:        "proposed",
			entries:     []gitx.TreeEntry{{Mode: "100644", Type: "blob", Object: blob, Path: activePath}},
			content:     []byte(acceptedStoryBytes),
			state:       specstate.Result{State: specstate.Proposed, Relation: specstate.RelationNew},
			wantRefusal: true,
		},
		{
			name:        "ambiguous successor is unproven",
			entries:     []gitx.TreeEntry{{Mode: "100644", Type: "blob", Object: blob, Path: activePath}},
			content:     []byte(acceptedStoryBytes),
			state:       specstate.Result{State: specstate.Unproven, Relation: specstate.RelationUnproven, Disclosures: []string{"multiple valid successors"}},
			wantRefusal: true,
		},
		{
			name:        "superseded",
			entries:     []gitx.TreeEntry{{Mode: "100644", Type: "blob", Object: blob, Path: activePath}},
			content:     []byte(acceptedStoryBytes),
			state:       specstate.Result{State: specstate.Superseded, Relation: specstate.RelationExact, Baseline: &specstate.Baseline{Path: activePath, Blob: blob, LandingCommit: landing}},
			wantRefusal: true,
		},
		{
			name:        "missing",
			wantRefusal: true,
		},
		{
			name:            "malformed accepted authority",
			entries:         []gitx.TreeEntry{{Mode: "100644", Type: "blob", Object: blob, Path: activePath}},
			content:         []byte("---\nid: spec/example-story\nunknown: true\n---\n"),
			wantOperational: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resolveCalls int
			git := authorityGit{
				tree: func(context.Context, string, string) ([]gitx.TreeEntry, error) {
					return append([]gitx.TreeEntry(nil), tt.entries...), nil
				},
				show: func(_ context.Context, _ string, _ string, gotPath string) ([]byte, error) {
					if len(tt.entries) != 1 || gotPath != tt.entries[0].Path {
						t.Fatalf("Show path = %q, entries = %+v", gotPath, tt.entries)
					}
					return append([]byte(nil), tt.content...), nil
				},
			}
			states := authorityStateResolver{resolve: func(context.Context, string, specstate.Candidate) (specstate.Result, error) {
				resolveCalls++
				return tt.state, nil
			}}

			got, err := ResolveAcceptedSpec(context.Background(), git, states, root, head, "spec/example-story")
			switch {
			case tt.wantRefusal:
				if err == nil {
					t.Fatal("ResolveAcceptedSpec: want refusal, got nil")
				}
				if !errors.Is(err, ErrAcceptedSpec) {
					t.Fatalf("errors.Is(err, ErrAcceptedSpec) = false: %T %v", err, err)
				}
				var refusal *AcceptedSpecRefusal
				if !errors.As(err, &refusal) {
					t.Fatalf("errors.As(err, *AcceptedSpecRefusal) = false: %T %v", err, err)
				}
				if !IsRefusal(err) {
					t.Fatalf("IsRefusal(%T) = false", err)
				}
			case tt.wantOperational:
				if err == nil {
					t.Fatal("ResolveAcceptedSpec: want operational error, got nil")
				}
				if IsRefusal(err) {
					t.Fatalf("malformed authority was classified as a refusal: %T %v", err, err)
				}
				if resolveCalls != 0 {
					t.Fatalf("malformed authority reached StateResolver %d times, want 0", resolveCalls)
				}
			default:
				if err != nil {
					t.Fatalf("ResolveAcceptedSpec: %v", err)
				}
				if got.Path != tt.wantPath || got.Blob != blob || got.Commit != landing {
					t.Fatalf("ResolveAcceptedSpec = %+v, want path %q blob %q commit %q", got, tt.wantPath, blob, landing)
				}
			}
		})
	}
}

func TestResolveAcceptedSpec_BrokenPortContractsAreOperational(t *testing.T) {
	const (
		root = "/repo"
		head = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		path = ".verdi/specs/active/example-story/spec.md"
		blob = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	)

	tests := []struct {
		name    string
		entries []gitx.TreeEntry
		result  specstate.Result
	}{
		{
			name: "same spec in both zones",
			entries: []gitx.TreeEntry{
				{Mode: "100644", Type: "blob", Object: blob, Path: path},
				{Mode: "100644", Type: "blob", Object: blob, Path: ".verdi/specs/archive/example-story/spec.md"},
			},
		},
		{
			name:    "accepted result without baseline",
			entries: []gitx.TreeEntry{{Mode: "100644", Type: "blob", Object: blob, Path: path}},
			result:  specstate.Result{State: specstate.AcceptedPendingBuild, Relation: specstate.RelationExact},
		},
		{
			name:    "baseline blob differs from HEAD tree",
			entries: []gitx.TreeEntry{{Mode: "100644", Type: "blob", Object: blob, Path: path}},
			result:  specstate.Result{State: specstate.AcceptedPendingBuild, Relation: specstate.RelationExact, Baseline: &specstate.Baseline{Path: path, Blob: strings.Repeat("c", 40), LandingCommit: strings.Repeat("d", 40)}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			git := authorityGit{
				tree: func(context.Context, string, string) ([]gitx.TreeEntry, error) { return tt.entries, nil },
				show: func(context.Context, string, string, string) ([]byte, error) { return []byte(acceptedStoryBytes), nil },
			}
			states := authorityStateResolver{resolve: func(context.Context, string, specstate.Candidate) (specstate.Result, error) { return tt.result, nil }}

			_, err := ResolveAcceptedSpec(context.Background(), git, states, root, head, "spec/example-story")
			if err == nil {
				t.Fatal("ResolveAcceptedSpec: want operational error, got nil")
			}
			if IsRefusal(err) {
				t.Fatalf("broken port contract was classified as refusal: %T %v", err, err)
			}
		})
	}
}

func TestResolveAcceptedSpec_PortErrorsAreOperational(t *testing.T) {
	want := errors.New("git unavailable")
	git := authorityGit{
		tree: func(context.Context, string, string) ([]gitx.TreeEntry, error) { return nil, want },
		show: func(context.Context, string, string, string) ([]byte, error) { panic("unexpected Show call") },
	}
	states := authorityStateResolver{resolve: func(context.Context, string, specstate.Candidate) (specstate.Result, error) {
		panic("unexpected Resolve call")
	}}

	_, err := ResolveAcceptedSpec(context.Background(), git, states, "/repo", strings.Repeat("a", 40), "spec/example-story")
	if !errors.Is(err, want) {
		t.Fatalf("ResolveAcceptedSpec error = %v, want wrapping %v", err, want)
	}
	if IsRefusal(err) {
		t.Fatalf("git error classified as refusal: %T %v", err, err)
	}
}

func TestResolveExpectedRepository(t *testing.T) {
	facts := repositoryfacts.Facts{
		RemoteOrigin:  repositoryfacts.StringFact{Known: true, Value: "github.com/example/repo"},
		Branch:        repositoryfacts.StringFact{Known: true, Value: "main"},
		Head:          repositoryfacts.StringFact{Known: true, Value: strings.Repeat("a", 40)},
		DefaultBranch: repositoryfacts.DefaultBranchFact{Known: true, Name: "main", Ref: "origin/main", Head: strings.Repeat("a", 40)},
		Relationship:  repositoryfacts.RelationshipEqual,
		Dirty:         repositoryfacts.BoolFact{Known: true},
		Staged:        repositoryfacts.BoolFact{Known: true},
		Source:        repositoryfacts.SourceHead,
	}

	tests := []struct {
		name     string
		expected *Expected
		mutate   func(*repositoryfacts.Facts)
		wantErr  bool
	}{
		{name: "omitted expectation"},
		{name: "exact match", expected: &Expected{Branch: "main", Head: strings.Repeat("a", 40)}},
		{name: "branch mismatch", expected: &Expected{Branch: "feature", Head: strings.Repeat("a", 40)}, wantErr: true},
		{name: "HEAD mismatch", expected: &Expected{Branch: "main", Head: strings.Repeat("b", 40)}, wantErr: true},
		{name: "computed branch unknown", expected: &Expected{Branch: "main", Head: strings.Repeat("a", 40)}, mutate: func(f *repositoryfacts.Facts) { f.Branch = repositoryfacts.StringFact{} }, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFacts := facts
			if tt.mutate != nil {
				tt.mutate(&gotFacts)
			}
			err := ResolveExpectedRepository(tt.expected, gotFacts)
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("ResolveExpectedRepository: %v", err)
				}
				return
			}
			if !errors.Is(err, ErrExpectedRepositoryMismatch) {
				t.Fatalf("errors.Is(err, ErrExpectedRepositoryMismatch) = false: %T %v", err, err)
			}
			var refusal *ExpectedRepositoryMismatchRefusal
			if !errors.As(err, &refusal) || !IsRefusal(err) {
				t.Fatalf("expected typed refusal, got %T %v", err, err)
			}
		})
	}
}

func TestResolveAcceptedSpec_RejectsWrongRequestedRef(t *testing.T) {
	git := authorityGit{
		tree: func(context.Context, string, string) ([]gitx.TreeEntry, error) { panic("unexpected tree call") },
		show: func(context.Context, string, string, string) ([]byte, error) { panic("unexpected show call") },
	}
	states := authorityStateResolver{resolve: func(context.Context, string, specstate.Candidate) (specstate.Result, error) {
		panic("unexpected state call")
	}}
	_, err := ResolveAcceptedSpec(context.Background(), git, states, "/repo", strings.Repeat("a", 40), "adr/not-a-spec")
	if err == nil || IsRefusal(err) {
		t.Fatalf("malformed caller ref = %T %v, want ordinary operational/malformed error", err, err)
	}
}

func TestIsRefusal_ClosedFamilies(t *testing.T) {
	errs := []error{
		&NoConstitutionRefusal{},
		&AdapterMismatchRefusal{Requested: AdapterRef{ID: "missing", Version: "1"}},
		&AcceptedSpecRefusal{Ref: "spec/example", State: specstate.Proposed},
		&ExpectedRepositoryMismatchRefusal{Expected: Expected{Branch: "main", Head: strings.Repeat("a", 40)}},
		&DeclaredScopeRefusal{Phase: PhaseBuild},
		&ProjectionDriftRefusal{Paths: []string{"AGENTS.md"}},
		&PhaseScopeRefusal{Phase: PhaseBuild, ScopePhases: []string{"design"}},
	}
	for _, err := range errs {
		if !IsRefusal(fmt.Errorf("wrapped: %w", err)) {
			t.Errorf("IsRefusal(wrapped %T) = false", err)
		}
	}
	if IsRefusal(errors.New("ordinary")) {
		t.Error("IsRefusal(ordinary) = true")
	}
	if !errors.Is(&PhaseScopeRefusal{Phase: PhaseBuild, ScopePhases: []string{"design"}}, ErrDeclaredScope) {
		t.Error("errors.Is(PhaseScopeRefusal, ErrDeclaredScope) = false")
	}
}

func installPolicyFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := []string{
		"constitution.md",
		"policies/go-toolchain.md",
		"overlays/frontend-go-version.md",
		"exemptions/legacy-service-go.md",
		"profiles/solo-default.md",
	}
	for _, rel := range files {
		data, err := os.ReadFile(filepath.Join("..", "policyartifact", "testdata", "store", filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("read policy fixture %s: %v", rel, err)
		}
		dst := filepath.Join(root, ".verdi", "policy", filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatalf("mkdir policy fixture %s: %v", rel, err)
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			t.Fatalf("write policy fixture %s: %v", rel, err)
		}
	}
	return root
}

func TestResolvePolicyAuthority(t *testing.T) {
	root := installPolicyFixture(t)

	got, err := ResolvePolicyAuthority(defaultAuthorityLoader{}, root, AdapterRef{ID: "codex", Version: "1"})
	if err != nil {
		t.Fatalf("ResolvePolicyAuthority: %v", err)
	}
	if got.Store == nil || got.Effective == nil {
		t.Fatalf("ResolvePolicyAuthority = %+v, want loaded store and effective policy", got)
	}
	if got.Adapter.ID != "codex" || got.Adapter.Version != "1" {
		t.Fatalf("resolved adapter = %+v, want exact codex/1 constitution row", got.Adapter)
	}
	wantDigest, err := got.Effective.Digest()
	if err != nil {
		t.Fatalf("effective policy digest: %v", err)
	}
	if got.EffectiveDigest != wantDigest {
		t.Fatalf("effective digest = %q, want %q", got.EffectiveDigest, wantDigest)
	}
}

func TestResolvePolicyAuthority_Refusals(t *testing.T) {
	t.Run("constitution not adopted", func(t *testing.T) {
		_, err := ResolvePolicyAuthority(defaultAuthorityLoader{}, t.TempDir(), AdapterRef{ID: "codex", Version: "1"})
		if !errors.Is(err, ErrNoConstitution) {
			t.Fatalf("errors.Is(err, ErrNoConstitution) = false: %T %v", err, err)
		}
		var refusal *NoConstitutionRefusal
		if !errors.As(err, &refusal) || !IsRefusal(err) {
			t.Fatalf("no constitution = %T %v, want typed refusal", err, err)
		}
	})

	t.Run("adapter mismatch", func(t *testing.T) {
		root := installPolicyFixture(t)
		_, err := ResolvePolicyAuthority(defaultAuthorityLoader{}, root, AdapterRef{ID: "codex", Version: "2"})
		if !errors.Is(err, ErrAdapterMismatch) {
			t.Fatalf("errors.Is(err, ErrAdapterMismatch) = false: %T %v", err, err)
		}
		var refusal *AdapterMismatchRefusal
		if !errors.As(err, &refusal) || !IsRefusal(err) {
			t.Fatalf("adapter mismatch = %T %v, want typed refusal", err, err)
		}
		if len(refusal.Registered) != 1 || refusal.Registered[0] != (AdapterRef{ID: "codex", Version: "1"}) {
			t.Fatalf("registered adapters = %+v, want exact sorted codex/1 row", refusal.Registered)
		}
	})
}

type failingAuthorityLoader struct {
	loadErr    error
	resolveErr error
}

func (l failingAuthorityLoader) Load(string) (*policyauthority.Store, error) {
	if l.loadErr != nil {
		return nil, l.loadErr
	}
	return &policyauthority.Store{}, nil
}

func (l failingAuthorityLoader) Resolve(*policyauthority.Store) (*policyauthority.EffectivePolicy, error) {
	return nil, l.resolveErr
}

func TestResolvePolicyAuthority_OperationalErrorsRemainOperational(t *testing.T) {
	tests := []struct {
		name   string
		loader AuthorityLoader
	}{
		{name: "malformed load", loader: failingAuthorityLoader{loadErr: errors.New("malformed constitution")}},
		{name: "broken resolve", loader: failingAuthorityLoader{resolveErr: errors.New("effective policy invalid")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ResolvePolicyAuthority(tt.loader, "/repo", AdapterRef{ID: "codex", Version: "1"})
			if err == nil {
				t.Fatal("ResolvePolicyAuthority: want operational error, got nil")
			}
			if IsRefusal(err) {
				t.Fatalf("operational error classified as refusal: %T %v", err, err)
			}
		})
	}
}
