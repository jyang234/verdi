// Package matrixprojection defines and assembles the canonical matrix record
// shared by the CLI text, CLI JSON, and MCP adapters.
package matrixprojection

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/evidence"
)

// SchemaID is the only accepted matrix record schema.
const SchemaID = "verdi.matrix/v1"

// Class is the closed matrix union discriminator.
type Class string

const (
	ClassStory   Class = "story"
	ClassFeature Class = "feature"
)

// EffectiveState is the closed Git-derived lifecycle-state vocabulary.
type EffectiveState string

const (
	StateProposed             EffectiveState = "proposed"
	StateAcceptedPendingBuild EffectiveState = "accepted-pending-build"
	StateSuperseded           EffectiveState = "superseded"
	StateClosed               EffectiveState = "closed"
	StateUnproven             EffectiveState = "unproven"
)

// AttestationState includes the fold's three states and the explicit value
// used by non-attestation projections.
type AttestationState string

const (
	AttestationNotApplicable AttestationState = "not-applicable"
	AttestationAbsent        AttestationState = "absent"
	AttestationUnauthored    AttestationState = "unauthored"
	AttestationAuthored      AttestationState = "authored"
)

// Record is the exact class-tagged verdi.matrix/v1 union.
type Record struct {
	Schema   string       `json:"schema"`
	Target   Target       `json:"target"`
	Preview  bool         `json:"preview"`
	Violated bool         `json:"violated"`
	Story    *StoryBody   `json:"story,omitempty"`
	Feature  *FeatureBody `json:"feature,omitempty"`
}

// Target identifies the projected story or feature and its effective state.
type Target struct {
	Class          Class          `json:"class"`
	SpecRef        string         `json:"spec_ref"`
	EffectiveState EffectiveState `json:"effective_state"`
}

// StoryBody is the story arm of Record.
type StoryBody struct {
	StoryRef string    `json:"story_ref"`
	Eligible bool      `json:"eligible"`
	ACs      []StoryAC `json:"acs"`
}

// StoryAC is one native story-fold acceptance-criterion projection.
type StoryAC struct {
	ID      string           `json:"id"`
	Text    string           `json:"text"`
	Status  string           `json:"status"`
	Summary string           `json:"summary"`
	Kinds   []KindProjection `json:"kinds"`
}

// KindProjection is one native story-fold evidence-kind projection.
type KindProjection struct {
	Kind              string                      `json:"kind"`
	Satisfied         bool                        `json:"satisfied"`
	AttestationState  AttestationState            `json:"attestation_state"`
	ViolatingWitness  string                      `json:"violating_witness"`
	ObligationQuality ObligationQualityProjection `json:"obligation_quality"`
}

// ObligationQualityProjection preserves the fold's quality assessment.
type ObligationQualityProjection struct {
	StructuralState string `json:"structural_state"`
	MatchState      string `json:"match_state"`
	Reason          string `json:"reason"`
	WitnessPath     string `json:"witness_path"`
}

// FeatureBody is the feature arm of Record.
type FeatureBody struct {
	ACs []FeatureAC `json:"acs"`
}

// FeatureAC is one native feature-fold acceptance-criterion projection.
type FeatureAC struct {
	ID                  string                 `json:"id"`
	Text                string                 `json:"text"`
	Status              string                 `json:"status"`
	Summary             string                 `json:"summary"`
	ImplementingStories []string               `json:"implementing_stories"`
	OutcomeFloor        OutcomeFloorProjection `json:"outcome_floor"`
}

// OutcomeFloorProjection preserves the feature fold's OR-across-signals
// outcome-floor result.
type OutcomeFloorProjection struct {
	Satisfied           bool             `json:"satisfied"`
	DeclaresAttestation bool             `json:"declares_attestation"`
	AttestationState    AttestationState `json:"attestation_state"`
	ViolatingWitness    string           `json:"violating_witness"`
}

