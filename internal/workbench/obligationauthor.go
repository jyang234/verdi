package workbench

// Obligation authoring on the board (spec/obligation-artifact ac-3): a
// scratch sticky graduates into an evidence-obligation artifact, bound to the
// STORY acceptance criterion its yarn was dropped on. This is the story
// wall's counterpart to the feature wall's scoping canvas (a proto-sticky
// graduating into a stub): both reuse the same sticky-graduate write ritual —
// the sticky's handwriting becomes a real record and its annotation flips to
// graduated (boardio.GraduateStickies) — but where a stub is spliced into the
// spec document, an obligation is a first-class markdown FILE under
// .verdi/obligations/, decoded through the single internal/artifact seam
// exactly like the attestation it mirrors. This story authors it only; the
// activation gate (feature ac-2) and the wall/matrix render (feature ac-4)
// are separate downstream stories (spec/obligation-artifact co-2).

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/boardio"
	"github.com/jyang234/verdi/internal/boardlayout"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/model"
)

// obligationGraduatePrefix marks a sticky-graduate request whose destination
// is an evidence-obligation artifact rather than a declared spec object: the
// board posts kind "obligation:<for-kind>" (e.g. "obligation:behavioral") —
// the evidence kind the for_kind picker chose. The story AC the obligation
// binds to travels in the request's `ref` field (the AC card the sticky's
// yarn was dropped on). Reusing the existing sticky-graduate action, rather
// than inventing a second write path, keeps one graduation ritual (05
// §Workbench).
const obligationGraduatePrefix = "obligation:"

