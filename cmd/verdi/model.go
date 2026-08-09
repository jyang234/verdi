// verdi model check (L-M1, spec/model-schema ac-3): validates a store's
// operating model — `.verdi/model.yaml` if present, else the embedded
// canonical default (model.Canonical()) — giving it the same fail-closed
// feedback `verdi lint` already gives a spec.
//
// Exit discipline (CLAUDE.md's 0/1/2 contract), per spec/model-schema
// ac-3 verbatim (frozen in both the story spec and its own obligation
// doc): 0 with an OK line naming the schema, class/transition counts,
// and the resolved model's digest, on any valid manifest — including the
// absent-model.yaml case (resolves to the canonical default) and a valid
// hand-written model.yaml (vocabulary/template changes only, dc-1's
// frontier); 1 with the ONE pinned frontier error text, printed
// verbatim, on a structurally deviant manifest — model.ErrFrontier is
// the sole condition that earns exit 1; 2 on operational trouble: a
// missing store, or an unreadable OR UNDECODABLE manifest — which
// explicitly includes a KERNEL VALIDATION rule violation (a bad
// scheme/kind, a missing obligations list, and so on), since ac-3's own
// text groups those with "undecodable," not with the frontier's exit 1.
// This plan's own Task 7 prose ("exit 1 on validation/frontier failure")
// reads more broadly than that frozen text; spec+obligation win per this
// build's precedence rule (reported in the phase report as a disclosed
// conflict) — implemented here exactly as ac-3 states it.
//
// spec/scaffold-templates ac-3 extends this check with the one genuinely
// new surface that story adds: for every class the resolved model
// declares, model check also instantiates the class's own resolved
// template — a store override under .verdi/templates/ when one exists,
// the embedded canonical default otherwise (designscaffold.LoadTemplate)
// — with placeholder data for every variant it can render, then
// strict-decodes each exactly like a real scaffold consumer's decode
// would. A template that fails to render,
// or that renders content failing strict decode, fails model check
// closed — exit 2, grouped with every other "undecodable manifest"
// condition above, never exit 1 (a broken TEMPLATE is not a structural
// model deviation; Class.Template is frontier-EXEMPT, internal/model/
// model.go's own Class doc comment) — naming the specific template file
// at fault, never a bare "model.yaml invalid" message.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/designscaffold"
	"github.com/jyang234/verdi/internal/humanartifact"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/store"
)

// runModelVerb dispatches `verdi model <subcommand>`. There is exactly
// one subcommand, `check` (mirroring runBuildVerb's single-subcommand
// shape for `verdi build start`); anything else is a usage error.
func runModelVerb(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "check" {
		fmt.Fprintln(stderr, "usage: verdi model check")
		return 2
	}
	return cmdModelCheck(args[1:], stdout, stderr)
}

// cmdModelCheck is `verdi model check`'s real entry point: it resolves
// the store root and delegates to the testable core, runModelCheck.
func cmdModelCheck(args []string, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintf(stderr, "model check: unexpected extra argument %q\n", args[0])
		return 2
	}

	root, err := store.FindRoot(".")
	if err != nil {
		fmt.Fprintln(stderr, "model check:", err)
		return 2
	}
	return runModelCheck(root, stdout, stderr)
}

// runModelCheck is the testable core: given an already-resolved store
// root, resolves the store's full config (store.Open — verdi.yaml AND,
// Task 6, model.yaml in the same bottleneck) and reports on its Model.
func runModelCheck(root string, stdout, stderr io.Writer) int {
	cfg, err := store.Open(root)
	if err != nil {
		if errors.Is(err, model.ErrFrontier) {
			fmt.Fprintln(stderr, "model check:", err)
			return 1
		}
		fmt.Fprintln(stderr, "model check:", err)
		return 2
	}

	if err := checkTemplates(cfg); err != nil {
		fmt.Fprintln(stderr, "model check:", err)
		return 2
	}

	if err := checkPolicyScaffolds(cfg.Root); err != nil {
		fmt.Fprintln(stderr, "model check:", err)
		return 2
	}

	digest, err := cfg.Model.Digest()
	if err != nil {
		fmt.Fprintln(stderr, "model check:", err)
		return 2
	}

	transitions := 0
	for _, lc := range cfg.Model.Lifecycle {
		transitions += len(lc.Transitions)
	}

	fmt.Fprintf(stdout, "model: OK — %s, %d classes, %d transitions, digest %s\n",
		cfg.Model.Schema, len(cfg.Model.Classes), transitions, digest)
	return 0
}

