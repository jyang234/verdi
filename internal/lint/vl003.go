package lint

import (
	"fmt"
	"regexp"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/store"
)

// vl003 enforces "all link refs resolve — verdi refs against the committed
// zone, svc/... external refs against discovery, and evidence-for bindings
// in discovered verdi.bindings.yaml sidecars against the named spec's ACs;
// pins name real commits; object-id fragments (#<object-id>) resolve
// against the target's parsed frontmatter objects, and their edge types are
// the closed five-value enum — unknown types fail closed" (02 §Lint rules,
// as amended, R4-I-3). internal/lint's walk deliberately decodes every
// document via artifact.DecodeStrict only, never the kind's own semantic
// Validate() (doc.go's design note: every semantic check is re-implemented
// by its own VL-xxx rule rather than centralized) — so this rule is the
// sole place that both an unknown link type at all, and a known type
// outside the closed five-value vocabulary targeting a fragment, are
// caught, by calling artifact.Link.Validate() itself per link. This rule's
// other new work (R4-I-3) is resolving a fragment's object id against the
// target spec's declared objects.
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
		// The spec's own top-level links: (implements/supersedes/etc, 02
		// §Object model — "belong to the spec itself") name no single
		// declared object, so a dangling one is a SPEC-LEVEL wall locus
		// (badge-computes dc-3) — the case file, not any one card.
		for _, l := range d.Base.Links {
			findings = append(findings, locusAll(r.checkLink(l, d.RelPath, "links[].ref", in.Snapshot, externalRefs), SpecLocus())...)
		}

		if d.Spec != nil {
			// context[] refs are likewise the spec's own declared surface,
			// not any one object's — spec-level.
			for _, ctxRef := range d.Spec.Context {
				findings = append(findings, locusAll(r.checkPin(in, d.RelPath, "context[]", ctxRef), SpecLocus())...)
			}
			// A decision's own links[] DO name a single rendered card (that
			// decision's own) — a dangling one badges exactly that card
			// (badge-computes dc-3's "dangling link refs" object-anchored
			// bucket).
			for _, dc := range d.Spec.Decisions {
				for _, l := range dc.Links {
					findings = append(findings, locusAll(r.checkLink(l, d.RelPath, fmt.Sprintf("decisions[%s].links[].ref", dc.ID), in.Snapshot, externalRefs), ObjectLocus(dc.ID))...)
				}
			}
		}
	}

	// Board pins (the v0 board.json artifact, a separate superseded
	// mechanism) and the cross-spec verdi.bindings.yaml join below declare
	// NO wall locus at all: neither names a single object this spec's own
	// wall renders, and both are store-structural/cross-file plumbing in
	// exactly wall-receipts dc-3's third bucket's sense — they stay in
	// `verdi lint`/CI, off the wall, fail-closed by omission.
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

// checkLink resolves a single link's ref: first, l.Validate() itself —
// covering both an unknown link type outright and, per R4-I-3, a known
// type outside the closed five-value spec-object edge vocabulary
// (implements/resolves/supersedes/exempts/depends-on) targeting a
// fragment (02 §Link taxonomy) — since internal/lint's walk deliberately
// decodes via artifact.DecodeStrict only, never the kind's own Validate()
// (see doc.go's design note: every semantic check is re-implemented by its
// own VL-xxx rule rather than centralized), nothing else in this engine
// would ever catch either case. Then: a story link is a tracker ref, not a
// corpus ref (02 §External refs scope), and is skipped; an svc/... external
// ref is checked against discovery only (fragments are not modeled for the
// provisional external-ref form, 02 §Identity: "External refs
// (provisional)"); every other ref is checked against the committed zone
// by its unpinned kind/name half, and, when it carries an object-id
// fragment (§Identity and references), the fragment is additionally
// resolved against the target's parsed frontmatter objects (§Object
// model).
func (vl003) checkLink(l artifact.Link, path, field string, snap *Snapshot, externalRefs map[string]bool) []Finding {
	if err := l.Validate(); err != nil {
		return []Finding{{Rule: "VL-003", Path: path, Message: fmt.Sprintf("%s %q: %v", field, l.Ref, err)}}
	}
	if l.Type == artifact.LinkStory {
		return nil
	}
	if externalRefShapeRe.MatchString(l.Ref) {
		if !externalRefs[l.Ref] {
			return []Finding{{Rule: "VL-003", Path: path, Message: fmt.Sprintf("%s %q does not resolve", field, l.Ref)}}
		}
		return nil
	}

	ref, err := artifact.ParseRef(l.Ref)
	if err != nil {
		// Decode already validated every link's ref shape (VL-001's scope);
		// this is unreachable in practice, but fail closed rather than
		// panicking if it ever isn't.
		return []Finding{{Rule: "VL-003", Path: path, Message: fmt.Sprintf("%s %q: %v", field, l.Ref, err)}}
	}
	unpinned := artifact.Ref{Kind: ref.Kind, Name: ref.Name}.String()
	targets, ok := snap.ByRef[unpinned]
	if !ok || len(targets) == 0 {
		return []Finding{{Rule: "VL-003", Path: path, Message: fmt.Sprintf("%s %q does not resolve", field, l.Ref)}}
	}

	if !ref.Fragment() {
		return nil
	}
	target := targets[0]
	if target.Spec == nil || !artifact.DeclaredObjectIDs(target.Spec)[ref.Object] {
		return []Finding{{Rule: "VL-003", Path: path, Message: fmt.Sprintf("%s %q fragment #%s does not resolve against %s's declared objects", field, l.Ref, ref.Object, unpinned)}}
	}
	return nil
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
