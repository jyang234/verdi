package main

// Built-binary tests for `verdi design import source|preview` (Task 4 CLI,
// spec-import-contract.md). Every test here runs the REAL compiled verdi
// binary (buildVerdiBinary) against a disposable fixturegit repository or
// plain temp directory — never the shared .build/bin/verdi or a user ATC
// checkout — and asserts exact exit codes, stdout/stderr content and (for
// preview) that the repository is left byte-for-byte unchanged. Apply and
// record scenarios live in designimportapply_test.go; both files share the
// helpers declared below plus designMutateRun/commandEnvironment/
// buildVerdiBinary from designmutate_test.go/serve_integration_test.go.

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/specimport"
)

// designImportSampleMarkdown reads the committed positive labeled fixture
// (cmd/verdi/testdata/specimport/sample.md): "# Sample Feature" with a
// two-line Problem, an Outcome, and two flat Acceptance Criteria bullets —
// the same shape internal/specimport/helpers_test.go's own
// validMarkdownSource pins at the unit level.
func designImportSampleMarkdown(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "specimport", "sample.md"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// designImportSampleMarkdownDir returns the absolute path of the directory
// holding sample.md, for `--root` in source tests that read the fixture
// from its committed location directly (no copy needed).
func designImportSampleMarkdownDir(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("testdata", "specimport"))
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

// designImportReadyRequest builds a well-formed markdown-v1 Request over
// the sample fixture: evidence-only mappings supply BOTH "static" and
// "attestation" evidence for ac-1/ac-2 (satisfying Normalize's own
// missing-evidence gate and lint's VL-006 feature-outcome attestation
// floor at once — a fresh draft feature spec requires attestation among
// each AC's declared evidence kinds), and RetainUnmapped disposes of
// everything else. mutate, if given, edits the built Request afterward
// (e.g. clearing Mappings for the blocking-preview case, or setting
// DeferStatements).
func designImportReadyRequest(t *testing.T, slug string, mutate ...func(*specimport.Request)) specimport.Request {
	t.Helper()
	req := specimport.Request{
		Schema:  specimport.RequestSchema,
		Target:  specimport.Target{Slug: slug, Class: "feature", Title: "Sample Feature"},
		Format:  specimport.FormatMarkdownV1,
		Primary: "source",
		Sources: []specimport.Source{{ID: "source", Label: "sample.md", Data: designImportSampleMarkdown(t)}},
		Mappings: []specimport.Mapping{
			{Target: "ac-1", Evidence: []string{"static", "attestation"}},
			{Target: "ac-2", Evidence: []string{"static", "attestation"}},
		},
		RetainUnmapped: true,
	}
	for _, m := range mutate {
		m(&req)
	}
	return req
}

// designImportRequestJSON marshals req with the standard encoder (not
// canonjson): these are wire-format decode inputs a real caller would
// produce, and DecodeRequest must accept ordinary encoding/json output,
// mirroring internal/specimport/helpers_test.go's own mustJSON rationale.
func designImportRequestJSON(t *testing.T, req specimport.Request) []byte {
	t.Helper()
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// designImportRequestWithUnknownField injects an unknown top-level field
// into an otherwise well-formed request's JSON — DecodeRequest's strict
// decode must refuse it.
func designImportRequestWithUnknownField(t *testing.T) []byte {
	t.Helper()
	data := designImportRequestJSON(t, designImportReadyRequest(t, "sample-feature-unknown-field"))
	if len(data) == 0 || data[len(data)-1] != '}' {
		t.Fatalf("expected request JSON to end with '}': %s", data)
	}
	out := make([]byte, 0, len(data)+24)
	out = append(out, data[:len(data)-1]...)
	out = append(out, []byte(`,"unknown_field":true}`)...)
	return out
}

// designImportRequestWithTrailingBytes appends trailing bytes after an
// otherwise well-formed request's single JSON value.
func designImportRequestWithTrailingBytes(t *testing.T) []byte {
	t.Helper()
	data := designImportRequestJSON(t, designImportReadyRequest(t, "sample-feature-trailing"))
	return append(data, []byte(" garbage")...)
}

// designImportRepo builds a hermetic, minimally valid store root on main
// with NO adopted policy authority at all (a bare verdi.yaml and a
// .verdi/.gitignore excluding data/) — the shape Preview must succeed
// against without any assistance policy, mirroring
// internal/specimport/servicefixture_test.go's own buildImportRepo (this
// package cannot import that unexported test helper, so this is an
// independent, deliberately identical copy).
func designImportRepo(t *testing.T) string {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/verdi.yaml": "schema: verdi.layout/v1\n",
			".verdi/.gitignore": "data/\n",
		},
		Message: "seed store",
	}})
	resolved, err := filepath.EvalSymlinks(repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.ToSlash(resolved)
}

