package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/featurecoverage"
	"github.com/jyang234/verdi/internal/index"
	"github.com/jyang234/verdi/internal/initwizard"
)

// newStoryRefsWire is GET /newstory-fixture/refs's wire contract, decoded
// strictly here rather than through the route's own type, so a renamed or
// added field fails this test instead of moving with the code.
type newStoryRefsWire struct {
	Porcelain []string `json:"porcelain"`
	Refs      []string `json:"refs"`
}

// startNewStoryThroughControl starts the new-story fixture through the
// harness's own control mux — the registered GET /newstory-fixture route,
// exactly as a Playwright file reaches it — and returns the fixture, the
// control server (whose read-only routes the tests call), and the
// fixture's served base URL.
func startNewStoryThroughControl(t *testing.T) (*newStoryFixture, *httptest.Server, string) {
	t.Helper()
	neutralizeCIEnvForTest(t)
	ctrl := newControlServer(t.TempDir(), testModuleRoot, "")
	t.Cleanup(ctrl.newStory.stop)
	srv := httptest.NewServer(ctrl.handler())
	t.Cleanup(srv.Close)
	status, body := httpGetBody(t, srv.URL+"/newstory-fixture")
	if status != http.StatusOK {
		t.Fatalf("GET /newstory-fixture = %d: %s", status, body)
	}
	base := strings.TrimSpace(body)
	if !strings.HasPrefix(base, "http://127.0.0.1:") || !strings.HasSuffix(base, "/") {
		t.Fatalf("fixture URL = %q, want a loopback base URL", base)
	}
	return ctrl.newStory, srv, base
}

// getRefs calls GET /newstory-fixture/refs and decodes it strictly,
// failing unless the body is canonical JSON: sorted keys, no HTML
// escaping, one trailing newline.
func getRefs(t *testing.T, ctrlURL string) newStoryRefsWire {
	t.Helper()
	status, body := httpGetBody(t, ctrlURL+"/newstory-fixture/refs")
	if status != http.StatusOK {
		t.Fatalf("GET /newstory-fixture/refs = %d: %s", status, body)
	}
	dec := json.NewDecoder(strings.NewReader(body))
	dec.DisallowUnknownFields()
	var got newStoryRefsWire
	if err := dec.Decode(&got); err != nil {
		t.Fatalf("decoding /refs: %v\n%s", err, body)
	}
	if dec.More() {
		t.Fatalf("/refs carries trailing data: %s", body)
	}
	if got.Porcelain == nil || got.Refs == nil {
		t.Fatalf("/refs = %s, want both fields present as arrays, never null", body)
	}
	var canonical bytes.Buffer
	enc := json.NewEncoder(&canonical)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(map[string][]string{"porcelain": got.Porcelain, "refs": got.Refs}); err != nil {
		t.Fatal(err)
	}
	if body != canonical.String() {
		t.Fatalf("/refs body is not canonical JSON:\n got %q\nwant %q", body, canonical.String())
	}
	return got
}

// showURL is GET /newstory-fixture/show's address for ref and path.
func showURL(ctrlURL, ref, path string) string {
	return ctrlURL + "/newstory-fixture/show?" + url.Values{"ref": {ref}, "path": {path}}.Encode()
}

// storeCoverage computes the fixture feature's criterion coverage from the
// store's own data through the real featurecoverage.Compute: the declared
// criteria and stubs decoded from the feature's spec, and the story half
// from the corpus index's implements backlinks.
func storeCoverage(t *testing.T, root string) ([]string, map[string]featurecoverage.Coverage) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(newStoryFeaturePath)))
	if err != nil {
		t.Fatal(err)
	}
	fm, _, err := artifact.SplitFrontmatter(raw)
	if err != nil {
		t.Fatal(err)
	}
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		t.Fatalf("the fixture feature does not decode: %v", err)
	}
	if spec.Class != artifact.ClassFeature || spec.Status != "accepted-pending-build" {
		t.Fatalf("the fixture feature is class %q, status %q; want an accepted-pending-build feature", spec.Class, spec.Status)
	}
	ix, err := index.Build(root)
	if err != nil {
		t.Fatalf("index.Build over the fixture store: %v", err)
	}
	var ids []string
	var links []featurecoverage.StoryLink
	for _, ac := range spec.AcceptanceCriteria {
		ids = append(ids, ac.ID)
		for _, bl := range ix.Backlinks("spec/" + newStoryFeatureName + "#" + ac.ID) {
			if bl.Type == "implemented-by" {
				links = append(links, featurecoverage.StoryLink{CriterionID: ac.ID, StoryRef: bl.From})
			}
		}
	}
	return ids, featurecoverage.Compute(ids, featurecoverage.StubDecls(spec.Stubs), links)
}

