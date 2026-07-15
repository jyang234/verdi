package workbench

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/disclosure"
	"github.com/jyang234/verdi/internal/refindex"
)

// fakeHomeGit is the workbench-side GitRunner double for the home page's
// two ref reads (the notfound branch probe; the resolve() default it
// stands in for). Every method returns the canned value — no git process.
type fakeHomeGit struct {
	local, remote []string
	err           error
}

func (f fakeHomeGit) DefaultBranch(ctx context.Context, dir string) (string, error) {
	return "", f.err
}
func (f fakeHomeGit) LocalDesignBranches(ctx context.Context, dir string) ([]string, error) {
	return f.local, f.err
}
func (f fakeHomeGit) RemoteDesignBranches(ctx context.Context, dir string) ([]string, error) {
	return f.remote, f.err
}
func (f fakeHomeGit) Show(ctx context.Context, dir, ref, path string) ([]byte, error) {
	return nil, f.err
}
func (f fakeHomeGit) ListTree(ctx context.Context, dir, ref, path string) ([]string, error) {
	return nil, f.err
}
func (f fakeHomeGit) IsAncestor(ctx context.Context, dir, ancestor, ref string) (bool, error) {
	return false, f.err
}

// fakeOpenMRs is the hermetic OpenMRLister double (co-2).
type fakeOpenMRs struct {
	branches []string
	err      error
}

func (f fakeOpenMRs) OpenMRSourceBranches(ctx context.Context) ([]string, error) {
	return f.branches, f.err
}

// cannedIndex returns a HomeDeps.Index over fixed entries.
func cannedIndex(entries []refindex.Entry, err error) func(context.Context) ([]refindex.Entry, error) {
	return func(context.Context) ([]refindex.Entry, error) { return entries, err }
}

// directoryFixtureEntries spans all four status groups, all three
// design-branch sources, a default-branch entry, and a disclosed
// (no-draft-spec) entry.
func directoryFixtureEntries() []refindex.Entry {
	noDraft := disclosure.New("refindex:no-draft-spec", "spec/uncharted-idea",
		`design branch "design/uncharted-idea" resolves but has no spec.md yet`)
	return []refindex.Entry{
		{Ref: "spec/settled-work", Source: refindex.SourceDefault, StatusGroup: refindex.StatusGroupTerminal, SpecStatus: "closed"},
		{Ref: "spec/live-component", Source: refindex.SourceDefault, StatusGroup: refindex.StatusGroupActiveComponents, SpecStatus: "active"},
		{Ref: "spec/next-build", Source: refindex.SourceDefault, StatusGroup: refindex.StatusGroupAcceptedPendingBuild, SpecStatus: "accepted-pending-build"},
		{Ref: "spec/local-draft", Source: refindex.SourceLocal, StatusGroup: refindex.StatusGroupDraftsInProgress, SpecStatus: "draft"},
		{Ref: "spec/remote-draft", Source: refindex.SourceRemote, StatusGroup: refindex.StatusGroupDraftsInProgress, SpecStatus: "draft"},
		{Ref: "spec/both-draft", Source: refindex.SourceBoth, StatusGroup: refindex.StatusGroupDraftsInProgress, SpecStatus: "draft"},
		{Ref: "spec/uncharted-idea", Source: refindex.SourceLocal, StatusGroup: refindex.StatusGroupDraftsInProgress, Disclosed: &noDraft},
	}
}

// writeActiveSpec plants a minimal decodable spec.md in root's active zone
// so a default-branch entry's board link (working-tree servability) and
// title enrichment have something to read.
func writeActiveSpec(t *testing.T, root, name, class, status, story string) {
	t.Helper()
	dir := filepath.Join(root, ".verdi", "specs", "active", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	spec := "---\nid: spec/" + name + "\nkind: spec\nclass: " + class + "\ntitle: \"Title of " + name + "\"\nstatus: " + status + "\nowners: [platform-team]\n"
	if story != "" {
		spec += "story: " + story + "\n"
	}
	if class == "feature" {
		spec += "acceptance_criteria:\n  - { id: ac-1, text: \"holds\", evidence: [static] }\n"
		if status != "draft" {
			spec += "frozen: { at: 2026-07-14, commit: 89f9926e9739b97e23eb52efb16206d0ff10ff4f }\n"
		}
	}
	spec += "---\n# " + name + "\n"
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}
}

