package specimport

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/provider"
)

// Target names the destination spec a Request populates
// (spec-import-contract.md, "Public operations and closed shapes").
type Target struct {
	Slug  string `json:"slug"`
	Class string `json:"class"`
	Title string `json:"title"`
	Story string `json:"story,omitempty"`
}

// Source is one explicitly selected, byte-preserved input document. Data is
// always the full bytes as read or uploaded; StartLine/EndLine (both zero
// meaning "the whole source") describe a sub-selection within Data that
// Normalize resolves — Source itself carries no digest field, so Data must
// stay whole for Normalize to report both an original-file digest and a
// selected-slice digest (see Normalize's doc comment).
type Source struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Data      []byte `json:"data"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
}

// Mapping is an explicit user correction or addition overriding (or
// supplying) one target's content or evidence
// (spec-import-contract.md, "Deterministic structural mapping"). Exactly
// one of three shapes applies (see validateMappings): evidence-only (no
// SourceID/Text/Transform/offsets, nonempty Evidence, an existing
// automatic ac- target), source-backed (SourceID plus Start/End/
// Transform, optional Text), or user-added (Text only, no SourceID).
type Mapping struct {
	Target    string   `json:"target"`
	SourceID  string   `json:"source_id,omitempty"`
	Start     int      `json:"start,omitempty"`
	End       int      `json:"end,omitempty"`
	Transform string   `json:"transform,omitempty"`
	Text      *string  `json:"text,omitempty"`
	Evidence  []string `json:"evidence,omitempty"`
}

// Link is an explicit user-added relationship, validated through the
// existing artifact.Link/LinkType closed vocabulary (never inferred from
// incidental URLs in source text).
type Link struct {
	Type string `json:"type"`
	Ref  string `json:"ref"`
}

// Request is the closed-shape wire contract for
// `verdi design import preview|apply --request`
// (spec-import-contract.md, "Public operations and closed shapes").
type Request struct {
	Schema          string    `json:"schema"`
	Target          Target    `json:"target"`
	Format          string    `json:"format"`
	Primary         string    `json:"primary"`
	Sources         []Source  `json:"sources"`
	Mappings        []Mapping `json:"mappings,omitempty"`
	Links           []Link    `json:"links,omitempty"`
	DeferStatements bool      `json:"defer_statements"`
	RetainUnmapped  bool      `json:"retain_unmapped"`
}

// RequestSchema is the one accepted Request.Schema value.
const RequestSchema = "verdi.spec-import-request/v1"

// Closed Format values (spec-import-contract.md: "Format is exactly
// native, markdown-v1, f13-reference-v1, or manual-v1; no silent
// fallback").
const (
	FormatNative       = "native"
	FormatMarkdownV1   = "markdown-v1"
	FormatF13Reference = "f13-reference-v1"
	FormatManualV1     = "manual-v1"
)

var validFormats = map[string]bool{
	FormatNative:       true,
	FormatMarkdownV1:   true,
	FormatF13Reference: true,
	FormatManualV1:     true,
}

// Closed Mapping.Transform values (spec-import-contract.md: "Transform is
// identity, trim-blank-lines, collapse-whitespace, or list-item").
const (
	TransformIdentity       = "identity"
	TransformTrimBlankLines = "trim-blank-lines"
	TransformCollapseWS     = "collapse-whitespace"
	TransformListItem       = "list-item"
)

var validTransforms = map[string]bool{
	TransformIdentity:       true,
	TransformTrimBlankLines: true,
	TransformCollapseWS:     true,
	TransformListItem:       true,
}

// Field.Origin values (spec-import-contract.md, "Deterministic structural
// mapping"): copied-source (automatic, or explicit with Text omitted/
// unchanged), user-edited-source (explicit mapping supplied different Text
// over a real selection), user-added (explicit mapping with no SourceID),
// native (native-mode strict-decoded content). "generated-deferral" is a
// Task 2 Compose-time origin, never produced by Normalize.
const (
	OriginCopiedSource  = "copied-source"
	OriginUserEditedSrc = "user-edited-source"
	OriginUserAdded     = "user-added"
	OriginNative        = "native"
)

// Coverage interval dispositions (spec-import-contract.md, "RetainUnmapped
// is the user's explicit disposition..."): every selected byte is exactly
// one of these three.
const (
	DispositionMapped     = "mapped"
	DispositionRetained   = "retained-only"
	DispositionUnresolved = "unresolved"
)

// sourceIDRe is the Source.ID/Mapping.SourceID grammar
// (spec-import-contract.md, "Source IDs use lowercase ASCII
// [a-z0-9][a-z0-9-]{0,63}, unique per request"). This is this contract's
// own literal grammar, not a copy of an internal/artifact private regex.
var sourceIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// mappingTargetIDRe recognizes the shape of a bare, prefixed object id
// (ac-1, co-1, dc-1, oq-1, or a preserved native multi-segment id like
// ac-primary-flow): one of the four known kind prefixes, a hyphen, then
// one or more kebab-case segments. It is independently derived from the
// contract's own prose ("valid ac-/co-/dc-/oq- object IDs") rather than
// reusing internal/artifact's private per-kind acIDRe/oqIDRe/objectIDRe
// (spec-import-contract.md's frozen-seam note: consume the object-id
// grammar through exported validators where they exist; none is exported
// for a bare object id, so this package owns its own narrow pattern
// covering exactly the four kinds import Mappings may target).
var mappingTargetIDRe = regexp.MustCompile(`^(?:ac|co|dc|oq)-[a-z0-9]+(?:-[a-z0-9]+)*$`)

// Size and count limits (spec-import-contract.md, "The entire JSON
// envelope is limited to 12 MiB. There are 1-32 sources, each nonempty
// valid UTF-8 and no more than 2 MiB, total supplied source bytes <=8 MiB").
const (
	MaxEnvelopeBytes    = 12 * 1024 * 1024
	MaxSourceBytes      = 2 * 1024 * 1024
	MaxTotalSourceBytes = 8 * 1024 * 1024
	MinSources          = 1
	MaxSources          = 32
	MaxLabelBytes       = 1024
)

// Validate owns every closed-shape, enum, required-field and limit check
// for a Request (spec-import-contract.md, "Public operations and closed
// shapes": "Request.Validate() error owns all closed-shape/enum/required-
// field/limit checks"). DecodeRequest calls it after decoding; Normalize
// calls it unconditionally on every input, including a Request built
// directly as a Go struct literal, so no caller — decoded or hand-built —
// can reach source-derived logic without passing the same gate.
//
// Validate deliberately stops at structural/shape checks that do not
// require interpreting a source's Markdown structure or resolving an
// automatically-extracted field: whether an explicit evidence-only Mapping
// names an EXISTING automatic AC target, for instance, can only be known
// once Normalize has attempted automatic extraction, so that check lives
// in Normalize's mapping-application step and still returns an error
// wrapping ErrInvalidRequest.
func (r Request) Validate() error {
	if r.Schema != RequestSchema {
		return fmt.Errorf("%w: schema %q must be %q", ErrInvalidRequest, r.Schema, RequestSchema)
	}
	if !validFormats[r.Format] {
		return fmt.Errorf("%w: format %q is not one of native, markdown-v1, f13-reference-v1, manual-v1", ErrUnsupportedFormat, r.Format)
	}
	if err := validateTarget(r.Target); err != nil {
		return err
	}
	if err := validateSources(r); err != nil {
		return err
	}
	sourceIDs := make(map[string]bool, len(r.Sources))
	for _, s := range r.Sources {
		sourceIDs[s.ID] = true
	}
	if err := validateMappings(r, sourceIDs); err != nil {
		return err
	}
	for i, l := range r.Links {
		al := artifact.Link{Type: artifact.LinkType(l.Type), Ref: l.Ref}
		if err := al.Validate(); err != nil {
			return fmt.Errorf("%w: links[%d]: %v", ErrInvalidRequest, i, err)
		}
	}
	if r.Format == FormatNative {
		if len(r.Mappings) != 0 {
			return fmt.Errorf("%w: native format refuses explicit mappings; edit the source or reupload", ErrInvalidRequest)
		}
		if len(r.Links) != 0 {
			return fmt.Errorf("%w: native format refuses explicit links; edit the source or reupload", ErrInvalidRequest)
		}
		if r.DeferStatements {
			return fmt.Errorf("%w: native format refuses statement deferral; edit the source or reupload", ErrInvalidRequest)
		}
	}

	canonical, err := canonjson.Marshal(r)
	if err != nil {
		return fmt.Errorf("%w: request is not canonical-JSON encodable: %v", ErrInvalidRequest, err)
	}
	if len(canonical) > MaxEnvelopeBytes {
		return fmt.Errorf("%w: canonical request is %d bytes, over the %d byte envelope limit", ErrInvalidRequest, len(canonical), MaxEnvelopeBytes)
	}
	return nil
}

func validateTarget(t Target) error {
	ref, err := artifact.ParseRef("spec/" + t.Slug)
	if err != nil {
		return fmt.Errorf("%w: target.slug %q: %v", ErrInvalidRequest, t.Slug, err)
	}
	// Target.Slug is a BARE spec name (spec-import-contract.md: "Target slug
	// follows the existing bare spec-name validator"; "Source.ID is a
	// validated identifier used as a trusted path component"). The shared
	// artifact validator also accepts "@commit" pins and "#object-id"
	// fragments, which are meaningful for a reference but not for the
	// trusted path component of .verdi/specs/active/<slug>/,
	// .verdi/imports/<slug>/ and refs/heads/design/<slug>; both are refused
	// here, on every direct entry path, before any path is constructed.
	if ref.Pinned() || ref.Fragment() {
		return fmt.Errorf("%w: target.slug %q must be a bare spec name, not a pinned (@commit) or fragment (#object-id) ref", ErrInvalidRequest, t.Slug)
	}
	if t.Class != string(artifact.ClassFeature) && t.Class != string(artifact.ClassStory) {
		return fmt.Errorf("%w: target.class %q must be feature or story", ErrInvalidRequest, t.Class)
	}
	if !nonBlankUTF8(t.Title) {
		return fmt.Errorf("%w: target.title must be a nonblank valid UTF-8 string", ErrInvalidRequest)
	}
	if t.Story != "" {
		if !utf8.ValidString(t.Story) {
			return fmt.Errorf("%w: target.story is not valid UTF-8", ErrInvalidRequest)
		}
		if _, _, err := provider.ParseStoryRef(provider.StoryRef(t.Story)); err != nil {
			return fmt.Errorf("%w: target.story %q: %v", ErrInvalidRequest, t.Story, err)
		}
	}
	return nil
}

func validateSources(r Request) error {
	if len(r.Sources) < MinSources || len(r.Sources) > MaxSources {
		return fmt.Errorf("%w: request has %d sources, want between %d and %d", ErrInvalidRequest, len(r.Sources), MinSources, MaxSources)
	}
	seenID := make(map[string]bool, len(r.Sources))
	totalBytes := 0
	primaryFound := false
	for i, s := range r.Sources {
		if !sourceIDRe.MatchString(s.ID) {
			return fmt.Errorf("%w: sources[%d].id %q must match %s", ErrInvalidRequest, i, s.ID, sourceIDRe.String())
		}
		if seenID[s.ID] {
			return fmt.Errorf("%w: sources[%d].id %q is duplicated", ErrInvalidRequest, i, s.ID)
		}
		seenID[s.ID] = true
		if s.ID == r.Primary {
			primaryFound = true
		}
		if err := validateSourceContent(s.Label, s.Data); err != nil {
			return fmt.Errorf("%w: sources[%d]: %v", ErrInvalidRequest, i, err)
		}
		totalBytes += len(s.Data)
		if _, err := selectLineRange(s.Data, s.StartLine, s.EndLine); err != nil {
			return fmt.Errorf("%w: sources[%d]: %v", ErrInvalidSource, i, err)
		}
	}
	if totalBytes > MaxTotalSourceBytes {
		return fmt.Errorf("%w: total supplied source bytes %d exceeds the %d byte limit", ErrInvalidRequest, totalBytes, MaxTotalSourceBytes)
	}
	if r.Primary == "" || !primaryFound {
		return fmt.Errorf("%w: primary %q must name exactly one of sources[].id", ErrInvalidRequest, r.Primary)
	}
	return nil
}

// validateSourceContent enforces one source's label and content
// constraints (spec-import-contract.md: "There are 1–32 sources, each
// nonempty valid UTF-8 and no more than 2 MiB"; "Labels are nonblank UTF-8
// display strings, at most 1024 bytes"). It is the shared seam both entry
// paths use: Request.Validate applies it to every decoded or hand-built
// source, and ReadSource applies the identical rules to the file it just
// read. It returns a bare error so each caller can wrap it with the
// operational code its own path reports — invalid-request for a supplied
// request, invalid-source for a file the reader itself selected.
func validateSourceContent(label string, data []byte) error {
	if !nonBlankUTF8(label) {
		return fmt.Errorf("label must be a nonblank valid UTF-8 string")
	}
	if len(label) > MaxLabelBytes {
		return fmt.Errorf("label is %d bytes, over the %d byte limit", len(label), MaxLabelBytes)
	}
	if len(data) == 0 {
		return fmt.Errorf("data must not be empty")
	}
	if !utf8.Valid(data) {
		return fmt.Errorf("data must be valid UTF-8")
	}
	if len(data) > MaxSourceBytes {
		return fmt.Errorf("data is %d bytes, over the %d byte per-source limit", len(data), MaxSourceBytes)
	}
	return nil
}

// validateMappings checks every Mapping matches exactly one of the three
// closed shapes the contract describes (spec-import-contract.md,
// "Deterministic structural mapping" / "Explicit text/span Mappings
// override..."):
//
//   - evidence-only: no SourceID/Text/Transform/zero offsets, nonempty
//     Evidence, target shaped like an acceptance criterion (ac-<id>).
//   - source-backed: SourceID present, Start/End a valid half-open span,
//     a known non-empty Transform, no Evidence (evidence is the
//     evidence-only shape's job, never layered onto a content mapping).
//   - user-added: no SourceID, nonblank Text, no Transform/offsets/
//     Evidence.
//
// Content-dependent rules this package cannot check without content —
// whether an evidence-only mapping's target was actually produced by
// automatic extraction, or whether a list-item span is really a
// recognized list item — are Normalize's job (see mapping.go).
func validateMappings(r Request, sourceIDs map[string]bool) error {
	seenTarget := make(map[string]bool, len(r.Mappings))
	for i, m := range r.Mappings {
		if m.Target != "problem" && m.Target != "outcome" && !mappingTargetIDRe.MatchString(m.Target) {
			return fmt.Errorf("%w: mappings[%d].target %q must be problem, outcome, or a valid ac-/co-/dc-/oq- object id", ErrInvalidRequest, i, m.Target)
		}
		if seenTarget[m.Target] {
			return fmt.Errorf("%w: mappings[%d].target %q is a duplicate explicit target", ErrInvalidRequest, i, m.Target)
		}
		seenTarget[m.Target] = true

		isObjectTarget := mappingTargetIDRe.MatchString(m.Target)
		isEvidenceOnlyShape := m.SourceID == "" && m.Text == nil && m.Transform == "" && m.Start == 0 && m.End == 0 && len(m.Evidence) > 0

		switch {
		case isEvidenceOnlyShape:
			if !strings.HasPrefix(m.Target, "ac-") {
				return fmt.Errorf("%w: mappings[%d] is evidence-only but target %q is not an acceptance criterion (ac-<id>); evidence applies to acceptance criteria only", ErrInvalidRequest, i, m.Target)
			}
			if err := validateEvidenceList(i, m.Evidence); err != nil {
				return err
			}

		case m.SourceID != "":
			if !sourceIDRe.MatchString(m.SourceID) {
				return fmt.Errorf("%w: mappings[%d].source_id %q must match %s", ErrInvalidRequest, i, m.SourceID, sourceIDRe.String())
			}
			if !sourceIDs[m.SourceID] {
				return fmt.Errorf("%w: mappings[%d].source_id %q does not name any of sources[].id", ErrInvalidRequest, i, m.SourceID)
			}
			if m.Start < 0 || m.End < m.Start {
				return fmt.Errorf("%w: mappings[%d] has an invalid half-open span [%d,%d)", ErrInvalidRequest, i, m.Start, m.End)
			}
			if !validTransforms[m.Transform] {
				return fmt.Errorf("%w: mappings[%d].transform %q must be identity, trim-blank-lines, collapse-whitespace, or list-item", ErrInvalidRequest, i, m.Transform)
			}
			if m.Transform == TransformListItem && !isObjectTarget {
				return fmt.Errorf("%w: mappings[%d] uses list-item transform for non-object target %q", ErrInvalidRequest, i, m.Target)
			}
			if len(m.Evidence) != 0 {
				return fmt.Errorf("%w: mappings[%d] is source-backed and must not also carry evidence; use a separate evidence-only mapping", ErrInvalidRequest, i)
			}

		case m.Text != nil:
			if !nonBlankUTF8(*m.Text) {
				return fmt.Errorf("%w: mappings[%d] has no source_id, so text must be nonblank", ErrInvalidRequest, i)
			}
			if m.Transform != "" || m.Start != 0 || m.End != 0 {
				return fmt.Errorf("%w: mappings[%d] has no source_id, so transform/start/end must be absent", ErrInvalidRequest, i)
			}
			if len(m.Evidence) != 0 {
				return fmt.Errorf("%w: mappings[%d] is user-added and must not also carry evidence; use a separate evidence-only mapping", ErrInvalidRequest, i)
			}

		default:
			return fmt.Errorf("%w: mappings[%d] must be source-backed (source_id), user-added (nonblank text), or evidence-only (an existing ac- target plus evidence)", ErrInvalidRequest, i)
		}
	}
	return nil
}

func validateEvidenceList(i int, evidence []string) error {
	seen := make(map[string]bool, len(evidence))
	for _, kind := range evidence {
		if !validEvidenceKind(kind) {
			return fmt.Errorf("%w: mappings[%d] evidence kind %q is not static/behavioral/runtime/attestation", ErrInvalidRequest, i, kind)
		}
		if seen[kind] {
			return fmt.Errorf("%w: mappings[%d] evidence kind %q is duplicated", ErrInvalidRequest, i, kind)
		}
		seen[kind] = true
	}
	return nil
}

func validEvidenceKind(kind string) bool {
	switch artifact.EvidenceKind(kind) {
	case artifact.EvidenceStatic, artifact.EvidenceBehavioral, artifact.EvidenceRuntime, artifact.EvidenceAttestation:
		return true
	default:
		return false
	}
}

// nonBlankUTF8 reports whether s is valid UTF-8 and has at least one
// non-whitespace byte once ASCII space/tab/newline/carriage-return
// padding is trimmed.
func nonBlankUTF8(s string) bool {
	if !utf8.ValidString(s) {
		return false
	}
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case ' ', '\t', '\n', '\r':
			continue
		default:
			return true
		}
	}
	return false
}
