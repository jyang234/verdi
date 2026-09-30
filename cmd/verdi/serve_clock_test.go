// serve_clock_test.go: `verdi serve`'s VERDI_NOW seam (SI-296,
// spec/index-data ac-2/ac-3) — the pure parser's table, then the shipped
// binary's three behavioral paths: set (the served index's ages and quiet
// marks are decided against the fixed instant, and the pages disclose it),
// unset (the wall clock, nothing disclosed), and malformed (exit 2 naming
// the variable, before any server effect).
package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/readinessload"
	"github.com/jyang234/verdi/internal/store"
)

func TestWorkbenchClock(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		raw  string
		// wantAt is the fixed instant; zero means no clock (the wall clock).
		wantAt time.Time
		// wantInText is the instant as the disclosure names it.
		wantInText string
		wantErr    bool
	}{
		{name: "unset: no fixed clock, nothing disclosed", raw: ""},
		{
			name:       "a UTC instant",
			raw:        "2024-06-15T12:00:00Z",
			wantAt:     time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC),
			wantInText: "2024-06-15T12:00:00Z",
		},
		{
			name:       "an offset instant is the same moment, named as given",
			raw:        "2024-06-15T14:00:00+02:00",
			wantAt:     time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC),
			wantInText: "2024-06-15T14:00:00+02:00",
		},
		{
			name:       "fractional seconds are RFC 3339 too",
			raw:        "2024-06-15T12:00:00.5Z",
			wantAt:     time.Date(2024, 6, 15, 12, 0, 0, 500_000_000, time.UTC),
			wantInText: "2024-06-15T12:00:00.5Z",
		},
		// B1-RR3: the four places Go's time.RFC3339 layout departs from a
		// strict RFC 3339 reading, pinned so any change is deliberate.
		{
			name:       "Go's layout accepts a comma decimal separator (named with a dot)",
			raw:        "2024-06-15T12:00:00,5Z",
			wantAt:     time.Date(2024, 6, 15, 12, 0, 0, 500_000_000, time.UTC),
			wantInText: "2024-06-15T12:00:00.5Z",
		},
		{
			name:       "Go's layout accepts a +24:00 offset",
			raw:        "2024-06-15T12:00:00+24:00",
			wantAt:     time.Date(2024, 6, 14, 12, 0, 0, 0, time.UTC),
			wantInText: "2024-06-15T12:00:00+24:00",
		},
		{name: "Go's layout refuses a lower-case t", raw: "2024-06-15t12:00:00Z", wantErr: true},
		{name: "Go's layout refuses a lower-case z", raw: "2024-06-15T12:00:00z", wantErr: true},
		{name: "Go's layout refuses the leap second :60", raw: "2016-12-31T23:59:60Z", wantErr: true},
		{name: "a relative word", raw: "yesterday", wantErr: true},
		{name: "a bare day", raw: "2024-06-15", wantErr: true},
		{name: "no offset", raw: "2024-06-15T12:00:00", wantErr: true},
		{name: "a space for the T", raw: "2024-06-15 12:00:00Z", wantErr: true},
		{name: "leading space", raw: " 2024-06-15T12:00:00Z", wantErr: true},
		{name: "trailing space", raw: "2024-06-15T12:00:00Z ", wantErr: true},
		{name: "month 13", raw: "2024-13-01T00:00:00Z", wantErr: true},
		{name: "unix seconds", raw: "1718452800", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			served, err := workbenchClock(tt.raw)
			clock, disclosed := served.now, served.disclosure
			if tt.wantErr {
				if err == nil {
					t.Fatalf("workbenchClock(%q) = nil error, want one", tt.raw)
				}
				if !strings.Contains(err.Error(), "VERDI_NOW") {
					t.Fatalf("error %q does not name the variable VERDI_NOW", err)
				}
				if clock != nil || disclosed != nil {
					t.Fatalf("workbenchClock(%q) returned a clock or disclosure alongside its error", tt.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("workbenchClock(%q): %v", tt.raw, err)
			}
			if tt.wantAt.IsZero() {
				if clock != nil || disclosed != nil {
					t.Fatalf("workbenchClock(%q) = (clock %v, disclosure %+v), want neither (the wall clock, undisclosed)", tt.raw, clock != nil, disclosed)
				}
				return
			}
			if clock == nil || disclosed == nil {
				t.Fatalf("workbenchClock(%q) = (clock %v, disclosure %+v), want both", tt.raw, clock != nil, disclosed)
			}
			for i := 0; i < 2; i++ {
				if got := clock(); !got.Equal(tt.wantAt) {
					t.Fatalf("clock() call %d = %v, want the fixed %v", i, got, tt.wantAt)
				}
			}
			if disclosed.Source != fixedClockSource || disclosed.Scope != "" {
				t.Fatalf("disclosure = %+v, want source %s with no scope (checkout-wide)", disclosed, fixedClockSource)
			}
			for _, want := range []string{"VERDI_NOW", tt.wantInText, "fixed"} {
				if !strings.Contains(disclosed.Text, want) {
					t.Errorf("disclosure text %q does not carry %q", disclosed.Text, want)
				}
			}
		})
	}
}

