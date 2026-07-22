package main

// vocabFixture (spec/vocabulary-surfaces ac-2) answers the one behavioral
// case the shared harness corpus provably cannot: a served board whose
// store carries a vocab-rename model.yaml. A display rename is STORE-WIDE
// by design ("a rename can never leak partially") — planting model.yaml
// in the shared scratch store would rename every status chip and class
// tag other suites pin by their bare-id text, exactly the invasive,
// high-blast-radius mutation the empty-glance fixture's ADJ-40 rationale
// rules out. So, following that adjudicated convention verbatim
// (emptyglance.go): a SEPARATE, fully hermetic workbench instance,
// in-process, over a REAL minimal store on disk — git init + bare origin
// (load-bearing: refindex's default-branch walk keys off
// refs/remotes/origin/HEAD; without it the directory would render the
// no-default-branch degradation, not the store's entries) — served
// through the SAME production wiring the real workbench uses
// (workbench.NewHandler → RegisterRoutesWithHome → store.Open →
// Config.Model), so the rename the browser sees flowed through the real
// model-resolution pipe, never a canned label.
//
// The store's .verdi/model.yaml is internal/model/testdata's
// vocab-rename.yaml — model-schema's own frontier fixture (accept ->
// "Sign off", accepted-pending-build -> "Ready to build", feature ->
// "Initiative", story -> "Workstream", spike -> "Timebox"), read from
// the module tree at provision time and reused verbatim, never
// duplicated. Its main-branch spec, vocab-probe (a round-four feature,
// accepted-pending-build), gives the home glance/directory a renamed
// status chip and the board a renamed case-file class tag. A second,
// DRAFT feature spec, vocab-draft, lives on its own design branch with
// the serving checkout LEFT on that branch (provisionBoard's own
// convention) — status draft + a non-default checkout branch is exactly
// what boardspec.go's mode switch requires — so /board/spec/vocab-draft
// serves in AUTHORING mode and the board's CLIENT-side prose becomes
// drivable in a real browser under the renamed vocabulary: the sticky
// type menu's STICKY_TYPES labels and the proto-yarn dialog/refusal
// copy (judged-client-js-prose-has-no-browser-proof).
import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/jyang234/verdi/internal/workbench"
)

// vocabProbeSpec is the store's one spec: a round-four feature (problem/
// outcome, so its board wears the case file and class tag), status
// accepted-pending-build (the state the vocab-rename model renames).
const vocabProbeSpec = `---
id: spec/vocab-probe
kind: spec
title: "Vocab probe"
owners: [platform-team]
class: feature
status: accepted-pending-build
story: jira:VOC-1
problem: { text: "display vocabulary is hard-coded per surface", anchor: problem }
outcome: { text: "one rename reaches every surface at once", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "renamed labels render in the browser", evidence: [behavioral] }
frozen: { at: 2026-01-01, commit: deadbeefdeadbeefdeadbeefdeadbeefdeadbeef }
---
# Vocab probe

## Problem

Display vocabulary is hard-coded per surface.

## Outcome

One rename reaches every surface at once.
`

// vocabDraftBranch carries the authoring fixture; the serving checkout
// stays on it (never main), which is what makes the draft board's mode
// authoring.
const vocabDraftBranch = "design/vocab-draft"

// vocabDraftSpec is the authoring fixture: a DRAFT round-four feature
// (class feature, so the client's STICKY_TYPES control offers the
// story/spike proto-stickies) with one acceptance criterion — ac-1 is
// the drop target a misaimed spike thread refuses against
// (boardspec.js's routeProtoYarn), which is the dialog copy the browser
// proof asserts speaks the renamed words.
const vocabDraftSpec = `---
id: spec/vocab-draft
kind: spec
title: "Vocab draft"
owners: [platform-team]
class: feature
status: draft
problem: { text: "client-side dialog copy is rendered by boardspec.js, not the server", anchor: problem }
outcome: { text: "the menu labels and refusal copy speak the model's renamed words", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "renamed class words reach JS-rendered prose", evidence: [behavioral] }
---
# Vocab draft

## Problem

Client-side dialog copy is rendered by boardspec.js, not the server.

## Outcome

The menu labels and refusal copy speak the model's renamed words.
`

