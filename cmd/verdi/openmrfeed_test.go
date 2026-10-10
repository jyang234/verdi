package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/forge"
	"github.com/jyang234/verdi/internal/forge/fake"
	"github.com/jyang234/verdi/internal/refindex"
	"github.com/jyang234/verdi/internal/workbench"
)

// resolvableDefaultBranchRoot builds a minimal fixturegit repo (local
// branch "main", no remote) and pins refs/remotes/origin/HEAD at it
// (pinFixtureDefaultBranch) so the default branch resolves to a real,
// git-resolvable ref (internal/specstate.ResolveDefaultBranch — the
// resolver internal/lint.ResolveDefaultBranch now delegates to — requires
// the named branch to actually resolve, not just be named; a bare
// t.TempDir() is not a git repository at all and no longer resolves)
// without any caller needing its own t.Setenv("CI_DEFAULT_BRANCH", ...).
// Shared by this file, reviewfeed_test.go, and supersessionfeed_test.go
// (all package main, all needing the same hermetic "give me a resolvable
// default branch" root).
func resolvableDefaultBranchRoot(t *testing.T) string {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"seed.txt": "seed\n"}, Message: "seed"},
	})
	pinFixtureDefaultBranch(t, repo.Dir)
	return repo.Dir
}

// TestForgeOpenMRs_ListsRefs drives the real adapter over the hermetic
// forge fake: every open MR targeting the resolved default branch
// contributes its source branch and its forge-native id (forge.OpenMR.ID,
// read as is — an empty one stays empty, never invented), sorted by branch
// then id.
func TestForgeOpenMRs_ListsRefs(t *testing.T) {
	t.Parallel()
	f := fake.New()
	f.SeedOpenMR("main", forge.OpenMR{ID: "2", SourceBranch: "design/zeta", Title: "Zeta"})
	f.SeedOpenMR("main", forge.OpenMR{ID: "1", SourceBranch: "design/alpha", Title: "Alpha"})
	f.SeedOpenMR("main", forge.OpenMR{SourceBranch: "design/beta", Title: "Beta, unnumbered"})
	f.SeedOpenMR("main", forge.OpenMR{ID: "9", SourceBranch: "design/zeta", Title: "Zeta again"})
	f.SeedOpenMR("release", forge.OpenMR{ID: "5", SourceBranch: "design/other", Title: "Not the default branch"})

	got, err := newForgeOpenMRs(f, resolvableDefaultBranchRoot(t)).OpenMRRefs(context.Background())
	if err != nil {
		t.Fatalf("OpenMRRefs: %v", err)
	}
	want := []workbench.OpenMRRef{
		{Branch: "design/alpha", ID: "1"},
		{Branch: "design/beta", ID: ""},
		{Branch: "design/zeta", ID: "2"},
		{Branch: "design/zeta", ID: "9"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("refs = %+v, want %+v", got, want)
	}
}

// TestForgeOpenMRs_UnresolvableDefaultBranch fails loud, not silent: with
// no default branch resolvable there is no target to list MRs against.
func TestForgeOpenMRs_UnresolvableDefaultBranch(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "")
	_, err := newForgeOpenMRs(fake.New(), t.TempDir()).OpenMRRefs(context.Background())
	if err == nil {
		t.Fatal("want an error when the default branch cannot be resolved, got nil")
	}
}