func getHome(t *testing.T, root string, home HomeDeps) (int, string) {
	t.Helper()
	h := NewHandlerWithHome(root, Deps{}, home)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code, rec.Body.String()
}

// TestRenderHome_DirectoryGroupsChipsAndLinks is ac-1's render witness: the
// four status groups organize the page in order, every entry appears
// exactly once under its group, status- and source-chipped, linked per
// dc-3's grammars (unprefixed default-branch addresses; the /b/ escaped
// per-branch grammar for design entries).
func TestRenderHome_DirectoryGroupsChipsAndLinks(t *testing.T) {
	root := t.TempDir()
	writeActiveSpec(t, root, "next-build", "feature", "accepted-pending-build", "jira:X-1")
	writeActiveSpec(t, root, "live-component", "component", "active", "")

	code, body := getHome(t, root, HomeDeps{
		Index: cannedIndex(directoryFixtureEntries(), nil),
		Git:   fakeHomeGit{},
	})
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", code, body)
	}

	// (a) the four groups, as the page's organizing structure, in order.
	order := []string{
		`data-testid="dir-group-drafts-in-progress"`,
		`data-testid="dir-group-accepted-pending-build"`,
		`data-testid="dir-group-active-components"`,
		`data-testid="dir-group-terminal"`,
	}
	last := -1
	for _, marker := range order {
		i := strings.Index(body, marker)
		if i < 0 {
			t.Fatalf("home missing group %s; got: %s", marker, body)
		}
		if i < last {
			t.Fatalf("group %s renders out of dc-2 order", marker)
		}
		last = i
	}

	// (b) every entry exactly once.
	for _, name := range []string{"settled-work", "live-component", "next-build", "local-draft", "remote-draft", "both-draft", "uncharted-idea"} {
		if n := strings.Count(body, `data-testid="dir-entry-`+name+`"`); n != 1 {
			t.Fatalf("entry %s appears %d times, want exactly 1", name, n)
		}
	}

	// (c) status chips in the shared badge vocabulary.
	for _, chip := range []string{
		`<span class="badge badge-draft">draft</span>`,
		`<span class="badge badge-accepted-pending-build">accepted-pending-build</span>`,
		`<span class="badge badge-active">active</span>`,
		`<span class="badge badge-closed">closed</span>`,
	} {
		if !strings.Contains(body, chip) {
			t.Fatalf("home missing status chip %s; got: %s", chip, body)
		}
	}

	// (d) link grammar per dc-3: unprefixed default addresses...
	if !strings.Contains(body, `href="/a/spec/next-build"`) {
		t.Fatalf("default entry missing its corpus link; got: %s", body)
	}
	if !strings.Contains(body, `href="/board/spec/next-build"`) {
		t.Fatalf("default entry missing its unprefixed board link; got: %s", body)
	}
	// ...feature enrichment (matrix/verdict via the scalar story ref)...
	if !strings.Contains(body, `href="/matrix/jira:X-1"`) || !strings.Contains(body, `href="/verdict/jira:X-1"`) {
		t.Fatalf("feature default entry missing matrix/verdict links; got: %s", body)
	}
	// ...title enrichment from the working tree...
	if !strings.Contains(body, "Title of next-build") {
		t.Fatalf("default entry missing its working-tree title; got: %s", body)
	}
	// ...and the /b/ per-branch grammar for design entries, slash escaped.
	if !strings.Contains(body, `href="/b/design%2Flocal-draft/board/spec/local-draft"`) {
		t.Fatalf("design entry missing its /b/ escaped board link; got: %s", body)
	}
	if !strings.Contains(body, `href="/b/design%2Fremote-draft/board/spec/remote-draft"`) {
		t.Fatalf("remote design entry missing its /b/ escaped board link (one grammar for both sources); got: %s", body)
	}

	// (e) source disclosure chips (feature dc-5) on every source.
	for _, chip := range []string{
		`badge-src-default">default branch</span>`,
		`badge-src-local">local branch</span>`,
		`badge-src-remote">remote-tracking</span>`,
		`badge-src-both">local + remote</span>`,
	} {
		if !strings.Contains(body, chip) {
			t.Fatalf("home missing source chip %s; got: %s", chip, body)
		}
	}

	// A default entry with NO working-tree presence emits no board link
	// (dc-3: only addresses the routing serves).
	settled := entryBlock(t, body, "settled-work")
	if strings.Contains(settled, `href="/board/spec/settled-work"`) {
		t.Fatalf("working-tree-absent entry must not emit a board link; got: %s", settled)
	}
	if !strings.Contains(settled, `href="/a/spec/settled-work"`) {
		t.Fatalf("working-tree-absent entry still links its corpus page; got: %s", settled)
	}
}

