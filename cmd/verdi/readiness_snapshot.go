package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/boardio"
	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/designprovenance"
	"github.com/jyang234/verdi/internal/journey"
	"github.com/jyang234/verdi/internal/policyconflict"
	"github.com/jyang234/verdi/internal/readinesspilot"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/workbench"
)

// readinessSnapshotBuilder is the serve command's one startup projection
// boundary. Implementations return an immutable value; callers never retain a
// path, decoder, provider, or live fact source behind the snapshot.
type readinessSnapshotBuilder interface {
	Build(ctx context.Context, root, requestPath string) (readinesspilot.Snapshot, error)
}

// localReadinessSnapshotBuilder adapts existing source owners into the pure
// readiness projection. Its optional function fields are narrow test seams;
// the useful zero value selects every production predecessor.
type localReadinessSnapshotBuilder struct {
	readFile        func(string) ([]byte, error)
	projectJourney  func(context.Context, *store.Config, string) (journey.Record, error)
	providerFactory contextConflictProviderFactory
	readAnnotations func(string) ([]*artifact.Annotation, error)
}

// Build captures one complete startup snapshot. All source reads and the
// predecessor-owned policy evaluation finish before the value is returned.
func (b localReadinessSnapshotBuilder) Build(ctx context.Context, root, requestPath string) (readinesspilot.Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return readinesspilot.Snapshot{}, fmt.Errorf("building readiness snapshot: %w", err)
	}
	if requestPath == "-" {
		return readinesspilot.Snapshot{}, errors.New("building readiness snapshot: --context-request does not accept stdin ('-')")
	}
	validatedPath, err := validatedConflictRequestPath(root, requestPath)
	if err != nil {
		return readinesspilot.Snapshot{}, fmt.Errorf("building readiness snapshot: %w", err)
	}

	readFile := b.readFile
	if readFile == nil {
		readFile = os.ReadFile
	}
	requestBytes, err := readFile(validatedPath)
	if err != nil {
		return readinesspilot.Snapshot{}, fmt.Errorf("building readiness snapshot: reading --context-request: %w", err)
	}
	request, err := contextcompile.DecodeRequest(requestBytes)
	if err != nil {
		return readinesspilot.Snapshot{}, fmt.Errorf("building readiness snapshot: decoding --context-request: %w", err)
	}
	if request.Phase != contextcompile.PhaseDesign {
		return readinesspilot.Snapshot{}, fmt.Errorf("building readiness snapshot: request must use phase %q, got %q", contextcompile.PhaseDesign, request.Phase)
	}

	ref, err := artifact.ParseRef(request.Spec)
	if err != nil || ref.Kind != artifact.KindSpec || ref.Pinned() || ref.Fragment() {
		return readinesspilot.Snapshot{}, fmt.Errorf("building readiness snapshot: request target %q is not an unpinned whole spec ref", request.Spec)
	}
	name := ref.Name

	cfg, err := store.Open(root)
	if err != nil {
		return readinesspilot.Snapshot{}, fmt.Errorf("building readiness snapshot: opening store: %w", err)
	}
	projectJourney := b.projectJourney
	if projectJourney == nil {
		projector := journey.NewProjector()
		projectJourney = projector.Project
	}
	record, err := projectJourney(ctx, cfg, request.Spec)
	if err != nil {
		return readinesspilot.Snapshot{}, fmt.Errorf("building readiness snapshot: projecting journey: %w", err)
	}
	if record.Target.Ref != request.Spec {
		return readinesspilot.Snapshot{}, fmt.Errorf("building readiness snapshot: journey target %q does not match request target %q", record.Target.Ref, request.Spec)
	}
	if record.Target.Path != store.ActiveSpecRelPath(name) {
		return readinesspilot.Snapshot{}, fmt.Errorf("building readiness snapshot: journey target path %q does not match active design target %q", record.Target.Path, store.ActiveSpecRelPath(name))
	}
	if !record.Repository.Branch.Known || !record.Repository.Head.Known {
		return readinesspilot.Snapshot{}, errors.New("building readiness snapshot: journey repository branch and HEAD must both be proven")
	}
	branch := record.Repository.Branch.Value
	head := record.Repository.Head.Value
	wantBranch := "design/" + name
	if branch != wantBranch {
		return readinesspilot.Snapshot{}, fmt.Errorf("building readiness snapshot: current branch %q is not selected design branch %q", branch, wantBranch)
	}

	computed := contextcompile.Expected{Branch: branch, Head: head}
	if request.Expected != nil && *request.Expected != computed {
		return readinesspilot.Snapshot{}, fmt.Errorf("building readiness snapshot: --context-request expected repository %+v does not match computed repository %+v", *request.Expected, computed)
	}
	request.Expected = &computed

	conflictRequest := policyconflict.Request{
		Schema: policyconflict.RequestSchema,
		Target: policyconflict.Target{
			Kind: policyconflict.TargetAcceptanceCandidate,
			AcceptanceCandidate: &policyconflict.AcceptanceCandidate{
				Adapter:  request.Adapter,
				Expected: computed,
				Grants:   request.Grants,
				Scope:    request.Scope,
				Spec:     request.Spec,
			},
		},
	}
	if err := conflictRequest.Validate(); err != nil {
		return readinesspilot.Snapshot{}, fmt.Errorf("building readiness snapshot: constructing conflict request: %w", err)
	}
	providerFactory := b.providerFactory
	if providerFactory == nil {
		providerFactory = newLocalContextConflictProvider
	}
	provider, err := providerFactory(root, conflictRequest)
	if err != nil {
		return readinesspilot.Snapshot{}, fmt.Errorf("building readiness snapshot: constructing policy-conflict provider: %w", err)
	}
	if provider == nil {
		return readinesspilot.Snapshot{}, errors.New("building readiness snapshot: policy-conflict provider is nil")
	}
	conflictResult, err := provider.Evaluate(ctx, conflictRequest)
	if err != nil {
		return readinesspilot.Snapshot{}, fmt.Errorf("building readiness snapshot: evaluating policy conflicts: %w", err)
	}
	if err := readinessReportIdentity(conflictResult.Report, request.Spec, record.Target.Path, branch, head); err != nil {
		return readinesspilot.Snapshot{}, fmt.Errorf("building readiness snapshot: %w", err)
	}

	specPath := filepath.Join(root, filepath.FromSlash(record.Target.Path))
	specBytes, err := readFile(specPath)
	if err != nil {
		return readinesspilot.Snapshot{}, fmt.Errorf("building readiness snapshot: reading target spec: %w", err)
	}
	frontmatter, _, err := artifact.SplitFrontmatter(specBytes)
	if err != nil {
		return readinesspilot.Snapshot{}, fmt.Errorf("building readiness snapshot: splitting target spec: %w", err)
	}
	spec, err := artifact.DecodeSpec(frontmatter)
	if err != nil {
		return readinesspilot.Snapshot{}, fmt.Errorf("building readiness snapshot: decoding target spec: %w", err)
	}
	if spec.ID != request.Spec || string(spec.Class) != record.Target.Class {
		return readinesspilot.Snapshot{}, fmt.Errorf("building readiness snapshot: decoded spec identity (%q, %q) does not match request/journey identity (%q, %q)", spec.ID, spec.Class, request.Spec, record.Target.Class)
	}
	if spec.Class != artifact.ClassFeature && spec.Class != artifact.ClassStory {
		// vocab:identity — operational diagnostic naming the fixed artifact class identities
		return readinesspilot.Snapshot{}, fmt.Errorf("building readiness snapshot: decoded spec class %q is not feature or story", spec.Class)
	}
	candidate := conflictResult.Report.Input.Target.Candidate
	if candidate.ContentDigest != readinessDigest(specBytes) {
		return readinesspilot.Snapshot{}, fmt.Errorf("building readiness snapshot: conflict report target content digest %q does not match decoded spec bytes %q", candidate.ContentDigest, readinessDigest(specBytes))
	}

	provenance, err := b.provenanceFacts(readFile, root, name, request.Spec, specBytes)
	if err != nil {
		return readinesspilot.Snapshot{}, err
	}
	mutation, err := readinessMutationFacts(root, name)
	if err != nil {
		return readinesspilot.Snapshot{}, err
	}
	provenance.MutationState = mutation.MutationState
	provenance.MutationWitnesses = mutation.MutationWitnesses
	readAnnotations := b.readAnnotations
	if readAnnotations == nil {
		readAnnotations = boardio.ReadAllAnnotations
	}
	board, err := readinessBoardFacts(readAnnotations, root, name)
	if err != nil {
		return readinesspilot.Snapshot{}, err
	}

	declared := artifact.DeclaredObjectIDs(spec)
	declaredIDs := make([]string, 0, len(declared))
	for id := range declared {
		declaredIDs = append(declaredIDs, id)
	}
	sort.Strings(declaredIDs)
	openQuestionIDs := make([]string, len(spec.OpenQuestions))
	for i, question := range spec.OpenQuestions {
		openQuestionIDs[i] = question.ID
	}
	sort.Strings(openQuestionIDs)

	input := readinesspilot.Input{
		Target: readinesspilot.TargetFacts{
			Ref: request.Spec, Class: string(spec.Class), Branch: branch, Head: head,
			BoardPath: workbench.BranchBoardHref(branch, name),
		},
		Shape: readinesspilot.ShapeFacts{
			ProblemPresent: spec.Problem != nil, OutcomePresent: spec.Outcome != nil,
			DeclaredObjectIDs: declaredIDs, OpenQuestionIDs: openQuestionIDs,
		},
		Provenance: provenance,
		Board:      board,
		Journey:    record,
		Conflict:   conflictResult.Report,
		Fallbacks: readinesspilot.Fallbacks{
			Shape:   []string{"verdi", "design", "provenance", request.Spec},
			Success: []string{"verdi", "journey", request.Spec, "--success"},
			Context: []string{"verdi", "context", "conflict", "--request", requestPath},
			Review:  []string{"verdi", "journey", request.Spec, "--review"},
		},
		RequestDigest: readinessDigest(requestBytes),
	}
	snapshot, err := readinesspilot.Derive(input)
	if err != nil {
		return readinesspilot.Snapshot{}, fmt.Errorf("building readiness snapshot: deriving projection: %w", err)
	}
	return snapshot, nil
}

