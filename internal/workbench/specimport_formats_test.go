package workbench

// Task 4 UI coverage of the remaining supported import surfaces the
// contract names (native, manual-v1, story class with tracker/parent/
// implements links) at the handler level, plus main's pre-review
// regression probe on the board affordance: a source-record link derived
// from FILE PRESENCE must never read as a verification claim — that claim
// belongs to a successful ReadRecord in the record view alone.
//
// Fixtures reuse the accepted backend rules verbatim: the native primary is
// internal/specimport's newSpecNativeFixture bytes (exact eligible bytes);
// the story store mirrors compose_external_test.go's trackerManifestYAML
// (a configured jira provider is what VL-005 requires) plus one landed
// parent feature — synthetic and test-only, no user configuration, no
// real tracker is ever contacted.

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/specimport"
	"github.com/jyang234/verdi/internal/store"
)

// specImportTrackerManifest mirrors internal/specimport/compose_external_test.go's
// trackerManifestYAML: the jira provider configuration VL-005 requires
// before a story's scheme-qualified tracker ref counts as configured.
const specImportTrackerManifest = "schema: verdi.layout/v1\n" +
	"providers:\n" +
	"  jira:\n" +
	"    base_url: https://example.atlassian.net\n" +
	"    rollup_field: customfield_00000\n"

// specImportParentSlug is the landed parent feature the story fixtures
// implement (testdata/specimport/parent-feature.md, mirrored by the e2e
// harness's isolated store).
const specImportParentSlug = "widget-parent"

func readWorkbenchImportFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "specimport", name))
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	return data
}

// newSpecImportStoreWithParent is newSpecImportStore plus the synthetic
// tracker manifest and the landed parent feature on main.
func newSpecImportStoreWithParent(t *testing.T) string {
	t.Helper()
	neutralizeCIEnv(t)
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/verdi.yaml":                           specImportTrackerManifest,
			".verdi/.gitignore":                           "data/\n",
			store.ActiveSpecRelPath(specImportParentSlug): string(readWorkbenchImportFixture(t, "parent-feature.md")),
		},
		Message: "seed store with a synthetic tracker and a landed parent feature",
	}})
	origin := filepath.Join(t.TempDir(), "origin.git")
	gitRun(t, "", "init", "--bare", "--quiet", "--initial-branch=main", origin)
	gitRun(t, repo.Dir, "remote", "add", "origin", origin)
	gitRun(t, repo.Dir, "push", "--quiet", "--set-upstream", "origin", "main")
	gitRun(t, repo.Dir, "remote", "set-head", "origin", "main")
	return repo.Dir
}

func blockingMessageContains(findings []specimport.Finding, want string) bool {
	for _, f := range findings {
		if f.Blocking && strings.Contains(f.Message, want) {
			return true
		}
	}
	return false
}

