package mcpserve

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/skillpack"
	"github.com/jyang234/verdi/internal/specimport"
)

// importFixtureStore builds the ready-path fixture: the same store with
// the fixture policy's design_assistance mode overridden to draft-write,
// so a delegated agent's apply is authorized.
func importFixtureStore(t *testing.T) string {
	t.Helper()
	return importFixtureStoreWithMode(t, "draft-write")
}

// importFixtureStoreWithMode builds one hermetic fixturegit repository for
// import_preview/import_apply tests: the same recipe
// internal/designapp/conformance_test.go's conformanceStore and
// cmd/verdi/designimport_test.go's designImportPolicyRepo both use
// (.verdi/verdi.yaml, .verdi/.gitignore, the internal/policyauthority
// testdata store tree with go-toolchain.md's mode set to mode) plus one
// committed README.md — used by
// TestImportPreview_NotReadyIsAResultAndInvalidIsAnError to prove the
// dirty-context refusal by writing to it — WITHOUT conformanceStore's own
// "git checkout -b design/sample" step: Apply itself create-only publishes
// the target design/<slug> branch, so this fixture stays on main.
//
// mode is the caller's choice exactly as internal/specimport's own
// publishpolicy_test.go parameterizes the identical fixture: "draft-write"
// authorizes a delegated agent, and "proposal-only" (the committed
// fixture's own value, rewritten to itself) makes the same apply a genuine
// policy-forbidden refusal.
func importFixtureStoreWithMode(t *testing.T, mode string) string {
	t.Helper()
	files := map[string]string{
		".verdi/verdi.yaml": "schema: verdi.layout/v1\n",
		".verdi/.gitignore": "data/\n",
		"README.md":         "# Import fixture store\n",
	}
	source := filepath.Join("..", "policyauthority", "testdata", "store")
	if err := filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if entry.Name() == "go-toolchain.md" {
			data = bytes.Replace(data, []byte("mode: proposal-only"), []byte("mode: "+mode), 1)
		}
		files[filepath.ToSlash(rel)] = string(data)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: files, Message: "adopt " + mode + " import policy"}})

	resolved, err := filepath.EvalSymlinks(repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.ToSlash(resolved)
}

// importReadyRequest mirrors cmd/verdi/designimport_test.go's
// designImportReadyRequest over the same committed sample markdown
// (internal/mcpserve/testdata/specimport/sample.md, an exact copy of
// cmd/verdi/testdata/specimport/sample.md — fixtures live under testdata/
// only). Both evidence-only Mappings are required, not cosmetic: sample.md's
// two Acceptance Criteria bullets become automatic ac-1/ac-2 fields with no
// declared evidence, and specimport's own missing-evidence rule
// (mapping.go: "acceptance-criterion Field with no declared Evidence" is
// ALWAYS blocking, spec-import-contract.md's "preview has a missing-evidence
// finding until an explicit Mapping supplies kinds") makes an evidence-less
// request's preview permanently not-ready — verified empirically (a scratch
// Normalize() call over this exact fixture, since deleted) before adding
// these two Mappings, which is exactly the CLI fixture's own workaround for
// the identical VL-006 feature-outcome attestation floor.
func importReadyRequest(t *testing.T, slug string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "specimport", "sample.md"))
	if err != nil {
		t.Fatal(err)
	}
	return map[string]any{
		"schema":  specimport.RequestSchema,
		"target":  map[string]any{"slug": slug, "class": "feature", "title": "Imported sample"},
		"format":  "markdown-v1",
		"primary": "brief",
		"sources": []map[string]any{{"id": "brief", "label": "brief.md", "data": base64.StdEncoding.EncodeToString(data)}},
		"mappings": []map[string]any{
			{"target": "ac-1", "evidence": []string{"static", "attestation"}},
			{"target": "ac-2", "evidence": []string{"static", "attestation"}},
		},
		"retain_unmapped": true,
	}
}

func decodeText(t *testing.T, res map[string]any) (text string, isError bool) {
	t.Helper()
	content := res["content"].([]map[string]any)
	isErr, _ := res["isError"].(bool)
	return content[0]["text"].(string), isErr
}

