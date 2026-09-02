package constitutionapp

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"testing/fstest"

	"github.com/jyang234/verdi/internal/constitutionimpact"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/policyauthority"
)

// AuthorityStore is constitutionapp's own consumer-owned port (04 §port
// pattern) over internal/policyauthority's one loader and one resolver —
// never a second constitution loader or a second effective-policy resolver
// (the Task 3 stop gate). Load reads whatever a caller-supplied fs.FS
// exposes (the current checkout's real filesystem for the proposed state, or
// a Git-tree-backed source for the accepted state — acceptedSource below);
// Resolve is byte-identical to policyauthority.Resolve.
type AuthorityStore interface {
	LoadFromSource(source fs.FS) (*policyauthority.Store, error)
	Resolve(store *policyauthority.Store) (*policyauthority.EffectivePolicy, error)
}

type policyauthorityStore struct{}

func (policyauthorityStore) LoadFromSource(source fs.FS) (*policyauthority.Store, error) {
	return policyauthority.LoadFromSource(source)
}
func (policyauthorityStore) Resolve(store *policyauthority.Store) (*policyauthority.EffectivePolicy, error) {
	return policyauthority.Resolve(store)
}

// SourceLayer is one constitution-store artifact's identity, ownership, and
// scope WITHOUT its post-refinement effective content — the "source layers"
// Wave 6 Task 3 requires exposed separately from the effective rule ledger,
// so a caller can see which policy/overlay/exemption/disposition artifacts
// exist and who owns them without the narrow-refinement projection
// Resolve() already computes (EffectivePolicy, carried on Snapshot.Effective
// unflattened).
type SourceLayer struct {
	Kind    string               `json:"kind"`
	ID      string               `json:"id"`
	Title   string               `json:"title"`
	Owners  []string             `json:"owners"`
	Scope   policyartifact.Scope `json:"scope"`
	Digest  string               `json:"digest"`
	Refines string               `json:"refines,omitempty"`
}

// Snapshot is one resolved constitution state — accepted or proposed — at an
// exact Git identity. Adopted is false when the store has not adopted a
// constitution at all (policyauthority.ErrNotAdopted) or has adopted one
// incompletely (policyauthority.ErrIncompleteAdoption); either is a
// disclosed, non-fatal fact, never an error (CO-1: an honest "not adopted"
// is not a fault). Any OTHER Load/Resolve failure (corrupted YAML, a broken
// cross-validation invariant, an unresolvable profile) is reported through
// the caller's own *Error instead — Snapshot itself only ever represents a
// clean read.
type Snapshot struct {
	Ref                string                           `json:"ref"`
	Adopted            bool                             `json:"adopted"`
	Reason             string                           `json:"reason,omitempty"`
	ConstitutionID     string                           `json:"constitution_id,omitempty"`
	ConstitutionDigest string                           `json:"constitution_digest,omitempty"`
	ProfileID          string                           `json:"profile_id,omitempty"`
	ProfileDigest      string                           `json:"profile_digest,omitempty"`
	Layers             []SourceLayer                    `json:"layers,omitempty"`
	Effective          *policyauthority.EffectivePolicy `json:"effective,omitempty"`
}

// loadSnapshot loads and resolves the constitution store exposed by source,
// classifying ErrNotAdopted/ErrIncompleteAdoption as a disclosed
// not-adopted Snapshot rather than an error. Any other failure is returned
// as a typed *Error naming code, so a caller can distinguish "there is no
// constitution here" from "the constitution here is broken."
func loadSnapshot(store AuthorityStore, source fs.FS, ref, corruptCode string) (Snapshot, *Error) {
	if store == nil {
		return Snapshot{}, operational("authority-store-unavailable", "authority store is not configured", nil)
	}
	s, err := store.LoadFromSource(source)
	if err != nil {
		if errors.Is(err, policyauthority.ErrNotAdopted) || errors.Is(err, policyauthority.ErrIncompleteAdoption) {
			return Snapshot{Ref: ref, Adopted: false, Reason: err.Error()}, nil
		}
		return Snapshot{}, verdictWithCause(corruptCode, "loading constitution store at "+ref, err)
	}
	effective, err := store.Resolve(s)
	if err != nil {
		return Snapshot{}, verdictWithCause(corruptCode, "resolving effective policy at "+ref, err)
	}

	layers := make([]SourceLayer, 0, len(s.Policies)+len(s.Overlays)+len(s.Exemptions)+len(s.Dispositions))
	for _, id := range sortedKeys(s.Policies) {
		p := s.Policies[id]
		digest, digestErr := p.Digest()
		if digestErr != nil {
			return Snapshot{}, verdictWithCause(corruptCode, "digesting policy "+id, digestErr)
		}
		layers = append(layers, SourceLayer{Kind: policyartifact.KindPolicy, ID: p.ID, Title: p.Title, Owners: p.Owners, Scope: p.Scope, Digest: digest})
	}
	for _, id := range sortedKeys(s.Overlays) {
		o := s.Overlays[id]
		digest, digestErr := o.Digest()
		if digestErr != nil {
			return Snapshot{}, verdictWithCause(corruptCode, "digesting overlay "+id, digestErr)
		}
		layers = append(layers, SourceLayer{Kind: policyartifact.KindOverlay, ID: o.ID, Title: o.Title, Owners: o.Owners, Scope: o.Scope, Digest: digest, Refines: o.Refines})
	}
	for _, id := range sortedKeys(s.Exemptions) {
		e := s.Exemptions[id]
		digest, digestErr := e.Digest()
		if digestErr != nil {
			return Snapshot{}, verdictWithCause(corruptCode, "digesting exemption "+id, digestErr)
		}
		layers = append(layers, SourceLayer{Kind: policyartifact.KindExemption, ID: e.ID, Title: e.Title, Owners: e.Owners, Scope: e.Scope, Digest: digest})
	}
	for _, id := range sortedKeys(s.Dispositions) {
		d := s.Dispositions[id]
		digest, digestErr := d.Digest()
		if digestErr != nil {
			return Snapshot{}, verdictWithCause(corruptCode, "digesting disposition "+id, digestErr)
		}
		layers = append(layers, SourceLayer{Kind: policyartifact.KindDisposition, ID: d.ID, Title: d.Title, Owners: d.Owners, Scope: d.Scope, Digest: digest})
	}

	constitutionDigest, digestErr := s.Constitution.Digest()
	if digestErr != nil {
		return Snapshot{}, verdictWithCause(corruptCode, "digesting constitution", digestErr)
	}

	return Snapshot{
		Ref:                ref,
		Adopted:            true,
		ConstitutionID:     s.Constitution.ID,
		ConstitutionDigest: constitutionDigest,
		ProfileID:          effective.ProfileID,
		ProfileDigest:      effective.ProfileDigest,
		Layers:             layers,
		Effective:          effective,
	}, nil
}

