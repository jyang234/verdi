package dex

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/index"
)

// meta is the render-fidelity metadata dex needs beyond index.Entry's
// generic view: owners, frozen/provenance stamps, and every feature-spec-
// only field (class, story, declared boundaries, acceptance criteria, the
// I-5 dispositions block). index.Entry deliberately stays generic (one
// struct for every committed-zone kind); dex re-decodes each artifact's
// frontmatter through the same internal/artifact seam to get the typed
// view its page anatomy renders.
type meta struct {
	Base   artifact.Base
	Status string

	// Feature-spec-only fields (02 §feature-spec frontmatter additions).
	Class              artifact.SpecClass
	Story              string
	Impacts            []string
	Context            []string
	Declares           *artifact.Declares
	AcceptanceCriteria []artifact.AcceptanceCriterion
	Dispositions       []artifact.Disposition

	// ADR-only field.
	Decided string

	// Waiver-only fields.
	Reason string
	Expiry string
}

// artifactPage pairs one committed-zone index.Entry with its decoded meta —
// everything dex's page anatomy (breadcrumb, title/badge, temporal banner,
// metadata card, body, connections, TOC) needs for one page.
type artifactPage struct {
	Entry   *index.Entry
	Meta    meta
	RelPath string // root-relative, slash-separated source path
}

// loadArtifactPages decodes full metadata for every committed-zone entry
// (i.e. every indexed entry except Kind == "external", which carries no
// frontmatter of its own) in ix, sorted by ref for deterministic
// iteration.
func loadArtifactPages(root string, ix *index.Index) ([]*artifactPage, error) {
	var pages []*artifactPage
	for _, e := range ix.All() {
		if e.Kind == "external" {
			continue
		}
		data, err := os.ReadFile(e.Path)
		if err != nil {
			return nil, fmt.Errorf("dex: reading %s: %w", e.Path, err)
		}
		fm, _, err := artifact.SplitFrontmatter(data)
		if err != nil {
			return nil, fmt.Errorf("dex: %s: %w", e.Path, err)
		}
		m, err := decodeMeta(e.Kind, fm)
		if err != nil {
			return nil, fmt.Errorf("dex: %s: %w", e.Path, err)
		}
		rel, err := filepath.Rel(root, e.Path)
		if err != nil {
			return nil, fmt.Errorf("dex: %s: %w", e.Path, err)
		}
		pages = append(pages, &artifactPage{Entry: e, Meta: m, RelPath: filepath.ToSlash(rel)})
	}
	sort.Slice(pages, func(i, j int) bool { return pages[i].Entry.Ref < pages[j].Entry.Ref })
	return pages, nil
}

// decodeMeta dispatches to internal/artifact's typed decoder for kind and
// projects the result into meta's kind-agnostic-plus-extras shape.
func decodeMeta(kind string, fm []byte) (meta, error) {
	switch kind {
	case "spec":
		s, err := artifact.DecodeSpec(fm)
		if err != nil {
			return meta{}, err
		}
		return meta{
			Base: s.Base, Status: string(s.Status),
			Class: s.Class, Story: s.Story, Impacts: s.Impacts, Context: s.Context,
			Declares: s.Declares, AcceptanceCriteria: s.AcceptanceCriteria, Dispositions: s.Dispositions,
		}, nil

	case "adr":
		a, err := artifact.DecodeADR(fm)
		if err != nil {
			return meta{}, err
		}
		return meta{Base: a.Base, Status: string(a.Status), Decided: a.Decided}, nil

	case "diagram":
		d, err := artifact.DecodeDiagram(fm)
		if err != nil {
			return meta{}, err
		}
		return meta{Base: d.Base, Status: string(d.Status)}, nil

	case "attestation":
		at, err := artifact.DecodeAttestation(fm)
		if err != nil {
			return meta{}, err
		}
		return meta{Base: at.Base}, nil

	case "waiver":
		w, err := artifact.DecodeWaiver(fm)
		if err != nil {
			return meta{}, err
		}
		return meta{Base: w.Base, Status: string(w.Status), Reason: w.Reason, Expiry: w.Expiry}, nil

	case "conflict":
		c, err := artifact.DecodeConflict(fm)
		if err != nil {
			return meta{}, err
		}
		return meta{Base: c.Base, Status: string(c.Status)}, nil

	default:
		return meta{}, fmt.Errorf("dex: unreachable: unhandled kind %q", kind)
	}
}
