package align

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/gitx"
	"github.com/OWNER/verdi/internal/store"
	"github.com/OWNER/verdi/internal/upstream"
)

// ComputedInput is Compute's input: an already-resolved store root, an
// injectable upstream.Runner (tests supply upstream.FakeRunner; production
// wires upstream.RealRunner, mirroring cmd/verdi/baseline.go and
// cmd/verdi/sync_regen.go), the build-head spec, and the build-head commit
// (`covers`).
type ComputedInput struct {
	Root   string
	Runner upstream.Runner
	Spec   *artifact.SpecFrontmatter
	Covers string
}

// ServiceBoundaryDiff is one impacted service's boundary-contract drift
// since spec acceptance: the acceptance-time (spec.Frozen.Commit) committed
// contract vs the freshly regenerated one at Covers (PLAN.md Phase 8: "the
// boundary diff vs the acceptance baseline where present"). Skipped is true
// when no committed contract exists for the service at the acceptance
// commit (e.g. a service the build itself introduced) — informational, not
// a finding requiring its own disposition (unlike the declares: diff below,
// this diff asserts nothing the spec declared; it is supporting evidence
// rendered alongside).
type ServiceBoundaryDiff struct {
	Service        string
	BaselineCommit string
	Entries        []upstream.BoundaryDiffEntry
	Skipped        bool
	SkipReason     string
}

// ComputedResult is Compute's output: the declares:-boundaries three-valued
// diff findings (kind: computed, one per declared boundary plus one per
// undeclared-but-present boundary discovered in a regenerated contract),
// the per-service acceptance-baseline boundary diff, and the impacted
// service names actually regenerated (for rendering and provenance.inputs).
type ComputedResult struct {
	Findings      []artifact.Finding
	BaselineDiffs []ServiceBoundaryDiff
	Impacted      []string
}

// Compute regenerates graph and boundary contract for spec's impacted
// services at Covers (PLAN.md Phase 8: "regenerate graph + boundary
// contract for impacted services at the build head") and computes the
// computed section's content.
//
// declares.boundaries mapping (a documented interpretation, flagged: no
// spike or spec fixture ever populated a boundary contract's
// published/consumed/external_dependencies arrays with real content — see
// internal/upstream/contract.go's own doc comment on NamedResource). A
// declared boundary `{from, to, via}` names a directed edge from service
// `from` to a resource named `to` of surface kind `via`. It HOLDS when
// `from`'s regenerated boundary contract contains a NamedResource entry
// {Name: to, Kind: via} in any of published/consumed/external_dependencies
// (the three arrays this package's own upstream.ComputeBoundaryDiff already
// treats uniformly by (name, kind) identity). Any NamedResource entry
// present in a regenerated contract with no matching declared boundary is
// UNDECLARED. A declared boundary whose `from` service is not among the
// spec's impacted, regenerable services is fail-closed VIOLATED (never
// silently skipped — CLAUDE.md constitution: "silence is never a pass").
func Compute(ctx context.Context, in ComputedInput) (*ComputedResult, error) {
	if in.Runner == nil {
		return nil, fmt.Errorf("align: Compute: no toolchain configured (verdi.yaml toolchain: block, I-4)")
	}
	if in.Spec == nil {
		return nil, fmt.Errorf("align: Compute: Spec is required")
	}
	if in.Root == "" {
		return nil, fmt.Errorf("align: Compute: Root must not be empty")
	}
	if in.Covers == "" {
		return nil, fmt.Errorf("align: Compute: Covers must not be empty")
	}

	services, err := store.DiscoverServices(in.Root)
	if err != nil {
		return nil, fmt.Errorf("align: discovering services: %w", err)
	}
	impacted := store.FilterImpacted(services, in.Spec.Impacts)

	contracts := make(map[string]*upstream.BoundaryContract, len(impacted))
	var baselineDiffs []ServiceBoundaryDiff
	impactedNames := make([]string, 0, len(impacted))

	for _, svc := range impacted {
		impactedNames = append(impactedNames, svc.Name)

		// The graph is regenerated for parity with "regenerate graph +
		// boundary contract" and to exercise/prove toolchain reachability;
		// v0's declares: diff is boundary-contract-only (03 §Alignment
		// report's "declares.boundaries", not obligations), so the decoded
		// graph itself is not otherwise consulted here.
		if _, err := upstream.RunGraph(ctx, in.Runner, svc.Dir, in.Covers); err != nil {
			return nil, fmt.Errorf("align: regenerating graph for %s: %w", svc.Name, err)
		}
		if err := upstream.BoundaryGenerate(ctx, in.Runner, svc.Dir); err != nil {
			return nil, fmt.Errorf("align: regenerating boundary contract for %s: %w", svc.Name, err)
		}
		contract, err := upstream.ReadBoundaryContract(filepath.Join(svc.Dir, store.BoundaryContractRelPath))
		if err != nil {
			return nil, fmt.Errorf("align: %s: %w", svc.Name, err)
		}
		contracts[svc.Name] = contract

		diff, err := baselineDiffFor(ctx, in.Root, svc, contract, in.Spec)
		if err != nil {
			return nil, err
		}
		baselineDiffs = append(baselineDiffs, diff)
	}

	var findings []artifact.Finding
	findings = append(findings, declaredBoundaryFindings(in.Spec, contracts)...)
	findings = append(findings, undeclaredBoundaryFindings(in.Spec, impacted, contracts)...)
	sort.Slice(findings, func(i, j int) bool { return findings[i].ID < findings[j].ID })

	sort.Strings(impactedNames)
	sort.Slice(baselineDiffs, func(i, j int) bool { return baselineDiffs[i].Service < baselineDiffs[j].Service })

	return &ComputedResult{Findings: findings, BaselineDiffs: baselineDiffs, Impacted: impactedNames}, nil
}