func readinessReportIdentity(report policyconflict.Report, targetRef, targetPath, branch, head string) error {
	if report.Input.Target.Kind != policyconflict.TargetAcceptanceCandidate || report.Input.Target.Candidate == nil || report.Input.Target.Accepted != nil {
		return errors.New("conflict report target is not exactly one acceptance candidate")
	}
	candidate := report.Input.Target.Candidate
	if candidate.Ref != targetRef || candidate.Path != targetPath {
		return fmt.Errorf("conflict report target (%q, %q) does not match journey target (%q, %q)", candidate.Ref, candidate.Path, targetRef, targetPath)
	}
	if candidate.Branch != branch || candidate.Head != head {
		return fmt.Errorf("conflict report target branch/HEAD (%q, %q) does not match journey repository (%q, %q)", candidate.Branch, candidate.Head, branch, head)
	}
	if !report.Input.Repository.Branch.Known || report.Input.Repository.Branch.Value != branch ||
		!report.Input.Repository.Head.Known || report.Input.Repository.Head.Value != head {
		return fmt.Errorf("conflict report repository branch/HEAD does not match journey repository (%q, %q)", branch, head)
	}
	return nil
}

func (b localReadinessSnapshotBuilder) provenanceFacts(readFile func(string) ([]byte, error), root, name, targetRef string, specBytes []byte) (readinesspilot.ProvenanceFacts, error) {
	path := store.DesignProvenancePath(root, store.ZoneActive, name)
	data, err := readFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return readinesspilot.ProvenanceFacts{
				ChainState: readinesspilot.StateUnproven, ChainWitnesses: []string{"design-provenance sidecar is absent"},
			}, nil
		}
		return readinesspilot.ProvenanceFacts{}, fmt.Errorf("building readiness snapshot: reading design provenance: %w", err)
	}
	entries, err := designprovenance.DecodeLog(data)
	if err != nil {
		return readinesspilot.ProvenanceFacts{}, fmt.Errorf("building readiness snapshot: decoding design provenance: %w", err)
	}
	if len(entries) == 0 {
		return readinesspilot.ProvenanceFacts{
			ChainState: readinesspilot.StateUnproven, ChainWitnesses: []string{"design-provenance sidecar contains no entries"},
		}, nil
	}

	state := readinesspilot.StateProven
	witnesses := []string{"design-provenance chain classified"}
	for i, entry := range entries {
		if entry.Spec != targetRef {
			return readinesspilot.ProvenanceFacts{}, fmt.Errorf("building readiness snapshot: design provenance entry[%d] target %q does not match %q", i, entry.Spec, targetRef)
		}
		if entry.Context == designprovenance.UnavailableContext() {
			state = readinesspilot.StateUnproven
			witnesses = append(witnesses, "design provenance context is unavailable: "+entry.Context.Reason)
		}
		if entry.UnclassifiedGap != nil {
			state = readinesspilot.StateUnproven
			witnesses = append(witnesses, "design-provenance chain contains an unclassified direct-Markdown gap")
		}
	}
	if entries[len(entries)-1].ResultDigest != readinessDigest(specBytes) {
		state = readinesspilot.StateUnproven
		witnesses = append(witnesses, "current spec bytes follow an unclassified direct Markdown change")
	}
	return readinesspilot.ProvenanceFacts{ChainState: state, ChainWitnesses: witnesses}, nil
}

