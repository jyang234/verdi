package main

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	forgepkg "github.com/jyang234/verdi/internal/forge"
	"github.com/jyang234/verdi/internal/forge/fake"
	"github.com/jyang234/verdi/internal/gotestjson"
)

// playwrightSyncStore is buildProduceDeps's store with the go-test
// obligations TestRunSync_Produce_GoTestEmitter uses (story-w ac-1 static
// TestA, ac-2 behavioral TestB, both for verify) and, when withPlaywright,
// three Playwright obligations for spec/story-w: ac-3 names a verify test in
// other.spec.ts, ac-4 a lint-job test in outcomes.spec.ts, and ac-5 a verify
// test in a file that does not exist. jobName is the CI job the fake forge
// reports.
func playwrightSyncStore(t *testing.T, jobName string, withPlaywright bool) (string, syncDeps, *fakeNamedGoTestRunner, *playwrightFakeRunner) {
	t.Helper()
	root, deps := buildProduceDeps(t)
	f := fake.New()
	f.SetCIContext(forgepkg.CIInfo{Pipeline: "913", Job: "7", JobName: jobName})
	deps.Forge = f
	writeGoMod(t, root)
	for _, o := range []struct{ ac, kind, test string }{{"ac-1", "static", "TestA"}, {"ac-2", "behavioral", "TestB"}} {
		writeTestProducerObligation(t, root, "story-w", o.ac, o.kind, "go-test:pkg/a:"+o.test, "verify")
	}
	goTest := &fakeNamedGoTestRunner{output: map[string][]byte{
		"./pkg/a": testGoTestJSON("pkg/a", map[string]string{"TestA": gotestjson.ActionPass, "TestB": gotestjson.ActionFail}),
	}}
	deps.NamedGoTest = goTest
	playwright := &playwrightFakeRunner{}
	if withPlaywright {
		playwrightSpecFiles(t, root, "other.spec.ts", "outcomes.spec.ts")
		writeTestProducerObligation(t, root, "story-w", "ac-3", "behavioral", "playwright:e2e/tests/other.spec.ts:outcomes › passes", "verify")
		writeTestProducerObligation(t, root, "story-w", "ac-4", "behavioral", "playwright:e2e/tests/outcomes.spec.ts:outcomes › fails", "lint")
		writeTestProducerObligation(t, root, "story-w", "ac-5", "behavioral", "playwright:e2e/tests/missing.spec.ts:outcomes › passes", "verify")
		playwright.report = playwrightMutatedReport(t, "outcomes", root, func(r map[string]any) {
			playwrightKeepFiles(t, r, map[string]string{"verify": "other.spec.ts", "lint": "outcomes.spec.ts"}[jobName])
		})
	}
	deps.Playwright = playwright
	return root, deps, goTest, playwright
}