// declaredBoundaryFindings computes one Finding per spec.Declares.Boundaries
// entry: declared-and-holds or declared-and-violated (with witness folded
// into Finding.Text — the schema carries no separate witness field).
func declaredBoundaryFindings(spec *artifact.SpecFrontmatter, contracts map[string]*upstream.BoundaryContract) []artifact.Finding {
	if spec.Declares == nil {
		return nil
	}
	out := make([]artifact.Finding, 0, len(spec.Declares.Boundaries))
	for _, b := range spec.Declares.Boundaries {
		out = append(out, declaredBoundaryFinding(b, contracts))
	}
	return out
}

func declaredBoundaryFinding(b artifact.Boundary, contracts map[string]*upstream.BoundaryContract) artifact.Finding {
	id := boundaryFindingID(b.From, b.To, b.Via)
	contract, ok := contracts[b.From]
	if !ok {
		return artifact.Finding{
			ID:   id,
			Kind: artifact.FindingComputed,
			Text: fmt.Sprintf("declared boundary %s->%s (%s): VIOLATED — service %q is not among the spec's impacted services with a regenerated boundary contract", b.From, b.To, b.Via, b.From),
		}
	}
	if boundaryHolds(contract, b.To, b.Via) {
		return artifact.Finding{
			ID:   id,
			Kind: artifact.FindingComputed,
			Text: fmt.Sprintf("declared boundary %s->%s (%s) holds (found in %s's regenerated boundary contract)", b.From, b.To, b.Via, b.From),
		}
	}
	return artifact.Finding{
		ID:   id,
		Kind: artifact.FindingComputed,
		Text: fmt.Sprintf("declared boundary %s->%s (%s): VIOLATED — not found among %s's regenerated published/consumed/external_dependencies", b.From, b.To, b.Via, b.From),
	}
}