// TestHTTPOpenMRFeed_Table drives the harness double's strict decode:
// happy path, unknown fields, trailing data, non-200, unreachable.
func TestHTTPOpenMRFeed_Table(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		body    string
		status  int
		want    []workbench.OpenMRRef
		wantErr bool
	}{
		{"happy", `[{"id":"7","source_branch":"design/x","title":"X"}]`, http.StatusOK, []workbench.OpenMRRef{{Branch: "design/x", ID: "7"}}, false},
		{"several, sorted by branch then id", `[{"id":"7","source_branch":"design/y","title":"Y"},{"id":"3","source_branch":"design/x","title":"X"},{"id":"1","source_branch":"design/y","title":"Y2"}]`, http.StatusOK, []workbench.OpenMRRef{{Branch: "design/x", ID: "3"}, {Branch: "design/y", ID: "1"}, {Branch: "design/y", ID: "7"}}, false},
		{"an empty id stays empty", `[{"id":"","source_branch":"design/x","title":"X"}]`, http.StatusOK, []workbench.OpenMRRef{{Branch: "design/x"}}, false},
		{"empty feed", `[]`, http.StatusOK, []workbench.OpenMRRef{}, false},
		{"unknown field fails closed", `[{"id":"7","source_branch":"design/x","title":"X","extra":1}]`, http.StatusOK, nil, true},
		{"trailing data rejected", `[] {"more":true}`, http.StatusOK, nil, true},
		{"non-200 is an error", `outage`, http.StatusServiceUnavailable, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			got, err := httpOpenMRFeed{url: srv.URL}.OpenMRRefs(context.Background())
			if tt.wantErr {
				if err == nil {
					t.Fatalf("want error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("OpenMRRefs: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("refs = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestHTTPOpenMRFeed_Unreachable: a closed server errors — the shape the
// home page degrades to its disclosed notice.
func TestHTTPOpenMRFeed_Unreachable(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()
	if _, err := (httpOpenMRFeed{url: url}).OpenMRRefs(context.Background()); err == nil {
		t.Fatal("want error against a closed server, got nil")
	}
}

// TestUnavailableOpenMRs always errors with the disclosed reason.
func TestUnavailableOpenMRs(t *testing.T) {
	t.Parallel()
	_, err := unavailableOpenMRs{reason: "forge \"gitlab\" is configured but unreachable"}.OpenMRRefs(context.Background())
	if err == nil || !strings.Contains(err.Error(), "unreachable") {
		t.Fatalf("err = %v, want the disclosed reason", err)
	}
}

// TestDirectoryHome_Integration_HTTPFeed is spec/directory-home ac-2's Go
// integration witness over the httptest double (co-2: hermetic, loopback
// only): the SAME home surface renders the in-review chip while the feed
// is up, and the disclosed "MR status unavailable" notice — with the
// refs-computed directory still complete — after the double goes away.
func TestDirectoryHome_Integration_HTTPFeed(t *testing.T) {
	t.Parallel()
	entries := []refindex.Entry{
		{Ref: "spec/mr-draft", Source: refindex.SourceBoth, StatusGroup: refindex.StatusGroupDraftsInProgress, SpecStatus: "draft"},
		{Ref: "spec/quiet-draft", Source: refindex.SourceLocal, StatusGroup: refindex.StatusGroupDraftsInProgress, SpecStatus: "draft"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"9","source_branch":"design/mr-draft","title":"MR draft"}]`))
	}))

	h := workbench.NewHandlerWithHome(t.TempDir(), workbench.Deps{}, workbench.HomeDeps{
		Index:   func(context.Context) ([]refindex.Entry, error) { return entries, nil },
		OpenMRs: httpOpenMRFeed{url: srv.URL},
	})

	get := func() string {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET / status = %d, want 200", rec.Code)
		}
		return rec.Body.String()
	}

	up := get()
	if !strings.Contains(up, "dir-inreview") || strings.Count(up, "dir-inreview") != 1 {
		t.Fatalf("feed up: want exactly one in-review chip; got: %s", up)
	}
	if !strings.Contains(up, `data-testid="dir-entry-mr-draft"`) {
		t.Fatalf("feed up: missing the chipped entry; got: %s", up)
	}

	srv.Close() // the forge double becomes unreachable

	down := get()
	if !strings.Contains(down, "MR status unavailable") {
		t.Fatalf("feed down: missing the disclosed notice; got: %s", down)
	}
	if strings.Contains(down, "dir-inreview") {
		t.Fatalf("feed down: must not fabricate in-review chips")
	}
	for _, name := range []string{"mr-draft", "quiet-draft"} {
		if !strings.Contains(down, `data-testid="dir-entry-`+name+`"`) {
			t.Fatalf("feed down: entry %s missing — the refs-computed directory must still render fully", name)
		}
	}
}