// clockProbeDraftDate is the probe draft's pinned committer date,
// 2024-01-15T00:00:00Z, in git's form and in the canonical form the
// served carrier reads.
const (
	clockProbeCommitDate = "1705276800 +0000"
	clockProbeWantDate   = "2024-01-15T00:00:00+00:00"
)

// newClockProbeStore is a real store with one design-branch draft whose
// tip is pinned at clockProbeCommitDate, main left checked out.
func newClockProbeStore(t *testing.T) string {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files:   map[string]string{".verdi/verdi.yaml": "schema: verdi.layout/v1\n", ".verdi/.gitignore": "data/\n"},
		Message: "store root",
	}})
	gitIn := func(env []string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo.Dir
		cmd.Env = append(os.Environ(), env...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	gitIn(nil, "checkout", "--quiet", "-b", "design/clock-probe")
	dir := filepath.Join(repo.Dir, ".verdi", "specs", "active", "clock-probe")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	spec := "---\nid: spec/clock-probe\nkind: spec\nclass: component\ntitle: \"Clock probe\"\nstatus: draft\nowners: [platform-team]\n---\n# Clock probe\n"
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(nil, "add", "-A")
	gitIn([]string{"GIT_AUTHOR_DATE=" + clockProbeCommitDate, "GIT_COMMITTER_DATE=" + clockProbeCommitDate}, "commit", "--quiet", "--no-verify", "-m", "clock probe draft")
	gitIn(nil, "checkout", "--quiet", "main")
	return repo.Dir
}

// serveEnv is the ambient environment with VERDI_NOW removed, plus
// VERDI_NOW=now when now is non-empty.
func serveEnv(now string) []string {
	env := make([]string, 0, len(os.Environ())+1)
	for _, kv := range os.Environ() {
		if key, _, _ := strings.Cut(kv, "="); key != fixedClockEnv {
			env = append(env, kv)
		}
	}
	if now != "" {
		env = append(env, fixedClockEnv+"="+now)
	}
	return env
}

var workbenchAddrRe = regexp.MustCompile(`serve: workbench at (http://\S+)`)

// startClockServe starts the built binary's serve over root under env and
// returns the workbench base URL.
func startClockServe(t *testing.T, root string, env []string) string {
	t.Helper()
	bin := buildVerdiBinary(t)
	cmd := exec.Command(bin, "serve", "--http", "127.0.0.1:0")
	cmd.Dir = root
	cmd.Env = env
	var out syncBuffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting verdi serve: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		_ = cmd.Wait()
	})
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if m := workbenchAddrRe.FindStringSubmatch(out.String()); m != nil {
			return m[1]
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("verdi serve never reported its workbench address:\n%s", out.String())
	return ""
}

func getBody(t *testing.T, url string) string {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading %s: %v", url, err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s = %d:\n%s", url, resp.StatusCode, body)
	}
	return string(body)
}

// probeEntryAttr returns attr's value on the probe draft's directory <li>,
// and whether it is present.
func probeEntryAttr(t *testing.T, page, attr string) (string, bool) {
	t.Helper()
	tag := regexp.MustCompile(`<li class="dir-entry[^"]*" data-testid="dir-entry-clock-probe"[^>]*>`).FindString(page)
	if tag == "" {
		t.Fatalf("no directory entry for the clock probe in:\n%s", page)
	}
	m := regexp.MustCompile(` ` + regexp.QuoteMeta(attr) + `="([^"]*)"`).FindStringSubmatch(tag)
	if m == nil {
		return "", false
	}
	return m[1], true
}

const fixedClockDisclosureID = `data-disclosure-id="serve:fixed-clock"`

