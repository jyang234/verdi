package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseCheckerBuiltCommandBoundaries(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "release-checker")
	build := exec.CommandContext(context.Background(), "go", "build", "-trimpath", "-o", binary, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build actual auxiliary command: %v: %s", err, out)
	}
	for _, tc := range []struct {
		name    string
		args    []string
		exit    int
		message string
	}{
		{"help", []string{"--help"}, 0, "Usage of"},
		{"unknown flag", []string{"--trust-this-success-report=pass.json"}, 2, "flag provided but not defined"},
		{"missing source inputs", nil, 2, "all release paths must be absolute"},
		{"positional success report", []string{"supplied-success.json"}, 2, "unexpected positional arguments"},
		{"incomplete source inputs", []string{"--verdi-source", t.TempDir()}, 2, "all release paths must be absolute"},
		{"missing protected bootstrap inputs", []string{"--bootstrap", filepath.Join(t.TempDir(), "isolated"), "--verdi-source", t.TempDir()}, 2, "missing or invalid approved source identities"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.CommandContext(context.Background(), binary, tc.args...)
			cmd.Env = withoutReleaseInputs(os.Environ())
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			err := cmd.Run()
			exit := 0
			if err != nil {
				var ee *exec.ExitError
				if !errors.As(err, &ee) {
					t.Fatal(err)
				}
				exit = ee.ExitCode()
			}
			if exit != tc.exit || stdout.Len() != 0 || !strings.Contains(stderr.String(), tc.message) {
				t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
			}
		})
	}
}
func withoutReleaseInputs(env []string) []string {
	out := make([]string, 0, len(env))
	for _, e := range env {
		if !strings.HasPrefix(e, "PUBLIC_RELEASE_") {
			out = append(out, e)
		}
	}
	return out
}
