package skillpack_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/draftmutation"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/mcpserve"
	"github.com/jyang234/verdi/internal/skillpack"
	"github.com/jyang234/verdi/internal/store"
)

// transcript is an ordered record of every tools/call a replay made,
// driven over mcpserve.ServeConn's real NDJSON framing (never a Backend
// method call), so the proof is a wire transcript.
type transcript struct {
	t     *testing.T
	srv   *mcpserve.Server
	calls []transcriptCall
}

type transcriptCall struct {
	Tool    string         `json:"tool"`
	Args    map[string]any `json:"args"`
	IsError bool           `json:"is_error"`
	Text    string         `json:"text"`
}

func (tr *transcript) call(tool string, args map[string]any) (text string, isError bool) {
	tr.t.Helper()
	req, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": len(tr.calls) + 1, "method": "tools/call", "params": map[string]any{"name": tool, "arguments": args}})
	var out bytes.Buffer
	if err := mcpserve.ServeConn(context.Background(), bytes.NewReader(append(req, '\n')), &out, tr.srv); err != nil {
		tr.t.Fatalf("ServeConn: %v", err)
	}
	var resp struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		tr.t.Fatalf("decoding response: %v\n%s", err, out.String())
	}
	text, isError = resp.Result.Content[0].Text, resp.Result.IsError
	tr.calls = append(tr.calls, transcriptCall{Tool: tool, Args: args, IsError: isError, Text: text})
	return text, isError
}

// assertFollows proves the recorded calls are exactly the template's
// declared call steps, in order, with loop bodies repeated n times.
func (tr *transcript) assertFollows(seq skillpack.Sequence, loopCount int) {
	tr.t.Helper()
	var want []skillpack.Step
	for i := 0; i < len(seq); i++ {
		st := seq[i]
		if st.Kind == "loop" {
			j := i + 1
			for ; seq[j].Kind != "end"; j++ {
			}
			body := seq[i+1 : j]
			for n := 0; n < loopCount; n++ {
				for _, b := range body {
					if b.Kind == "call" {
						want = append(want, b)
					}
				}
			}
			i = j
			continue
		}
		if st.Kind == "call" {
			want = append(want, st)
		}
	}
	if len(want) != len(tr.calls) {
		tr.t.Fatalf("transcript has %d calls, the template declares %d: %+v", len(tr.calls), len(want), tr.calls)
	}
	for i := range want {
		if tr.calls[i].Tool != want[i].Tool {
			tr.t.Fatalf("call %d is %s, template declares %s", i, tr.calls[i].Tool, want[i].Tool)
		}
		if k := want[i].Args["kind"]; k != "" && tr.calls[i].Args["kind"] != k {
			tr.t.Fatalf("call %d kind %v, template declares %s", i, tr.calls[i].Args["kind"], k)
		}
		if p := want[i].Args["proposed"]; p != "" && fmt.Sprint(tr.calls[i].Args["proposed"]) != p {
			tr.t.Fatalf("call %d proposed %v, template declares %s", i, tr.calls[i].Args["proposed"], p)
		}
		if n := want[i].Args["operations"]; n != "" {
			ops, _ := tr.calls[i].Args["operations"].([]map[string]any)
			if fmt.Sprint(len(ops)) != n {
				tr.t.Fatalf("call %d carries %d operations, template declares %s", i, len(ops), n)
			}
		}
	}
}

func (tr *transcript) write(dir, skill string) {
	tr.t.Helper()
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	for _, c := range tr.calls {
		if err := enc.Encode(c); err != nil {
			tr.t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "transcript-"+skill+".ndjson"), b.Bytes(), 0o644); err != nil {
		tr.t.Fatal(err)
	}
}

const draftSpecName = "sample"

// verdiFixtureFiles is the store recipe draftStore and importStore share:
// the manifest, gitignore, and the internal/policyauthority ASD policy
// fixture tree (go-toolchain.md's design_assistance mode rewritten to
// draft-write so a delegated agent's mutate_draft/import_apply is
// authorized), plus one committed README.md. The README is what lets
// importStore — the same recipe with no design/sample checkout and no
// draft write — sit on a clean tracked main checkout (mirrors
// internal/mcpserve/tool_import_test.go's importFixtureStoreWithMode,
// which needs the identical clean-checkout precondition for import's own
// dirty-context check).
func verdiFixtureFiles(t *testing.T) map[string]string {
	t.Helper()
	files := map[string]string{
		".verdi/verdi.yaml": "schema: verdi.layout/v1\n",
		".verdi/.gitignore": "data/\n",
		"README.md":         "# Transcript fixture store\n",
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
	return files
}

// draftStore is internal/designapp/conformance_test.go's conformanceStore
// recipe: a committed store with the draft-write policy adopted, checked
// out on design/sample, with testdata/store/spec.md as the working-tree
// draft. It also commits README.md so the import arm's preview sees a clean
// tracked checkout on main BEFORE the design branch checkout (the specify
// replay runs against a second store built by importStore).
func draftStore(t *testing.T) string {
	t.Helper()
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: verdiFixtureFiles(t), Message: "adopt draft mutation policy"}})

	checkout := exec.Command("git", "checkout", "-b", "design/"+draftSpecName)
	checkout.Dir = repo.Dir
	if output, err := checkout.CombinedOutput(); err != nil {
		t.Fatalf("git checkout design/%s: %v\n%s", draftSpecName, err, output)
	}

	resolved, err := filepath.EvalSymlinks(repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.ToSlash(resolved)

	spec, err := os.ReadFile(filepath.Join("testdata", "store", "spec.md"))
	if err != nil {
		t.Fatal(err)
	}
	specDir := store.SpecDir(root, store.ZoneActive, draftSpecName)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.SpecPath(root, store.ZoneActive, draftSpecName), spec, 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// importStore is the same recipe verdiFixtureFiles shares with draftStore,
// with no design/sample checkout and no draft written: it stays on a
// clean tracked main, exactly what import_apply needs to create
// design/<slug> itself.
func importStore(t *testing.T) string {
	t.Helper()
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: verdiFixtureFiles(t), Message: "adopt draft mutation policy"}})
	resolved, err := filepath.EvalSymlinks(repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.ToSlash(resolved)
}

