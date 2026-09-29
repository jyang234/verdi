package workbench

// spec/wall-changes: the behavioral obligations (ac-1/ac-2/ac-3--
// behavioral), each proven over a real fixturegit repository — the ONLY
// git integration this story's tests exercise (co-2: never the network,
// never the live store).

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/boardlayout"
	"github.com/jyang234/verdi/internal/gitx"
)

const (
	wallFixtureName     = "wall-fixture"
	wallFixtureSpecPath = ".verdi/specs/active/" + wallFixtureName + "/spec.md"
	// wallNestedPrefix is where the nested-store fixture puts its store
	// below the git top level.
	wallNestedPrefix = "store/"
)

// wallFixtureLayoutPath is the spec's layout sidecar, derived exactly as
// production derives it.
var wallFixtureLayoutPath = filepath.ToSlash(boardlayout.FilePath(path.Dir(wallFixtureSpecPath)))

// wallChangesUndecodableFields has valid delimiters but a frontmatter key
// the strict decoder refuses — its failure text names the key, so a body
// quoting it can only be quoting the working tree's own bytes.
var wallChangesUndecodableFields = strings.Replace(wallChangesHeadSpec, "owners: [platform-team]\n", "owners: [platform-team]\nnot_a_spec_field: 1\n", 1)

// newWallChangesFixture builds an authoring-board fixture (buildAuthoringFixture,
// testfixture_test.go): main carries no revision of the spec at all; the
// design branch commits headSpec as its own HEAD, then stays checked out —
// the shape every case below layers its own uncommitted edit onto.
// headSpec == "" commits nothing (the "missing spec" case: the spec never
// reaches HEAD at all). It returns the git top level, which is also the
// store root.
func newWallChangesFixture(t *testing.T, headSpec string) string {
	t.Helper()
	top, _ := buildWallFixture(t, "", headSpec, nil)
	return top
}

// buildWallFixture builds the fixture with its store at prefix below the
// git top level ("" for a store at the top level) and returns both the top
// level and the store root the wall is served from. extraDraft adds files
// (store-relative) to the design branch's commit.
func buildWallFixture(t *testing.T, prefix, headSpec string, extraDraft map[string]string) (top, root string) {
	t.Helper()
	// commitFilesOnBranch (testfixture_test.go) commits whatever draft
	// carries; files unrelated to the spec keep the branch's commit real
	// even when headSpec == "" and give the rename/delete cases tracked
	// paths to move.
	draft := map[string]string{
		prefix + "README-fixture.md": "wall-changes fixture branch\n",
		prefix + "docs/guide.md":     "a tracked guide\n",
	}
	if headSpec != "" {
		draft[prefix+wallFixtureSpecPath] = headSpec
	}
	for rel, content := range extraDraft {
		draft[prefix+rel] = content
	}
	mainFiles := map[string]string{prefix + ".verdi/.gitignore": "data/\n"}
	if prefix != "" {
		mainFiles["outside-store.md"] = "a tracked file above the store\n"
	}
	top = buildAuthoringFixture(t, "design/"+wallFixtureName, mainFiles, draft)
	return top, filepath.Join(top, filepath.FromSlash(prefix))
}

