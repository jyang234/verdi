// Real, built-binary end-to-end tests for `verdi model check`
// (obligation/model-schema--ac-3--behavioral): mirrors close_test.go's
// own style — driving the actual compiled binary, never a package-
// internal unit test standing in for it — over a plain, non-git store
// root (model check touches no git state at all, matching disposition_
// test.go's writeDispositionStoreRoot precedent).
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/model"
)

// writeModelCheckStoreRoot builds a plain store root: verdi.yaml always,
// model.yaml only when modelYAML != "".
func writeModelCheckStoreRoot(t *testing.T, modelYAML string) string {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, ".verdi", "verdi.yaml"), []byte("schema: verdi.layout/v1\n"))
	if modelYAML != "" {
		writeTestFile(t, filepath.Join(root, ".verdi", "model.yaml"), []byte(modelYAML))
	}
	return root
}

// runModelCheckBinary execs the built verdi binary's "model check" verb
// with cwd=dir, capturing stdout/stderr separately — mirroring
// runDispositionBinary's exact pattern (disposition_test.go).
func runModelCheckBinary(t *testing.T, bin, dir string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command(bin, "model", "check")
	cmd.Dir = dir
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return outBuf.String(), errBuf.String(), ee.ExitCode()
		}
		t.Fatalf("running verdi model check: %v", err)
	}
	return outBuf.String(), errBuf.String(), 0
}

