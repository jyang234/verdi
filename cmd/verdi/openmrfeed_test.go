package main

import (
	"context"
	"fmt"
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
		Index:     func(context.Context) ([]refindex.Entry, error) { return entries, nil },
		OpenMRs:   httpOpenMRFeed{url: srv.URL},
		ForgeKind: workbench.ForgeGitLab,
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
	if !strings.Contains(up, `<span class="badge badge-open dir-inreview">MR !9 open</span>`) {
		t.Fatalf("feed up: the chip must name the merge request's number in GitLab notation; got: %s", up)
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

// TestDirectoryHome_Integration_ForgeAdapter is the live adapter's path to
// the chip: the real forgeOpenMRs over the hermetic forge fake, on a
// GitHub store, names the lowest open pull request from the draft's
// branch and counts the rest; an open pull request the forge lists with no
// number still chips its draft and discloses the number.
func TestDirectoryHome_Integration_ForgeAdapter(t *testing.T) {
	t.Parallel()
	f := fake.New()
	f.SeedOpenMR("main", forge.OpenMR{ID: "31", SourceBranch: "design/mr-draft", Title: "MR draft"})
	f.SeedOpenMR("main", forge.OpenMR{ID: "4", SourceBranch: "design/mr-draft", Title: "MR draft, earlier"})
	f.SeedOpenMR("main", forge.OpenMR{SourceBranch: "design/unnumbered", Title: "Unnumbered"})
	entries := []refindex.Entry{
		{Ref: "spec/mr-draft", Source: refindex.SourceBoth, StatusGroup: refindex.StatusGroupDraftsInProgress, SpecStatus: "draft"},
		{Ref: "spec/unnumbered", Source: refindex.SourceLocal, StatusGroup: refindex.StatusGroupDraftsInProgress, SpecStatus: "draft"},
	}
	h := workbench.NewHandlerWithHome(t.TempDir(), workbench.Deps{}, workbench.HomeDeps{
		Index:     func(context.Context) ([]refindex.Entry, error) { return entries, nil },
		OpenMRs:   newForgeOpenMRs(f, resolvableDefaultBranchRoot(t)),
		ForgeKind: workbench.ForgeGitHub,
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `<span class="badge badge-open dir-inreview">PR #4 open · +1</span>`) {
		t.Fatalf("want the lowest pull request named and the other counted; got: %s", body)
	}
	if !strings.Contains(body, `">in review · number unavailable</span>`) {
		t.Fatalf("want the unnumbered pull request's draft chipped with its number disclosed; got: %s", body)
	}
	if got := strings.Count(body, "dir-inreview"); got != 2 {
		t.Fatalf("in-review chips = %d, want 2 (one per draft with an open pull request)", got)
	}
}

// TestHomeOpenMRs is serve.go's in-review wiring (spec/directory-home dc-4,
// in the review feed's precedence order) and the forge kind it hands the
// chip: the live forge, else the harness feed, else — a forge configured
// but unreachable — the always-erroring lister, else nothing. The kind is
// always the configured one, as forgeBestEffort resolved it.
func TestHomeOpenMRs(t *testing.T) {
	t.Parallel()
	live := fake.New()
	tests := []struct {
		name       string
		port       forge.Forge
		configured string
		feedURL    string
		wantType   string
		wantKind   workbench.ForgeKind
	}{
		{"a live GitHub forge", live, "github", "", "*main.forgeOpenMRs", workbench.ForgeGitHub},
		{"a live GitLab forge wins over the harness feed", live, "gitlab", "http://127.0.0.1:9/openmrs", "*main.forgeOpenMRs", workbench.ForgeGitLab},
		{"the harness feed on a GitLab store", nil, "gitlab", "http://127.0.0.1:9/openmrs", "main.httpOpenMRFeed", workbench.ForgeGitLab},
		{"the harness feed with no forge configured: no kind", nil, "", "http://127.0.0.1:9/openmrs", "main.httpOpenMRFeed", ""},
		{"a configured, unreachable GitHub forge", nil, "github", "", "main.unavailableOpenMRs", workbench.ForgeGitHub},
		{"no forge configured: no lister", nil, "", "", "<nil>", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lister, kind := homeOpenMRs(tt.port, tt.configured, t.TempDir(), tt.feedURL)
			if got := fmt.Sprintf("%T", lister); got != tt.wantType {
				t.Fatalf("lister = %s, want %s", got, tt.wantType)
			}
			if tt.wantType == "<nil>" && lister != nil {
				t.Fatalf("no forge configured must leave HomeDeps.OpenMRs nil (the silent absence), got %#v", lister)
			}
			if kind != tt.wantKind {
				t.Fatalf("kind = %q, want %q", kind, tt.wantKind)
			}
		})
	}
}