// writeWallFile writes content at root/relPath, creating parent
// directories as needed — an UNCOMMITTED edit layered onto a fixture's own
// HEAD.
func writeWallFile(t *testing.T, root, relPath, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// runWallGit runs git in dir, failing the test on error.
func runWallGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// stageThenRevertSpec stages a prose edit to the spec, then restores the
// working tree to HEAD's bytes: the index alone holds the change.
func stageThenRevertSpec(t *testing.T, root string) {
	t.Helper()
	writeWallFile(t, root, wallFixtureSpecPath, wallChangesProseOnly)
	if err := gitx.AddAll(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	writeWallFile(t, root, wallFixtureSpecPath, wallChangesHeadSpec)
}

// wallSnapshotResult is one served snapshot: the strict-decoded value, and
// the raw bytes of its git.changes object for wire-literal assertions.
type wallSnapshotResult struct {
	rec        *httptest.ResponseRecorder
	snap       asdSnapshot
	rawChanges json.RawMessage
}

// fetchWallSnapshot GETs /board/spec/{name}/snapshot and, on 200,
// strict-decodes it (unknown fields and trailing data both refused) and
// pulls out the raw git.changes object.
func fetchWallSnapshot(t *testing.T, h http.Handler, name string) wallSnapshotResult {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/board/spec/"+name+"/snapshot", nil))
	out := wallSnapshotResult{rec: rec}
	if rec.Code != http.StatusOK {
		return out
	}
	if err := artifact.DecodeStrictJSON(rec.Body.Bytes(), &out.snap); err != nil {
		t.Fatalf("strict-decoding the snapshot JSON: %v\n%s", err, rec.Body.String())
	}
	out.rawChanges = rawGitChanges(t, rec.Body.Bytes())
	return out
}

// rawGitChanges returns the raw git.changes object of a JSON document
// whose top level carries a "git" object.
func rawGitChanges(t *testing.T, doc []byte) json.RawMessage {
	t.Helper()
	var top map[string]json.RawMessage
	if err := json.Unmarshal(doc, &top); err != nil {
		t.Fatalf("decoding document: %v", err)
	}
	var git map[string]json.RawMessage
	if err := json.Unmarshal(top["git"], &git); err != nil {
		t.Fatalf("decoding git: %v\n%s", err, top["git"])
	}
	return git["changes"]
}

// fetchWallPageChanges GETs the wall page and returns the raw git.changes
// object of its embedded page model (window.__BOARDV2__).
func fetchWallPageChanges(t *testing.T, h http.Handler, name string) json.RawMessage {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/board/spec/"+name, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET page = %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	const marker = "window.__BOARDV2__ = "
	start := strings.Index(body, marker)
	if start < 0 {
		t.Fatal("page embeds no window.__BOARDV2__ state")
	}
	rest := body[start+len(marker):]
	end := strings.Index(rest, ";\n</script>")
	if end < 0 {
		t.Fatal("page state is not terminated")
	}
	return rawGitChanges(t, []byte(rest[:end]))
}

// changesKeys lists the top-level keys of a raw changes object, sorted.
func changesKeys(t *testing.T, raw json.RawMessage) []string {
	t.Helper()
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("decoding changes %s: %v", raw, err)
	}
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// typedTargets lists the typed changes' targets in order.
func typedTargets(c *wallChanges) []string {
	targets := []string{}
	for _, change := range c.Typed {
		targets = append(targets, change.Target)
	}
	return targets
}

// unclassified is a literal-reason entry: the reasons are the public wire
// enum, so the tests spell them out rather than reuse the Go constants.
func unclassified(p, reason string) wallUnclassifiedChange {
	return wallUnclassifiedChange{Path: p, Reason: wallChangeReason(reason)}
}

// wallServedCase is one state of a served wall.
type wallServedCase struct {
	name     string
	headSpec string
	// prefix places the store below the git top level (a nested store).
	prefix string
	// extraDraft adds store-relative files to the committed design branch.
	extraDraft map[string]string
	// edit layers the uncommitted state onto the fixture; top is the git
	// top level, root the store root.
	edit func(t *testing.T, top, root string)

	// wantCode is the snapshot's status (0 means 200); wantBody lists
	// substrings a non-200 body must carry.
	wantCode int
	wantBody []string

	wantDirty bool
	// wantTyped lists the typed targets; nil means the comparison is
	// unreadable and "typed" must be absent from the JSON.
	wantTyped        []string
	wantUnclassified []wallUnclassifiedChange
	// wantUnreadable is a substring the unreadable reason must carry,
	// naming which side could not be read; "" means readable.
	wantUnreadable string
}

// wallServedCases is every served state TestWallChanges_Snapshot covers:
// each case of the static table that git can produce, plus the states only
// git can produce (a mode change, a staged-then-reverted spec, an unborn
// HEAD, a nested store, an ignored spec).
func wallServedCases() []wallServedCase {
	spec := wallFixtureSpecPath
	layout := wallFixtureLayoutPath
	return []wallServedCase{
		{
			name: "typed-only edit", headSpec: wallChangesHeadSpec,
			edit:      func(t *testing.T, _, root string) { writeWallFile(t, root, spec, wallChangesTypedOnly) },
			wantDirty: true, wantTyped: []string{"ac-1"}, wantUnclassified: []wallUnclassifiedChange{},
		},
		{
			name: "prose-only edit", headSpec: wallChangesHeadSpec,
			edit:      func(t *testing.T, _, root string) { writeWallFile(t, root, spec, wallChangesProseOnly) },
			wantDirty: true, wantTyped: []string{},
			wantUnclassified: []wallUnclassifiedChange{unclassified(spec, "prose-or-body-text")},
		},
		{
			name: "layout-only edit", headSpec: wallChangesHeadSpec,
			edit: func(t *testing.T, _, root string) {
				writeWallFile(t, root, layout, `{"schema":"verdi.boardlayout/v1","positions":{"ac-1":{"x":1,"y":2}}}`)
			},
			wantDirty: true, wantTyped: []string{},
			wantUnclassified: []wallUnclassifiedChange{unclassified(layout, "layout")},
		},
		{
			name: "another staged path", headSpec: wallChangesHeadSpec,
			edit: func(t *testing.T, _, root string) {
				writeWallFile(t, root, "docs/notes.md", "unrelated content\n")
				if err := gitx.AddAll(context.Background(), root); err != nil {
					t.Fatal(err)
				}
			},
			wantDirty: true, wantTyped: []string{},
			wantUnclassified: []wallUnclassifiedChange{unclassified("docs/notes.md", "another-staged-path")},
		},
		{
			name: "another path edited but not staged (SI-299)", headSpec: wallChangesHeadSpec,
			edit: func(t *testing.T, _, root string) {
				writeWallFile(t, root, "README-fixture.md", "edited, never staged\n")
			},
			wantDirty: true, wantTyped: []string{},
			wantUnclassified: []wallUnclassifiedChange{unclassified("README-fixture.md", "another-staged-path")},
		},
		{
			name: "a deletion and both sides of a rename (SI-299)", headSpec: wallChangesHeadSpec,
			edit: func(t *testing.T, _, root string) {
				if err := os.Remove(filepath.Join(root, "README-fixture.md")); err != nil {
					t.Fatal(err)
				}
				runWallGit(t, root, "mv", "docs/guide.md", "docs/moved.md")
			},
			wantDirty: true, wantTyped: []string{},
			wantUnclassified: []wallUnclassifiedChange{
				unclassified("README-fixture.md", "another-staged-path"),
				unclassified("docs/guide.md", "another-staged-path"),
				unclassified("docs/moved.md", "another-staged-path"),
			},
		},
		{
			name: "an untracked file", headSpec: wallChangesHeadSpec,
			edit:      func(t *testing.T, _, root string) { writeWallFile(t, root, "notes.txt", "scratch\n") },
			wantDirty: true, wantTyped: []string{},
			wantUnclassified: []wallUnclassifiedChange{unclassified("notes.txt", "untracked-file")},
		},
		{
			name: "an untracked file with an unusual name", headSpec: wallChangesHeadSpec,
			edit:      func(t *testing.T, _, root string) { writeWallFile(t, root, `café "q".txt`, "scratch\n") },
			wantDirty: true, wantTyped: []string{},
			wantUnclassified: []wallUnclassifiedChange{unclassified(`café "q".txt`, "untracked-file")},
		},
		{
			name: "mixed tree", headSpec: wallChangesHeadSpec,
			edit: func(t *testing.T, _, root string) {
				writeWallFile(t, root, "docs/notes.md", "staged content\n")
				if err := gitx.AddAll(context.Background(), root); err != nil {
					t.Fatal(err)
				}
				writeWallFile(t, root, spec, wallChangesMixed)
				writeWallFile(t, root, layout, `{"schema":"verdi.boardlayout/v1","positions":{}}`)
				writeWallFile(t, root, "notes.txt", "scratch\n")
			},
			wantDirty: true, wantTyped: []string{"ac-1"},
			wantUnclassified: []wallUnclassifiedChange{
				unclassified(layout, "layout"),
				unclassified(spec, "prose-or-body-text"),
				unclassified("docs/notes.md", "another-staged-path"),
				unclassified("notes.txt", "untracked-file"),
			},
		},
		{
			name: "a title-only edit (SI-298)", headSpec: wallChangesHeadSpec,
			edit:      func(t *testing.T, _, root string) { writeWallFile(t, root, spec, wallChangesTitleOnly) },
			wantDirty: true, wantTyped: []string{},
			wantUnclassified: []wallUnclassifiedChange{unclassified(spec, "unrecognized-spec-change")},
		},
		{
			name: "a title edit beside a typed edit (SI-298)", headSpec: wallChangesHeadSpec,
			edit:      func(t *testing.T, _, root string) { writeWallFile(t, root, spec, wallChangesTitleAndTyped) },
			wantDirty: true, wantTyped: []string{"ac-1"},
			wantUnclassified: []wallUnclassifiedChange{unclassified(spec, "unrecognized-spec-change")},
		},
		{
			name: "a mode-only change (SI-298)", headSpec: wallChangesHeadSpec,
			edit: func(t *testing.T, _, root string) {
				if err := os.Chmod(filepath.Join(root, filepath.FromSlash(spec)), 0o755); err != nil {
					t.Fatal(err)
				}
			},
			wantDirty: true, wantTyped: []string{},
			wantUnclassified: []wallUnclassifiedChange{unclassified(spec, "unrecognized-spec-change")},
		},
		{
			name: "a spec staged then reverted (SI-298)", headSpec: wallChangesHeadSpec,
			edit:      func(t *testing.T, _, root string) { stageThenRevertSpec(t, root) },
			wantDirty: true, wantTyped: []string{},
			wantUnclassified: []wallUnclassifiedChange{unclassified(spec, "unrecognized-spec-change")},
		},
		{
			name: "unreadable: a missing spec still lists other changes", headSpec: "",
			edit: func(t *testing.T, _, root string) {
				writeWallFile(t, root, spec, wallChangesHeadSpec)
				writeWallFile(t, root, "notes.txt", "scratch\n")
			},
			wantDirty: true, wantUnreadable: "has no revision at HEAD",
			wantUnclassified: []wallUnclassifiedChange{
				unclassified(spec, "untracked-file"),
				unclassified("notes.txt", "untracked-file"),
			},
		},
		{
			name: "unreadable: an undecodable HEAD spec names HEAD and lists other changes", headSpec: wallChangesUndecodable,
			edit: func(t *testing.T, _, root string) {
				writeWallFile(t, root, spec, wallChangesHeadSpec)
				writeWallFile(t, root, "notes.txt", "scratch\n")
			},
			wantDirty: true, wantUnreadable: "HEAD's spec.md could not be read",
			wantUnclassified: []wallUnclassifiedChange{
				unclassified(spec, "unrecognized-spec-change"),
				unclassified("notes.txt", "untracked-file"),
			},
		},
		{
			name: "unreadable: an unborn HEAD is disclosed, never an error", headSpec: wallChangesHeadSpec,
			edit:      func(t *testing.T, top, _ string) { runWallGit(t, top, "checkout", "-q", "--orphan", "design/unborn") },
			wantDirty: true, wantUnreadable: "HEAD does not name a commit yet",
			wantUnclassified: []wallUnclassifiedChange{
				unclassified(".verdi/.gitignore", "another-staged-path"),
				unclassified(spec, "unrecognized-spec-change"),
				unclassified("README-fixture.md", "another-staged-path"),
				unclassified("docs/guide.md", "another-staged-path"),
			},
		},
		{
			name: "unreadable: an ignored spec never committed, over a clean tree", headSpec: "",
			extraDraft: map[string]string{".gitignore": wallFixtureSpecPath + "\n"},
			edit:       func(t *testing.T, _, root string) { writeWallFile(t, root, spec, wallChangesHeadSpec) },
			wantDirty:  false, wantUnreadable: "has no revision at HEAD",
			wantUnclassified: []wallUnclassifiedChange{},
		},
		{
			name: "an undecodable working-tree spec fails the snapshot, naming the working tree's decode failure", headSpec: wallChangesHeadSpec,
			edit:     func(t *testing.T, _, root string) { writeWallFile(t, root, spec, wallChangesUndecodableFields) },
			wantCode: http.StatusInternalServerError,
			wantBody: []string{"workbench: spec " + wallFixtureName + ":", "not_a_spec_field"},
		},
		{
			name: "a nested store resolves every path against the git top level", headSpec: wallChangesHeadSpec, prefix: wallNestedPrefix,
			edit: func(t *testing.T, top, root string) {
				writeWallFile(t, root, spec, wallChangesTypedOnly)
				writeWallFile(t, root, layout, `{"schema":"verdi.boardlayout/v1","positions":{}}`)
				writeWallFile(t, root, "notes.txt", "inside the store\n")
				writeWallFile(t, top, "top-notes.txt", "above the store\n")
				writeWallFile(t, top, "outside-store.md", "edited above the store\n")
			},
			wantDirty: true, wantTyped: []string{"ac-1"},
			wantUnclassified: []wallUnclassifiedChange{
				unclassified("outside-store.md", "another-staged-path"),
				unclassified(wallNestedPrefix+layout, "layout"),
				unclassified(wallNestedPrefix+"notes.txt", "untracked-file"),
				unclassified("top-notes.txt", "untracked-file"),
			},
		},
	}
}

// TestWallChanges_Snapshot is spec/wall-changes ac-1's behavioral
// obligation: served over a fixturegit repository, the wall's snapshot JSON
// and its page model carry the typed, unclassified, and unreadable changes
// the working tree actually has, for each state the static obligation's
// table covers — and the states only git produces. Every snapshot is
// strict-decoded, the wire keys and reason literals are pinned, and the
// page model's changes are byte-identical to the snapshot's.
func TestWallChanges_Snapshot(t *testing.T) {
	for _, tc := range wallServedCases() {
		t.Run(tc.name, func(t *testing.T) {
			top, root := buildWallFixture(t, tc.prefix, tc.headSpec, tc.extraDraft)
			h := NewHandler(root)
			tc.edit(t, top, root)

			got := fetchWallSnapshot(t, h, wallFixtureName)
			wantCode := tc.wantCode
			if wantCode == 0 {
				wantCode = http.StatusOK
			}
			if got.rec.Code != wantCode {
				t.Fatalf("GET snapshot = %d, want %d\n%s", got.rec.Code, wantCode, got.rec.Body.String())
			}
			if wantCode != http.StatusOK {
				body := got.rec.Body.String()
				for _, want := range tc.wantBody {
					if !strings.Contains(body, want) {
						t.Errorf("body %s does not carry %q", body, want)
					}
				}
				if strings.Contains(body, `"changes"`) {
					t.Errorf("a failed snapshot still carries a changes summary: %s", body)
				}
				return
			}

			if got.snap.Git == nil || got.snap.Git.Changes == nil {
				t.Fatal("snapshot carries no wall-changes summary")
			}
			if got.snap.Git.Dirty != tc.wantDirty {
				t.Errorf("dirty = %v, want %v (git status's own answer)", got.snap.Git.Dirty, tc.wantDirty)
			}
			changes := got.snap.Git.Changes
			if tc.wantUnreadable == "" {
				if changes.UnreadableReason != "" {
					t.Errorf("unreadableReason = %q, want a readable comparison", changes.UnreadableReason)
				}
				if keys := changesKeys(t, got.rawChanges); !reflect.DeepEqual(keys, []string{"typed", "unclassified"}) {
					t.Errorf("changes keys = %v, want [typed unclassified]: %s", keys, got.rawChanges)
				}
				if targets := typedTargets(changes); !reflect.DeepEqual(targets, tc.wantTyped) {
					t.Errorf("typed targets = %v, want %v", targets, tc.wantTyped)
				}
			} else {
				if !strings.Contains(changes.UnreadableReason, tc.wantUnreadable) {
					t.Errorf("unreadableReason = %q, want it to carry %q", changes.UnreadableReason, tc.wantUnreadable)
				}
				if keys := changesKeys(t, got.rawChanges); !reflect.DeepEqual(keys, []string{"unclassified", "unreadableReason"}) {
					t.Errorf("changes keys = %v, want [unclassified unreadableReason] (no typed list, dc-2): %s", keys, got.rawChanges)
				}
			}
			if !reflect.DeepEqual(changes.Unclassified, tc.wantUnclassified) {
				t.Errorf("unclassified = %+v\nwant           %+v", changes.Unclassified, tc.wantUnclassified)
			}

			if page := fetchWallPageChanges(t, h, wallFixtureName); !bytes.Equal(page, got.rawChanges) {
				t.Errorf("page model changes = %s\nsnapshot changes   = %s", page, got.rawChanges)
			}
		})
	}
}

// TestWallChanges_SnapshotStaysDirty is spec/wall-changes ac-2's behavioral
// obligation: a wall served over a repository whose only changes are
// unclassified still reports dirty (boardGitState.Dirty, git status's own
// independent answer) in its snapshot, and lists every one of them.
func TestWallChanges_SnapshotStaysDirty(t *testing.T) {
	cases := []struct {
		name  string
		apply func(t *testing.T, root string)
	}{
		{name: "prose-only", apply: func(t *testing.T, root string) { writeWallFile(t, root, wallFixtureSpecPath, wallChangesProseOnly) }},
		{name: "untracked-only", apply: func(t *testing.T, root string) { writeWallFile(t, root, "notes.txt", "scratch\n") }},
		{name: "layout-only", apply: func(t *testing.T, root string) {
			writeWallFile(t, root, wallFixtureLayoutPath, `{"schema":"verdi.boardlayout/v1","positions":{}}`)
		}},
		{name: "another path, unstaged", apply: func(t *testing.T, root string) { writeWallFile(t, root, "README-fixture.md", "edited\n") }},
		{name: "title-only", apply: func(t *testing.T, root string) { writeWallFile(t, root, wallFixtureSpecPath, wallChangesTitleOnly) }},
		{name: "staged then reverted, no other change", apply: stageThenRevertSpec},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := newWallChangesFixture(t, wallChangesHeadSpec)
			h := NewHandler(root)
			tc.apply(t, root)

			got := fetchWallSnapshot(t, h, wallFixtureName)
			if got.rec.Code != http.StatusOK {
				t.Fatalf("GET snapshot = %d\n%s", got.rec.Code, got.rec.Body.String())
			}
			if got.snap.Git == nil || !got.snap.Git.Dirty {
				t.Fatal("snapshot does not report dirty although the working tree has an unclassified change")
			}
			changes := got.snap.Git.Changes
			if changes == nil || len(changes.Typed) != 0 {
				t.Fatalf("changes = %+v, want zero recognized operations", changes)
			}
			if len(changes.Unclassified) == 0 {
				t.Fatal("unclassified is empty: a remaining change with zero recognized operations, silently reported clean")
			}
		})
	}
}

