package specdoc

import (
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/objsupersede"
)

// The closed-spec object supersession lines a document adds beside an
// object (design §6; SI-263, SI-279): Build converts the objsupersede
// views a consumer supplied into verbatim lines with the refs they name
// linked, RenderMarkdown adds markup only, and a nil supply renders every
// object exactly as before (the goldens in markdown_test.go prove the
// byte-identity; this file proves what a supply adds).

const (
	tsBy       = "spec/s1#dc-1"
	tsConflict = "conflict/c1"
	tsRevision = "spec/s2"
	tsObject   = "spec/lockbox#ac-1"
)

func tsLinks() map[string]string {
	return map[string]string{
		tsBy:       "/a/spec/s1/document/#dc-1",
		tsConflict: "/a/conflict/c1/",
		tsRevision: "/a/spec/s2/document/",
		tsObject:   "/a/spec/lockbox/document/#ac-1",
		"spec/s1":  "/a/spec/s1/document/",
	}
}

func tsSuperseded() objsupersede.ObjectView {
	return objsupersede.ObjectView{
		Object: tsObject, State: objsupersede.ObjectSuperseded,
		By: tsBy, Conflict: tsConflict, Since: "2024-02-15", Closed: "2024-01-10",
	}
}

func TestBuild_SupersessionObjects(t *testing.T) {
	fm, body := fixtureSpec(t)
	commit := strings.Repeat("0", 39) + "1"
	build := func(t *testing.T, facts *SupersessionFacts) (Document, error) {
		t.Helper()
		return Build(Input{Spec: fm, Body: body, Stamp: Stamp{Ref: "spec/lockbox", Commit: commit}, Facts: Facts{Supersession: facts}, Kind: KindSpec})
	}
	carried := tsSuperseded()
	carried.Carry, carried.Revision = objsupersede.CarryCarried, tsRevision
	dropped := tsSuperseded()
	dropped.Carry, dropped.Revision = objsupersede.CarryDropped, tsRevision
	// The core names the head revision on an unproven carry (views.go's
	// carry sets Revision before the acceptance switch): the disclosure
	// still gets no link.
	unprovenCarry := tsSuperseded()
	unprovenCarry.Carry, unprovenCarry.Revision, unprovenCarry.Witness = objsupersede.CarryUnproven, tsRevision, "spec/s2 is not accepted"
	closedUnproven := tsSuperseded()
	closedUnproven.Closed, closedUnproven.ClosedWitness = "", "shallow history"

	tests := []struct {
		name string
		view objsupersede.ObjectView
		want *Supersession
	}{
		{"superseded, heading its chain", tsSuperseded(), &Supersession{State: "superseded", Lines: []SupersessionLine{
			{Kind: "governed", Text: "governed spec/lockbox's completed work (closed 2024-01-10)"},
			{Kind: "since", Text: "superseded since 2024-02-15 by spec/s1#dc-1", Links: []RefLink{{tsBy, "/a/spec/s1/document/#dc-1"}}, Trailing: []RefLink{{tsConflict, "/a/conflict/c1/"}}},
		}}},
		{"carried by a later revision", carried, &Supersession{State: "superseded", Lines: []SupersessionLine{
			{Kind: "governed", Text: "governed spec/lockbox's completed work (closed 2024-01-10)"},
			{Kind: "since", Text: "superseded since 2024-02-15 by spec/s1#dc-1", Links: []RefLink{{tsBy, "/a/spec/s1/document/#dc-1"}}, Trailing: []RefLink{{tsConflict, "/a/conflict/c1/"}}},
			{Kind: "carry", Text: "carried by spec/s2", Links: []RefLink{{tsRevision, "/a/spec/s2/document/"}}},
		}}},
		{"no longer carried", dropped, &Supersession{State: "superseded", Lines: []SupersessionLine{
			{Kind: "governed", Text: "governed spec/lockbox's completed work (closed 2024-01-10)"},
			{Kind: "since", Text: "superseded since 2024-02-15 by spec/s1#dc-1", Links: []RefLink{{tsBy, "/a/spec/s1/document/#dc-1"}}, Trailing: []RefLink{{tsConflict, "/a/conflict/c1/"}}},
			{Kind: "carry", Text: "no longer carried by the current revision (spec/s2)", Links: []RefLink{{tsRevision, "/a/spec/s2/document/"}}},
		}}},
		{"carrying unproven: the witness, unlinked", unprovenCarry, &Supersession{State: "superseded", Lines: []SupersessionLine{
			{Kind: "governed", Text: "governed spec/lockbox's completed work (closed 2024-01-10)"},
			{Kind: "since", Text: "superseded since 2024-02-15 by spec/s1#dc-1", Links: []RefLink{{tsBy, "/a/spec/s1/document/#dc-1"}}, Trailing: []RefLink{{tsConflict, "/a/conflict/c1/"}}},
			{Kind: "carry", Text: "carrying unproven: spec/s2 is not accepted", Disclosure: true},
		}}},
		{"closed date unproven: the views' text, never a guessed date", closedUnproven, &Supersession{State: "superseded", Lines: []SupersessionLine{
			{Kind: "governed", Text: "governed spec/lockbox's completed work (closed date unproven: shallow history)"},
			{Kind: "since", Text: "superseded since 2024-02-15 by spec/s1#dc-1", Links: []RefLink{{tsBy, "/a/spec/s1/document/#dc-1"}}, Trailing: []RefLink{{tsConflict, "/a/conflict/c1/"}}},
		}}},
		{"supersession unproven", objsupersede.ObjectView{Object: tsObject, State: objsupersede.ObjectUnproven, Witness: "records do not decode: x"},
			&Supersession{State: "unproven", Lines: []SupersessionLine{{Kind: "unproven", Text: "supersession unproven: records do not decode: x", Disclosure: true}}}},
		{"not superseded: nothing added", objsupersede.ObjectView{Object: tsObject, State: objsupersede.ObjectNotSuperseded}, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := build(t, &SupersessionFacts{Objects: map[string]objsupersede.ObjectView{"ac-1": tc.view}, Links: tsLinks()})
			if err != nil {
				t.Fatal(err)
			}
			if got := doc.Criteria[0].Supersession; !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ac-1 supersession:\n got %+v\nwant %+v", got, tc.want)
			}
			if doc.Criteria[1].Supersession != nil || doc.Decisions[0].Supersession != nil || doc.Decisions[0].Supersedes != nil {
				t.Errorf("objects the facts do not name gained a supersession: %+v %+v", doc.Criteria[1], doc.Decisions[0])
			}
		})
	}

	t.Run("a decision object and a ref with no URL", func(t *testing.T) {
		v := tsSuperseded()
		v.Object = "spec/lockbox#dc-1"
		doc, err := build(t, &SupersessionFacts{Objects: map[string]objsupersede.ObjectView{"dc-1": v}, Links: map[string]string{}})
		if err != nil {
			t.Fatal(err)
		}
		// No URL: the deciding decision stays in the text unlinked, and
		// the trailing conflict ref stays, URL-less (SI-280 as amended:
		// only the href is omitted, never the ref).
		want := &Supersession{State: "superseded", Lines: []SupersessionLine{
			{Kind: "governed", Text: "governed spec/lockbox's completed work (closed 2024-01-10)"},
			{Kind: "since", Text: "superseded since 2024-02-15 by spec/s1#dc-1", Trailing: []RefLink{{tsConflict, ""}}},
		}}
		if got := doc.Decisions[0].Supersession; !reflect.DeepEqual(got, want) {
			t.Errorf("dc-1 supersession:\n got %+v\nwant %+v", got, want)
		}
	})

	t.Run("nil facts: nothing added", func(t *testing.T) {
		doc, err := build(t, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range doc.Criteria {
			if c.Supersession != nil {
				t.Errorf("%s gained a supersession with no facts supplied", c.ID)
			}
		}
	})

	for _, tc := range []struct {
		name string
		view objsupersede.ObjectView
	}{
		{"superseded without its deciding decision", objsupersede.ObjectView{Object: tsObject, State: objsupersede.ObjectSuperseded, Conflict: tsConflict, Since: "2024-02-15", Closed: "2024-01-10"}},
		{"unproven without a witness", objsupersede.ObjectView{Object: tsObject, State: objsupersede.ObjectUnproven}},
		{"an unknown state", objsupersede.ObjectView{Object: tsObject, State: "gone"}},
	} {
		t.Run("malformed view fails Build: "+tc.name, func(t *testing.T) {
			if _, err := build(t, &SupersessionFacts{Objects: map[string]objsupersede.ObjectView{"ac-1": tc.view}}); err == nil {
				t.Fatal("Build accepted a malformed view; want an error, never a rendering")
			}
		})
	}
}

