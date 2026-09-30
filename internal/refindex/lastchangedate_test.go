package refindex

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/specstate"
)

// oneDesignDraftGitRunner is a fake whose only entry is the ordinary
// design-branch draft on branch (no default branch resolves), dated by
// commitDatesFn.
func oneDesignDraftGitRunner(branch string, commitDatesFn func(context.Context, string, []string) (map[string]string, error)) *fakeGitRunner {
	return &fakeGitRunner{
		defaultBranchFn: func(context.Context, string) (string, error) { return "", nil },
		localDesignFn:   func(context.Context, string) ([]string, error) { return []string{branch}, nil },
		remoteDesignFn:  func(context.Context, string) ([]string, error) { return nil, nil },
		listTreeFn: func(ctx context.Context, dir, ref, path string) ([]string, error) {
			return []string{path}, nil
		},
		showFn: func(ctx context.Context, dir, ref, path string) ([]byte, error) {
			return []byte(fakeStatuslessStorySpec), nil
		},
		commitDatesFn: commitDatesFn,
	}
}

// resolverWith answers every candidate with r (its Baseline's Path set to
// the candidate's own).
func resolverWith(r specstate.Result) *fakeStateResolver {
	return &fakeStateResolver{resolveManyFn: func(_ context.Context, _ string, candidates []specstate.Candidate) ([]specstate.Result, error) {
		out := make([]specstate.Result, len(candidates))
		for i := range candidates {
			out[i] = r
			if r.Baseline != nil {
				b := *r.Baseline
				b.Path = candidates[i].Path
				out[i].Baseline = &b
			}
		}
		return out, nil
	}}
}

// datesAnswer is a CommitDates double answering exactly dates.
func datesAnswer(dates map[string]string) func(context.Context, string, []string) (map[string]string, error) {
	return func(context.Context, string, []string) (map[string]string, error) { return dates, nil }
}

const (
	landingSHA   = "deadbeef00000000000000000000000000000000"
	componentSHA = "cafef00d00000000000000000000000000000000"
	// oldBlanketDateText is the nil-Baseline text SI-297 retired: false
	// whenever specstate computed the landing commit but returned Unproven
	// for another reason (an incomplete corpus scan, a link-only
	// successor).
	oldBlanketDateText = "no landing commit could be proven for this entry's current bytes"
	scanIncompleteText = "specstate: .verdi/specs/active/fake-story/spec.md cannot be proven not-superseded — the default-branch active-spec scan is incomplete"
)

