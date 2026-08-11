package draftmutation

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/designprovenance"
)

type WarningCode string

const (
	WarningDestructiveRemoval WarningCode = "destructive-removal"
	WarningSemanticReorder    WarningCode = "semantic-reorder"
	WarningRelationshipChange WarningCode = "relationship-change"
	WarningLargeReplacement   WarningCode = "large-replacement"
)

type Warning struct {
	Code   WarningCode `json:"code"`
	Target string      `json:"target"`
}

func (w Warning) validate() error {
	if w.Target == "" {
		return fmt.Errorf("draftmutation: warning target is empty")
	}
	switch w.Code {
	case WarningDestructiveRemoval, WarningSemanticReorder, WarningRelationshipChange, WarningLargeReplacement:
		return nil
	default:
		return fmt.Errorf("draftmutation: unknown warning code %q", w.Code)
	}
}

type DisclosureCode string

const (
	DisclosureUnclassifiedDirectEdit DisclosureCode = "unclassified-direct-edit"
	DisclosureContextUnavailable     DisclosureCode = "context-unavailable"
)

// Disclosure is an exact two-arm union.
type Disclosure struct {
	Code       DisclosureCode `json:"code"`
	FromDigest string         `json:"from_digest,omitempty"`
	ToDigest   string         `json:"to_digest,omitempty"`
	Reason     string         `json:"reason,omitempty"`
}

func (d Disclosure) validate() error {
	switch d.Code {
	case DisclosureUnclassifiedDirectEdit:
		if !artifact.ValidDigest(d.FromDigest) || !artifact.ValidDigest(d.ToDigest) || d.Reason != "" {
			return fmt.Errorf("draftmutation: unclassified-direct-edit disclosure has the wrong arm")
		}
	case DisclosureContextUnavailable:
		if d.FromDigest != "" || d.ToDigest != "" || d.Reason != designprovenance.ContextUnavailableReason {
			return fmt.Errorf("draftmutation: context-unavailable disclosure has the wrong arm")
		}
	default:
		return fmt.Errorf("draftmutation: unknown disclosure code %q", d.Code)
	}
	return nil
}

type Result struct {
	Schema         string       `json:"schema"`
	Identity       Identity     `json:"identity"`
	PreviousDigest string       `json:"previous_digest"`
	ResultDigest   string       `json:"result_digest"`
	Changes        []Change     `json:"changes"`
	Warnings       []Warning    `json:"warnings"`
	Disclosures    []Disclosure `json:"disclosures"`
}

func (r Result) Validate() error {
	if r.Schema != ResultSchema {
		return fmt.Errorf("draftmutation: result schema %q is invalid", r.Schema)
	}
	if err := r.Identity.Validate(); err != nil {
		return err
	}
	if !artifact.ValidDigest(r.PreviousDigest) || !artifact.ValidDigest(r.ResultDigest) {
		return fmt.Errorf("draftmutation: result digests are invalid")
	}
	if r.Changes == nil || r.Warnings == nil || r.Disclosures == nil {
		return fmt.Errorf("draftmutation: result collections must be arrays")
	}
	for i, change := range r.Changes {
		if err := change.Validate(); err != nil {
			return fmt.Errorf("draftmutation: changes[%d]: %w", i, err)
		}
	}
	for i, warning := range r.Warnings {
		if err := warning.validate(); err != nil {
			return fmt.Errorf("draftmutation: warnings[%d]: %w", i, err)
		}
	}
	for i, disclosure := range r.Disclosures {
		if err := disclosure.validate(); err != nil {
			return fmt.Errorf("draftmutation: disclosures[%d]: %w", i, err)
		}
	}
	return nil
}

func EncodeResult(result Result) ([]byte, error) {
	if err := result.Validate(); err != nil {
		return nil, err
	}
	return canonjson.Marshal(result)
}

type StaleRefusal struct {
	Schema         string   `json:"schema"`
	Identity       Identity `json:"identity"`
	Code           Code     `json:"code"`
	CurrentDigest  string   `json:"current_digest"`
	ChangedTargets []string `json:"changed_targets"`
}

func (r StaleRefusal) Validate() error {
	if r.Schema != RefusalSchema || r.Code != CodeStaleBase {
		return fmt.Errorf("draftmutation: stale refusal schema/code is invalid")
	}
	if err := r.Identity.Validate(); err != nil {
		return err
	}
	if !artifact.ValidDigest(r.CurrentDigest) || r.ChangedTargets == nil {
		return fmt.Errorf("draftmutation: stale refusal digest/targets are invalid")
	}
	for i, target := range r.ChangedTargets {
		if target == "" || i > 0 && r.ChangedTargets[i-1] >= target {
			return fmt.Errorf("draftmutation: stale changed_targets must be nonempty, unique, and sorted")
		}
	}
	return nil
}

func EncodeStaleRefusal(refusal StaleRefusal) ([]byte, error) {
	if err := refusal.Validate(); err != nil {
		return nil, err
	}
	return canonjson.Marshal(refusal)
}

// DigestBytes hashes exact file bytes, not a canonicalized projection.
func DigestBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}