func TestBuild_SupersessionDecisions(t *testing.T) {
	fm, body := fixtureSpec(t)
	commit := strings.Repeat("0", 39) + "1"
	build := func(t *testing.T, views []objsupersede.DecisionView) (Document, error) {
		t.Helper()
		return Build(Input{Spec: fm, Body: body, Stamp: Stamp{Ref: "spec/lockbox", Commit: commit}, Kind: KindSpec,
			Facts: Facts{Supersession: &SupersessionFacts{Decisions: map[string][]objsupersede.DecisionView{"dc-1": views}, Links: map[string]string{
				"spec/t#dc-1": "/a/spec/t/document/#dc-1", "spec/s1": "/a/spec/s1/document/", tsConflict: "/a/conflict/c1/",
			}}}})
	}
	inForce := objsupersede.DecisionView{Decision: "spec/lockbox#dc-1", Edge: "spec/t#dc-1", Object: "spec/t#dc-1", State: objsupersede.DecisionInForce, Conflict: tsConflict, Establisher: "spec/lockbox", Since: "2024-02-15"}
	proposed := objsupersede.DecisionView{Decision: "spec/lockbox#dc-1", Edge: "spec/t#dc-1", Object: "spec/t#dc-1", State: objsupersede.DecisionProposed}
	carried := objsupersede.DecisionView{Decision: "spec/lockbox#dc-1", Edge: "spec/t#dc-1", Object: "spec/t#dc-1", State: objsupersede.DecisionInForce, Carried: true, Conflict: tsConflict, Establisher: "spec/s1", Since: "2024-02-15"}
	notEst := objsupersede.DecisionView{Decision: "spec/lockbox#dc-1", Edge: "spec/t#dc-1", Object: "spec/t#dc-1", State: objsupersede.DecisionNotEstablished, Reason: "no conflict challenges spec/t#dc-1"}

	tests := []struct {
		name  string
		views []objsupersede.DecisionView
		want  []Supersession
	}{
		{"in force", []objsupersede.DecisionView{inForce}, []Supersession{{State: "in-force", Object: "spec/t#dc-1", Lines: []SupersessionLine{
			{Kind: "edge", Text: "supersedes spec/t#dc-1", Links: []RefLink{{"spec/t#dc-1", "/a/spec/t/document/#dc-1"}}},
		}}}},
		{"proposed on its design branch", []objsupersede.DecisionView{proposed}, []Supersession{{State: "proposed", Object: "spec/t#dc-1", Lines: []SupersessionLine{
			{Kind: "edge", Text: "proposed — supersedes spec/t#dc-1 when spec/lockbox is accepted", Links: []RefLink{{"spec/t#dc-1", "/a/spec/t/document/#dc-1"}}},
		}}}},
		{"carried: the establishing successor and its conflict linked in the §5 line", []objsupersede.DecisionView{carried}, []Supersession{{State: "in-force", Object: "spec/t#dc-1", Lines: []SupersessionLine{
			{Kind: "edge", Text: "supersedes spec/t#dc-1", Links: []RefLink{{"spec/t#dc-1", "/a/spec/t/document/#dc-1"}}},
			{Kind: "carries", Text: "carries the replacement established by spec/s1 (conflict/c1, since 2024-02-15)", Links: []RefLink{{"spec/s1", "/a/spec/s1/document/"}, {tsConflict, "/a/conflict/c1/"}}},
		}}}},
		{"not established: the reason, never a supersession", []objsupersede.DecisionView{notEst}, []Supersession{{State: "not-established", Object: "spec/t#dc-1", Lines: []SupersessionLine{
			{Kind: "not-established", Text: "supersession not established: no conflict challenges spec/t#dc-1", Disclosure: true},
		}}}},
		{"two edges keep link order", []objsupersede.DecisionView{notEst, inForce}, []Supersession{
			{State: "not-established", Object: "spec/t#dc-1", Lines: []SupersessionLine{{Kind: "not-established", Text: "supersession not established: no conflict challenges spec/t#dc-1", Disclosure: true}}},
			{State: "in-force", Object: "spec/t#dc-1", Lines: []SupersessionLine{{Kind: "edge", Text: "supersedes spec/t#dc-1", Links: []RefLink{{"spec/t#dc-1", "/a/spec/t/document/#dc-1"}}}}},
		}},
		{"no views: nothing added", nil, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := build(t, tc.views)
			if err != nil {
				t.Fatal(err)
			}
			if got := doc.Decisions[0].Supersedes; !reflect.DeepEqual(got, tc.want) {
				t.Errorf("dc-1 supersedes:\n got %+v\nwant %+v", got, tc.want)
			}
		})
	}
	t.Run("a malformed decision view fails Build", func(t *testing.T) {
		if _, err := build(t, []objsupersede.DecisionView{{Decision: "spec/lockbox#dc-1", State: objsupersede.DecisionNotEstablished}}); err == nil {
			t.Fatal("Build accepted a not-established view with no reason")
		}
	})
}