func TestImportPreviewThenApply_RecordNamesHarnessAndSession(t *testing.T) {
	root := importFixtureStore(t)
	b := &Backend{Root: root}
	req := importReadyRequest(t, "imported-sample")

	raw, _ := json.Marshal(map[string]any{"request": req})
	text, isErr := decodeText(t, b.ImportPreview(context.Background(), raw))
	if isErr {
		t.Fatalf("preview errored: %s", text)
	}
	var preview specimport.PreviewResult
	if err := json.Unmarshal([]byte(text), &preview); err != nil {
		t.Fatal(err)
	}
	if !preview.Ready || preview.Schema != specimport.PreviewResultSchema || len(preview.Digest) != 64 {
		t.Fatalf("preview = %+v", preview)
	}
	// Preview is read-only: no design branch yet. Asked of git itself
	// (packed refs included), never of a loose ref file.
	if exists, err := gitx.HasLocalBranch(context.Background(), root, "design/imported-sample"); err != nil || exists {
		t.Fatalf("preview must not create the design branch (exists=%v err=%v)", exists, err)
	}

	raw, _ = json.Marshal(map[string]any{"harness": "claude-code", "session": "s-42", "preview_digest": preview.Digest, "request": req})
	text, isErr = decodeText(t, b.ImportApply(context.Background(), raw))
	if isErr {
		t.Fatalf("apply errored: %s", text)
	}
	var result specimport.Result
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		t.Fatal(err)
	}
	if result.Status != specimport.StatusCreated || result.Branch != "design/imported-sample" || result.PreviewDigest != preview.Digest {
		t.Fatalf("apply = %+v", result)
	}
	// The positive arm of the same branch check the refusal paths use,
	// so "no branch was created" is never a vacuous answer.
	if exists, err := gitx.HasLocalBranch(context.Background(), root, result.Branch); err != nil || !exists {
		t.Fatalf("apply must create %s (exists=%v err=%v)", result.Branch, exists, err)
	}
	view, err := specimport.ReadRecord(context.Background(), root, result.Branch, "imported-sample")
	if err != nil {
		t.Fatal(err)
	}
	if view.Record.Actor.Harness != "claude-code" || view.Record.Actor.Session != "s-42" {
		t.Fatalf("record actor = %+v, want harness claude-code session s-42", view.Record.Actor)
	}
	if !view.CurrentSpecMatches {
		t.Fatalf("record view = %+v", view)
	}
	// Retry with the same digest is already-created, never a second branch.
	text, isErr = decodeText(t, b.ImportApply(context.Background(), raw))
	if isErr || !strings.Contains(text, `"status":"already-created"`) {
		t.Fatalf("retry: isErr %v text %s", isErr, text)
	}
}

func TestImportApply_Refusals(t *testing.T) {
	root := importFixtureStore(t)
	b := &Backend{Root: root}
	req := importReadyRequest(t, "refused")
	for _, tc := range []struct {
		name string
		args map[string]any
		want string
	}{
		{"missing harness", map[string]any{"preview_digest": strings.Repeat("a", 64), "request": req}, "import_apply: harness is required"},
		// A whitespace-only harness is the same refusal as an absent one:
		// one spelling for one rule, decided here rather than reaching
		// NewDelegatedAgent's differently worded nonblank check.
		{"whitespace harness", map[string]any{"harness": " \t ", "preview_digest": strings.Repeat("a", 64), "request": req}, "import_apply: harness is required"},
		{"bad digest shape", map[string]any{"harness": "codex", "preview_digest": "HEAD", "request": req}, "import_apply: preview_digest must be 64 lowercase hex characters"},
		{"stale digest", map[string]any{"harness": "codex", "preview_digest": strings.Repeat("a", 64), "request": req}, "import_apply: stale-preview:"},
		{"actor field refused", map[string]any{"harness": "codex", "preview_digest": strings.Repeat("a", 64), "request": req, "actor": "human"}, "import_apply: malformed arguments"},
		{"unknown request field", map[string]any{"harness": "codex", "preview_digest": strings.Repeat("a", 64), "request": func() map[string]any { r := importReadyRequest(t, "x"); r["candidate"] = "zz"; return r }()}, "import_apply: invalid-request:"},
	} {
		raw, _ := json.Marshal(tc.args)
		text, isErr := decodeText(t, b.ImportApply(context.Background(), raw))
		if !isErr || !strings.HasPrefix(text, tc.want) {
			t.Fatalf("%s: isErr %v text %q, want prefix %q", tc.name, isErr, text, tc.want)
		}
	}
}

