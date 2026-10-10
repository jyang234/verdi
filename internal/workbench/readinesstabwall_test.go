package workbench

// Walls served through the production readiness loader, read through
// their record drawer's Readiness tab. The retired wall shell's tests are
// repointed here in place (spec/wall-strip-and-drawer-v2 ac-7, dc-4;
// SI-368 (32) T1): every claim they made about a family the shell and the
// loader both produce is asserted on the tab, which renders the loader's
// facts in the loader's prose.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	stdhtml "html"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/jyang234/verdi/internal/readinessload"
	"github.com/jyang234/verdi/internal/readinesspilot"
)

// teeLoader passes every load to the production loader and records an
// independent copy of the snapshot each returned, taken before the tab
// renders it, so a test compares the tab with the exact value its one
// load produced and never with the object the tab rendered from: a tab
// that rewrote that object in place cannot rewrite the copy (F3G3R-2).
type teeLoader struct {
	mu    sync.Mutex
	inner ReadinessLoader
	snaps []readinesspilot.Snapshot
	errs  []error
}

func (l *teeLoader) Load(ctx context.Context, ref string) (readinesspilot.Snapshot, error) {
	snap, err := l.inner.Load(ctx, ref)
	kept, copyErr := independentSnapshot(snap)
	l.mu.Lock()
	defer l.mu.Unlock()
	l.snaps = append(l.snaps, kept)
	l.errs = append(l.errs, errors.Join(err, copyErr))
	return snap, err
}

// independentSnapshot is a deep copy of snap that shares no memory with
// it: a JSON round trip, which every field of the snapshot survives — the
// copy is checked equal to snap on the spot, so a field the round trip
// lost would fail the load rather than drop out of the comparison.
func independentSnapshot(snap readinesspilot.Snapshot) (readinesspilot.Snapshot, error) {
	raw, err := json.Marshal(snap)
	if err != nil {
		return readinesspilot.Snapshot{}, fmt.Errorf("copying the snapshot: %w", err)
	}
	var kept readinesspilot.Snapshot
	if err := json.Unmarshal(raw, &kept); err != nil {
		return readinesspilot.Snapshot{}, fmt.Errorf("copying the snapshot: %w", err)
	}
	if !reflect.DeepEqual(kept, snap) {
		return readinesspilot.Snapshot{}, errors.New("copying the snapshot: the JSON round trip changed it")
	}
	return kept, nil
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

// tab GETs the wall's Readiness tab and returns its body and the tee's
// independent copy of the snapshot its one readiness load returned; a tab
// that is unavailable, or that loads other than once, fails the test.
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

// tabRow is one concern row's facts as the Readiness tab renders them:
// its state and step, the step's label (Stage) and, on a row that waits,
// the label of the step it waits on (WaitsOn); the plain Human-review
// label with its formal id and work class ("" on a row without one) and
// the row's human-review mark; the primary line, the formal facts, the
// work class, the wall target, the witnesses, the destination as the
// Technical details list it, and the CLI fallback's tokens.
type tabRow struct {
	State, Area, Stage, WaitsOn, HumanReview, ReviewMark           string
	Primary, Fact, Blocking, Timing, WorkClass, TargetKind, Target string
	Witnesses, Destination, CLI                                    []string
}

var (
	tabRowState       = regexp.MustCompile(`readiness-concern--(proven|violated-with-witness|unproven)`)
	tabRowArea        = regexp.MustCompile(`data-area-id="([^"]*)"`)
	tabRowStage       = regexp.MustCompile(`<p class="readiness-stage">([^<]*?)(?: <span class="readiness-when readiness-when--(?:now|later)">(?:now|later — waits on ([^<]*))</span>)?</p>`)
	tabRowHumanReview = regexp.MustCompile(`<p class="readiness-human-review" data-testid="readiness-human-review">Human review<span class="readiness-human-review-formal"> · <code>([^<]*)</code>(?: · <code>([^<]*)</code>)?</span></p>`)
	tabRowPrimary     = regexp.MustCompile(`<div class="readiness-primary"><p class="readiness-summary[^"]*">(.*?)</p>`)
	tabRowFact        = regexp.MustCompile(`<dd class="readiness-fact">(.*?)</dd>`)
	tabRowBlocking    = regexp.MustCompile(`<dt>Blocking</dt><dd><code>(.*?)</code></dd>`)
	tabRowTiming      = regexp.MustCompile(`<dt>Timing</dt><dd><code>(.*?)</code></dd>`)
	tabRowWorkClass   = regexp.MustCompile(`<dt>Work class</dt><dd><code>(.*?)</code></dd>`)
	tabRowTarget      = regexp.MustCompile(`data-target-kind="([^"]*)"(?: data-target="([^"]*)")?`)
	tabRowWitnesses   = regexp.MustCompile(`<ul class="readiness-witnesses">(.*?)</ul>`)
	tabRowWitness     = regexp.MustCompile(`<li><code>(.*?)</code></li>`)
	tabRowDestination = regexp.MustCompile(`<dt>Destination</dt><dd>(.*?)</dd>`)
	tabRowCode        = regexp.MustCompile(`<code>(.*?)</code>`)
	tabRowCLI         = regexp.MustCompile(`<p class="readiness-dest readiness-cli"[^>]*>(.*?)</p>`)
	tabRowCLIToken    = regexp.MustCompile(`<code class="readiness-cli-token">(.*?)</code>`)
)

// tabRowTexts is every first group re finds in s, unescaped; nil for none.
func tabRowTexts(re *regexp.Regexp, s string) []string {
	var out []string
	for _, m := range re.FindAllStringSubmatch(s, -1) {
		out = append(out, stdhtml.UnescapeString(m[1]))
	}
	return out
}

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
		Blocking: first(tabRowBlocking), Timing: first(tabRowTiming), WorkClass: first(tabRowWorkClass),
	}
	if m := tabRowStage.FindStringSubmatch(row); m != nil {
		r.Stage, r.WaitsOn = stdhtml.UnescapeString(m[1]), stdhtml.UnescapeString(m[2])
	}
	switch m := tabRowHumanReview.FindStringSubmatch(row); {
	case m != nil:
		r.HumanReview = "Human review · " + stdhtml.UnescapeString(m[1])
		if m[2] != "" {
			r.HumanReview += " · " + stdhtml.UnescapeString(m[2])
		}
	case strings.Contains(row, `data-testid="readiness-human-review"`):
		r.HumanReview = "(a Human-review label this parser cannot read)"
	}
	r.ReviewMark = strconv.FormatBool(strings.Contains(row[:strings.Index(row, ">")], " readiness-concern--human-review"))
	if m := tabRowTarget.FindStringSubmatch(row); m != nil {
		r.TargetKind, r.Target = m[1], stdhtml.UnescapeString(m[2])
	}
	if m := tabRowWitnesses.FindStringSubmatch(row); m != nil {
		r.Witnesses = tabRowTexts(tabRowWitness, m[1])
	}
	if m := tabRowDestination.FindStringSubmatch(row); m != nil {
		r.Destination = tabRowTexts(tabRowCode, m[1])
	}
	if m := tabRowCLI.FindStringSubmatch(row); m != nil {
		r.CLI = tabRowTexts(tabRowCLIToken, m[1])
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
