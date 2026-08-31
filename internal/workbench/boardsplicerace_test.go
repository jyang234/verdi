package workbench

// Task 1B Phase 2 (Wave 6 design §6.1.2 item 6, SI-177): until Task 2
// deletes the legacy board splice writer, boardSpecServer.spliceSpec's
// complete read/parse/apply/validate/atomic-write must run INSIDE the
// checkout-wide writer transaction boundary (draftmutation.WithWriterLock)
// so it can never race a draft-mutation transaction on the same spec.md.
//
// Base RED (pre-routing): spliceSpec takes no checkout-wide exclusion, so
// a draftmutation mutate that reads/commits while the splice is paused
// between ITS read and write is silently clobbered — the mutate reports a
// clean result, writes its spec and provenance, and the resumed splice
// then overwrites spec.md with bytes computed from the pre-mutation
// document. One writer silently loses the other's update (the exact M-2
// lost-update shape, across the board/transaction boundary writeMu cannot
// see). GREEN (post-routing): the paused splice HOLDS the per-checkout
// transaction mutex, the concurrent mutate cannot enter its transaction
// until the splice completes, and the ordered legacy-splice-first,
// mutate-second run preserves both results with design provenance
// matching the final spec bytes.
//
// The mutate is driven directly at draftmutation.Service.Mutate with the
// production ports (real Git identity/state, real constitution policy
// authority) rather than through internal/mcpserve's in-process ServeConn:
// mcpserve imports this package (get_board via LoadProjection), so a
// package-internal test here cannot import it back. No outer lifetime
// lock is held, so both writers genuinely run (the mandate's
// without-an-outer-lifetime-lock shape).

import (
	"bytes"
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/artifact/splice"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/designprovenance"
	"github.com/jyang234/verdi/internal/draftmutation"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/store"
)

const spliceRaceName = "splice-race"

// spliceRaceSpec is a statusless proposed draft (merge-signaled
// acceptance) that both writers can edit: spliceSpec via
// SetObjectText("ac-1", ...) and the draft mutation via set-problem —
// distinct targets, so "both results preserved" is directly observable.
const spliceRaceSpec = `---
id: spec/splice-race
kind: spec
class: feature
title: "Splice race"
owners: [platform-team]
problem: { text: "old problem", anchor: "#problem" }
outcome: { text: "old outcome", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "first", evidence: [attestation], anchor: "#ac-1" }
---
# Splice race

## Problem

Old prose stays.

## Outcome

Old prose stays.

## ac-1

First.
`

// buildSpliceRaceFixture builds the one fixture both writers share: the
// policy-authority store (mode flipped to draft-write, exactly as
// cmd/verdi's designMutateStore does) landed on a provable default
// branch, and the statusless draft spec authored on design/splice-race —
// the Proposed/RelationNew shape AuthorizeState permits. Returns the
// symlink-resolved checkout root (macOS TempDir lives behind /var ->
// /private/var; canonical identity resolves through git, which reports
// the resolved spelling), its slash form for the request's expected
// identity, and the design branch HEAD.
func buildSpliceRaceFixture(t *testing.T) (root, rootSlash, head string) {
	t.Helper()
	mainFiles := map[string]string{
		".verdi/verdi.yaml": "schema: verdi.layout/v1\n",
		".verdi/.gitignore": "data/\n",
	}
	source := filepath.Join("..", "policyauthority", "testdata", "store")
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
			data = bytes.Replace(data, []byte("mode: proposal-only"), []byte("mode: draft-write"), 1)
		}
		mainFiles[filepath.ToSlash(rel)] = string(data)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	dir := buildAuthoringFixture(t, "design/"+spliceRaceName, mainFiles, map[string]string{
		".verdi/specs/active/" + spliceRaceName + "/spec.md": spliceRaceSpec,
	})
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	head, err = gitx.RevParse(context.Background(), resolved, "HEAD")
	if err != nil {
		t.Fatalf("resolving fixture HEAD: %v", err)
	}
	return resolved, filepath.ToSlash(resolved), head
}

// spliceRaceMutateOutcome carries one direct Service.Mutate drive's
// result back to the main test goroutine (no t.Fatalf off the main
// goroutine).
type spliceRaceMutateOutcome struct {
	response draftmutation.Response
	typed    *draftmutation.Error
	buildErr error
}

func (o spliceRaceMutateOutcome) clean() bool {
	return o.buildErr == nil && o.typed == nil && o.response.Result != nil
}

