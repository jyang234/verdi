// Real, built-binary end-to-end tests for `verdi init` (spec/init-wizard,
// ledger L-N5): mirrors model_test.go's/disposition_test.go's own style —
// driving the actual compiled binary, never a package-internal unit test
// standing in for it — over plain, non-git target directories (init
// touches no git state at all). The wizard path's interview is driven via
// this story's own disclosed stdin-script harness: a scripted answer
// sequence fed over a real OS pipe (cmd.Stdin, never a real terminal),
// with the disclosed, test-only VERDI_INIT_ASSUME_TTY=1 environment
// override standing in for the TTY predicate alone — chosen over a pty
// harness for hermetic, deterministic, dependency-free CI portability.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/initwizard"
	"github.com/jyang234/verdi/internal/model"
)

// runInitBinary execs the built verdi binary's "init" verb with cwd=dir,
// optional extra args, extra environment variables (appended to the
// child's inherited environment, so VERDI_INIT_ASSUME_TTY/
// VERDI_INIT_SIMULATE_CRASH_AFTER can be set without disturbing anything
// else), and an optional scripted stdin — capturing stdout/stderr
// separately, mirroring runModelCheckBinary's/runDispositionBinary's own
// pattern (model_test.go, disposition_test.go).
func runInitBinary(t *testing.T, bin, dir, stdin string, extraEnv []string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command(bin, append([]string{"init"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), extraEnv...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return outBuf.String(), errBuf.String(), ee.ExitCode()
		}
		t.Fatalf("running verdi init %v: %v", args, err)
	}
	return outBuf.String(), errBuf.String(), 0
}

// listDirEntries returns the sorted, relative-path listing of every file
// under dir (directories included) — used to prove "nothing at the real
// root" and "no leftover sibling temp directory" by comparing a full
// before/after snapshot rather than checking one hardcoded path.
func listDirEntries(t *testing.T, dir string) []string {
	t.Helper()
	var entries []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == dir {
			return nil
		}
		rel, rerr := filepath.Rel(dir, path)
		if rerr != nil {
			return rerr
		}
		entries = append(entries, rel)
		return nil
	})
	if err != nil {
		t.Fatalf("listing %s: %v", dir, err)
	}
	sort.Strings(entries)
	return entries
}

// wizardAllDefaultsScript answers every wizard prompt with its default:
// 9 blank lines (3 classes, 4 states, 2 verbs — RenameableIDs), "n" to
// the template-copy offer, "n" to the structural-request probe, "y" to
// confirm the write.
const wizardAllDefaultsScript = "\n\n\n\n\n\n\n\n\nn\nn\ny\n"

