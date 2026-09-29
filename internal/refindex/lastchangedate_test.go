package refindex

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/specstate"
)

// TestComputeIndex_LastChangeDates is ac-1's static obligation: a
// design-branch entry's Date is its tip's committer date, a default-branch
// entry's Date (component or feature/story alike) is its resolved
// Baseline's landing-commit committer date, and either an unreadable tip
// date, an unreadable landing commit, or no provable landing commit at all
// degrades to an empty Date plus a populated DateDisclosed — never a zero
// or current-date stand-in.
func TestComputeIndex_LastChangeDates(t *testing.T) {
	t.Run("design-branch entry: tip committer date", func(t *testing.T) {
		const wantDate = "2024-03-01T00:00:00+00:00"
		local, remote := []string{"design/gamma"}, []string(nil)
		f := &fakeGitRunner{
			defaultBranchFn: func(context.Context, string) (string, error) { return "", nil },
			localDesignFn:   func(context.Context, string) ([]string, error) { return local, nil },
			remoteDesignFn:  func(context.Context, string) ([]string, error) { return remote, nil },
			listTreeFn: func(ctx context.Context, dir, ref, path string) ([]string, error) {
				return []string{path}, nil
			},
			showFn: func(ctx context.Context, dir, ref, path string) ([]byte, error) {
				return []byte(fakeStatuslessStorySpec), nil
			},
			commitDateFn: func(ctx context.Context, dir, rev string) (string, error) {
				if rev != "design/gamma" {
					t.Fatalf("CommitDate called with rev %q, want the branch's own tip revision %q", rev, "design/gamma")
				}
				return wantDate, nil
			},
		}
		got, err := ComputeIndex(context.Background(), "/fake", f, proposedResolver())
		if err != nil {
			t.Fatalf("ComputeIndex: %v", err)
		}
		e := entryByRef(t, got, "spec/gamma")
		if e.Date != wantDate {
			t.Fatalf("Date = %q, want %q", e.Date, wantDate)
		}
		if e.DateDisclosed != nil {
			t.Fatalf("DateDisclosed = %+v, want nil for a readable tip date", e.DateDisclosed)
		}
	})

	t.Run("default-branch feature/story entry: landing-commit committer date", func(t *testing.T) {
		const wantDate = "2024-02-14T00:00:00+00:00"
		const landingSHA = "deadbeef00000000000000000000000000000000"
		f := defaultBranchOneSpecGitRunner(".verdi/specs/active/fake-story/spec.md", fakeStatuslessStorySpec)
		f.commitDateFn = func(ctx context.Context, dir, rev string) (string, error) {
			if rev != landingSHA {
				t.Fatalf("CommitDate called with rev %q, want the resolved landing commit %q", rev, landingSHA)
			}
			return wantDate, nil
		}
		resolver := &fakeStateResolver{resolveManyFn: func(_ context.Context, _ string, candidates []specstate.Candidate) ([]specstate.Result, error) {
			out := make([]specstate.Result, len(candidates))
			for i := range candidates {
				out[i] = specstate.Result{
					State:    specstate.AcceptedPendingBuild,
					Relation: specstate.RelationExact,
					Baseline: &specstate.Baseline{Path: candidates[i].Path, Blob: "blob1", LandingCommit: landingSHA},
				}
			}
			return out, nil
		}}

		got, err := ComputeIndex(context.Background(), "/fake", f, resolver)
		if err != nil {
			t.Fatalf("ComputeIndex: %v", err)
		}
		e := entryByRef(t, got, "spec/fake-story")
		if e.Date != wantDate {
			t.Fatalf("Date = %q, want %q", e.Date, wantDate)
		}
		if e.DateDisclosed != nil {
			t.Fatalf("DateDisclosed = %+v, want nil for a readable landing commit", e.DateDisclosed)
		}
	})

	t.Run("default-branch component entry: also gets a landing-commit date", func(t *testing.T) {
		const wantDate = "2024-05-05T00:00:00+00:00"
		const landingSHA = "cafef00d00000000000000000000000000000000"
		f := defaultBranchOneSpecGitRunner(".verdi/specs/active/fake/spec.md", fakeComponentSpec)
		f.commitDateFn = func(ctx context.Context, dir, rev string) (string, error) {
			if rev != landingSHA {
				t.Fatalf("CommitDate called with rev %q, want the resolved landing commit %q", rev, landingSHA)
			}
			return wantDate, nil
		}
		resolver := &fakeStateResolver{resolveManyFn: func(_ context.Context, _ string, candidates []specstate.Candidate) ([]specstate.Result, error) {
			out := make([]specstate.Result, len(candidates))
			for i := range candidates {
				out[i] = specstate.Result{
					State:    specstate.AcceptedPendingBuild,
					Relation: specstate.RelationExact,
					Baseline: &specstate.Baseline{Path: candidates[i].Path, Blob: "blob2", LandingCommit: landingSHA},
				}
			}
			return out, nil
		}}

		got, err := ComputeIndex(context.Background(), "/fake", f, resolver)
		if err != nil {
			t.Fatalf("ComputeIndex: %v", err)
		}
		e := entryByRef(t, got, "spec/fake")
		// The component's StatusGroup/SpecStatus still come from raw
		// frontmatter (mapStatusGroup), never the resolver's State — proven
		// unchanged alongside the new Date behavior.
		if e.StatusGroup != StatusGroupActiveComponents {
			t.Fatalf("StatusGroup = %q, want %q (mapStatusGroup from raw frontmatter, unaffected by the resolver batch)", e.StatusGroup, StatusGroupActiveComponents)
		}
		if e.Date != wantDate {
			t.Fatalf("Date = %q, want %q", e.Date, wantDate)
		}
		if e.DateDisclosed != nil {
			t.Fatalf("DateDisclosed = %+v, want nil for a readable landing commit", e.DateDisclosed)
		}
	})

	t.Run("unreadable tip date: design-branch entry disclosed, never zero or now", func(t *testing.T) {
		sentinel := errors.New("git log failed")
		f := &fakeGitRunner{
			defaultBranchFn: func(context.Context, string) (string, error) { return "", nil },
			localDesignFn:   func(context.Context, string) ([]string, error) { return []string{"design/gamma"}, nil },
			remoteDesignFn:  func(context.Context, string) ([]string, error) { return nil, nil },
			listTreeFn: func(ctx context.Context, dir, ref, path string) ([]string, error) {
				return []string{path}, nil
			},
			showFn: func(ctx context.Context, dir, ref, path string) ([]byte, error) {
				return []byte(fakeStatuslessStorySpec), nil
			},
			commitDateFn: func(ctx context.Context, dir, rev string) (string, error) {
				return "", sentinel
			},
		}
		got, err := ComputeIndex(context.Background(), "/fake", f, proposedResolver())
		if err != nil {
			t.Fatalf("ComputeIndex: unexpected error (an unreadable date must degrade the entry, never fail the whole render): %v", err)
		}
		e := entryByRef(t, got, "spec/gamma")
		if e.Date != "" {
			t.Fatalf("Date = %q, want empty (never a zero or current-date stand-in)", e.Date)
		}
		if e.DateDisclosed == nil {
			t.Fatal("DateDisclosed = nil, want a populated disclosure naming the unreadable tip date")
		}
		if !strings.Contains(e.DateDisclosed.Text, sentinel.Error()) {
			t.Fatalf("DateDisclosed.Text = %q, want it to carry the underlying error %q", e.DateDisclosed.Text, sentinel.Error())
		}
	})

	t.Run("unreadable landing commit: default-branch entry disclosed, never zero or now", func(t *testing.T) {
		sentinel := errors.New("git log failed")
		const landingSHA = "deadbeef00000000000000000000000000000000"
		f := defaultBranchOneSpecGitRunner(".verdi/specs/active/fake-story/spec.md", fakeStatuslessStorySpec)
		f.commitDateFn = func(ctx context.Context, dir, rev string) (string, error) { return "", sentinel }
		resolver := &fakeStateResolver{resolveManyFn: func(_ context.Context, _ string, candidates []specstate.Candidate) ([]specstate.Result, error) {
			out := make([]specstate.Result, len(candidates))
			for i := range candidates {
				out[i] = specstate.Result{
					State:    specstate.AcceptedPendingBuild,
					Relation: specstate.RelationExact,
					Baseline: &specstate.Baseline{Path: candidates[i].Path, Blob: "blob1", LandingCommit: landingSHA},
				}
			}
			return out, nil
		}}

		got, err := ComputeIndex(context.Background(), "/fake", f, resolver)
		if err != nil {
			t.Fatalf("ComputeIndex: unexpected error: %v", err)
		}
		e := entryByRef(t, got, "spec/fake-story")
		if e.Date != "" {
			t.Fatalf("Date = %q, want empty", e.Date)
		}
		if e.DateDisclosed == nil {
			t.Fatal("DateDisclosed = nil, want a populated disclosure naming the unreadable landing commit")
		}
		if !strings.Contains(e.DateDisclosed.Text, sentinel.Error()) {
			t.Fatalf("DateDisclosed.Text = %q, want it to carry the underlying error %q", e.DateDisclosed.Text, sentinel.Error())
		}
	})

	t.Run("no provable landing commit: default-branch entry disclosed, CommitDate never called", func(t *testing.T) {
		f := defaultBranchOneSpecGitRunner(".verdi/specs/active/fake-story/spec.md", fakeStatuslessStorySpec)
		f.commitDateFn = func(ctx context.Context, dir, rev string) (string, error) {
			t.Fatalf("CommitDate called with rev %q, want it never called when no landing commit was proven", rev)
			return "", nil
		}
		resolver := &fakeStateResolver{resolveManyFn: func(_ context.Context, _ string, candidates []specstate.Candidate) ([]specstate.Result, error) {
			out := make([]specstate.Result, len(candidates))
			for i := range candidates {
				out[i] = specstate.Result{
					State:       specstate.Unproven,
					Relation:    specstate.RelationUnproven,
					Disclosures: []string{"specstate: no first-parent landing witness"},
				}
			}
			return out, nil
		}}

		got, err := ComputeIndex(context.Background(), "/fake", f, resolver)
		if err != nil {
			t.Fatalf("ComputeIndex: unexpected error: %v", err)
		}
		e := entryByRef(t, got, "spec/fake-story")
		if e.Date != "" {
			t.Fatalf("Date = %q, want empty", e.Date)
		}
		if e.DateDisclosed == nil {
			t.Fatal("DateDisclosed = nil, want a populated disclosure — do not invent a fallback date")
		}
	})
}