// TestRunSync_Produce_PlaywrightBesideGoTest proves design §7 item 3: `sync
// --produce` runs the Playwright producer after the unchanged Go-test
// producer, selecting by the running job (GITHUB_JOB, CIInfo.JobName) and the
// playwright: prefix. The go-test runner's calls and records are exactly
// those of the same store without any Playwright obligation; only the running
// job's present Playwright files run, once; the Playwright record carries the
// run's provenance; an absent file is disclosed; outside a named CI job
// nothing runs at all; and a Playwright producer failure is the verb's
// operational exit 2.
func TestRunSync_Produce_PlaywrightBesideGoTest(t *testing.T) {
	cases := []struct {
		name          string
		job           string
		playwrightErr error
		wantExit      int
		wantGoCalls   int
		wantRun       []string // the files the Playwright runner ran; nil: never called
		wantRecord    map[string]artifact.EvidenceVerdict
	}{
		{name: "the verify job", job: "verify", wantGoCalls: 1, wantRun: []string{"other.spec.ts"},
			wantRecord: map[string]artifact.EvidenceVerdict{"ac-3": artifact.VerdictPass}},
		{name: "the lint job", job: "lint", wantRun: []string{"outcomes.spec.ts"},
			wantRecord: map[string]artifact.EvidenceVerdict{"ac-4": artifact.VerdictFail}},
		{name: "no detected CI job", job: ""},
		{name: "the Playwright producer fails", job: "verify", playwrightErr: errors.New("make e2e-setup: exit status 2"), wantExit: 2, wantGoCalls: 1, wantRun: []string{"other.spec.ts"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("CI", "true")
			root, deps, goTest, playwright := playwrightSyncStore(t, c.job, true)
			playwright.err = c.playwrightErr
			code := runSync(context.Background(), root, testRef, testCommit, false, true, false, deps)
			stderr := deps.Stderr.(*bytes.Buffer).String()
			if code != c.wantExit {
				t.Fatalf("runSync(--produce) = %d, want %d; stderr=%s", code, c.wantExit, stderr)
			}
			if len(goTest.calls) != c.wantGoCalls {
				t.Errorf("go-test runner calls = %+v, want %d", goTest.calls, c.wantGoCalls)
			}
			var ran []string
			for _, call := range playwright.calls {
				if call.root != root {
					t.Errorf("Playwright ran at %s, want the store root %s", call.root, root)
				}
				ran = append(ran, call.specs...)
			}
			if len(playwright.calls) > 1 || !reflect.DeepEqual(ran, c.wantRun) {
				t.Fatalf("Playwright runner calls = %+v, want one run of %q", playwright.calls, c.wantRun)
			}
			if c.wantExit != 0 {
				if !strings.Contains(stderr, "make e2e-setup: exit status 2") {
					t.Errorf("stderr = %q, want the producer's error", stderr)
				}
				return
			}

			// The same store without Playwright obligations: the go-test
			// production must be identical.
			baseRoot, baseDeps, baseGoTest, basePlaywright := playwrightSyncStore(t, c.job, false)
			if code := runSync(context.Background(), baseRoot, testRef, testCommit, false, true, false, baseDeps); code != 0 {
				t.Fatalf("baseline runSync = %d; stderr=%s", code, baseDeps.Stderr.(*bytes.Buffer).String())
			}
			if !reflect.DeepEqual(goTest.calls, baseGoTest.calls) {
				t.Errorf("go-test runner calls = %+v, want the baseline's %+v", goTest.calls, baseGoTest.calls)
			}
			if len(basePlaywright.calls) != 0 {
				t.Errorf("baseline Playwright calls = %+v, want none", basePlaywright.calls)
			}
			goTestRecords := func(recs []artifact.Evidence) []artifact.Evidence {
				var out []artifact.Evidence
				for _, r := range recs {
					if strings.HasPrefix(r.Producer, "go-test:") {
						out = append(out, r)
					}
				}
				return out
			}
			got := readVerdicts(t, root, "spec/story-w", testCommit)
			base := readVerdicts(t, baseRoot, "spec/story-w", testCommit)
			if !reflect.DeepEqual(goTestRecords(got), goTestRecords(base)) {
				t.Errorf("go-test records = %+v, want the baseline's %+v", goTestRecords(got), goTestRecords(base))
			}

			wantProv := artifact.EvidenceProvenance{Source: artifact.SourceCI, Pipeline: "913", Job: "7", JobName: c.job, Commit: testCommit}
			playwrightRecords := map[string]artifact.Evidence{}
			for _, r := range got {
				if strings.HasPrefix(r.Producer, "playwright:") {
					playwrightRecords[r.EvidenceFor[0]] = r
				}
			}
			if len(playwrightRecords) != len(c.wantRecord) {
				t.Errorf("Playwright records = %+v, want %v", playwrightRecords, c.wantRecord)
			}
			for ac, verdict := range c.wantRecord {
				r, ok := playwrightRecords[ac]
				if !ok || r.Verdict != verdict || r.Provenance != wantProv {
					t.Errorf("%s: record %+v, want verdict %s provenance %+v", ac, r, verdict, wantProv)
				}
			}
			stdout := deps.Stdout.(*bytes.Buffer).String()
			absent := playwrightDisclosureFor(stdout, "obligation/story-w--ac-5--behavioral")
			if c.job == "verify" && !strings.Contains(absent, "does not exist") {
				t.Errorf("ac-5 disclosure = %q, want the absent file disclosed", absent)
			}
		})
	}
}
