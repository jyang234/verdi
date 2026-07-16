package workbench

import (
	"errors"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/disclosure"
	"github.com/jyang234/verdi/internal/refindex"
)

// glanceFixtureEntries spans spec/home-status-glance's whole population
// rule in one store: an ordinary design-branch draft (on-the-desk) and a
// disclosed one (dc-1: glance-excluded), a default-branch feature building
// entry with a story ref (in-flight; the matrix/verdict-bearing case), a
// default-branch active component and a default-branch active-zone
// superseded component (both settling, ADJ-36's total-partition reading),
// a default-branch entry that is glance-eligible (active zone) yet carries
// no working-tree file at all (ADJ-35's residual truth-source divergence:
// still shown, board link honestly withheld), and an ARCHIVE-zone terminal
// entry (dc-2/ADJ-32 f1: excluded from the glance entirely, yet still
// owed to the exhaustive Directory section by ac-2's no-loss bar).
func glanceFixtureEntries() []refindex.Entry {
	noDraft := disclosure.New("refindex:no-draft-spec", "spec/blank-branch",
		`design branch "design/blank-branch" resolves but has no spec.md yet`)
	return []refindex.Entry{
		{Ref: "spec/glance-draft", Source: refindex.SourceLocal, StatusGroup: refindex.StatusGroupDraftsInProgress, SpecStatus: "draft", Zone: refindex.ZoneActive},
		{Ref: "spec/blank-branch", Source: refindex.SourceLocal, StatusGroup: refindex.StatusGroupDraftsInProgress, Disclosed: &noDraft, Zone: refindex.ZoneActive},
		{Ref: "spec/glance-building", Source: refindex.SourceDefault, StatusGroup: refindex.StatusGroupAcceptedPendingBuild, SpecStatus: "accepted-pending-build", Zone: refindex.ZoneActive},
		{Ref: "spec/glance-component", Source: refindex.SourceDefault, StatusGroup: refindex.StatusGroupActiveComponents, SpecStatus: "active", Zone: refindex.ZoneActive},
		{Ref: "spec/glance-superseded", Source: refindex.SourceDefault, StatusGroup: refindex.StatusGroupTerminal, SpecStatus: "superseded", Zone: refindex.ZoneActive},
		{Ref: "spec/glance-orphaned", Source: refindex.SourceDefault, StatusGroup: refindex.StatusGroupTerminal, SpecStatus: "superseded", Zone: refindex.ZoneActive},
		{Ref: "spec/glance-archived", Source: refindex.SourceDefault, StatusGroup: refindex.StatusGroupTerminal, SpecStatus: "closed", Zone: refindex.ZoneArchive},
	}
}

// glanceEntryBlock cuts one glance-entry <li> block out of the rendered
// body (entryBlock's sibling, for the leaner glance markup).
func glanceEntryBlock(t *testing.T, body, name string) string {
	t.Helper()
	re := regexp.MustCompile(`(?s)<li class="glance-entry" data-testid="glance-entry-` + regexp.QuoteMeta(name) + `".*?</li>`)
	m := re.FindString(body)
	if m == "" {
		t.Fatalf("no glance-entry block for %s in: %s", name, body)
	}
	return m
}

// glanceGroupBlock cuts one glance-group <section> block out of the
// rendered body — safe as a non-greedy match since a bucket never nests a
// second <section> inside it.
func glanceGroupBlock(t *testing.T, body, slug string) string {
	t.Helper()
	re := regexp.MustCompile(`(?s)<section class="glance-group" data-testid="glance-group-` + regexp.QuoteMeta(slug) + `".*?</section>`)
	m := re.FindString(body)
	if m == "" {
		t.Fatalf("no glance-group block for %s in: %s", slug, body)
	}
	return m
}