// TestSpecImport_Native_ExactBytesIdentityMismatchAndRefusedControls: a
// native primary imports byte-identically once its identity matches the
// target; a mismatching target slug is an invalid-request refusal naming
// the mismatch (corrected by fixing the target, never by rewriting bytes);
// explicit mappings are refused for native.
func TestSpecImport_Native_ExactBytesIdentityMismatchAndRefusedControls(t *testing.T) {
	root := newSpecImportStore(t)
	c := newImportClient(t, NewHandlerWith(root, Deps{}))
	native := readWorkbenchImportFixture(t, "native-widget.md")
	request := map[string]any{
		"schema":           specimport.RequestSchema,
		"target":           map[string]any{"slug": "native-gadget", "class": "feature", "title": "Native Widget"},
		"format":           specimport.FormatNative,
		"primary":          "native",
		"sources":          []any{importSource("native", "native-widget.md", native)},
		"defer_statements": false,
		"retain_unmapped":  false,
	}
	status, _, failure := c.preview(request)
	if status != http.StatusBadRequest || failure.Code != "invalid-request" || !strings.Contains(failure.Error, "does not match target slug") {
		t.Fatalf("mismatching native identity = %d %+v, want 400 invalid-request naming the slug mismatch", status, failure)
	}

	request["target"].(map[string]any)["slug"] = "native-widget"
	status, preview, failure := c.preview(request)
	if status != http.StatusOK || !preview.Ready {
		t.Fatalf("native preview = %d ready=%v %+v %+v", status, preview.Ready, preview.Findings, failure)
	}
	if !bytes.Equal(preview.Candidate, native) {
		t.Fatalf("native candidate is not the exact source bytes:\n%s", preview.Candidate)
	}
	// Native carries no per-target field views (the candidate IS the
	// primary, byte for byte); coverage maps the whole primary once.
	if len(preview.Fields) != 0 {
		t.Fatalf("native preview must not fabricate field views: %+v", preview.Fields)
	}
	if len(preview.Coverage) != 1 || preview.Coverage[0].MappedBytes != preview.Coverage[0].TotalBytes || preview.Coverage[0].TotalBytes != len(native) {
		t.Fatalf("native coverage = %+v", preview.Coverage)
	}
	status, created, failure := c.apply(request, preview.Digest, nil)
	if status != http.StatusOK || created.Status != specimport.StatusCreated {
		t.Fatalf("native apply = %d %+v %+v", status, created, failure)
	}
	status, record := c.get("/design/import/record?branch=design%2Fnative-widget&spec=native-widget")
	if status != http.StatusOK || !strings.Contains(record, `data-current-spec-matches="true"`) {
		t.Fatalf("native record = %d: %s", status, record)
	}
	// The committed candidate digest equals the retained primary's selected
	// digest: the record view shows the same digest as both.
	if strings.Count(record, preview.Sources[0].Digest) < 2 {
		t.Fatalf("native record does not show the candidate digest equal to the primary's selected digest %s", preview.Sources[0].Digest)
	}

	request["mappings"] = []any{map[string]any{"target": "problem", "text": "rewritten"}}
	if status, _, failure := c.preview(request); status != http.StatusBadRequest || !strings.Contains(failure.Error, "refuses explicit mappings") {
		t.Fatalf("native with mappings = %d %+v, want 400 refusing mappings", status, failure)
	}
	delete(request, "mappings")
	request["defer_statements"] = true
	if status, _, failure := c.preview(request); status != http.StatusBadRequest || !strings.Contains(failure.Error, "refuses statement deferral") {
		t.Fatalf("native with deferral = %d %+v, want 400 refusing deferral", status, failure)
	}
}