func sequenceFor(t *testing.T, skill string) skillpack.Sequence {
	t.Helper()
	tmpl, err := skillpack.Template(skill)
	if err != nil {
		t.Fatal(err)
	}
	seq, err := skillpack.ParseSequence(tmpl)
	if err != nil {
		t.Fatal(err)
	}
	return seq
}

func mutateArgs(t *testing.T, root string, op map[string]any) map[string]any {
	t.Helper()
	base, err := os.ReadFile(store.SpecPath(root, store.ZoneActive, draftSpecName))
	if err != nil {
		t.Fatal(err)
	}
	identity, err := draftmutation.ResolveCanonicalIdentity(context.Background(), root, "spec/"+draftSpecName, draftmutation.GitIdentityReader{})
	if err != nil {
		t.Fatal(err)
	}
	return map[string]any{
		"harness": "claude-code", "session": "transcript",
		"schema": draftmutation.RequestSchema, "spec": "spec/" + draftSpecName,
		"base_digest": draftmutation.DigestBytes(base), "base_spec_b64": base64.StdEncoding.EncodeToString(base),
		"expected":   map[string]any{"checkout": identity.Checkout, "branch": identity.Branch, "head": identity.Head},
		"operations": []map[string]any{op},
	}
}

func TestTranscript_Specify(t *testing.T) {
	root := importStore(t)
	tr := &transcript{t: t, srv: mcpserve.NewServer(root)}
	seq := sequenceFor(t, "specify")
	data, _ := os.ReadFile(filepath.Join("testdata", "specimport", "sample.md"))
	req := map[string]any{
		"schema": "verdi.spec-import-request/v1", "format": "markdown-v1", "primary": "brief",
		"target":  map[string]any{"slug": "imported", "class": "feature", "title": "Imported"},
		"sources": []map[string]any{{"id": "brief", "label": "brief.md", "data": base64.StdEncoding.EncodeToString(data)}},
		// Both evidence-only mappings tool_import_test.go's importReadyRequest
		// uses: sample.md's two Acceptance Criteria bullets become automatic
		// ac-1/ac-2 fields with no declared evidence, and specimport's own
		// missing-evidence rule makes an evidence-less request's preview
		// permanently not-ready.
		"mappings": []map[string]any{
			{"target": "ac-1", "evidence": []string{"static", "attestation"}},
			{"target": "ac-2", "evidence": []string{"static", "attestation"}},
		},
		"retain_unmapped": true,
	}
	text, isErr := tr.call("import_preview", map[string]any{"request": req})
	if isErr {
		t.Fatal(text)
	}
	var preview struct {
		Digest string `json:"digest"`
		Ready  bool   `json:"ready"`
	}
	json.Unmarshal([]byte(text), &preview)
	if !preview.Ready {
		t.Fatalf("preview not ready: %s", text)
	}
	// The human sees the digest (show, confirm) — then apply with THAT digest.
	text, isErr = tr.call("import_apply", map[string]any{"harness": "claude-code", "session": "transcript", "preview_digest": preview.Digest, "request": req})
	if isErr || !strings.Contains(text, `"status":"created"`) {
		t.Fatalf("apply: %v %s", isErr, text)
	}
	tr.assertFollows(seq, 0)
	tr.write(t.TempDir(), "specify")
	// Handshake witness: a different digest is refused and writes nothing new.
	tr2 := &transcript{t: t, srv: mcpserve.NewServer(importStore(t))}
	if text, isErr := tr2.call("import_apply", map[string]any{"harness": "claude-code", "preview_digest": strings.Repeat("0", 64), "request": req}); !isErr || !strings.Contains(text, "stale-preview") {
		t.Fatalf("stale digest must refuse: %v %s", isErr, text)
	}
}