// TestWallChanges_BranchGuardIndependent is spec/wall-changes ac-3's
// behavioral obligation: the branch-switch guard (actionGitSwitch,
// unmodified by this story) reads the working tree's own git status
// through gitx.StatusDirty, independent of the changes summary — a zero
// typed-change count and an unreadable comparison alike still refuse a
// switch while git reports the tree dirty, and a clean tree switches even
// while the summary is unreadable.
func TestWallChanges_BranchGuardIndependent(t *testing.T) {
	cases := []struct {
		name       string
		headSpec   string
		extraDraft map[string]string
		apply      func(t *testing.T, root string)
		// check asserts the summary really is in the state the case tests.
		check    func(t *testing.T, got wallSnapshotResult)
		wantCode int
	}{
		{
			name: "zero typed changes, dirty tree", headSpec: wallChangesHeadSpec,
			apply: func(t *testing.T, root string) { writeWallFile(t, root, "notes.txt", "scratch\n") },
			check: func(t *testing.T, got wallSnapshotResult) {
				if len(got.snap.Git.Changes.Typed) != 0 || !got.snap.Git.Dirty {
					t.Fatalf("fixture = %+v dirty=%v, want zero typed changes over a dirty tree", got.snap.Git.Changes, got.snap.Git.Dirty)
				}
			},
			wantCode: http.StatusConflict,
		},
		{
			name: "unreadable comparison, dirty tree", headSpec: wallChangesUndecodable,
			apply: func(t *testing.T, root string) { writeWallFile(t, root, wallFixtureSpecPath, wallChangesHeadSpec) },
			check: func(t *testing.T, got wallSnapshotResult) {
				if got.snap.Git.Changes.UnreadableReason == "" || !got.snap.Git.Dirty {
					t.Fatalf("fixture = %+v dirty=%v, want an unreadable comparison over a dirty tree", got.snap.Git.Changes, got.snap.Git.Dirty)
				}
			},
			wantCode: http.StatusConflict,
		},
		{
			name: "staged then reverted, no other change", headSpec: wallChangesHeadSpec,
			apply: stageThenRevertSpec,
			check: func(t *testing.T, got wallSnapshotResult) {
				if len(got.snap.Git.Changes.Typed) != 0 || !got.snap.Git.Dirty {
					t.Fatalf("fixture = %+v dirty=%v, want zero typed changes over a dirty tree", got.snap.Git.Changes, got.snap.Git.Dirty)
				}
			},
			wantCode: http.StatusConflict,
		},
		{
			name: "clean tree, unreadable summary: the switch proceeds", headSpec: "",
			extraDraft: map[string]string{".gitignore": wallFixtureSpecPath + "\n"},
			apply:      func(t *testing.T, root string) { writeWallFile(t, root, wallFixtureSpecPath, wallChangesHeadSpec) },
			check: func(t *testing.T, got wallSnapshotResult) {
				if got.snap.Git.Changes.UnreadableReason == "" || got.snap.Git.Dirty {
					t.Fatalf("fixture = %+v dirty=%v, want an unreadable comparison over a clean tree", got.snap.Git.Changes, got.snap.Git.Dirty)
				}
			},
			wantCode: http.StatusOK,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, root := buildWallFixture(t, "", tc.headSpec, tc.extraDraft)
			h := NewHandler(root)
			tc.apply(t, root)

			got := fetchWallSnapshot(t, h, wallFixtureName)
			if got.rec.Code != http.StatusOK {
				t.Fatalf("GET snapshot = %d\n%s", got.rec.Code, got.rec.Body.String())
			}
			tc.check(t, got)

			switchRec := postBoardAPI(t, h, wallFixtureName, "git-switch", `{"branch":"main"}`)
			if switchRec.Code != tc.wantCode {
				t.Fatalf("git-switch = %d, want %d\n%s", switchRec.Code, tc.wantCode, switchRec.Body.String())
			}
		})
	}
}