func TestContainsToken(t *testing.T) {
	tests := []struct {
		text, ref string
		want      bool
	}{
		{"superseded since 2024-02-15 by spec/s1#dc-1", "spec/s1#dc-1", true},
		{"superseded since 2024-02-15 by spec/s1#dc-1", "spec/s1", false},
		{"carried by spec/s1-v2", "spec/s1", false},
		{"carried by spec/s1-v2", "spec/s1-v2", true},
		{"established by spec/s1 (conflict/c1, since 2024-02-15)", "spec/s1", true},
		{"established by spec/s1 (conflict/c1, since 2024-02-15)", "conflict/c1", true},
		{"established by spec/s1 (conflict/c10, since 2024-02-15)", "conflict/c1", false},
		{"no longer carried by the current revision (spec/s2)", "spec/s2", true},
		{"the edge spec/t@0a1b2c3#dc-1 is pinned", "spec/t", false},
		{"", "spec/s1", false},
		{"spec/s1", "", false},
	}
	for _, tc := range tests {
		if got := containsToken(tc.text, tc.ref); got != tc.want {
			t.Errorf("containsToken(%q, %q) = %v, want %v", tc.text, tc.ref, got, tc.want)
		}
	}
}

func TestRenderMarkdown_Supersession(t *testing.T) {
	fm, body := fixtureSpec(t)
	commit := strings.Repeat("0", 39) + "1"
	carried := tsSuperseded()
	carried.Carry, carried.Revision = objsupersede.CarryCarried, tsRevision
	facts := &SupersessionFacts{
		Objects: map[string]objsupersede.ObjectView{"ac-1": carried, "dc-1": {Object: "spec/lockbox#dc-1", State: objsupersede.ObjectUnproven, Witness: "shallow <history>"}},
		Decisions: map[string][]objsupersede.DecisionView{"dc-1": {
			{Decision: "spec/lockbox#dc-1", Edge: "spec/t#dc-1", Object: "spec/t#dc-1", State: objsupersede.DecisionInForce, Carried: true, Conflict: tsConflict, Establisher: "spec/s1", Since: "2024-02-15"},
		}},
		Links: tsLinks(),
	}
	facts.Links["spec/t#dc-1"] = "/a/spec/t/document/#dc-1?q=\"x\""
	doc, err := Build(Input{Spec: fm, Body: body, Stamp: Stamp{Ref: "spec/lockbox", Commit: commit}, Facts: Facts{Supersession: facts}, Kind: KindSpec})
	if err != nil {
		t.Fatal(err)
	}
	md := RenderMarkdown(doc)
	for _, want := range []string{
		// The criterion's lines follow its own facts, indented as its bullets are; text verbatim, markup only.
		"   - Coverage: not computed for this render.\n" +
			"   - <span class=\"objsupersede objsupersede--superseded\" data-testid=\"objsupersede-ac-1-governed\" data-state=\"superseded\">governed spec/lockbox's completed work (closed 2024-01-10)</span>\n" +
			"   - <span class=\"objsupersede objsupersede--superseded\" data-testid=\"objsupersede-ac-1-since\" data-state=\"superseded\">superseded since 2024-02-15 by <a href=\"/a/spec/s1/document/#dc-1\">spec/s1#dc-1</a></span> <a class=\"objsupersede-conflict\" data-testid=\"objsupersede-ac-1-conflict\" href=\"/a/conflict/c1/\">conflict/c1</a>\n" +
			"   - <span class=\"objsupersede objsupersede--superseded\" data-testid=\"objsupersede-ac-1-carry\" data-state=\"superseded\">carried by <a href=\"/a/spec/s2/document/\">spec/s2</a></span>\n",
		// The decision: its own object view, then its edge views, before its rationale.
		"### dc-1 — One holder per key. <a id=\"dc-1\"></a>\n\n" +
			"- <span class=\"objsupersede objsupersede--unproven objsupersede--disclosure\" data-testid=\"objsupersede-dc-1-unproven\" data-state=\"unproven\">supersession unproven: shallow &lt;history&gt;</span>\n" +
			"- <span class=\"objsupersede objsupersede--in-force\" data-testid=\"objsupersede-dc-1-spec-t-dc-1-edge\" data-state=\"in-force\">supersedes <a href=\"/a/spec/t/document/#dc-1?q=&quot;x&quot;\">spec/t#dc-1</a></span>\n" +
			"- <span class=\"objsupersede objsupersede--in-force\" data-testid=\"objsupersede-dc-1-spec-t-dc-1-carries\" data-state=\"in-force\">carries the replacement established by <a href=\"/a/spec/s1/document/\">spec/s1</a> (<a href=\"/a/conflict/c1/\">conflict/c1</a>, since 2024-02-15)</span>\n\n",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown lacks:\n%s\n--- got ---\n%s", want, md)
		}
	}
	// ac-2 and the constraint gained nothing.
	if strings.Count(md, "objsupersede-ac-") != 4 {
		t.Errorf("objsupersede markup count on criteria = %d, want ac-1's three lines plus its conflict link only:\n%s", strings.Count(md, "objsupersede-ac-"), md)
	}
	// The HTML keeps the markup (inline HTML passes through the store's engine).
	html, err := RenderHTML(doc)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`data-testid="objsupersede-ac-1-since"`,
		`<a href="/a/spec/s1/document/#dc-1">spec/s1#dc-1</a>`,
		`<a class="objsupersede-conflict" data-testid="objsupersede-ac-1-conflict" href="/a/conflict/c1/">conflict/c1</a>`,
		`data-testid="objsupersede-dc-1-spec-t-dc-1-carries"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("html lacks %q:\n%s", want, html)
		}
	}
}

// TestRenderMarkdown_SupersessionAbsentIsByteIdentical: a document built
// with a supplied-but-empty supply renders the same bytes as one with none
// — the supply adds lines only where a view exists.
func TestRenderMarkdown_SupersessionAbsentIsByteIdentical(t *testing.T) {
	fm, body := fixtureSpec(t)
	commit := strings.Repeat("0", 39) + "1"
	in := Input{Spec: fm, Body: body, Stamp: Stamp{Ref: "spec/lockbox", Commit: commit}, Kind: KindSpec}
	base, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	in.Facts.Supersession = &SupersessionFacts{Objects: map[string]objsupersede.ObjectView{}, Decisions: map[string][]objsupersede.DecisionView{}}
	empty, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	if RenderMarkdown(base) != RenderMarkdown(empty) {
		t.Errorf("an empty supply changed the bytes:\n%s", RenderMarkdown(empty))
	}
}

// TestSupersessionLineMarkdown_EscapesInlineSyntax (the L5 docs review's
// M-2): a witness or reason carrying Markdown inline syntax — backticks,
// asterisks, underscores, brackets, backslashes, angle brackets — renders
// literally through the store's engine, never as code, emphasis, a link
// or a tag, and the HTML the engine produces equals the HTML-surface
// markup (SupersessionLineMarkup) for the same line.
func TestSupersessionLineMarkdown_EscapesInlineSyntax(t *testing.T) {
	witness := "run `git remote set-head origin <branch>` and *retry* a_b\\c [x](y) ~z~ & done"
	fm, body := fixtureSpec(t)
	commit := strings.Repeat("0", 39) + "1"
	facts := &SupersessionFacts{Objects: map[string]objsupersede.ObjectView{"dc-1": {Object: "spec/lockbox#dc-1", State: objsupersede.ObjectUnproven, Witness: witness}}}
	doc, err := Build(Input{Spec: fm, Body: body, Stamp: Stamp{Ref: "spec/lockbox", Commit: commit}, Facts: Facts{Supersession: facts}, Kind: KindSpec})
	if err != nil {
		t.Fatal(err)
	}
	md := RenderMarkdown(doc)
	wantMD := `<span class="objsupersede objsupersede--unproven objsupersede--disclosure" data-testid="objsupersede-dc-1-unproven" data-state="unproven">supersession unproven: run ` + "\\`" + `git remote set-head origin &lt;branch&gt;` + "\\`" + ` and \*retry\* a\_b\\c \[x\](y) \~z\~ &amp; done</span>`
	if !strings.Contains(md, wantMD) {
		t.Errorf("markdown lacks the escaped line:\n%s\n--- got ---\n%s", wantMD, md)
	}
	html, err := RenderHTML(doc)
	if err != nil {
		t.Fatal(err)
	}
	wantHTML := `<span class="objsupersede objsupersede--unproven objsupersede--disclosure" data-testid="objsupersede-dc-1-unproven" data-state="unproven">supersession unproven: run ` + "`" + `git remote set-head origin &lt;branch&gt;` + "`" + ` and *retry* a_b\c [x](y) ~z~ &amp; done</span>`
	if !strings.Contains(html, wantHTML) {
		t.Errorf("html lacks the literal line:\n%s\n--- got ---\n%s", wantHTML, html)
	}
	// Within the line itself (the document carries code spans of its own
	// elsewhere), no syntax was read.
	start := strings.Index(html, `data-testid="objsupersede-dc-1-unproven"`)
	if start < 0 {
		t.Fatalf("html lacks the line:\n%s", html)
	}
	line := html[start : start+strings.Index(html[start:], "</span>")]
	for _, bad := range []string{"<code>", "<em>", "<del>", "<a "} {
		if strings.Contains(line, bad) {
			t.Errorf("the engine read the witness as syntax (%s):\n%s", bad, line)
		}
	}
	// The HTML surface's markup for the same line is what the engine produced.
	if got := SupersessionLineMarkup("dc-1", *doc.Decisions[0].Supersession, doc.Decisions[0].Supersession.Lines[0]); got != wantHTML {
		t.Errorf("SupersessionLineMarkup differs from the engine's HTML:\n got %s\nwant %s", got, wantHTML)
	}
}

// TestSupersession_DisclosureLines (the L5 docs review's M-5): unproven
// disclosures — an unproven supersession, an unproven carry, a
// not-established reason (SI-279) — are marked as disclosures and render
// in the muted disclosure register; §6 lines are not.
func TestSupersession_DisclosureLines(t *testing.T) {
	carried := tsSuperseded()
	carried.Carry, carried.Revision = objsupersede.CarryCarried, tsRevision
	unprovenCarry := tsSuperseded()
	unprovenCarry.Carry, unprovenCarry.Revision, unprovenCarry.Witness = objsupersede.CarryUnproven, tsRevision, "spec/s2 is not accepted"
	for _, tc := range []struct {
		name string
		view objsupersede.ObjectView
		want []bool // per line
	}{
		{"superseded, carried: §6 lines only", carried, []bool{false, false, false}},
		{"carrying unproven: the carry line is a disclosure", unprovenCarry, []bool{false, false, true}},
		{"supersession unproven", objsupersede.ObjectView{Object: tsObject, State: objsupersede.ObjectUnproven, Witness: "w"}, []bool{true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, err := ObjectSupersession(tc.view, tsLinks())
			if err != nil {
				t.Fatal(err)
			}
			var got []bool
			for _, l := range s.Lines {
				got = append(got, l.Disclosure)
				markup := SupersessionLineMarkup("x", *s, l)
				if strings.Contains(markup, "objsupersede--disclosure") != l.Disclosure {
					t.Errorf("line %q: disclosure class present=%v, want %v", l.Text, !l.Disclosure, l.Disclosure)
				}
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("disclosure flags = %v, want %v", got, tc.want)
			}
		})
	}
	notEst := objsupersede.DecisionView{Decision: "spec/lockbox#dc-1", Edge: "spec/t#dc-1", Object: "spec/t#dc-1", State: objsupersede.DecisionNotEstablished, Reason: "no conflict challenges spec/t#dc-1"}
	s, err := DecisionSupersession(notEst, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Lines) != 1 || !s.Lines[0].Disclosure {
		t.Errorf("a not-established reason is not marked as a disclosure: %+v", s.Lines)
	}
	inForce := objsupersede.DecisionView{Decision: "spec/lockbox#dc-1", Edge: "spec/t#dc-1", Object: "spec/t#dc-1", State: objsupersede.DecisionInForce}
	if s, err := DecisionSupersession(inForce, nil); err != nil || s.Lines[0].Disclosure {
		t.Errorf("an in-force edge line is marked as a disclosure: %+v, %v", s.Lines, err)
	}
}

// TestSupersessionLineMarkdown_NoAutolinks (lane L5 fix pass 2, item 12):
// a bare URL, a www. host or an email address in a witness never becomes
// a GFM autolink — a link the surface did not vet — and renders as
// written.
func TestSupersessionLineMarkdown_NoAutolinks(t *testing.T) {
	witness := "see https://example.com/x and www.example.org and a@b.io"
	fm, body := fixtureSpec(t)
	commit := strings.Repeat("0", 39) + "1"
	facts := &SupersessionFacts{Objects: map[string]objsupersede.ObjectView{"dc-1": {Object: "spec/lockbox#dc-1", State: objsupersede.ObjectUnproven, Witness: witness}}}
	doc, err := Build(Input{Spec: fm, Body: body, Stamp: Stamp{Ref: "spec/lockbox", Commit: commit}, Facts: Facts{Supersession: facts}, Kind: KindSpec})
	if err != nil {
		t.Fatal(err)
	}
	md := RenderMarkdown(doc)
	if !strings.Contains(md, `see https\://example.com/x and www\.example.org and a\@b.io`) {
		t.Errorf("markdown lacks the escaped witness:\n%s", md)
	}
	html, err := RenderHTML(doc)
	if err != nil {
		t.Fatal(err)
	}
	start := strings.Index(html, `data-testid="objsupersede-dc-1-unproven"`)
	if start < 0 {
		t.Fatalf("html lacks the line:\n%s", html)
	}
	line := html[start : start+strings.Index(html[start:], "</span>")]
	if strings.Contains(line, "<a ") {
		t.Errorf("the engine autolinked the witness:\n%s", line)
	}
	if !strings.Contains(line, "see https://example.com/x and www.example.org and a@b.io") {
		t.Errorf("the witness is not rendered as written:\n%s", line)
	}
}