// TestModelCheck_NoModelYAML_OK is ac-3's absent-file case: no
// .verdi/model.yaml at all resolves to the embedded canonical default,
// exit 0, with an OK line naming the schema, canonical's own class/
// transition counts, and canonical's own digest.
func TestModelCheck_NoModelYAML_OK(t *testing.T) {
	bin := buildVerdiBinary(t)
	root := writeModelCheckStoreRoot(t, "")

	stdout, stderr, code := runModelCheckBinary(t, bin, root)
	if code != 0 {
		t.Fatalf("verdi model check (no model.yaml) exit = %d, want 0\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if !strings.HasPrefix(stdout, "model: OK — verdi.model/v1, ") {
		t.Fatalf("stdout = %q, want it to start with the OK line", stdout)
	}
	wantDigest, err := model.Canonical().Digest()
	if err != nil {
		t.Fatalf("model.Canonical().Digest(): %v", err)
	}
	if !strings.Contains(stdout, wantDigest) {
		t.Fatalf("stdout = %q, want it to contain the canonical model's own digest %q", stdout, wantDigest)
	}
	if !strings.Contains(stdout, "2 classes") || !strings.Contains(stdout, "4 transitions") {
		t.Fatalf("stdout = %q, want it to name canonical's 2 classes / 4 transitions", stdout)
	}
}

// vocabRenameFeatureTemplate and vocabRenameStoryTemplate back vocab-
// rename.yaml's renamed Class.Template filenames (custom-feature.md /
// custom-story.md) — minimal but real templates, standing in for a store
// that actually shipped the override its renamed filename promises
// (spec/scaffold-templates ac-3: model check now instantiates and
// strict-decodes every resolved template, so a renamed-but-unbacked
// filename is correctly a failure, not a documentation-only field —
// TestModelCheck_BrokenTemplate_NamesFile below pins that failure case).
//
// The story template carries the same {{if .Spike}} branch the canonical
// story.md does, so it renders a VALID spike variant as well as the
// non-spike one: model check now round-trips both variants of every story
// template (judged-spike-variant-unchecked-by-model-check), and a story
// template that could not render a valid spike — one that emits resolves
// edges without spike: true, say — is correctly a failure, not a valid
// rename (TestModelCheck_BrokenTemplateInSpikeBranch_Exit2_NamesFile pins
// that case). A complete story template is one that handles both.
const vocabRenameFeatureTemplate = `---
id: {{.Ref}}
kind: spec
title: {{printf "%q" .Title}}
owners: {{.Owners}}
class: feature
status: draft
problem: { text: "{{.Problem}}", anchor: problem }
outcome: { text: "{{.Outcome}}", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static] }
---
# {{.Title}}
`

const vocabRenameStoryTemplate = `---
id: {{.Ref}}
kind: spec
title: {{printf "%q" .Title}}
owners: {{.Owners}}
class: story
status: draft
story: {{.StoryRef}}
{{if .Spike}}spike: true
{{end}}problem: { text: "{{.Problem}}", anchor: problem }
outcome: { text: "{{.Outcome}}", anchor: outcome }
links:
{{range .Links}}  - { type: {{.Type}}, ref: {{printf "%q" .Ref}} }
{{end}}---
# {{.Title}}
`

// TestModelCheck_ValidVocabRename_OK is ac-3's valid-hand-written-
// model.yaml case: a manifest varying only vocabulary and per-class
// template filenames (dc-1's frontier) still exits 0, over ITS OWN
// counts and digest (not canonical's — proving the store's file, not
// the embedded default, was actually read) — AND over its own renamed
// templates, backed here by a real .verdi/templates/ override for each
// (ac-3's template round trip requires the file to actually exist).
func TestModelCheck_ValidVocabRename_OK(t *testing.T) {
	bin := buildVerdiBinary(t)
	vocabRenameYAML := readModelTestdata(t, "vocab-rename.yaml")
	root := writeModelCheckStoreRoot(t, vocabRenameYAML)
	writeTestFile(t, filepath.Join(root, ".verdi", "templates", "custom-feature.md"), []byte(vocabRenameFeatureTemplate))
	writeTestFile(t, filepath.Join(root, ".verdi", "templates", "custom-story.md"), []byte(vocabRenameStoryTemplate))

	stdout, stderr, code := runModelCheckBinary(t, bin, root)
	if code != 0 {
		t.Fatalf("verdi model check (vocab-rename) exit = %d, want 0\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if !strings.HasPrefix(stdout, "model: OK — verdi.model/v1, ") {
		t.Fatalf("stdout = %q, want it to start with the OK line", stdout)
	}

	decoded, err := model.DecodeModel([]byte(vocabRenameYAML))
	if err != nil {
		t.Fatalf("test setup: decoding vocab-rename.yaml: %v", err)
	}
	wantDigest, err := decoded.Digest()
	if err != nil {
		t.Fatalf("test setup: computing vocab-rename.yaml's digest: %v", err)
	}
	if !strings.Contains(stdout, wantDigest) {
		t.Fatalf("stdout = %q, want it to contain vocab-rename.yaml's OWN digest %q (proving the store's file was read, not the embedded default)", stdout, wantDigest)
	}
}

// TestModelCheck_BrokenTemplateSyntax_Exit2_NamesFile is spec/scaffold-
// templates ac-3's broken-template case (malformed syntax half): a store
// override under .verdi/templates/ with unparseable text/template syntax
// fails model check closed at exit 2 (a broken template is not a
// structural model deviation — Class.Template is frontier-exempt, so this
// is never the frontier's exit 1), naming the specific offending template
// file rather than a bare "model.yaml invalid" message.
func TestModelCheck_BrokenTemplateSyntax_Exit2_NamesFile(t *testing.T) {
	bin := buildVerdiBinary(t)
	root := writeModelCheckStoreRoot(t, "")
	writeTestFile(t, filepath.Join(root, ".verdi", "templates", "feature.md"), []byte("title: {{.Title\n"))

	stdout, stderr, code := runModelCheckBinary(t, bin, root)
	if code != 2 {
		t.Fatalf("verdi model check (malformed template syntax) exit = %d, want 2\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if !strings.Contains(stderr, "feature.md") {
		t.Fatalf("stderr = %q, want it to name the offending template file feature.md, never a bare \"model.yaml invalid\"", stderr)
	}
}

// TestModelCheck_BrokenTemplateDecode_Exit2_NamesFile is spec/scaffold-
// templates ac-3's broken-template case (failed-strict-decode half): a
// store override whose rendered OUTPUT is syntactically valid template
// source but decodes to a spec that fails strict decode (here, an unknown
// frontmatter field, KnownFields) also fails model check closed at exit
// 2, naming the offending template file.
func TestModelCheck_BrokenTemplateDecode_Exit2_NamesFile(t *testing.T) {
	bin := buildVerdiBinary(t)
	root := writeModelCheckStoreRoot(t, "")
	const brokenDecodeTemplate = `---
id: {{.Ref}}
kind: spec
title: {{printf "%q" .Title}}
owners: {{.Owners}}
class: story
status: draft
story: {{.StoryRef}}
bogus_unknown_field: 1
problem: { text: "{{.Problem}}", anchor: problem }
outcome: { text: "{{.Outcome}}", anchor: outcome }
links:
{{range .Links}}  - { type: {{.Type}}, ref: {{printf "%q" .Ref}} }
{{end}}---
# {{.Title}}
`
	writeTestFile(t, filepath.Join(root, ".verdi", "templates", "story.md"), []byte(brokenDecodeTemplate))

	stdout, stderr, code := runModelCheckBinary(t, bin, root)
	if code != 2 {
		t.Fatalf("verdi model check (template renders undecodable content) exit = %d, want 2\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if !strings.Contains(stderr, "story.md") {
		t.Fatalf("stderr = %q, want it to name the offending template file story.md, never a bare \"model.yaml invalid\"", stderr)
	}
}

// TestModelCheck_BrokenTemplateInSpikeBranch_Exit2_NamesFile is judged-
// spike-variant-unchecked-by-model-check's regression: a story.md override
// that decodes cleanly for the NON-spike variant but is broken only inside
// its {{if .Spike}} branch (here, an unknown frontmatter field the spike
// render emits). checkTemplates round-trips every variant a real scaffold
// consumer can render — design start renders the non-spike story, but
// stub-instantiate renders the spike story from a spike stub — so the
// breakage is caught at check time, not at some future spike scaffold's
// first use. The failure names the offending template file AND the spike
// variant (before this fix, model check rendered only the non-spike
// variant and this store passed clean).
func TestModelCheck_BrokenTemplateInSpikeBranch_Exit2_NamesFile(t *testing.T) {
	bin := buildVerdiBinary(t)
	root := writeModelCheckStoreRoot(t, "")
	const brokenSpikeBranchTemplate = `---
id: {{.Ref}}
kind: spec
title: {{printf "%q" .Title}}
owners: {{.Owners}}
class: story
status: draft
story: {{.StoryRef}}
{{if .Spike}}spike: true
bogus_spike_only_field: 1
{{end}}problem: { text: "{{.Problem}}", anchor: problem }
outcome: { text: "{{.Outcome}}", anchor: outcome }
{{if not .Spike}}acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static], anchor: ac-1 }
{{end}}links:
{{range .Links}}  - { type: {{.Type}}, ref: {{printf "%q" .Ref}} }
{{end}}---
# {{.Title}}
`
	writeTestFile(t, filepath.Join(root, ".verdi", "templates", "story.md"), []byte(brokenSpikeBranchTemplate))

	stdout, stderr, code := runModelCheckBinary(t, bin, root)
	if code != 2 {
		t.Fatalf("verdi model check (story template broken only in its spike branch) exit = %d, want 2\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if !strings.Contains(stderr, "story.md") {
		t.Fatalf("stderr = %q, want it to name the offending template file story.md", stderr)
	}
	if !strings.Contains(stderr, "spike") {
		t.Fatalf("stderr = %q, want it to name the spike variant (the check must say WHICH variant failed)", stderr)
	}
}

// TestModelCheck_BrokenTemplateInFeatureNoStoryRefBranch_Exit2_NamesFile is
// judged-model-check-feature-no-storyref-variant-unchecked's regression: a
// feature.md override that decodes cleanly WITH a story ref but is broken
// only inside its {{if .StoryRef}} empty branch (here, an unknown frontmatter
// field the no-story-ref render emits). checkTemplates round-trips every
// variant a real scaffold consumer can produce — design start --kind feature
// WITH a tracker ref renders the with-story-ref variant, a ref-less design
// start renders the no-story-ref variant (05 §CLI) — so the breakage is
// caught at check time, not at someone's first ref-less design start. The
// failure names the offending template file AND the no-story-ref variant
// (before this fix, model check rendered only the with-story-ref feature
// variant and this store passed clean).
func TestModelCheck_BrokenTemplateInFeatureNoStoryRefBranch_Exit2_NamesFile(t *testing.T) {
	bin := buildVerdiBinary(t)
	root := writeModelCheckStoreRoot(t, "")
	const brokenNoStoryRefTemplate = `---
id: {{.Ref}}
kind: spec
title: {{printf "%q" .Title}}
owners: {{.Owners}}
class: feature{{if .StoryRef}}
story: {{.StoryRef}}{{else}}
bogus_no_storyref_field: 1{{end}}
status: draft
problem: { text: "{{.Problem}}", anchor: problem }
outcome: { text: "{{.Outcome}}", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static], anchor: ac-1 }
---
# {{.Title}}
`
	writeTestFile(t, filepath.Join(root, ".verdi", "templates", "feature.md"), []byte(brokenNoStoryRefTemplate))

	stdout, stderr, code := runModelCheckBinary(t, bin, root)
	if code != 2 {
		t.Fatalf("verdi model check (feature template broken only in its no-story-ref branch) exit = %d, want 2\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if !strings.Contains(stderr, "feature.md") {
		t.Fatalf("stderr = %q, want it to name the offending template file feature.md", stderr)
	}
	if !strings.Contains(stderr, "no-story-ref") {
		t.Fatalf("stderr = %q, want it to name the no-story-ref variant (the check must say WHICH variant failed)", stderr)
	}
}

// TestModelCheck_TemplatePathEscape_Exit2_NamesRule proves the kernel's
// bare-filename rule reaches the built binary (judged-template-filename-
// escapes-templates-dir): a hand-written model.yaml whose class template
// escapes .verdi/templates/ (here "../../evil.md") fails model check closed
// at exit 2 — a kernel VALIDATION violation, grouped with every other
// "undecodable manifest" condition, never the frontier's exit 1 (a bad
// template value is not a structural model deviation) — with the error
// naming the offending class and the bare-filename rule, never a bare
// "model.yaml invalid" message.
func TestModelCheck_TemplatePathEscape_Exit2_NamesRule(t *testing.T) {
	bin := buildVerdiBinary(t)
	root := writeModelCheckStoreRoot(t, readModelTestdata(t, "viol-template-path-escape.yaml"))

	stdout, stderr, code := runModelCheckBinary(t, bin, root)
	if code != 2 {
		t.Fatalf("verdi model check (template path escape) exit = %d, want 2\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if !strings.Contains(stderr, "bare filename") {
		t.Fatalf("stderr = %q, want it to name the bare-filename rule", stderr)
	}
	if !strings.Contains(stderr, `class "feature"`) {
		t.Fatalf("stderr = %q, want it to name the offending class", stderr)
	}
}

// TestModelCheck_FrontierViolation_Exit1_PinnedText is ac-3's
// structurally-deviant case: exit 1, with the ONE pinned frontier error
// text printed VERBATIM — never a paraphrase.
func TestModelCheck_FrontierViolation_Exit1_PinnedText(t *testing.T) {
	bin := buildVerdiBinary(t)
	root := writeModelCheckStoreRoot(t, readModelTestdata(t, "viol-frontier-structural.yaml"))

	stdout, stderr, code := runModelCheckBinary(t, bin, root)
	if code != 1 {
		t.Fatalf("verdi model check (frontier violation) exit = %d, want 1\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	const pinned = "structural model configuration is behind the frontier (verdi.model/v1 accepts the canonical model with vocabulary/template changes only)"
	if !strings.Contains(stderr, pinned) {
		t.Fatalf("stderr = %q, want it to contain the pinned frontier text verbatim: %q", stderr, pinned)
	}
}

// TestModelCheck_KernelViolation_Exit2 proves ac-3's own (frozen)
// grouping: a KERNEL VALIDATION rule violation (here, an obligation kind
// outside the closed catalog) is "undecodable" and so exits 2 — NOT
// exit 1, despite this build's plan document's looser "exit 1 on
// validation/frontier failure" prose (a disclosed plan/spec conflict:
// spec+obligation win, per this build's own precedence rule).
func TestModelCheck_KernelViolation_Exit2(t *testing.T) {
	bin := buildVerdiBinary(t)
	root := writeModelCheckStoreRoot(t, readModelTestdata(t, "viol-kind-unknown.yaml"))

	stdout, stderr, code := runModelCheckBinary(t, bin, root)
	if code != 2 {
		t.Fatalf("verdi model check (kernel rule violation) exit = %d, want 2 (ac-3's own text: an undecodable manifest is operational trouble)\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if !strings.Contains(stderr, "obligation kind") {
		t.Fatalf("stderr = %q, want it to surface the kernel rule's own error", stderr)
	}
}

// TestModelCheck_StoreLessCwd_Exit2 proves a missing store is
// operational trouble (ac-3's own text names this explicitly).
func TestModelCheck_StoreLessCwd_Exit2(t *testing.T) {
	bin := buildVerdiBinary(t)
	dir := t.TempDir() // no .verdi/ anywhere under this tree

	stdout, stderr, code := runModelCheckBinary(t, bin, dir)
	if code != 2 {
		t.Fatalf("verdi model check (no store) exit = %d, want 2\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if stderr == "" {
		t.Fatal("stderr is empty, want a store-not-found error")
	}
}

// TestModelCheck_UnknownSubcommand_Exit2Usage proves an unrecognized
// `model` subcommand is a usage error (exit 2), never a silent no-op.
func TestModelCheck_UnknownSubcommand_Exit2Usage(t *testing.T) {
	bin := buildVerdiBinary(t)
	root := writeModelCheckStoreRoot(t, "")

	cmd := exec.Command(bin, "model", "bogus")
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	ee, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("running verdi model bogus: %v", err)
	}
	if ee.ExitCode() != 2 {
		t.Fatalf("verdi model bogus exit = %d, want 2\nstdout: %s\nstderr: %s", ee.ExitCode(), stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "usage: verdi model check") {
		t.Fatalf("stderr = %q, want it to mention 'usage: verdi model check'", stderr.String())
	}
}

// TestModelCheck_BareVerb_Exit2Usage proves `verdi model` with no
// subcommand at all is the same usage error, not a crash or a silent
// default.
func TestModelCheck_BareVerb_Exit2Usage(t *testing.T) {
	bin := buildVerdiBinary(t)
	root := writeModelCheckStoreRoot(t, "")

	cmd := exec.Command(bin, "model")
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	ee, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("running bare verdi model: %v", err)
	}
	if ee.ExitCode() != 2 {
		t.Fatalf("bare verdi model exit = %d, want 2\nstdout: %s\nstderr: %s", ee.ExitCode(), stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "usage: verdi model check") {
		t.Fatalf("stderr = %q, want it to mention 'usage: verdi model check'", stderr.String())
	}
}

// readModelTestdata reads a fixture from internal/model/testdata — this
// package's own tests reuse Task 5's committed fixtures rather than
// duplicating their content (CLAUDE.md: never copy-paste shared content).
func readModelTestdata(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "internal", "model", "testdata", name))
	if err != nil {
		t.Fatalf("reading internal/model/testdata/%s: %v", name, err)
	}
	return string(data)
}
