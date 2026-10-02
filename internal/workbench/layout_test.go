package workbench

import (
	"bytes"
	"context"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/readinesspilot"
)

// preSlotPage renders data exactly as renderPage did before the bar slots
// existed (G6): the bar drawn from the page's facts with the heading, the
// surface word, and the nav links only. It is the byte reference every
// shared-layout page that fills no slot must still match.
func preSlotPage(t *testing.T, ctx context.Context, root string, data pageData) []byte {
	t.Helper()
	data.Bar = branchBarFacts(ctx, root, data.Title)
	data.TopBar = renderTopBar(&data.Bar, topBarOptions{Heading: true, Surface: data.Surface, Nav: data.Nav})
	data.HasMermaid = strings.Contains(string(data.BodyHTML), `<pre class="mermaid">`)
	var buf bytes.Buffer
	if err := pageTemplate.Execute(&buf, data); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestRenderPage_EmptyBarSlotsKeepTheBytes is G6's byte proof: a
// shared-layout page that fills neither bar slot renders byte-identically
// to the pre-slot composition, for the page shapes the shared layout
// serves (nav only, surface word, a body with a mermaid block, metadata
// rows).
func TestRenderPage_EmptyBarSlotsKeepTheBytes(t *testing.T) {
	ctx := context.Background()
	for _, data := range []pageData{
		{Title: "Disclosures", Nav: `<a href="/">index</a>`, BodyHTML: "<p>body</p>"},
		{Title: "verdi workbench", Surface: true, BodyHTML: "<p>index</p>"},
		{Title: "spec/x", Nav: `<a href="/">index</a>`, MetaRows: []metaRow{{Label: "kind", Value: "spec"}}, BodyHTML: `<pre class="mermaid">graph TD</pre>`, ExtraHTML: "<p>extra</p>"},
		{Title: "Readiness", Nav: `<a href="/">index</a> <span class="current">readiness</span>`, BodyHTML: "<p>r</p>"},
	} {
		t.Run(data.Title, func(t *testing.T) {
			got, err := renderPage(ctx, "", data)
			if err != nil {
				t.Fatal(err)
			}
			if want := preSlotPage(t, ctx, "", data); !bytes.Equal(got, want) {
				t.Fatalf("an empty-slot page changed bytes:\n--- got ---\n%s\n--- want ---\n%s", got, want)
			}
			if bytes.Contains(got, []byte("topbar-controls")) {
				t.Fatalf("an empty-slot page carries a controls slot:\n%s", got)
			}
		})
	}
}

// TestRenderPage_BarSlotsRenderInTheBar is G6's additive slot: a page that
// fills BarChips and BarControls gets them in its top bar row — chips
// after the title, controls in the bar's controls slot — and nowhere else.
func TestRenderPage_BarSlotsRenderInTheBar(t *testing.T) {
	data := pageData{
		Title:       "Readiness",
		Nav:         `<a href="/">index</a>`,
		BarChips:    template.HTML(`<span class="probe-chip">chip</span>`),
		BarControls: template.HTML(`<a class="probe-control" href="/b/x">Open the wall →</a>`),
		BodyHTML:    "<p>body</p>",
	}
	out, err := renderPage(context.Background(), "", data)
	if err != nil {
		t.Fatal(err)
	}
	page := string(out)
	bar := page[strings.Index(page, `<header class="topbar"`):strings.Index(page, `</header>`)]
	title := `<h1 class="topbar-title" data-testid="topbar-title">Readiness</h1>`
	for _, want := range []string{
		title + `<span class="probe-chip">chip</span>`,
		`<div class="topbar-controls" data-testid="topbar-controls"><a class="probe-control" href="/b/x">Open the wall →</a></div>`,
	} {
		if !strings.Contains(bar, want) {
			t.Fatalf("bar lacks %q:\n%s", want, bar)
		}
	}
	if strings.Count(page, "probe-chip") != 1 || strings.Count(page, "probe-control") != 1 {
		t.Fatalf("a slot rendered outside the bar or twice:\n%s", page)
	}

	// The next page that fills no slot carries none of the previous page's.
	next, err := renderPage(context.Background(), "", pageData{Title: "Disclosures", Nav: `<a href="/">index</a>`, BodyHTML: "<p>body</p>"})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(next, []byte("probe-")) || bytes.Contains(next, []byte("topbar-controls")) {
		t.Fatalf("a slot leaked into the next page:\n%s", next)
	}
}

// TestSharedLayoutRoutesFillNoBarSlot drives the shared-layout routes
// through the real handler: none of them fills a bar slot today, so none
// carries the bar's controls slot (a slot leaking from one page into
// another would show here).
func TestSharedLayoutRoutesFillNoBarSlot(t *testing.T) {
	h := NewHandlerWith(t.TempDir(), Deps{})
	for _, path := range []string{"/", "/disclosures", "/readiness", "/a/spec/absent", "/verdict/absent", "/matrix/absent", "/no-such-page"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		body, err := io.ReadAll(rec.Result().Body)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(body, []byte(`data-testid="topbar"`)) {
			t.Fatalf("%s: no shared top bar rendered (status %d):\n%s", path, rec.Code, body)
		}
		if bytes.Contains(body, []byte("topbar-controls")) {
			t.Fatalf("%s carries a bar controls slot no shared-layout page fills:\n%s", path, body)
		}
	}
}

// TestReadinessRouteReceivesTheModel is G4: the readiness route is built
// with the operating model every other shared-layout page receives, so the
// page can speak the plain vocabulary. readinessRoute is the route's
// dependency set as RegisterRoutesWithHome builds it.
func TestReadinessRouteReceivesTheModel(t *testing.T) {
	mdl := model.Canonical()
	loader := stubReadinessLoader{}
	route := newReadinessRoute("/root", Deps{Model: mdl, ReadinessLoader: loader, ReadinessDefaultSpec: "spec/x"})
	if route.root != "/root" || route.mdl != mdl || route.loader != loader || route.defaultSpec != "spec/x" {
		t.Fatalf("readiness route = %+v, want the root, the model, the loader, and the default spec it was given", route)
	}
	if empty := newReadinessRoute("", Deps{}); empty.mdl != nil || empty.loader != nil || empty.defaultSpec != "" {
		t.Fatalf("readiness route over empty deps = %+v, want zero dependencies", empty)
	}
}

// stubReadinessLoader is a comparable ReadinessLoader for identity checks.
type stubReadinessLoader struct{}

func (stubReadinessLoader) Load(context.Context, string) (readinesspilot.Snapshot, error) {
	return readinesspilot.Snapshot{}, nil
}
