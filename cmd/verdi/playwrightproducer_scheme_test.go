package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/disclosure"
)

// TestGoTestProducerSkipsPlaywrightScheme proves SI-307 (8): each per-test
// producer handles only its own scheme. The go-test producer skips an
// obligation whose producer ref starts with "playwright:" before its
// runtime-kind check and before its grammar check, so it neither selects nor
// discloses a Playwright obligation of any kind, well-formed or not; every
// other ref that is not go-test:, including a differently cased
// "Playwright:", keeps the go-test producer's own disclosure.
func TestGoTestProducerSkipsPlaywrightScheme(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		kind       string
		ref        string
		wantSource string // "": the go-test producer discloses nothing
	}{
		{name: "a Playwright ref", kind: "behavioral", ref: "playwright:e2e/tests/a.spec.ts:suite › case: one"},
		{name: "a runtime-kind Playwright ref", kind: "runtime", ref: "playwright:e2e/tests/a.spec.ts:suite › case: one"},
		{name: "a malformed Playwright ref", kind: "behavioral", ref: "playwright:e2e/tests/a.spec.ts"},
		{name: "another scheme", kind: "behavioral", ref: "junit:suite:case", wantSource: goTestProducerMalformedRefSource},
		{name: "the Playwright scheme in another case", kind: "behavioral", ref: "Playwright:e2e/tests/a.spec.ts:t", wantSource: goTestProducerMalformedRefSource},
		{name: "a runtime-kind ref of another scheme", kind: "runtime", ref: "junit:suite:case", wantSource: goTestProducerRuntimeKindSource},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writeGoMod(t, root)
			writeTestProducerObligation(t, root, "story-a", "ac-1", c.kind, c.ref, "verify")

			candidates, _, err := discoverTestProducerObligations(root)
			if err != nil {
				t.Fatal(err)
			}
			if len(candidates) != 1 {
				t.Fatalf("candidates = %+v, want the one obligation", candidates)
			}
			selected, discl := selectGoTestObligations(root, candidates, "verify")
			if len(selected) != 0 {
				t.Errorf("the go-test producer selected %+v, want nothing", selected)
			}
			var stdout bytes.Buffer
			if err := produceGoTestEvidence(context.Background(), root, "c0ffee", "verify", nil, artifact.EvidenceProvenance{Source: artifact.SourceCI, Commit: "c0ffee"}, &stdout); err != nil {
				t.Fatalf("produceGoTestEvidence: %v", err)
			}
			if c.wantSource == "" {
				if len(discl) != 0 || stdout.Len() != 0 {
					t.Fatalf("the go-test producer disclosed %+v and printed %q for a Playwright obligation, want nothing", discl, stdout.String())
				}
				return
			}
			if len(discl) != 1 || discl[0].Source != c.wantSource {
				t.Fatalf("disclosures = %+v, want one from %s", discl, c.wantSource)
			}
			if line := strings.TrimSuffix(stdout.String(), "\n"); line != disclosure.Render(discl[0]) {
				t.Errorf("printed %q, want exactly %q", line, disclosure.Render(discl[0]))
			}
		})
	}
}

// TestRunSync_Produce_PlaywrightObligationsDisclosedOnce proves SI-307 (8)
// through `sync --produce` itself: over a store holding go-test and Playwright
// obligations, no go-test producer disclosure names a Playwright obligation,
// and the verify job's absent-file Playwright obligation is disclosed exactly
// once, by the Playwright producer.
func TestRunSync_Produce_PlaywrightObligationsDisclosedOnce(t *testing.T) {
	t.Setenv("CI", "true")
	root, deps, _, _ := playwrightSyncStore(t, "verify", true)
	if code := runSync(context.Background(), root, testRef, testCommit, false, true, false, deps); code != 0 {
		t.Fatalf("runSync(--produce) = %d; stderr=%s", code, deps.Stderr.(*bytes.Buffer).String())
	}
	stdout := deps.Stdout.(*bytes.Buffer).String()
	for _, ac := range []string{"ac-3", "ac-4", "ac-5"} {
		id := "obligation/story-w--" + ac + "--behavioral"
		var goTestLines, allLines []string
		for _, line := range strings.Split(stdout, "\n") {
			if !disclosure.IsRendered(line) || !strings.Contains(line, "] "+id+": ") {
				continue
			}
			allLines = append(allLines, line)
			if strings.Contains(line, "[sync:go-test-producer-") {
				goTestLines = append(goTestLines, line)
			}
		}
		if len(goTestLines) != 0 {
			t.Errorf("%s: the go-test producer disclosed a Playwright obligation: %q", ac, goTestLines)
		}
		want := 0
		if ac == "ac-5" {
			want = 1 // the verify job's absent file, disclosed by the Playwright producer
		}
		if len(allLines) != want {
			t.Errorf("%s: %d disclosures %q, want %d", ac, len(allLines), allLines, want)
		}
	}
}