// glanceSectionHTML extracts just the glance's own rendered HTML (dc-5:
// it renders immediately before the exhaustive Directory section), so an
// absence assertion (no source chip, no in-review chip) cannot be a false
// negative from the exhaustive section immediately following it, which
// legitimately carries both.
func glanceSectionHTML(t *testing.T, body string) string {
	t.Helper()
	start := strings.Index(body, `data-testid="home-glance"`)
	if start < 0 {
		t.Fatalf("no home-glance section in: %s", body)
	}
	end := strings.Index(body, `class="home-directory"`)
	if end < 0 || end < start {
		t.Fatalf("home-directory section not found after home-glance in: %s", body)
	}
	return body[start:end]
}

// TestGlanceEligibleEntries is dc-1/dc-2's population rule as a pure,
// table-driven unit (happy + negative rows), independent of rendering.
func TestGlanceEligibleEntries(t *testing.T) {
	disclosed := disclosure.New("refindex:no-draft-spec", "spec/x", "no spec.md yet")
	tests := []struct {
		name string
		e    refindex.Entry
		want bool
	}{
		{"a default active-zone entry is eligible", refindex.Entry{Ref: "spec/a", Source: refindex.SourceDefault, Zone: refindex.ZoneActive}, true},
		{"a default archive-zone entry is excluded (dc-2, ADJ-32 f1)", refindex.Entry{Ref: "spec/b", Source: refindex.SourceDefault, Zone: refindex.ZoneArchive}, false},
		{"an ordinary design-branch entry is eligible", refindex.Entry{Ref: "spec/c", Source: refindex.SourceLocal, Zone: refindex.ZoneActive}, true},
		{"a disclosed design-branch entry is excluded regardless of zone (dc-1)", refindex.Entry{Ref: "spec/d", Source: refindex.SourceLocal, Zone: refindex.ZoneActive, Disclosed: &disclosed}, false},
		{"an entry with an unset/unrecognized zone fails closed (excluded)", refindex.Entry{Ref: "spec/e", Source: refindex.SourceDefault}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := glanceEligibleEntries([]refindex.Entry{tt.e})
			gotIn := len(got) == 1
			if gotIn != tt.want {
				t.Fatalf("eligible = %v, want %v (entries: %+v)", gotIn, tt.want, got)
			}
		})
	}
}

// TestGlanceBuckets_TotalPartition proves ADJ-36's total-partition reading
// directly against refindex's closed, four-value StatusGroup vocabulary:
// every one of the four values lands in EXACTLY one bucket — never zero,
// never two.
func TestGlanceBuckets_TotalPartition(t *testing.T) {
	tests := []struct {
		group    refindex.StatusGroup
		wantSlug string
	}{
		{refindex.StatusGroupDraftsInProgress, "on-the-desk"},
		{refindex.StatusGroupAcceptedPendingBuild, "in-flight"},
		{refindex.StatusGroupActiveComponents, "settling"},
		{refindex.StatusGroupTerminal, "settling"},
	}
	for _, tt := range tests {
		t.Run(string(tt.group), func(t *testing.T) {
			var matched []string
			for _, b := range glanceBuckets {
				if b.member(tt.group) {
					matched = append(matched, b.slug)
				}
			}
			if len(matched) != 1 || matched[0] != tt.wantSlug {
				t.Fatalf("StatusGroup %q matched buckets %v, want exactly [%q]", tt.group, matched, tt.wantSlug)
			}
		})
	}
}

