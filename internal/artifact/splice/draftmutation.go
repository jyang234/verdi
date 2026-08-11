package splice

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/designprovenance"
)

// ApplyDraftMutations applies an ordered batch against exact spec bytes. Each
// operation is planned against the preceding operation's result; no partial
// result is observable when any operation or final validation fails.
func ApplyDraftMutations(src []byte, operations []designprovenance.Operation) ([]byte, error) {
	if len(operations) == 0 {
		// vocab:identity — ASD protocol operation name in a machinery diagnostic
		return nil, fmt.Errorf("splice: draft mutation requires at least one operation")
	}
	result := append([]byte(nil), src...)
	if err := Validate(result); err != nil {
		return nil, err
	}
	base, err := Parse(result)
	if err != nil {
		return nil, err
	}
	if err := base.validateDraftSets(); err != nil {
		return nil, err
	}
	for i, operation := range operations {
		if err := operation.Validate(); err != nil {
			return nil, fmt.Errorf("splice: operation[%d]: %w", i, err)
		}
		doc, err := Parse(result)
		if err != nil {
			return nil, fmt.Errorf("splice: operation[%d]: %w", i, err)
		}
		edits, err := doc.draftEdits(operation)
		if err != nil {
			return nil, fmt.Errorf("splice: operation[%d] %s: %w", i, operation.Op, err)
		}
		result, err = doc.Apply(edits)
		if err != nil {
			return nil, fmt.Errorf("splice: operation[%d] %s: %w", i, operation.Op, err)
		}
	}
	if err := Validate(result); err != nil {
		return nil, err
	}
	return result, nil
}

