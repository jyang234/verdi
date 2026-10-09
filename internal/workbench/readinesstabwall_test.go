package workbench

// Walls served through the production readiness loader, read through
// their record drawer's Readiness tab. The retired wall shell's tests are
// repointed here in place (spec/wall-strip-and-drawer-v2 ac-7, dc-4;
// SI-368 (32) T1): every claim they made about a family the shell and the
// loader both produce is asserted on the tab, which renders the loader's
// facts in the loader's prose.

import (
	"context"
	stdhtml "html"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/jyang234/verdi/internal/readinessload"
	"github.com/jyang234/verdi/internal/readinesspilot"
)

// teeLoader passes every load to the production loader and records the
// snapshot each returned, so a test compares the tab with the exact value
// its one load produced.
type teeLoader struct {
	mu    sync.Mutex
	inner ReadinessLoader
	snaps []readinesspilot.Snapshot
	errs  []error
}

func (l *teeLoader) Load(ctx context.Context, ref string) (readinesspilot.Snapshot, error) {
	snap, err := l.inner.Load(ctx, ref)
	l.mu.Lock()
	defer l.mu.Unlock()
	l.snaps = append(l.snaps, snap)
	l.errs = append(l.errs, err)
	return snap, err
}

// loads is how many loads the tee has recorded.
func (l *teeLoader) loads() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.snaps)
}

// last is the most recent load's outcome.
func (l *teeLoader) last() (readinesspilot.Snapshot, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.snaps[len(l.snaps)-1], l.errs[len(l.errs)-1]
}

// tabWall is one authoring wall on design/<name>, served with the
// production readiness loader behind a tee and the design bridge wired,
// as `verdi serve` serves it.
type tabWall struct {
	name   string
	root   string
	h      http.Handler
	loader *teeLoader
}

// newTabWall builds the wall of spec name from its spec.md and any extra
// draft-branch files, beside the store's manifest the loader opens.
func newTabWall(t *testing.T, name, spec string, extra map[string]string) *tabWall {
	t.Helper()
	draft := map[string]string{".verdi/specs/active/" + name + "/spec.md": spec}
	for path, body := range extra {
		draft[path] = body
	}
	root := buildAuthoringFixture(t, "design/"+name,
		map[string]string{".verdi/.gitignore": "data/\n", ".verdi/verdi.yaml": "schema: verdi.layout/v1\n"}, draft)
	tee := &teeLoader{inner: readinessload.Loader{Root: root, Opts: readinessload.Options{BoardHref: BranchBoardHref}}}
	return &tabWall{name: name, root: root, h: NewHandlerWith(root, Deps{Design: readinessGapCapsBridge(), ReadinessLoader: tee}), loader: tee}
}

// tab GETs the wall's Readiness tab and returns its body and the snapshot
// its one readiness load returned; a tab that is unavailable, or that
// loads other than once, fails the test.
func (w *tabWall) tab(t *testing.T) (string, readinesspilot.Snapshot) {
	t.Helper()
	before := w.loader.loads()
	rec := tabGet(t, t.Context(), w.h, "/board/spec/"+w.name+"/readiness")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s's tab = %d\n%s", w.name, rec.Code, rec.Body.String())
	}
	if got := w.loader.loads() - before; got != 1 {
		t.Fatalf("%s's tab loaded readiness %d times, want once", w.name, got)
	}
	snap, err := w.loader.last()
	if err != nil {
		t.Fatalf("%s's readiness load failed: %v", w.name, err)
	}
	body := rec.Body.String()
	if strings.Contains(body, "data-readiness-unavailable") {
		t.Fatalf("%s's tab is unavailable:\n%s", w.name, body)
	}
	return body, snap
}

// postSticky leaves one open scratch sticky of kind on the wall through
// its own route, as a browser does.
func (w *tabWall) postSticky(t *testing.T, kind, text string) {
	t.Helper()
	rec := httptest.NewRecorder()
	w.h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/board/spec/"+w.name+"/api/sticky", strings.NewReader(`{"text":"`+text+`","type":"`+kind+`"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("posting a sticky on %s = %d\n%s", w.name, rec.Code, rec.Body.String())
	}
}

// tabRow is one concern row's facts as the Readiness tab renders them.
type tabRow struct {
	State, Area, Primary, Fact, Blocking, Timing, TargetKind, Target string
	Witnesses                                                        []string
}

var (
	tabRowState     = regexp.MustCompile(`readiness-concern--(proven|violated-with-witness|unproven)`)
	tabRowArea      = regexp.MustCompile(`data-area-id="([^"]*)"`)
	tabRowPrimary   = regexp.MustCompile(`<div class="readiness-primary"><p class="readiness-summary[^"]*">(.*?)</p>`)
	tabRowFact      = regexp.MustCompile(`<dd class="readiness-fact">(.*?)</dd>`)
	tabRowBlocking  = regexp.MustCompile(`<dt>Blocking</dt><dd><code>(.*?)</code></dd>`)
	tabRowTiming    = regexp.MustCompile(`<dt>Timing</dt><dd><code>(.*?)</code></dd>`)
	tabRowTarget    = regexp.MustCompile(`data-target-kind="([^"]*)"(?: data-target="([^"]*)")?`)
	tabRowWitnesses = regexp.MustCompile(`<ul class="readiness-witnesses">(.*?)</ul>`)
	tabRowWitness   = regexp.MustCompile(`<li><code>(.*?)</code></li>`)
)

// readTabRow parses concern id's one row out of the tab body.
func readTabRow(t *testing.T, tab, id string) tabRow {
	t.Helper()
	if n := strings.Count(tab, `data-concern-id="`+stdhtml.EscapeString(id)+`"`); n != 1 {
		t.Fatalf("concern %s renders %d times in the tab, want once", id, n)
	}
	row := concernRow(t, tab, stdhtml.EscapeString(id))
	first := func(re *regexp.Regexp) string {
		m := re.FindStringSubmatch(row)
		if m == nil {
			return ""
		}
		return stdhtml.UnescapeString(m[1])
	}
	r := tabRow{
		State: first(tabRowState), Area: first(tabRowArea), Primary: first(tabRowPrimary), Fact: first(tabRowFact),
		Blocking: first(tabRowBlocking), Timing: first(tabRowTiming),
	}
	if m := tabRowTarget.FindStringSubmatch(row); m != nil {
		r.TargetKind, r.Target = m[1], stdhtml.UnescapeString(m[2])
	}
	if m := tabRowWitnesses.FindStringSubmatch(row); m != nil {
		for _, w := range tabRowWitness.FindAllStringSubmatch(m[1], -1) {
			r.Witnesses = append(r.Witnesses, stdhtml.UnescapeString(w[1]))
		}
	}
	return r
}

// tabConcernIDs is every concern id the tab renders, in its order.
func tabConcernIDs(tab string) []string {
	var ids []string
	for _, m := range regexp.MustCompile(`data-concern-id="([^"]+)"`).FindAllStringSubmatch(tab, -1) {
		ids = append(ids, stdhtml.UnescapeString(m[1]))
	}
	return ids
}

// snapConcern is concern id of snap, or a failed test.
func snapConcern(t *testing.T, snap readinesspilot.Snapshot, id string) readinesspilot.Concern {
	t.Helper()
	for _, c := range snap.AllConcerns {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("the loader's snapshot has no concern %s", id)
	return readinesspilot.Concern{}
}
