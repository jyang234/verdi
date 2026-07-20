package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	forgepkg "github.com/jyang234/verdi/internal/forge"
	"github.com/jyang234/verdi/internal/forge/fake"
	"github.com/jyang234/verdi/internal/upstream"
)

// quarantineTestRecord renders one hand-built verdi.evidence/v1 record
// array (matching internal/evidence/records_test.go's own recordJSON
// convention) whose sole record's provenance.commit is commit — the
// exactly-controlled shape spec/evidence-resilience ac-1's fixturegit
// cases need.
func quarantineTestRecord(commit string) string {
	return `[{"schema":"verdi.evidence/v1","evidence_for":["ac-1"],"kind":"static","verdict":"pass",` +
		`"witness":"w","provenance":{"source":"ci","pipeline":"1","commit":"` + commit + `"},` +
		`"digest":"sha256:` + strings.Repeat("ab", 32) + `"}]`
}

// readMaterializedRecords reads and strict-decodes a materialized
// verdicts.json (or runtime.json) at root/.verdi/data/derived/<key>/<name>.
func readMaterializedRecords(t *testing.T, root, specKey, commit, name string) []artifact.Evidence {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".verdi", "data", "derived", specKey, commit, name))
	if err != nil {
		t.Fatalf("reading materialized %s: %v", name, err)
	}
	var records []artifact.Evidence
	if err := artifact.DecodeStrictJSON(data, &records); err != nil {
		t.Fatalf("decoding materialized %s: %v", name, err)
	}
	return records
}

