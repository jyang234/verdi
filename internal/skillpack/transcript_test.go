package skillpack_test

import (
	"bufio"
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
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/mcpserve"
	"github.com/jyang234/verdi/internal/skillpack"
	"github.com/jyang234/verdi/internal/specimport"
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
	req, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": len(tr.calls) + 1, "method": "tools/call", "params": map[string]any{"name": tool, "arguments": args}})
	if err != nil {
		tr.t.Fatalf("marshaling %s request: %v", tool, err)
	}
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
	// F8: a tool result always carries exactly one text content item
	// (mcpserve's toolText/toolError/toolJSON, the package's only
	// producers of a result); indexing [0] unguarded turns any future
	// violation of that invariant into a panic instead of a diagnosis.
	if len(resp.Result.Content) == 0 {
		tr.t.Fatalf("tools/call %s: result has no content: %s", tool, out.String())
	}
	text, isError = resp.Result.Content[0].Text, resp.Result.IsError
	tr.calls = append(tr.calls, transcriptCall{Tool: tool, Args: args, IsError: isError, Text: text})
	return text, isError
}

// assertFollows proves the recorded calls are exactly the template's
// declared call steps, in order, with loop bodies repeated n times: same
// tool; same declared kind/proposed/operations in EITHER direction — a
// step the template does not declare an arg for must not have been
// called with it either (F2: an arg the transcript sends that the
// template omits went unconstrained before this fix, so a template edit
// that silently DROPS a requirement, e.g. drops proposed=true, was not
// caught); and no recorded call may have failed (F1: a failed step is
// never a proven step — every individual tr.call site that expects
// success also checks isError directly, and this is the second,
// blanket layer). The specify handshake's expected refusal deliberately
// runs its own separate *transcript (tr2) and is never passed to this
// method, so this blanket rule never has to special-case an expected
// failure.
func (tr *transcript) assertFollows(seq skillpack.Sequence, loopCount int) {
	tr.t.Helper()
	for i, c := range tr.calls {
		if c.IsError {
			tr.t.Fatalf("call %d (%s) returned an error; a failed step is not a proven sequence: %s", i, c.Tool, c.Text)
		}
	}
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

		gotKind, hasKind := tr.calls[i].Args["kind"]
		wantKind := want[i].Args["kind"]
		if wantKind != "" && fmt.Sprint(gotKind) != wantKind {
			tr.t.Fatalf("call %d kind %v, template declares %s", i, gotKind, wantKind)
		}
		if wantKind == "" && hasKind {
			tr.t.Fatalf("call %d (%s): carries kind %v, but step %d of the template declares no kind", i, want[i].Tool, gotKind, i)
		}

		gotProposed, hasProposed := tr.calls[i].Args["proposed"]
		wantProposed := want[i].Args["proposed"]
		if wantProposed != "" && fmt.Sprint(gotProposed) != wantProposed {
			tr.t.Fatalf("call %d proposed %v, template declares %s", i, gotProposed, wantProposed)
		}
		if wantProposed == "" && hasProposed {
			tr.t.Fatalf("call %d (%s): carries proposed %v, but step %d of the template declares no proposed", i, want[i].Tool, gotProposed, i)
		}

		gotOperations, hasOperations := tr.calls[i].Args["operations"]
		wantOperations := want[i].Args["operations"]
		if wantOperations != "" {
			ops, _ := gotOperations.([]map[string]any)
			if fmt.Sprint(len(ops)) != wantOperations {
				tr.t.Fatalf("call %d carries %d operations, template declares %s", i, len(ops), wantOperations)
			}
		}
		if wantOperations == "" && hasOperations {
			tr.t.Fatalf("call %d (%s): carries operations, but step %d of the template declares none", i, want[i].Tool, i)
		}
	}
}

// write serializes every recorded call as one NDJSON line under dir and
// returns the written file's path.
func (tr *transcript) write(dir, skill string) string {
	tr.t.Helper()
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	for _, c := range tr.calls {
		if err := enc.Encode(c); err != nil {
			tr.t.Fatal(err)
		}
	}
	path := filepath.Join(dir, "transcript-"+skill+".ndjson")
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		tr.t.Fatal(err)
	}
	return path
}