// TestNewStoryFixture_ThreeCoverageStates (SI-369 (10)): the store's one
// feature holds exactly the three coverage states, read through the real
// featurecoverage.Compute — ac-1 covered by a declared stub only, ac-2 by
// the accepted story only, ac-3 by nothing — and the story is accepted.
func TestNewStoryFixture_ThreeCoverageStates(t *testing.T) {
	root, err := provisionNewStoryStore(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("provisionNewStoryStore: %v", err)
	}
	ids, cov := storeCoverage(t, root)
	if want := []string{"ac-1", "ac-2", "ac-3"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("criteria = %v, want %v", ids, want)
	}
	want := map[string]featurecoverage.Coverage{
		"ac-1": {Stubs: []string{newStoryStubSlug}},
		"ac-2": {Stories: []string{"spec/" + newStoryStoryName}},
		"ac-3": {},
	}
	if !reflect.DeepEqual(cov, want) {
		t.Fatalf("coverage = %+v, want %+v", cov, want)
	}
	for id, uncovered := range map[string]bool{"ac-1": false, "ac-2": false, "ac-3": true} {
		if got := cov[id].Uncovered(); got != uncovered {
			t.Errorf("%s Uncovered() = %v, want %v", id, got, uncovered)
		}
	}

	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(newStoryStoryPath)))
	if err != nil {
		t.Fatal(err)
	}
	fm, _, err := artifact.SplitFrontmatter(raw)
	if err != nil {
		t.Fatal(err)
	}
	story, err := artifact.DecodeSpec(fm)
	if err != nil {
		t.Fatalf("the fixture story does not decode: %v", err)
	}
	if story.Class != artifact.ClassStory || story.Status != "accepted-pending-build" {
		t.Errorf("the fixture story is class %q, status %q; want an accepted story", story.Class, story.Status)
	}

	model, err := os.ReadFile(filepath.Join(root, ".verdi", "model.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(model, initwizard.RenderModelYAML(initwizard.PlainPreset())) {
		t.Errorf("model.yaml is not the plain vocabulary preset:\n%s", model)
	}
}

// TestNewStoryFixture_ServesSealedWallDialogAndCallToAction: the served
// fixture is what F6a's and F6b's tests open — the index card's call to
// action claims ac-3 alone, in the plain preset's words; the sealed wall
// renders the create dialog with all three criteria and wears the stub
// half's chips (ac-1 one stub, ac-2 and ac-3 none). Loading both pages
// leaves the refs and porcelain snapshot unchanged.
func TestNewStoryFixture_ServesSealedWallDialogAndCallToAction(t *testing.T) {
	_, ctrl, base := startNewStoryThroughControl(t)
	before := getRefs(t, ctrl.URL)

	status, home := httpGetBody(t, base)
	if status != http.StatusOK {
		t.Fatalf("GET %s = %d", base, status)
	}
	for _, want := range []string{
		`data-testid="dir-cta" data-cta-ac="ac-3" data-cta-unclaimed="1" href="/board/spec/` + newStoryFeatureName + `?new-story=ac-3"`,
		`1 AC unclaimed · ac-3 · New planned story`,
	} {
		if !strings.Contains(home, want) {
			t.Errorf("the served index lacks %q", want)
		}
	}
	if got := strings.Count(home, `data-testid="dir-cta"`); got != 1 {
		t.Errorf("index calls to action = %d, want exactly 1 (the feature's)", got)
	}

	status, wall := httpGetBody(t, base+"board/spec/"+newStoryFeatureName)
	if status != http.StatusOK {
		t.Fatalf("GET the feature wall = %d", status)
	}
	for _, want := range []string{
		`id="create-dialog"`,
		`New planned story`,
		`data-testid="create-ac-ac-1"`,
		`data-testid="create-ac-ac-2"`,
		`data-testid="create-ac-ac-3"`,
		`data-testid="coverage-ac-1" data-coverage="1"`,
		`data-testid="coverage-ac-2" data-coverage="0"`,
		`data-testid="coverage-ac-3" data-coverage="0"`,
	} {
		if !strings.Contains(wall, want) {
			t.Errorf("the served wall lacks %q", want)
		}
	}

	if after := getRefs(t, ctrl.URL); !reflect.DeepEqual(after, before) {
		t.Errorf("loading the index and the wall changed the snapshot:\nbefore %+v\nafter  %+v", before, after)
	}
}