// TestFakePort_DatesEveryEntry is ac-3's static obligation: the fake port
// returns a deterministic date for every fixture entry ComputeIndex
// produces — an ordinary local design-branch draft, an ordinary
// remote-tracking design-branch draft, a degraded (no-draft-spec)
// design-branch entry, and a default-branch component — proven from ONE
// ComputeIndex call over ONE fake fixture, exercising every call site that
// sets Entry.Date.
func TestFakePort_DatesEveryEntry(t *testing.T) {
	const mainLandingSHA = "main-landing-sha"
	dates := map[string]string{
		"design/alpha":       "2024-01-01T00:00:00+00:00",
		"origin/design/beta": "2024-01-02T00:00:00+00:00",
		"design/gamma-empty": "2024-01-03T00:00:00+00:00",
		mainLandingSHA:       "2024-01-04T00:00:00+00:00",
	}

	f := &fakeGitRunner{
		defaultBranchFn: func(context.Context, string) (string, error) { return "main", nil },
		localDesignFn: func(context.Context, string) ([]string, error) {
			return []string{"design/alpha", "design/gamma-empty"}, nil
		},
		remoteDesignFn: func(context.Context, string) ([]string, error) {
			return []string{"design/beta"}, nil
		},
		listTreeFn: func(ctx context.Context, dir, ref, path string) ([]string, error) {
			switch {
			case ref == "main" && path == ".verdi/specs/active":
				return []string{".verdi/specs/active/fake/spec.md"}, nil
			case ref == "main" && path == ".verdi/specs/archive":
				return nil, nil
			case ref == "design/alpha", ref == "origin/design/beta":
				return []string{path}, nil // ordinary: the probed spec.md exists
			case ref == "design/gamma-empty":
				return nil, nil // degraded: no spec.md was ever committed
			default:
				t.Fatalf("unexpected ListTree(ref=%q, path=%q)", ref, path)
				return nil, nil
			}
		},
		showFn: func(ctx context.Context, dir, ref, path string) ([]byte, error) {
			switch ref {
			case "main":
				return []byte(fakeComponentSpec), nil
			case "design/alpha", "origin/design/beta":
				return []byte(fakeStatuslessStorySpec), nil
			default:
				t.Fatalf("unexpected Show(ref=%q, path=%q)", ref, path)
				return nil, nil
			}
		},
		isAncestorFn: func(context.Context, string, string, string) (bool, error) {
			return false, nil // neither ordinary design branch is merged yet
		},
		commitDateFn: func(ctx context.Context, dir, rev string) (string, error) {
			date, ok := dates[rev]
			if !ok {
				t.Fatalf("CommitDate called with unexpected rev %q", rev)
			}
			return date, nil
		},
	}

	resolver := &fakeStateResolver{resolveManyFn: func(_ context.Context, _ string, candidates []specstate.Candidate) ([]specstate.Result, error) {
		out := make([]specstate.Result, len(candidates))
		for i, c := range candidates {
			if c.Path == ".verdi/specs/active/fake/spec.md" {
				out[i] = specstate.Result{
					State:    specstate.AcceptedPendingBuild,
					Relation: specstate.RelationExact,
					Baseline: &specstate.Baseline{Path: c.Path, Blob: "blob", LandingCommit: mainLandingSHA},
				}
				continue
			}
			out[i] = specstate.Result{State: specstate.Proposed, Relation: specstate.RelationNew}
		}
		return out, nil
	}}

	got, err := ComputeIndex(context.Background(), "/fake", f, resolver)
	if err != nil {
		t.Fatalf("ComputeIndex: %v", err)
	}
	wantRefs := []string{"spec/alpha", "spec/beta", "spec/fake", "spec/gamma-empty"}
	if gotRefs := refs(got); !equalStrings(gotRefs, wantRefs) {
		t.Fatalf("ComputeIndex refs = %v, want %v", gotRefs, wantRefs)
	}
	for _, e := range got {
		if e.Date == "" {
			t.Errorf("%s: Date is empty, want a deterministic fixture date", e.Ref)
		}
		if e.DateDisclosed != nil {
			t.Errorf("%s: DateDisclosed = %+v, want nil (every fixture entry in this test has a known date)", e.Ref, e.DateDisclosed)
		}
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