// designImportPolicyRepo builds the same minimal store root, overlaid with
// internal/policyauthority/testdata/store's adopted policy constitution —
// go-toolchain.md's design_assistance mode replaced with mode (mirroring
// cmd/verdi/designmutate_test.go's designMutateStore overlay, but never
// reusing it: designMutateStore also checks out design/sample and writes
// an active spec, which apply's own create-only target must NOT already
// have). The fixture stays on main throughout.
func designImportPolicyRepo(t *testing.T, mode string) string {
	t.Helper()
	files := map[string]string{
		".verdi/verdi.yaml": "schema: verdi.layout/v1\n",
		".verdi/.gitignore": "data/\n",
	}
	source := filepath.Join("..", "..", "internal", "policyauthority", "testdata", "store")
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
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: files, Message: "adopt design import policy"}})
	resolved, err := filepath.EvalSymlinks(repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.ToSlash(resolved)
}

// runDesignImportBinary runs `verdi design import <args...>` against dir,
// with CI_DEFAULT_BRANCH pinned to main (as designMutateStore's own tests
// do) plus any extraEnv overrides — reusing designMutateRun/
// commandEnvironment from designmutate_test.go, never redefining them.
func runDesignImportBinary(t *testing.T, bin, dir string, stdin []byte, extraEnv map[string]string, args ...string) designMutateRun {
	t.Helper()
	env := map[string]string{"CI_DEFAULT_BRANCH": "main"}
	for k, v := range extraEnv {
		env[k] = v
	}
	cmd := exec.Command(bin, append([]string{"design", "import"}, args...)...)
	cmd.Dir = filepath.FromSlash(dir)
	cmd.Stdin = bytes.NewReader(stdin)
	cmd.Env = commandEnvironment(env)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if err == nil {
		return designMutateRun{stdout: stdout.String(), stderr: stderr.String()}
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return designMutateRun{stdout: stdout.String(), stderr: stderr.String(), code: exitErr.ExitCode()}
	}
	t.Fatalf("running verdi design import %v: %v", args, err)
	return designMutateRun{}
}

// designImportGitOutput runs a plain git command in dir and returns its
// combined output, failing the test on a non-zero exit.
func designImportGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = filepath.FromSlash(dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// designImportRepoState is the working-tree/index/refs projection this
// file's before/after assertions compare — proving Preview (always) and a
// refused Apply (in designimportapply_test.go) leave the repository
// byte-for-byte untouched: no checkout/index change and no new ref.
type designImportRepoState struct {
	head   string
	branch string
	refs   string
	status string
}

func designImportRepoStateNow(t *testing.T, dir string) designImportRepoState {
	t.Helper()
	return designImportRepoState{
		head:   strings.TrimSpace(designImportGitOutput(t, dir, "rev-parse", "HEAD")),
		branch: strings.TrimSpace(designImportGitOutput(t, dir, "symbolic-ref", "--short", "HEAD")),
		refs:   designImportGitOutput(t, dir, "for-each-ref", "--format=%(refname) %(objectname)"),
		status: designImportGitOutput(t, dir, "status", "--porcelain=v1", "-z", "--untracked-files=all"),
	}
}

// designImportBranchExists reports whether dir has a local
// refs/heads/<branch> ref.
func designImportBranchExists(t *testing.T, dir, branch string) bool {
	t.Helper()
	cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	cmd.Dir = filepath.FromSlash(dir)
	return cmd.Run() == nil
}

// TestDesignImportDispatchBuiltBinary proves the missing/unknown `design
// import` subcommand prints the whole design usage line (unchanged
// convention: any unrecognized design subcommand does this) and that the
// usage line now names the import grammar this task adds.
func TestDesignImportDispatchBuiltBinary(t *testing.T) {
	t.Parallel()
	bin := buildVerdiBinary(t)
	root := designImportRepo(t)

	for _, args := range [][]string{
		{"import"},
		{"import", "bogus"},
	} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			run := runDesignSubBinary(t, bin, root, args...)
			if run.code != 2 || run.stdout != "" {
				t.Fatalf("dispatch %v = %+v", args, run)
			}
			if !strings.Contains(run.stderr, "usage: verdi design start") {
				t.Fatalf("dispatch %v stderr = %q, want the whole design usage line", args, run.stderr)
			}
			if !strings.Contains(run.stderr, "verdi design import source") ||
				!strings.Contains(run.stderr, "verdi design import preview") ||
				!strings.Contains(run.stderr, "verdi design import apply") ||
				!strings.Contains(run.stderr, "verdi design import record") {
				t.Fatalf("dispatch %v stderr = %q, want the import grammar named", args, run.stderr)
			}
		})
	}
}

