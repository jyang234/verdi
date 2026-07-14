package upstream

import (
	"encoding/json"
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
)

// ObligationStatus is a graph obligation's `status` field (PLAN.md §3:
// "per-obligation status arrives in graph JSON obligations[] as SATISFIED /
// VIOLATED / CANT-PROVE / UNMATCHED"). All four values were observed in
// spike S1's captures (testdata/svcfix-canned).
type ObligationStatus string

const (
	ObligationSatisfied ObligationStatus = "SATISFIED"
	ObligationViolated  ObligationStatus = "VIOLATED"
	ObligationCantProve ObligationStatus = "CANT-PROVE"
	ObligationUnmatched ObligationStatus = "UNMATCHED"
)

var validObligationStatuses = map[ObligationStatus]bool{
	ObligationSatisfied: true,
	ObligationViolated:  true,
	ObligationCantProve: true,
	ObligationUnmatched: true,
}

// Node is one `graph.nodes[]` entry.
type Node struct {
	FQN      string `json:"fqn"`
	Sig      string `json:"sig,omitempty"`
	Tier     int    `json:"tier,omitempty"`
	Package  string `json:"package,omitempty"`
	Fallible bool   `json:"fallible,omitempty"`
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
	EndLine  int    `json:"end_line,omitempty"`
}

// Edge is one `graph.edges[]` entry.
type Edge struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Tier     int    `json:"tier,omitempty"`
	Boundary string `json:"boundary,omitempty"`
}

// GraphEntrypoint is one `graph.entrypoints[]` entry (the unscoped, full-
// graph shape; distinct from the scoped run's singular `entrypoint`
// string field — see Graph.Entrypoint).
type GraphEntrypoint struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	Fn   string `json:"fn,omitempty"`
}

// EffectOrderEntry is one `graph.effect_order[]` entry: an observed
// ordering constraint between an effect and a subsequent call.
type EffectOrderEntry struct {
	Fn         string `json:"fn"`
	Effect     string `json:"effect"`
	EffectSite string `json:"effect_site,omitempty"`
	Callee     string `json:"callee,omitempty"`
	CalleeSite string `json:"callee_site,omitempty"`
	Always     bool   `json:"always,omitempty"`
	Via        string `json:"via,omitempty"`
}

// Obligation is one `graph.obligations[]` entry: the per-call-site (or, for
// UNMATCHED, per-rule) verdict for a `.flowmap.yaml` obligation. Fn and
// Site are empty for UNMATCHED — the rule's anchor never matched any call
// site, so there is no site to report (spike S1's "renamed-away" capture).
type Obligation struct {
	Rule   string           `json:"rule"`
	Kind   string           `json:"kind,omitempty"`
	Fn     string           `json:"fn,omitempty"`
	Site   string           `json:"site,omitempty"`
	Status ObligationStatus `json:"status"`
	Detail string           `json:"detail,omitempty"`
}

// Validate checks Rule is non-empty and Status is a known enum, failing
// closed on any value spike S1 did not observe (CLAUDE.md: "unknown enum
// values fail closed").
func (o Obligation) Validate() error {
	if o.Rule == "" {
		return fmt.Errorf("upstream: graph obligation has an empty rule")
	}
	if !validObligationStatuses[o.Status] {
		return fmt.Errorf("upstream: graph obligation %q: unknown status %q", o.Rule, o.Status)
	}
	return nil
}

// FrontierMarker is one `graph.frontier.markers[]` entry: a disclosed point
// where flowmap's static analysis could not resolve a target (e.g. a
// dynamic bus publish target), named rather than silently dropped.
type FrontierMarker struct {
	Kind          string `json:"kind"`
	Bin           string `json:"bin,omitempty"`
	Site          string `json:"site,omitempty"`
	Owner         string `json:"owner,omitempty"`
	ReclaimerHint string `json:"reclaimer_hint,omitempty"`
}

