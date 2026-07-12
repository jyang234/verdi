package splice

// Stub-authoring ops (spec/scoping-canvas ac-2/ac-5): AppendStub and
// AppendSpikeStub append one entry to a feature spec's `stubs:` block —
// the board's stub-graduate action's write path — mirroring AppendObject's
// exact block-style house shape (ops.go) rather than reinventing a new
// insertion strategy: `stubs:` is a fifth top-level block, not itself one
// of the four id-prefixed object blocks AppendObject targets, but the
// same three insertion shapes apply (absent block, block-style sequence,
// flow-style sequence), so this file reuses appendToFlowSeq/
// appendToBlockSeq verbatim and fails closed exactly where those already
// do (house style only).

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// AppendStub appends a plain stub entry (`{ slug, acceptance_criteria }`)
// to the feature spec's `stubs:` block, creating the block if absent (the
// first-stub case mirrors AppendObject's first-block insertion, itself
// mirroring AppendDecisionLink's first-yarn pattern: a whole new
// "stubs:\n  - {...}\n" block inserted immediately before the closing
// frontmatter delimiter).
func (d *Doc) AppendStub(slug string, acIDs []string) (Edit, error) {
	if slug == "" {
		return Edit{}, fmt.Errorf("splice: AppendStub requires a non-empty slug")
	}
	if len(acIDs) == 0 {
		return Edit{}, fmt.Errorf("splice: AppendStub requires at least one acceptance criterion id")
	}
	entry := "{ slug: " + slug + ", acceptance_criteria: [" + strings.Join(acIDs, ", ") + "] }"
	return d.appendStubEntry(entry)
}

// AppendSpikeStub appends a spike stub entry (`{ slug, spike: true,
// resolves }`) to the feature spec's `stubs:` block, creating the block if
// absent — the DC-4 flag-discriminated sibling of AppendStub, same
// insertion machinery.
func (d *Doc) AppendSpikeStub(slug string, oqIDs []string) (Edit, error) {
	if slug == "" {
		return Edit{}, fmt.Errorf("splice: AppendSpikeStub requires a non-empty slug")
	}
	if len(oqIDs) == 0 {
		return Edit{}, fmt.Errorf("splice: AppendSpikeStub requires at least one open-question id")
	}
	entry := "{ slug: " + slug + ", spike: true, resolves: [" + strings.Join(oqIDs, ", ") + "] }"
	return d.appendStubEntry(entry)
}

// appendStubEntry inserts entry into the stubs: block, handling all three
// insertion shapes AppendObject already proves: no stubs: key at all (the
// common first-stub case), an existing flow-style sequence, and an
// existing block-style sequence (the house style the scaffold and every
// hand-authored stub in this store use). Anything else — a non-sequence
// stubs: value, or a block-style sequence whose elements are not "- "
// prefixed flow maps — fails closed via appendToBlockSeq/appendToFlowSeq's
// own existing checks (house style only).
func (d *Doc) appendStubEntry(entry string) (Edit, error) {
	seq := mapGet(d.fm, "stubs")
	if seq == nil {
		return Edit{Start: d.fmCloseOffset, End: d.fmCloseOffset, Replace: "stubs:\n  - " + entry + "\n"}, nil
	}
	if seq.Kind != yaml.SequenceNode {
		return Edit{}, fmt.Errorf("splice: stubs is not a sequence; fail closed")
	}
	if seq.Style&yaml.FlowStyle != 0 {
		return d.appendToFlowSeq(seq, entry)
	}
	return d.appendToBlockSeq(seq, entry)
}