// TestSpecImport_Manual_SourceSpansUserTextAndEvidence: manual-v1 yields no
// automatic fields; explicit source-backed spans (copied-source), a
// user-authored statement (user-added) and explicit evidence on the
// criterion make the candidate ready, with byte coverage accounting for
// exactly the mapped spans.
func TestSpecImport_Manual_SourceSpansUserTextAndEvidence(t *testing.T) {
	root := newSpecImportStore(t)
	c := newImportClient(t, NewHandlerWith(root, Deps{}))
	primary := readSpecImportFixture(t, "markdown/positive-basic.md")
	problemStart := bytes.Index(primary, []byte("Operators currently"))
	problemEnd := bytes.Index(primary, []byte("\n\n## Outcome"))
	acText := "The importer reads a Markdown file."
	acStart := bytes.Index(primary, []byte(acText))
	if problemStart < 0 || problemEnd < 0 || acStart < 0 {
		t.Fatal("fixture offsets not found")
	}
	acEnd := acStart + len(acText)
	request := map[string]any{
		"schema":  specimport.RequestSchema,
		"target":  map[string]any{"slug": "manual-widget", "class": "feature", "title": "Manual Widget"},
		"format":  specimport.FormatManualV1,
		"primary": "widget",
		"sources": []any{importSource("widget", "positive-basic.md", primary)},
		"mappings": []any{
			map[string]any{"target": "problem", "source_id": "widget", "start": problemStart, "end": problemEnd, "transform": "identity"},
			map[string]any{"target": "outcome", "text": "Operators bring existing specs onto a board without retyping them."},
			map[string]any{"target": "ac-1", "source_id": "widget", "start": acStart, "end": acEnd, "transform": "identity", "evidence": []string{"attestation"}},
		},
		"defer_statements": false,
		"retain_unmapped":  false,
	}
	status, unresolved, failure := c.preview(request)
	if status != http.StatusOK || unresolved.Ready || findingCounts(unresolved.Findings)[specimport.FindingUnresolvedCoverage] != 1 {
		t.Fatalf("manual preview without retention = %d ready=%v %+v %+v", status, unresolved.Ready, unresolved.Findings, failure)
	}
	request["retain_unmapped"] = true
	status, preview, failure := c.preview(request)
	if status != http.StatusOK || !preview.Ready {
		t.Fatalf("manual preview = %d ready=%v %+v %+v", status, preview.Ready, preview.Findings, failure)
	}
	problem, _ := fieldByTarget(preview.Fields, "problem")
	if problem.Origin != specimport.OriginCopiedSource || problem.Text != "Operators currently retype every requirement by hand.\nThis wastes their afternoon." {
		t.Fatalf("manual problem = %+v", problem)
	}
	outcome, _ := fieldByTarget(preview.Fields, "outcome")
	if outcome.Origin != specimport.OriginUserAdded || len(outcome.Spans) != 0 {
		t.Fatalf("manual outcome = %+v", outcome)
	}
	ac, _ := fieldByTarget(preview.Fields, "ac-1")
	if ac.Origin != specimport.OriginCopiedSource || ac.Text != acText || strings.Join(ac.Evidence, ",") != "attestation" {
		t.Fatalf("manual ac-1 = %+v", ac)
	}
	cov := preview.Coverage[0]
	if cov.MappedBytes != (problemEnd-problemStart)+(acEnd-acStart) || cov.UnresolvedBytes != 0 || cov.RetainedBytes != cov.TotalBytes-cov.MappedBytes {
		t.Fatalf("manual coverage = %+v", cov)
	}
	status, created, failure := c.apply(request, preview.Digest, nil)
	if status != http.StatusOK || created.Status != specimport.StatusCreated {
		t.Fatalf("manual apply = %d %+v %+v", status, created, failure)
	}
	status, record := c.get("/design/import/record?branch=design%2Fmanual-widget&spec=manual-widget")
	if status != http.StatusOK || !strings.Contains(record, `data-origin="user-added"`) || !strings.Contains(record, `data-origin="copied-source"`) || !strings.Contains(record, "without retyping them") {
		t.Fatalf("manual record = %d: %s", status, record)
	}
}

// TestSpecImport_Story_TrackerParentImplementsAndInvalidRefCorrection: a
// story-class import needs its configured tracker scheme and a resolving
// implements edge to a landed parent; a missing edge, an unresolvable ref
// and an unconfigured scheme are each blocking findings the user corrects
// in place — never a synthesized tracker or an invented parent.
func TestSpecImport_Story_TrackerParentImplementsAndInvalidRefCorrection(t *testing.T) {
	root := newSpecImportStoreWithParent(t)
	c := newImportClient(t, NewHandlerWith(root, Deps{}))
	request := labeledRequest(t, "widget-story")
	request["target"] = map[string]any{"slug": "widget-story", "class": "story", "title": "Widget Import", "story": "bogus:WID-1"}
	request["retain_unmapped"] = true
	request["mappings"] = []any{
		map[string]any{"target": "ac-1", "evidence": []string{"static"}},
		map[string]any{"target": "ac-2", "evidence": []string{"static"}},
		map[string]any{"target": "ac-3", "evidence": []string{"static"}},
	}

	status, noEdge, failure := c.preview(request)
	if status != http.StatusOK || noEdge.Ready || !blockingMessageContains(noEdge.Findings, "implements edge") {
		t.Fatalf("story without an implements edge = %d ready=%v %+v %+v", status, noEdge.Ready, noEdge.Findings, failure)
	}

	request["links"] = []any{map[string]any{"type": "implements", "ref": "spec/no-such-feature#ac-1"}}
	status, dangling, _ := c.preview(request)
	if status != http.StatusOK || dangling.Ready || !blockingMessageContains(dangling.Findings, "VL-003") {
		t.Fatalf("story with a dangling implements ref = %d ready=%v %+v", status, dangling.Ready, dangling.Findings)
	}

	request["links"] = []any{map[string]any{"type": "implements", "ref": "spec/" + specImportParentSlug + "#ac-1"}}
	status, unconfigured, _ := c.preview(request)
	if status != http.StatusOK || unconfigured.Ready || !blockingMessageContains(unconfigured.Findings, "VL-005") || blockingMessageContains(unconfigured.Findings, "VL-003") {
		t.Fatalf("story with an unconfigured tracker scheme = %d ready=%v %+v", status, unconfigured.Ready, unconfigured.Findings)
	}

	request["target"].(map[string]any)["story"] = "jira:WID-1"
	status, ready, failure := c.preview(request)
	if status != http.StatusOK || !ready.Ready {
		t.Fatalf("corrected story preview = %d ready=%v %+v %+v", status, ready.Ready, ready.Findings, failure)
	}
	status, created, failure := c.apply(request, ready.Digest, nil)
	if status != http.StatusOK || created.Status != specimport.StatusCreated || created.SpecRef != "spec/widget-story" {
		t.Fatalf("story apply = %d %+v %+v", status, created, failure)
	}
	status, board := c.get(created.BoardPath)
	if status != http.StatusOK || !strings.Contains(board, `data-board-mode="authoring"`) || !strings.Contains(board, specImportParentSlug) {
		t.Fatalf("story board = %d: missing authoring mode or the parent feature link", status)
	}
}

