package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/boardio"
	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/designprovenance"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/policyconflict"
	"github.com/jyang234/verdi/internal/readinesspilot"
	"github.com/jyang234/verdi/internal/store"
)

func TestReadinessSnapshotFeatureAndStoryTargets(t *testing.T) {
	for _, tc := range []struct {
		class string
		title string
	}{
		{class: "feature", title: "Feature Alpha"},
		{class: "story", title: "Borrower appeal: exact source title"},
	} {
		t.Run(tc.class, func(t *testing.T) {
			repo, requestPath, _, targetRef, _ := readinessSnapshotRepo(t, tc.class)
			builder := localReadinessSnapshotBuilder{providerFactory: readinessSnapshotProviderFactory(t, repo.Dir, policyconflict.VerdictPass, nil)}
			snapshot, err := builder.Build(context.Background(), repo.Dir, requestPath)
			if err != nil {
				t.Fatalf("Build: %v", err)
			}
			if snapshot.TargetRef != targetRef || snapshot.TargetTitle != tc.title || snapshot.TargetClass != tc.class || snapshot.Branch != "design/"+strings.TrimPrefix(targetRef, "spec/") || snapshot.Head != repo.Head {
				t.Fatalf("snapshot identity = %+v, want %s %q %s at current design branch/%s", snapshot, targetRef, tc.title, tc.class, repo.Head)
			}
			if err := snapshot.Validate(); err != nil {
				t.Fatalf("snapshot Validate: %v", err)
			}
		})
	}
}