// assertWritten reads path back — the file write produced — and asserts
// it decodes to exactly len(tr.calls) NDJSON lines whose tool names match
// the recorded calls in order (F4: the transcript file was previously
// written and never read back or compared to anything).
func (tr *transcript) assertWritten(path string) {
	tr.t.Helper()
	f, err := os.Open(path)
	if err != nil {
		tr.t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	var got []transcriptCall
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var c transcriptCall
		if err := json.Unmarshal(sc.Bytes(), &c); err != nil {
			tr.t.Fatalf("decoding %q: %v", sc.Text(), err)
		}
		got = append(got, c)
	}
	if err := sc.Err(); err != nil {
		tr.t.Fatal(err)
	}
	if len(got) != len(tr.calls) {
		tr.t.Fatalf("%s has %d lines, want %d (one per recorded call)", path, len(got), len(tr.calls))
	}
	for i, c := range got {
		if c.Tool != tr.calls[i].Tool {
			tr.t.Fatalf("%s line %d names tool %q, recorded call %d is %q", path, i, c.Tool, i, tr.calls[i].Tool)
		}
	}
}

// objectBlock returns the rendered document's segment starting at the
// bold "**id**" marker through (not including) the next blank line — the
// one line a criterion's Coverage or a question's Claims is on — so a
// caller can assert a fact scoped to THAT object rather than the whole
// document (F7: a bare strings.Contains(doc, id) matches the id anywhere
// it is mentioned, and a whole-document phrase count can collide with
// unrelated prose using similar words).
func objectBlock(t *testing.T, doc, id string) string {
	t.Helper()
	marker := "**" + id + "**"
	start := strings.Index(doc, marker)
	if start < 0 {
		t.Fatalf("no %q marker in document:\n%s", marker, doc)
	}
	rest := doc[start:]
	if end := strings.Index(rest, "\n\n"); end >= 0 {
		return rest[:end]
	}
	return rest
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

// designContextIdentity decodes the `identity` object out of a
// get_design_context RESULT and returns exactly the three fields
// mutate_draft's `expected` carries. This is the hand-off the
// clarify/plan skills' step 1 instructs ("Keep `identity` (`checkout`,
// `branch`, `head`) ... every `mutate_draft` call carries exactly those
// three as `expected`"): before final-review F9 the replay rebuilt
// `expected` from draftmutation.ResolveCanonicalIdentity instead, so
// the two sides agreed by construction — both being the same
// draftmutation.Identity — and the transcript proved the call sequence
// but never that what get_design_context HANDS an agent is what
// mutate_draft accepts.
func designContextIdentity(t *testing.T, contextResult string) map[string]any {
	t.Helper()
	var res struct {
		Identity struct {
			Checkout string `json:"checkout"`
			Branch   string `json:"branch"`
			Head     string `json:"head"`
		} `json:"identity"`
	}
	if err := json.Unmarshal([]byte(contextResult), &res); err != nil {
		t.Fatalf("decoding get_design_context result: %v\n%s", err, contextResult)
	}
	id := res.Identity
	if id.Checkout == "" || id.Branch == "" || id.Head == "" {
		t.Fatalf("get_design_context returned an incomplete identity %+v — the skills tell the agent to carry all three:\n%s", id, contextResult)
	}
	return map[string]any{"checkout": id.Checkout, "branch": id.Branch, "head": id.Head}
}

// mutateArgs builds a mutate_draft request carrying op. The base bytes
// and their digest come from the spec file on disk (the draft the agent
// is editing); `expected` is whatever get_design_context handed back,
// never a second, independent resolution of the same identity (F9).
func mutateArgs(t *testing.T, root string, identity map[string]any, op map[string]any) map[string]any {
	t.Helper()
	base, err := os.ReadFile(store.SpecPath(root, store.ZoneActive, draftSpecName))
	if err != nil {
		t.Fatal(err)
	}
	return map[string]any{
		"harness": "claude-code", "session": "transcript",
		"schema": draftmutation.RequestSchema, "spec": "spec/" + draftSpecName,
		"base_digest": draftmutation.DigestBytes(base), "base_spec_b64": base64.StdEncoding.EncodeToString(base),
		"expected":   identity,
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
	if err := json.Unmarshal([]byte(text), &preview); err != nil {
		t.Fatalf("decoding preview: %v\n%s", err, text)
	}
	if !preview.Ready {
		t.Fatalf("preview not ready: %s", text)
	}
	// The human sees the digest (show, confirm) — then apply with THAT digest.
	text, isErr = tr.call("import_apply", map[string]any{"harness": "claude-code", "session": "transcript", "preview_digest": preview.Digest, "request": req})
	if isErr || !strings.Contains(text, `"status":"created"`) {
		t.Fatalf("apply: %v %s", isErr, text)
	}
	// F5: the import record names the actor that proposed the mapping —
	// THIS replay's own harness/session, not merely some harness/session.
	// tool_import_test.go already proves the record mechanism generally,
	// with different values (harness codex, session s-42); this ties it
	// to this transcript's own actor (claude-code / transcript).
	view, err := specimport.ReadRecord(context.Background(), root, "design/imported", "imported")
	if err != nil {
		t.Fatal(err)
	}
	if view.Record.Actor.Harness != "claude-code" || view.Record.Actor.Session != "transcript" {
		t.Fatalf("import record actor = %+v, want harness claude-code session transcript", view.Record.Actor)
	}
	tr.assertFollows(seq, 0)
	tr.assertWritten(tr.write(t.TempDir(), "specify"))
	// Handshake witness: a different digest is refused, and the branch it
	// would have created never appears (F6: "refused" alone does not rule
	// out a partial write happening before the digest check; checking
	// gitx.HasLocalBranch, the same witness tool_import_test.go's own
	// refusal tests use, rules that out directly instead of by inference).
	tr2Root := importStore(t)
	tr2 := &transcript{t: t, srv: mcpserve.NewServer(tr2Root)}
	if text, isErr := tr2.call("import_apply", map[string]any{"harness": "claude-code", "preview_digest": strings.Repeat("0", 64), "request": req}); !isErr || !strings.Contains(text, "stale-preview") {
		t.Fatalf("stale digest must refuse: %v %s", isErr, text)
	}
	if exists, err := gitx.HasLocalBranch(context.Background(), tr2Root, "design/imported"); err != nil || exists {
		t.Fatalf("a stale-preview refusal must not create the branch (exists=%v err=%v)", exists, err)
	}
}

func TestTranscript_Clarify(t *testing.T) {
	root := draftStore(t)
	tr := &transcript{t: t, srv: mcpserve.NewServer(root)}
	seq := sequenceFor(t, "clarify")
	contextText, isContextErr := tr.call("get_design_context", map[string]any{"ref": "spec/" + draftSpecName})
	if isContextErr {
		t.Fatal(contextText)
	}
	identity := designContextIdentity(t, contextText)
	text, isErr := tr.call("get_document", map[string]any{"ref": "spec/" + draftSpecName, "kind": "spec", "proposed": true})
	if isErr {
		t.Fatal(text)
	}
	if !strings.Contains(text, "Readiness was not supplied for this render.") {
		t.Fatalf("standalone server must disclose absent readiness:\n%s", text)
	}
	// Exactly one unclaimed question in the fixture: oq-2 (oq-1 is claimed
	// by the fixture's spike stub). F7: located to oq-2's own line rather
	// than a bare strings.Contains(text, "oq-2"), which matches the id
	// anywhere it is mentioned in the document.
	if strings.Count(text, "unclaimed; blocks acceptance") != 1 {
		t.Fatalf("fixture must render exactly one unclaimed question:\n%s", text)
	}
	if !strings.Contains(objectBlock(t, text, "oq-2"), "unclaimed; blocks acceptance") {
		t.Fatalf("oq-2 must be the unclaimed question:\n%s", text)
	}
	// One proposal, shown and confirmed, then written: a research stub claiming oq-2.
	spike := true
	if text, isErr := tr.call("mutate_draft", mutateArgs(t, root, identity, map[string]any{"op": "add-stub", "slug": "answer-oq-2", "spike": spike, "resolves": []string{"oq-2"}})); isErr {
		t.Fatal(text)
	}
	tr.assertFollows(seq, 1)
	tr.assertWritten(tr.write(t.TempDir(), "clarify"))
	// The document now shows the question claimed.
	text, isErr = tr.call("get_document", map[string]any{"ref": "spec/" + draftSpecName, "kind": "spec", "proposed": true})
	if isErr {
		t.Fatal(text)
	}
	if strings.Contains(text, "unclaimed; blocks acceptance") {
		t.Fatalf("oq-2 still unclaimed after the stub:\n%s", text)
	}
}

func TestTranscript_Plan(t *testing.T) {
	root := draftStore(t)
	tr := &transcript{t: t, srv: mcpserve.NewServer(root)}
	seq := sequenceFor(t, "plan")
	contextText, isContextErr := tr.call("get_design_context", map[string]any{"ref": "spec/" + draftSpecName})
	if isContextErr {
		t.Fatal(contextText)
	}
	identity := designContextIdentity(t, contextText)
	if text, isErr := tr.call("get_document", map[string]any{"ref": "spec/" + draftSpecName, "kind": "plan", "proposed": true}); isErr || !strings.Contains(text, "## Plan") {
		t.Fatalf("plan document: %v\n%s", isErr, text)
	}
	text, isErr := tr.call("get_document", map[string]any{"ref": "spec/" + draftSpecName, "kind": "spec", "proposed": true})
	if isErr {
		t.Fatal(text)
	}
	// F7: the full Coverage line, counted document-wide and located to
	// ac-2's own block — the bare phrase "not yet planned." also matches
	// the fixture's own outcome prose ("...honestly reported as not yet
	// planned, and...", testdata/store/spec.md), and a bare
	// strings.Contains(text, "ac-2") matches the id anywhere.
	if strings.Count(text, "- Coverage: not yet planned.") != 1 {
		t.Fatalf("fixture must render exactly one uncovered criterion:\n%s", text)
	}
	if !strings.Contains(objectBlock(t, text, "ac-2"), "Coverage: not yet planned.") {
		t.Fatalf("ac-2 must be the uncovered criterion:\n%s", text)
	}
	if text, isErr := tr.call("mutate_draft", mutateArgs(t, root, identity, map[string]any{"op": "add-stub", "slug": "cover-ac-2", "acceptance_criteria": []string{"ac-2"}})); isErr {
		t.Fatal(text)
	}
	text, isErr = tr.call("get_document", map[string]any{"ref": "spec/" + draftSpecName, "kind": "plan", "proposed": true})
	if isErr {
		t.Fatal(text)
	}
	if !strings.Contains(text, "cover-ac-2") {
		t.Fatalf("plan document does not list the new stub:\n%s", text)
	}
	tr.assertFollows(seq, 1)
	tr.assertWritten(tr.write(t.TempDir(), "plan"))
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
	out, err := exec.Command("git", "-C", root, "status", "--porcelain", "--untracked-files=all").Output()
	if err != nil {
		// F3: a git failure must fail the test, never be treated as an
		// empty, clean status — in this fixture the draft is always
		// untracked, so an empty status is never a legitimate outcome and
		// could previously only ever be masking an error here.
		t.Fatalf("git status: %v", err)
	}
	want := "?? " + filepath.ToSlash(strings.TrimPrefix(store.SpecPath(root, store.ZoneActive, draftSpecName), root+"/"))
	if strings.TrimSpace(string(out)) != want {
		t.Fatalf("verdi-tasks left the checkout changed:\n%s", out)
	}
	tr.assertWritten(tr.write(t.TempDir(), "tasks"))
}