// entryBlock cuts one dir-entry <li> block out of the rendered body.
func entryBlock(t *testing.T, body, name string) string {
	t.Helper()
	re := regexp.MustCompile(`(?s)<li class="dir-entry[^"]*" data-testid="dir-entry-` + name + `".*?</li>`)
	m := re.FindString(body)
	if m == "" {
		t.Fatalf("no dir-entry block for %s in: %s", name, body)
	}
	return m
}

// TestRenderHome_DisclosedEntry_IsNoticeNotLink is ac-3's no-draft-spec
// shape at the render layer: the entry names the branch, states the
// absence through the shared disclosure vocabulary, and carries no link.
func TestRenderHome_DisclosedEntry_IsNoticeNotLink(t *testing.T) {
	root := t.TempDir()
	_, body := getHome(t, root, HomeDeps{
		Index: cannedIndex(directoryFixtureEntries(), nil),
		Git:   fakeHomeGit{},
	})

	block := entryBlock(t, body, "uncharted-idea")
	if strings.Contains(block, "<a ") || strings.Contains(block, "href=") {
		t.Fatalf("disclosed entry must not be linked as if a board existed; got: %s", block)
	}
	if !strings.Contains(block, "design/uncharted-idea") {
		t.Fatalf("disclosed entry does not name its branch; got: %s", block)
	}
	if !strings.Contains(block, "disclosed-unproven") {
		t.Fatalf("disclosed entry not rendered in the shared disclosure vocabulary; got: %s", block)
	}
}

// TestRenderHome_InReviewChip is ac-2's chip half: the branch the forge
// reports an open MR for — and only that one — is chipped in review, and
// the second source is disclosed on the page.
func TestRenderHome_InReviewChip(t *testing.T) {
	root := t.TempDir()
	_, body := getHome(t, root, HomeDeps{
		Index:   cannedIndex(directoryFixtureEntries(), nil),
		Git:     fakeHomeGit{},
		OpenMRs: fakeOpenMRs{branches: []string{"design/both-draft"}},
	})

	if got := strings.Count(body, `class="badge badge-open dir-inreview"`); got != 1 {
		t.Fatalf("in-review chip count = %d, want exactly 1; body: %s", got, body)
	}
	if !strings.Contains(entryBlock(t, body, "both-draft"), "in review") {
		t.Fatalf("the open-MR branch's entry is not the chipped one")
	}
	if !strings.Contains(body, "a second source beside the refs") {
		t.Fatalf("the forge consultation is not disclosed as a second source; got: %s", body)
	}
	if strings.Contains(body, "MR status unavailable") {
		t.Fatalf("healthy consultation must not render the unavailable notice")
	}
}