// NewStory maps a native story fold to the story arm without reordering or
// re-deriving any fold fact.
func NewStory(target Target, preview bool, result evidence.StoryResult) (Record, error) {
	if target.Class != ClassStory {
		return Record{}, fmt.Errorf("matrix projection: tagged body requires target class %q, got %q", ClassStory, target.Class)
	}
	if result.SpecRef != target.SpecRef {
		return Record{}, fmt.Errorf("matrix projection: source fold spec_ref %q does not match target %q", result.SpecRef, target.SpecRef)
	}

	acs := make([]StoryAC, 0, len(result.ACs))
	for _, ac := range result.ACs {
		kinds := make([]KindProjection, 0, len(ac.Kinds))
		for _, kind := range ac.Kinds {
			attestation := AttestationNotApplicable
			if kind.Kind == artifact.EvidenceAttestation {
				var err error
				attestation, err = projectAttestationState(kind.Attestation)
				if err != nil {
					return Record{}, fmt.Errorf("matrix projection: AC %s kind %s: %w", ac.ID, kind.Kind, err)
				}
			}
			kinds = append(kinds, KindProjection{
				Kind:             string(kind.Kind),
				Satisfied:        kind.Satisfied,
				AttestationState: attestation,
				ViolatingWitness: evidenceWitness(kind.Violating),
				ObligationQuality: ObligationQualityProjection{
					StructuralState: string(kind.ObligationQuality.StructuralState),
					MatchState:      string(kind.ObligationQuality.MatchState),
					Reason:          string(kind.ObligationQuality.Reason),
					WitnessPath:     kind.ObligationQuality.WitnessPath,
				},
			})
		}
		acs = append(acs, StoryAC{ID: ac.ID, Text: ac.Text, Status: string(ac.Status), Summary: ac.Summary, Kinds: kinds})
	}

	record := Record{
		Schema:   SchemaID,
		Target:   target,
		Preview:  preview,
		Violated: result.Violated,
		Story:    &StoryBody{StoryRef: result.Story, Eligible: result.Eligible, ACs: acs},
	}
	if err := record.Validate(); err != nil {
		return Record{}, fmt.Errorf("matrix projection: assembling target record: %w", err)
	}
	return record, nil
}

// NewFeature maps a native feature fold to the feature arm without reordering
// or re-deriving any fold fact.
func NewFeature(target Target, preview bool, result evidence.FeatureResult) (Record, error) {
	if target.Class != ClassFeature {
		return Record{}, fmt.Errorf("matrix projection: tagged body requires target class %q, got %q", ClassFeature, target.Class)
	}
	if result.SpecRef != target.SpecRef {
		return Record{}, fmt.Errorf("matrix projection: source fold spec_ref %q does not match target %q", result.SpecRef, target.SpecRef)
	}

	acs := make([]FeatureAC, 0, len(result.ACs))
	for _, ac := range result.ACs {
		stories := append([]string(nil), ac.ImplementingStories...)
		if stories == nil {
			stories = []string{}
		}
		attestation := AttestationNotApplicable
		if ac.Floor.DeclaresAttestation {
			var err error
			attestation, err = projectAttestationState(ac.Floor.Attestation)
			if err != nil {
				return Record{}, fmt.Errorf("matrix projection: AC %s outcome floor: %w", ac.ID, err)
			}
		}
		acs = append(acs, FeatureAC{
			ID:                  ac.ID,
			Text:                ac.Text,
			Status:              string(ac.Status),
			Summary:             ac.Summary,
			ImplementingStories: stories,
			OutcomeFloor: OutcomeFloorProjection{
				Satisfied:           ac.Floor.Satisfied,
				DeclaresAttestation: ac.Floor.DeclaresAttestation,
				AttestationState:    attestation,
				ViolatingWitness:    evidenceWitness(ac.Floor.Violating),
			},
		})
	}

	record := Record{
		Schema:   SchemaID,
		Target:   target,
		Preview:  preview,
		Violated: result.Violated,
		Feature:  &FeatureBody{ACs: acs},
	}
	if err := record.Validate(); err != nil {
		return Record{}, fmt.Errorf("matrix projection: assembling target record: %w", err)
	}
	return record, nil
}

// Marshal validates and canonically encodes a matrix record.
func Marshal(record Record) ([]byte, error) {
	if err := record.Validate(); err != nil {
		return nil, fmt.Errorf("matrix projection: marshal: %w", err)
	}
	out, err := canonjson.Marshal(record)
	if err != nil {
		return nil, fmt.Errorf("matrix projection: marshal: %w", err)
	}
	return out, nil
}

