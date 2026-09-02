package constitutionimpact

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"sort"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/policyartifact"
)

type loadedSide struct {
	evidence  InventoryEvidence
	consumers []plannedConsumer
	reasons   []Reason
}

// BuildPlan loads both inventories and their action catalogs from the supplied
// exact-tree views, then seals the canonical union for a nonempty layer diff.
// A malformed present artifact is an operational error; missing or unavailable
// exact-tree evidence is retained as an unproven coverage reason.
func BuildPlan(ctx context.Context, accepted, proposed ExactTree, layers []LayerChange) (Plan, error) {
	if ctx == nil {
		return Plan{}, fmt.Errorf("constitutionimpact: build plan: context is nil")
	}
	if err := validateExactTree("accepted", accepted); err != nil {
		return Plan{}, err
	}
	if err := validateExactTree("proposed", proposed); err != nil {
		return Plan{}, err
	}
	canonicalLayers, err := canonicalLayerChanges(layers)
	if err != nil {
		return Plan{}, fmt.Errorf("constitutionimpact: build plan: %w", err)
	}
	acceptedSide, err := loadSide(ctx, "accepted", accepted)
	if err != nil {
		return Plan{}, err
	}
	proposedSide, err := loadSide(ctx, "proposed", proposed)
	if err != nil {
		return Plan{}, err
	}

	initial := append(cloneReasons(acceptedSide.reasons), proposedSide.reasons...)
	union := []plannedConsumer{}
	if len(canonicalLayers) != 0 {
		byIdentity := make(map[string]plannedConsumer, len(acceptedSide.consumers)+len(proposedSide.consumers))
		for _, row := range append(append([]plannedConsumer(nil), acceptedSide.consumers...), proposedSide.consumers...) {
			if previous, ok := byIdentity[row.identity]; ok && !bytes.Equal(previous.canonical, row.canonical) {
				return Plan{}, fmt.Errorf("constitutionimpact: build plan: consumer identity collision %s", row.identity)
			}
			byIdentity[row.identity] = row
		}
		identities := make([]string, 0, len(byIdentity))
		for identity := range byIdentity {
			identities = append(identities, identity)
		}
		sort.Strings(identities)
		union = make([]plannedConsumer, len(identities))
		for i, identity := range identities {
			row := byIdentity[identity]
			row.consumer = cloneConsumer(row.consumer)
			row.canonical = append([]byte(nil), row.canonical...)
			union[i] = row
		}
		if len(union) == 0 && acceptedSide.evidence.Presence == PresencePresent && proposedSide.evidence.Presence == PresencePresent {
			initial = append(initial, Reason{Code: ReasonConsumerUniverseEmpty, Witnesses: []string{InventoryPath}})
		}
	}
	initial = normalizedReasons(initial)
	return Plan{
		accepted:       acceptedSide.evidence,
		proposed:       proposedSide.evidence,
		layers:         canonicalLayers,
		layerChanged:   len(canonicalLayers) != 0,
		consumers:      union,
		initialReasons: initial,
	}, nil
}

// Consumers returns an alias-safe copy of the canonical registered union.
func (p Plan) Consumers() []Consumer {
	out := make([]Consumer, len(p.consumers))
	for i := range p.consumers {
		out[i] = cloneConsumer(p.consumers[i].consumer)
	}
	return out
}

func validateExactTree(name string, tree ExactTree) error {
	if err := gitx.ValidateFullOID(tree.Commit); err != nil {
		return fmt.Errorf("constitutionimpact: build plan: %s commit identity: %w", name, err)
	}
	if err := gitx.ValidateFullOID(tree.Tree); err != nil {
		return fmt.Errorf("constitutionimpact: build plan: %s tree identity: %w", name, err)
	}
	return nil
}