// TestRenderHome_MRStatusUnavailable_DirectoryStillFull is ac-2's
// degradation half: an erroring forge consultation renders the disclosed
// "MR status unavailable" notice while every refs-computed entry still
// renders — complete, not blocked, not partial.
func TestRenderHome_MRStatusUnavailable_DirectoryStillFull(t *testing.T) {
	root := t.TempDir()
	code, body := getHome(t, root, HomeDeps{
		Index:   cannedIndex(directoryFixtureEntries(), nil),
		Git:     fakeHomeGit{},
		OpenMRs: fakeOpenMRs{err: errors.New("dial tcp 127.0.0.1:9999: connection refused")},
	})
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (never a blocked directory)", code)
	}
	if !strings.Contains(body, `data-testid="mr-status-unavailable"`) || !strings.Contains(body, "MR status unavailable") {
		t.Fatalf("degraded consultation missing the disclosed notice; got: %s", body)
	}
	for _, name := range []string{"settled-work", "live-component", "next-build", "local-draft", "remote-draft", "both-draft", "uncharted-idea"} {
		if !strings.Contains(body, `data-testid="dir-entry-`+name+`"`) {
			t.Fatalf("degraded render dropped entry %s — the directory must still render fully", name)
		}
	}
	if strings.Contains(body, "dir-inreview") {
		t.Fatalf("degraded render must not fabricate in-review chips")
	}
}

// TestRenderHome_NoForgeConfigured_SilentAbsence: a nil OpenMRs lister is
// the legitimate not-configured state — no chips, no notice, no
// second-source provenance line.
func TestRenderHome_NoForgeConfigured_SilentAbsence(t *testing.T) {
	root := t.TempDir()
	_, body := getHome(t, root, HomeDeps{
		Index: cannedIndex(directoryFixtureEntries(), nil),
		Git:   fakeHomeGit{},
	})
	if strings.Contains(body, "MR status unavailable") || strings.Contains(body, "dir-inreview") {
		t.Fatalf("unconfigured forge must be silently absent; got: %s", body)
	}
	if strings.Contains(body, "a second source beside the refs") {
		t.Fatalf("unconfigured forge must not disclose a second source that is not consulted")
	}
}

// TestRenderHome_IndexFailure_StillServes is dc-5's negative path: an
// index-computation failure renders as a disclosed inline notice in a
// still-served page — the surviving sections (services, boards) intact.
func TestRenderHome_IndexFailure_StillServes(t *testing.T) {
	root := t.TempDir()
	code, body := getHome(t, root, HomeDeps{
		Index: cannedIndex(nil, errors.New("refindex: default-branch walk: boom")),
		Git:   fakeHomeGit{},
	})
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (the home page is never a dead end)", code)
	}
	if !strings.Contains(body, "Could not compute the directory index") || !strings.Contains(body, "boom") {
		t.Fatalf("index failure not disclosed inline; got: %s", body)
	}
	if !strings.Contains(body, "home-services") || !strings.Contains(body, "home-boards") {
		t.Fatalf("surviving sections missing from the still-served page; got: %s", body)
	}
}

// TestConsultOpenMRs_Nil covers the not-configured contract directly.
func TestConsultOpenMRs_Nil(t *testing.T) {
	inReview, notice := consultOpenMRs(context.Background(), nil)
	if inReview != nil || notice != "" {
		t.Fatalf("nil lister: got (%v, %q), want (nil, \"\")", inReview, notice)
	}
}

// TestConsultOpenMRs_Table drives the happy and degraded consultations.
func TestConsultOpenMRs_Table(t *testing.T) {
	tests := []struct {
		name       string
		mrs        OpenMRLister
		wantBranch string
		wantNotice bool
	}{
		{"open MR reported", fakeOpenMRs{branches: []string{"design/x"}}, "design/x", false},
		{"forge unreachable", fakeOpenMRs{err: errors.New("connection refused")}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inReview, notice := consultOpenMRs(context.Background(), tt.mrs)
			if tt.wantNotice {
				if !strings.Contains(notice, "MR status unavailable") {
					t.Fatalf("notice = %q, want the disclosed MR-status absence", notice)
				}
				if len(inReview) != 0 {
					t.Fatalf("degraded consultation fabricated chips: %v", inReview)
				}
				return
			}
			if notice != "" {
				t.Fatalf("unexpected notice %q", notice)
			}
			if !inReview[tt.wantBranch] {
				t.Fatalf("inReview = %v, want %s", inReview, tt.wantBranch)
			}
		})
	}
}