// TestComputeIndex_LastChangeDates is ac-1's static obligation: a
// design-branch entry's Date is its tip's committer date, a default-branch
// entry's Date (component or feature/story alike) is its resolved
// Baseline's landing-commit committer date, and every way that date can
// fail to be read — the batch read failing, the rev absent from its
// answer, an empty or unparsable answer (B1-R6), or no landing commit at
// all because the effective state is unproven (B1-R3, SI-297) — degrades to
// an empty Date plus a populated DateDisclosed, never a zero or
// current-date stand-in and never an operational error. Each walk asks for
// its dates in exactly one port call, and asks for exactly the tip or the
// landing commit — never the default branch's tip.
func TestComputeIndex_LastChangeDates(t *testing.T) {
	sentinel := errors.New("git cat-file failed")
	accepted := func(landing string) func() *fakeStateResolver {
		return func() *fakeStateResolver {
			return resolverWith(specstate.Result{State: specstate.AcceptedPendingBuild, Relation: specstate.RelationExact, Baseline: &specstate.Baseline{Blob: "b1", LandingCommit: landing}})
		}
	}
	tests := []struct {
		name     string
		git      func() *fakeGitRunner
		resolver func() *fakeStateResolver
		ref      string
		// wantDate is the entry's Date; "" means none.
		wantDate string
		// wantDisclosed lists substrings DateDisclosed.Text must carry; nil
		// means DateDisclosed must be nil.
		wantDisclosed []string
		// wantSource is DateDisclosed.Source when disclosed.
		wantSource string
		// wantBatches is every CommitDates call's revs, in call order.
		wantBatches [][]string
	}{
		{
			name: "design-branch entry: its tip's committer date",
			git: func() *fakeGitRunner {
				return oneDesignDraftGitRunner("design/gamma", datesAnswer(map[string]string{"design/gamma": "2024-03-01T00:00:00+00:00"}))
			},
			resolver:    proposedResolver,
			ref:         "spec/gamma",
			wantDate:    "2024-03-01T00:00:00+00:00",
			wantBatches: [][]string{{"design/gamma"}},
		},
		{
			name: "default-branch feature/story entry: its landing commit's committer date, never the branch tip's",
			git: func() *fakeGitRunner {
				f := defaultBranchOneSpecGitRunner(".verdi/specs/active/fake-story/spec.md", fakeStatuslessStorySpec)
				f.commitDatesFn = datesAnswer(map[string]string{landingSHA: "2024-02-14T00:00:00+00:00", "main": "2024-09-09T00:00:00+00:00", "HEAD": "2024-09-09T00:00:00+00:00"})
				return f
			},
			resolver:    accepted(landingSHA),
			ref:         "spec/fake-story",
			wantDate:    "2024-02-14T00:00:00+00:00",
			wantBatches: [][]string{{landingSHA}},
		},
		{
			name: "default-branch component entry: also its landing commit's committer date",
			git: func() *fakeGitRunner {
				f := defaultBranchOneSpecGitRunner(".verdi/specs/active/fake/spec.md", fakeComponentSpec)
				f.commitDatesFn = datesAnswer(map[string]string{componentSHA: "2024-05-05T00:00:00+00:00"})
				return f
			},
			resolver:    accepted(componentSHA),
			ref:         "spec/fake",
			wantDate:    "2024-05-05T00:00:00+00:00",
			wantBatches: [][]string{{componentSHA}},
		},
		{
			name: "the batch read fails: design-branch entry disclosed with the error, never zero or now",
			git: func() *fakeGitRunner {
				return oneDesignDraftGitRunner("design/gamma", func(context.Context, string, []string) (map[string]string, error) { return nil, sentinel })
			},
			resolver:      proposedResolver,
			ref:           "spec/gamma",
			wantDisclosed: []string{"last-change date unreadable", sentinel.Error()},
			wantSource:    "refindex:date-unreadable",
			wantBatches:   [][]string{{"design/gamma"}},
		},
		{
			name: "the batch read fails: default-branch entry disclosed with the error",
			git: func() *fakeGitRunner {
				f := defaultBranchOneSpecGitRunner(".verdi/specs/active/fake-story/spec.md", fakeStatuslessStorySpec)
				f.commitDatesFn = func(context.Context, string, []string) (map[string]string, error) { return nil, sentinel }
				return f
			},
			resolver:      accepted(landingSHA),
			ref:           "spec/fake-story",
			wantDisclosed: []string{"last-change date unreadable", sentinel.Error()},
			wantSource:    "refindex:date-unreadable",
			wantBatches:   [][]string{{landingSHA}},
		},
		{
			name: "the rev is absent from the answer: disclosed naming the rev",
			git: func() *fakeGitRunner {
				return oneDesignDraftGitRunner("design/gamma", datesAnswer(map[string]string{"design/other": "2024-03-01T00:00:00+00:00"}))
			},
			resolver:      proposedResolver,
			ref:           "spec/gamma",
			wantDisclosed: []string{"last-change date unreadable", "design/gamma"},
			wantSource:    "refindex:date-unreadable",
			wantBatches:   [][]string{{"design/gamma"}},
		},
		{
			name: `an empty answer ("", nil) is unreadable, never a readable empty date (B1-R6)`,
			git: func() *fakeGitRunner {
				return oneDesignDraftGitRunner("design/gamma", datesAnswer(map[string]string{"design/gamma": ""}))
			},
			resolver:      proposedResolver,
			ref:           "spec/gamma",
			wantDisclosed: []string{"last-change date unreadable", "design/gamma"},
			wantSource:    "refindex:date-unreadable",
			wantBatches:   [][]string{{"design/gamma"}},
		},
		{
			name: "an answer that is not a committer date is unreadable (B1-R6)",
			git: func() *fakeGitRunner {
				return oneDesignDraftGitRunner("design/gamma", datesAnswer(map[string]string{"design/gamma": "last tuesday"}))
			},
			resolver:      proposedResolver,
			ref:           "spec/gamma",
			wantDisclosed: []string{"last-change date unreadable", "last tuesday", "not a committer date"},
			wantSource:    "refindex:date-unreadable",
			wantBatches:   [][]string{{"design/gamma"}},
		},
		{
			name: "unproven effective state: disclosed as unproven with specstate's own disclosure, no date read (B1-R3)",
			git: func() *fakeGitRunner {
				return defaultBranchOneSpecGitRunner(".verdi/specs/active/fake-story/spec.md", fakeStatuslessStorySpec)
			},
			resolver: func() *fakeStateResolver {
				return resolverWith(specstate.Result{State: specstate.Unproven, Relation: specstate.RelationUnproven, Disclosures: []string{scanIncompleteText}})
			},
			ref:           "spec/fake-story",
			wantDisclosed: []string{"last-change date unproven", "effective lifecycle state is unproven", scanIncompleteText},
			wantSource:    "refindex:date-unproven",
			wantBatches:   nil,
		},
		{
			name: "unproven effective state with no specstate disclosure still names the unproven state",
			git: func() *fakeGitRunner {
				return defaultBranchOneSpecGitRunner(".verdi/specs/active/fake-story/spec.md", fakeStatuslessStorySpec)
			},
			resolver: func() *fakeStateResolver {
				return resolverWith(specstate.Result{State: specstate.Unproven, Relation: specstate.RelationUnproven})
			},
			ref:           "spec/fake-story",
			wantDisclosed: []string{"last-change date unproven", "effective lifecycle state is unproven"},
			wantSource:    "refindex:date-unproven",
			wantBatches:   nil,
		},
		{
			name: "a result carrying a baseline but no landing commit is disclosed naming its state (defensive)",
			git: func() *fakeGitRunner {
				return defaultBranchOneSpecGitRunner(".verdi/specs/active/fake-story/spec.md", fakeStatuslessStorySpec)
			},
			resolver: func() *fakeStateResolver {
				return resolverWith(specstate.Result{State: specstate.Proposed, Relation: specstate.RelationDiverged, Baseline: &specstate.Baseline{Blob: "b3"}})
			},
			ref:           "spec/fake-story",
			wantDisclosed: []string{"last-change date unproven", string(specstate.Proposed)},
			wantSource:    "refindex:date-unproven",
			wantBatches:   nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := tt.git()
			got, err := ComputeIndex(context.Background(), "/fake", f, tt.resolver())
			if err != nil {
				t.Fatalf("ComputeIndex: %v (an unreadable date must degrade the entry, never fail the render)", err)
			}
			e := entryByRef(t, got, tt.ref)
			if e.Date != tt.wantDate {
				t.Fatalf("Date = %q, want %q (never a zero or current-date stand-in)", e.Date, tt.wantDate)
			}
			if tt.wantDisclosed == nil {
				if e.DateDisclosed != nil {
					t.Fatalf("DateDisclosed = %+v, want nil for a readable date", e.DateDisclosed)
				}
			} else {
				if e.DateDisclosed == nil {
					t.Fatal("DateDisclosed = nil, want a populated disclosure")
				}
				for _, want := range tt.wantDisclosed {
					if !strings.Contains(e.DateDisclosed.Text, want) {
						t.Errorf("DateDisclosed.Text = %q, want it to carry %q", e.DateDisclosed.Text, want)
					}
				}
				if strings.Contains(e.DateDisclosed.Text, oldBlanketDateText) {
					t.Errorf("DateDisclosed.Text = %q carries the blanket %q claim SI-297 retired", e.DateDisclosed.Text, oldBlanketDateText)
				}
				if e.DateDisclosed.Source != tt.wantSource {
					t.Errorf("DateDisclosed.Source = %q, want %q", e.DateDisclosed.Source, tt.wantSource)
				}
				if e.DateDisclosed.Scope != tt.ref {
					t.Errorf("DateDisclosed.Scope = %q, want the entry's own ref %q", e.DateDisclosed.Scope, tt.ref)
				}
			}
			if !reflect.DeepEqual(f.commitDatesCalls, tt.wantBatches) {
				t.Fatalf("CommitDates calls = %q, want %q (one call per walk, asking exactly the tip or the landing commit)", f.commitDatesCalls, tt.wantBatches)
			}
		})
	}
}