// modelCheckVariant is one (StoryRef, Spike, links) shape a class's resolved
// template can render into. checkTemplates round-trips every variant a real
// scaffold consumer can produce, not just one (judged-spike-variant-
// unchecked-by-model-check, judged-model-check-feature-no-storyref-variant-
// unchecked): a template broken only inside a {{if .Spike}} or {{if .StoryRef}}
// branch must fail model check, not surface at that variant's first scaffold.
// name labels the variant so a failure names it alongside the template file.
type modelCheckVariant struct {
	name string
	data designscaffold.ScaffoldData
}

// modelCheckDefaultData is the non-spike ScaffoldData every class's resolved
// template is round-tripped with (spec/scaffold-templates ac-3):
// representative enough to satisfy every kernel validation rule a real
// scaffold consumer's own decode would also enforce — notably validateStory's
// >=1 implements edge requirement for a non-spike story, which the canonical/
// embedded story.md template renders unconditionally — without being mistaken
// for a real spec (its own ref and title say so).
func modelCheckDefaultData() designscaffold.ScaffoldData {
	return designscaffold.ScaffoldData{
		Ref:      "spec/model-check-placeholder",
		Title:    "Model Check Placeholder",
		Owners:   "[unassigned]",
		Problem:  "model check placeholder problem",
		Outcome:  "model check placeholder outcome",
		StoryRef: "todo:MODEL-CHECK-PLACEHOLDER",
		Links:    []designscaffold.StoryLink{{Type: artifact.LinkImplements, Ref: "spec/model-check-placeholder#ac-1"}},
	}
}

// modelCheckSpikeData is the spike ScaffoldData the story template's
// {{if .Spike}} branch is round-tripped with: Spike true, and a resolves
// edge to an open-question fragment in place of the implements edge — a
// spike story must carry no implements edge and >=1 resolves edge
// (validateStory), so the spike render decodes exactly as a real
// stub-instantiate of a spike stub would.
func modelCheckSpikeData() designscaffold.ScaffoldData {
	d := modelCheckDefaultData()
	d.Spike = true
	d.Links = []designscaffold.StoryLink{{Type: artifact.LinkResolves, Ref: "spec/model-check-placeholder#oq-1"}}
	return d
}

// modelCheckFeatureNoStoryRefData is the no-story-ref feature ScaffoldData
// the feature template's {{if .StoryRef}} EMPTY branch is round-tripped with:
// StoryRef "" (the story: line is optional for the feature class, 05 §CLI),
// so the render decodes exactly as a real ref-less `design start --kind
// feature` would. The (feature-ignored) implements link stays as-is;
// feature.md references neither .Links nor .Spike, so only StoryRef distinguishes
// this variant from the with-story-ref one.
func modelCheckFeatureNoStoryRefData() designscaffold.ScaffoldData {
	d := modelCheckDefaultData()
	d.StoryRef = ""
	return d
}