func readinessMutationFacts(root, name string) (readinesspilot.ProvenanceFacts, error) {
	type residuePath struct {
		path  string
		label string
	}
	paths := []residuePath{
		// vocab:identity — predecessor store-path identities, not lifecycle display prose
		{path: store.DraftMutationDir(root, name), label: "draft mutation staging directory"},
		// vocab:identity — predecessor store-path identities, not lifecycle display prose
		{path: store.DraftMutationJournalPath(root, name), label: "draft mutation journal"},
		// vocab:identity — predecessor store-path identities, not lifecycle display prose
		{path: store.DraftMutationSpecStagePath(root, name), label: "draft mutation spec stage"},
		// vocab:identity — predecessor store-path identities, not lifecycle display prose
		{path: store.DraftMutationProvenanceStagePath(root, name), label: "draft mutation provenance stage"},
	}
	witnesses := make([]string, 0, len(paths))
	for _, candidate := range paths {
		_, err := os.Lstat(candidate.path)
		switch {
		case err == nil:
			rel, relErr := filepath.Rel(root, candidate.path)
			if relErr != nil {
				return readinesspilot.ProvenanceFacts{}, fmt.Errorf("building readiness snapshot: naming %s: %w", candidate.label, relErr)
			}
			witnesses = append(witnesses, candidate.label+" is present: "+filepath.ToSlash(rel))
		case errors.Is(err, os.ErrNotExist):
			continue
		default:
			return readinesspilot.ProvenanceFacts{}, fmt.Errorf("building readiness snapshot: inspecting %s: %w", candidate.label, err)
		}
	}
	if len(witnesses) == 0 {
		// vocab:identity — predecessor store-path identity, not lifecycle display prose
		return readinesspilot.ProvenanceFacts{MutationState: readinesspilot.StateProven, MutationWitnesses: []string{"no draft mutation residue"}}, nil
	}
	return readinesspilot.ProvenanceFacts{MutationState: readinesspilot.StateUnproven, MutationWitnesses: witnesses}, nil
}