// undeclaredBoundaryFindings computes one Finding per NamedResource entry
// present in an impacted service's regenerated contract that no declared
// boundary names (PLAN.md Phase 8: "undeclared").
func undeclaredBoundaryFindings(spec *artifact.SpecFrontmatter, impacted []store.Service, contracts map[string]*upstream.BoundaryContract) []artifact.Finding {
	declared := make(map[string]bool)
	if spec.Declares != nil {
		for _, b := range spec.Declares.Boundaries {
			declared[boundaryKey(b.From, b.To, b.Via)] = true
		}
	}

	var out []artifact.Finding
	seen := make(map[string]bool)
	for _, svc := range impacted {
		contract := contracts[svc.Name]
		if contract == nil {
			continue
		}
		for _, r := range allNamedResources(contract) {
			if r.Name == "" {
				continue
			}
			if declared[boundaryKey(svc.Name, r.Name, r.Kind)] {
				continue
			}
			id := boundaryFindingID(svc.Name, r.Name, r.Kind)
			if seen[id] {
				continue
			}
			seen[id] = true
			out = append(out, artifact.Finding{
				ID:   id,
				Kind: artifact.FindingComputed,
				Text: fmt.Sprintf("boundary %s->%s (%s): UNDECLARED — present in %s's regenerated boundary contract but not declared in the spec's declares.boundaries", svc.Name, r.Name, r.Kind, svc.Name),
			})
		}
	}
	return out
}

func boundaryHolds(c *upstream.BoundaryContract, to, via string) bool {
	for _, r := range allNamedResources(c) {
		if r.Name == to && r.Kind == via {
			return true
		}
	}
	return false
}

func allNamedResources(c *upstream.BoundaryContract) []upstream.NamedResource {
	out := make([]upstream.NamedResource, 0, len(c.Published)+len(c.Consumed)+len(c.ExternalDependencies))
	out = append(out, c.Published...)
	out = append(out, c.Consumed...)
	out = append(out, c.ExternalDependencies...)
	return out
}

func boundaryKey(from, to, via string) string { return from + "\x00" + to + "\x00" + via }

// boundaryFindingID derives a stable, human-legible finding id from a
// boundary's identity, reusing store.RefSlug (the module's one normative
// slugging rule, CLAUDE.md: don't invent a second one) rather than a
// content hash — see identity.go for why finding IDENTITY (used to
// preserve dispositions across regeneration) is not simply this ID.
func boundaryFindingID(from, to, via string) string {
	return "boundary-" + store.RefSlug(from) + "-" + store.RefSlug(to) + "-" + store.RefSlug(via)
}

// baselineDiffFor computes svc's boundary-contract drift since spec
// acceptance: the committed contract at spec.Frozen.Commit (git-native,
// always resolvable independent of any local derived/ bundle's lifecycle —
// see doc.go) vs branch, the freshly regenerated contract at the build
// head. "Where present" (03 §Alignment report / PLAN.md Phase 8): a
// service with no committed contract at the acceptance commit (e.g. a
// service the build itself introduces) is Skipped, not an error.
func baselineDiffFor(ctx context.Context, root string, svc store.Service, branch *upstream.BoundaryContract, spec *artifact.SpecFrontmatter) (ServiceBoundaryDiff, error) {
	out := ServiceBoundaryDiff{Service: svc.Name}
	if spec.Frozen == nil {
		out.Skipped = true
		out.SkipReason = "spec carries no frozen (acceptance) stamp"
		return out, nil
	}
	out.BaselineCommit = spec.Frozen.Commit

	relDir, err := filepath.Rel(root, svc.Dir)
	if err != nil {
		return out, fmt.Errorf("align: resolving %s relative to store root: %w", svc.Dir, err)
	}
	relPath := filepath.ToSlash(filepath.Join(relDir, store.BoundaryContractRelPath))

	raw, err := gitx.Show(ctx, root, spec.Frozen.Commit, relPath)
	if err != nil {
		out.Skipped = true
		out.SkipReason = fmt.Sprintf("no committed boundary contract for %s at acceptance commit %s", svc.Name, spec.Frozen.Commit)
		return out, nil
	}
	baseline, err := upstream.DecodeBoundaryContract(raw)
	if err != nil {
		return out, fmt.Errorf("align: decoding acceptance-baseline boundary contract for %s at %s: %w", svc.Name, spec.Frozen.Commit, err)
	}
	out.Entries = upstream.ComputeBoundaryDiff(baseline, branch)
	return out, nil
}
