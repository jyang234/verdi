// serve_clock_test.go: `verdi serve`'s VERDI_NOW seam (SI-296,
// spec/index-data ac-2/ac-3) — the pure parser's table, then the shipped
// binary's three behavioral paths: set (the served index's ages and quiet
// marks are decided against the fixed instant, and the pages disclose it),
// unset (the wall clock, nothing disclosed), and malformed (exit 2 naming
// the variable, before any server effect).
package main

import (
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

	"github.com/jyang234/verdi/internal/fixturegit"
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
			clock, disclosed, err := workbenchClock(tt.raw)
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