func sortedKeys[V any](m map[string]*V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// exactTreeAt materializes the policy and constitution-impact metadata from
// one pinned commit as a read-only, non-following fs.FS. One enumeration feeds
// both the authority snapshot and constitutionimpact.BuildPlan. Git modes are
// preserved: executable regular blobs stay executable and policy symlinks stay
// symlinks so policyauthority performs its own refusal rather than consuming a
// target outside the tree.
func (s Service) exactTreeAt(ctx context.Context, root, commit string) (constitutionimpact.ExactTree, error) {
	if s.Git == nil {
		return constitutionimpact.ExactTree{}, errors.New("constitutionapp: git reader is not configured")
	}
	if err := gitx.ValidateFullOID(commit); err != nil {
		return constitutionimpact.ExactTree{}, fmt.Errorf("constitutionapp: exact tree commit: %w", err)
	}
	resolved, err := s.Git.RevParse(ctx, root, commit+"^{commit}")
	if err != nil {
		return constitutionimpact.ExactTree{}, fmt.Errorf("constitutionapp: resolve exact commit %s: %w", commit, err)
	}
	if resolved != commit {
		return constitutionimpact.ExactTree{}, fmt.Errorf("constitutionapp: exact commit resolved as %s, want %s", resolved, commit)
	}
	tree, err := s.Git.RevParse(ctx, root, commit+"^{tree}")
	if err != nil {
		return constitutionimpact.ExactTree{}, fmt.Errorf("constitutionapp: resolve tree for %s: %w", commit, err)
	}
	if err := gitx.ValidateFullOID(tree); err != nil {
		return constitutionimpact.ExactTree{}, fmt.Errorf("constitutionapp: exact tree identity: %w", err)
	}
	entries, err := s.Git.LsTreeEntries(ctx, root, commit)
	if err != nil {
		return constitutionimpact.ExactTree{}, err
	}
	source := fstest.MapFS{}
	for _, entry := range entries {
		if !constitutionTreePath(entry.Path) {
			continue
		}
		if entry.Type != "blob" {
			return constitutionimpact.ExactTree{}, fmt.Errorf("constitutionapp: exact tree entry %s has unsupported object type %s", entry.Path, entry.Type)
		}
		var mode fs.FileMode
		switch entry.Mode {
		case "100644":
			mode = 0o644
		case "100755":
			mode = 0o755
		case "120000":
			if entry.Path == constitutionimpact.InventoryPath || entry.Path == constitutionImpactDir {
				return constitutionimpact.ExactTree{}, fmt.Errorf("constitutionapp: exact tree entry %s is a symlink; constitution impact metadata must be a regular file", entry.Path)
			}
			mode = fs.ModeSymlink | 0o777
		default:
			return constitutionimpact.ExactTree{}, fmt.Errorf("constitutionapp: exact tree entry %s has unsupported blob mode %s", entry.Path, entry.Mode)
		}
		data, readErr := s.Git.Show(ctx, root, commit, entry.Path)
		if readErr != nil {
			return constitutionimpact.ExactTree{}, readErr
		}
		source[entry.Path] = &fstest.MapFile{Data: data, Mode: mode}
	}
	return constitutionimpact.ExactTree{Commit: commit, Tree: tree, FS: source}, nil
}

func (s Service) acceptedSource(ctx context.Context, root, commit string) (fs.FS, error) {
	exact, err := s.exactTreeAt(ctx, root, commit)
	if err != nil {
		return nil, err
	}
	return exact.FS, nil
}

const constitutionPolicyDir = ".verdi/policy"
const constitutionImpactDir = ".verdi/constitution"

// constitutionPolicyPath reports whether path is the constitution store
// root itself (a symlink entry there must survive into the source so it is
// refused, never skipped) or a file inside it.
func constitutionPolicyPath(path string) bool {
	return path == constitutionPolicyDir || len(path) > len(constitutionPolicyDir) && path[:len(constitutionPolicyDir)+1] == constitutionPolicyDir+"/"
}

func constitutionTreePath(path string) bool {
	return constitutionPolicyPath(path) || path == constitutionImpactDir || path == constitutionimpact.InventoryPath
}