// Decode strictly decodes and validates one matrix record.
func Decode(data []byte) (Record, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var record Record
	if err := dec.Decode(&record); err != nil {
		return Record{}, fmt.Errorf("matrix projection: decode: %w", err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		if err == nil {
			return Record{}, fmt.Errorf("matrix projection: decode: trailing data after top-level value")
		}
		return Record{}, fmt.Errorf("matrix projection: decode: trailing data: %w", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return Record{}, fmt.Errorf("matrix projection: decode arm presence: %w", err)
	}
	for _, arm := range []string{"story", "feature"} {
		if value, ok := raw[arm]; ok && bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return Record{}, fmt.Errorf("matrix projection: decode: %s must be absent rather than null", arm)
		}
	}
	if err := validateRequiredFieldPresence(raw); err != nil {
		return Record{}, fmt.Errorf("matrix projection: decode: %w", err)
	}

	if err := record.Validate(); err != nil {
		return Record{}, fmt.Errorf("matrix projection: decode: %w", err)
	}
	return record, nil
}

func validateRequiredFieldPresence(root map[string]json.RawMessage) error {
	if err := requireJSONFields(root, "record", "schema", "target", "preview", "violated"); err != nil {
		return err
	}
	target, err := decodeJSONObject(root["target"], "target")
	if err != nil {
		return err
	}
	if err := requireJSONFields(target, "target", "class", "spec_ref", "effective_state"); err != nil {
		return err
	}
	if story, ok := root["story"]; ok {
		if err := validateStoryFieldPresence(story); err != nil {
			return err
		}
	}
	if feature, ok := root["feature"]; ok {
		if err := validateFeatureFieldPresence(feature); err != nil {
			return err
		}
	}
	return nil
}

func validateStoryFieldPresence(raw json.RawMessage) error {
	story, err := decodeJSONObject(raw, "story")
	if err != nil {
		return err
	}
	if err := requireJSONFields(story, "story", "story_ref", "eligible", "acs"); err != nil {
		return err
	}
	acs, err := decodeJSONArray(story["acs"], "story.acs")
	if err != nil {
		return err
	}
	for i, rawAC := range acs {
		path := fmt.Sprintf("story.acs[%d]", i)
		ac, err := decodeJSONObject(rawAC, path)
		if err != nil {
			return err
		}
		if err := requireJSONFields(ac, path, "id", "text", "status", "summary", "kinds"); err != nil {
			return err
		}
		kinds, err := decodeJSONArray(ac["kinds"], path+".kinds")
		if err != nil {
			return err
		}
		for j, rawKind := range kinds {
			kindPath := fmt.Sprintf("%s.kinds[%d]", path, j)
			kind, err := decodeJSONObject(rawKind, kindPath)
			if err != nil {
				return err
			}
			if err := requireJSONFields(kind, kindPath, "kind", "satisfied", "attestation_state", "violating_witness", "obligation_quality"); err != nil {
				return err
			}
			qualityPath := kindPath + ".obligation_quality"
			quality, err := decodeJSONObject(kind["obligation_quality"], qualityPath)
			if err != nil {
				return err
			}
			if err := requireJSONFields(quality, qualityPath, "structural_state", "match_state", "reason", "witness_path"); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateFeatureFieldPresence(raw json.RawMessage) error {
	feature, err := decodeJSONObject(raw, "feature")
	if err != nil {
		return err
	}
	if err := requireJSONFields(feature, "feature", "acs"); err != nil {
		return err
	}
	acs, err := decodeJSONArray(feature["acs"], "feature.acs")
	if err != nil {
		return err
	}
	for i, rawAC := range acs {
		path := fmt.Sprintf("feature.acs[%d]", i)
		ac, err := decodeJSONObject(rawAC, path)
		if err != nil {
			return err
		}
		if err := requireJSONFields(ac, path, "id", "text", "status", "summary", "implementing_stories", "outcome_floor"); err != nil {
			return err
		}
		floorPath := path + ".outcome_floor"
		floor, err := decodeJSONObject(ac["outcome_floor"], floorPath)
		if err != nil {
			return err
		}
		if err := requireJSONFields(floor, floorPath, "satisfied", "declares_attestation", "attestation_state", "violating_witness"); err != nil {
			return err
		}
	}
	return nil
}

func requireJSONFields(object map[string]json.RawMessage, path string, fields ...string) error {
	for _, field := range fields {
		if _, ok := object[field]; !ok {
			return fmt.Errorf("%s.%s is required", path, field)
		}
	}
	return nil
}

func decodeJSONObject(raw json.RawMessage, path string) (map[string]json.RawMessage, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, fmt.Errorf("%s must be an object: %w", path, err)
	}
	if object == nil {
		return nil, fmt.Errorf("%s must be an object", path)
	}
	return object, nil
}

func decodeJSONArray(raw json.RawMessage, path string) ([]json.RawMessage, error) {
	var array []json.RawMessage
	if err := json.Unmarshal(raw, &array); err != nil {
		return nil, fmt.Errorf("%s must be an array: %w", path, err)
	}
	if array == nil {
		return nil, fmt.Errorf("%s must be an array", path)
	}
	return array, nil
}

// Validate enforces the closed union, enum, and explicit-array contract.
func (r Record) Validate() error {
	if r.Schema != SchemaID {
		return fmt.Errorf("schema %q, want %q", r.Schema, SchemaID)
	}
	if err := r.Target.validate(); err != nil {
		return err
	}
	if (r.Story == nil) == (r.Feature == nil) {
		return fmt.Errorf("exactly one tagged body must be present")
	}
	if r.Target.Class == ClassStory {
		if r.Story == nil || r.Feature != nil {
			return fmt.Errorf("target class %q requires exactly its matching body", r.Target.Class)
		}
		return r.validateStory()
	}
	if r.Feature == nil || r.Story != nil {
		return fmt.Errorf("target class %q requires exactly its matching body", r.Target.Class)
	}
	return r.validateFeature()
}

func (t Target) validate() error {
	if t.Class != ClassStory && t.Class != ClassFeature {
		return fmt.Errorf("target class %q is unknown", t.Class)
	}
	if t.SpecRef == "" {
		return fmt.Errorf("target spec_ref is required")
	}
	if !validEffectiveStates[t.EffectiveState] {
		return fmt.Errorf("target effective_state %q is unknown", t.EffectiveState)
	}
	return nil
}

func (r Record) validateStory() error {
	if r.Story.StoryRef == "" {
		return fmt.Errorf("story_ref is required")
	}
	if r.Story.ACs == nil {
		return fmt.Errorf("tagged body acs must be an array")
	}
	violated := false
	eligible := true
	for i, ac := range r.Story.ACs {
		if err := validateCommonAC("story", i, ac.ID, ac.Status); err != nil {
			return err
		}
		if ac.Kinds == nil {
			return fmt.Errorf("tagged body acs[%d].kinds must be an array", i)
		}
		for j, kind := range ac.Kinds {
			if err := kind.validate(i, j); err != nil {
				return err
			}
		}
		if ac.Status == string(evidence.StatusViolated) {
			violated = true
		}
		if ac.Status != string(evidence.StatusEvidenced) && ac.Status != string(evidence.StatusWaived) {
			eligible = false
		}
	}
	if r.Violated != violated {
		return fmt.Errorf("violated %t does not match target AC statuses", r.Violated)
	}
	if r.Story.Eligible != eligible {
		return fmt.Errorf("eligible %t does not match AC statuses", r.Story.Eligible)
	}
	return nil
}

func (r Record) validateFeature() error {
	if r.Feature.ACs == nil {
		return fmt.Errorf("tagged body acs must be an array")
	}
	violated := false
	for i, ac := range r.Feature.ACs {
		if err := validateCommonAC("feature", i, ac.ID, ac.Status); err != nil {
			return err
		}
		if ac.Status == string(evidence.StatusWaived) {
			return fmt.Errorf("tagged body acs[%d].status %q is not permitted for target class %q", i, ac.Status, r.Target.Class)
		}
		if ac.ImplementingStories == nil {
			return fmt.Errorf("tagged body acs[%d].implementing_stories must be an array", i)
		}
		if err := ac.OutcomeFloor.validate(i); err != nil {
			return err
		}
		if ac.Status == string(evidence.StatusViolated) {
			violated = true
		}
	}
	if r.Violated != violated {
		return fmt.Errorf("violated %t does not match target AC statuses", r.Violated)
	}
	return nil
}

func validateCommonAC(arm string, index int, id, status string) error {
	if id == "" {
		return fmt.Errorf("%s acs[%d].id is required", arm, index)
	}
	if !validStatuses[status] {
		return fmt.Errorf("%s acs[%d].status %q is unknown", arm, index, status)
	}
	return nil
}

func (k KindProjection) validate(acIndex, kindIndex int) error {
	if !validKinds[k.Kind] {
		return fmt.Errorf("tagged body acs[%d].kinds[%d].kind %q is unknown", acIndex, kindIndex, k.Kind)
	}
	if k.Kind == string(artifact.EvidenceAttestation) {
		if !validApplicableAttestationStates[k.AttestationState] {
			return fmt.Errorf("tagged body acs[%d].kinds[%d].attestation_state %q is unknown", acIndex, kindIndex, k.AttestationState)
		}
	} else if k.AttestationState != AttestationNotApplicable {
		return fmt.Errorf("tagged body acs[%d].kinds[%d].attestation_state must be %q for kind %q", acIndex, kindIndex, AttestationNotApplicable, k.Kind)
	}
	if err := k.ObligationQuality.validate(acIndex, kindIndex); err != nil {
		return err
	}
	return nil
}

func (q ObligationQualityProjection) validate(acIndex, kindIndex int) error {
	if !validStructuralStates[q.StructuralState] {
		return fmt.Errorf("tagged body acs[%d].kinds[%d].obligation_quality.structural_state %q is unknown", acIndex, kindIndex, q.StructuralState)
	}
	if !validMatchStates[q.MatchState] {
		return fmt.Errorf("tagged body acs[%d].kinds[%d].obligation_quality.match_state %q is unknown", acIndex, kindIndex, q.MatchState)
	}
	if !validReasons[q.Reason] {
		return fmt.Errorf("tagged body acs[%d].kinds[%d].obligation_quality.reason %q is unknown", acIndex, kindIndex, q.Reason)
	}
	if q.WitnessPath == "" {
		return fmt.Errorf("tagged body acs[%d].kinds[%d].obligation_quality.witness_path is required", acIndex, kindIndex)
	}
	return nil
}

func (f OutcomeFloorProjection) validate(acIndex int) error {
	if f.DeclaresAttestation {
		if !validApplicableAttestationStates[f.AttestationState] {
			return fmt.Errorf("tagged body acs[%d].outcome_floor.attestation_state %q is unknown", acIndex, f.AttestationState)
		}
	} else if f.AttestationState != AttestationNotApplicable {
		return fmt.Errorf("tagged body acs[%d].outcome_floor.attestation_state must be %q when attestation is not declared", acIndex, AttestationNotApplicable)
	}
	return nil
}

func evidenceWitness(record *artifact.Evidence) string {
	if record == nil {
		return ""
	}
	return record.Witness
}

func projectAttestationState(state evidence.AttestationState) (AttestationState, error) {
	switch state {
	case evidence.AttestationAbsent:
		return AttestationAbsent, nil
	case evidence.AttestationUnauthored:
		return AttestationUnauthored, nil
	case evidence.AttestationAuthored:
		return AttestationAuthored, nil
	default:
		return "", fmt.Errorf("unknown native attestation state %d", state)
	}
}

var (
	validEffectiveStates = map[EffectiveState]bool{
		StateProposed: true, StateAcceptedPendingBuild: true, StateSuperseded: true,
		StateClosed: true, StateUnproven: true,
	}
	validStatuses = map[string]bool{
		string(evidence.StatusWaived): true, string(evidence.StatusViolated): true,
		string(evidence.StatusEvidenced): true, string(evidence.StatusPending): true,
		string(evidence.StatusNoSignal): true,
	}
	validKinds = map[string]bool{
		string(artifact.EvidenceStatic): true, string(artifact.EvidenceBehavioral): true,
		string(artifact.EvidenceRuntime): true, string(artifact.EvidenceAttestation): true,
	}
	validApplicableAttestationStates = map[AttestationState]bool{
		AttestationAbsent: true, AttestationUnauthored: true, AttestationAuthored: true,
	}
	validStructuralStates = map[string]bool{
		string(evidence.ObligationElaborated): true, string(evidence.ObligationUnresolvedDesignDebt): true,
		string(evidence.ObligationLegacyUnelaborated): true, string(evidence.ObligationMissing): true,
	}
	validMatchStates = map[string]bool{
		string(evidence.ObligationMatched): true, string(evidence.ObligationViolatedWithWitness): true,
		string(evidence.ObligationUnproven): true,
	}
	validReasons = map[string]bool{
		"": true,
		string(evidence.ObligationReasonProducerMissing):   true,
		string(evidence.ObligationReasonProducerMismatch):  true,
		string(evidence.ObligationReasonSourceMismatch):    true,
		string(evidence.ObligationReasonSourceRefMissing):  true,
		string(evidence.ObligationReasonSourceRefMismatch): true,
		string(evidence.ObligationReasonFreshnessStale):    true,
		string(evidence.ObligationReasonFreshnessUnproven): true,
	}
)
