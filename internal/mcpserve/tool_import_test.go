package mcpserve

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/specimport"
)

// importFixtureStore builds one hermetic fixturegit repository for
// import_preview/import_apply tests: the same recipe
// internal/designapp/conformance_test.go's conformanceStore and
// cmd/verdi/designimport_test.go's designImportPolicyRepo both use
// (.verdi/verdi.yaml, .verdi/.gitignore, the internal/policyauthority
// testdata store tree with go-toolchain.md's mode overridden to
// draft-write) plus one committed README.md — used by
// TestImportPreview_NotReadyIsAResultAndInvalidIsAnError to prove the
// dirty-context refusal by writing to it — WITHOUT conformanceStore's own
// "git checkout -b design/sample" step: Apply itself create-only publishes
// the target design/<slug> branch, so this fixture stays on main.
func importFixtureStore(t *testing.T) string {
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
			data = bytes.Replace(data, []byte("mode: proposal-only"), []byte("mode: draft-write"), 1)
		}
		files[filepath.ToSlash(rel)] = string(data)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: files, Message: "adopt draft-write import policy"}})

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
	// Preview is read-only: no design branch yet.
	if _, err := os.Stat(filepath.Join(root, ".git", "refs", "heads", "design", "imported-sample")); err == nil {
		t.Fatal("preview must not create the design branch")
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
