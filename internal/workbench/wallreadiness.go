package workbench

// The wall's readiness facts beside its marks (spec/wall-strip-and-drawer-v2
// ac-4, ac-5, ac-7; ledger SI-368): the drawer's Readiness tab, served on
// demand from one readiness load and rendered by the readiness page's own
// body renderer in the wall's words, and each of its items' wall targets.

import (
	"context"
	"errors"
	"fmt"
	stdhtml "html"
	"net/http"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/boardlayout"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/readinesspilot"
)

// readinessTarget is one Readiness tab item's wall target (SI-368 (16)):
// what clicking the item selects on the wall. Kind is one of the
// readinessTarget* kinds below; Value is the object id, the stub slug,
// the strip half (problem or outcome), or the slot's object kind.
type readinessTarget struct {
	Kind  string
	Value string
}

// The Readiness tab items' target kinds (SI-368 (16)): an object's card,
// a stub's card, a half of the case-file strip, an object column's add
// slot, or none — a plain row that selects nothing.
const (
	readinessTargetObject = "object"
	readinessTargetStub   = "stub"
	readinessTargetStrip  = "strip"
	readinessTargetSlot   = "slot"
	readinessTargetNone   = "none"
)

// readinessWallWords is the wall's triad, which the drawer's Readiness
// tab keeps (SI-368 (14); spec-documents ac-12): the words the wall
// shell's state chips speak (asdPlainState), never the page's (SI-339
// (10)). The stepper's count words read as a count of items (SI-368
// (24)(h)): "1 needs attention", "2 need attention", "3 ready", "1
// without enough evidence yet".
func readinessWallWords() readinessWords {
	return readinessWords{
		proven: "Ready", violated: "Needs attention", unproven: "Not enough evidence yet",
		provenCount: "ready", violatedCount: "needs attention", unprovenCount: "without enough evidence yet",
		provenMany: "ready", violatedMany: "need attention", unprovenMany: "without enough evidence yet",
	}
}

// readinessTabSurface is the Readiness tab's surface: the wall's words,
// the tab's own scope (the page's stylesheet block reads
// readiness-standalone, which the tab does not carry), its purpose line,
// and each concern's wall target.
func readinessTabSurface(targets map[string]readinessTarget) readinessSurface {
	return readinessSurface{
		words:   readinessWallWords(),
		class:   "readiness-tab",
		testID:  "readiness-tab",
		purpose: "This tab derives readiness for this wall on every request.",
		wall:    true,
		targets: targets,
	}
}

// readinessTargets maps each concern of snap to its wall target (SI-368
// (16)), over the wall's object card ids and stub card slugs: a concern
// whose Object is a card on the wall selects that card; a stub concern
// selects the one stub card journey's slug rule maps onto it — the
// marks' rule (stubSlugsByConcern), so two stubs sharing the concern
// leave it selecting nothing rather than a guessed card (SI-362 (3));
// the problem and outcome rows select their half of the case-file strip
// where the wall draws that half (halves, caseStripHalves: a spec lacking
// a problem has no problem half, so its row selects nothing rather than
// shutting the drawer onto nothing) and the criteria row the criteria
// column's add slot. Every other concern has no entry: a plain row that
// selects nothing.
func readinessTargets(snap readinesspilot.Snapshot, objects map[string]bool, stubSlugs []string, halves map[string]bool) map[string]readinessTarget {
	stubs := stubSlugsByConcern(stubSlugs)
	targets := make(map[string]readinessTarget, len(snap.AllConcerns))
	for _, c := range snap.AllConcerns {
		switch {
		case c.Object != "" && objects[c.Object]:
			targets[c.ID] = readinessTarget{Kind: readinessTargetObject, Value: c.Object}
		case len(stubs[c.ID]) == 1:
			targets[c.ID] = readinessTarget{Kind: readinessTargetStub, Value: stubs[c.ID][0]}
		case c.ID == "shape/problem" && halves["problem"]:
			targets[c.ID] = readinessTarget{Kind: readinessTargetStrip, Value: "problem"}
		case c.ID == "shape/outcome" && halves["outcome"]:
			targets[c.ID] = readinessTarget{Kind: readinessTargetStrip, Value: "outcome"}
		case c.ID == "success/criteria":
			targets[c.ID] = readinessTarget{Kind: readinessTargetSlot, Value: string(boardlayout.ZoneAC)}
		}
	}
	return targets
}

// renderReadinessTab is the Readiness tab's body for one snapshot: the
// readiness page's body renderer on the tab's surface (SI-368 (1)).
func renderReadinessTab(mdl *model.Model, snap readinesspilot.Snapshot, targets map[string]readinessTarget) string {
	var b strings.Builder
	writeReadinessBody(&b, mdl, snap, readinessTabSurface(targets))
	return b.String()
}

// renderReadinessTabUnavailable is the Readiness tab's body when the
// wall's readiness cannot be read: the one reason, and no readiness fact
// (ac-5: the posture reason when a projection is unavailable).
func renderReadinessTabUnavailable(reason string) string {
	return `<div class="readiness-tab" data-testid="readiness-tab" data-readiness-unavailable="1"><p class="readiness-tab-unavailable" data-testid="readiness-unavailable" role="status">Readiness is unavailable: ` + stdhtml.EscapeString(strings.TrimSuffix(reason, ".")) + `.</p></div>`
}