func (d *Doc) validateDraftSets() error {
	if context := mapGet(d.fm, "context"); context != nil {
		seen := map[string]bool{}
		for _, node := range context.Content {
			if seen[node.Value] {
				return fmt.Errorf("splice: duplicate context ref %q", node.Value)
			}
			seen[node.Value] = true
		}
	}
	if links := mapGet(d.fm, "links"); links != nil {
		if err := validateExactLinkSet("spec", links); err != nil {
			return err
		}
	}
	if decisions := mapGet(d.fm, "decisions"); decisions != nil {
		for _, decision := range decisions.Content {
			id := mapGet(decision, "id")
			links := mapGet(decision, "links")
			if id != nil && links != nil {
				if err := validateExactLinkSet(id.Value, links); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func validateExactLinkSet(source string, links *yaml.Node) error {
	type linkIdentity struct {
		typeValue string
		ref       string
		note      string
		hasNote   bool
	}
	seen := map[linkIdentity]bool{}
	for _, link := range links.Content {
		typeNode, refNode, noteNode := mapGet(link, "type"), mapGet(link, "ref"), mapGet(link, "note")
		if typeNode == nil || refNode == nil {
			continue
		}
		identity := linkIdentity{typeValue: typeNode.Value, ref: refNode.Value, hasNote: noteNode != nil}
		if noteNode != nil {
			identity.note = noteNode.Value
		}
		if seen[identity] {
			return fmt.Errorf("splice: duplicate exact link tuple on source %q", source)
		}
		seen[identity] = true
	}
	return nil
}

func (d *Doc) draftEdits(op designprovenance.Operation) ([]Edit, error) {
	switch op.Op {
	case designprovenance.OpSetProblem:
		return d.setAttribute("problem", op.Text, op.Anchor)
	case designprovenance.OpSetOutcome:
		return d.setAttribute("outcome", op.Text, op.Anchor)
	case designprovenance.OpAddAC, designprovenance.OpAddConstraint, designprovenance.OpAddDecision, designprovenance.OpAddQuestion:
		if err := requireObjectPrefix(op); err != nil {
			return nil, err
		}
		return d.addDraftObject(op)
	case designprovenance.OpEditAC, designprovenance.OpEditConstraint, designprovenance.OpEditDecision, designprovenance.OpEditQuestion:
		if err := requireObjectPrefix(op); err != nil {
			return nil, err
		}
		edit, err := d.replaceObject(op)
		return oneEdit(edit, err)
	case designprovenance.OpRemoveAC, designprovenance.OpRemoveConstraint, designprovenance.OpRemoveDecision, designprovenance.OpRemoveQuestion:
		if err := requireObjectPrefix(op); err != nil {
			return nil, err
		}
		edit, err := d.RemoveObjectEntry(op.ID)
		return oneEdit(edit, err)
	case designprovenance.OpReorderAC:
		if !strings.HasPrefix(op.ID, "ac-") || (op.AfterID != "" && !strings.HasPrefix(op.AfterID, "ac-")) {
			return nil, fmt.Errorf("acceptance criterion reorder requires ac- ids")
		}
		edit, err := d.reorderByField("acceptance_criteria", "id", op.ID, op.AfterID)
		return oneEdit(edit, err)
	case designprovenance.OpSetACEvidence:
		edit, err := d.setACEvidence(op.ID, op.Evidence)
		return oneEdit(edit, err)
	case designprovenance.OpAddLink:
		edit, err := d.addExactLink(op)
		return oneEdit(edit, err)
	case designprovenance.OpRemoveLink:
		edit, err := d.removeExactLink(op)
		return oneEdit(edit, err)
	case designprovenance.OpAddStub:
		edit, err := d.addDraftStub(op)
		return oneEdit(edit, err)
	case designprovenance.OpEditStub:
		edit, err := d.editDraftStub(op)
		return oneEdit(edit, err)
	case designprovenance.OpRemoveStub:
		edit, err := d.removeByField("stubs", "slug", op.Slug)
		return oneEdit(edit, err)
	case designprovenance.OpReorderStub:
		edit, err := d.reorderByField("stubs", "slug", op.Slug, op.AfterSlug)
		return oneEdit(edit, err)
	case designprovenance.OpAddContextRef:
		edit, err := d.addContextRef(op.Ref)
		return oneEdit(edit, err)
	case designprovenance.OpRemoveContextRef:
		edit, err := d.removeByField("context", "", op.Ref)
		return oneEdit(edit, err)
	default:
		// vocab:identity — ASD protocol operation name in a machinery diagnostic
		return nil, fmt.Errorf("unknown draft operation %q", op.Op)
	}
}

func oneEdit(edit Edit, err error) ([]Edit, error) {
	if err != nil {
		return nil, err
	}
	return []Edit{edit}, nil
}

func (d *Doc) setAttribute(key, text, anchor string) ([]Edit, error) {
	replacement := "{ text: " + quoteYAML(text) + ", anchor: " + quoteYAML(anchor) + " }"
	node := mapGet(d.fm, key)
	if node == nil {
		return []Edit{{Start: d.fmCloseOffset, End: d.fmCloseOffset, Replace: key + ": " + replacement + "\n"}}, nil
	}
	start, end, err := d.span(node)
	if err != nil {
		return nil, err
	}
	return []Edit{{Start: start, End: end, Replace: replacement}}, nil
}

func requireObjectPrefix(op designprovenance.Operation) error {
	want := ""
	switch op.Op {
	case designprovenance.OpAddAC, designprovenance.OpEditAC, designprovenance.OpRemoveAC:
		want = "ac-"
	case designprovenance.OpAddConstraint, designprovenance.OpEditConstraint, designprovenance.OpRemoveConstraint:
		want = "co-"
	case designprovenance.OpAddDecision, designprovenance.OpEditDecision, designprovenance.OpRemoveDecision:
		want = "dc-"
	case designprovenance.OpAddQuestion, designprovenance.OpEditQuestion, designprovenance.OpRemoveQuestion:
		want = "oq-"
	}
	if !strings.HasPrefix(op.ID, want) {
		return fmt.Errorf("operation %s requires a %s object id", op.Op, want)
	}
	return nil
}

func (d *Doc) addDraftObject(op designprovenance.Operation) ([]Edit, error) {
	block, err := blockForID(op.ID)
	if err != nil {
		return nil, err
	}
	if seq := mapGet(d.fm, block); seq != nil && seqFindByID(seq, op.ID) != nil {
		return nil, fmt.Errorf("duplicate object %q", op.ID)
	}
	entry := formatDraftObject(op)
	seq := mapGet(d.fm, block)
	var fmEdit Edit
	switch {
	case seq == nil:
		fmEdit = Edit{Start: d.fmCloseOffset, End: d.fmCloseOffset, Replace: block + ":\n  - " + entry + "\n"}
	case seq.Style&yaml.FlowStyle != 0:
		fmEdit, err = d.appendToFlowSeq(seq, entry)
	default:
		fmEdit, err = d.appendToBlockSeq(seq, entry)
	}
	if err != nil {
		return nil, err
	}
	heading := strings.TrimPrefix(op.Anchor, "#")
	body := Edit{Start: len(d.src), End: len(d.src), Replace: "\n## " + heading + "\n\n" + op.Text + "\n"}
	return []Edit{fmEdit, body}, nil
}

func (d *Doc) replaceObject(op designprovenance.Operation) (Edit, error) {
	elem, err := d.objectElem(op.ID)
	if err != nil {
		return Edit{}, err
	}
	start, end, err := d.span(elem)
	if err != nil {
		return Edit{}, err
	}
	return Edit{Start: start, End: end, Replace: formatDraftObject(op)}, nil
}

func formatDraftObject(op designprovenance.Operation) string {
	s := "{ id: " + op.ID + ", text: " + quoteYAML(op.Text)
	if op.Op == designprovenance.OpAddAC || op.Op == designprovenance.OpEditAC {
		values := make([]string, len(op.Evidence))
		for i, value := range op.Evidence {
			values[i] = string(value)
		}
		s += ", evidence: [" + strings.Join(values, ", ") + "]"
	}
	return s + ", anchor: " + quoteYAML(op.Anchor) + " }"
}

func (d *Doc) setACEvidence(id string, evidence []artifact.EvidenceKind) (Edit, error) {
	if !strings.HasPrefix(id, "ac-") {
		return Edit{}, fmt.Errorf("set-ac-evidence requires an ac- id")
	}
	elem, err := d.objectElem(id)
	if err != nil {
		return Edit{}, err
	}
	node := mapGet(elem, "evidence")
	if node == nil {
		return Edit{}, fmt.Errorf("acceptance criterion %q has no evidence field", id)
	}
	start, end, err := d.span(node)
	if err != nil {
		return Edit{}, err
	}
	values := make([]string, len(evidence))
	for i, value := range evidence {
		values[i] = string(value)
	}
	return Edit{Start: start, End: end, Replace: "[" + strings.Join(values, ", ") + "]"}, nil
}

func (d *Doc) sequenceForSource(source string) (*yaml.Node, *yaml.Node, error) {
	switch {
	case source == "spec":
		key, seq := mapKeyValue(d.fm, "links")
		return key, seq, nil
	case strings.HasPrefix(source, "dc-"):
		elem, err := d.objectElem(source)
		if err != nil {
			return nil, nil, err
		}
		key, seq := mapKeyValue(elem, "links")
		return key, seq, nil
	default:
		return nil, nil, fmt.Errorf("link source %q must be spec or a declared decision id", source)
	}
}

func exactLinkMatch(node *yaml.Node, op designprovenance.Operation) bool {
	typeNode, refNode, noteNode := mapGet(node, "type"), mapGet(node, "ref"), mapGet(node, "note")
	if typeNode == nil || refNode == nil || typeNode.Value != string(op.Type) || refNode.Value != op.Ref {
		return false
	}
	if op.Note == "" {
		return noteNode == nil
	}
	return noteNode != nil && noteNode.Value == op.Note
}

func (d *Doc) addExactLink(op designprovenance.Operation) (Edit, error) {
	link := artifact.Link{Type: op.Type, Ref: op.Ref, Note: op.Note}
	if err := link.Validate(); err != nil {
		return Edit{}, err
	}
	_, seq, err := d.sequenceForSource(op.Source)
	if err != nil {
		return Edit{}, err
	}
	if seq != nil {
		for _, node := range seq.Content {
			if exactLinkMatch(node, op) {
				return Edit{}, fmt.Errorf("duplicate exact link tuple")
			}
		}
		if seq.Style&yaml.FlowStyle != 0 {
			return d.appendToFlowSeq(seq, formatLink(link))
		}
		return d.appendToBlockSeq(seq, formatLink(link))
	}
	if op.Source == "spec" {
		return Edit{Start: d.fmCloseOffset, End: d.fmCloseOffset, Replace: "links:\n  - " + formatLink(link) + "\n"}, nil
	}
	return d.AppendDecisionLink(op.Source, link)
}

func (d *Doc) removeExactLink(op designprovenance.Operation) (Edit, error) {
	key, seq, err := d.sequenceForSource(op.Source)
	if err != nil {
		return Edit{}, err
	}
	if seq == nil {
		return Edit{}, fmt.Errorf("source %q has no links; exact tuple not found", op.Source)
	}
	idx := -1
	for i, node := range seq.Content {
		if exactLinkMatch(node, op) {
			idx = i
			break
		}
	}
	if idx < 0 {
		return Edit{}, fmt.Errorf("source %q has no exact link tuple", op.Source)
	}
	return d.removeSequenceElement(key, seq, idx, "links")
}

func formatDraftStub(op designprovenance.Operation) string {
	if op.Spike != nil && *op.Spike {
		return "{ slug: " + op.Slug + ", spike: true, resolves: [" + strings.Join(op.Resolves, ", ") + "] }"
	}
	return "{ slug: " + op.Slug + ", acceptance_criteria: [" + strings.Join(op.AcceptanceCriteria, ", ") + "] }"
}

func (d *Doc) addDraftStub(op designprovenance.Operation) (Edit, error) {
	seq := mapGet(d.fm, "stubs")
	if seq != nil && sequenceIndex(seq, "slug", op.Slug) >= 0 {
		return Edit{}, fmt.Errorf("duplicate stub %q", op.Slug)
	}
	return d.appendStubEntry(formatDraftStub(op))
}

func (d *Doc) editDraftStub(op designprovenance.Operation) (Edit, error) {
	seq := mapGet(d.fm, "stubs")
	if seq == nil {
		return Edit{}, fmt.Errorf("spec has no stubs block")
	}
	idx := sequenceIndex(seq, "slug", op.Slug)
	if idx < 0 {
		return Edit{}, fmt.Errorf("no stub %q", op.Slug)
	}
	start, end, err := d.span(seq.Content[idx])
	if err != nil {
		return Edit{}, err
	}
	return Edit{Start: start, End: end, Replace: formatDraftStub(op)}, nil
}

func (d *Doc) addContextRef(ref string) (Edit, error) {
	if _, err := artifact.ParsePinnedRef(ref); err != nil {
		return Edit{}, err
	}
	seq := mapGet(d.fm, "context")
	if seq == nil {
		return Edit{Start: d.fmCloseOffset, End: d.fmCloseOffset, Replace: "context:\n  - " + ref + "\n"}, nil
	}
	if sequenceIndex(seq, "", ref) >= 0 {
		return Edit{}, fmt.Errorf("duplicate context ref %q", ref)
	}
	if seq.Style&yaml.FlowStyle != 0 {
		return d.appendToFlowSeq(seq, ref)
	}
	return d.appendToBlockSeq(seq, ref)
}

func sequenceIndex(seq *yaml.Node, field, value string) int {
	if seq == nil || seq.Kind != yaml.SequenceNode {
		return -1
	}
	for i, node := range seq.Content {
		candidate := node
		if field != "" {
			candidate = mapGet(node, field)
		}
		if candidate != nil && candidate.Value == value {
			return i
		}
	}
	return -1
}

func (d *Doc) removeByField(block, field, value string) (Edit, error) {
	key, seq := mapKeyValue(d.fm, block)
	if seq == nil {
		return Edit{}, fmt.Errorf("spec has no %s block", block)
	}
	idx := sequenceIndex(seq, field, value)
	if idx < 0 {
		return Edit{}, fmt.Errorf("no %s target %q", block, value)
	}
	return d.removeSequenceElement(key, seq, idx, block)
}

func (d *Doc) removeSequenceElement(key, seq *yaml.Node, idx int, block string) (Edit, error) {
	if seq.Kind != yaml.SequenceNode {
		return Edit{}, fmt.Errorf("%s is not a sequence", block)
	}
	if len(seq.Content) == 1 {
		if seq.Style&yaml.FlowStyle != 0 {
			start, end, err := d.span(seq)
			if err != nil {
				return Edit{}, err
			}
			return Edit{Start: start, End: end, Replace: "[]"}, nil
		}
		return d.removeWholeLineSpan(key, seq.Content[0], block)
	}
	elemStart, elemEnd, err := d.span(seq.Content[idx])
	if err != nil {
		return Edit{}, err
	}
	if seq.Style&yaml.FlowStyle != 0 {
		if idx > 0 {
			_, previousEnd, err := d.span(seq.Content[idx-1])
			if err != nil {
				return Edit{}, err
			}
			return Edit{Start: previousEnd, End: elemEnd}, nil
		}
		nextStart, _, err := d.span(seq.Content[1])
		if err != nil {
			return Edit{}, err
		}
		return Edit{Start: elemStart, End: nextStart}, nil
	}
	lineStart, err := byteOffset(d.offsets, seq.Content[idx].Line+d.lineDelta, 1)
	if err != nil {
		return Edit{}, err
	}
	end, err := extendThroughNewline(d.src, elemEnd)
	if err != nil {
		return Edit{}, err
	}
	return Edit{Start: lineStart, End: end}, nil
}

func (d *Doc) reorderByField(block, field, value, after string) (Edit, error) {
	if value == after && after != "" {
		return Edit{}, fmt.Errorf("target %q cannot be reordered after itself", value)
	}
	_, seq := mapKeyValue(d.fm, block)
	if seq == nil {
		return Edit{}, fmt.Errorf("spec has no %s block", block)
	}
	from := sequenceIndex(seq, field, value)
	if from < 0 {
		return Edit{}, fmt.Errorf("no %s target %q", block, value)
	}
	if after != "" && sequenceIndex(seq, field, after) < 0 {
		return Edit{}, fmt.Errorf("no %s after target %q", block, after)
	}
	order := make([]int, 0, len(seq.Content))
	for i := range seq.Content {
		if i != from {
			order = append(order, i)
		}
	}
	insert := 0
	if after != "" {
		for i, original := range order {
			candidate := seq.Content[original]
			if mapGet(candidate, field).Value == after {
				insert = i + 1
				break
			}
		}
	}
	order = append(order, 0)
	copy(order[insert+1:], order[insert:])
	order[insert] = from
	return d.rewriteSequence(seq, order)
}

func (d *Doc) rewriteSequence(seq *yaml.Node, order []int) (Edit, error) {
	values := make([]string, len(order))
	for i, idx := range order {
		start, end, err := d.span(seq.Content[idx])
		if err != nil {
			return Edit{}, err
		}
		values[i] = string(d.src[start:end])
	}
	if seq.Style&yaml.FlowStyle != 0 {
		start, end, err := d.span(seq)
		if err != nil {
			return Edit{}, err
		}
		return Edit{Start: start, End: end, Replace: "[ " + strings.Join(values, ", ") + " ]"}, nil
	}
	first := seq.Content[0]
	firstStart, err := byteOffset(d.offsets, first.Line+d.lineDelta, 1)
	if err != nil {
		return Edit{}, err
	}
	valueStart, _, err := d.span(first)
	if err != nil {
		return Edit{}, err
	}
	prefix := string(d.src[firstStart:valueStart])
	if strings.TrimLeft(prefix, " \t") != "- " {
		return Edit{}, fmt.Errorf("block sequence does not use the proven line shape")
	}
	_, lastEnd, err := d.span(seq.Content[len(seq.Content)-1])
	if err != nil {
		return Edit{}, err
	}
	end, err := extendThroughNewline(d.src, lastEnd)
	if err != nil {
		return Edit{}, err
	}
	return Edit{Start: firstStart, End: end, Replace: prefix + strings.Join(values, "\n"+prefix) + "\n"}, nil
}