// TestDesignImportSourceBuiltBinary exercises `verdi design import
// source`: exact Source JSON for a ranged read, every unsafe-path
// refusal, and malformed-flag refusals. Every failure asserts empty
// stdout.
func TestDesignImportSourceBuiltBinary(t *testing.T) {
	t.Parallel()
	bin := buildVerdiBinary(t)
	fixtureDir := designImportSampleMarkdownDir(t)

	t.Run("ranged read returns exact Source JSON", func(t *testing.T) {
		run := runDesignImportBinary(t, bin, fixtureDir, nil, nil,
			"source", "--root", fixtureDir, "--file", "sample.md", "--start-line", "5", "--end-line", "6")
		if run.code != 0 || run.stderr != "" {
			t.Fatalf("ranged source = %+v", run)
		}
		if !bytes.HasSuffix([]byte(run.stdout), []byte("\n")) {
			t.Fatalf("stdout is not newline-terminated: %q", run.stdout)
		}
		var src specimport.Source
		if err := json.Unmarshal([]byte(run.stdout), &src); err != nil {
			t.Fatalf("decoding source: %v\n%s", err, run.stdout)
		}
		want := designImportSampleMarkdown(t)
		if src.ID != "sample" || src.Label != "sample.md" || src.StartLine != 5 || src.EndLine != 6 || !bytes.Equal(src.Data, want) {
			t.Fatalf("source = %+v (want id=sample label=sample.md start=5 end=6 and full fixture bytes)", src)
		}
	})

	t.Run("whole-file read omits start/end line", func(t *testing.T) {
		run := runDesignImportBinary(t, bin, fixtureDir, nil, nil, "source", "--root", fixtureDir, "--file", "sample.md")
		if run.code != 0 || run.stderr != "" {
			t.Fatalf("whole-file source = %+v", run)
		}
		var decoded map[string]json.RawMessage
		if err := json.Unmarshal([]byte(run.stdout), &decoded); err != nil {
			t.Fatalf("decoding source: %v\n%s", err, run.stdout)
		}
		if _, ok := decoded["start_line"]; ok {
			t.Fatalf("whole-file source carries start_line: %s", run.stdout)
		}
		if _, ok := decoded["end_line"]; ok {
			t.Fatalf("whole-file source carries end_line: %s", run.stdout)
		}
	})

	unsafe := []struct {
		name    string
		setup   func(t *testing.T) (root, file string)
		wantErr string
	}{
		{
			name: "traversal component",
			setup: func(t *testing.T) (string, string) {
				return t.TempDir(), "../x"
			},
			wantErr: "invalid-source",
		},
		{
			name: "absolute path",
			setup: func(t *testing.T) (string, string) {
				return t.TempDir(), "/etc/passwd"
			},
			wantErr: "invalid-source",
		},
		{
			name: "symlink component",
			setup: func(t *testing.T) (string, string) {
				dir := t.TempDir()
				if err := os.WriteFile(filepath.Join(dir, "target.md"), []byte("hi"), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(dir, "target.md"), filepath.Join(dir, "link.md")); err != nil {
					t.Fatal(err)
				}
				return dir, "link.md"
			},
			wantErr: "invalid-source",
		},
		{
			name: "directory target",
			setup: func(t *testing.T) (string, string) {
				dir := t.TempDir()
				if err := os.Mkdir(filepath.Join(dir, "subdir"), 0o755); err != nil {
					t.Fatal(err)
				}
				return dir, "subdir"
			},
			wantErr: "invalid-source",
		},
		{
			name: "missing file",
			setup: func(t *testing.T) (string, string) {
				return t.TempDir(), "does-not-exist.md"
			},
			wantErr: "io-failure",
		},
	}
	for _, tc := range unsafe {
		t.Run(tc.name, func(t *testing.T) {
			root, file := tc.setup(t)
			run := runDesignImportBinary(t, bin, root, nil, nil, "source", "--root", root, "--file", file)
			if run.code != 2 || run.stdout != "" {
				t.Fatalf("%s = %+v", tc.name, run)
			}
			if !strings.HasPrefix(run.stderr, "design import source: "+tc.wantErr+":") {
				t.Fatalf("%s stderr = %q, want prefix design import source: %s:", tc.name, run.stderr, tc.wantErr)
			}
		})
	}

	malformed := []struct {
		name string
		args []string
	}{
		{"missing --root", []string{"source", "--file", "sample.md"}},
		{"missing --file", []string{"source", "--root", fixtureDir}},
		{"unknown flag", []string{"source", "--root", fixtureDir, "--file", "sample.md", "--bogus", "x"}},
		{"duplicate --root", []string{"source", "--root", fixtureDir, "--root", fixtureDir, "--file", "sample.md"}},
		{"lone --start-line", []string{"source", "--root", fixtureDir, "--file", "sample.md", "--start-line", "1"}},
		{"lone --end-line", []string{"source", "--root", fixtureDir, "--file", "sample.md", "--end-line", "1"}},
		{"non-integer start line", []string{"source", "--root", fixtureDir, "--file", "sample.md", "--start-line", "abc", "--end-line", "2"}},
		{"non-positive start line", []string{"source", "--root", fixtureDir, "--file", "sample.md", "--start-line", "0", "--end-line", "2"}},
	}
	for _, tc := range malformed {
		t.Run(tc.name, func(t *testing.T) {
			run := runDesignImportBinary(t, bin, fixtureDir, nil, nil, tc.args...)
			if run.code != 2 || run.stdout != "" || !strings.HasPrefix(run.stderr, "design import source: invalid-request:") {
				t.Fatalf("%s = %+v", tc.name, run)
			}
		})
	}
}