// TestRunSync_CIFetch_QuarantinesUnreachableCommitRecord is ac-1's core
// proof: a fetched CI bundle carrying a record whose provenance.commit is
// not reachable from HEAD (the X-15 shape — the branch that produced it
// has since been deleted) is quarantined, not dropped and not an
// operational failure. The record is kept on disk, annotated with the
// quarantine reason, and sync itself exits 0.
func TestRunSync_CIFetch_QuarantinesUnreachableCommitRecord(t *testing.T) {
	root := buildTestStore(t)
	head := gitInitTestStore(t, root)
	const unreachable = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"

	f := fake.New()
	f.SeedBundle(testRef, head, forgepkg.DerivedTree{
		"spec--x/" + head + "/verdicts.json": []byte(quarantineTestRecord(unreachable)),
	})

	var stdout, stderr bytes.Buffer
	deps := syncDeps{Runner: upstream.NewFakeRunner(), Forge: f, GoTest: fakeGoTest{}, Stdout: &stdout, Stderr: &stderr}
	code := runSync(context.Background(), root, testRef, head, false, false, false, deps)
	if code != 0 {
		t.Fatalf("runSync exit = %d, want 0 (a quarantined record is never, by itself, an operational failure); stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "quarantine") {
		t.Errorf("stdout = %q, want a disclosure naming the quarantine", stdout.String())
	}

	records := readMaterializedRecords(t, root, "spec--x", head, "verdicts.json")
	if len(records) != 1 {
		t.Fatalf("materialized verdicts.json has %d records, want 1 (kept, never dropped)", len(records))
	}
	if records[0].Quarantine == nil {
		t.Fatal("records[0].Quarantine = nil, want a quarantine annotation")
	}
	if !strings.Contains(records[0].Quarantine.Reason, unreachable) {
		t.Errorf("records[0].Quarantine.Reason = %q, want the unreachable commit %q named", records[0].Quarantine.Reason, unreachable)
	}
}

// TestRunSync_CIFetch_ReachableRecord_NotQuarantined regression-pins the
// ordinary case: a record whose provenance.commit legitimately IS
// reachable (here, HEAD itself) is left entirely unquarantined, and sync
// prints no quarantine disclosure at all.
func TestRunSync_CIFetch_ReachableRecord_NotQuarantined(t *testing.T) {
	root := buildTestStore(t)
	head := gitInitTestStore(t, root)

	f := fake.New()
	f.SeedBundle(testRef, head, forgepkg.DerivedTree{
		"spec--x/" + head + "/verdicts.json": []byte(quarantineTestRecord(head)),
	})

	var stdout, stderr bytes.Buffer
	deps := syncDeps{Runner: upstream.NewFakeRunner(), Forge: f, GoTest: fakeGoTest{}, Stdout: &stdout, Stderr: &stderr}
	code := runSync(context.Background(), root, testRef, head, false, false, false, deps)
	if code != 0 {
		t.Fatalf("runSync exit = %d, want 0; stderr=%s", code, stderr.String())
	}

	records := readMaterializedRecords(t, root, "spec--x", head, "verdicts.json")
	if len(records) != 1 {
		t.Fatalf("materialized verdicts.json has %d records, want 1", len(records))
	}
	if records[0].Quarantine != nil {
		t.Errorf("records[0].Quarantine = %+v, want nil (provenance.commit is HEAD itself, trivially reachable)", records[0].Quarantine)
	}
	if strings.Contains(stdout.String(), "quarantine") {
		t.Errorf("stdout = %q, must not mention quarantine when nothing was quarantined", stdout.String())
	}
}

// TestRunSync_CIFetch_QuarantineAppliesToRuntimeJSON proves the write-time
// quarantine pass scans BOTH record-bearing files the fold reads
// (internal/evidence.RecordFileNames), not just verdicts.json — a
// runtime.json record referencing an unreachable commit is quarantined
// exactly the same way.
func TestRunSync_CIFetch_QuarantineAppliesToRuntimeJSON(t *testing.T) {
	root := buildTestStore(t)
	head := gitInitTestStore(t, root)
	const unreachable = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"

	f := fake.New()
	f.SeedBundle(testRef, head, forgepkg.DerivedTree{
		"spec--x/" + head + "/runtime.json": []byte(quarantineTestRecord(unreachable)),
	})

	var stdout, stderr bytes.Buffer
	deps := syncDeps{Runner: upstream.NewFakeRunner(), Forge: f, GoTest: fakeGoTest{}, Stdout: &stdout, Stderr: &stderr}
	code := runSync(context.Background(), root, testRef, head, false, false, false, deps)
	if code != 0 {
		t.Fatalf("runSync exit = %d, want 0; stderr=%s", code, stderr.String())
	}

	records := readMaterializedRecords(t, root, "spec--x", head, "runtime.json")
	if len(records) != 1 || records[0].Quarantine == nil {
		t.Fatalf("runtime.json records = %+v, want exactly 1 quarantined record", records)
	}
}

// TestRunSync_CIFetch_NonRecordFilesUntouchedByQuarantine proves the
// quarantine pass never rewrites a file it has no business touching:
// review.json (not an evidence-record file at all) is written byte-for-
// byte identical to what was fetched, even in the same commit directory
// as a verdicts.json that DID get quarantined.
func TestRunSync_CIFetch_NonRecordFilesUntouchedByQuarantine(t *testing.T) {
	root := buildTestStore(t)
	head := gitInitTestStore(t, root)
	const unreachable = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	// An empty review array — evaluateTree decodes this file looking for
	// BLOCK verdicts, so it must stay real upstream.Review-array JSON; its
	// exact bytes are what this test pins as untouched, not its content.
	const reviewBytes = "[]\n"

	f := fake.New()
	f.SeedBundle(testRef, head, forgepkg.DerivedTree{
		"spec--x/" + head + "/verdicts.json": []byte(quarantineTestRecord(unreachable)),
		"spec--x/" + head + "/review.json":   []byte(reviewBytes),
	})

	var stdout, stderr bytes.Buffer
	deps := syncDeps{Runner: upstream.NewFakeRunner(), Forge: f, GoTest: fakeGoTest{}, Stdout: &stdout, Stderr: &stderr}
	code := runSync(context.Background(), root, testRef, head, false, false, false, deps)
	if code != 0 {
		t.Fatalf("runSync exit = %d, want 0; stderr=%s", code, stderr.String())
	}

	got, err := os.ReadFile(filepath.Join(root, ".verdi", "data", "derived", "spec--x", head, "review.json"))
	if err != nil {
		t.Fatalf("reading materialized review.json: %v", err)
	}
	if string(got) != "[]\n" {
		t.Errorf("materialized review.json = %q, want the fetched bytes untouched", got)
	}
}

// TestRunSync_CIFetch_UndecodableFetchedFile_NotOperational is
// spec/evidence-resilience finding-3's regression pin: a fetched record file
// (runtime.json here) that FAILS strict decode must NOT make sync exit 2
// operationally. Before the fix, quarantineUnreachable strict-decoded every
// fetched verdicts.json/runtime.json for its reachability check and surfaced
// a decode failure as an operational error — a NEW sync-time hard-fail on
// inputs unrelated to commit reachability, on the exact fetch path ac-1
// hardens (previously the fetch path wrote runtime.json without decoding it).
// The undecodable file is quarantined-by-default (kept verbatim on disk,
// never dropped), sync exits 0, and stdout notes it.
func TestRunSync_CIFetch_UndecodableFetchedFile_NotOperational(t *testing.T) {
	root := buildTestStore(t)
	head := gitInitTestStore(t, root)
	const unreachable = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	// Truncated JSON under an UNREACHABLE dir key — the realistic
	// stale-poisoned-bundle debris shape (its source branch since deleted).
	const malformed = `[{"schema":"verdi.evidence/v1"`

	f := fake.New()
	f.SeedBundle(testRef, head, forgepkg.DerivedTree{
		"spec--x/" + unreachable + "/runtime.json": []byte(malformed),
	})

	var stdout, stderr bytes.Buffer
	deps := syncDeps{Runner: upstream.NewFakeRunner(), Forge: f, GoTest: fakeGoTest{}, Stdout: &stdout, Stderr: &stderr}
	code := runSync(context.Background(), root, testRef, head, false, false, false, deps)
	if code != 0 {
		t.Fatalf("runSync exit = %d, want 0 (an undecodable fetched record file is quarantined-by-default, never an operational failure — finding 3); stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "undecodable") {
		t.Errorf("stdout = %q, want sync to note the undecodable quarantined file", stdout.String())
	}
	got, err := os.ReadFile(filepath.Join(root, ".verdi", "data", "derived", "spec--x", unreachable, "runtime.json"))
	if err != nil {
		t.Fatalf("reading materialized runtime.json: %v", err)
	}
	if string(got) != malformed {
		t.Errorf("materialized runtime.json = %q, want the fetched bytes kept verbatim (never dropped)", got)
	}
}

// TestRunSync_CIFetch_QuarantinedFailRecord_ExcludedFromVerdict is
// spec/evidence-resilience finding-4's regression pin: a record whose
// provenance.commit is unreachable (so sync quarantines it) that ALSO carries
// verdict:fail must NOT drive sync's exit code — a record the system has just
// declared non-authoritative-and-excluded cannot control sync's verdict, or a
// poisoned bundle's stale fail record keeps sync red on every re-sync (X-15's
// "re-syncing did not help" shape at exit-1 severity). sync exits 0 (not 1),
// disclosing that the quarantined record was excluded from the verdict.
func TestRunSync_CIFetch_QuarantinedFailRecord_ExcludedFromVerdict(t *testing.T) {
	root := buildTestStore(t)
	head := gitInitTestStore(t, root)
	const unreachable = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"

	failRecord := `[{"schema":"verdi.evidence/v1","evidence_for":["ac-1"],"kind":"static","verdict":"fail",` +
		`"witness":"w","provenance":{"source":"ci","pipeline":"1","commit":"` + unreachable + `"},` +
		`"digest":"sha256:` + strings.Repeat("ab", 32) + `"}]`

	f := fake.New()
	f.SeedBundle(testRef, head, forgepkg.DerivedTree{
		"spec--x/" + head + "/verdicts.json": []byte(failRecord),
	})

	var stdout, stderr bytes.Buffer
	deps := syncDeps{Runner: upstream.NewFakeRunner(), Forge: f, GoTest: fakeGoTest{}, Stdout: &stdout, Stderr: &stderr}
	code := runSync(context.Background(), root, testRef, head, false, false, false, deps)
	if code != 0 {
		t.Fatalf("runSync exit = %d, want 0 (a quarantined fail record must not drive sync's verdict exit — finding 4); stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "excluded from verdict") {
		t.Errorf("stdout = %q, want the disclosure that quarantined record(s) were excluded from the verdict", stdout.String())
	}
}