// modelCheckVariantsFor returns the template variants checkTemplates must
// round-trip for the class named className. The consumer-producible variant
// matrix is FINITE and enumerated here in full — a variant exists exactly
// when a real scaffold consumer (design start, stub-instantiate) can request
// a shape AND a template branches on it:
//
//	feature × {with-story-ref, no-story-ref}
//	    feature.md branches on {{if .StoryRef}} (the story: line is optional
//	    for the feature class); `design start --kind feature` renders BOTH — a
//	    tracker ref passes a non-empty storyRef, a ref-less start passes ""
//	    (05 §CLI, runDesignStart).
//	story × {plain, spike}
//	    story.md branches on {{if .Spike}} / {{if not .Spike}}; design start
//	    renders the plain story, stub-instantiate renders the spike variant
//	    from a spike stub (spike: true, a resolves edge, no implements edge —
//	    validateStory).
//
// This is the COMPLETE matrix: {{if .StoryRef}} and {{if .Spike}} are the
// only conditional branches the two embedded templates carry, and
// with/without-ref and plain/spike are the only shapes the two consumers can
// request — so every variant a broken store override could hide a defect in
// is round-tripped here, never surfacing first at a scaffold consumer's use
// (spec/scaffold-templates ac-3). Adding a new conditional branch to a
// template, or a new consumer-selectable shape, OBLIGATES a new entry here:
// this switch is the single place that obligation is discharged, so a
// silently-widened gap shows up as a missing case rather than as an
// unchecked branch. Keyed on the canonical class names (artifact.ClassFeature/
// ClassStory — the map key stays canonical across vocabulary renames, only
// Display renames, internal/model/model.go), matching stub-instantiate's own
// Classes[string(artifact.ClassStory)] lookup (internal/workbench); any other
// class the frontier might one day admit round-trips at least the default
// shape until its own spec round extends this matrix.
func modelCheckVariantsFor(className string) []modelCheckVariant {
	switch className {
	case string(artifact.ClassFeature):
		return []modelCheckVariant{
			{name: "with-story-ref", data: modelCheckDefaultData()},
			{name: "no-story-ref", data: modelCheckFeatureNoStoryRefData()},
		}
	case string(artifact.ClassStory):
		return []modelCheckVariant{
			{name: "plain", data: modelCheckDefaultData()},
			{name: "spike", data: modelCheckSpikeData()},
		}
	default:
		return []modelCheckVariant{{name: "default", data: modelCheckDefaultData()}}
	}
}

// checkTemplates instantiates and strict-decodes every class's resolved
// template (store override or embedded canonical, designscaffold.
// LoadTemplate) across every variant it can render — the non-spike scaffold
// every class produces, plus the story template's spike variant — exactly
// like a real scaffold consumer's decode would (spec/scaffold-templates
// ac-3). A broken template fails closed here, naming the offending file AND
// the offending variant, rather than surfacing for the first time as a
// confusing decode error at someone's first design start or spike
// stub-instantiate. Classes are checked in sorted-name order, default
// variant before spike, so a multi-failure names a deterministic first
// offender across runs (CLAUDE.md: deterministic outputs), though today's
// frontier (dc-1) permits only the canonical {feature, story} class set.
// Beyond strict-decoding, each render's own class: line must also agree
// with the class it was resolved under (designscaffold.CheckClass, K1): a
// class's Template filename is DATA, so a misconfigured model.yaml can
// bind one class's Template to another class's template file and still
// strict-decode clean as the OTHER class — this identity check is the
// only thing that catches it, at check time rather than at someone's
// first design start or stub-instantiate.
func checkTemplates(cfg *store.Config) error {
	names := make([]string, 0, len(cfg.Model.Classes))
	for name := range cfg.Model.Classes {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		class := cfg.Model.Classes[name]
		tmpl, err := designscaffold.LoadTemplate(cfg.Root, class.Template)
		if err != nil {
			return fmt.Errorf("template %s (class %s): %w", class.Template, name, err)
		}
		for _, v := range modelCheckVariantsFor(name) {
			content, err := designscaffold.Render(tmpl, v.data)
			if err != nil {
				return fmt.Errorf("template %s (class %s, %s variant) failed to render: %w", class.Template, name, v.name, err)
			}
			fm, _, err := artifact.SplitFrontmatter([]byte(content))
			if err != nil {
				return fmt.Errorf("template %s (class %s, %s variant) rendered content failed strict decode: %w", class.Template, name, v.name, err)
			}
			spec, err := artifact.DecodeSpec(fm)
			if err != nil {
				return fmt.Errorf("template %s (class %s, %s variant) rendered content failed strict decode: %w", class.Template, name, v.name, err)
			}
			// K1: the rendered content's OWN class must agree with the
			// class it was resolved under (name) — a class's Template is
			// DATA (model.Class.Template), and a misconfigured model.yaml
			// can bind one class's Template to another class's template
			// file. That still strict-decodes clean (as the OTHER class),
			// so only this identity check catches it, at check time.
			if err := designscaffold.CheckClass(spec, artifact.SpecClass(name)); err != nil {
				return fmt.Errorf("template %s (class %s, %s variant): %w", class.Template, name, v.name, err)
			}
		}
	}
	return nil
}