func TestImportPreview_NotReadyIsAResultAndInvalidIsAnError(t *testing.T) {
	root := importFixtureStore(t)
	b := &Backend{Root: root}
	// retain_unmapped=false with unmapped prose (sample.md's own headings,
	// blank lines and bullet markers are never part of any mapped field
	// span — verified empirically over this exact fixture) → an
	// unresolved-coverage finding, ready:false, NOT isError (R-W3-5).
	req := importReadyRequest(t, "notready")
	req["retain_unmapped"] = false
	raw, _ := json.Marshal(map[string]any{"request": req})
	text, isErr := decodeText(t, b.ImportPreview(context.Background(), raw))
	if isErr || !strings.Contains(text, `"ready":false`) || !strings.Contains(text, `"unresolved-coverage"`) {
		t.Fatalf("not-ready preview: isErr %v text %s", isErr, text)
	}
	// Invalid request → isError with the CLI's code.
	bad := importReadyRequest(t, "bad")
	bad["format"] = "docx"
	raw, _ = json.Marshal(map[string]any{"request": bad})
	text, isErr = decodeText(t, b.ImportPreview(context.Background(), raw))
	if !isErr || !strings.HasPrefix(text, "import_preview: unsupported-format:") && !strings.HasPrefix(text, "import_preview: invalid-request:") {
		t.Fatalf("invalid preview: isErr %v text %q", isErr, text)
	}
	// Oversize envelope → isError before any decode.
	huge := json.RawMessage(`{"request":{"pad":"` + strings.Repeat("x", specimport.MaxEnvelopeBytes+2) + `"}}`)
	if text, isErr := decodeText(t, b.ImportPreview(context.Background(), huge)); !isErr || !strings.Contains(text, "exceed") {
		t.Fatalf("oversize: isErr %v text %.80q", isErr, text)
	}
	// Dirty checkout → dirty-context.
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	raw, _ = json.Marshal(map[string]any{"request": importReadyRequest(t, "dirty")})
	if text, isErr := decodeText(t, b.ImportPreview(context.Background(), raw)); !isErr || !strings.HasPrefix(text, "import_preview: dirty-context:") {
		t.Fatalf("dirty: isErr %v text %q", isErr, text)
	}
}

// mangledRequest is one request document specimport's own decoder refuses
// and a re-encoding pass would silently repair, plus the substring the
// refusal must name.
type mangledRequest struct {
	name    string
	request []byte
	detail  string
}