// TestWriteGlanceSection_BucketsOrderMembershipBadgesLinks is ac-1's full
// render witness: three fixed-order buckets, every eligible entry exactly
// once under its correct bucket, real status badges, dc-3's link grammar
// (mirrored from directory.go's own address helpers, never re-derived),
// matrix/verdict gated to a default-branch feature with a story ref, no
// source/in-review chip anywhere in the glance, dc-1's disclosed-entry
// exclusion, dc-2's archive-zone exclusion paired with ac-2's no-loss bar,
// and ADJ-35's honest-degradation residual (a glance-eligible entry absent
// from the serving working tree).
func TestWriteGlanceSection_BucketsOrderMembershipBadgesLinks(t *testing.T) {
	root := t.TempDir()
	writeActiveSpec(t, root, "glance-building", "feature", "accepted-pending-build", "jira:GL-1")
	writeActiveSpec(t, root, "glance-component", "component", "active", "")
	writeActiveSpec(t, root, "glance-superseded", "component", "superseded", "")
	// glance-orphaned and glance-archived deliberately get NO working-tree
	// file: the former proves ADJ-35's honest degradation (glance-eligible
	// by the index, but the serving checkout lacks the file); the latter
	// is excluded from the glance outright so its own working-tree state
	// is moot.

	code, body := getHome(t, root, HomeDeps{
		Index:   cannedIndex(glanceFixtureEntries(), nil),
		Git:     fakeHomeGit{},
		OpenMRs: fakeOpenMRs{branches: []string{"design/glance-draft"}},
	})
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", code, body)
	}

	// dc-5: the glance renders before the exhaustive Directory section.
	glanceAt := strings.Index(body, `data-testid="home-glance"`)
	dirAt := strings.Index(body, `class="home-directory"`)
	if glanceAt < 0 {
		t.Fatalf("home-glance section missing; got: %s", body)
	}
	if dirAt < 0 || dirAt < glanceAt {
		t.Fatalf("home-glance must render BEFORE the exhaustive Directory section (dc-5); got: %s", body)
	}

	// ac-1: three buckets, fixed order.
	order := []string{
		`data-testid="glance-group-on-the-desk"`,
		`data-testid="glance-group-in-flight"`,
		`data-testid="glance-group-settling"`,
	}
	last := -1
	for _, marker := range order {
		i := strings.Index(body, marker)
		if i < 0 {
			t.Fatalf("home missing glance group %s; got: %s", marker, body)
		}
		if i < last {
			t.Fatalf("glance group %s renders out of dc-2/ADJ-36 order", marker)
		}
		last = i
	}

	// Every eligible entry appears EXACTLY ONCE, under its correct bucket.
	cases := []struct{ name, group string }{
		{"glance-draft", "on-the-desk"},
		{"glance-building", "in-flight"},
		{"glance-component", "settling"},
		{"glance-superseded", "settling"},
		{"glance-orphaned", "settling"},
	}
	for _, tc := range cases {
		if n := strings.Count(body, `data-testid="glance-entry-`+tc.name+`"`); n != 1 {
			t.Fatalf("glance entry %s appears %d times, want exactly 1; got: %s", tc.name, n, body)
		}
		grp := glanceGroupBlock(t, body, tc.group)
		if !strings.Contains(grp, `data-testid="glance-entry-`+tc.name+`"`) {
			t.Fatalf("glance entry %s is not inside bucket %s; got group: %s", tc.name, tc.group, grp)
		}
	}

	// dc-1: a disclosed (no-draft-spec) design-branch entry never appears.
	if strings.Contains(body, `data-testid="glance-entry-blank-branch"`) {
		t.Fatalf("a disclosed design-branch entry must not appear in the glance (dc-1); got: %s", body)
	}

	// dc-2 (ADJ-32 f1 sustained): an archive-zone entry is excluded from
	// the glance entirely...
	if strings.Contains(body, `data-testid="glance-entry-glance-archived"`) {
		t.Fatalf("an archive-zone entry must be excluded from the glance (dc-2); got: %s", body)
	}
	// ...yet ac-2's no-loss bar holds: it is STILL present, unchanged, in
	// the exhaustive Directory section — the regression proof that
	// directory.go (untouched by this story) renders byte-identically
	// regardless of the new Zone field.
	if !strings.Contains(body, `data-testid="dir-entry-glance-archived"`) {
		t.Fatalf("archive-zone entry must still render in the exhaustive Directory section (ac-2 no-loss); got: %s", body)
	}
	if !strings.Contains(entryBlock(t, body, "glance-archived"), `badge-closed`) {
		t.Fatalf("archive-zone entry's exhaustive rendering lost its status chip; got: %s", body)
	}

	// dc-3: status badges read the real raw status.
	for _, tc := range []struct{ name, chip string }{
		{"glance-draft", `badge-draft`},
		{"glance-building", `badge-accepted-pending-build`},
		{"glance-component", `badge-active`},
		{"glance-superseded", `badge-superseded`},
		{"glance-orphaned", `badge-superseded`},
	} {
		if !strings.Contains(glanceEntryBlock(t, body, tc.name), tc.chip) {
			t.Fatalf("glance entry %s missing status chip %s; got: %s", tc.name, tc.chip, body)
		}
	}

	// dc-3: working links. The design-branch draft's title IS its one
	// link, via the /b/ per-branch grammar.
	if !strings.Contains(glanceEntryBlock(t, body, "glance-draft"), `href="/b/design%2Fglance-draft/board/spec/glance-draft"`) {
		t.Fatalf("design-branch glance entry missing its /b/ escaped board link; got: %s", body)
	}
	// The feature entry gets board + matrix + verdict (class:feature, a
	// non-empty story field — dc-3's exact condition).
	building := glanceEntryBlock(t, body, "glance-building")
	for _, href := range []string{
		`href="/board/spec/glance-building"`,
		`href="/matrix/jira:GL-1"`,
		`href="/verdict/jira:GL-1"`,
	} {
		if !strings.Contains(building, href) {
			t.Fatalf("feature glance entry missing %s; got: %s", href, building)
		}
	}
	// The component entries get a board link but NEVER matrix/verdict.
	component := glanceEntryBlock(t, body, "glance-component")
	if !strings.Contains(component, `href="/board/spec/glance-component"`) {
		t.Fatalf("component glance entry missing its board link; got: %s", component)
	}
	if strings.Contains(component, "/matrix/") || strings.Contains(component, "/verdict/") {
		t.Fatalf("a component entry must never carry matrix/verdict links; got: %s", component)
	}

	// ADJ-35: glance-orphaned is glance-eligible (active zone, per the
	// index) yet the serving working tree carries no file for it — the
	// board link is honestly withheld, never broken; title falls back to
	// the bare ref, exactly as the exhaustive section already degrades.
	orphaned := glanceEntryBlock(t, body, "glance-orphaned")
	if strings.Contains(orphaned, "/board/spec/glance-orphaned") {
		t.Fatalf("a working-tree-absent glance entry must not emit a board link (ADJ-35); got: %s", orphaned)
	}
	if !strings.Contains(orphaned, "spec/glance-orphaned") {
		t.Fatalf("working-tree-absent glance entry should still name its ref as a fallback title; got: %s", orphaned)
	}

	// dc-3: the in-review chip is a real, available second source (the
	// directory entry for glance-draft still carries it) — proving its
	// absence from the glance is a deliberate omission, not a vacuous one.
	if !strings.Contains(entryBlock(t, body, "glance-draft"), "in review") {
		t.Fatalf("directory entry for glance-draft should still show its in-review chip; got: %s", body)
	}
	if strings.Contains(glanceEntryBlock(t, body, "glance-draft"), "in review") {
		t.Fatalf("glance entry for glance-draft must NOT show an in-review chip (dc-3); got: %s", body)
	}

	// dc-3: lean cards — no source chip, no in-review chip anywhere in the
	// glance section as a whole (scoped so the exhaustive section's own
	// legitimate use of both cannot produce a false negative).
	glance := glanceSectionHTML(t, body)
	if strings.Contains(glance, "badge-src") {
		t.Fatalf("the glance must never carry a source chip (dc-3); got: %s", glance)
	}
	if strings.Contains(glance, "dir-inreview") {
		t.Fatalf("the glance must never carry an in-review chip (dc-3); got: %s", glance)
	}
}

