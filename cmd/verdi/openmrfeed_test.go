package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/forge"
	"github.com/jyang234/verdi/internal/forge/fake"
	"github.com/jyang234/verdi/internal/refindex"
	"github.com/jyang234/verdi/internal/workbench"
)

// TestForgeOpenMRs_ListsSourceBranches drives the real adapter over the
// hermetic forge fake: every open MR targeting the resolved default branch
// contributes its source branch, sorted.
func TestForgeOpenMRs_ListsSourceBranches(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main") // hermetic default-branch resolution (no git remote)

	f := fake.New()
	f.SeedOpenMR("main", forge.OpenMR{ID: "2", SourceBranch: "design/zeta", Title: "Zeta"})
	f.SeedOpenMR("main", forge.OpenMR{ID: "1", SourceBranch: "design/alpha", Title: "Alpha"})

	got, err := newForgeOpenMRs(f, t.TempDir()).OpenMRSourceBranches(context.Background())
	if err != nil {
		t.Fatalf("OpenMRSourceBranches: %v", err)
	}
	want := []string{"design/alpha", "design/zeta"}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("branches = %v, want %v", got, want)
	}
}

// TestForgeOpenMRs_UnresolvableDefaultBranch fails loud, not silent: with
// no default branch resolvable there is no target to list MRs against.
func TestForgeOpenMRs_UnresolvableDefaultBranch(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "")
	_, err := newForgeOpenMRs(fake.New(), t.TempDir()).OpenMRSourceBranches(context.Background())
	if err == nil {
		t.Fatal("want an error when the default branch cannot be resolved, got nil")
	}
}

// TestHTTPOpenMRFeed_Table drives the harness double's strict decode:
// happy path, unknown fields, trailing data, non-200, unreachable.
func TestHTTPOpenMRFeed_Table(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		status  int
		want    []string
		wantErr bool
	}{
		{"happy", `[{"id":"7","source_branch":"design/x","title":"X"}]`, http.StatusOK, []string{"design/x"}, false},
		{"empty feed", `[]`, http.StatusOK, nil, false},
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

			got, err := httpOpenMRFeed{url: srv.URL}.OpenMRSourceBranches(context.Background())
			if tt.wantErr {
				if err == nil {
					t.Fatalf("want error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("OpenMRSourceBranches: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("branches = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("branches = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

// TestHTTPOpenMRFeed_Unreachable: a closed server errors — the shape the
// home page degrades to its disclosed notice.
func TestHTTPOpenMRFeed_Unreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()
	if _, err := (httpOpenMRFeed{url: url}).OpenMRSourceBranches(context.Background()); err == nil {
		t.Fatal("want error against a closed server, got nil")
	}
}

// TestUnavailableOpenMRs always errors with the disclosed reason.
func TestUnavailableOpenMRs(t *testing.T) {
	_, err := unavailableOpenMRs{reason: "forge \"gitlab\" is configured but unreachable"}.OpenMRSourceBranches(context.Background())
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