// importMangledRequests builds the review's three probes plus the
// nested-unknown-field control. Each document starts from
// importReadyRequest, so the ONLY difference from a request that previews
// ready is the defect under test — an accepted document therefore shows
// up as a successful preview of a mangled reading, never as an incidental
// error. json.Unmarshal → canonjson.Marshal destroys exactly this
// evidence: duplicate keys collapse last-wins, invalid UTF-8 becomes
// U+FFFD, and a float spelling of an int field is renormalized.
func importMangledRequests(t *testing.T) []mangledRequest {
	t.Helper()
	base, err := json.Marshal(importReadyRequest(t, "dup-b"))
	if err != nil {
		t.Fatal(err)
	}
	// json.Marshal sorts a map's keys, so base carries exactly one
	// "target" member; prepending a second one gives the document two
	// readings (first-wins dup-a, Go's map-last-wins dup-b).
	duplicated := append([]byte(`{"target":{"class":"feature","slug":"dup-a","title":"Imported sample"},`), base[1:]...)

	badUTF8 := importReadyRequest(t, "utf8-probe")
	badUTF8["target"] = map[string]any{"slug": "utf8-probe", "class": "feature", "title": "BadXtitle"}
	badUTF8Raw, err := json.Marshal(badUTF8)
	if err != nil {
		t.Fatal(err)
	}
	// Go's encoder replaces invalid UTF-8 on the way out, so the 0xFF has
	// to be spliced into the finished bytes.
	badUTF8Raw = bytes.Replace(badUTF8Raw, []byte("BadXtitle"), []byte("Bad\xfftitle"), 1)

	// A float spelling of Source.StartLine/EndLine. 1..15 is sample.md's
	// whole 15-line body, so the ONLY defect is the number spelling: a
	// canonicalizing pass renormalizes 1.0 to 1 and the request then
	// previews exactly as the ready one does. The refusal names whichever
	// of the pair the decoder reaches first — end_line, since json.Marshal
	// sorts a map's keys — and quotes the float spelling verbatim, which
	// is the evidence a re-encoding pass destroys.
	floatSpelled := importReadyRequest(t, "floatline-probe")
	floatSource := floatSpelled["sources"].([]map[string]any)[0]
	floatSource["start_line"] = json.Number("1.0")
	floatSource["end_line"] = json.Number("15.0")
	floatRaw, err := json.Marshal(floatSpelled)
	if err != nil {
		t.Fatal(err)
	}

	// Control: a nested unknown field, which every decode refuses.
	nested := importReadyRequest(t, "nested-probe")
	nested["sources"].([]map[string]any)[0]["origin"] = "invented"
	nestedRaw, err := json.Marshal(nested)
	if err != nil {
		t.Fatal(err)
	}

	return []mangledRequest{
		{"duplicate top-level key", duplicated, `duplicate JSON key "target"`},
		{"invalid UTF-8 in target.title", badUTF8Raw, "must be valid UTF-8"},
		{"float spelling of an int field", floatRaw, "cannot unmarshal number 15.0"},
		{"nested unknown field (control)", nestedRaw, `unknown field "origin"`},
	}
}

// TestDecodeImportRequest_HandsCallerBytesToTheSoleDecoder pins R-W3-10 at
// the seam itself: specimport.DecodeRequest is the import contract's sole
// request decoder (spec-import-contract.md: "DecodeRequest([]byte)
// (Request, error) is the sole request decoder"), and it must see the
// caller's bytes, not a re-encoding of one possible reading of them. The
// argument envelope's own exact decode refuses two of these documents one
// seam earlier, which is why this test calls decodeImportRequest directly:
// that outer gate must never be the only thing between a mangled reading
// and the write path.
func TestDecodeImportRequest_HandsCallerBytesToTheSoleDecoder(t *testing.T) {
	for _, tc := range importMangledRequests(t) {
		t.Run(tc.name, func(t *testing.T) {
			req, failure := decodeImportRequest("import_preview", tc.request)
			if failure == nil {
				t.Fatalf("accepted, decoded target %+v; the caller's bytes must reach specimport.DecodeRequest untouched (R-W3-10)", req.Target)
			}
			text, isErr := decodeText(t, failure)
			if !isErr || !strings.HasPrefix(text, "import_preview: invalid-request:") || !strings.Contains(text, tc.detail) {
				t.Fatalf("isErr %v text %q, want an invalid-request refusal naming %q", isErr, text, tc.detail)
			}
		})
	}
	// The empty arm: no request member at all.
	req, failure := decodeImportRequest("import_apply", nil)
	if failure == nil {
		t.Fatalf("absent request accepted as %+v", req)
	}
	if text, isErr := decodeText(t, failure); !isErr || text != "import_apply: request is required" {
		t.Fatalf("absent request: isErr %v text %q", isErr, text)
	}
}

