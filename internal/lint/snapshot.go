package lint

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/store"
)

// Board is a committed board.json found under a spec directory, alongside
// its decode outcome — VL-014 needs both the successfully-decoded board
// (to cross-check against dispositions) and a clean signal when decode
// itself failed.
type Board struct {
	// SpecDir is the RelPath of the spec directory this board.json sits in
	// (e.g. ".verdi/specs/active/stale-decline").
	SpecDir string
	// RelPath is board.json's own store-relative path.
	RelPath   string
	Board     *artifact.Board
	DecodeErr error
}

// Layout is a committed layout.json sidecar found under a spec directory
// (01 §Directory layout, 02 §Record schemas "Board layout"), alongside its
// decode outcome — VL-018 needs both the successfully-decoded layout (to
// resolve its positions keys against the sibling spec's declared objects)
// and a clean signal when decode itself failed.
type Layout struct {
	// SpecDir is the RelPath of the spec directory this layout.json sits in
	// (e.g. ".verdi/specs/active/accepted-pending-build").
	SpecDir string
	// RelPath is layout.json's own store-relative path.
	RelPath   string
	Layout    *artifact.BoardLayout
	DecodeErr error
}

// Snapshot is everything the rules read: every committed-zone
// document (decoded or not), every committed board.json, the repo-root
// .gitattributes, the store manifest, and discovered services. Building a
// Snapshot never fails on a single bad artifact file — only on an
// operational problem (can't read .verdi/ at all, can't discover
// services). Per-file decode failures live in Document.DecodeErr /
// Board.DecodeErr instead, so every rule still runs over everything else.
type Snapshot struct {
	Root string

	Docs            []*Document
	TopLevelEntries []string
	Boards          []*Board
	Layouts         []*Layout

	// ByRef indexes decoded documents (DecodeErr == nil, id parses) by
	// their frontmatter id — VL-002's global-uniqueness check.
	ByRef map[string][]*Document

	GitAttributes    []byte // repo-root .gitattributes content, nil if absent
	GitAttributesErr error  // non-nil only for a real read error (not "absent")

	Manifest    *store.Manifest
	ManifestErr error

	Services []store.Service
}

// BuildSnapshot walks root and assembles a Snapshot.
func BuildSnapshot(root string, opts Options) (*Snapshot, error) {
	docs, err := walkDocuments(root, opts)
	if err != nil {
		return nil, err
	}
	tops, err := topLevelEntries(root)
	if err != nil {
		return nil, err
	}
	boards, err := walkBoards(root)
	if err != nil {
		return nil, err
	}
	layouts, err := walkLayouts(root)
	if err != nil {
		return nil, err
	}
	services, err := store.DiscoverServices(root)
	if err != nil {
		return nil, fmt.Errorf("lint: discovering services: %w", err)
	}

	snap := &Snapshot{
		Root:            root,
		Docs:            docs,
		TopLevelEntries: tops,
		Boards:          boards,
		Layouts:         layouts,
		ByRef:           make(map[string][]*Document),
		Services:        services,
	}

	for _, d := range docs {
		if d.DecodeErr != nil || d.Base.ID == "" {
			continue
		}
		snap.ByRef[d.Base.ID] = append(snap.ByRef[d.Base.ID], d)
	}

	gaPath := filepath.Join(root, ".gitattributes")
	if data, err := os.ReadFile(gaPath); err == nil {
		snap.GitAttributes = data
	} else if !os.IsNotExist(err) {
		snap.GitAttributesErr = err
	}

	manifestPath := filepath.Join(root, ".verdi", "verdi.yaml")
	if data, err := os.ReadFile(manifestPath); err == nil {
		m, decErr := store.DecodeManifest(data)
		if decErr != nil {
			snap.ManifestErr = decErr
		} else {
			snap.Manifest = m
		}
	} else if !os.IsNotExist(err) {
		snap.ManifestErr = err
	}

	return snap, nil
}

// walkBoards finds every specs/*/*/board.json (both active and archive) and
// tolerantly decodes each.
func walkBoards(root string) ([]*Board, error) {
	var boards []*Board
	for _, statusDir := range []string{"active", "archive"} {
		base := filepath.Join(root, ".verdi", "specs", statusDir)
		entries, err := os.ReadDir(base)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("lint: reading %s: %w", base, err)
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			boardPath := filepath.Join(base, e.Name(), "board.json")
			data, err := os.ReadFile(boardPath)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return nil, fmt.Errorf("lint: reading %s: %w", boardPath, err)
			}
			specDir := fmt.Sprintf(".verdi/specs/%s/%s", statusDir, e.Name())
			b := &Board{SpecDir: specDir, RelPath: specDir + "/board.json"}
			if bv, err := artifact.DecodeBoard(data); err != nil {
				b.DecodeErr = err
			} else {
				b.Board = bv
			}
			boards = append(boards, b)
		}
	}
	return boards, nil
}

// walkLayouts finds every specs/*/*/layout.json (both active and archive)
// and tolerantly decodes each.
func walkLayouts(root string) ([]*Layout, error) {
	var layouts []*Layout
	for _, statusDir := range []string{"active", "archive"} {
		base := filepath.Join(root, ".verdi", "specs", statusDir)
		entries, err := os.ReadDir(base)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("lint: reading %s: %w", base, err)
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			layoutPath := filepath.Join(base, e.Name(), "layout.json")
			data, err := os.ReadFile(layoutPath)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return nil, fmt.Errorf("lint: reading %s: %w", layoutPath, err)
			}
			specDir := fmt.Sprintf(".verdi/specs/%s/%s", statusDir, e.Name())
			l := &Layout{SpecDir: specDir, RelPath: specDir + "/layout.json"}
			if lv, err := artifact.DecodeBoardLayout(data); err != nil {
				l.DecodeErr = err
			} else {
				l.Layout = lv
			}
			layouts = append(layouts, l)
		}
	}
	return layouts, nil
}

// specDirOf returns a spec Document's containing directory, store-relative
// (e.g. ".verdi/specs/active/stale-decline" for
// ".verdi/specs/active/stale-decline/spec.md").
func specDirOf(doc *Document) string {
	return strings.TrimSuffix(doc.RelPath, "/spec.md")
}
