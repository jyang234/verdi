package instructionprojection

import (
	"sort"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/policyartifact"
)

// ManifestSchema is the canonical machine-proof schema identifier for
// one adapter's projection manifest (mirroring
// policyauthority.EffectivePolicySchema's own schema-tagged, digest-
// bound posture).
const ManifestSchema = "verdi.instruction-projection/v1"

// manifest is one adapter's canonical projection manifest: the exact
// shape canonjson.Marshal serializes to
// .verdi/policy/projections/<adapter-id>.json. Every field is a plain
// value (no seal) — a manifest is itself a verified PROJECTION, not an
// authority record Verify can trust; Verify always recomputes it fresh
// rather than reading it as ground truth (contract: "Verify must NOT
// trust the stored manifest as authority for what 'should' exist").
type manifest struct {
	Schema          string           `json:"schema"`
	Adapter         manifestAdapter  `json:"adapter"`
	AuthorityDigest string           `json:"authority_digest"`
	Profile         manifestProfile  `json:"profile"`
	Files           []manifestedFile `json:"files"`
}

type manifestAdapter struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

type manifestProfile struct {
	ID     string `json:"id"`
	Digest string `json:"digest"`
}

type manifestedFile struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
}

// buildManifest assembles adapter's canonical manifest from in (the same
// resolved authority renderProjection used) and files (the managed
// paths and content digests Generate wrote or Verify recomputed), sorted
// by path.
func buildManifest(adapter policyartifact.Adapter, in *projectionInput, files []FileDigest) manifest {
	mf := make([]manifestedFile, 0, len(files))
	for _, f := range files {
		mf = append(mf, manifestedFile(f))
	}
	sort.Slice(mf, func(i, j int) bool { return mf[i].Path < mf[j].Path })

	return manifest{
		Schema:          ManifestSchema,
		Adapter:         manifestAdapter{ID: adapter.ID, Version: adapter.Version},
		AuthorityDigest: in.AuthorityDigest,
		Profile:         manifestProfile{ID: in.ProfileID, Digest: in.ProfileDigest},
		Files:           mf,
	}
}

// manifestBytes returns m's canonical JSON encoding (sorted keys, no
// HTML escaping, trailing newline — internal/canonjson.Marshal).
func manifestBytes(m manifest) ([]byte, error) {
	return canonjson.Marshal(m)
}
