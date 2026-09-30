package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// playwrightFakeTools puts a fake `make` and a fake `npx` alone on PATH for
// the rest of t: the real runner's exec seam. Each records, one per line, its
// working directory, then every argument, then the Playwright-relevant
// environment ("<unset>" when a variable is absent), into a file named after
// itself in the returned directory, and exits with its given status. The fake
// npx writes a one-line report to $PLAYWRIGHT_JSON_OUTPUT_FILE first. An
// empty script body omits that tool. No Node is needed.
func playwrightFakeTools(t *testing.T, makeExit, npxExit string) string {
	t.Helper()
	bin := t.TempDir()
	logs := t.TempDir()
	record := func(tool string) string {
		return "#!/bin/sh\n{ pwd; for a in \"$@\"; do printf '%s\\n' \"$a\"; done; " +
			"printf 'VERDI_E2E_SPECS=%s\\n' \"${VERDI_E2E_SPECS-<unset>}\"; " +
			"printf 'PLAYWRIGHT_JSON_OUTPUT_FILE=%s\\n' \"${PLAYWRIGHT_JSON_OUTPUT_FILE-<unset>}\"; " +
			"printf 'V1_ACCEPTANCE=%s\\n' \"${V1_ACCEPTANCE-<unset>}\"; } > '" + filepath.Join(logs, tool) + "'\n"
	}
	tools := map[string]string{}
	if makeExit != "" {
		tools["make"] = record("make") + "echo installing\nexit " + makeExit + "\n"
	}
	if npxExit != "" {
		tools["npx"] = record("npx") + "printf '{}' > \"$PLAYWRIGHT_JSON_OUTPUT_FILE\"\necho running\nexit " + npxExit + "\n"
	}
	for name, script := range tools {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	return logs
}

// playwrightToolLog reads what a fake tool recorded, or nil if it never ran.
func playwrightToolLog(t *testing.T, logs, tool string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(logs, tool))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	// The recorded working directory, with symlinks resolved (a temporary
	// directory may sit behind one), so it compares with EvalSymlinks(want).
	if real, err := filepath.EvalSymlinks(lines[0]); err == nil {
		lines[0] = real
	}
	return lines
}

// TestRealPlaywrightRunner_Invocation pins the real runner's two commands
// through the fake-tool seam: `make e2e-setup` at the store root, then, in
// e2e/, exactly `npx playwright test --workers=1 --retries=0 --reporter=json
// --trace=off --output=<beside the report>` with no positional argument —
// Playwright's positional filters are regular-expression substring matches
// that could select extra files — and the files named only through
// VERDI_E2E_SPECS, the harness's exact, fail-closed selector (SI-268). The
// report path reaches Playwright through PLAYWRIGHT_JSON_OUTPUT_FILE, an
// inherited V1_ACCEPTANCE (which adds the v1-acceptance project) is removed,
// an inherited VERDI_E2E_SPECS is replaced, and the tools' output goes to the
// runner's log. A failing test's nonzero Playwright exit is not an error.
func TestRealPlaywrightRunner_Invocation(t *testing.T) {
	logs := playwrightFakeTools(t, "0", "1")
	t.Setenv("V1_ACCEPTANCE", "1")
	t.Setenv("VERDI_E2E_SPECS", "stale.spec.ts")
	t.Setenv("PLAYWRIGHT_JSON_OUTPUT_FILE", "/stale/report.json")
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "e2e"), 0o755); err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	reportPath := filepath.Join(work, "report.json")
	var log bytes.Buffer

	if err := (realPlaywrightRunner{Log: &log}).RunPlaywright(context.Background(), root, []string{"a.spec.ts", "b.spec.ts"}, reportPath); err != nil {
		t.Fatalf("RunPlaywright: %v", err)
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	wantMake := []string{realRoot, "e2e-setup", "VERDI_E2E_SPECS=stale.spec.ts", "PLAYWRIGHT_JSON_OUTPUT_FILE=/stale/report.json", "V1_ACCEPTANCE=1"}
	if got := playwrightToolLog(t, logs, "make"); !reflect.DeepEqual(got, wantMake) {
		t.Errorf("make recorded %q, want %q", got, wantMake)
	}
	wantNpx := []string{
		filepath.Join(realRoot, "e2e"),
		"playwright", "test", "--workers=1", "--retries=0", "--reporter=json", "--trace=off", "--output=" + filepath.Join(work, "test-results"),
		"VERDI_E2E_SPECS=a.spec.ts b.spec.ts", "PLAYWRIGHT_JSON_OUTPUT_FILE=" + reportPath, "V1_ACCEPTANCE=<unset>",
	}
	if got := playwrightToolLog(t, logs, "npx"); !reflect.DeepEqual(got, wantNpx) {
		t.Errorf("npx recorded %q, want %q", got, wantNpx)
	}
	if data, err := os.ReadFile(reportPath); err != nil || string(data) != "{}" {
		t.Errorf("report = %q, %v; want the fake's report at the path given", data, err)
	}
	if got := log.String(); got != "installing\nrunning\n" {
		t.Errorf("log = %q, want both tools' output", got)
	}
}

// TestRealPlaywrightRunner_Errors proves the runner's error surface: a failed
// install, a missing make or npx, and a context cancelled before the run are
// errors naming why, and a failed install never runs Playwright.
func TestRealPlaywrightRunner_Errors(t *testing.T) {
	cancelled := func() context.Context {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		return ctx
	}
	cases := []struct {
		name     string
		makeExit string
		npxExit  string
		ctx      func() context.Context
		wantErr  string
		wantIs   error
	}{
		{name: "the install fails", makeExit: "2", npxExit: "0", ctx: context.Background, wantErr: "make e2e-setup"},
		{name: "no make", npxExit: "0", ctx: context.Background, wantIs: exec.ErrNotFound},
		{name: "no npx", makeExit: "0", ctx: context.Background, wantIs: exec.ErrNotFound},
		{name: "cancelled before the run", makeExit: "0", npxExit: "0", ctx: cancelled, wantIs: context.Canceled},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			logs := playwrightFakeTools(t, c.makeExit, c.npxExit)
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, "e2e"), 0o755); err != nil {
				t.Fatal(err)
			}
			start := time.Now()
			err := (realPlaywrightRunner{}).RunPlaywright(c.ctx(), root, []string{"a.spec.ts"}, filepath.Join(t.TempDir(), "report.json"))
			if time.Since(start) > 20*time.Second {
				t.Errorf("RunPlaywright took %v", time.Since(start))
			}
			if err == nil {
				t.Fatal("RunPlaywright = nil, want an error")
			}
			if c.wantErr != "" && !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("err = %v, want it to contain %q", err, c.wantErr)
			}
			if c.wantIs != nil && !errors.Is(err, c.wantIs) {
				t.Errorf("err = %v, want one wrapping %v", err, c.wantIs)
			}
			if got := playwrightToolLog(t, logs, "npx"); got != nil && c.makeExit != "0" {
				t.Errorf("npx ran after a failed install: %q", got)
			}
		})
	}
}
