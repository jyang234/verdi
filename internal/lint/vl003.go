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
// half must resolve in the committed zone, and the commit half must be
// reachable from HEAD in this repository's history.
//
// spec/evidence-resilience ac-3 (X-11b): this used to check mere object
// existence (gitx.CommitExists), a predicate a locally-dangling object — one
// no branch or ref anywhere reaches — satisfies just as well as a genuinely
// pinned commit, so a pin could "look pinned to real history" while that
// history had already stopped being retained as reachable (and the old red's
// own claim, "not real git history", is precisely what a dangling object still
// passes). Tightened to gitx.ReachableFromHEAD — the same primitive VL-009's
// frozen.commit check now uses (internal/lint/vl009.go) — folding "does not
// exist at all" and "exists but unreachable" into the same honest false,
// closing X-11b's hole from VL-003's own git predicate too.
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
	ok, err := gitx.ReachableFromHEAD(in.Ctx, in.Root, ref.Commit, "HEAD")
	if err != nil {
		findings = append(findings, Finding{Rule: "VL-003", Path: path, Message: fmt.Sprintf("%s %q: checking commit %s: %v", field, pinned, ref.Commit, err)})
	} else if !ok {
		findings = append(findings, Finding{Rule: "VL-003", Path: path, Message: fmt.Sprintf("%s %q pins commit %s, which is not reachable from HEAD in this repository's history", field, pinned, ref.Commit)})
	}
	return findings
}

// checkBindings validates every discovered verdi.bindings.yaml sidecar's
// evidence-for join: its `spec:` must resolve to a spec in the committed
// zone, and every AC id its bindings name must be declared by that spec.
// Beyond the Services loop's own discovered sidecars, the module root's own
// verdi.bindings.yaml is root-discovered directly (spec/ritual-traps ac-3):
// a Service is only ever discovered from a directory containing
// .flowmap.yaml, and per D6-4 this repository deliberately has none at its
// module root, so without this second, unconditional check the root file —
// the very file this design series' stories append fragment-qualified
// entries to — would stay invisible to checkBindings forever (chronicle
// P2-3(b)).
func (r vl003) checkBindings(in *RunInput) []Finding {
	var findings []Finding
	rootDiscoveredAsService := false
	for _, svc := range in.Snapshot.Services {
		if svc.Dir == in.Root {
			rootDiscoveredAsService = true
		}
		if svc.Bindings == nil {
			continue
		}
		findings = append(findings, r.checkOneBindingsFile(in, "verdi.bindings.yaml ("+svc.Name+")", svc.Bindings)...)
	}

	// Root-discovery: skipped only when some discovered Service is ALREADY
	// rooted exactly at the module root (a .flowmap.yaml there would make
	// this store a flowmap service of itself — never true for this
	// repository per D6-4, but guarding against it keeps a hypothetical
	// store that does from having its one root bindings file double-checked
	// under two different labels).
	if !rootDiscoveredAsService {
		switch {
		case in.Snapshot.RootBindingsErr != nil:
			findings = append(findings, Finding{Rule: "VL-003", Path: rootBindingsDisplayPath, Message: fmt.Sprintf("does not decode: %v", in.Snapshot.RootBindingsErr)})
		case in.Snapshot.RootBindings != nil:
			findings = append(findings, r.checkOneBindingsFile(in, rootBindingsDisplayPath, in.Snapshot.RootBindings)...)
		}
	}

	return findings
}

// rootBindingsDisplayPath is the finding Path label for the module root's
// own verdi.bindings.yaml (spec/ritual-traps ac-3) — distinguished from a
// discovered Service's own "verdi.bindings.yaml (<service>)" label.
const rootBindingsDisplayPath = "verdi.bindings.yaml (root)"

// checkOneBindingsFile validates one decoded Bindings artifact's
// evidence-for join against the committed zone: `spec:` (the file's own
// primary/owning spec) must resolve, and every AC id every binding names
// must be declared — a bare ac-<slug> entry against the owning spec's own
// declared criteria, a fragment-qualified spec/<name>#<ac-id> entry against
// the NAMED spec's own declared criteria instead (spec/ritual-traps ac-4;
// artifact.ResolveBindingAC is the same resolution helper
// cmd/verdi/selfevidence.go's self-hosted producer already uses for this
// exact bare-vs-fragment distinction). path is the display label naming
// which discovered/root bindings file a finding belongs to.
func (vl003) checkOneBindingsFile(in *RunInput, path string, bindings *artifact.Bindings) []Finding {
	var findings []Finding

	if _, ok := in.Snapshot.ByRef[bindings.Spec]; !ok {
		findings = append(findings, Finding{Rule: "VL-003", Path: path, Message: fmt.Sprintf("spec %q does not resolve to a spec in the committed zone", bindings.Spec)})
		return findings
	}

	for _, b := range bindings.Bindings {
		for _, entry := range b.ACs {
			specRef, acID, err := artifact.ResolveBindingAC(bindings.Spec, entry)
			if err != nil {
				// Binding.Validate() (decode time) already shape-checks every
				// entry as a bare ac-<slug> id or a spec/<name>#<ac-id>
				// fragment ref, and bindings.Spec itself is confirmed above —
				// unreachable in practice, but fail closed rather than
				// silently skip if it somehow still does.
				findings = append(findings, Finding{Rule: "VL-003", Path: path, Message: fmt.Sprintf("evidence-for binding %q: ac entry %q: %v", b.Producer, entry, err)})
				continue
			}
			acs, resolved := targetACSet(in, specRef)
			if !resolved {
				findings = append(findings, Finding{Rule: "VL-003", Path: path, Message: fmt.Sprintf("evidence-for binding %q names ac %q, whose target spec %q does not resolve to a spec in the committed zone", b.Producer, entry, specRef)})
				continue
			}
			if !acs[acID] {
				findings = append(findings, Finding{Rule: "VL-003", Path: path, Message: fmt.Sprintf("evidence-for binding %q names ac %q, which %q does not declare", b.Producer, entry, specRef)})
			}
		}
	}
	return findings
}

// targetACSet resolves specRef against the committed zone and returns its
// declared AC-id set; resolved is false when specRef does not resolve to a
// decoded spec at all (a nil map is never returned alongside resolved ==
// true, even for a spec declaring zero ACs, so callers can trust the bool
// alone).
func targetACSet(in *RunInput, specRef string) (acs map[string]bool, resolved bool) {
	d, ok := in.Snapshot.ByRef[specRef]
	if !ok || len(d) == 0 || d[0].Spec == nil {
		return nil, false
	}
	acs = make(map[string]bool, len(d[0].Spec.AcceptanceCriteria))
	for _, ac := range d[0].Spec.AcceptanceCriteria {
		acs[ac.ID] = true
	}
	return acs, true
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