func loadSide(ctx context.Context, name string, tree ExactTree) (loadedSide, error) {
	evidence := InventoryEvidence{Commit: tree.Commit, Tree: tree.Tree, Presence: PresenceUnavailable}
	if tree.FS == nil {
		return loadedSide{evidence: evidence, reasons: []Reason{treeUnavailableReason(name)}}, nil
	}
	if err := ctx.Err(); err != nil {
		return loadedSide{}, fmt.Errorf("constitutionimpact: loading %s tree: %w", name, err)
	}
	inventoryBytes, err := fs.ReadFile(tree.FS, InventoryPath)
	if errors.Is(err, fs.ErrNotExist) {
		evidence.Presence = PresenceMissing
		return loadedSide{evidence: evidence, reasons: []Reason{inventoryMissingReason(name)}}, nil
	}
	if err != nil {
		return loadedSide{evidence: evidence, reasons: []Reason{treeUnavailableReason(name)}}, nil
	}
	inventory, err := DecodeInventory(inventoryBytes)
	if err != nil {
		return loadedSide{}, fmt.Errorf("constitutionimpact: loading %s inventory: %w", name, err)
	}
	evidence.Presence = PresencePresent
	evidence.InventoryDigest = digestBytes(inventoryBytes)

	if err := ctx.Err(); err != nil {
		return loadedSide{}, fmt.Errorf("constitutionimpact: loading %s tree: %w", name, err)
	}
	constitutionBytes, err := fs.ReadFile(tree.FS, constitutionPath)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return loadedSide{}, fmt.Errorf("constitutionimpact: loading %s tree: %w", name, ctxErr)
	}
	if errors.Is(err, fs.ErrNotExist) || err != nil {
		consumers, duplicateReasons, makeErr := plannedConsumers(name, inventory)
		if makeErr != nil {
			return loadedSide{}, makeErr
		}
		reasons := append(duplicateReasons, catalogUnavailableReason(name))
		return loadedSide{evidence: evidence, consumers: consumers, reasons: reasons}, nil
	}
	constitution, err := policyartifact.DecodeConstitution(constitutionBytes)
	if err != nil {
		return loadedSide{}, fmt.Errorf("constitutionimpact: loading %s constitution: %w", name, err)
	}
	evidence.ConstitutionDigest, err = constitution.Digest()
	if err != nil {
		return loadedSide{}, fmt.Errorf("constitutionimpact: loading %s constitution digest: %w", name, err)
	}
	for i, consumer := range inventory.Consumers {
		for j, operation := range consumer.GovernedOperations {
			if !constitution.Subjects.Has(policyartifact.FamilyAction, operation) {
				return loadedSide{}, fmt.Errorf("constitutionimpact: loading %s inventory: consumers[%d].governed_operations[%d] %q is not registered in that tree's action-subject catalog", name, i, j, operation)
			}
		}
	}
	consumers, duplicateReasons, err := plannedConsumers(name, inventory)
	if err != nil {
		return loadedSide{}, err
	}
	return loadedSide{evidence: evidence, consumers: consumers, reasons: duplicateReasons}, nil
}

func plannedConsumers(side string, inventory Inventory) ([]plannedConsumer, []Reason, error) {
	rows := make([]plannedConsumer, len(inventory.Consumers))
	seen := make(map[string]bool, len(rows))
	duplicates := []string{}
	for i, consumer := range inventory.Consumers {
		doc, err := consumerDocFor(consumer)
		if err != nil {
			return nil, nil, fmt.Errorf("constitutionimpact: %s inventory consumer %d: %w", side, i, err)
		}
		identity, err := consumerIdentityFromDoc(doc)
		if err != nil {
			return nil, nil, fmt.Errorf("constitutionimpact: %s inventory consumer %d identity: %w", side, i, err)
		}
		canonical, err := canonjson.Marshal(doc)
		if err != nil {
			return nil, nil, fmt.Errorf("constitutionimpact: %s inventory consumer %d canonical form: %w", side, i, err)
		}
		rows[i] = plannedConsumer{identity: identity, consumer: cloneConsumer(consumer), canonical: canonical}
		if seen[identity] {
			duplicates = append(duplicates, side+":"+identity)
		}
		seen[identity] = true
	}
	reasons := []Reason{}
	if len(duplicates) != 0 {
		sort.Strings(duplicates)
		reasons = append(reasons, Reason{Code: ReasonInventoryDuplicate, Witnesses: duplicates})
	}
	return rows, reasons, nil
}