func readinessBoardFacts(readAnnotations func(string) ([]*artifact.Annotation, error), root, name string) (readinesspilot.BoardFacts, error) {
	annotations, err := readAnnotations(boardio.AnnotationsDir(root))
	if err != nil {
		var pathErr *os.PathError
		if !errors.As(err, &pathErr) {
			return readinesspilot.BoardFacts{}, fmt.Errorf("building readiness snapshot: reading scratch board annotations: %w", err)
		}
		witness := strings.ReplaceAll(err.Error(), root, "<store-root>")
		return readinesspilot.BoardFacts{
			State: readinesspilot.StateUnproven, OpenItems: []readinesspilot.BoardItem{},
			Witnesses: []string{"scratch board enumeration unavailable: " + witness},
		}, nil
	}
	items := make([]readinesspilot.BoardItem, 0)
	for _, annotation := range annotations {
		if annotation == nil || annotation.Status != artifact.AnnotationOpen || annotation.Board == nil || annotation.Board.Story != name {
			continue
		}
		switch annotation.Type {
		case artifact.AnnotationQuestion:
			items = append(items, readinesspilot.BoardItem{ID: annotation.ID, Kind: "question"})
		case artifact.AnnotationAgentTask:
			items = append(items, readinesspilot.BoardItem{ID: annotation.ID, Kind: "agent-task"})
		}
	}
	return readinesspilot.BoardFacts{
		State: readinesspilot.StateProven, OpenItems: items, Witnesses: []string{"scratch board enumerated"},
	}, nil
}

func readinessDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

var _ readinessSnapshotBuilder = localReadinessSnapshotBuilder{}
