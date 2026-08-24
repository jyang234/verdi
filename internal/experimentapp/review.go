package experimentapp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/experiment"
)

// ReviewPacket is the deterministic read-only registration-review surface.
// It spells out the registered scientific contract instead of projecting a
// second state machine from experiment bytes.
type ReviewPacket struct {
	Experiment             experiment.ProvenanceExperiment `json:"experiment"`
	AcceptedHead           string                          `json:"accepted_head"`
	ExperimentPath         string                          `json:"experiment_path"`
	Question               string                          `json:"question"`
	Class                  string                          `json:"class"`
	Candidates             []experiment.Candidate          `json:"candidates"`
	Evaluator              experiment.Evaluator            `json:"evaluator"`
	Workload               experiment.ArtifactRef          `json:"workload"`
	Fixtures               []experiment.ArtifactRef        `json:"fixtures"`
	Contract               experiment.ArtifactRef          `json:"contract"`
	PrimaryMetric          experiment.PrimaryMetric        `json:"primary_metric"`
	Guards                 []experiment.Guard              `json:"guards"`
	Execution              experiment.Execution            `json:"execution"`
	Reproduction           *experiment.ReproductionRule    `json:"reproduction,omitempty"`
	RetentionPolicy        string                          `json:"retention_policy"`
	DefinitionDigest       string                          `json:"definition_digest"`
	CapabilitiesDigest     string                          `json:"capabilities_digest"`
	Capabilities           experiment.Capabilities         `json:"capabilities"`
	PolicyDigest           string                          `json:"policy_digest"`
	RetainedArtifactBytes  int64                           `json:"retained_artifact_bytes"`
	AcceptedArtifactDigest string                          `json:"accepted_artifact_digest"`
	ProposedArtifactDigest string                          `json:"proposed_artifact_digest"`
}

// ReviewRegistrationResult contains both the typed packet and its canonical
// bytes. A verdict result can still carry the deterministic evidence packet.
type ReviewRegistrationResult struct {
	Outcome                Outcome
	Packet                 ReviewPacket
	PacketBytes            []byte
	PacketDigest           string
	AcceptedArtifactDigest string
	ProposedArtifactDigest string
}

type artifactEntry struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
}

// ReviewRegistration constructs registration evidence without any writer
// capability. Accepted bytes are all read from one exact commit.
func (s *Service) ReviewRegistration(ctx context.Context, identity Identity) ReviewRegistrationResult {
	if err := identity.validate(); err != nil {
		return ReviewRegistrationResult{Outcome: operationalOutcome("invalid-request", err)}
	}
	accepted, err := resolveAcceptedBase(ctx, s.git, identity)
	if err != nil {
		var stale *staleAcceptedHeadError
		if errors.As(err, &stale) {
			return ReviewRegistrationResult{Outcome: verdictOutcome("accepted-head-stale", stale.Error())}
		}
		return ReviewRegistrationResult{Outcome: operationalOutcome("accepted-tree-invalid", err)}
	}
	validation := s.validateDraftAtRevision(ctx, identity, accepted.revision)
	if validation.Outcome.Classification != ClassificationClean {
		return ReviewRegistrationResult{Outcome: validation.Outcome}
	}
	proposedFiles, err := readProposedArtifactFiles(identity.CheckoutRoot, validation.ExperimentPath)
	if err != nil {
		return ReviewRegistrationResult{Outcome: operationalOutcome("proposal-tree-invalid", err)}
	}
	acceptedPath := accepted.experimentPath
	if acceptedPath == "" {
		acceptedPath = validation.ExperimentPath
	}
	acceptedDigest, err := artifactSetDigest(accepted.source.files, acceptedPath)
	if err != nil {
		return ReviewRegistrationResult{Outcome: operationalOutcome("accepted-artifact-digest-invalid", err)}
	}
	proposedDigest, err := artifactSetDigest(proposedFiles, validation.ExperimentPath)
	if err != nil {
		return ReviewRegistrationResult{Outcome: operationalOutcome("proposed-artifact-digest-invalid", err)}
	}

	definition := validation.Definition
	packet := ReviewPacket{
		Experiment:   experiment.ProvenanceExperiment{Spike: identity.Spike, ID: identity.ExperimentID},
		AcceptedHead: accepted.revision.Head, ExperimentPath: validation.ExperimentPath,
		Question: definition.Question, Class: definition.Class,
		Candidates: append([]experiment.Candidate(nil), definition.Candidates...),
		Evaluator:  cloneDefinition(definition).Evaluator, Workload: definition.Workload,
		Fixtures: append([]experiment.ArtifactRef(nil), definition.Fixtures...), Contract: definition.Contract,
		PrimaryMetric: definition.Decision.PrimaryMetric,
		Guards:        append([]experiment.Guard(nil), cloneDefinition(definition).Decision.Guards...),
		Execution:     definition.Execution, Reproduction: cloneDefinition(definition).Reproduction,
		RetentionPolicy:  definition.RetentionPolicy,
		DefinitionDigest: validation.DefinitionDigest, CapabilitiesDigest: validation.CapabilitiesDigest,
		Capabilities: cloneCapabilities(validation.Capabilities), PolicyDigest: validation.PolicyDigest,
		RetainedArtifactBytes: validation.PolicyLimits.RetainedArtifactBytes, AcceptedArtifactDigest: acceptedDigest,
		ProposedArtifactDigest: proposedDigest,
	}
	packetBytes, err := canonjson.Marshal(packet)
	if err != nil {
		return ReviewRegistrationResult{Outcome: operationalOutcome("review-packet-invalid", err)}
	}
	packetDigest, err := canonjson.Digest(packet)
	if err != nil {
		return ReviewRegistrationResult{Outcome: operationalOutcome("review-packet-invalid", err)}
	}
	result := ReviewRegistrationResult{
		Outcome: cleanOutcome(), Packet: packet, PacketBytes: append([]byte(nil), packetBytes...),
		PacketDigest: packetDigest, AcceptedArtifactDigest: acceptedDigest,
		ProposedArtifactDigest: proposedDigest,
	}
	history, err := compareProvenanceHistories(
		accepted.source.files, acceptedPath,
		proposedFiles, validation.ExperimentPath,
		packet.Experiment, acceptedDigest,
	)
	if err != nil {
		result.Outcome = operationalOutcome("mutation-provenance-invalid", err)
		return result
	}
	if history.mismatch != "" {
		result.Outcome = verdictOutcome("direct-draft-unreconciled", history.mismatch)
		return result
	}
	if !history.reaches(proposedDigest) {
		result.Outcome = verdictOutcome("direct-draft-unreconciled", "mutation provenance does not reach the proposed human-authored artifact digest")
	}
	return result
}

