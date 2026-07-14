package gitlab

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestListOpenMRs_DrainsMultiplePages_XNextPage is ac-2's REST witness for
// GitLab: page one carries only an unrelated MR; the decisive open MR
// (iid 101) sits on page two only, reached via GitLab's X-Next-Page
// response header. A walker that stopped at page one would silently drop
// it — pendingsupersession.go's scan is the consumer this protects.
func TestListOpenMRs_DrainsMultiplePages_XNextPage(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/projects/42/merge_requests", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "2" {
			_ = json.NewEncoder(w).Encode([]mergeRequestJSON{
				{IID: 101, SourceBranch: "design/page-two-branch", Title: "decisive, page two only"},
			})
			return
		}
		w.Header().Set("X-Next-Page", "2")
		_ = json.NewEncoder(w).Encode([]mergeRequestJSON{
			{IID: 1, SourceBranch: "design/page-one-branch", Title: "page one"},
		})
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	a := New(Config{BaseURL: ts.URL, ProjectID: "42", HTTPClient: ts.Client()})
	mrs, err := a.ListOpenMRs(context.Background(), "main")
	if err != nil {
		t.Fatalf("ListOpenMRs: %v", err)
	}
	if len(mrs) != 2 {
		t.Fatalf("ListOpenMRs drained %d MRs, want 2 (one per page): %+v", len(mrs), mrs)
	}
	var found bool
	for _, m := range mrs {
		if m.ID == "101" {
			found = true
		}
	}
	if !found {
		t.Fatalf("ListOpenMRs drained result missing the page-two decisive MR iid=101: %+v", mrs)
	}
}

// TestFindJob_DrainsMultiplePages_XNextPage is ac-2's second GitLab REST
// walker witness (findJob's listing, distinct from ListOpenMRs): the
// matching "verdi-evidence" job sits on page two only — page one carries
// an unrelated job. A walker stopping at page one would report
// forge.ErrNoBundle even though the job exists.
func TestFindJob_DrainsMultiplePages_XNextPage(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/projects/42/pipelines/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "2" {
			_ = json.NewEncoder(w).Encode([]job{{ID: 999, Name: defaultJobName}})
			return
		}
		w.Header().Set("X-Next-Page", "2")
		_ = json.NewEncoder(w).Encode([]job{{ID: 1, Name: "unrelated-job"}})
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	a := New(Config{BaseURL: ts.URL, ProjectID: "42", HTTPClient: ts.Client()})
	jobID, err := a.findJob(context.Background(), 55)
	if err != nil {
		t.Fatalf("findJob: %v", err)
	}
	if jobID != 999 {
		t.Fatalf("findJob = %d, want 999 (the page-two %q job)", jobID, defaultJobName)
	}
}

// TestListOpenMRs_MalformedXNextPage_StopsCleanly proves a present but
// unparseable X-Next-Page value stops the walk after page one — no error,
// no second request, no hang (co-1's negative witness, mirroring github's
// malformed-Link-header handling).
func TestListOpenMRs_MalformedXNextPage_StopsCleanly(t *testing.T) {
	requests := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/projects/42/merge_requests", func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("X-Next-Page", "not-a-number")
		_ = json.NewEncoder(w).Encode([]mergeRequestJSON{{IID: 1, SourceBranch: "b", Title: "only page"}})
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	a := New(Config{BaseURL: ts.URL, ProjectID: "42", HTTPClient: ts.Client()})
	mrs, err := a.ListOpenMRs(context.Background(), "main")
	if err != nil {
		t.Fatalf("ListOpenMRs with a malformed X-Next-Page: %v", err)
	}
	if len(mrs) != 1 {
		t.Fatalf("ListOpenMRs with a malformed X-Next-Page returned %d MRs, want exactly 1 (stop cleanly on page one)", len(mrs))
	}
	if requests != 1 {
		t.Fatalf("ListOpenMRs with a malformed X-Next-Page issued %d requests, want exactly 1", requests)
	}
}

// TestGitlabDrainList_RepeatedNextPage_FailsLoud proves a server whose
// X-Next-Page always echoes the page just requested fails loud (a named
// "pagination loop detected" error) rather than looping forever — bounded
// by the guard firing on the first repeat, not by a production page cap
// (co-1).
func TestGitlabDrainList_RepeatedNextPage_FailsLoud(t *testing.T) {
	requests := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/projects/42/merge_requests", func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("X-Next-Page", r.URL.Query().Get("page"))
		_ = json.NewEncoder(w).Encode([]mergeRequestJSON{{IID: 1, SourceBranch: "b", Title: "t"}})
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	a := New(Config{BaseURL: ts.URL, ProjectID: "42", HTTPClient: ts.Client()})
	_, err := a.ListOpenMRs(context.Background(), "main")
	if err == nil {
		t.Fatal("ListOpenMRs against an X-Next-Page repeating the just-fetched page: want error, got nil")
	}
	if !strings.Contains(err.Error(), "loop") {
		t.Errorf("error %q does not name the pagination loop", err.Error())
	}
	if requests != 1 {
		t.Fatalf("ListOpenMRs issued %d requests before failing loud, want exactly 1 (the guard fires on the first repeat, never spins)", requests)
	}
}
