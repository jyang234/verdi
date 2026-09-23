package unsealedprovenance

import (
	"errors"
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
)

// payloadDoc is the strict decode target. Every field except cutoff is
// required when the payload is present (SI-255), so a missing or null field
// must be distinguishable from its zero value:
//
//   - permitted and cap are captured as raw nodes and must carry exactly the
//     YAML bool / int tag. Decoding straight into *int would let yaml.v3
//     truncate `cap: 1.5` to 1, and *bool would admit YAML 1.1 spellings
//     such as `yes`; neither is an explicit value of the declared type.
//   - inventory elements are pointers, because yaml.v3 silently drops a null
//     element decoded into a struct slice.
//   - cutoff is a raw node so an explicit null is refused rather than read as
//     absence: the one spelling of "no cutoff recorded" is omitting the key.
type payloadDoc struct {
	Permitted artifact.RawNode      `yaml:"permitted"`
	Cap       artifact.RawNode      `yaml:"cap"`
	Inventory *[]*inventoryEntryDoc `yaml:"inventory"`
	Cutoff    artifact.RawNode      `yaml:"cutoff"`
}

type inventoryEntryDoc struct {
	ID         *string `yaml:"id"`
	Story      *string `yaml:"story"`
	AdmittedBy *string `yaml:"admitted_by"`
}

type cutoffDoc struct {
	Commit *string `yaml:"commit"`
}

const (
	yamlBoolTag = "!!bool"
	yamlIntTag  = "!!int"
	yamlNullTag = "!!null"
)

// DecodePayload strict-decodes one `unsealed-provenance` payload through the
// shared artifact seam (known fields only, restricted dialect) and validates
// it. Unknown keys at any level, a missing or null required field, a scalar
// of the wrong YAML type, and an explicit null cutoff are refused.
func DecodePayload(raw []byte) (*Payload, error) {
	var doc payloadDoc
	if err := artifact.DecodeStrict(raw, &doc); err != nil {
		return nil, fmt.Errorf("unsealedprovenance: decode payload: %w", err)
	}
	payload := &Payload{}
	if err := decodeRequiredScalar(&doc.Permitted, "permitted", yamlBoolTag, &payload.Permitted); err != nil {
		return nil, err
	}
	if err := decodeRequiredScalar(&doc.Cap, "cap", yamlIntTag, &payload.Cap); err != nil {
		return nil, err
	}
	if doc.Inventory == nil {
		return nil, errors.New("unsealedprovenance: inventory is missing or null")
	}
	payload.Inventory = make([]InventoryEntry, len(*doc.Inventory))
	for i, entry := range *doc.Inventory {
		if entry == nil || entry.ID == nil || entry.Story == nil || entry.AdmittedBy == nil {
			return nil, fmt.Errorf("unsealedprovenance: inventory[%d] requires id, story, and admitted_by", i)
		}
		payload.Inventory[i] = InventoryEntry{ID: *entry.ID, Story: *entry.Story, AdmittedBy: *entry.AdmittedBy}
	}
	cutoff, err := decodeCutoff(&doc.Cutoff)
	if err != nil {
		return nil, err
	}
	payload.Cutoff = cutoff
	if err := payload.Validate(); err != nil {
		return nil, err
	}
	return payload, nil
}

// decodeRequiredScalar decodes one required scalar whose resolved YAML tag
// must be exactly tag.
func decodeRequiredScalar(node *artifact.RawNode, field, tag string, out interface{}) error {
	if node.IsZero() || node.ShortTag() == yamlNullTag {
		return fmt.Errorf("unsealedprovenance: %s is missing or null", field)
	}
	if got := node.ShortTag(); got != tag {
		return fmt.Errorf("unsealedprovenance: %s must be a YAML %s scalar, got %s", field, tag, got)
	}
	raw, err := artifact.EncodeRawNode(node)
	if err != nil {
		return fmt.Errorf("unsealedprovenance: decode %s: %w", field, err)
	}
	if err := artifact.DecodeStrict(raw, out); err != nil {
		return fmt.Errorf("unsealedprovenance: decode %s: %w", field, err)
	}
	return nil
}

// decodeCutoff reads the optional cutoff node: absent is nil, an explicit
// null is refused, and anything else must strict-decode as {commit}.
func decodeCutoff(node *artifact.RawNode) (*Cutoff, error) {
	if node.IsZero() {
		return nil, nil
	}
	if node.ShortTag() == yamlNullTag {
		return nil, errors.New("unsealedprovenance: cutoff is null; omit the key when no cutoff is recorded")
	}
	raw, err := artifact.EncodeRawNode(node)
	if err != nil {
		return nil, fmt.Errorf("unsealedprovenance: decode cutoff: %w", err)
	}
	var doc cutoffDoc
	if err := artifact.DecodeStrict(raw, &doc); err != nil {
		return nil, fmt.Errorf("unsealedprovenance: decode cutoff: %w", err)
	}
	if doc.Commit == nil {
		return nil, errors.New("unsealedprovenance: cutoff.commit is missing or null")
	}
	return &Cutoff{Commit: *doc.Commit}, nil
}