// TestServe_VerdiNowSet_FixesTheServedClock: with VERDI_NOW three days
// after the probe draft's tip, the SERVED index reads it not quiet — the
// wall clock (years later) would read it quiet — and the disclosures page
// names the fixed instant.
func TestServe_VerdiNowSet_FixesTheServedClock(t *testing.T) {
	t.Parallel()
	root := newClockProbeStore(t)
	base := startClockServe(t, root, serveEnv("2024-01-18T00:00:00Z"))

	home := getBody(t, base+"/")
	if got, _ := probeEntryAttr(t, home, "data-last-change"); got != clockProbeWantDate {
		t.Errorf("data-last-change = %q, want the pinned tip date %q", got, clockProbeWantDate)
	}
	if got, ok := probeEntryAttr(t, home, "data-quiet"); !ok || got != "false" {
		t.Errorf("data-quiet = %q (present %v), want \"false\" three days after the tip under VERDI_NOW", got, ok)
	}
	disclosures := getBody(t, base+"/disclosures")
	if !strings.Contains(disclosures, fixedClockDisclosureID) || !strings.Contains(disclosures, "2024-01-18T00:00:00Z") {
		t.Errorf("the disclosures page does not disclose the fixed clock and its instant:\n%s", disclosures)
	}
}

// TestServe_VerdiNowUnset_UsesTheWallClock: without VERDI_NOW the served
// index decides quiet against the wall clock at render time (the probe tip
// is years old, so quiet) and nothing is disclosed about the clock.
func TestServe_VerdiNowUnset_UsesTheWallClock(t *testing.T) {
	t.Parallel()
	root := newClockProbeStore(t)
	base := startClockServe(t, root, serveEnv(""))

	home := getBody(t, base+"/")
	if got, ok := probeEntryAttr(t, home, "data-quiet"); !ok || got != "true" {
		t.Errorf("data-quiet = %q (present %v), want \"true\": the tip is years before the wall clock", got, ok)
	}
	if disclosures := getBody(t, base+"/disclosures"); strings.Contains(disclosures, fixedClockDisclosureID) {
		t.Errorf("the disclosures page discloses a fixed clock without VERDI_NOW:\n%s", disclosures)
	}
}

