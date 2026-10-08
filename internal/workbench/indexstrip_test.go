package workbench

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/index"
)

// stripBlock cuts one strip <details> out of a rendered body.
func stripBlock(t *testing.T, body, class string) string {
	t.Helper()
	re := regexp.MustCompile(`(?s)<details class="` + class + `" data-testid="` + class + `">.*?</details>`)
	m := re.FindString(body)
	if m == "" {
		t.Fatalf("no %s details in: %s", class, body)
	}
	return m
}

// TestRenderHome_OtherRecordsStrip is ac-5's strip witness: below the
// columns, the other kinds, the services and the boards each render as a
// collapsed <details> (no open attribute: it opens natively, without
// JavaScript) whose summary names the section and carries its count, with
// the full listing inside, under the kept classes.
func TestRenderHome_OtherRecordsStrip(t *testing.T) {
	repo := buildWorkbenchFixtureRepo(t)
	provisionHomeRefs(t, repo.Dir)
	svcDir := filepath.Join(repo.Dir, "strip-service")
	if err := os.MkdirAll(svcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(svcDir, ".flowmap.yaml"), []byte("version: 1\nservice: strip-service\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, body := getHome(t, repo.Dir, HomeDeps{})

	strip := regexp.MustCompile(`(?s)<section class="home-strip" aria-labelledby="home-strip-heading"><h2 id="home-strip-heading">Other records</h2>.*?</section>`).FindString(body)
	if strip == "" {
		t.Fatalf("no other-records strip; got: %s", body)
	}
	if columns := strings.Index(body, `<div class="dir-columns">`); columns < 0 || strings.Index(body, strip) < columns {
		t.Fatalf("the strip must sit below the columns")
	}
	if regexp.MustCompile(`<details[^>]* open[ >=]`).MatchString(strip) {
		t.Fatalf("every strip section must render collapsed; got: %s", strip)
	}

	kinds := stripBlock(t, strip, "home-kinds")
	n := strings.Count(kinds, "<li>")
	if n == 0 || !strings.Contains(kinds, `<summary>Other artifacts <span class="count">`+strconv.Itoa(n)+`</span></summary>`) {
		t.Errorf("the kinds fold's count must equal its listed artifacts (%d); got: %s", n, kinds)
	}
	if !strings.Contains(kinds, `<h3>adr <span class="count">`) || !strings.Contains(kinds, `href="/a/adr/0002-outbox-events"`) {
		t.Errorf("the kinds fold must still list every kind with its count and links; got: %s", kinds)
	}

	services := stripBlock(t, strip, "home-services")
	if !strings.Contains(services, `<summary>Services <span class="count">1</span></summary><ul><li>strip-service</li></ul>`) {
		t.Errorf("the services fold must carry its count and listing; got: %s", services)
	}

	boards := stripBlock(t, strip, "home-boards")
	if !strings.Contains(boards, `<summary>Boards <span class="count">1</span></summary><ul><li><a href="/board/STORY-1482">STORY-1482</a></li></ul>`) {
		t.Errorf("the boards fold must carry its count and listing; got: %s", boards)
	}
}

// TestStripSections_EmptyAndUnproven: an empty section folds with a zero
// count and its honest empty line; a section whose listing could not be
// read folds with a disclosed unproven mark and the notice, never a zero.
func TestStripSections_EmptyAndUnproven(t *testing.T) {
	var buf bytes.Buffer
	writeOtherKindsSection(&buf, &index.Index{}, nil)
	if got := buf.String(); got != `<details class="home-kinds" data-testid="home-kinds"><summary>Other artifacts <span class="count">0</span></summary><p class="empty">No other artifacts.</p></details>` {
		t.Errorf("empty kinds fold = %s", got)
	}

	buf.Reset()
	writeOtherKindsSection(&buf, nil, errors.New("corpus: boom"))
	got := buf.String()
	if strings.Contains(got, `class="count"`) || !strings.Contains(got, `<summary>Other artifacts <span class="dir-unproven" title="corpus: boom">count unproven</span></summary><p class="notice">Could not read the corpus for this store: corpus: boom</p>`) {
		t.Errorf("unreadable corpus fold must disclose, never count; got: %s", got)
	}

	root := t.TempDir()
	buf.Reset()
	writeServicesSection(&buf, root)
	if got := buf.String(); got != `<details class="home-services" data-testid="home-services"><summary>Services <span class="count">0</span></summary><p class="empty">No services discovered.</p></details>` {
		t.Errorf("empty services fold = %s", got)
	}

	buf.Reset()
	writeBoardsSection(&buf, root)
	if got := buf.String(); got != `<details class="home-boards" data-testid="home-boards"><summary>Boards <span class="count">0</span></summary><p class="empty">No boards yet.</p></details>` {
		t.Errorf("absent boards fold = %s", got)
	}

	// A boards directory that is a file: an unreadable listing.
	if err := os.MkdirAll(filepath.Join(root, ".verdi", "data", "mutable"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(boardsDirForTest(root), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	buf.Reset()
	writeBoardsSection(&buf, root)
	got = buf.String()
	if strings.Contains(got, `class="count"`) || !strings.Contains(got, `count unproven</span></summary><p class="notice">Could not read boards: `) {
		t.Errorf("unreadable boards fold must disclose, never count; got: %s", got)
	}
}
