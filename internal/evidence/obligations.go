package evidence

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/atomicfile"
)

// Obligation is what a caller needs to render one (AC, evidence-kind)
// pair's evidence-obligation artifact (spec/obligation-wall DC-1): enough
// to back both surfaces the one loader serves — `verdi matrix`'s
// title-only row (spec/obligation-wall ac-1) and the board AC card's
// title-plus-prose (ac-2, a Fable follow-on that consumes this same
// loader, per DC-1's "not two readers").
type Obligation struct {
	// Title is the obligation artifact's own `title:` frontmatter field.
	Title string
	// Body is the obligation's prose — the markdown body following the
	// frontmatter's closing "---" — trimmed of surrounding whitespace.
	Body string
}

// Obligations reads every evidence-obligation artifact on disk for the
// story spec named specName's acceptance criterion acID, keyed by for_kind
// (spec/obligation-wall DC-1: "a small loader ... returns an AC's
// obligations keyed by for_kind" — the one reader both `verdi matrix`
// (ac-1) and the board AC card (ac-2) consume, mirroring how
// AttestationExists loads attestations by path).
//
// specName is the spec's OWN directory name under specs/active/ (e.g.
// "obligation-wall" for spec/obligation-wall) — NOT the story's tracker
// slug AttestationExists/WaiverActive key by (store.RefSlug of the spec's
// `story:` field). DC-1 is explicit that obligations are loaded by
// (spec-name, ac-id), the same spec-name keying spec/obligation-artifact's
// on-disk convention settled and internal/workbench's obligation-author
// already writes to (its `dir := filepath.Join(s.root, ".verdi",
// "obligations", name)`, where name is the wall's own spec directory
// name) and internal/lint's VL-011/VL-020 already read.
//
// It scans .verdi/obligations/<specName>/ for files named
// "<acID>--*.md" — the on-disk home spec/obligation-artifact DC-2 fixes —
// strict-decoding each match through the internal/artifact seam
// (artifact.DecodeObligation) and keying the result by the decoded
// for_kind field (already validated internally consistent with the file's
// own id).
//
// A kind with no matching file is simply absent from the returned map:
// spec/obligation-wall DC-2's disclosure posture makes "no obligation yet"
// the ordinary case, never an error — an AC's evidence kind may be
// declared long before its obligation is authored on the wall. A missing
// .verdi/obligations/ tree entirely (or a missing specName subdirectory)
// reads the same honest way (evidence.LoadRecords's own "no derived data
// yet" posture: absence is not failure). Only a file that exists but
// fails strict decode — malformed frontmatter, a for_kind that disagrees
// with its own id, more than one verifies link, ... — is a surfaced
// error: a broken obligation is not "no obligation," and silently
// treating it as absent would hide a real authoring fault behind the same
// disclosure this function reserves for genuine absence.
func Obligations(storeRoot, specName, acID string) (map[artifact.EvidenceKind]Obligation, error) {
	dir := filepath.Join(storeRoot, ".verdi", "obligations", specName)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("evidence: reading %s: %w", dir, err)
	}

	prefix := acID + "--"
	var out map[artifact.EvidenceKind]Obligation
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".md") {
			continue
		}

		path := filepath.Join(dir, name)
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("evidence: reading obligation %s: %w", path, err)
		}
		fm, body, err := artifact.SplitFrontmatter(raw)
		if err != nil {
			return nil, fmt.Errorf("evidence: obligation %s: %w", path, err)
		}
		decoded, err := artifact.DecodeObligation(fm)
		if err != nil {
			return nil, fmt.Errorf("evidence: obligation %s: %w", path, err)
		}

		if out == nil {
			out = make(map[artifact.EvidenceKind]Obligation)
		}
		out[decoded.ForKind] = Obligation{Title: decoded.Title, Body: strings.TrimSpace(string(body))}
	}
	return out, nil
}

// UnauthoredObligationMarker is the fixed, exported sentinel `verdi
// obligation author` writes into a freshly created or regenerated
// obligation's body until an operator replaces it with a real statement of
// what the evidence must specifically show (spec/obligation-seam ac-5) —
// UnauthoredAttestationMarker's own sibling for obligations, defined here
// so the CLI writer (cmd/verdi) and any future fold/wall reader share one
// literal rather than a copy-pasted string.
const UnauthoredObligationMarker = "<!-- verdi:obligation-unauthored -->"