// TestImportPreview_MangledRequestIsRefusedEndToEnd drives the same four
// documents through the live tool against a real fixture store, where an
// accepted mangled request answers isError:false with a candidate built
// from the mangled reading (exactly what the review observed). Each row
// asserts the refusal the tool actually produces: the duplicate-key and
// invalid-UTF-8 documents are refused by the argument envelope's exact
// decode, the other two by the request decoder.
func TestImportPreview_MangledRequestIsRefusedEndToEnd(t *testing.T) {
	b := &Backend{Root: importFixtureStore(t)}
	for _, tc := range importMangledRequests(t) {
		t.Run(tc.name, func(t *testing.T) {
			raw := append(append([]byte(`{"request":`), tc.request...), '}')
			text, isErr := decodeText(t, b.ImportPreview(context.Background(), raw))
			if !isErr || !strings.Contains(text, tc.detail) {
				t.Fatalf("isErr %v text %.200q, want a refusal naming %q", isErr, text, tc.detail)
			}
		})
	}
	if text, isErr := decodeText(t, b.ImportPreview(context.Background(), json.RawMessage(`{}`))); !isErr || text != "import_preview: request is required" {
		t.Fatalf("absent request: isErr %v text %q", isErr, text)
	}
}

// TestImportApply_RawArgumentRefusals covers the argument-envelope arms a
// map literal cannot express. A duplicated `harness` key is the one that
// matters: import_apply is the first tool where a duplicated argument key
// would silently select the RECORDED ACTOR IDENTITY (ac-9: the record
// names the harness and session that proposed the mapping), so a relay
// reading first-wins and a server reading last-wins would disagree about
// who acted.
func TestImportApply_RawArgumentRefusals(t *testing.T) {
	b := &Backend{Root: importFixtureStore(t)}
	request, err := json.Marshal(importReadyRequest(t, "raw-refused"))
	if err != nil {
		t.Fatal(err)
	}
	digest := strings.Repeat("a", 64)
	for _, tc := range []struct {
		name string
		args string
		want string
	}{
		{
			"duplicate harness key",
			`{"harness":"claude-code","harness":"impersonated","preview_digest":"` + digest + `","request":` + string(request) + `}`,
			`import_apply: malformed arguments: artifact: duplicate JSON key "harness"`,
		},
		{
			"absent request",
			`{"harness":"claude-code","preview_digest":"` + digest + `"}`,
			"import_apply: request is required",
		},
	} {
		text, isErr := decodeText(t, b.ImportApply(context.Background(), json.RawMessage(tc.args)))
		if !isErr || text != tc.want {
			t.Fatalf("%s: isErr %v text %q, want %q", tc.name, isErr, text, tc.want)
		}
	}
	// Oversize envelope → refused before any decode, exactly as
	// import_preview's own arm is.
	huge := json.RawMessage(`{"harness":"codex","preview_digest":"` + digest + `","request":{"pad":"` + strings.Repeat("x", specimport.MaxEnvelopeBytes+2) + `"}}`)
	if text, isErr := decodeText(t, b.ImportApply(context.Background(), huge)); !isErr || !strings.Contains(text, "exceed") {
		t.Fatalf("oversize: isErr %v text %.80q", isErr, text)
	}
}

// TestImportApply_ProposalOnlyIsPolicyForbidden exercises the
// policy-forbidden row of importSentinels over the real authorization
// path: the identical fixture store with the design_assistance mode left
// at the committed proposal-only value. Preview is read-only and
// unaffected, so the refusal arrives with a MATCHING digest — the
// authorization decision itself, not a handshake or cleanliness bail.
func TestImportApply_ProposalOnlyIsPolicyForbidden(t *testing.T) {
	root := importFixtureStoreWithMode(t, "proposal-only")
	b := &Backend{Root: root}
	req := importReadyRequest(t, "policy-refused")

	raw, _ := json.Marshal(map[string]any{"request": req})
	text, isErr := decodeText(t, b.ImportPreview(context.Background(), raw))
	if isErr {
		t.Fatalf("preview errored: %s", text)
	}
	var preview specimport.PreviewResult
	if err := json.Unmarshal([]byte(text), &preview); err != nil {
		t.Fatal(err)
	}
	if !preview.Ready {
		t.Fatalf("preview must be ready for the refusal to be the policy decision: %+v", preview)
	}

	raw, _ = json.Marshal(map[string]any{"harness": "claude-code", "preview_digest": preview.Digest, "request": req})
	text, isErr = decodeText(t, b.ImportApply(context.Background(), raw))
	if !isErr || !strings.HasPrefix(text, "import_apply: policy-forbidden:") {
		t.Fatalf("proposal-only apply: isErr %v text %q, want the policy-forbidden refusal", isErr, text)
	}
	if exists, err := gitx.HasLocalBranch(context.Background(), root, "design/policy-refused"); err != nil || exists {
		t.Fatalf("a branch was created despite the policy-forbidden refusal (exists=%v err=%v)", exists, err)
	}
}