// actionObligationGraduate authors an obligation from a sticky dropped on a
// story AC (spec/obligation-artifact ac-3). It seeds the obligation's
// `verifies` edge (→ the WHOLE story spec, no fragment — the AC lives in the
// id and path, mirroring an attestation) and its for_kind, writes the
// markdown file to .verdi/obligations/<story-slug>/<ac-id>--<for-kind>.md
// (DC-2), and flips the sticky to graduated through the same
// boardio.GraduateStickies machinery sticky- and stub-graduate use. Every
// refusal is plain-language and fail-closed: no malformed obligation is ever
// written.
func (s *boardSpecServer) actionObligationGraduate(ctx context.Context, name string, proj *BoardProjection, req boardAPIRequest) error {
	// The evidence kind the obligation states its AC's evidence must show
	// for (DC-1), posted as kind "obligation:<for-kind>". An unknown kind
	// fails closed here, before anything is written.
	forKind := artifact.EvidenceKind(strings.TrimPrefix(req.Kind, obligationGraduatePrefix))
	switch forKind {
	case artifact.EvidenceStatic, artifact.EvidenceBehavioral, artifact.EvidenceRuntime, artifact.EvidenceAttestation:
	default:
		return fmt.Errorf("obligation for_kind %q is not a known evidence kind (one of static, behavioral, runtime, attestation); fail closed", forKind)
	}

	// Obligations attach to STORY acceptance criteria only (ac-2/DC-3; 03
	// §The feature fold's feature-blind / story-scoped split) — the story
	// wall's counterpart to the feature wall's stub graduation. A feature
	// (or any non-story) wall refuses in plain language, the mirror of the
	// scoping canvas's proto-stickies being feature-class only.
	if proj.Class != string(artifact.ClassStory) {
		// The spoken class words are display and resolve (L-M13a(6)); the
		// class COMPARISON stays on bare ids.
		storyWord := s.model.DisplayClass("story")
		return fmt.Errorf("an obligation attaches to %s acceptance criterion, but this wall is class %s — obligations are %s-only (03 §The feature fold)", model.Indefinite(storyWord), s.model.DisplayClass(proj.Class), storyWord)
	}

	var sticky *scratchStickyView
	for i := range proj.Stickies {
		if proj.Stickies[i].ID == req.ID {
			sticky = &proj.Stickies[i]
			break
		}
	}
	if sticky == nil {
		return fmt.Errorf("no sticky %q on this board", req.ID)
	}

	// The story AC the sticky's yarn was dropped on (req.ref): it must be a
	// declared acceptance-criterion card on THIS wall. A decision, a
	// constraint, an open question, a reference card — anything that is not
	// an AC — is refused, naming the offending target (the D6-18 lesson:
	// never a silent absence; never a malformed obligation).
	acID := req.Ref
	if declaredKindsOf(proj)[acID] != string(boardlayout.ZoneAC) {
		// The class word is display and resolves (L-M13a(6)); the echoed
		// target id is identity.
		return fmt.Errorf("an obligation binds to %s acceptance criterion, but %q is not a declared AC on this wall", model.Indefinite(s.model.DisplayClass("story")), acID)
	}

	// The (story, ac, for-kind) triple has two views the artifact requires
	// to agree (DC-2): the id obligation/<story-slug>--<ac-id>--<for-kind>
	// and the on-disk path. Both derive from the same three parts here. The
	// story slug is this wall's own spec name; the verifies edge names the
	// WHOLE story spec (no fragment), exactly as an attestation's does.
	obID := "obligation/" + name + "--" + acID + "--" + string(forKind)
	verifiesRef := "spec/" + name
	title := firstLine(sticky.Body)

	head, err := gitx.RevParse(ctx, s.root, "HEAD")
	if err != nil {
		return err
	}
	// L-M4: at is HEAD's own committer date, never wall clock — the same
	// commit-derived stamp align/freeze.go's callers use (cmd/verdi's
	// align.go/accept.go), shared here via gitx.CommitDateOnly rather than
	// re-deriving wall-clock "now" as this file used to.
	at, err := gitx.CommitDateOnly(ctx, s.root, head)
	if err != nil {
		return err
	}
	frozen := artifact.NewFrozen(at, head)

	content := evidence.RenderObligation(evidence.ObligationInput{
		ID: obID, Title: title, ForKind: forKind, VerifiesRef: verifiesRef,
		Body: sticky.Body, Owners: []string{annotationAuthor()}, Frozen: frozen,
	})

	// On-disk home .verdi/obligations/<story-slug>/<ac-id>--<for-kind>.md
	// (DC-2). Refuse an already-authored obligation rather than overwrite
	// one — the same fail-closed posture stub-graduate wears on a slug
	// collision. This board-side existence check is this call site's own
	// policy, layered above evidence.WriteObligationFile's unconditional
	// write (spec/obligation-seam ac-4/O-5: the shared seam does the
	// render + pre-write self-validate + atomic write; whether to refuse
	// on an existing file is each caller's own decision — `verdi
	// obligation author`, cmd/verdi, makes a different one for its own
	// pre-freeze regenerate case).
	dir := filepath.Join(s.root, ".verdi", "obligations", name)
	fileName := acID + "--" + string(forKind) + ".md"
	path := filepath.Join(dir, fileName)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("an obligation for %s's %s evidence already exists (.verdi/obligations/%s/%s) — nothing written", acID, forKind, name, fileName)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("workbench: checking obligation path %s: %w", path, err)
	}
	if err := evidence.WriteObligationFile(path, content); err != nil {
		return err
	}

	// The handwriting becomes the record: the sticky flips to graduated in
	// the mutable stream (the same GraduateStickies machinery sticky- and
	// stub-graduate use — one graduation ritual, not a second).
	_, err = boardio.GraduateStickies(boardio.AnnotationsDir(s.root), []string{req.ID})
	return err
}

// renderObligation and writeObligationFile used to live here, hand-rendering
// and atomically writing an obligation's markdown content. Both are now
// internal/evidence.RenderObligation and internal/evidence.WriteObligationFile
// (spec/obligation-seam ac-4/O-5): the ONE shared seam accept's freeze-moment
// backstop, `verdi obligation author`, and this board action all three call,
// so no second render/write implementation exists to drift from this one.

// firstLine returns s's first non-empty line, trimmed — the obligation's
// one-line title (the `title:` field and the body's `# ` heading), derived
// from the sticky's handwriting. A sticky's text is required non-empty at
// creation (actionSticky), so this is never "".
func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}