// TestNewStoryFixture_RefsSnapshot: GET /newstory-fixture/refs is the
// store's refs and porcelain, and a write made in the test shows in it —
// a cut branch in refs, an untracked and a modified file in porcelain,
// each line exactly as git prints it (the leading status column kept).
func TestNewStoryFixture_RefsSnapshot(t *testing.T) {
	f, ctrl, _ := startNewStoryThroughControl(t)
	ctx := context.Background()
	mainSHA, err := gitOutput(ctx, f.root, "rev-parse", "main")
	if err != nil {
		t.Fatal(err)
	}
	clean := newStoryRefsWire{
		Porcelain: []string{},
		Refs: []string{
			mainSHA + " refs/heads/main",
			mainSHA + " refs/remotes/origin/HEAD",
			mainSHA + " refs/remotes/origin/main",
		},
	}
	if got := getRefs(t, ctrl.URL); !reflect.DeepEqual(got, clean) {
		t.Fatalf("/refs = %+v, want %+v", got, clean)
	}

	if err := runGit(ctx, f.root, nil, "branch", "design/probe", "main"); err != nil {
		t.Fatal(err)
	}
	cut := getRefs(t, ctrl.URL)
	wantRefs := []string{clean.Refs[0], mainSHA + " refs/heads/design/probe", clean.Refs[1], clean.Refs[2]}
	sort.Strings(wantRefs)
	gotRefs := append([]string(nil), cut.Refs...)
	sort.Strings(gotRefs)
	if !reflect.DeepEqual(gotRefs, wantRefs) || len(cut.Porcelain) != 0 {
		t.Fatalf("/refs after a branch cut = %+v, want refs %v and a clean porcelain", cut, wantRefs)
	}

	if err := os.WriteFile(filepath.Join(f.root, "untracked.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.root, filepath.FromSlash(newStoryStoryPath)), []byte("edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dirty := getRefs(t, ctrl.URL)
	wantPorcelain := []string{" M " + newStoryStoryPath, "?? untracked.txt"}
	if !reflect.DeepEqual(dirty.Porcelain, wantPorcelain) {
		t.Fatalf("/refs porcelain after two writes = %q, want %q", dirty.Porcelain, wantPorcelain)
	}
}

// TestNewStoryFixture_Show: GET /newstory-fixture/show answers a file's
// exact bytes at a ref, 404 for an unknown ref, a path absent at the ref,
// or a path naming a directory, and 400 for a malformed ref or path.
func TestNewStoryFixture_Show(t *testing.T) {
	_, ctrl, _ := startNewStoryThroughControl(t)

	status, body := httpGetBody(t, showURL(ctrl.URL, "main", newStoryFeaturePath))
	if status != http.StatusOK || body != newStoryFeatureSpec {
		t.Fatalf("show main:%s = %d %q, want 200 and the spec's exact bytes", newStoryFeaturePath, status, body)
	}
	status, body = httpGetBody(t, showURL(ctrl.URL, "refs/remotes/origin/main", newStoryStoryPath))
	if status != http.StatusOK || body != newStoryStorySpec {
		t.Fatalf("show origin/main:%s = %d %q, want 200 and the spec's exact bytes", newStoryStoryPath, status, body)
	}

	for _, tc := range []struct {
		name, ref, path string
		want            int
	}{
		{"unknown ref", "design/never-cut", newStoryFeaturePath, http.StatusNotFound},
		{"path absent at the ref", "main", ".verdi/specs/active/never-written/spec.md", http.StatusNotFound},
		{"path names a directory", "main", ".verdi/specs/active", http.StatusNotFound},
		{"empty ref", "", newStoryFeaturePath, http.StatusBadRequest},
		{"ref as an option", "--output=x", newStoryFeaturePath, http.StatusBadRequest},
		{"ref carrying a colon", "main:x", newStoryFeaturePath, http.StatusBadRequest},
		{"ref with a revision suffix", "main~1", newStoryFeaturePath, http.StatusBadRequest},
		{"ref with a dot-dot", "main..main", newStoryFeaturePath, http.StatusBadRequest},
		{"empty path", "main", "", http.StatusBadRequest},
		{"absolute path", "main", "/etc/hosts", http.StatusBadRequest},
		{"traversing path", "main", ".verdi/../../x", http.StatusBadRequest},
		{"path with a newline", "main", "a\nb", http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if status, body := httpGetBody(t, showURL(ctrl.URL, tc.ref, tc.path)); status != tc.want {
				t.Errorf("show ref=%q path=%q = %d (%s), want %d", tc.ref, tc.path, status, body, tc.want)
			}
		})
	}
}

// TestNewStoryFixture_ReadRoutesBeforeStart: before the fixture is
// started, /refs and /show refuse with 409 and never start it — a read
// route provisions nothing.
func TestNewStoryFixture_ReadRoutesBeforeStart(t *testing.T) {
	ctrl := newControlServer(t.TempDir(), testModuleRoot, "")
	t.Cleanup(ctrl.newStory.stop)
	h := ctrl.handler()
	for _, target := range []string{"/newstory-fixture/refs", "/newstory-fixture/show?ref=main&path=x"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
		if rec.Code != http.StatusConflict {
			t.Errorf("GET %s before start = %d, want 409", target, rec.Code)
		}
	}
	if ctrl.newStory.url != "" || ctrl.newStory.root != "" {
		t.Fatalf("a read route started the fixture (url %q, root %q)", ctrl.newStory.url, ctrl.newStory.root)
	}
}

// TestNewStoryFixture_RoutesRefuseNonGET: every route answers GET only,
// and a refused request starts nothing.
func TestNewStoryFixture_RoutesRefuseNonGET(t *testing.T) {
	ctrl := newControlServer(t.TempDir(), testModuleRoot, "")
	t.Cleanup(ctrl.newStory.stop)
	h := ctrl.handler()
	for _, target := range []string{"/newstory-fixture", "/newstory-fixture/refs", "/newstory-fixture/show?ref=main&path=x"} {
		for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s %s = %d, want 405", method, target, rec.Code)
			}
		}
	}
	if ctrl.newStory.url != "" {
		t.Fatalf("a refused request started the fixture at %s", ctrl.newStory.url)
	}
}