// TestImportToolError_UnknownSentinelIsIOFailure covers importToolError's
// default arm: every future specimport sentinel this table does not name
// yet lands there, and must still carry a code from the CLI's closed
// vocabulary rather than a bare message.
func TestImportToolError_UnknownSentinelIsIOFailure(t *testing.T) {
	text, isErr := decodeText(t, importToolError("import_preview", errors.New("boom")))
	if !isErr || text != "import_preview: io-failure: boom" {
		t.Fatalf("unknown sentinel: isErr %v text %q", isErr, text)
	}
	// A named sentinel still wins over the default arm.
	wrapped := fmt.Errorf("%w: detail", specimport.ErrTargetExists)
	if text, isErr := decodeText(t, importToolError("import_apply", wrapped)); !isErr || !strings.HasPrefix(text, "import_apply: target-exists:") {
		t.Fatalf("named sentinel: isErr %v text %q", isErr, text)
	}
}

// refusalSectionRe captures the body of the verdi-specify template's
// "Refusals you must relay verbatim" section: everything up to the next
// heading or fenced block.
var refusalSectionRe = regexp.MustCompile("(?s)## Refusals you must relay verbatim\n(.*?)(?:\n## |\n```)")

// refusalCodeSpanRe captures each backticked token in that section. Every
// token there is a refusal code — the section names nothing else.
var refusalCodeSpanRe = regexp.MustCompile("`([a-z-]+)`")

// TestImportRefusalVocabularyMatchesTheSpecifySkill closes final-review
// F3 in both directions: the verdi-specify skill tells an agent to relay
// import refusals verbatim, so the closed vocabulary it prints must be
// exactly the set of codes importSentinels emits — no code the agent can
// meet and not find on the list (the shipped list omitted four, including
// `io-failure`, which is ALSO importToolError's fallback prefix for every
// error the contract does not map, so it is the code an agent is most
// likely to see), and no code on the list the tool cannot produce.
//
// The template is read through skillpack.Template — the same embedded
// bytes verdi harness render writes and verdi harness check gates — so
// this test binds the instruction to the implementation, not to a copy.
func TestImportRefusalVocabularyMatchesTheSpecifySkill(t *testing.T) {
	tmpl, err := skillpack.Template("specify")
	if err != nil {
		t.Fatal(err)
	}
	section := refusalSectionRe.FindSubmatch(tmpl)
	if section == nil {
		t.Fatalf("the verdi-specify template has no %q section", "Refusals you must relay verbatim")
	}

	emitted := map[string]bool{}
	for _, s := range importSentinels {
		emitted[s.code] = true
	}
	if len(emitted) == 0 {
		t.Fatal("importSentinels is empty; this test would pass vacuously")
	}

	listed := map[string]bool{}
	for _, m := range refusalCodeSpanRe.FindAllSubmatch(section[1], -1) {
		listed[string(m[1])] = true
	}
	if len(listed) == 0 {
		t.Fatalf("no backticked refusal code found in the section:\n%s", section[1])
	}

	for code := range emitted {
		if !listed[code] {
			t.Errorf("import refusal code %q is emitted by importSentinels but not listed in the verdi-specify template's refusal section — an agent told to relay refusals verbatim would not find it", code)
		}
	}
	for code := range listed {
		if !emitted[code] {
			t.Errorf("the verdi-specify template lists refusal code %q, which importSentinels never emits", code)
		}
	}
}