// wallGitRecorder records every git invocation made on a context.
type wallGitRecorder struct {
	mu    sync.Mutex
	calls [][]string
}

func (r *wallGitRecorder) Observe(_ string, args []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, append([]string(nil), args...))
}

// ranSummary reports whether any recorded call is the summary's own
// untracked listing (gitx.UntrackedPaths: ls-files --others without
// --cached, which nothing else on these paths runs).
func (r *wallGitRecorder) ranSummary() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, args := range r.calls {
		if slices.Contains(args, "ls-files") && slices.Contains(args, "--others") && !slices.Contains(args, "--cached") {
			return true
		}
	}
	return false
}

// TestWallChanges_SummaryOnlyOnWallPageAndSnapshot proves only the wall page
// and its snapshot compute the summary: every other loadBoard caller (the
// fragment, the API actions, the MCP get_board projection) neither pays for
// it nor inherits its failures.
func TestWallChanges_SummaryOnlyOnWallPageAndSnapshot(t *testing.T) {
	cases := []struct {
		name     string
		call     func(ctx context.Context, h http.Handler, root string) int
		wantPays bool
	}{
		{name: "the wall page", wantPays: true, call: func(ctx context.Context, h http.Handler, _ string) int {
			return serveWall(ctx, h, http.MethodGet, "/board/spec/"+wallFixtureName, "")
		}},
		{name: "the snapshot", wantPays: true, call: func(ctx context.Context, h http.Handler, _ string) int {
			return serveWall(ctx, h, http.MethodGet, "/board/spec/"+wallFixtureName+"/snapshot", "")
		}},
		{name: "the fragment", call: func(ctx context.Context, h http.Handler, _ string) int {
			return serveWall(ctx, h, http.MethodGet, "/board/spec/"+wallFixtureName+"/fragment", "")
		}},
		{name: "an API action (git-switch)", call: func(ctx context.Context, h http.Handler, _ string) int {
			if code := serveWall(ctx, h, http.MethodPost, "/board/spec/"+wallFixtureName+"/api/git-switch", `{"branch":"main"}`); code != http.StatusConflict {
				return code
			}
			return http.StatusOK
		}},
		{name: "the MCP get_board projection (LoadProjection)", call: func(ctx context.Context, _ http.Handler, root string) int {
			if _, _, err := LoadProjection(ctx, root, wallFixtureName, nil, "", nil); err != nil {
				return http.StatusInternalServerError
			}
			return http.StatusOK
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := newWallChangesFixture(t, wallChangesHeadSpec)
			writeWallFile(t, root, "notes.txt", "scratch\n")
			rec := &wallGitRecorder{}
			ctx := gitx.WithObserver(context.Background(), rec)
			if code := tc.call(ctx, NewHandler(root), root); code != http.StatusOK {
				t.Fatalf("call = %d, want 200", code)
			}
			if got := rec.ranSummary(); got != tc.wantPays {
				t.Fatalf("computed the changes summary = %v, want %v", got, tc.wantPays)
			}
		})
	}
}