// TestGlanceLinks_MirrorDirectoryExactly is dc-3's "never a third grammar"
// obligation, proven by cross-derivation rather than a hard-coded string
// compared against itself twice: the glance and the exhaustive Directory
// section render the IDENTICAL href for the SAME entry.
func TestGlanceLinks_MirrorDirectoryExactly(t *testing.T) {
	root := t.TempDir()
	writeActiveSpec(t, root, "glance-building", "feature", "accepted-pending-build", "jira:GL-1")

	_, body := getHome(t, root, HomeDeps{
		Index: cannedIndex(glanceFixtureEntries(), nil),
		Git:   fakeHomeGit{},
	})

	dir := entryBlock(t, body, "glance-building")
	glance := glanceEntryBlock(t, body, "glance-building")
	for _, href := range []string{
		`href="/board/spec/glance-building"`,
		`href="/matrix/jira:GL-1"`,
		`href="/verdict/jira:GL-1"`,
	} {
		if !strings.Contains(dir, href) {
			t.Fatalf("directory entry missing %s; got: %s", href, dir)
		}
		if !strings.Contains(glance, href) {
			t.Fatalf("glance entry missing the SAME %s the directory renders (a third grammar?); got: %s", href, glance)
		}
	}
}

// TestWriteGlanceSection_EmptyBucketRendersHeadingCountAndNotice is ac-3/
// dc-4's obligation: a bucket with zero matching entries still renders
// its heading, its "(0)" count, and the same "None." empty-state notice
// the exhaustive Directory section's own empty-group precedent uses —
// never silently omitted from the DOM.
func TestWriteGlanceSection_EmptyBucketRendersHeadingCountAndNotice(t *testing.T) {
	root := t.TempDir()
	entries := []refindex.Entry{
		{Ref: "spec/glance-draft", Source: refindex.SourceLocal, StatusGroup: refindex.StatusGroupDraftsInProgress, SpecStatus: "draft", Zone: refindex.ZoneActive},
		{Ref: "spec/glance-component", Source: refindex.SourceDefault, StatusGroup: refindex.StatusGroupActiveComponents, SpecStatus: "active", Zone: refindex.ZoneActive},
	}
	_, body := getHome(t, root, HomeDeps{
		Index: cannedIndex(entries, nil),
		Git:   fakeHomeGit{},
	})

	group := glanceGroupBlock(t, body, "in-flight")
	if !strings.Contains(group, `data-testid="glance-group-in-flight"`) {
		t.Fatalf("empty bucket must still render its heading, never vanish; got: %s", group)
	}
	if !strings.Contains(group, "(0)") {
		t.Fatalf("empty in-flight bucket missing its zero count; got: %s", group)
	}
	if !strings.Contains(group, `<p class="empty">None.</p>`) {
		t.Fatalf("empty in-flight bucket missing the None. empty-state notice (mirroring directory.go's own precedent); got: %s", group)
	}
}

// TestWriteGlanceSection_IndexFailure_RendersNothing is CO-2's negative
// path: on the SAME indexErr renderHome's exhaustive section already
// discloses, the glance renders no section at all — never a second,
// contradictory notice, and never a fabricated or partial bucket set.
func TestWriteGlanceSection_IndexFailure_RendersNothing(t *testing.T) {
	root := t.TempDir()
	code, body := getHome(t, root, HomeDeps{
		Index: cannedIndex(nil, errors.New("refindex: default-branch walk: boom")),
		Git:   fakeHomeGit{},
	})
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (the home page is never a dead end)", code)
	}
	if strings.Contains(body, `data-testid="home-glance"`) {
		t.Fatalf("CO-2: the glance must render nothing (never a second, contradictory notice, never a fabricated bucket) when the shared index computation failed; got: %s", body)
	}
	if !strings.Contains(body, "Could not compute the directory index") || !strings.Contains(body, "boom") {
		t.Fatalf("the exhaustive section must still disclose the SAME indexErr inline; got: %s", body)
	}
}