func readProposedArtifactFiles(root, experimentPath string) (map[string][]byte, error) {
	base := filepath.Join(root, filepath.FromSlash(experimentPath))
	files := map[string][]byte{}
	err := filepath.WalkDir(base, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		repoPath := filepath.ToSlash(rel)
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("experimentapp: proposed artifact %q is not a regular file", repoPath)
		}
		if err := experiment.ValidateRepoRelativePath(repoPath); err != nil {
			return err
		}
		data, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		files[repoPath] = data
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("experimentapp: read proposed artifact tree: %w", err)
	}
	return files, nil
}

func artifactSetDigest(files map[string][]byte, experimentPath string) (string, error) {
	prefix := strings.TrimSuffix(experimentPath, "/") + "/"
	entries := make([]artifactEntry, 0, len(files))
	for name, data := range files {
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		relative := strings.TrimPrefix(name, prefix)
		if !isMutationArtifact(relative) {
			continue
		}
		entries = append(entries, artifactEntry{Path: relative, Digest: rawDigest(data)})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return canonjson.Digest(entries)
}

// isMutationArtifact is the one human/agent mutation projection. Machine
// evidence has independent receipts and identities, so only the three
// design-owned evidence locations are omitted from mutation provenance.
func isMutationArtifact(relative string) bool {
	if relative == experiment.ProvenanceFile || relative == "recommendation.md" {
		return false
	}
	return !strings.HasPrefix(relative, "runs/") && !strings.HasPrefix(relative, "selected/")
}

type provenanceHistories struct {
	accepted       []experiment.ProvenanceRecord
	proposed       []experiment.ProvenanceRecord
	acceptedDigest string
	mismatch       string
}

func (h provenanceHistories) reaches(digest string) bool {
	if len(h.proposed) == 0 {
		return digest == h.acceptedDigest
	}
	return h.proposed[len(h.proposed)-1].ResultDigest == digest
}

func compareProvenanceHistories(acceptedFiles map[string][]byte, acceptedPath string, proposedFiles map[string][]byte, proposedPath string, identity experiment.ProvenanceExperiment, acceptedDigest string) (provenanceHistories, error) {
	acceptedBytes := acceptedFiles[path.Join(acceptedPath, experiment.ProvenanceFile)]
	proposedBytes := proposedFiles[path.Join(proposedPath, experiment.ProvenanceFile)]
	acceptedRecords, err := experiment.DecodeProvenanceLog(acceptedBytes)
	if err != nil {
		return provenanceHistories{}, fmt.Errorf("accepted mutation provenance: %w", err)
	}
	proposedRecords, err := experiment.DecodeProvenanceLog(proposedBytes)
	if err != nil {
		return provenanceHistories{}, fmt.Errorf("proposed mutation provenance: %w", err)
	}
	history := provenanceHistories{accepted: acceptedRecords, proposed: proposedRecords, acceptedDigest: acceptedDigest}
	for _, records := range [][]experiment.ProvenanceRecord{acceptedRecords, proposedRecords} {
		for _, record := range records {
			if record.Experiment != identity {
				history.mismatch = "mutation provenance belongs to a different experiment"
				return history, nil
			}
		}
	}
	if len(acceptedRecords) > 0 && acceptedRecords[len(acceptedRecords)-1].ResultDigest != acceptedDigest {
		history.mismatch = "accepted mutation provenance does not reach the accepted human-authored artifact digest"
		return history, nil
	}
	if !bytes.HasPrefix(proposedBytes, acceptedBytes) || len(proposedRecords) < len(acceptedRecords) {
		history.mismatch = "proposed mutation provenance does not retain the exact accepted append-only prefix"
		return history, nil
	}
	if len(acceptedRecords) == 0 && len(proposedRecords) > 0 && proposedRecords[0].PreviousDigest != acceptedDigest {
		history.mismatch = "proposed mutation provenance is not anchored to the accepted human-authored artifact digest"
	}
	return history, nil
}