func TestTranscript_Clarify(t *testing.T) {
	root := draftStore(t)
	tr := &transcript{t: t, srv: mcpserve.NewServer(root)}
	seq := sequenceFor(t, "clarify")
	if text, isErr := tr.call("get_design_context", map[string]any{"ref": "spec/" + draftSpecName}); isErr {
		t.Fatal(text)
	}
	text, isErr := tr.call("get_document", map[string]any{"ref": "spec/" + draftSpecName, "kind": "spec", "proposed": true})
	if isErr {
		t.Fatal(text)
	}
	if !strings.Contains(text, "Readiness was not supplied for this render.") {
		t.Fatalf("standalone server must disclose absent readiness:\n%s", text)
	}
	// Exactly one unclaimed question in the fixture: oq-2 (oq-1 is claimed by the fixture's spike stub).
	if strings.Count(text, "unclaimed; blocks acceptance") != 1 || !strings.Contains(text, "oq-2") {
		t.Fatalf("fixture must render one unclaimed question:\n%s", text)
	}
	// One proposal, shown and confirmed, then written: a research stub claiming oq-2.
	spike := true
	if text, isErr := tr.call("mutate_draft", mutateArgs(t, root, map[string]any{"op": "add-stub", "slug": "answer-oq-2", "spike": spike, "resolves": []string{"oq-2"}})); isErr {
		t.Fatal(text)
	}
	tr.assertFollows(seq, 1)
	tr.write(t.TempDir(), "clarify")
	// The document now shows the question claimed.
	text, _ = tr.call("get_document", map[string]any{"ref": "spec/" + draftSpecName, "kind": "spec", "proposed": true})
	if strings.Contains(text, "unclaimed; blocks acceptance") {
		t.Fatalf("oq-2 still unclaimed after the stub:\n%s", text)
	}
}

func TestTranscript_Plan(t *testing.T) {
	root := draftStore(t)
	tr := &transcript{t: t, srv: mcpserve.NewServer(root)}
	seq := sequenceFor(t, "plan")
	tr.call("get_design_context", map[string]any{"ref": "spec/" + draftSpecName})
	if text, isErr := tr.call("get_document", map[string]any{"ref": "spec/" + draftSpecName, "kind": "plan", "proposed": true}); isErr || !strings.Contains(text, "## Plan") {
		t.Fatalf("plan document: %v\n%s", isErr, text)
	}
	text, _ := tr.call("get_document", map[string]any{"ref": "spec/" + draftSpecName, "kind": "spec", "proposed": true})
	if strings.Count(text, "not yet planned.") != 1 || !strings.Contains(text, "ac-2") {
		t.Fatalf("fixture must render exactly one uncovered criterion (ac-2):\n%s", text)
	}
	if text, isErr := tr.call("mutate_draft", mutateArgs(t, root, map[string]any{"op": "add-stub", "slug": "cover-ac-2", "acceptance_criteria": []string{"ac-2"}})); isErr {
		t.Fatal(text)
	}
	text, _ = tr.call("get_document", map[string]any{"ref": "spec/" + draftSpecName, "kind": "plan", "proposed": true})
	if !strings.Contains(text, "cover-ac-2") {
		t.Fatalf("plan document does not list the new stub:\n%s", text)
	}
	tr.assertFollows(seq, 1)
	tr.write(t.TempDir(), "plan")
}

func TestTranscript_Tasks(t *testing.T) {
	root := draftStore(t)
	tr := &transcript{t: t, srv: mcpserve.NewServer(root)}
	seq := sequenceFor(t, "tasks")
	before, _ := os.ReadFile(store.SpecPath(root, store.ZoneActive, draftSpecName))
	text, isErr := tr.call("get_document", map[string]any{"ref": "spec/" + draftSpecName, "kind": "tasks", "proposed": true})
	if isErr || !strings.Contains(text, "## Plan") || !strings.Contains(text, "## Readiness") {
		t.Fatalf("tasks document: %v\n%s", isErr, text)
	}
	tr.assertFollows(seq, 0)
	for _, c := range tr.calls {
		if c.Tool == "mutate_draft" || c.Tool == "add_annotation" || c.Tool == "import_apply" {
			t.Fatalf("verdi-tasks called a write tool: %s", c.Tool)
		}
	}
	after, _ := os.ReadFile(store.SpecPath(root, store.ZoneActive, draftSpecName))
	if !bytes.Equal(before, after) {
		t.Fatal("verdi-tasks changed the draft")
	}
	// --untracked-files=all: the default "normal" mode collapses a wholly
	// untracked directory (here, .verdi/specs/, since no ancestor of the
	// draft path is tracked) to one directory-level line, which would make
	// this comparison pass even if a second untracked file appeared beside
	// the draft. "all" lists every untracked file individually, so the
	// comparison actually checks the one file this fixture's draft is.
	out, _ := exec.Command("git", "-C", root, "status", "--porcelain", "--untracked-files=all").Output()
	if strings.TrimSpace(string(out)) != "?? "+filepath.ToSlash(strings.TrimPrefix(store.SpecPath(root, store.ZoneActive, draftSpecName), root+"/")) && strings.TrimSpace(string(out)) != "" {
		// the draft itself is untracked in this fixture; nothing else may appear
		t.Fatalf("verdi-tasks left the checkout changed:\n%s", out)
	}
	tr.write(t.TempDir(), "tasks")
}
