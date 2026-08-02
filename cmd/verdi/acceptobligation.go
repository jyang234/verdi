// Shared rendering/owner-resolution helpers for a scaffolded evidence
// obligation's placeholder body (spec/obligation-seam, spec/creation-
// surfaces#ac-4, ledger L-N8 §12 addendum). Historically these backed
// accept's own freeze-moment obligation backstop; Task 7 (docs/
// superpowers/specs/2026-08-01-merge-signals-spec-acceptance-design.md)
// retires that backstop along with every other accept-time mutation —
// obligation preparation moves entirely to the PRE-REVIEW `verdi
// obligation scaffold <story-ref>` surface (obligation.go), which now
// owns the creation core these helpers used to back (moved there in the
// same change) plus these two body-rendering helpers themselves. Unlike
// the old
// accept-time backstop's disclosure line, this prose is never routed
// through model.DisplayVerb: "obligation scaffold" is a CLI subcommand
// name, not a declared lifecycle-transition verb (internal/model's own
// validation restricts Vocabulary.Verbs keys to exactly those), so there
// is no rename to route.
//
// Kept in its own file per the lint.go/sync.go/matrix.go/dex.go/attest.go
// convention, so obligation.go's own diff for the scaffold subcommand
// stays focused on the creation loop itself.
package main

import (
	"fmt"
	"os"

	"github.com/jyang234/verdi/internal/artifact"
)

// fallbackOperatorOwner is the disclosed sentinel operatorOwner falls back
// to when $USER is unset — honest, greppable, and never mistaken for a
// real username. Mirrors internal/workbench/boardspecapi.go's own
// annotationAuthor()'s "board" fallback, scoped to obligation scaffolding's
// own domain instead of the board's.
const fallbackOperatorOwner = "unassigned-operator"

// operatorOwner names the operator running `verdi obligation scaffold` for
// a scaffolded obligation's owners: field. The OS user is honest
// attribution for who ran the scaffold; fallbackOperatorOwner covers a
// bare/CI environment with no USER set.
func operatorOwner() string {
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	return fallbackOperatorOwner
}

// obligationBackstopDisclosureLine renders the required disclosure: every
// scaffolded obligation's body carries this line verbatim so it is
// reviewed honestly as disclosed-as-unproven, never disguised as
// elaborated intent.
func obligationBackstopDisclosureLine() string {
	return "This obligation was scaffolded by `verdi obligation scaffold`; not elaborated."
}

// backstopObligationBody renders a scaffolded obligation's body: the
// required disclosure line (above), plain-language pointers at what to do
// next, and the acceptance criterion's own already-declared text — never a
// fabricated claim about what the evidence specifically shows, since the
// whole point of the disclosure is that nobody has said that yet.
func backstopObligationBody(specRef, acID string, kind artifact.EvidenceKind, acText string) string {
	return fmt.Sprintf(
		"%s It is a placeholder for %s's %s evidence, written by `verdi\n"+
			"obligation scaffold` because no obligation existed for this pair yet\n"+
			"(spec/creation-surfaces#ac-4). Replace this body with a first-person\n"+
			"statement of what that evidence must specifically show before relying\n"+
			"on it — by hand, or via `verdi obligation author %s %s %s` on this\n"+
			"same design branch, before this pull request merges.\n"+
			"The acceptance criterion's own declared text, for reference:\n\n%s\n",
		obligationBackstopDisclosureLine(), acID, kind, specRef, acID, kind, acText)
}