// TestSupersessionLineMarkup_UnservableRefStaysAsText (the L7 review's
// F2; SI-280 as amended): a render whose surface serves no page for the
// refs a §6 line names — a per-branch board's Document tab or card, whose
// links map is empty — keeps every ref as plain text under the same test
// id and omits only the hrefs: the trailing conflict ref, which the first
// implementation dropped, stays. The text a reader sees is the linked
// render's text exactly, and the Markdown form's engine output equals the
// HTML surface's markup, as it does for a linked line.
func TestSupersessionLineMarkup_UnservableRefStaysAsText(t *testing.T) {
	linked, err := ObjectSupersession(tsSuperseded(), tsLinks())
	if err != nil {
		t.Fatal(err)
	}
	unlinked, err := ObjectSupersession(tsSuperseded(), map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if len(linked.Lines) != 2 || len(unlinked.Lines) != 2 {
		t.Fatalf("lines: linked %d, unlinked %d, want 2 each", len(linked.Lines), len(unlinked.Lines))
	}
	const wantUnlinked = `<span class="objsupersede objsupersede--superseded" data-testid="objsupersede-ac-1-since" data-state="superseded">superseded since 2024-02-15 by spec/s1#dc-1</span> <span class="objsupersede-conflict" data-testid="objsupersede-ac-1-conflict">conflict/c1</span>`
	gotUnlinked := SupersessionLineMarkup("ac-1", *unlinked, unlinked.Lines[1])
	if gotUnlinked != wantUnlinked {
		t.Errorf("unlinked since line:\n got %s\nwant %s", gotUnlinked, wantUnlinked)
	}
	if strings.Contains(gotUnlinked, "href=") || strings.Contains(gotUnlinked, "<a ") {
		t.Errorf("an unlinked line carries a link: %s", gotUnlinked)
	}
	gotLinked := SupersessionLineMarkup("ac-1", *linked, linked.Lines[1])
	if !strings.Contains(gotLinked, `<a class="objsupersede-conflict" data-testid="objsupersede-ac-1-conflict" href="/a/conflict/c1/">conflict/c1</a>`) {
		t.Fatalf("the linked line lacks its conflict link: %s", gotLinked)
	}
	// Only the hrefs differ: the visible text is identical.
	if stripTags(gotLinked) != stripTags(gotUnlinked) {
		t.Errorf("the text a reader sees differs:\n linked   %q\n unlinked %q", stripTags(gotLinked), stripTags(gotUnlinked))
	}
	// The Markdown form renders to the same HTML through the engine.
	fm, body := fixtureSpec(t)
	commit := strings.Repeat("0", 39) + "1"
	doc, err := Build(Input{Spec: fm, Body: body, Stamp: Stamp{Ref: "spec/lockbox", Commit: commit}, Kind: KindSpec,
		Facts: Facts{Supersession: &SupersessionFacts{Objects: map[string]objsupersede.ObjectView{"ac-1": tsSuperseded()}, Links: map[string]string{}}}})
	if err != nil {
		t.Fatal(err)
	}
	html, err := RenderHTML(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, wantUnlinked) {
		t.Errorf("the engine's HTML lacks the unlinked line:\n%s\n--- got ---\n%s", wantUnlinked, html)
	}
	if strings.Contains(html, `href="/a/`) {
		t.Errorf("an unlinked document links a corpus page:\n%s", html)
	}
}

// stripTags drops every HTML tag, leaving the text a reader sees.
func stripTags(s string) string {
	return regexp.MustCompile(`<[^>]+>`).ReplaceAllString(s, "")
}