// Frontier is the `graph.frontier` object (obligation-bearing graphs only):
// a coarse, non-gating disclosure of routes and dynamic-dispatch points
// flowmap could not confirm reach any effect.
type Frontier struct {
	UnconfirmedRoutes int              `json:"unconfirmed_routes,omitempty"`
	Coverage          string           `json:"coverage,omitempty"`
	Markers           []FrontierMarker `json:"markers,omitempty"`
}

// Annotation is one `graph.annotations[]` entry: human/AI context attached to
// a blind spot, keyed by (Site, Kind) to the graph's blind-spot manifest.
// Disclosure-only upstream — no verdict reads it — but it is a real,
// omitempty field of upstream's Graph struct that a service's config
// populates, so strict decode must model it (Site/Kind/Note are always
// present; By/Claim are optional). Shape mirrors verdi-go graphio.Annotation,
// verified read-only against the pinned toolchain.
type Annotation struct {
	Site  string `json:"site"`
	Kind  string `json:"kind"`
	Note  string `json:"note"`
	By    string `json:"by,omitempty"`
	Claim string `json:"claim,omitempty"`
}

// Graph is `flowmap graph`'s JSON output (PLAN.md §3: "graph JSON carries
// no schema_version — versioning is the pinned binary plus strict
// structural decode"). Every field is optional/omitempty per upstream's own
// shape: an unscoped run omits Entrypoint (singular) and carries
// Entrypoints (plural); a --entry-scoped run is the reverse; a graph with
// no `.flowmap.yaml` obligations: block omits Obligations and Frontier
// entirely. OmittedPackages (types-only first-party imports the rollup
// discloses) and Annotations (human/AI blind-spot context) are two further
// omitempty disclosure fields — absent from svcfix's capture but declared by
// upstream's Graph struct, so they must be modeled or strict decode fails
// closed the moment a real service's config populates them.
//
// BlindSpots, and the boundary-contract-only nested arrays elsewhere in
// this package, use []json.RawMessage where spike S1's captures never
// populated the field: modeling a concrete struct for content nobody has
// ever observed would be inventing a schema verdi does not own. Strict
// decode still enforces the field is present as a JSON array; it simply
// does not constrain elements this package has no evidence for.
type Graph struct {
	Stamp            string             `json:"stamp,omitempty"`
	Tool             string             `json:"tool,omitempty"`
	Algo             string             `json:"algo,omitempty"`
	Caveats          []string           `json:"caveats,omitempty"`
	Nodes            []Node             `json:"nodes,omitempty"`
	Edges            []Edge             `json:"edges,omitempty"`
	BlindSpots       []json.RawMessage  `json:"blind_spots,omitempty"`
	Entrypoints      []GraphEntrypoint  `json:"entrypoints,omitempty"`
	Entrypoint       *string            `json:"entrypoint,omitempty"`
	CompositionRoots []string           `json:"composition_roots,omitempty"`
	OmittedPackages  []string           `json:"omitted_packages,omitempty"`
	EffectOrder      []EffectOrderEntry `json:"effect_order,omitempty"`
	Obligations      []Obligation       `json:"obligations,omitempty"`
	Frontier         *Frontier          `json:"frontier,omitempty"`
	Annotations      []Annotation       `json:"annotations,omitempty"`
}

// DecodeGraph strict-decodes `flowmap graph`'s stdout JSON (DisallowUnknownFields
// + trailing-data rejection) and validates every obligation's status enum.
func DecodeGraph(data []byte) (*Graph, error) {
	var g Graph
	if err := artifact.DecodeStrictJSON(data, &g); err != nil {
		return nil, fmt.Errorf("upstream: decoding graph JSON: %w", err)
	}
	for i, o := range g.Obligations {
		if err := o.Validate(); err != nil {
			return nil, fmt.Errorf("upstream: graph JSON: obligations[%d]: %w", i, err)
		}
	}
	return &g, nil
}
