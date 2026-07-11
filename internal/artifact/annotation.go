package artifact

import (
	"fmt"
	"time"
)

// AnnotationType is the `type` field of an annotation record
// (02 §Record schemas).
type AnnotationType string

const (
	AnnotationComment        AnnotationType = "comment"
	AnnotationQuestion       AnnotationType = "question"
	AnnotationDecisionNeeded AnnotationType = "decision-needed"
	AnnotationAgentTask      AnnotationType = "agent-task"
)

var validAnnotationTypes = map[AnnotationType]bool{
	AnnotationComment:        true,
	AnnotationQuestion:       true,
	AnnotationDecisionNeeded: true,
	AnnotationAgentTask:      true,
}

// AnnotationStatus is the `status` field of an annotation record.
type AnnotationStatus string

const (
	AnnotationOpen      AnnotationStatus = "open"
	AnnotationResolved  AnnotationStatus = "resolved"
	AnnotationGraduated AnnotationStatus = "graduated"
)

var validAnnotationStatuses = map[AnnotationStatus]bool{
	AnnotationOpen:      true,
	AnnotationResolved:  true,
	AnnotationGraduated: true,
}

// Selector anchors an annotation's target inside an artifact's rendered
// body (02 §Record schemas: "target.selector"). Line is a pointer so a
// present-but-null JSON value round-trips distinctly from an absent field.
type Selector struct {
	Heading string `json:"heading"`
	Quote   string `json:"quote"`
	Line    *int   `json:"line"`
}

// Target is an annotation's pinned artifact anchor.
type Target struct {
	Ref      string   `json:"ref"`
	Selector Selector `json:"selector"`
}

// BoardAnchor is an annotation's position on a story's murder board.
type BoardAnchor struct {
	Story string  `json:"story"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
}

// Annotation is schema verdi.annotation/v1 (02 §Record schemas), stored
// append-only in data/mutable/annotations/<kind>--<name>.jsonl. It has no
// `schema` field of its own — the literal example in 02 omits one, unlike
// the other record schemas.
type Annotation struct {
	ID     string           `json:"id"`
	TS     string           `json:"ts"`
	Author string           `json:"author"`
	Target *Target          `json:"target,omitempty"`
	Board  *BoardAnchor     `json:"board,omitempty"`
	Type   AnnotationType   `json:"type"`
	Body   string           `json:"body"`
	Status AnnotationStatus `json:"status"`
}

// DecodeAnnotation strict-decodes and validates a single annotation
// record (one line of a JSONL file).
func DecodeAnnotation(data []byte) (*Annotation, error) {
	var a Annotation
	if err := DecodeStrictJSON(data, &a); err != nil {
		return nil, err
	}
	if err := a.Validate(); err != nil {
		return nil, err
	}
	return &a, nil
}

// Validate checks id shape (a-<ULID>, I-11), ts is RFC3339, author is
// present, at least one of target|board is present (02 §Record schemas),
// target.ref (if present) is a pinned ref, type and status are known
// enums, and body is non-empty.
func (a Annotation) Validate() error {
	if !annotationIDRe.MatchString(a.ID) {
		return fmt.Errorf("artifact: annotation id %q is not a valid a-<ULID> (I-11)", a.ID)
	}
	if _, err := time.Parse(time.RFC3339, a.TS); err != nil {
		return fmt.Errorf("artifact: annotation ts %q is not RFC3339: %w", a.TS, err)
	}
	if a.Author == "" {
		return fmt.Errorf("artifact: annotation author is required")
	}
	if a.Target == nil && a.Board == nil {
		return fmt.Errorf("artifact: annotation must carry target, board, or both (02 §Record schemas)")
	}
	if a.Target != nil {
		if _, err := ParsePinnedRef(a.Target.Ref); err != nil {
			return fmt.Errorf("artifact: annotation target.ref: %w", err)
		}
	}
	if a.Board != nil && a.Board.Story == "" {
		return fmt.Errorf("artifact: annotation board.story is required when board is present")
	}
	if !validAnnotationTypes[a.Type] {
		return fmt.Errorf("artifact: annotation type %q is not a known type", a.Type)
	}
	if a.Body == "" {
		return fmt.Errorf("artifact: annotation body is required")
	}
	if !validAnnotationStatuses[a.Status] {
		return fmt.Errorf("artifact: annotation status %q is not a known status", a.Status)
	}
	return nil
}
