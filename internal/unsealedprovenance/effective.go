package unsealedprovenance

import (
	"errors"
	"fmt"

	"github.com/jyang234/verdi/internal/policyauthority"
)

// Effective returns the store's single effective payload, or the default
// {Permitted: false, Cap: 0, Inventory: []} with present == false when no
// policy carries one (design §8: an absent payload is `permitted: false`).
// The kind is a singleton, so two carriers is an error; policyauthority.Load
// already refuses that store, and Effective stays defensive about any
// effective policy it is handed. ep is expected to come from
// policyauthority.Resolve; the returned payload is a deep copy, so a caller
// can never disturb the effective policy's seal through it.
func Effective(ep *policyauthority.EffectivePolicy) (payload *Payload, present bool, err error) {
	if ep == nil {
		return nil, false, errors.New("unsealedprovenance: effective policy is nil")
	}
	var found *Payload
	var owner string
	for _, entry := range ep.Policies {
		raw, ok := entry.Payloads[PayloadKind]
		if !ok {
			continue
		}
		if found != nil {
			return nil, false, fmt.Errorf("unsealedprovenance: payload %s is carried by both policy %s and policy %s", PayloadKind, owner, entry.PolicyID)
		}
		typed, ok := raw.(*Payload)
		if !ok || typed == nil {
			return nil, false, fmt.Errorf("unsealedprovenance: policy %s payload %s is not the registered typed payload (%T)", entry.PolicyID, PayloadKind, raw)
		}
		if err := typed.Validate(); err != nil {
			return nil, false, fmt.Errorf("unsealedprovenance: policy %s: %w", entry.PolicyID, err)
		}
		found, owner = typed, entry.PolicyID
	}
	if found == nil {
		return &Payload{Permitted: false, Cap: 0, Inventory: []InventoryEntry{}}, false, nil
	}
	return found.clone(), true, nil
}
