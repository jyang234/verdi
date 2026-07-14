package evidence

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
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
