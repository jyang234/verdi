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
// — with placeholder data, then strict-decodes the result exactly like a
// real scaffold consumer's decode would. A template that fails to render,
// or that renders content failing strict decode, fails model check
// closed — exit 2, grouped with every other "undecodable manifest"
// condition above, never exit 1 (a broken TEMPLATE is not a structural
// model deviation; Class.Template is frontier-EXEMPT, internal/model/
// model.go's own Class doc comment) — naming the specific template file
// at fault, never a bare "model.yaml invalid" message.
package main

import (
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/designscaffold"
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

// modelCheckPlaceholderData is the ScaffoldData every class's resolved
// template is instantiated with during checkTemplates (spec/scaffold-
// templates ac-3): representative enough to satisfy every kernel
// validation rule a real scaffold consumer's own decode would also
// enforce — notably validateStory's >=1 implements edge requirement for a
// non-spike story, which the canonical/embedded story.md template renders
// unconditionally — without being mistaken for a real spec (its own ref
// and title say so).
func modelCheckPlaceholderData() designscaffold.ScaffoldData {
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

// checkTemplates instantiates and strict-decodes every class's resolved
// template (store override or embedded canonical, designscaffold.
// LoadTemplate) with placeholder data, exactly like a real scaffold
// consumer's decode would (spec/scaffold-templates ac-3) — a broken
// template fails closed here, naming the offending file, rather than
// surfacing for the first time as a confusing decode error at someone's
// first design start. Classes are checked in sorted-name order so a
// multi-class failure names a deterministic first offender across runs
// (CLAUDE.md: deterministic outputs), though today's frontier (dc-1)
// permits only the canonical {feature, story} class set.
func checkTemplates(cfg *store.Config) error {
	names := make([]string, 0, len(cfg.Model.Classes))
	for name := range cfg.Model.Classes {
		names = append(names, name)
	}
	sort.Strings(names)

	data := modelCheckPlaceholderData()
	for _, name := range names {
		class := cfg.Model.Classes[name]
		tmpl, err := designscaffold.LoadTemplate(cfg.Root, class.Template)
		if err != nil {
			return fmt.Errorf("template %s (class %s): %w", class.Template, name, err)
		}
		content, err := designscaffold.Render(tmpl, data)
		if err != nil {
			return fmt.Errorf("template %s (class %s) failed to render: %w", class.Template, name, err)
		}
		fm, _, err := artifact.SplitFrontmatter([]byte(content))
		if err != nil {
			return fmt.Errorf("template %s (class %s) rendered content failed strict decode: %w", class.Template, name, err)
		}
		if _, err := artifact.DecodeSpec(fm); err != nil {
			return fmt.Errorf("template %s (class %s) rendered content failed strict decode: %w", class.Template, name, err)
		}
	}
	return nil
}