// ObligationInput bundles RenderObligation's inputs (spec/obligation-artifact
// DC-1, mirroring AttestationScaffold's own shape): every field here is
// structure the caller already resolved or derived — RenderObligation itself
// does no resolution and no I/O.
type ObligationInput struct {
	// ID is the obligation's own id: "obligation/<story-slug>--<ac-id>--
	// <for-kind>" (DC-2's id/for_kind/path agreement).
	ID string
	// Title is the obligation's one-line `title:` field and `# ` heading.
	Title string
	// ForKind is the one evidence kind this obligation states what that
	// evidence must specifically show for.
	ForKind artifact.EvidenceKind
	// VerifiesRef names the WHOLE story spec this obligation backs (e.g.
	// "spec/stale-decline") — no object fragment; the AC lives in ID and
	// the on-disk path, mirroring an attestation's own verifies edge.
	VerifiesRef string
	// Body is the obligation's prose, stating what ForKind evidence must
	// specifically show.
	Body string
	// Owners is the obligation's owners: list — never empty (Base.Validate
	// requires it).
	Owners []string
	// Frozen is the frozen stamp: obligations are frozen unconditionally
	// (DC-1: "existence is the record", mirroring an attestation).
	Frozen artifact.Frozen
}

// RenderObligation hand-renders an obligation artifact's full markdown
// content (frontmatter + prose body) — the inverse of
// artifact.DecodeObligation. Frontmatter is hand-rendered, never
// yaml.Marshal'd (the module-wide posture: internal/align/render.go,
// internal/artifact/splice/doc.go's "never decode→struct→yaml.Marshal→
// reassemble"), so field order and the restricted flow-mapping style match
// the obligation fixtures exactly. The single `verifies` edge names the
// WHOLE story spec (DC-1): the AC is carried by the id and path, not the
// edge.
//
// Extracted verbatim from internal/workbench/obligationauthor.go's
// unexported renderObligation (spec/obligation-seam ac-4/O-5): this is now
// the ONE seam the board's sticky-graduate action, accept's freeze-moment
// backstop (cmd/verdi), and `verdi obligation author` (cmd/verdi) all three
// call — never a second, independent render. Byte-for-byte identical to the
// pre-extraction function for the same inputs, proven by
// internal/workbench/obligationauthor_test.go's existing suite passing
// unmodified.
func RenderObligation(in ObligationInput) string {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "id: %s\n", in.ID)
	b.WriteString("kind: obligation\n")
	fmt.Fprintf(&b, "title: %s\n", artifact.YAMLDoubleQuote(in.Title))
	quotedOwners := make([]string, len(in.Owners))
	for i, o := range in.Owners {
		quotedOwners[i] = artifact.YAMLDoubleQuote(o)
	}
	fmt.Fprintf(&b, "owners: [%s]\n", strings.Join(quotedOwners, ", "))
	fmt.Fprintf(&b, "for_kind: %s\n", in.ForKind)
	b.WriteString("links:\n")
	fmt.Fprintf(&b, "  - { type: verifies, ref: %q }\n", in.VerifiesRef)
	fmt.Fprintf(&b, "frozen: { at: %s, commit: %s }\n", in.Frozen.At, in.Frozen.Commit)
	b.WriteString("---\n")
	fmt.Fprintf(&b, "# %s\n\n%s\n", in.Title, in.Body)
	return b.String()
}

// WriteObligationFile self-validates content — the exact split +
// artifact.DecodeObligation pre-write check every obligation writer shares
// (CLAUDE.md: "never fake success") — then writes it atomically via
// internal/atomicfile.Write (MkdirAll + CreateTemp + fsync + Rename-into-
// place). Every caller (the board's sticky-graduate action, accept's
// freeze-moment backstop, `verdi obligation author`) shares this one write
// path (spec/obligation-seam ac-4/O-5), so a malformed obligation can never
// reach disk regardless of which surface authored it, and no caller can
// skip the self-validate step. Unconditional: it creates or overwrites
// whatever is at path — callers that must never overwrite (the board) or
// must refuse on an already-frozen path (`verdi obligation author`) decide
// that policy before calling this, exactly as the board's own pre-existing
// os.Stat check already does today.
func WriteObligationFile(path, content string) error {
	fm, _, err := artifact.SplitFrontmatter([]byte(content))
	if err != nil {
		return fmt.Errorf("evidence: obligation scaffold failed self-validation: %w", err)
	}
	if _, err := artifact.DecodeObligation(fm); err != nil {
		return fmt.Errorf("evidence: obligation scaffold failed self-validation: %w", err)
	}
	if err := atomicfile.Write(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("evidence: %w", err)
	}
	return nil
}
