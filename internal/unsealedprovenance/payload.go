// Package unsealedprovenance owns the typed constitution policy payload
// `unsealed-provenance` (unsealed-provenance exemption design §6–§8;
// SI-244, SI-245, SI-246; wire form SI-255): whether unsealed-provenance
// exemptions are permitted at all, the cap on active exemptions, the
// owner-reviewed inventory of stories that may be exempted, and the
// append-only cutoff record. It is a single-owner (singleton) payload inside
// Context Integrity's one policy-authority system (DC-23): policyauthority
// stores, loads, resolves, and digests it; this package alone interprets its
// fields. The payload never changes evidence semantics or proof formats.
package unsealedprovenance

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/policyartifact"
)

// PayloadKind is the registered payload key (design §8, SI-246).
const PayloadKind = "unsealed-provenance"

// Payload is the complete `unsealed-provenance` field set (SI-255). An
// absent payload reads as Permitted false, Cap 0, and an empty inventory
// (Effective). Permission and the inventory are independent walls: an
// inventory may be non-empty while Permitted is false.
type Payload struct {
	Permitted bool             `yaml:"permitted" json:"permitted"`
	Cap       int              `yaml:"cap" json:"cap"`
	Inventory []InventoryEntry `yaml:"inventory" json:"inventory"` // non-nil; sorted by ID
	Cutoff    *Cutoff          `yaml:"cutoff,omitempty" json:"cutoff,omitempty"`
}

// InventoryEntry is one owner-reviewed inventory entry (design §6, SI-244).
type InventoryEntry struct {
	ID         string `yaml:"id" json:"id"`                   // kebab-case
	Story      string `yaml:"story" json:"story"`             // spec/<name>, unpinned, unfragmented
	AdmittedBy string `yaml:"admitted_by" json:"admitted_by"` // one non-blank line naming the governed change
}

// Cutoff is the append-only sunset record (design §7, SI-245): once the
// accepted constitution carries it, no later constitution change may remove
// or replace it (CutoffViolation).
type Cutoff struct {
	Commit string `yaml:"commit" json:"commit"` // C_cut: lowercase hex object id, 40 or 64 characters
}

// inventoryIDRe is the house kebab-case identifier grammar (02 §Identity).
var inventoryIDRe = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// PayloadKind implements policyartifact.Payload.
func (*Payload) PayloadKind() string { return PayloadKind }

// Validate enforces the closed payload grammar without normalizing authored
// authority: the author's inventory order is digest-bound, so an unsorted
// inventory is refused rather than re-sorted.
func (p *Payload) Validate() error {
	if p == nil {
		return errors.New("unsealedprovenance: payload is nil")
	}
	if p.Cap < 0 {
		return fmt.Errorf("unsealedprovenance: cap %d must be a non-negative integer", p.Cap)
	}
	if p.Inventory == nil {
		return errors.New("unsealedprovenance: inventory is missing or null")
	}
	ids := make(map[string]bool, len(p.Inventory))
	storyOwners := make(map[string]string, len(p.Inventory))
	for i, entry := range p.Inventory {
		if err := entry.validate(); err != nil {
			return fmt.Errorf("unsealedprovenance: inventory[%d]%w", i, err)
		}
		if ids[entry.ID] {
			return fmt.Errorf("unsealedprovenance: duplicate inventory id %q", entry.ID)
		}
		ids[entry.ID] = true
		if owner, dup := storyOwners[entry.Story]; dup {
			return fmt.Errorf("unsealedprovenance: inventory id %q names spec ref %q already named by inventory id %q", entry.ID, entry.Story, owner)
		}
		storyOwners[entry.Story] = entry.ID
		if i > 0 && p.Inventory[i-1].ID > entry.ID {
			return fmt.Errorf("unsealedprovenance: inventory must be sorted by id (%q precedes %q)", p.Inventory[i-1].ID, entry.ID)
		}
	}
	if p.Cutoff != nil {
		if err := gitx.ValidateFullOID(p.Cutoff.Commit); err != nil {
			return fmt.Errorf("unsealedprovenance: cutoff.commit %q: %w", p.Cutoff.Commit, err)
		}
	}
	return nil
}

// validate checks one entry's own fields. Its errors begin with the field
// path suffix so Validate can prefix the entry index.
func (e InventoryEntry) validate() error {
	if !inventoryIDRe.MatchString(e.ID) {
		return fmt.Errorf(".id %q must be kebab-case", e.ID)
	}
	ref, err := artifact.ParseRef(e.Story)
	if err != nil {
		return fmt.Errorf(".story %q: %w", e.Story, err)
	}
	switch {
	case ref.Kind != artifact.KindSpec:
		return fmt.Errorf(".story %q must be a spec ref (spec/<name>)", e.Story)
	case ref.Pinned():
		return fmt.Errorf(".story %q must not be pinned", e.Story)
	case ref.Fragment():
		return fmt.Errorf(".story %q must not carry a fragment", e.Story)
	}
	if strings.TrimSpace(e.AdmittedBy) == "" {
		return errors.New(".admitted_by must name the governed change that admitted the entry")
	}
	if strings.ContainsAny(e.AdmittedBy, "\n\r") {
		return errors.New(".admitted_by must be a single line")
	}
	return nil
}

// Entry returns the inventory entry with id, if any.
func (p *Payload) Entry(id string) (InventoryEntry, bool) {
	if p == nil {
		return InventoryEntry{}, false
	}
	for _, entry := range p.Inventory {
		if entry.ID == id {
			return entry, true
		}
	}
	return InventoryEntry{}, false
}

// clone returns a deep copy that shares no slice or pointer with p.
func (p *Payload) clone() *Payload {
	out := &Payload{Permitted: p.Permitted, Cap: p.Cap, Inventory: append([]InventoryEntry{}, p.Inventory...)}
	if p.Cutoff != nil {
		cutoff := *p.Cutoff
		out.Cutoff = &cutoff
	}
	return out
}

func init() {
	policyartifact.RegisterPayloadKind(PayloadKind, func(raw []byte) (policyartifact.Payload, error) {
		return DecodePayload(raw)
	})
}