// runSpliceRaceMutate drives one draft mutation directly through the
// production service (real Git identity/state ports, real constitution
// policy source) against the fixture checkout, with base as the exact
// old-spec authority the request claims.
func runSpliceRaceMutate(root, rootSlash, head string, base []byte, text string, coordinator draftmutation.Coordinator) spliceRaceMutateOutcome {
	raw, err := canonjson.Marshal(map[string]any{
		"schema":        draftmutation.RequestSchema,
		"spec":          "spec/" + spliceRaceName,
		"base_digest":   draftmutation.DigestBytes(base),
		"base_spec_b64": base64.StdEncoding.EncodeToString(base),
		"expected":      map[string]any{"checkout": rootSlash, "branch": "design/" + spliceRaceName, "head": head},
		"operations":    []map[string]any{{"op": "set-problem", "text": text, "anchor": "#problem"}},
	})
	if err != nil {
		return spliceRaceMutateOutcome{buildErr: err}
	}
	request, err := draftmutation.DecodeRequest(raw)
	if err != nil {
		return spliceRaceMutateOutcome{buildErr: err}
	}
	actor, err := draftmutation.NewDelegatedAgent("codex", "session-1")
	if err != nil {
		return spliceRaceMutateOutcome{buildErr: err}
	}
	service := draftmutation.NewService()
	service.Coordinator = coordinator
	response, typed := service.Mutate(context.Background(), root, request, actor)
	return spliceRaceMutateOutcome{response: response, typed: typed}
}