// affordanceOf slices the source-record affordance out of a rendered board.
func affordanceOf(t *testing.T, board string) string {
	t.Helper()
	start := strings.Index(board, `data-testid="asd-import-origin"`)
	if start < 0 {
		t.Fatal("board carries no source-record affordance")
	}
	end := strings.Index(board[start:], "</p>")
	if end < 0 {
		t.Fatal("unterminated affordance")
	}
	return board[start : start+end]
}

// TestSpecImport_BoardAffordanceNeverClaimsVerificationFromPresence is
// main's pre-review regression probe: the board's link is derived from the
// PRESENCE of a record file in the working tree, so its wording must send
// the reader to the record view (the one verifier) and never assert that
// this spec's proof was verified — on a normal imported board and on a
// board whose committed record was corrupted before its worktree was cut.
func TestSpecImport_BoardAffordanceNeverClaimsVerificationFromPresence(t *testing.T) {
	root := newSpecImportStore(t)
	c := newImportClient(t, NewHandlerWith(root, Deps{}))

	normal := importLabeled(t, c, "presence-normal")
	corrupt := importLabeled(t, c, "presence-corrupt")
	commitOnBranchDetached(t, root, corrupt.Branch, store.ImportRecordRelPath("presence-corrupt", corrupt.PreviewDigest), func(data []byte) []byte {
		return data[:10]
	}, "tamper with the import record before any board is cut")

	for _, tc := range []struct {
		name   string
		result specimport.Result
		slug   string
	}{{"normal imported board", normal, "presence-normal"}, {"board over a corrupted committed record", corrupt, "presence-corrupt"}} {
		status, board := c.get(tc.result.BoardPath)
		if status != http.StatusOK {
			t.Fatalf("%s: GET board = %d", tc.name, status)
		}
		panel := affordanceOf(t, board)
		href := "/design/import/record?branch=design%2F" + tc.slug + "&amp;spec=" + tc.slug
		if !strings.Contains(panel, href) {
			t.Errorf("%s: affordance lost the record link: %s", tc.name, panel)
		}
		if strings.Contains(panel, "verified against") {
			t.Errorf("%s: affordance asserts verification from file presence: %s", tc.name, panel)
		}
		if !strings.Contains(panel, "not verified here") {
			t.Errorf("%s: affordance must say the record is not verified on the board: %s", tc.name, panel)
		}
		for _, want := range []string{"not an ASD provenance entry", "classify the creation as unclassified", "not evidence of acceptance"} {
			if !strings.Contains(panel, want) {
				t.Errorf("%s: affordance missing %q", tc.name, want)
			}
		}
	}
	// The one verifier still discloses the corruption when followed.
	status, page := c.get("/design/import/record?branch=design%2Fpresence-corrupt&spec=presence-corrupt")
	if status != http.StatusConflict || !strings.Contains(page, `data-testid="import-record-unavailable"`) {
		t.Fatalf("record over the corrupted proof = %d, want the 409 unavailable page", status)
	}
}
