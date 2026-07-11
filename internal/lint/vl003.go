package lint

import (
	"fmt"
	"regexp"

	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/gitx"
	"github.com/OWNER/verdi/internal/store"
)

// vl003 enforces "all link refs resolve — verdi refs against the committed
// zone, svc/... external refs against discovery, and evidence-for bindings
// in discovered verdi.bindings.yaml sidecars against the named spec's ACs;
// pins name real commits" (02 §Lint rules).
type vl003 struct{}

func (vl003) ID() string { return "VL-003" }

var externalRefShapeRe = regexp.MustCompile(`^svc/`)

func (r vl003) Check(in *RunInput) []Finding {
	var findings []Finding
	externalRefs := externalRefSet(in.Snapshot.Services)

	for _, d := range in.Snapshot.Docs {
		if d.Grandfathered || d.DecodeErr != nil {
			continue
		}
		for _, l := range d.Base.Links {
			if l.Type == artifact.LinkStory {
				continue // tracker ref, not a corpus ref (02 §External refs scope)
			}
			if !r.refResolves(l.Ref, in.Snapshot, externalRefs) {
				findings = append(findings, Finding{Rule: "VL-003", Path: d.RelPath, Message: fmt.Sprintf("links[].ref %q does not resolve", l.Ref)})
			}
		}

		if d.Spec != nil {
			for _, ctxRef := range d.Spec.Context {
				findings = append(findings, r.checkPin(in, d.RelPath, "context[]", ctxRef)...)
			}
		}
	}

	for _, b := range in.Snapshot.Boards {
		if b.DecodeErr != nil || b.Board == nil {
			continue
		}
		for _, p := range b.Board.Pins {
			findings = append(findings, r.checkPin(in, b.RelPath, "pins[].ref", p.Ref)...)
		}
	}

	findings = append(findings, r.checkBindings(in)...)

	return findings
}

// refResolves reports whether ref (an unpinned link target) resolves
// against either the committed zone or a discovered external ref.
func (vl003) refResolves(ref string, snap *Snapshot, externalRefs map[string]bool) bool {
	if externalRefShapeRe.MatchString(ref) {
		return externalRefs[ref]
	}
	_, ok := snap.ByRef[ref]
	return ok
}

// checkPin validates a pinned ref (kind/name@commit): the unpinned kind/name
// half must resolve in the committed zone, and the commit half must name
// real history.
func (r vl003) checkPin(in *RunInput, path, field, pinned string) []Finding {
	var findings []Finding
	ref, err := artifact.ParsePinnedRef(pinned)
	if err != nil {
		findings = append(findings, Finding{Rule: "VL-003", Path: path, Message: fmt.Sprintf("%s %q is not a well-formed pinned ref: %v", field, pinned, err)})
		return findings
	}
	unpinned := artifact.Ref{Kind: ref.Kind, Name: ref.Name}.String()
	if _, ok := in.Snapshot.ByRef[unpinned]; !ok {
		findings = append(findings, Finding{Rule: "VL-003", Path: path, Message: fmt.Sprintf("%s %q names %q, which does not resolve in the committed zone", field, pinned, unpinned)})
	}
	ok, err := gitx.CommitExists(in.Ctx, in.Root, ref.Commit)
	if err != nil {
		findings = append(findings, Finding{Rule: "VL-003", Path: path, Message: fmt.Sprintf("%s %q: checking commit %s: %v", field, pinned, ref.Commit, err)})
	} else if !ok {
		findings = append(findings, Finding{Rule: "VL-003", Path: path, Message: fmt.Sprintf("%s %q pins commit %s, which is not real git history", field, pinned, ref.Commit)})
	}
	return findings
}

// checkBindings validates every discovered verdi.bindings.yaml sidecar's
// evidence-for join: its `spec:` must resolve to a spec in the committed
// zone, and every AC id its bindings name must be declared by that spec.
func (vl003) checkBindings(in *RunInput) []Finding {
	var findings []Finding
	for _, svc := range in.Snapshot.Services {
		if svc.Bindings == nil {
			continue
		}
		bindingsPath := "verdi.bindings.yaml (" + svc.Name + ")"

		d, ok := in.Snapshot.ByRef[svc.Bindings.Spec]
		if !ok || len(d) == 0 || d[0].Spec == nil {
			findings = append(findings, Finding{Rule: "VL-003", Path: bindingsPath, Message: fmt.Sprintf("spec %q does not resolve to a spec in the committed zone", svc.Bindings.Spec)})
			continue
		}
		acs := make(map[string]bool, len(d[0].Spec.AcceptanceCriteria))
		for _, ac := range d[0].Spec.AcceptanceCriteria {
			acs[ac.ID] = true
		}
		for _, b := range svc.Bindings.Bindings {
			for _, ac := range b.ACs {
				if !acs[ac] {
					findings = append(findings, Finding{Rule: "VL-003", Path: bindingsPath, Message: fmt.Sprintf("evidence-for binding %q names ac %q, which %q does not declare", b.Producer, ac, svc.Bindings.Spec)})
				}
			}
		}
	}
	return findings
}

// externalRefSet computes every index-minted external ref
// (02 §External refs) discovery would mint for services, mirroring
// internal/index/external.go's minting scheme (duplicated rather than
// imported: that logic is unexported and reading the underlying files'
// content, which lint does not need).
func externalRefSet(services []store.Service) map[string]bool {
	refs := make(map[string]bool)
	for _, svc := range services {
		if svc.BoundaryContractPath != "" {
			refs[fmt.Sprintf("svc/%s/boundary-contract", svc.Name)] = true
		}
		for _, obligation := range svc.Obligations {
			refs[fmt.Sprintf("svc/%s/obligations/%s", svc.Name, obligation)] = true
		}
		if svc.OpenAPIPath != "" {
			refs[fmt.Sprintf("svc/%s/api", svc.Name)] = true
		}
	}
	return refs
}