// TestServe_VerdiNowMalformed_ExitsTwo: a malformed VERDI_NOW is an
// operational error — exit 2, naming the variable — raised before any
// server effect (no writer lock taken, no workbench bound).
func TestServe_VerdiNowMalformed_ExitsTwo(t *testing.T) {
	t.Parallel()
	bin := buildVerdiBinary(t)
	root := newIntegrationStoreRoot(t)
	cmd := exec.Command(bin, "serve", "--http", "127.0.0.1:0")
	cmd.Dir = root
	cmd.Env = serveEnv("next tuesday")
	var stdout, stderr syncBuffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting verdi serve: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	var waitErr error
	select {
	case waitErr = <-done:
	case <-time.After(20 * time.Second):
		_ = cmd.Process.Kill()
		<-done
		t.Fatalf("verdi serve with a malformed VERDI_NOW kept running; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	var exitErr *exec.ExitError
	if !errors.As(waitErr, &exitErr) || exitErr.ExitCode() != 2 {
		t.Fatalf("exit = %v, want exit status 2; stderr=%q", waitErr, stderr.String())
	}
	if !strings.Contains(stderr.String(), "VERDI_NOW") {
		t.Errorf("stderr %q does not name VERDI_NOW", stderr.String())
	}
	if strings.Contains(stdout.String(), "workbench at") {
		t.Errorf("the workbench was bound before the malformed clock was refused: %q", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".verdi", "data", "writer.lock")); !os.IsNotExist(err) {
		t.Errorf("writer.lock exists (stat err %v): the refusal came after a server effect", err)
	}
}

// TestServeVerdiNowResolvedBeforeTheWarmUp is B1-RR1 (SI-296: a malformed
// value is refused at startup): cmdServeWithDeps resolves VERDI_NOW before
// the --context-request readiness warm-up and before any other effect,
// then carries the resolved clock and its disclosure into the run. Driven
// through the REAL warm-up (readinessLoadBuilder) over a store with a
// configured judge that counts its launches, so an ordering regression
// would visibly launch the judge, take the transient writer lock, and
// write .verdi/data/cache — none of which a malformed value may do.
func TestServeVerdiNowResolvedBeforeTheWarmUp(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		verdiNow string
		wantCode int
		// wantWarm: the warm-up ran (one build, one judge launch).
		wantWarm bool
		// wantAt: the clock the run received; zero means none (the wall
		// clock).
		wantAt time.Time
	}{
		{name: "unset-warms-up-then-runs-on-the-wall-clock", verdiNow: "", wantCode: 0, wantWarm: true},
		{name: "valid-warms-up-then-runs-on-the-fixed-clock", verdiNow: "2024-06-15T12:00:00Z", wantCode: 0, wantWarm: true, wantAt: time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)},
		{name: "malformed-refused-before-any-effect", verdiNow: "next tuesday", wantCode: 2},
	}
	// Subtest names are single tokens: t.TempDir embeds them in the judge
	// script's path, and the judge command line is whitespace-split.
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := buildContextCompileRepo(t, map[string]string{
				".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
			})
			counterPath := filepath.Join(t.TempDir(), "judge-calls")
			judge := writeContextConflictJudge(t, "c=$(cat '"+counterPath+"' 2>/dev/null || echo 0); echo $((c+1)) > '"+counterPath+"'; printf '%s\\n' '"+contextConflictNoConflictJudgeResult+"'")
			configureContextConflictJudge(t, repo, judge, 0)
			checkoutBranch(t, repo.Dir, "design/feature-alpha")
			requestPath := writeContextRequestFile(t, repo.Dir, "readiness-request.json", contextRequestBytes(t, "spec/feature-alpha", contextcompile.PhaseDesign, nil))
			dataDir := filepath.Join(repo.Dir, ".verdi", "data")
			if _, err := os.Stat(dataDir); !os.IsNotExist(err) {
				t.Fatalf("precondition: %s exists before serve (stat err %v)", dataDir, err)
			}

			builds := 0
			entered := false
			var got servedClock
			deps := serveCommandDeps{
				findRoot: func(string) (string, error) { return repo.Dir, nil },
				getenv: func(key string) string {
					if key == fixedClockEnv {
						return tt.verdiNow
					}
					return ""
				},
				readiness: readinessSnapshotBuilderFunc(func(ctx context.Context, root, path string) (string, *readinessload.PredecodedRequest, error) {
					builds++
					return readinessLoadBuilder{}.Build(ctx, root, path)
				}),
				run: func(_, _ string, _ readinessload.Loader, _ string, clock servedClock, _, _ io.Writer) int {
					entered = true
					got = clock
					return 0
				},
			}
			var stdout, stderr bytes.Buffer
			code := cmdServeWithDeps([]string{"--http", "127.0.0.1:0", "--context-request", requestPath}, &stdout, &stderr, deps)
			if code != tt.wantCode {
				t.Fatalf("exit = %d, want %d; stderr=%q", code, tt.wantCode, stderr.String())
			}

			judgeCalls, judgeErr := os.ReadFile(counterPath)
			if !tt.wantWarm {
				if !strings.Contains(stderr.String(), "VERDI_NOW") {
					t.Errorf("stderr %q does not name VERDI_NOW", stderr.String())
				}
				if builds != 0 {
					t.Errorf("the readiness warm-up builder ran %d times before the malformed clock was refused", builds)
				}
				if !os.IsNotExist(judgeErr) {
					t.Errorf("the judge was launched (calls %q, err %v) before the malformed clock was refused", judgeCalls, judgeErr)
				}
				if entered {
					t.Error("the run was entered with a malformed VERDI_NOW")
				}
				if stdout.Len() != 0 {
					t.Errorf("stdout = %q, want nothing (no warm-up line)", stdout.String())
				}
				// No writer lock, no data zone, no cache file: nothing under
				// .verdi/data exists at all.
				if _, err := os.Stat(dataDir); !os.IsNotExist(err) {
					t.Errorf("%s exists after the refusal (stat err %v): a lock or cache was written first", dataDir, err)
				}
				if _, err := os.Stat(store.WriterLockPath(repo.Dir)); !os.IsNotExist(err) {
					t.Errorf("a writer lock exists after the refusal (stat err %v)", err)
				}
				return
			}

			if builds != 1 || strings.TrimSpace(string(judgeCalls)) != "1" {
				t.Errorf("warm-up builds = %d, judge calls = %q (err %v), want exactly one of each", builds, judgeCalls, judgeErr)
			}
			if !entered {
				t.Fatal("the run was never entered")
			}
			if tt.wantAt.IsZero() {
				if got.now != nil || got.disclosure != nil {
					t.Errorf("run received a fixed clock (%v) or disclosure (%+v) with VERDI_NOW unset", got.now != nil, got.disclosure)
				}
				return
			}
			if got.now == nil {
				t.Errorf("run received no clock, want the fixed %v", tt.wantAt)
			} else if at := got.now(); !at.Equal(tt.wantAt) {
				t.Errorf("run's clock = %v, want the fixed %v", at, tt.wantAt)
			}
			if got.disclosure == nil || got.disclosure.Source != fixedClockSource {
				t.Errorf("run's clock disclosure = %+v, want source %s", got.disclosure, fixedClockSource)
			}
		})
	}
}