// checkPolicyScaffolds round-trips every constitution scaffold —
// spec/context-integrity-v2 AC-1's three named human-authored
// constitution kinds (policies, overlays, exemptions), independent of
// the resolved model's own Classes (constitution kinds are not model-
// registered classes; SI-6 assigns their own directory grammar to
// internal/policyartifact) — a store override under .verdi/templates/
// when one exists, the embedded
// canonical default otherwise (humanartifact.ResolveScaffold, the SAME
// resolver checkTemplates' own designscaffold.LoadTemplate mirrors for
// the spec-store classes) — through humanartifact.RenderPolicy/
// RenderOverlay/RenderExemption's render/strict-decode/kernel-round-trip
// path, exactly like a real scaffold consumer's decode would (AC-1:
// "verdi model check renders and strict-decodes every configured
// template and proves parity across creation surfaces"). A failure
// (render error, strict-decode failure, or kernel round-trip mismatch)
// fails model check closed here, naming the offending template file —
// grouped with checkTemplates' own undecodable-manifest conditions
// above, never the frontier's exit 1 (a broken constitution scaffold is
// not a structural MODEL deviation any more than a broken class
// template is).
func checkPolicyScaffolds(root string) error {
	adopted, err := storeHasBegunPolicyAdoption(root)
	if err != nil {
		return err
	}
	if !adopted {
		return nil
	}
	if err := checkPolicyScaffold(root); err != nil {
		return err
	}
	if err := checkOverlayScaffold(root); err != nil {
		return err
	}
	return checkExemptionScaffold(root)
}

// storeHasBegunPolicyAdoption reports whether root has opted in to the
// constitution capability, which is exactly the existence of the
// .verdi/policy/ directory (internal/policyauthority.Load's own adoption
// signal: an absent .verdi/policy/ is ErrNotAdopted).
//
// The gate exists because adoption is PROSPECTIVE (DC-15): an unchanged
// legacy store's behavior is preserved, and adoption is opt-in. The
// constitution scaffolds resolve through humanartifact.ResolveScaffold,
// which prefers a store override at .verdi/templates/policy.md — a
// filename a pre-adoption store may legitimately carry for its own
// unrelated purposes. Checking it unconditionally would make an
// untouched legacy store start exiting 2 for a file whose meaning it
// never changed.
//
// A directory that exists but carries no constitution.md is INCOMPLETE
// adoption, not absent adoption: the store has opted in, so the checks
// run and still fail closed. An unreadable .verdi/policy is operational
// trouble, never an assumed absence (CO-1).
//
// A .verdi/policy that EXISTS but is not a directory is operational
// trouble too, never an assumed absence: internal/policyauthority.Load
// classifies that exact filesystem state as a hard error
// ("policyauthority: .verdi/policy exists but is not a directory"), and
// one state classified two ways across two store entry points is what
// CO-1's uniform fail-closed posture forbids. The wording is matched
// deliberately; the classification, not the code, is what is shared
// (cmd/verdi does not depend on internal/policyauthority).
func storeHasBegunPolicyAdoption(root string) (bool, error) {
	info, err := os.Stat(filepath.Join(root, ".verdi", "policy"))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("statting .verdi/policy: %w", err)
	}
	if !info.IsDir() {
		return false, errors.New(".verdi/policy exists but is not a directory")
	}
	return true, nil
}