// fixtureTreeState records every file and directory under dir — mode,
// size, modification time, and a file's content digest — so two states
// are equal only when nothing under dir was created, removed, or written.
func fixtureTreeState(t *testing.T, dir string) map[string]string {
	t.Helper()
	state := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		entry := fmt.Sprintf("%v %d %d", info.Mode(), info.Size(), info.ModTime().UnixNano())
		if info.Mode().IsRegular() {
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			entry += fmt.Sprintf(" %x", sha256.Sum256(raw))
		}
		state[rel] = entry
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", dir, err)
	}
	return state
}

// treeStateDiff names every path whose state differs between a and b.
func treeStateDiff(a, b map[string]string) []string {
	var diff []string
	for path, sa := range a {
		if sb, ok := b[path]; !ok {
			diff = append(diff, "removed "+path)
		} else if sa != sb {
			diff = append(diff, "changed "+path)
		}
	}
	for path := range b {
		if _, ok := a[path]; !ok {
			diff = append(diff, "added "+path)
		}
	}
	sort.Strings(diff)
	return diff
}

// TestTreeStateDiff: the witness itself sees an added, a removed, and a
// rewritten file, and nothing when nothing changed.
func TestTreeStateDiff(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"keep", "drop", "edit"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	before := fixtureTreeState(t, dir)
	if diff := treeStateDiff(before, fixtureTreeState(t, dir)); len(diff) != 0 {
		t.Fatalf("an untouched tree differs: %v", diff)
	}
	if err := os.Remove(filepath.Join(dir, "drop")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "edit"), []byte("EDIT"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "new"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	// The directory's own entry may or may not change with it: a coarse
	// filesystem clock can leave its modification time where it was.
	var got []string
	for _, d := range treeStateDiff(before, fixtureTreeState(t, dir)) {
		if d != "changed ." {
			got = append(got, d)
		}
	}
	want := []string{"added new", "changed edit", "removed drop"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("treeStateDiff = %v, want %v", got, want)
	}
}

// TestNewStoryFixture_GETsWriteNothing: once the fixture is started, no
// GET on its control routes — the base URL, /refs, and /show answering
// 200, 404, or 400 — writes anything under the fixture's directory: the
// store, its .git, and the bare origin are byte-for-byte and
// timestamp-for-timestamp unchanged.
func TestNewStoryFixture_GETsWriteNothing(t *testing.T) {
	f, ctrl, _ := startNewStoryThroughControl(t)
	before := fixtureTreeState(t, f.tmp)
	for _, target := range []string{
		ctrl.URL + "/newstory-fixture",
		ctrl.URL + "/newstory-fixture/refs",
		ctrl.URL + "/newstory-fixture/refs",
		showURL(ctrl.URL, "main", newStoryFeaturePath),
		showURL(ctrl.URL, "design/never-cut", newStoryFeaturePath),
		showURL(ctrl.URL, "main", ".verdi/specs/active"),
		showURL(ctrl.URL, "main", "../x"),
	} {
		httpGetBody(t, target)
	}
	if diff := treeStateDiff(before, fixtureTreeState(t, f.tmp)); len(diff) != 0 {
		t.Fatalf("a GET wrote under the fixture: %v", diff)
	}
}

// TestNewStoryFixture_CreateCutsALocalBranchOnly: the store hosts the
// existing creation path — a Create on the sealed wall cuts
// design/<name> in the store, which /refs then lists and /show reads,
// while the working tree stays clean and the bare origin is untouched.
func TestNewStoryFixture_CreateCutsALocalBranchOnly(t *testing.T) {
	f, ctrl, base := startNewStoryThroughControl(t)
	ctx := context.Background()
	origin := filepath.Join(f.tmp, "origin.git")
	originBefore, err := gitOutput(ctx, origin, "for-each-ref")
	if err != nil {
		t.Fatal(err)
	}
	before := getRefs(t, ctrl.URL)

	const name = "escrow-surplus-refund"
	status, body := httpPostJSON(t, base+"board/spec/"+newStoryFeatureName+"/api/create", map[string]any{
		"name":   name,
		"values": map[string]string{"Problem": "a surplus waits for a manual refund", "Outcome": "a surplus over the threshold is refunded automatically"},
		"acs":    []string{"ac-3"},
	})
	if status != http.StatusOK {
		t.Fatalf("Create = %d: %s", status, body)
	}

	after := getRefs(t, ctrl.URL)
	if !reflect.DeepEqual(after.Porcelain, before.Porcelain) {
		t.Errorf("Create changed the porcelain: %q -> %q", before.Porcelain, after.Porcelain)
	}
	var added []string
	for _, line := range after.Refs {
		if !containsString(before.Refs, line) {
			added = append(added, line)
		}
	}
	if len(added) != 1 || !strings.HasSuffix(added[0], " refs/heads/design/"+name) || len(after.Refs) != len(before.Refs)+1 {
		t.Fatalf("Create's ref change = %v (refs %v -> %v), want exactly refs/heads/design/%s added", added, before.Refs, after.Refs, name)
	}
	if originAfter, err := gitOutput(ctx, origin, "for-each-ref"); err != nil || originAfter != originBefore {
		t.Errorf("the bare origin changed: %q -> %q (%v)", originBefore, originAfter, err)
	}

	status, spec := httpGetBody(t, showURL(ctrl.URL, "design/"+name, ".verdi/specs/active/"+name+"/spec.md"))
	if status != http.StatusOK {
		t.Fatalf("show the created spec = %d: %s", status, spec)
	}
	for _, want := range []string{"id: spec/" + name, "spec/" + newStoryFeatureName + "#ac-3"} {
		if !strings.Contains(spec, want) {
			t.Errorf("the created spec lacks %q:\n%s", want, spec)
		}
	}
}

// TestNewStoryFixture_Idempotent: repeated requests return the SAME URL,
// never a second instance.
func TestNewStoryFixture_Idempotent(t *testing.T) {
	f := newNewStoryFixture()
	t.Cleanup(f.stop)
	get := func() string {
		rec := httptest.NewRecorder()
		f.handler(rec, httptest.NewRequest(http.MethodGet, "/newstory-fixture", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET /newstory-fixture = %d: %s", rec.Code, rec.Body.String())
		}
		return strings.TrimSpace(rec.Body.String())
	}
	if first, second := get(), get(); first != second {
		t.Fatalf("url changed across calls: %q then %q, want the same instance reused", first, second)
	}
}

// TestNewStoryFixture_StopClosesAndRemovesStore: stop() closes the
// isolated server, removes the fixture's temporary store, and forgets the
// store, so the read routes refuse again; it is idempotent — safe when
// never started, safe twice.
func TestNewStoryFixture_StopClosesAndRemovesStore(t *testing.T) {
	never := newNewStoryFixture()
	never.stop()
	never.stop()

	f, ctrl, base := startNewStoryThroughControl(t)
	tmp := f.tmp
	if tmp == "" || f.root == "" {
		t.Fatal("a started fixture recorded no temporary directory or store root")
	}
	f.stop()
	f.stop()
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Errorf("fixture temp dir %s still exists after stop (stat err %v)", tmp, err)
	}
	if status, _ := httpGetBody(t, ctrl.URL+"/newstory-fixture/refs"); status != http.StatusConflict {
		t.Errorf("GET /refs after stop = %d, want 409", status)
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, base, nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Timeout: 2 * time.Second}
	if resp, err := client.Do(req); err == nil {
		_ = resp.Body.Close()
		t.Errorf("the isolated server still answers at %s after stop", base)
	}
}

// TestProvisionNewStoryStore_Repositories: the store is cut for the
// dialog — main checked out and clean, a bare origin whose HEAD names
// main (so the default branch resolves and Create's base is local), a
// repo-local identity for Create's plumbing commit, and git's background
// maintenance off in both repositories (initRepo; BL-148).
func TestProvisionNewStoryStore_Repositories(t *testing.T) {
	ctx := context.Background()
	parent := t.TempDir()
	root, err := provisionNewStoryStore(ctx, parent)
	if err != nil {
		t.Fatalf("provisionNewStoryStore: %v", err)
	}
	if root != filepath.Join(parent, "store") {
		t.Fatalf("root = %s, want %s", root, filepath.Join(parent, "store"))
	}
	origin := filepath.Join(parent, "origin.git")
	for _, tc := range []struct {
		dir  string
		args []string
		want string
	}{
		{root, []string{"symbolic-ref", "HEAD"}, "refs/heads/main"},
		{root, []string{"symbolic-ref", "refs/remotes/origin/HEAD"}, "refs/remotes/origin/main"},
		{root, []string{"status", "--porcelain"}, ""},
		{root, []string{"config", "--local", "user.name"}, "verdi-e2e"},
		{root, []string{"config", "--local", "user.email"}, "e2e@verdi.invalid"},
		{root, []string{"config", "--local", "gc.autoDetach"}, "false"},
		{root, []string{"config", "--local", "maintenance.auto"}, "false"},
		{origin, []string{"rev-parse", "--is-bare-repository"}, "true"},
		{origin, []string{"symbolic-ref", "HEAD"}, "refs/heads/main"},
		{origin, []string{"config", "--local", "gc.autoDetach"}, "false"},
		{origin, []string{"config", "--local", "maintenance.auto"}, "false"},
	} {
		got, err := gitOutput(ctx, tc.dir, tc.args...)
		if err != nil || got != tc.want {
			t.Errorf("%s: git %v = %q (%v), want %q", tc.dir, tc.args, got, err, tc.want)
		}
	}
	local, err := gitOutput(ctx, root, "rev-parse", "main")
	if err != nil {
		t.Fatal(err)
	}
	if remote, err := gitOutput(ctx, origin, "rev-parse", "main"); err != nil || remote != local {
		t.Errorf("origin main = %q (%v), want the store's main %q", remote, err, local)
	}
}

// TestProvisionNewStoryStore_Refusals: provisioning honours ctx (main.go's
// interrupt path) and fails where its store cannot be created — an error,
// never a half-provisioned store reported as ready.
func TestProvisionNewStoryStore_Refusals(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := provisionNewStoryStore(ctx, t.TempDir()); !errors.Is(err, context.Canceled) {
		t.Errorf("provisionNewStoryStore under a cancelled context = %v, want context.Canceled", err)
	}
	blocker := filepath.Join(t.TempDir(), "a-file")
	if err := os.WriteFile(blocker, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := provisionNewStoryStore(context.Background(), blocker); err == nil {
		t.Error("provisionNewStoryStore beneath a regular file: want an error, got nil")
	}
}

// TestOutputLines: a command's output splits into its lines, each kept
// exactly (a leading space included), with no line for the final newline
// and an empty, non-nil list for no output.
func TestOutputLines(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want []string
	}{
		{"no output", "", []string{}},
		{"one line", "a\n", []string{"a"}},
		{"leading space kept", " M a\n?? b\n", []string{" M a", "?? b"}},
		{"no final newline", "a\nb", []string{"a", "b"}},
		{"a blank line inside", "a\n\nb\n", []string{"a", "", "b"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := outputLines([]byte(tc.raw))
			if got == nil || !reflect.DeepEqual(got, tc.want) {
				t.Errorf("outputLines(%q) = %#v, want %#v", tc.raw, got, tc.want)
			}
		})
	}
}