func serveWall(ctx context.Context, h http.Handler, method, target, body string) int {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, strings.NewReader(body)).WithContext(ctx))
	return rec.Code
}

// TestWallChanges_UnbornHeadLeavesOtherCallersWhole proves an unborn HEAD —
// the summary's missing-at-HEAD state — never fails a caller that does not
// compute the summary: the MCP get_board projection still loads.
func TestWallChanges_UnbornHeadLeavesOtherCallersWhole(t *testing.T) {
	root := newWallChangesFixture(t, wallChangesHeadSpec)
	runWallGit(t, root, "checkout", "-q", "--orphan", "design/unborn")
	if _, _, err := LoadProjection(context.Background(), root, wallFixtureName, nil, "", nil); err != nil {
		t.Fatalf("LoadProjection over an unborn HEAD: %v", err)
	}
}

// TestWallChanges_ComputePerformsNoGitWrite is co-1: computing the summary
// writes nothing to git. A stale index is the case that matters: a plain
// `git status` refreshes it and writes it back.
func TestWallChanges_ComputePerformsNoGitWrite(t *testing.T) {
	root := newWallChangesFixture(t, wallChangesHeadSpec)
	writeWallFile(t, root, "notes.txt", "scratch\n")
	stale := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(filepath.Join(root, "README-fixture.md"), stale, stale); err != nil {
		t.Fatal(err)
	}
	indexPath := filepath.Join(root, ".git", "index")
	before, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := computeWallChanges(context.Background(), root, wallFixtureName, []byte(wallChangesHeadSpec)); err != nil {
		t.Fatalf("computeWallChanges: %v", err)
	}
	after, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("computing the changes summary rewrote .git/index (co-1: no git write)")
	}
}

// TestWallChanges_ComputeOutsideARepository is computeWallChanges'
// operational failure: no repository is an error, never an empty summary.
func TestWallChanges_ComputeOutsideARepository(t *testing.T) {
	got, err := computeWallChanges(context.Background(), t.TempDir(), wallFixtureName, []byte(wallChangesHeadSpec))
	if err == nil {
		t.Fatalf("computeWallChanges outside a repository = %+v, want an error", got)
	}
	if !strings.Contains(err.Error(), "wall changes for "+wallFixtureName) {
		t.Fatalf("error %q does not name the spec", err)
	}
}
