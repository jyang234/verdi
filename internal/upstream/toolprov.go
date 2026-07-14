package upstream

import (
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
)

// ToolProvenance is a derived bundle's toolchain.json (spec/forge-transport
// ac-4/dc-4, adjudicated 2026-07-13): the recorded tool provenance carrier
// a fetched evidence bundle presents for the I-4 secondary defense. Tool is
// the pinned toolchain's pseudo-version string exactly as `flowmap graph`
// stamped it into Graph.Tool — the string CheckToolPin compares against
// verdi.yaml's toolchain.commit at intake (cmd/verdi/sync.go).
//
// The file is OPTIONAL in a bundle: internal/bundle.Assemble writes it only
// when an upstream tool actually ran (a non-empty Graph.Tool), so a
// pre-carrier bundle or one from a producer that runs no upstream tool
// (cmd/verdi/selfevidence.go) simply omits it — intake discloses that as
// unproven rather than refusing or silently passing.
type ToolProvenance struct {
	Tool string `json:"tool"`
}

// DecodeToolProvenance strict-decodes a toolchain.json (verdi-owned, so
// full strict decode: DisallowUnknownFields + trailing-data rejection via
// the internal/artifact seam, matching the other derived files' posture).
// A present-but-empty tool field fails closed: the producer only writes
// the file when the tool string is non-empty, so emptiness is malformation,
// never a legitimate "no provenance" signal (absence of the file is).
func DecodeToolProvenance(data []byte) (*ToolProvenance, error) {
	var p ToolProvenance
	if err := artifact.DecodeStrictJSON(data, &p); err != nil {
		return nil, fmt.Errorf("upstream: decoding toolchain.json: %w", err)
	}
	if p.Tool == "" {
		return nil, fmt.Errorf("upstream: toolchain.json has an empty tool field (a bundle with no tool provenance omits the file entirely)")
	}
	return &p, nil
}