func canonicalLayerChanges(layers []LayerChange) ([]LayerChange, error) {
	if layers == nil {
		return []LayerChange{}, nil
	}
	out := make([]LayerChange, len(layers))
	copy(out, layers)
	for i, layer := range out {
		if layer.Kind == "" || layer.ID == "" {
			return nil, fmt.Errorf("layers[%d] kind and id are mandatory", i)
		}
		switch layer.Change {
		case "added":
			if layer.AcceptedDigest != "" || layer.ProposedDigest == "" {
				return nil, fmt.Errorf("layers[%d] added change must carry only proposed_digest", i)
			}
			if !artifact.ValidDigest(layer.ProposedDigest) {
				return nil, fmt.Errorf("layers[%d] proposed_digest is not a canonical sha256 digest", i)
			}
		case "removed":
			if layer.AcceptedDigest == "" || layer.ProposedDigest != "" {
				return nil, fmt.Errorf("layers[%d] removed change must carry only accepted_digest", i)
			}
			if !artifact.ValidDigest(layer.AcceptedDigest) {
				return nil, fmt.Errorf("layers[%d] accepted_digest is not a canonical sha256 digest", i)
			}
		case "changed":
			if layer.AcceptedDigest == "" || layer.ProposedDigest == "" || layer.AcceptedDigest == layer.ProposedDigest {
				return nil, fmt.Errorf("layers[%d] changed change must carry distinct accepted/proposed digests", i)
			}
			if !artifact.ValidDigest(layer.AcceptedDigest) || !artifact.ValidDigest(layer.ProposedDigest) {
				return nil, fmt.Errorf("layers[%d] accepted/proposed digests must be canonical sha256 digests", i)
			}
		default:
			return nil, fmt.Errorf("layers[%d] unknown change %q", i, layer.Change)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Kind+"\x00"+out[i].ID < out[j].Kind+"\x00"+out[j].ID })
	for i := 1; i < len(out); i++ {
		if out[i-1].Kind == out[i].Kind && out[i-1].ID == out[i].ID {
			return nil, fmt.Errorf("layers contains duplicate %s/%s", out[i].Kind, out[i].ID)
		}
	}
	return out, nil
}

func digestBytes(data []byte) string {
	digest := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func inventoryMissingReason(side string) Reason {
	if side == "accepted" {
		return Reason{Code: ReasonAcceptedInventoryMissing, Witnesses: []string{InventoryPath}}
	}
	return Reason{Code: ReasonProposedInventoryMissing, Witnesses: []string{InventoryPath}}
}

func treeUnavailableReason(side string) Reason {
	if side == "accepted" {
		return Reason{Code: ReasonAcceptedTreeUnavailable, Witnesses: []string{side}}
	}
	return Reason{Code: ReasonProposedTreeUnavailable, Witnesses: []string{side}}
}

func catalogUnavailableReason(side string) Reason {
	if side == "accepted" {
		return Reason{Code: ReasonAcceptedCatalogUnavailable, Witnesses: []string{constitutionPath}}
	}
	return Reason{Code: ReasonProposedCatalogUnavailable, Witnesses: []string{constitutionPath}}
}

func cloneReasons(in []Reason) []Reason {
	out := make([]Reason, len(in))
	for i := range in {
		out[i] = Reason{Code: in[i].Code, Witnesses: cloneStrings(in[i].Witnesses)}
	}
	return out
}
