package contextcompile

import (
	"bytes"
	"fmt"
	"unicode/utf8"

	"github.com/jyang234/verdi/internal/canonjson"
)

// BuildDataItem constructs and canonically encodes one provenance-wrapped
// non-authoritative payload. Instruction projections are deliberately refused:
// their exact bytes stay raw on the authority channel.
func BuildDataItem(candidate Candidate, kind IncludedKind, content []byte) (DataItem, []byte, error) {
	if kind == IncludedInstructionProjection {
		return DataItem{}, nil, fmt.Errorf("contextcompile: BuildDataItem: instruction-projection never receives a data wrapper")
	}
	if nonTextContent(content) {
		return DataItem{}, nil, fmt.Errorf("contextcompile: BuildDataItem: content for %s/%s is not text (invalid UTF-8 or contains NUL)", candidate.Source, candidate.ID)
	}
	if err := validateDataCandidate(candidate, kind); err != nil {
		return DataItem{}, nil, err
	}

	item := DataItem{
		Schema:         DataItemSchema,
		ID:             candidate.ID,
		Source:         candidate.Source,
		Kind:           kind,
		Classification: DataItemClassification,
		ContentDigest:  rawContentDigest(content),
		Content:        string(content),
	}
	if candidate.Path != "" {
		path := candidate.Path
		item.Path = &path
	}
	if candidate.Ref != "" {
		ref := candidate.Ref
		item.Ref = &ref
	}
	digest, err := canonjson.Digest(dataItemDocFor(item, ""))
	if err != nil {
		return DataItem{}, nil, fmt.Errorf("contextcompile: BuildDataItem: computing wrapper digest: %w", err)
	}
	item.Digest = digest
	encoded, err := EncodeDataItem(item)
	if err != nil {
		return DataItem{}, nil, fmt.Errorf("contextcompile: BuildDataItem: %w", err)
	}
	return item, encoded, nil
}

// nonTextContent is SI-89's exact binary predicate for complete committed
// bytes. BuildDataItem applies the same fail-closed boundary to every data
// payload so direct callers cannot wrap bytes the classifier would exclude.
func nonTextContent(content []byte) bool {
	return !utf8.Valid(content) || bytes.IndexByte(content, 0) >= 0
}

func validateDataCandidate(candidate Candidate, kind IncludedKind) error {
	if candidate.ID == "" {
		return fmt.Errorf("contextcompile: BuildDataItem: candidate id is empty")
	}
	if err := candidate.Source.Validate(); err != nil {
		return fmt.Errorf("contextcompile: BuildDataItem: candidate source: %w", err)
	}
	if err := kind.Validate(); err != nil {
		return fmt.Errorf("contextcompile: BuildDataItem: kind: %w", err)
	}

	switch candidate.Source {
	case SourceHeadTree:
		if kind != IncludedRepositoryFile {
			return fmt.Errorf("contextcompile: BuildDataItem: source %q requires kind %q, got %q", candidate.Source, IncludedRepositoryFile, kind)
		}
		if candidate.Path == "" || candidate.ID != pathID(candidate.Path) || candidate.Ref != "" {
			return fmt.Errorf("contextcompile: BuildDataItem: source %q candidate is not canonical path identity", candidate.Source)
		}
		if err := validateCandidatePath(candidate.Path); err != nil {
			return fmt.Errorf("contextcompile: BuildDataItem: %w", err)
		}
	case SourceStoreAuthority:
		switch kind {
		case IncludedAcceptedSpec, IncludedParentFeatureFragment, IncludedObligation, IncludedPolicyArtifact:
		default:
			return fmt.Errorf("contextcompile: BuildDataItem: source %q cannot carry kind %q", candidate.Source, kind)
		}
		if candidate.Ref == "" || candidate.ID != refID(candidate.Ref) || candidate.Path != "" {
			return fmt.Errorf("contextcompile: BuildDataItem: source %q candidate is not canonical ref identity", candidate.Source)
		}
		validateRef := validateArtifactRef
		if kind == IncludedPolicyArtifact {
			validateRef = validatePolicyArtifactRef
		}
		if err := validateRef("BuildDataItem candidate ref", candidate.Ref); err != nil {
			return err
		}
	case SourceDeclaredContext:
		if kind != IncludedDeclaredContextRef {
			return fmt.Errorf("contextcompile: BuildDataItem: source %q requires kind %q, got %q", candidate.Source, IncludedDeclaredContextRef, kind)
		}
		if candidate.Ref == "" || candidate.ID != refID(candidate.Ref) || candidate.Path != "" {
			return fmt.Errorf("contextcompile: BuildDataItem: source %q candidate is not canonical ref identity", candidate.Source)
		}
		if err := validateArtifactRef("BuildDataItem candidate ref", candidate.Ref); err != nil {
			return err
		}
	default:
		return fmt.Errorf("contextcompile: BuildDataItem: source %q cannot carry a data item", candidate.Source)
	}
	return nil
}