// modelCheckPolicyPlaceholderDigest is a fixed, computed (never hand-
// typed) sha256:<64 hex> placeholder — the exemption scaffold's witness
// claim_digest needs a well-formed digest, and computing it here means
// its length can never silently drift from policyartifact's own grammar.
func modelCheckPolicyPlaceholderDigest() string {
	sum := sha256.Sum256([]byte("model-check-placeholder-claim"))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// checkPolicyScaffold round-trips policy.md: humanartifact.ResolveScaffold
// then humanartifact.RenderPolicy against fixed placeholder data
// (mirrors modelCheckDefaultData's own "representative enough, never
// mistaken for a real artifact" posture).
func checkPolicyScaffold(root string) error {
	const filename = "policy.md"
	scaffold, err := humanartifact.ResolveScaffold(root, filename)
	if err != nil {
		return fmt.Errorf("policy scaffold %s (kind policy): %w", filename, err)
	}
	data := humanartifact.PolicyScaffoldData{
		Name:             "model-check-placeholder",
		Title:            "Model Check Placeholder Policy",
		Owners:           []string{"model-check-placeholder-team"},
		TemplateIdentity: scaffold.Identity,
		TemplateDigest:   scaffold.Digest,
	}
	if _, err := humanartifact.RenderPolicy(scaffold, data); err != nil {
		return fmt.Errorf("policy scaffold %s (kind policy): %w", filename, err)
	}
	return nil
}

// checkOverlayScaffold round-trips policy-overlay.md.
func checkOverlayScaffold(root string) error {
	const filename = "policy-overlay.md"
	scaffold, err := humanartifact.ResolveScaffold(root, filename)
	if err != nil {
		return fmt.Errorf("policy scaffold %s (kind policy-overlay): %w", filename, err)
	}
	data := humanartifact.OverlayScaffoldData{
		Name:             "model-check-placeholder",
		Title:            "Model Check Placeholder Overlay",
		Owners:           []string{"model-check-placeholder-team"},
		RefinesPolicy:    "policy/model-check-placeholder",
		ClaimName:        "model-check-placeholder-claim",
		TemplateIdentity: scaffold.Identity,
		TemplateDigest:   scaffold.Digest,
	}
	if _, err := humanartifact.RenderOverlay(scaffold, data); err != nil {
		return fmt.Errorf("policy scaffold %s (kind policy-overlay): %w", filename, err)
	}
	return nil
}

// checkExemptionScaffold round-trips policy-exemption.md.
func checkExemptionScaffold(root string) error {
	const filename = "policy-exemption.md"
	scaffold, err := humanartifact.ResolveScaffold(root, filename)
	if err != nil {
		return fmt.Errorf("policy scaffold %s (kind policy-exemption): %w", filename, err)
	}
	data := humanartifact.ExemptionScaffoldData{
		Name:               "model-check-placeholder",
		Title:              "Model Check Placeholder Exemption",
		Owners:             []string{"model-check-placeholder-team"},
		WitnessPolicy:      "policy/model-check-placeholder",
		WitnessClaim:       "model-check-placeholder-claim",
		WitnessClaimDigest: modelCheckPolicyPlaceholderDigest(),
		ApprovalRole:       "policy-owner",
		ApprovalPrincipal:  "principal/github-org/YWxpY2U",
		Expiry:             "2099-12-31",
		TemplateIdentity:   scaffold.Identity,
		TemplateDigest:     scaffold.Digest,
	}
	if _, err := humanartifact.RenderExemption(scaffold, data); err != nil {
		return fmt.Errorf("policy scaffold %s (kind policy-exemption): %w", filename, err)
	}
	return nil
}