// readinessTabUnreadable is the reason one readiness load's outcome
// cannot be the tab of the wall whose spec is ref, or "": the load
// failed, or its snapshot describes another spec (SI-362 (4)'s tripwire,
// failing closed). The snapshot's branch and HEAD are its own facts,
// stated in the tab's refs and derivation stamp.
func readinessTabUnreadable(ref string, snap readinesspilot.Snapshot, err error) string {
	if err != nil {
		return readinessLoadFailed(err)
	}
	if snap.TargetRef != ref {
		return fmt.Sprintf("the readiness snapshot describes %s, and this wall shows %s", orUnresolved(snap.TargetRef), ref)
	}
	return ""
}

// boardReadinessTabHandler answers GET /board/spec/{name}/readiness, and
// the same route beneath /b/{branch}: the drawer's Readiness tab body,
// loaded when the tab opens (SI-368 (1); co-1). On a wall whose marks are
// fixed for this server instance (instanceMarks: a /b/ wall whose branch
// is not the serving root's, or no loader wired) it answers that one
// reason without loading readiness (SI-360 (3); SI-364 (3)). Otherwise it
// is one application projection (openProjection: one read session, one
// accepted-HEAD resolution, which the load joins) around exactly one
// readiness load, rendered with the wall's targets from the working
// tree's own spec (Wave 6 §5.3). A load that cannot be read is the tab's
// unavailable body with its reason. GET only; it writes nothing.
func (s *boardSpecServer) boardReadinessTabHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name := r.PathValue("name")
		_, _, fm, err := s.readActiveSpec(name)
		if errors.Is(err, ErrBoardNotFound) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			renderError(r.Context(), w, s.root, http.StatusInternalServerError, err)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(s.readinessTab(r.Context(), name, fm))) // response body write; post-header error is unactionable
	}
}

// readinessTab is the Readiness tab's body for spec name, whose
// working-tree frontmatter is fm (boardReadinessTabHandler), followed by
// the policy setup guide when the wall's capabilities call for one — on
// a wall whose readiness is fixed as much as on one whose readiness loads
// (readinessTabGuide).
func (s *boardSpecServer) readinessTab(ctx context.Context, name string, fm *artifact.SpecFrontmatter) string {
	if fixed := s.instanceMarks(); fixed != nil {
		return renderReadinessTabUnavailable(fixed.Unavailable) + s.readinessTabGuide(ctx, name)
	}
	ctx, release := s.openProjection(ctx)
	defer release()
	guide := s.readinessTabGuide(ctx, name)
	ref := "spec/" + name
	snap, err := s.readinessLoader.Load(ctx, ref)
	if reason := readinessTabUnreadable(ref, snap, err); reason != "" {
		return renderReadinessTabUnavailable(reason) + guide
	}
	stubs := make([]string, 0, len(fm.Stubs))
	for _, st := range fm.Stubs {
		stubs = append(stubs, st.Slug)
	}
	return renderReadinessTab(s.model, snap, readinessTargets(snap, artifact.DeclaredObjectIDs(fm), stubs, caseStripHalves(fm))) + guide
}

// caseStripHalves is the case-file strip's halves the wall renders for
// fm: a half is on the wall exactly when its statement has text
// (buildProjection, writeCaseStrip), so a spec lacking a problem has no
// problem half to go to.
func caseStripHalves(fm *artifact.SpecFrontmatter) map[string]bool {
	return map[string]bool{
		"problem": fm.Problem != nil && fm.Problem.Text != "",
		"outcome": fm.Outcome != nil && fm.Outcome.Text != "",
	}
}

// readinessTabGuide is the policy setup guide the Readiness tab carries
// (SI-368 (3), (24)(f)): chosen by policyGuideFor from the capabilities
// consultation alone, never from readiness, so it needs no readiness
// load; "" when no design service is wired or no guide applies.
func (s *boardSpecServer) readinessTabGuide(ctx context.Context, name string) string {
	if s.design == nil {
		return ""
	}
	outcome, view := s.design.GetDesignCapabilities(ctx, s.root, "spec/"+name)
	return renderReadinessTabGuide(policyGuideFor(true, view, outcome.Failure))
}

// renderReadinessTabGuide renders the guide beside the tab's readiness,
// labelled as capabilities (SI-368 (3)): the inline, read-only guide the
// wall shell carried (writePolicySetupGuide), its test ids kept; "" for
// no guide.
func renderReadinessTabGuide(g policyGuide) string {
	if g.Kind == policyGuideNone {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<section class="readiness-tab-capabilities" data-testid="readiness-tab-capabilities" aria-label="Capabilities">`)
	b.WriteString(`<p class="readiness-eyebrow">Capabilities</p><p class="readiness-purpose">From the wall&#39;s design capabilities, not from its readiness.</p>`)
	writePolicySetupGuide(&b, g.Kind, g.Code, g.Detail)
	b.WriteString(`</section>`)
	return b.String()
}
