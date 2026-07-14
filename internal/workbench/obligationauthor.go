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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/boardio"
	"github.com/jyang234/verdi/internal/boardlayout"
	"github.com/jyang234/verdi/internal/gitx"
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
		return fmt.Errorf("an obligation attaches to a story acceptance criterion, but this wall is class %s — obligations are story-only (03 §The feature fold)", proj.Class)
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
		return fmt.Errorf("an obligation binds to a story acceptance criterion, but %q is not a declared AC on this wall", acID)
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
	frozen := artifact.Frozen{At: time.Now().UTC().Format("2006-01-02"), Commit: head}

	content := renderObligation(obID, title, string(forKind), verifiesRef, sticky.Body, []string{annotationAuthor()}, frozen)

	// Self-validate before touching disk (CLAUDE.md: never fake success —
	// the same pre-write posture stub-instantiate wears): the rendered
	// obligation must split and strict-decode through the artifact seam, or
	// the graduation refuses rather than writing a record that would not
	// round-trip.
	fm, _, err := artifact.SplitFrontmatter([]byte(content))
	if err != nil {
		return fmt.Errorf("workbench: internal error: obligation scaffold failed self-validation: %w", err)
	}
	if _, err := artifact.DecodeObligation(fm); err != nil {
		return fmt.Errorf("workbench: internal error: obligation scaffold failed self-validation: %w", err)
	}

	// On-disk home .verdi/obligations/<story-slug>/<ac-id>--<for-kind>.md
	// (DC-2). Refuse an already-authored obligation rather than overwrite
	// one — the same fail-closed posture stub-graduate wears on a slug
	// collision.
	dir := filepath.Join(s.root, ".verdi", "obligations", name)
	fileName := acID + "--" + string(forKind) + ".md"
	path := filepath.Join(dir, fileName)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("an obligation for %s's %s evidence already exists (.verdi/obligations/%s/%s) — nothing written", acID, forKind, name, fileName)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("workbench: checking obligation path %s: %w", path, err)
	}
	if err := writeObligationFile(dir, path, []byte(content)); err != nil {
		return err
	}

	// The handwriting becomes the record: the sticky flips to graduated in
	// the mutable stream (the same GraduateStickies machinery sticky- and
	// stub-graduate use — one graduation ritual, not a second).
	_, err = boardio.GraduateStickies(boardio.AnnotationsDir(s.root), []string{req.ID})
	return err
}

// renderObligation hand-renders an obligation artifact's full markdown
// content (frontmatter + prose body) — the inverse of
// artifact.DecodeObligation. Frontmatter is hand-rendered, never
// yaml.Marshal'd (the module-wide posture: internal/align/render.go,
// internal/artifact/splice/doc.go's "never decode→struct→yaml.Marshal→
// reassemble"), so field order and the restricted flow-mapping style match
// the obligation fixtures exactly. The single `verifies` edge names the WHOLE
// story spec (DC-1): the AC is carried by the id and path, not the edge.
func renderObligation(id, title, forKind, verifiesRef, body string, owners []string, frozen artifact.Frozen) string {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "id: %s\n", id)
	b.WriteString("kind: obligation\n")
	fmt.Fprintf(&b, "title: %s\n", yamlDoubleQuote(title))
	quotedOwners := make([]string, len(owners))
	for i, o := range owners {
		quotedOwners[i] = yamlDoubleQuote(o)
	}
	fmt.Fprintf(&b, "owners: [%s]\n", strings.Join(quotedOwners, ", "))
	fmt.Fprintf(&b, "for_kind: %s\n", forKind)
	b.WriteString("links:\n")
	fmt.Fprintf(&b, "  - { type: verifies, ref: %q }\n", verifiesRef)
	fmt.Fprintf(&b, "frozen: { at: %s, commit: %s }\n", frozen.At, frozen.Commit)
	b.WriteString("---\n")
	fmt.Fprintf(&b, "# %s\n\n%s\n", title, body)
	return b.String()
}

// writeObligationFile writes the obligation atomically (temp-then-rename in
// the destination directory), mirroring spliceSpec's and GraduateStickies'
// own write discipline so a crash mid-write never leaves a half-written
// artifact in the committed zone.
func writeObligationFile(dir, path string, data []byte) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("workbench: creating %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".obligation-*.md")
	if err != nil {
		return fmt.Errorf("workbench: temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("workbench: writing %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("workbench: closing %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("workbench: replacing %s: %w", path, err)
	}
	return nil
}

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

// yamlDoubleQuote renders s as a YAML double-quoted scalar via encoding/json,
// whose string escaping is a valid subset of YAML's — the same safe,
// hand-render quoting internal/align/render.go's yamlDQ uses, so a title
// carrying a quote or colon lands in the frontmatter without a second
// hand-rolled escaper.
func yamlDoubleQuote(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return `""`
	}
	return string(b)
}