// TestInit_Bare_EmptyDir_CreatesMinimalSkeleton is obligation/init-wizard--
// ac-1--behavioral's happy path: bare `verdi init` in an empty directory
// creates EXACTLY .verdi/verdi.yaml with schema: verdi.layout/v1 and
// nothing else, and the result passes `verdi model check` cleanly,
// resolving to the canonical model (mirroring model_test.go's own
// TestModelCheck_NoModelYAML_OK witness).
func TestInit_Bare_EmptyDir_CreatesMinimalSkeleton(t *testing.T) {
	bin := buildVerdiBinary(t)
	dir := t.TempDir()

	stdout, stderr, code := runInitBinary(t, bin, dir, "", nil)
	if code != 0 {
		t.Fatalf("verdi init (bare, empty dir) exit = %d, want 0\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	got := listDirEntries(t, dir)
	want := []string{".verdi", filepath.Join(".verdi", "verdi.yaml")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("bare init tree = %v, want exactly %v (no model.yaml, no templates/, no specs/)", got, want)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".verdi", "verdi.yaml"))
	if err != nil {
		t.Fatalf("reading .verdi/verdi.yaml: %v", err)
	}
	if string(data) != initwizard.VerdiYAMLContent {
		t.Fatalf(".verdi/verdi.yaml = %q, want %q", data, initwizard.VerdiYAMLContent)
	}

	checkOut, checkErr, checkCode := runModelCheckBinary(t, bin, dir)
	if checkCode != 0 {
		t.Fatalf("verdi model check over the freshly-init'd store exit = %d, want 0\nstdout: %s\nstderr: %s", checkCode, checkOut, checkErr)
	}
	if !strings.HasPrefix(checkOut, "model: OK — verdi.model/v1, ") {
		t.Fatalf("verdi model check stdout = %q, want it to start with the canonical OK line", checkOut)
	}
}

// TestInit_RefusesExistingVerdiDir_Table is obligation/init-wizard--ac-1--
// behavioral's refusal half: BOTH the bare and --wizard path refuse, exit
// 2, against a target directory that already carries a .verdi/ entry —
// once with a full verdi.yaml manifest present, once with only a stray,
// otherwise-empty .verdi/ directory (W-3b: the predicate is "any .verdi/
// entry," not "an existing manifest") — naming what exists, and leaving
// the pre-existing tree completely byte-untouched.
func TestInit_RefusesExistingVerdiDir_Table(t *testing.T) {
	bin := buildVerdiBinary(t)

	cases := []struct {
		name  string
		setup func(t *testing.T, dir string)
	}{
		{
			name: "full manifest present",
			setup: func(t *testing.T, dir string) {
				if err := os.MkdirAll(filepath.Join(dir, ".verdi"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, ".verdi", "verdi.yaml"), []byte("schema: verdi.layout/v1\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "stray empty .verdi dir, no manifest",
			setup: func(t *testing.T, dir string) {
				if err := os.MkdirAll(filepath.Join(dir, ".verdi"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
		},
	}

	for _, tc := range cases {
		for _, args := range [][]string{{}, {"--wizard"}} {
			label := tc.name
			if len(args) > 0 {
				label += "/--wizard"
			} else {
				label += "/bare"
			}
			t.Run(label, func(t *testing.T) {
				dir := t.TempDir()
				tc.setup(t, dir)
				before := listDirEntries(t, dir)

				stdout, stderr, code := runInitBinary(t, bin, dir, "", []string{"VERDI_INIT_ASSUME_TTY=1"}, args...)
				if code != 2 {
					t.Fatalf("verdi init %v against %s exit = %d, want 2\nstdout: %s\nstderr: %s", args, tc.name, code, stdout, stderr)
				}
				if !strings.Contains(stderr, ".verdi") {
					t.Fatalf("refusal stderr = %q, want it to name the existing .verdi path", stderr)
				}

				after := listDirEntries(t, dir)
				if !reflect.DeepEqual(before, after) {
					t.Fatalf("refused init changed the pre-existing tree: before %v, after %v", before, after)
				}
			})
		}
	}
}

// TestInit_Wizard_NoTTY_Refuses is obligation/init-wizard--ac-2--
// behavioral's TTY-gate half: --wizard with stdin wired to a plain pipe
// and NO VERDI_INIT_ASSUME_TTY override exits 2 naming the missing TTY,
// writing nothing at all.
func TestInit_Wizard_NoTTY_Refuses(t *testing.T) {
	bin := buildVerdiBinary(t)
	dir := t.TempDir()

	stdout, stderr, code := runInitBinary(t, bin, dir, wizardAllDefaultsScript, nil, "--wizard")
	if code != 2 {
		t.Fatalf("verdi init --wizard (no TTY) exit = %d, want 2\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	lower := strings.ToLower(stderr)
	if !strings.Contains(lower, "tty") && !strings.Contains(lower, "terminal") {
		t.Fatalf("refusal stderr = %q, want it to mention the missing TTY/terminal", stderr)
	}
	if entries := listDirEntries(t, dir); len(entries) != 0 {
		t.Fatalf("verdi init --wizard (no TTY) wrote something: %v, want nothing", entries)
	}
}

// TestInit_Wizard_AllDefaults_MatchesBarePath is obligation/init-wizard--
// ac-2--behavioral's zero-divergence pin: a wizard run answering every
// prompt with its default produces the SAME store bare init would — no
// model.yaml at all.
func TestInit_Wizard_AllDefaults_MatchesBarePath(t *testing.T) {
	bin := buildVerdiBinary(t)
	dir := t.TempDir()

	stdout, stderr, code := runInitBinary(t, bin, dir, wizardAllDefaultsScript, []string{"VERDI_INIT_ASSUME_TTY=1"}, "--wizard")
	if code != 0 {
		t.Fatalf("verdi init --wizard (all defaults) exit = %d, want 0\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	got := listDirEntries(t, dir)
	want := []string{".verdi", filepath.Join(".verdi", "verdi.yaml")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("all-defaults wizard tree = %v, want exactly %v (no model.yaml)", got, want)
	}
}

// TestInit_Wizard_StructuralRequest_RefusesButContinues is obligation/
// init-wizard--ac-2--behavioral's frontier-refusal pin: answering "y" to
// the interview's structural-request probe must not abort the run — the
// combined output must name the frontier, and the store must still be
// created.
func TestInit_Wizard_StructuralRequest_RefusesButContinues(t *testing.T) {
	bin := buildVerdiBinary(t)
	dir := t.TempDir()

	script := strings.Repeat("\n", 9) + "n\n" + "y\n" /* structural: yes */ + "y\n" /* confirm write */

	stdout, stderr, code := runInitBinary(t, bin, dir, script, []string{"VERDI_INIT_ASSUME_TTY=1"}, "--wizard")
	if code != 0 {
		t.Fatalf("verdi init --wizard (structural request) exit = %d, want 0 (refused-but-continues, not aborted)\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if !strings.Contains(strings.ToLower(stdout), "frontier") {
		t.Fatalf("stdout does not name the frontier when a structural request is made:\n%s", stdout)
	}
	if _, err := os.Stat(filepath.Join(dir, ".verdi", "verdi.yaml")); err != nil {
		t.Fatalf("the store was not created despite the interview continuing past the structural request: %v", err)
	}
}

// TestInit_Wizard_RealRenames_AndTemplateCopy is obligation/init-wizard--
// ac-2--behavioral's positive-content pin: real, non-default renames for
// at least one class/state/verb id plus a "yes" to the template-set copy
// question land in the promoted store's model.yaml vocabulary: block and
// as local .verdi/templates/ override copies.
func TestInit_Wizard_RealRenames_AndTemplateCopy(t *testing.T) {
	bin := buildVerdiBinary(t)
	dir := t.TempDir()

	// Order: classes [feature, spike, story], states [accepted-pending-build,
	// closed, draft, superseded], verbs [accept, close].
	script := "Epic\n\nTask\n" +
		"\n\n\n\n" +
		"Sign off\n\n" +
		"y\n" + // copy templates
		"n\n" + // structural probe
		"y\n" // confirm write

	stdout, stderr, code := runInitBinary(t, bin, dir, script, []string{"VERDI_INIT_ASSUME_TTY=1"}, "--wizard")
	if code != 0 {
		t.Fatalf("verdi init --wizard (real renames) exit = %d, want 0\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	modelBytes, err := os.ReadFile(filepath.Join(dir, ".verdi", "model.yaml"))
	if err != nil {
		t.Fatalf("reading promoted model.yaml: %v", err)
	}
	decoded, err := model.DecodeModel(modelBytes)
	if err != nil {
		t.Fatalf("promoted model.yaml failed to decode: %v", err)
	}
	wantVocab := model.Vocabulary{
		Classes: map[string]string{"feature": "Epic", "story": "Task"},
		Verbs:   map[string]string{"accept": "Sign off"},
	}
	if !reflect.DeepEqual(decoded.Vocabulary, wantVocab) {
		t.Fatalf("promoted model.yaml Vocabulary = %+v, want %+v", decoded.Vocabulary, wantVocab)
	}

	for _, name := range []string{"feature.md", "story.md"} {
		if data, err := os.ReadFile(filepath.Join(dir, ".verdi", "templates", name)); err != nil || len(data) == 0 {
			t.Fatalf("expected a copied template override at .verdi/templates/%s: %v", name, err)
		}
	}
}

// TestInit_Wizard_MidInterviewAbort_LeavesNothing is obligation/init-
// wizard--ac-3--behavioral's abort pin: stdin ending before every prompt
// is answered exits 2 and leaves NOTHING under the target directory — no
// .verdi/, no leftover sibling temp directory.
func TestInit_Wizard_MidInterviewAbort_LeavesNothing(t *testing.T) {
	bin := buildVerdiBinary(t)
	dir := t.TempDir()

	// Only 3 of the 9 rename prompts answered, then stdin ends.
	truncated := "\n\n\n"

	stdout, stderr, code := runInitBinary(t, bin, dir, truncated, []string{"VERDI_INIT_ASSUME_TTY=1"}, "--wizard")
	if code != 2 {
		t.Fatalf("verdi init --wizard (truncated stdin) exit = %d, want 2\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if entries := listDirEntries(t, dir); len(entries) != 0 {
		t.Fatalf("mid-interview abort left something behind: %v, want nothing", entries)
	}
}

// TestInit_Wizard_SimulatedCrash_LeavesNothing is obligation/init-wizard--
// ac-3--behavioral's crash-injection pin: a scripted, otherwise-complete
// run driven with VERDI_INIT_SIMULATE_CRASH_AFTER=model.yaml set exits 2
// after that file is staged but before promotion, leaving nothing at the
// real root and no temp litter.
func TestInit_Wizard_SimulatedCrash_LeavesNothing(t *testing.T) {
	bin := buildVerdiBinary(t)
	dir := t.TempDir()

	script := "Epic\n" + strings.Repeat("\n", 8) + "n\nn\ny\n" // one real rename, otherwise complete

	stdout, stderr, code := runInitBinary(t, bin, dir, script, []string{"VERDI_INIT_ASSUME_TTY=1", "VERDI_INIT_SIMULATE_CRASH_AFTER=model.yaml"}, "--wizard")
	if code != 2 {
		t.Fatalf("verdi init --wizard (simulated crash after model.yaml) exit = %d, want 2\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if !strings.Contains(stderr, "model.yaml") {
		t.Fatalf("crash refusal stderr = %q, want it to name the simulated-crash file", stderr)
	}
	if entries := listDirEntries(t, dir); len(entries) != 0 {
		t.Fatalf("simulated crash left something behind: %v, want nothing at the real root", entries)
	}
}

// TestInit_Wizard_SimulatedCrash_AfterVerdiYAML_AlsoLeavesNothing proves
// the crash-injection hook fires at more than one staged file — an
// earlier crash point (right after verdi.yaml, before model.yaml is even
// staged) must be equally clean.
func TestInit_Wizard_SimulatedCrash_AfterVerdiYAML_AlsoLeavesNothing(t *testing.T) {
	bin := buildVerdiBinary(t)
	dir := t.TempDir()

	stdout, stderr, code := runInitBinary(t, bin, dir, wizardAllDefaultsScript, []string{"VERDI_INIT_ASSUME_TTY=1", "VERDI_INIT_SIMULATE_CRASH_AFTER=verdi.yaml"}, "--wizard")
	if code != 2 {
		t.Fatalf("verdi init --wizard (simulated crash after verdi.yaml) exit = %d, want 2\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if entries := listDirEntries(t, dir); len(entries) != 0 {
		t.Fatalf("simulated crash left something behind: %v, want nothing at the real root", entries)
	}
}

// TestInit_Wizard_PromotionIsSingleRename_AndDecodeComparesEqual is
// obligation/init-wizard--ac-3--behavioral's W-4/W-2 pin: after a real,
// successful run, no sibling temp directory is left behind (the whole
// tree is exactly the promoted .verdi/), and the promoted model.yaml
// decodes to a Model reflect.DeepEqual to what
// initwizard.CandidateModel computes for the SAME scripted vocabulary —
// the decode-compare-equal-to-the-interview's-own-intent property,
// proven end to end through the real subprocess's file output.
func TestInit_Wizard_PromotionIsSingleRename_AndDecodeComparesEqual(t *testing.T) {
	bin := buildVerdiBinary(t)
	dir := t.TempDir()

	script := "Epic\n" + strings.Repeat("\n", 8) + "n\nn\ny\n"

	stdout, stderr, code := runInitBinary(t, bin, dir, script, []string{"VERDI_INIT_ASSUME_TTY=1"}, "--wizard")
	if code != 0 {
		t.Fatalf("verdi init --wizard exit = %d, want 0\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	got := listDirEntries(t, dir)
	want := []string{
		".verdi",
		filepath.Join(".verdi", "model.yaml"),
		filepath.Join(".verdi", "verdi.yaml"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("post-promotion tree = %v, want exactly %v (no sibling temp directory left behind)", got, want)
	}

	modelBytes, err := os.ReadFile(filepath.Join(dir, ".verdi", "model.yaml"))
	if err != nil {
		t.Fatalf("reading promoted model.yaml: %v", err)
	}
	decoded, err := model.DecodeModel(modelBytes)
	if err != nil {
		t.Fatalf("promoted model.yaml failed to decode: %v", err)
	}
	want2 := initwizard.CandidateModel(model.Vocabulary{Classes: map[string]string{"feature": "Epic"}})
	if !reflect.DeepEqual(decoded, want2) {
		t.Fatalf("promoted model.yaml decode-compare mismatch:\ngot:  %+v\nwant: %+v", decoded, want2)
	}
}

// TestInit_UnknownArgument_UsageError proves a malformed invocation
// refuses cleanly rather than being silently ignored.
func TestInit_UnknownArgument_UsageError(t *testing.T) {
	bin := buildVerdiBinary(t)
	dir := t.TempDir()

	_, stderr, code := runInitBinary(t, bin, dir, "", nil, "--bogus")
	if code != 2 {
		t.Fatalf("verdi init --bogus exit = %d, want 2", code)
	}
	if stderr == "" {
		t.Fatal("verdi init --bogus produced no stderr message")
	}
}