// vocabCustomStoryTemplate is the store's own .verdi/templates/
// custom-story.md — the file the vocab-rename model's story class
// DECLARES (template: custom-story.md), so the tailored store is real:
// template resolution must go through the store's own override, exactly
// the L-M12 property the creation form's e2e proves in a browser
// (spec/creation-form ac-3). The canonical story shape plus a
// distinctive Delivery Notes section the created spec must carry.
const vocabCustomStoryTemplate = `---
id: {{safe .Ref}}
kind: spec
title: {{printf "%q" .Title}}
owners: {{safe .Owners}}
class: story
status: draft
story: {{safe .StoryRef}}
{{if .Spike}}spike: true
{{end}}problem: { text: {{printf "%q" .Problem}}, anchor: problem }
outcome: { text: {{printf "%q" .Outcome}}, anchor: outcome }
{{if not .Spike}}acceptance_criteria:
  - { id: ac-1, text: "TODO: replace with real acceptance criteria before accept", evidence: [static], anchor: ac-1 }
{{end}}links:
{{range .Links}}  - { type: {{.Type}}, ref: {{printf "%q" .Ref}} }
{{end}}---
# {{.Title}}

## Problem

TODO: design notes.

## Outcome

TODO: design notes.

## Delivery Notes

TODO: how this lands safely.
{{if not .Spike}}
## Ac 1

TODO: design notes.
{{end}}`

// vocabCustomFeatureTemplate is .verdi/templates/custom-feature.md — the
// feature class's declared template, present for the same
// store-is-really-tailored reason (the canonical feature shape).
const vocabCustomFeatureTemplate = `---
id: {{safe .Ref}}
kind: spec
title: {{printf "%q" .Title}}
owners: {{safe .Owners}}
class: feature{{if .StoryRef}}
story: {{safe .StoryRef}}{{end}}
status: draft
problem: { text: {{printf "%q" .Problem}}, anchor: problem }
outcome: { text: {{printf "%q" .Outcome}}, anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "TODO: replace with real acceptance criteria before accept", evidence: [static, attestation], anchor: ac-1 }
---
# {{.Title}}

## Problem

TODO: design notes.

## Outcome

TODO: design notes.

## Ac 1

TODO: design notes.
`

// vocabFixture lazily starts its isolated server and remembers its bound
// URL — the same start-once cache shape emptyGlanceFixture uses.
type vocabFixture struct {
	moduleRoot string

	mu   sync.Mutex
	url  string
	root string
}

func newVocabFixture(moduleRoot string) *vocabFixture {
	return &vocabFixture{moduleRoot: moduleRoot}
}

// handler answers GET with the fixture's URL as a plain-text body,
// starting the isolated server on the first call.
func (f *vocabFixture) handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	url, err := f.ensureStarted()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(url))
}

// ensureStarted provisions the vocab-rename store and starts the isolated
// workbench instance over it on first call, returning its URL on every
// call thereafter, unchanged.
func (f *vocabFixture) ensureStarted() (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.url != "" {
		return f.url, nil
	}

	root, err := provisionVocabStore(f.moduleRoot)
	if err != nil {
		return "", err
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	srv := &http.Server{Handler: workbench.NewHandler(root)}
	go func() { _ = srv.Serve(ln) }()

	f.url = "http://" + ln.Addr().String() + "/"
	f.root = root
	return f.url, nil
}

// showHandler is the vocab fixture's read-only inspection window
// (spec/creation-form ac-3's outside-the-browser verification), the
// inspect.go posture scoped to this isolated store: GET ?ref=<design
// branch>&path=<store-relative path> answers `git show ref:path` from
// the fixture's own repository — the suite's proof that a board-created
// spec landed on exactly the branch the receipt named, without trusting
// the browser's own claims. Read-only; design/* refs only; loopback
// only, like every control route.
func (f *vocabFixture) showHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	f.mu.Lock()
	root := f.root
	f.mu.Unlock()
	if root == "" {
		http.Error(w, "vocab fixture not started yet (GET /vocab-fixture first)", http.StatusConflict)
		return
	}
	ref := r.URL.Query().Get("ref")
	rel := r.URL.Query().Get("path")
	if !strings.HasPrefix(ref, "design/") || strings.ContainsAny(ref, " \t\n:") {
		http.Error(w, "ref must name a design/* branch", http.StatusBadRequest)
		return
	}
	if rel == "" || filepath.IsAbs(rel) || strings.Contains(rel, "..") {
		http.Error(w, "path must be a store-relative, traversal-free path", http.StatusBadRequest)
		return
	}
	out, err := gitOutput(root, "show", ref+":"+rel)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(out))
}

