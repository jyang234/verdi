// facts.go carries the non-conflict source-fact resolvers Load composes:
// design-provenance chain classification, draft-mutation residue
// detection, scratch-board enumeration, the spike-stub claimed-question
// mapping, and the report-target identity cross-check — moved verbatim
// from cmd/verdi/readiness_snapshot.go, unexported and renamed to drop
// their "readiness"/adapter-specific prefixes now that they live in their
// own package.
package readinessload

import (
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
	"github.com/jyang234/verdi/internal/designprovenance"
	"github.com/jyang234/verdi/internal/policyconflict"
	"github.com/jyang234/verdi/internal/readinesspilot"
	"github.com/jyang234/verdi/internal/store"
)

// reportIdentity cross-checks a completed policy-conflict report's target
// identity against the resolved journey target and repository, exactly as
// cmd/verdi/readiness_snapshot.go's old readinessReportIdentity did.
func reportIdentity(report policyconflict.Report, targetRef, targetPath, branch, head string) error {
	if report.Input.Target.Kind != policyconflict.TargetAcceptanceCandidate || report.Input.Target.Candidate == nil || report.Input.Target.Accepted != nil {
		return errors.New("conflict report target is not exactly one acceptance candidate")
	}
	candidate := report.Input.Target.Candidate
	if candidate.Ref != targetRef || candidate.Path != targetPath {
		return fmt.Errorf("conflict report target (%q, %q) does not match resolved target (%q, %q)", candidate.Ref, candidate.Path, targetRef, targetPath)
	}
	if candidate.Branch != branch || candidate.Head != head {
		return fmt.Errorf("conflict report target branch/HEAD (%q, %q) does not match resolved repository (%q, %q)", candidate.Branch, candidate.Head, branch, head)
	}
	if !report.Input.Repository.Branch.Known || report.Input.Repository.Branch.Value != branch ||
		!report.Input.Repository.Head.Known || report.Input.Repository.Head.Value != head {
		return fmt.Errorf("conflict report repository branch/HEAD does not match resolved repository (%q, %q)", branch, head)
	}
	return nil
}

// provenanceFacts classifies the target's design-provenance sidecar,
// exactly as cmd/verdi/readiness_snapshot.go's old
// localReadinessSnapshotBuilder.provenanceFacts did.
func provenanceFacts(readFile func(string) ([]byte, error), root, name, targetRef string, specBytes []byte) (readinesspilot.ProvenanceFacts, error) {
	path := store.DesignProvenancePath(root, store.ZoneActive, name)
	data, err := readFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return readinesspilot.ProvenanceFacts{
				ChainState: readinesspilot.StateUnproven, ChainWitnesses: []string{"design-provenance sidecar is absent"},
			}, nil
		}
		return readinesspilot.ProvenanceFacts{}, fmt.Errorf("readinessload: reading design provenance: %w", err)
	}
	entries, err := designprovenance.DecodeLog(data)
	if err != nil {
		return readinesspilot.ProvenanceFacts{}, fmt.Errorf("readinessload: decoding design provenance: %w", err)
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
			return readinesspilot.ProvenanceFacts{}, fmt.Errorf("readinessload: design provenance entry[%d] target %q does not match %q", i, entry.Spec, targetRef)
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

// mutationFacts detects draft-mutation residue, exactly as
// cmd/verdi/readiness_snapshot.go's old readinessMutationFacts did.
func mutationFacts(root, name string) (readinesspilot.ProvenanceFacts, error) {
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
				return readinesspilot.ProvenanceFacts{}, fmt.Errorf("readinessload: naming %s: %w", candidate.label, relErr)
			}
			witnesses = append(witnesses, candidate.label+" is present: "+filepath.ToSlash(rel))
		case errors.Is(err, os.ErrNotExist):
			continue
		default:
			return readinesspilot.ProvenanceFacts{}, fmt.Errorf("readinessload: inspecting %s: %w", candidate.label, err)
		}
	}
	if len(witnesses) == 0 {
		// vocab:identity — predecessor store-path identity, not lifecycle display prose
		return readinesspilot.ProvenanceFacts{MutationState: readinesspilot.StateProven, MutationWitnesses: []string{"no draft mutation residue"}}, nil
	}
	return readinesspilot.ProvenanceFacts{MutationState: readinesspilot.StateUnproven, MutationWitnesses: witnesses}, nil
}

// boardFacts enumerates the target's open scratch-board items, exactly as
// cmd/verdi/readiness_snapshot.go's old readinessBoardFacts did.
func boardFacts(readAnnotations func(string) ([]*artifact.Annotation, error), root, name string) (readinesspilot.BoardFacts, error) {
	annotations, err := readAnnotations(boardio.AnnotationsDir(root))
	if err != nil {
		var pathErr *os.PathError
		if !errors.As(err, &pathErr) {
			return readinesspilot.BoardFacts{}, fmt.Errorf("readinessload: reading scratch board annotations: %w", err)
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

// claimedQuestionsOf maps a decoded feature spec's spike stubs
// (`spec.Stubs`, artifact.Stub{Spike,Resolves,Slug}) onto
// readinesspilot.ShapeFacts.ClaimedQuestions (PLAN.md §7 I-128 option (a);
// spec/uat-round-1 ac-10) — moved verbatim from
// cmd/verdi/readiness_snapshot.go's old readinessClaimedQuestions. Always
// non-nil, matching the field's required posture. VL-006
// (internal/lint/vl006.go) already guarantees `resolves` names a declared
// open question of the same spec; a spec that reaches this loader without
// having passed lint is refused by readinesspilot.Input.validate's own
// membership check instead of being silently dropped here.
func claimedQuestionsOf(stubs []artifact.Stub) []readinesspilot.ClaimedQuestion {
	bySlug := map[string][]string{}
	for _, stub := range stubs {
		if !stub.Spike {
			continue
		}
		for _, id := range stub.Resolves {
			bySlug[id] = append(bySlug[id], stub.Slug)
		}
	}
	ids := make([]string, 0, len(bySlug))
	for id := range bySlug {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	claims := make([]readinesspilot.ClaimedQuestion, 0, len(ids))
	for _, id := range ids {
		slugs := append([]string(nil), bySlug[id]...)
		sort.Strings(slugs)
		claims = append(claims, readinesspilot.ClaimedQuestion{QuestionID: id, StubSlugs: slugs})
	}
	return claims
}

// digest is the canonical sha256:<hex> content digest Load uses for both
// RequestDigest and the design-provenance result-digest comparison.
func readinessDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}