// TestDesignImportPreviewBuiltBinary exercises `verdi design import
// preview`: a ready preview with NO adopted assistance policy leaves the
// repository untouched, a blocking (missing-evidence) preview is
// structured exit 1, and every malformed request/flag case is exit 2.
func TestDesignImportPreviewBuiltBinary(t *testing.T) {
	t.Parallel()
	bin := buildVerdiBinary(t)

	t.Run("ready preview requires no assistance policy and is read-only", func(t *testing.T) {
		root := designImportRepo(t)
		before := designImportRepoStateNow(t, root)
		reqBytes := designImportRequestJSON(t, designImportReadyRequest(t, "sample-feature-preview"))

		run := runDesignImportBinary(t, bin, root, reqBytes, nil, "preview", "--request", "-")
		if run.code != 0 || run.stderr != "" {
			t.Fatalf("ready preview = %+v", run)
		}
		var result specimport.PreviewResult
		if err := json.Unmarshal([]byte(run.stdout), &result); err != nil {
			t.Fatalf("decoding preview result: %v\n%s", err, run.stdout)
		}
		if result.Schema != specimport.PreviewResultSchema || !result.Ready || len(result.Digest) != 64 {
			t.Fatalf("preview result = %+v", result)
		}
		for _, f := range result.Findings {
			if f.Blocking {
				t.Fatalf("ready preview carries a blocking finding: %+v", f)
			}
		}

		after := designImportRepoStateNow(t, root)
		if before != after {
			t.Fatalf("preview changed the repository:\nbefore: %+v\nafter:  %+v", before, after)
		}
	})

	t.Run("missing evidence is a structured blocking preview at exit 1", func(t *testing.T) {
		root := designImportRepo(t)
		req := designImportReadyRequest(t, "sample-feature-blocked", func(r *specimport.Request) { r.Mappings = nil })

		run := runDesignImportBinary(t, bin, root, designImportRequestJSON(t, req), nil, "preview", "--request", "-")
		if run.code != 1 || run.stderr != "" || run.stdout == "" {
			t.Fatalf("blocking preview = %+v", run)
		}
		var result specimport.PreviewResult
		if err := json.Unmarshal([]byte(run.stdout), &result); err != nil {
			t.Fatalf("decoding blocking preview: %v\n%s", err, run.stdout)
		}
		if result.Ready {
			t.Fatalf("blocking preview reported ready: %+v", result)
		}
		found := false
		for _, f := range result.Findings {
			if f.Blocking && f.Code == specimport.FindingMissingEvidence {
				found = true
			}
		}
		if !found {
			t.Fatalf("blocking preview missing a blocking missing-evidence finding: %+v", result.Findings)
		}
	})

	t.Run("malformed request and flags", func(t *testing.T) {
		root := designImportRepo(t)
		cases := []struct {
			name  string
			args  []string
			stdin []byte
		}{
			{"null request", []string{"preview", "--request", "-"}, []byte("null")},
			{"unknown field", []string{"preview", "--request", "-"}, designImportRequestWithUnknownField(t)},
			{"trailing bytes", []string{"preview", "--request", "-"}, designImportRequestWithTrailingBytes(t)},
			{"oversize envelope", []string{"preview", "--request", "-"}, bytes.Repeat([]byte("a"), specimport.MaxEnvelopeBytes+1)},
			{"missing --request", []string{"preview"}, nil},
			{"duplicate flag", []string{"preview", "--request", "-", "--request", "-"}, []byte("null")},
			{"unknown flag", []string{"preview", "--bogus", "x"}, nil},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				run := runDesignImportBinary(t, bin, root, tc.stdin, nil, tc.args...)
				if run.code != 2 || run.stdout != "" || !strings.HasPrefix(run.stderr, "design import preview: invalid-request:") {
					t.Fatalf("%s = %+v", tc.name, run)
				}
			})
		}
	})
}