func TestReadinessSnapshotProvenanceMutationAndBoard(t *testing.T) {
	t.Run("missing provenance is explicitly unproven", func(t *testing.T) {
		repo, requestPath, _, _, _ := readinessSnapshotRepo(t, "feature")
		snapshot := buildReadinessSnapshot(t, repo, requestPath, policyconflict.VerdictPass, localReadinessSnapshotBuilder{})
		concern := readinessSnapshotConcern(t, snapshot, "shape/provenance")
		if concern.State != readinesspilot.StateUnproven {
			t.Fatalf("provenance state = %q, want unproven", concern.State)
		}
		assertReadinessStringsContain(t, concern.Witnesses, "sidecar is absent")
		assertReadinessBoardDestination(t, concern, "/b/design%2Ffeature-alpha/board/spec/feature-alpha")
	})

	t.Run("malformed provenance is operational", func(t *testing.T) {
		repo, requestPath, _, _, _ := readinessSnapshotRepo(t, "feature")
		path := store.DesignProvenancePath(repo.Dir, store.ZoneActive, "feature-alpha")
		if err := os.WriteFile(path, []byte("not-jsonl\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		builder := localReadinessSnapshotBuilder{providerFactory: readinessSnapshotProviderFactory(t, repo.Dir, policyconflict.VerdictPass, nil)}
		_, err := builder.Build(context.Background(), repo.Dir, requestPath)
		if err == nil || !strings.Contains(err.Error(), "decoding design provenance") {
			t.Fatalf("Build error = %v, want malformed provenance operational error", err)
		}
	})

	t.Run("direct Markdown gap stays unproven", func(t *testing.T) {
		repo, requestPath, _, _, _ := readinessSnapshotRepo(t, "feature")
		entry := readinessProvenanceEntry(t, "spec/feature-alpha")
		encoded, err := designprovenance.EncodeEntry(entry)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(store.DesignProvenancePath(repo.Dir, store.ZoneActive, "feature-alpha"), encoded, 0o644); err != nil {
			t.Fatal(err)
		}
		snapshot := buildReadinessSnapshot(t, repo, requestPath, policyconflict.VerdictPass, localReadinessSnapshotBuilder{})
		concern := readinessSnapshotConcern(t, snapshot, "shape/provenance")
		if concern.State != readinesspilot.StateUnproven {
			t.Fatalf("provenance state = %q, want unproven", concern.State)
		}
		assertReadinessStringsContain(t, concern.Witnesses, "direct Markdown")
		assertReadinessBoardDestination(t, concern, "/b/design%2Ffeature-alpha/board/spec/feature-alpha")
	})

	t.Run("mutation residue is disclosed without recovery", func(t *testing.T) {
		repo, requestPath, _, _, _ := readinessSnapshotRepo(t, "feature")
		journal := store.DraftMutationJournalPath(repo.Dir, "feature-alpha")
		stage := store.DraftMutationSpecStagePath(repo.Dir, "feature-alpha")
		if err := os.MkdirAll(filepath.Dir(journal), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(journal, []byte("leave-unread"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(stage, []byte("leave-unread"), 0o644); err != nil {
			t.Fatal(err)
		}
		snapshot := buildReadinessSnapshot(t, repo, requestPath, policyconflict.VerdictPass, localReadinessSnapshotBuilder{})
		concern := readinessSnapshotConcern(t, snapshot, "shape/mutation")
		if concern.State != readinesspilot.StateUnproven {
			t.Fatalf("mutation state = %q, want unproven", concern.State)
		}
		assertReadinessStringsContain(t, concern.Witnesses, "journal")
		assertReadinessStringsContain(t, concern.Witnesses, "spec stage")
		assertReadinessBoardDestination(t, concern, "/b/design%2Ffeature-alpha/board/spec/feature-alpha")
		journalBytes, _ := os.ReadFile(journal)
		stageBytes, _ := os.ReadFile(stage)
		if string(journalBytes) != "leave-unread" || string(stageBytes) != "leave-unread" {
			t.Fatalf("builder recovered or rewrote mutation residue: journal=%q stage=%q", journalBytes, stageBytes)
		}
	})

	t.Run("scratch board read outcomes are explicit", func(t *testing.T) {
		t.Run("open", func(t *testing.T) {
			repo, requestPath, _, _, _ := readinessSnapshotRepo(t, "feature")
			annotation := &artifact.Annotation{
				ID: "a-01ARZ3NDEKTSV4RRFFQ69G5FAV", TS: "2026-08-18T12:00:00Z", Author: "tester",
				Board: &artifact.BoardAnchor{Story: "feature-alpha", X: 1, Y: 2}, Type: artifact.AnnotationQuestion,
				Body: "Which route?", Status: artifact.AnnotationOpen,
			}
			if err := boardio.AppendAnnotation(boardio.AnnotationsDir(repo.Dir), boardio.AnnotationFileForBoard("feature-alpha"), annotation); err != nil {
				t.Fatal(err)
			}
			snapshot := buildReadinessSnapshot(t, repo, requestPath, policyconflict.VerdictPass, localReadinessSnapshotBuilder{})
			concern := readinessSnapshotConcern(t, snapshot, "shape/board/question/"+annotation.ID)
			if concern.State != readinesspilot.StateUnproven || concern.Destination.BoardPath != "/b/design%2Ffeature-alpha/board/spec/feature-alpha" {
				t.Fatalf("open board concern = %+v", concern)
			}
		})

		t.Run("unavailable", func(t *testing.T) {
			repo, requestPath, _, _, _ := readinessSnapshotRepo(t, "feature")
			builder := localReadinessSnapshotBuilder{readAnnotations: func(string) ([]*artifact.Annotation, error) {
				return nil, &os.PathError{Op: "open", Path: boardio.AnnotationsDir(repo.Dir), Err: os.ErrPermission}
			}}
			snapshot := buildReadinessSnapshot(t, repo, requestPath, policyconflict.VerdictPass, builder)
			concern := readinessSnapshotConcern(t, snapshot, "shape/board")
			if concern.State != readinesspilot.StateUnproven {
				t.Fatalf("board state = %q, want unproven", concern.State)
			}
			assertReadinessStringsContain(t, concern.Witnesses, "permission denied")
			assertReadinessBoardDestination(t, concern, "/b/design%2Ffeature-alpha/board/spec/feature-alpha")
		})

		t.Run("malformed record is operational", func(t *testing.T) {
			repo, requestPath, _, _, _ := readinessSnapshotRepo(t, "feature")
			dir := boardio.AnnotationsDir(repo.Dir)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "board--feature-alpha.jsonl"), []byte("not-json\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			builder := localReadinessSnapshotBuilder{providerFactory: readinessSnapshotProviderFactory(t, repo.Dir, policyconflict.VerdictPass, nil)}
			_, err := builder.Build(context.Background(), repo.Dir, requestPath)
			if err == nil || !strings.Contains(err.Error(), "reading scratch board annotations") || !strings.Contains(err.Error(), "strict json decode") {
				t.Fatalf("Build error = %v, want malformed board record operational error", err)
			}
		})
	})
}

func TestReadinessSnapshotConflictVerdictsAndDestinations(t *testing.T) {
	tests := []struct {
		verdict policyconflict.Verdict
		state   readinesspilot.State
	}{
		{verdict: policyconflict.VerdictPass, state: readinesspilot.StateProven},
		{verdict: policyconflict.VerdictBlockedViolated, state: readinesspilot.StateViolated},
		{verdict: policyconflict.VerdictBlockedUnproven, state: readinesspilot.StateUnproven},
	}
	for _, tc := range tests {
		t.Run(string(tc.verdict), func(t *testing.T) {
			repo, requestPath, _, targetRef, _ := readinessSnapshotRepo(t, "feature")
			snapshot := buildReadinessSnapshot(t, repo, requestPath, tc.verdict, localReadinessSnapshotBuilder{})
			concern := readinessSnapshotConcern(t, snapshot, "context/verdict")
			if concern.State != tc.state {
				t.Fatalf("context verdict state = %q, want %q", concern.State, tc.state)
			}
			if tc.state != readinesspilot.StateProven {
				assertReadinessCLI(t, concern.Destination.CLI, []string{"verdi", "context", "conflict", "--request", requestPath})
			}
			provenance := readinessSnapshotConcern(t, snapshot, "shape/provenance")
			if provenance.Destination.BoardPath != "/b/design%2Ffeature-alpha/board/spec/feature-alpha" || len(provenance.Destination.CLI) != 0 {
				t.Fatalf("shape provenance destination = %+v, want the existing branch board and no CLI", provenance.Destination)
			}
			review := readinessSnapshotConcern(t, snapshot, "review/action")
			assertReadinessCLI(t, review.Destination.CLI, []string{"verdi", "journey", targetRef})
		})
	}
}

func TestReadinessSnapshotEmittedCLIVectorsReachRegisteredCommands(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo, requestPath, _, _, _ := readinessSnapshotRepo(t, "story")

	snapshot, err := (localReadinessSnapshotBuilder{}).Build(context.Background(), repo.Dir, requestPath)
	if err != nil {
		t.Fatalf("Build with production dependencies: %v", err)
	}
	executedAreas := map[readinesspilot.AreaID]bool{}
	for _, concern := range snapshot.AllConcerns {
		cli := concern.Destination.CLI
		if len(cli) == 0 {
			continue
		}
		executedAreas[concern.Area] = true
		t.Run(concern.ID, func(t *testing.T) {
			stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, []string{"CI_DEFAULT_BRANCH=main"}, cli[1:]...)
			if strings.Contains(stderr, "usage:") || strings.Contains(stderr, "not implemented") {
				t.Fatalf("emitted CLI %q was rejected by registered dispatch grammar: exit=%d stdout=%q stderr=%q", cli, code, stdout, stderr)
			}
			if code < 0 || code > 2 {
				t.Fatalf("emitted CLI %q returned exit=%d, want honest clean/verdict/operational exit 0/1/2; stdout=%q stderr=%q", cli, code, stdout, stderr)
			}
		})
	}

	for _, area := range []readinesspilot.AreaID{readinesspilot.AreaSuccess, readinesspilot.AreaContext, readinesspilot.AreaReview} {
		if !executedAreas[area] {
			t.Errorf("no emitted CLI vector exercised for area %q", area)
		}
	}
	if executedAreas[readinesspilot.AreaShape] {
		t.Error("shape area emitted a CLI vector instead of the existing branch board destination")
	}
	provenance := readinessSnapshotConcern(t, snapshot, "shape/provenance")
	if provenance.Destination.BoardPath != "/b/design%2Fstory-alpha/board/spec/story-alpha" || len(provenance.Destination.CLI) != 0 {
		t.Errorf("shape provenance destination = %+v, want the existing branch board and no invented CLI", provenance.Destination)
	}
}

func assertReadinessBoardDestination(t *testing.T, concern readinesspilot.Concern, want string) {
	t.Helper()
	if concern.Destination.BoardPath != want || len(concern.Destination.CLI) != 0 {
		t.Fatalf("concern %q destination = %+v, want board path %q and no CLI", concern.ID, concern.Destination, want)
	}
}

func TestReadinessSnapshotPersistenceBoundaryAndD4Cache(t *testing.T) {
	t.Run("fake provider creates no readiness-named path or record", func(t *testing.T) {
		repo, requestPath, _, _, _ := readinessSnapshotRepo(t, "feature")
		_ = buildReadinessSnapshot(t, repo, requestPath, policyconflict.VerdictPass, localReadinessSnapshotBuilder{})
		assertNoReadinessPersistence(t, repo.Dir)
	})

	t.Run("real semantic miss publishes only D4 and releases transient lock", func(t *testing.T) {
		repo := buildContextCompileRepo(t, map[string]string{
			".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
		})
		judge := writeContextConflictJudge(t, "printf '%s\\n' '"+contextConflictNoConflictJudgeResult+"'")
		configureContextConflictJudge(t, repo, judge, 0)
		checkoutBranch(t, repo.Dir, "design/feature-alpha")
		requestPath := writeContextRequestFile(t, repo.Dir, "readiness-request.json", contextRequestBytes(t, "spec/feature-alpha", contextcompile.PhaseDesign, nil))

		snapshot, err := (localReadinessSnapshotBuilder{}).Build(context.Background(), repo.Dir, requestPath)
		if err != nil {
			t.Fatalf("Build with real provider: %v", err)
		}
		if snapshot.TargetRef != "spec/feature-alpha" {
			t.Fatalf("snapshot target = %q", snapshot.TargetRef)
		}
		matches, err := filepath.Glob(filepath.Join(repo.Dir, ".verdi", "data", "cache", "policy-conflict-*.json"))
		if err != nil || len(matches) != 1 {
			t.Fatalf("D4 cache matches = %q, err=%v, want exactly one predecessor-owned record", matches, err)
		}
		if _, err := os.Stat(store.WriterLockPath(repo.Dir)); !os.IsNotExist(err) {
			t.Fatalf("writer lock remains after Build returned: %v", err)
		}
		assertNoReadinessPersistence(t, repo.Dir)
	})
}

func buildReadinessSnapshot(t *testing.T, repo *fixturegit.Repo, requestPath string, verdict policyconflict.Verdict, builder localReadinessSnapshotBuilder) readinesspilot.Snapshot {
	t.Helper()
	builder.providerFactory = readinessSnapshotProviderFactory(t, repo.Dir, verdict, nil)
	snapshot, err := builder.Build(context.Background(), repo.Dir, requestPath)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return snapshot
}

func readinessSnapshotRepo(t *testing.T, class string) (*fixturegit.Repo, string, []byte, string, string) {
	t.Helper()
	files := map[string]string{
		".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
	}
	name := "feature-alpha"
	if class == "story" {
		name = "story-alpha"
		files[".verdi/specs/active/story-alpha/spec.md"] = readinessStorySpec
	}
	repo := buildContextCompileRepo(t, files)
	checkoutBranch(t, repo.Dir, "design/"+name)
	targetRef := "spec/" + name
	requestBytes := contextRequestBytes(t, targetRef, contextcompile.PhaseDesign, nil)
	requestPath := writeContextRequestFile(t, repo.Dir, "readiness-request.json", requestBytes)
	return repo, requestPath, requestBytes, targetRef, store.ActiveSpecPath(repo.Dir, name)
}

func readinessProvenanceEntry(t *testing.T, spec string) designprovenance.Entry {
	t.Helper()
	entry := designprovenance.Entry{
		Schema:         designprovenance.Schema,
		Spec:           spec,
		PreviousDigest: "sha256:" + strings.Repeat("a", 64),
		ResultDigest:   "sha256:" + strings.Repeat("b", 64),
		Attribution:    governanceprincipal.NewUnauthenticatedAttribution(),
		Harness:        "codex",
		PolicyDigest:   "sha256:" + strings.Repeat("c", 64),
		Context:        designprovenance.UnavailableContext(),
		Operations: []designprovenance.Operation{{
			Op: designprovenance.OpSetProblem, Text: "changed problem", Anchor: "problem",
		}},
		Changes: []designprovenance.Change{{
			Target: "problem", Change: designprovenance.ChangeReplaced,
			BeforeDigest: "sha256:" + strings.Repeat("a", 64), AfterDigest: "sha256:" + strings.Repeat("b", 64),
		}},
		Excerpts: []designprovenance.Excerpt{},
	}
	if err := entry.Seal(); err != nil {
		t.Fatalf("Seal provenance entry: %v", err)
	}
	return entry
}

func assertNoReadinessPersistence(t *testing.T, root string) {
	t.Helper()
	dataRoot := filepath.Join(root, ".verdi", "data")
	err := filepath.WalkDir(dataRoot, func(path string, entry os.DirEntry, err error) error {
		if errors.Is(err, os.ErrNotExist) {
			return filepath.SkipDir
		}
		if err != nil {
			return err
		}
		if strings.Contains(strings.ToLower(entry.Name()), "readiness") {
			t.Fatalf("readiness-owned persistence found at %s", path)
		}
		return nil
	})
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("walk .verdi/data: %v", err)
	}
}

const readinessStorySpec = `---
id: spec/story-alpha
kind: spec
title: "Borrower appeal: exact source title"
owners: [alpha-team]
class: story
story: jira:ALPHA-1
problem: {text: "The story is unclear.", anchor: problem}
outcome: {text: "The story is reviewable.", anchor: outcome}
acceptance_criteria:
  - {id: ac-1, text: "the story works", evidence: [behavioral], anchor: ac-1}
open_questions:
  - {id: oq-1, text: "which route applies?", anchor: oq-1}
links:
  - {type: implements, ref: spec/feature-alpha#ac-1}
---
# Story Alpha

## Problem

The story is unclear.

## Outcome

The story is reviewable.

## AC-1

The story works.

## OQ-1

Which route applies?
`