// TestFakePort_DatesEveryEntry is ac-3's static obligation: the fake port
// returns a deterministic date for every fixture entry ComputeIndex
// produces — an ordinary local design-branch draft, an ordinary
// remote-tracking design-branch draft, a degraded (no-draft-spec)
// design-branch entry, and two default-branch entries sharing one landing
// commit — proven from ONE ComputeIndex call over ONE fake fixture,
// exercising every call site that sets Entry.Date. Each walk's dates come
// from exactly one port call, deduplicated and sorted.
func TestFakePort_DatesEveryEntry(t *testing.T) {
	const mainLandingSHA = "main-landing-sha"
	dates := map[string]string{
		"design/alpha":       "2024-01-01T00:00:00+00:00",
		"origin/design/beta": "2024-01-02T00:00:00+00:00",
		"design/gamma-empty": "2024-01-03T00:00:00+00:00",
		mainLandingSHA:       "2024-01-04T00:00:00+00:00",
	}
	wantDates := map[string]string{
		"spec/alpha":       "2024-01-01T00:00:00+00:00",
		"spec/beta":        "2024-01-02T00:00:00+00:00",
		"spec/gamma-empty": "2024-01-03T00:00:00+00:00",
		"spec/fake":        "2024-01-04T00:00:00+00:00",
		"spec/fake-too":    "2024-01-04T00:00:00+00:00",
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
				return []string{".verdi/specs/active/fake/spec.md", ".verdi/specs/active/fake-too/spec.md"}, nil
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
		commitDatesFn: func(ctx context.Context, dir string, revs []string) (map[string]string, error) {
			out := map[string]string{}
			for _, rev := range revs {
				date, ok := dates[rev]
				if !ok {
					t.Fatalf("CommitDates asked for unexpected rev %q", rev)
				}
				out[rev] = date
			}
			return out, nil
		},
	}

	resolver := &fakeStateResolver{resolveManyFn: func(_ context.Context, _ string, candidates []specstate.Candidate) ([]specstate.Result, error) {
		out := make([]specstate.Result, len(candidates))
		for i, c := range candidates {
			if strings.HasPrefix(c.Path, ".verdi/specs/active/fake") && len(f.commitDatesCalls) == 0 {
				// The default-branch walk's candidates (resolved before any
				// date read): both landed in one commit.
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
	if len(got) != len(wantDates) {
		t.Fatalf("ComputeIndex returned %d entries (%v), want %d", len(got), refs(got), len(wantDates))
	}
	for _, e := range got {
		want, ok := wantDates[e.Ref]
		if !ok {
			t.Errorf("unexpected entry %s", e.Ref)
			continue
		}
		if e.Date != want {
			t.Errorf("%s: Date = %q, want the deterministic fixture date %q", e.Ref, e.Date, want)
		}
		if e.DateDisclosed != nil {
			t.Errorf("%s: DateDisclosed = %+v, want nil (every fixture entry in this test has a known date)", e.Ref, e.DateDisclosed)
		}
	}
	wantBatches := [][]string{
		{mainLandingSHA}, // the default-branch walk: two entries, one shared landing commit, asked once
		{"design/alpha", "design/gamma-empty", "origin/design/beta"}, // the design-branch walk, sorted
	}
	if !reflect.DeepEqual(f.commitDatesCalls, wantBatches) {
		t.Fatalf("CommitDates calls = %q, want %q (exactly one call per walk)", f.commitDatesCalls, wantBatches)
	}
}