// provisionVocabStore builds a REAL, minimal, hermetic store on disk and
// returns its root: the manifest, the vocab-rename model.yaml (copied
// from the module tree's committed fixture), and the one probe spec —
// committed on main with a bare local origin whose HEAD names main
// (provisionEmptyStore's own load-bearing origin setup, mirrored), so
// refindex's real default-branch walk lists vocab-probe in the home
// directory rather than short-circuiting on the no-remote path.
func provisionVocabStore(moduleRoot string) (string, error) {
	tmp, err := os.MkdirTemp("", "verdi-e2e-vocab-*")
	if err != nil {
		return "", err
	}
	root := filepath.Join(tmp, "store")
	originDir := filepath.Join(tmp, "origin.git")

	specDir := filepath.Join(root, ".verdi", "specs", "active", "vocab-probe")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		return "", fmt.Errorf("creating vocab-probe spec dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".verdi", "templates"), 0o755); err != nil {
		return "", fmt.Errorf("creating templates dir: %w", err)
	}

	modelYAML, err := os.ReadFile(filepath.Join(moduleRoot, "internal", "model", "testdata", "vocab-rename.yaml"))
	if err != nil {
		return "", fmt.Errorf("reading the vocab-rename model fixture: %w", err)
	}

	files := map[string][]byte{
		filepath.Join(root, ".verdi", "verdi.yaml"): []byte(emptyStoreManifest),
		filepath.Join(root, ".verdi", "model.yaml"): modelYAML,
		filepath.Join(root, ".verdi", ".gitignore"): []byte("data/\n"),
		filepath.Join(specDir, "spec.md"):           []byte(vocabProbeSpec),
		// The templates the model DECLARES (custom-story.md /
		// custom-feature.md): the store's own overrides, so template
		// resolution exercises the real tailored-store path
		// (spec/creation-form ac-3's override property in a browser).
		filepath.Join(root, ".verdi", "templates", "custom-story.md"):   []byte(vocabCustomStoryTemplate),
		filepath.Join(root, ".verdi", "templates", "custom-feature.md"): []byte(vocabCustomFeatureTemplate),
	}
	for path, content := range files {
		if err := os.WriteFile(path, content, 0o644); err != nil {
			return "", fmt.Errorf("writing %s: %w", path, err)
		}
	}

	if err := runGit(root, nil, "init", "--quiet", "--initial-branch=main"); err != nil {
		return "", err
	}
	if err := runGit(root, nil, "add", "-A"); err != nil {
		return "", err
	}
	if err := runGit(root, nil, "commit", "--quiet", "--no-verify", "-m", "vocab-rename store: manifest, model.yaml, one accepted feature"); err != nil {
		return "", err
	}
	if err := runGit("", nil, "init", "--bare", "--quiet", "--initial-branch=main", originDir); err != nil {
		return "", err
	}
	if err := runGit(root, nil, "remote", "add", "origin", originDir); err != nil {
		return "", err
	}
	if err := runGit(root, nil, "push", "--quiet", "--set-upstream", "origin", "main"); err != nil {
		return "", err
	}
	if err := runGit(root, nil, "remote", "set-head", "origin", "main"); err != nil {
		return "", err
	}

	// The authoring half: the draft feature spec on its design branch,
	// committed there (draft never lands on main — the same VL-004 posture
	// provisionBoard cites) and the checkout LEFT on the branch, so
	// boardspec.go's mode switch (status draft + branch != default) serves
	// /board/spec/vocab-draft in authoring mode.
	if err := runGit(root, nil, "checkout", "--quiet", "-b", vocabDraftBranch); err != nil {
		return "", err
	}
	draftDir := filepath.Join(root, ".verdi", "specs", "active", "vocab-draft")
	if err := os.MkdirAll(draftDir, 0o755); err != nil {
		return "", fmt.Errorf("creating vocab-draft spec dir: %w", err)
	}
	if err := os.WriteFile(filepath.Join(draftDir, "spec.md"), []byte(vocabDraftSpec), 0o644); err != nil {
		return "", fmt.Errorf("writing vocab-draft spec: %w", err)
	}
	if err := runGit(root, nil, "add", "-A"); err != nil {
		return "", err
	}
	if err := runGit(root, nil, "commit", "--quiet", "--no-verify", "-m", "design: vocab-draft authoring fixture"); err != nil {
		return "", err
	}
	if err := runGit(root, nil, "push", "--quiet", "--set-upstream", "origin", vocabDraftBranch); err != nil {
		return "", err
	}

	return root, nil
}