// TestLegacyBoardSpliceSerializesWithWriterTransaction is Task 1B's
// deterministic legacy-splice race RED (the plan's third named RED test).
//
// Part 1 opens the race window: the legacy splice is paused between its
// read and its write (the spliceSpecTestPause test-only hook) while a
// draft mutation built from the same base bytes runs concurrently. At
// base the mutation completes cleanly inside the window and the resumed
// splice silently discards it — the RED. Routed, the paused splice holds
// the checkout's transaction boundary, so the mutation either lands and
// survives or is refused loudly (stale base) — never silently lost.
//
// Part 2 is the plan's ordered GREEN: the legacy splice runs first to
// completion, the mutate second from the post-splice base; both ordered
// results are preserved and the design provenance tail matches the final
// spec bytes exactly.
func TestLegacyBoardSpliceSerializesWithWriterTransaction(t *testing.T) {
	root, rootSlash, head := buildSpliceRaceFixture(t)
	server := &boardSpecServer{root: root}
	specPath := store.SpecPath(root, store.ZoneActive, spliceRaceName)

	base, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatal(err)
	}

	// --- Part 1: the deterministic race window. ---
	const (
		raceSpliceText = "first, spliced inside the race window"
		raceMutateText = "problem text mutated inside the splice window"
	)
	pauseEntered := make(chan struct{})
	pauseRelease := make(chan struct{})
	pause := func() {
		close(pauseEntered)
		<-pauseRelease
	}
	spliceSpecTestPause.Store(&pause)
	defer spliceSpecTestPause.Store(nil)

	spliceDone := make(chan error, 1)
	go func() {
		spliceDone <- server.spliceSpec(spliceRaceName, func(d *splice.Doc) ([]splice.Edit, error) {
			e, err := d.SetObjectText("ac-1", raceSpliceText)
			if err != nil {
				return nil, err
			}
			return []splice.Edit{e}, nil
		})
	}()
	<-pauseEntered // the splice has read the base spec and not yet written

	// Two Coordinator.After signals with distinct meanings:
	//   - mutatePreamble: a directory-parent-fsync step. ensureDirectory
	//     runs inside WithWriterLock BEFORE the per-checkout transaction
	//     mutex, so this fires in BOTH worlds — it proves the mutate has
	//     reached WithWriterLock and is at most instants from the mutex.
	//   - mutateEntered: StepJournalWrite, reached only INSIDE a
	//     committing transaction (the routed stale refusal never reaches
	//     Commit) — the exact "the transaction boundary admitted a second
	//     writer's commit" signal.
	var preambleOnce, enteredOnce sync.Once
	mutatePreamble := make(chan struct{})
	mutateEntered := make(chan struct{})
	coordinator := draftmutation.Coordinator{After: func(step string) error {
		if strings.HasPrefix(step, draftmutation.StepDirectoryParentSync) {
			preambleOnce.Do(func() { close(mutatePreamble) })
		}
		if step == draftmutation.StepJournalWrite {
			enteredOnce.Do(func() { close(mutateEntered) })
		}
		return nil
	}}
	mutateDone := make(chan spliceRaceMutateOutcome, 1)
	go func() {
		mutateDone <- runSpliceRaceMutate(root, rootSlash, head, base, raceMutateText, coordinator)
	}()

	// Base: the splice holds no exclusion, so the mutate enters its
	// transaction DURING the pause — wait for its full completion before
	// resuming the splice (fully deterministic clobber). Routed: the
	// paused splice holds the per-checkout transaction mutex, so the
	// mutate provably CANNOT enter while paused. "Blocked on the mutex"
	// is deliberately unobservable (the kernel carries no test hooks —
	// SI-177 authorized none), so the routed GREEN path must pay ONE
	// bounded wait; it is kept short by first waiting for the pre-mutex
	// preamble unconditionally (fires in both worlds), leaving the bound
	// to cover only the preamble→journal-write gap of a committing base
	// mutate — a liveness bound, never the routed verdict.
	mutateRanInsideWindow := false
	var raceOutcome spliceRaceMutateOutcome
	raceOutcomeReady := false
	select {
	case <-mutatePreamble:
		select {
		case <-mutateEntered:
			mutateRanInsideWindow = true
		case <-time.After(2 * time.Second):
		}
	case raceOutcome = <-mutateDone:
		// The mutate failed before ever reaching WithWriterLock (fixture
		// or identity trouble) — surfaced below as a non-stale failure.
		raceOutcomeReady = true
	}
	if mutateRanInsideWindow {
		raceOutcome = <-mutateDone
		close(pauseRelease)
	} else {
		close(pauseRelease)
		if !raceOutcomeReady {
			raceOutcome = <-mutateDone
		}
	}
	if err := <-spliceDone; err != nil {
		t.Fatalf("legacy splice inside the race window failed: %v", err)
	}
	spliceSpecTestPause.Store(nil)

	if raceOutcome.buildErr != nil {
		t.Fatalf("building/driving the concurrent draft mutation: %v", raceOutcome.buildErr)
	}
	final, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(final, []byte(raceSpliceText)) {
		t.Fatalf("legacy splice edit missing from the final spec:\n%s", final)
	}
	switch {
	case raceOutcome.clean():
		if !bytes.Contains(final, []byte(raceMutateText)) {
			t.Fatalf("SILENT LOST UPDATE: the draft mutation reported a clean result inside the legacy splice's read-modify-write window, but the resumed splice overwrote it — the final spec carries only the splice's edit.\nmutate ran inside window=%t\nfinal spec:\n%s",
				mutateRanInsideWindow, final)
		}
	case raceOutcome.response.Stale != nil,
		raceOutcome.typed != nil && raceOutcome.typed.Code == draftmutation.CodeStaleBase:
		// The serialized shape: the mutate waited out the splice and was
		// refused LOUDLY (stale base) — an explicit refusal the caller
		// sees, never a silent loss.
	default:
		t.Fatalf("concurrent draft mutation failed for a reason other than stale base: %v", raceOutcome.typed)
	}

	// --- Part 2: the plan's ordered GREEN — splice first, mutate second. ---
	const (
		orderedSpliceText = "first, spliced before the ordered mutate"
		orderedMutateText = "problem text mutated after the ordered splice"
	)
	if err := server.spliceSpec(spliceRaceName, func(d *splice.Doc) ([]splice.Edit, error) {
		e, err := d.SetObjectText("ac-1", orderedSpliceText)
		if err != nil {
			return nil, err
		}
		return []splice.Edit{e}, nil
	}); err != nil {
		t.Fatalf("ordered legacy splice failed: %v", err)
	}
	afterSplice, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatal(err)
	}
	ordered := runSpliceRaceMutate(root, rootSlash, head, afterSplice, orderedMutateText, draftmutation.Coordinator{})
	if ordered.buildErr != nil {
		t.Fatalf("building/driving the ordered draft mutation: %v", ordered.buildErr)
	}
	if !ordered.clean() {
		t.Fatalf("ordered draft mutation after the legacy splice = stale=%v typed=%v, want the canonical clean result", ordered.response.Stale, ordered.typed)
	}

	finalOrdered, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(finalOrdered, []byte(orderedSpliceText)) {
		t.Fatalf("ordered run lost the legacy splice's edit:\n%s", finalOrdered)
	}
	if !bytes.Contains(finalOrdered, []byte(orderedMutateText)) {
		t.Fatalf("ordered run lost the draft mutation's edit:\n%s", finalOrdered)
	}

	logBytes, err := os.ReadFile(store.DesignProvenancePath(root, store.ZoneActive, spliceRaceName))
	if err != nil {
		t.Fatalf("reading design provenance after the ordered run: %v", err)
	}
	entries, err := designprovenance.DecodeLog(logBytes)
	if err != nil || len(entries) == 0 {
		t.Fatalf("design provenance after the ordered run = %d entries, %v", len(entries), err)
	}
	tail := entries[len(entries)-1]
	if tail.Spec != "spec/"+spliceRaceName {
		t.Fatalf("provenance tail names %q, want spec/%s", tail.Spec, spliceRaceName)
	}
	if got, want := tail.ResultDigest, draftmutation.DigestBytes(finalOrdered); got != want {
		t.Fatalf("design provenance tail result digest = %s, want the final spec bytes' digest %s", got, want)
	}
}
