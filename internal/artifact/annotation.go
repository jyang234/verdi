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
	// AnnotationPin pins an existing artifact to a board as planning
	// material (02 §Record schemas, round-5.2 amendment; ledgered
	// R4-I-41): Target names the pinned artifact (a whole-artifact ref —
	// no selector, no fragment), Board carries its wall position, and
	// Body optionally says why. It never enters the spec document; its
	// graduation is drawing a typed edge to the pinned target — or it
	// dies, taking its untyped relates threads with it.
	AnnotationPin AnnotationType = "pin"
	// AnnotationRelates is the annotation layer's untyped scratch thread
	// between two elements (R4 concept §5, 02 §Record schemas): TargetB
	// names the thread's second endpoint and is present only for this type.
	AnnotationRelates AnnotationType = "relates"
	// AnnotationReview records a review sticky (02 §Record schemas). Its
	// canonical home is a forge MR inline comment, not this stream; a local
	// mirror carries in Body the same "[vd:<object-id>]" token the forge
	// comment carries.
	AnnotationReview AnnotationType = "review"
	// AnnotationStory and AnnotationSpike (round 5.4) are the scoping
	// canvas's typed proto-stickies (02 §Record schemas): a feature wall's
	// claim that a story (or spike) will exist. Board carries the parking
	// spot, Body the working title; their untyped relates-threads to
	// acceptance criteria (story) or open questions (spike) carry the
	// coverage/resolution attribution. Graduation mints the frontmatter
	// stub (a spike stub carrying resolves); legal only on feature-class
	// walls (fail closed elsewhere — enforced by internal/workbench, which
	// alone knows the wall's class).
	AnnotationStory AnnotationType = "story"
	AnnotationSpike AnnotationType = "spike"
)

var validAnnotationTypes = map[AnnotationType]bool{
	AnnotationComment:        true,
	AnnotationQuestion:       true,
	AnnotationDecisionNeeded: true,
	AnnotationAgentTask:      true,
	AnnotationPin:            true,
	AnnotationRelates:        true,
	AnnotationReview:         true,
	AnnotationStory:          true,
	AnnotationSpike:          true,
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
	ID      string           `json:"id"`
	TS      string           `json:"ts"`
	Author  string           `json:"author"`
	Target  *Target          `json:"target,omitempty"`
	TargetB *Target          `json:"target_b,omitempty"`
	Board   *BoardAnchor     `json:"board,omitempty"`
	Type    AnnotationType   `json:"type"`
	Body    string           `json:"body"`
	Status  AnnotationStatus `json:"status"`
}

// validateEndpointRef checks an annotation target/target_b ref resolves
// either as a pinned artifact ref (the general case, every annotation
// type) or, when allowAnnotationID is true (type relates only), as an
// "a-<ULID>" board-annotation id (02 §Record schemas, round 5.4: "a
// relates endpoint may name a board annotation by id ... as well as an
// artifact ref"). The annotation-id form is checked first since
// ParsePinnedRef would otherwise reject it outright (it is not artifact
// ref shaped at all).
func validateEndpointRef(ref string, allowAnnotationID bool) error {
	if allowAnnotationID && IsAnnotationID(ref) {
		return nil
	}
	if _, err := ParsePinnedRef(ref); err != nil {
		return err
	}
	return nil
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
		if err := validateEndpointRef(a.Target.Ref, a.Type == AnnotationRelates); err != nil {
			return fmt.Errorf("artifact: annotation target.ref: %w", err)
		}
	}
	if a.Board != nil && a.Board.Story == "" {
		return fmt.Errorf("artifact: annotation board.story is required when board is present")
	}
	if !validAnnotationTypes[a.Type] {
		return fmt.Errorf("artifact: annotation type %q is not a known type", a.Type)
	}
	// target_b is present only for type: relates — the untyped scratch
	// thread's second endpoint (02 §Record schemas).
	if a.Type == AnnotationRelates {
		if a.TargetB == nil {
			return fmt.Errorf("artifact: annotation type relates requires target_b")
		}
	} else if a.TargetB != nil {
		return fmt.Errorf("artifact: annotation target_b is present only for type relates, not %q", a.Type)
	}
	if a.TargetB != nil {
		// TargetB exists only for type relates (checked just above), so
		// the annotation-id endpoint form is always allowed here.
		if err := validateEndpointRef(a.TargetB.Ref, true); err != nil {
			return fmt.Errorf("artifact: annotation target_b.ref: %w", err)
		}
	}
	// Round 5.4's proto-stickies (story/spike): a board parking spot and a
	// non-empty body are required (body is enforced by the general rule
	// below, same as every non-pin type); no target/selector or target_b
	// of their own — they are pure board claims, closed the same way a
	// free-floating sticky of any other type is (fail closed, not
	// silently permitted).
	if a.Type == AnnotationStory || a.Type == AnnotationSpike {
		if a.Board == nil {
			return fmt.Errorf("artifact: annotation type %s requires a board position (02 §Record schemas)", a.Type)
		}
	}
	// A pin's closed shape (02 §Record schemas, round-5.2): the pinned
	// artifact AND a wall position are both required; the target is a
	// whole artifact (no selector, no fragment); body stays optional —
	// it only says why. Anything else fails closed.
	if a.Type == AnnotationPin {
		if a.Target == nil {
			return fmt.Errorf("artifact: annotation type pin requires a target (the pinned artifact)")
		}
		if a.Board == nil {
			return fmt.Errorf("artifact: annotation type pin requires a board position")
		}
		if a.Target.Selector != (Selector{}) {
			return fmt.Errorf("artifact: a pin's target takes no selector (it pins the whole artifact)")
		}
		r, err := ParsePinnedRef(a.Target.Ref)
		if err != nil {
			return fmt.Errorf("artifact: annotation target.ref: %w", err)
		}
		if r.Fragment() {
			return fmt.Errorf("artifact: a pin's target %q must name a whole artifact, not a fragment", a.Target.Ref)
		}
	}
	if a.Body == "" && a.Type != AnnotationPin {
		return fmt.Errorf("artifact: annotation body is required")
	}
	if !validAnnotationStatuses[a.Status] {
		return fmt.Errorf("artifact: annotation status %q is not a known status", a.Status)
	}
	return nil
}
