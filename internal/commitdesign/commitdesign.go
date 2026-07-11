package commitdesign

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/boardio"
	"github.com/OWNER/verdi/internal/canonjson"
	"github.com/OWNER/verdi/internal/gitx"
)

// storyRefShapeRe mirrors internal/artifact's own (unexported) story-ref
// shape check — duplicated rather than imported since that regex is
// private to package artifact; kept minimal and covered by this package's
// own tests against real artifact.DecodeSpec round-trips, so a drift
// between the two would fail loudly (a spec written with a story: value
// this regex accepted but artifact.DecodeSpec rejects self-validates as
// an internal error below, never a silently-written invalid spec).
var storyRefShapeRe = regexp.MustCompile(`^[a-z][a-z0-9]*:[A-Za-z0-9][A-Za-z0-9-]*$`)

// specNameRe is 02 §Identity's kebab-case name shape.
var specNameRe = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// Input is Run's request: everything the ritual needs to locate the board
// and place the new draft spec.
type Input struct {
	// Root is the store root.
	Root string
	// BoardKey is the board's own key (data/mutable/boards/<BoardKey>.json).
	BoardKey string
	// SpecName is the new draft spec's directory name under
	// specs/active/ (kebab-case, must not already exist).
	SpecName string
	// StoryRef is the scheme:key value the new spec's `story:` field
	// carries. Optional: when empty, Run accepts BoardKey itself if (and
	// only if) it already has scheme:key shape (see package doc).
	StoryRef string
}

// Result is what the ritual produced.
type Result struct {
	SpecRef      string // e.g. "spec/my-new-feature"
	SpecRelPath  string // e.g. ".verdi/specs/active/my-new-feature/spec.md"
	BoardRelPath string // e.g. ".verdi/specs/active/my-new-feature/board.json"
	Dispositions []artifact.Disposition
	Commit       string // the new commit sha
}

// Run performs the mechanical half of commit-to-design end to end: reads
// the board state, writes the draft spec skeleton + frozen board.json
// snapshot + dispositions block, graduates every dispositioned sticky's
// annotation record in the mutable stream, and commits everything to the
// CURRENT branch (the ritual runs on the design branch; Run itself does
// not cut one — that is `verdi design start`'s job, already done before
// a board accumulates anything worth committing).
func Run(ctx context.Context, in Input) (*Result, error) {
	if in.Root == "" {
		return nil, fmt.Errorf("commitdesign: Root is required")
	}
	if !boardio.ValidStoryKey(in.BoardKey) {
		return nil, fmt.Errorf("commitdesign: %q is not a valid board key", in.BoardKey)
	}
	if !specNameRe.MatchString(in.SpecName) {
		return nil, fmt.Errorf("commitdesign: spec name %q must be kebab-case", in.SpecName)
	}

	storyRef := in.StoryRef
	if storyRef == "" {
		if !storyRefShapeRe.MatchString(in.BoardKey) {
			return nil, fmt.Errorf("commitdesign: no story ref given, and board key %q is not itself scheme:key-shaped; pass StoryRef explicitly", in.BoardKey)
		}
		storyRef = in.BoardKey
	}
	if !storyRefShapeRe.MatchString(storyRef) {
		return nil, fmt.Errorf("commitdesign: story ref %q must be scheme:key form (e.g. jira:LOAN-1482)", storyRef)
	}

	specDir := filepath.Join(in.Root, ".verdi", "specs", "active", in.SpecName)
	if _, statErr := os.Stat(specDir); statErr == nil {
		return nil, fmt.Errorf("commitdesign: %s already exists", specDir)
	}

	boardPath, err := boardio.BoardStatePath(in.Root, in.BoardKey)
	if err != nil {
		return nil, fmt.Errorf("commitdesign: %w", err)
	}
	board, err := boardio.LoadBoardState(boardPath)
	if err != nil {
		return nil, fmt.Errorf("commitdesign: loading board %s: %w", in.BoardKey, err)
	}

	preCommit, err := gitx.RevParse(ctx, in.Root, "HEAD")
	if err != nil {
		return nil, fmt.Errorf("commitdesign: %w", err)
	}
	at := time.Now().UTC().Format("2006-01-02")
	relBoardPath, err := filepath.Rel(in.Root, boardPath)
	if err != nil {
		return nil, fmt.Errorf("commitdesign: %w", err)
	}

	dispositions := make([]artifact.Disposition, 0, len(board.Stickies))
	stickyIDs := make([]string, 0, len(board.Stickies))
	for _, s := range board.Stickies {
		dispositions = append(dispositions, artifact.Disposition{Sticky: s.ID, Disposition: artifact.DispositionOpenQuestion})
		stickyIDs = append(stickyIDs, s.ID)
	}

	specRef := "spec/" + in.SpecName
	content := scaffoldSpec(specRef, storyRef, in.SpecName, board.Pins, dispositions)

	// Self-validate before writing anything to disk (CLAUDE.md: "never
	// fake success").
	fm, _, splitErr := artifact.SplitFrontmatter([]byte(content))
	if splitErr != nil {
		return nil, fmt.Errorf("commitdesign: internal error: scaffold failed self-validation: %w", splitErr)
	}
	if _, decErr := artifact.DecodeSpec(fm); decErr != nil {
		return nil, fmt.Errorf("commitdesign: internal error: scaffold failed self-validation: %w", decErr)
	}

	frozenBoard, err := freezeBoard(board, relBoardPath, preCommit, at)
	if err != nil {
		return nil, fmt.Errorf("commitdesign: %w", err)
	}

	if err := os.MkdirAll(specDir, 0o755); err != nil {
		return nil, fmt.Errorf("commitdesign: %w", err)
	}
	specPath := filepath.Join(specDir, "spec.md")
	if err := os.WriteFile(specPath, []byte(content), 0o644); err != nil {
		return nil, fmt.Errorf("commitdesign: %w", err)
	}
	boardSnapshotPath := filepath.Join(specDir, "board.json")
	if err := boardio.SaveBoardState(boardSnapshotPath, frozenBoard); err != nil {
		return nil, fmt.Errorf("commitdesign: writing frozen board.json: %w", err)
	}

	if len(stickyIDs) > 0 {
		if _, err := boardio.GraduateStickies(boardio.AnnotationsDir(in.Root), stickyIDs); err != nil {
			return nil, fmt.Errorf("commitdesign: graduating stickies: %w", err)
		}
	}

	if err := gitx.AddAll(ctx, in.Root); err != nil {
		return nil, fmt.Errorf("commitdesign: %w", err)
	}
	commit, err := gitx.CreateCommit(ctx, in.Root, fmt.Sprintf("commit-to-design: %s from board %s", specRef, in.BoardKey))
	if err != nil {
		return nil, fmt.Errorf("commitdesign: %w", err)
	}

	specRel, err := filepath.Rel(in.Root, specPath)
	if err != nil {
		return nil, fmt.Errorf("commitdesign: %w", err)
	}
	boardRel, err := filepath.Rel(in.Root, boardSnapshotPath)
	if err != nil {
		return nil, fmt.Errorf("commitdesign: %w", err)
	}

	return &Result{
		SpecRef:      specRef,
		SpecRelPath:  filepath.ToSlash(specRel),
		BoardRelPath: filepath.ToSlash(boardRel),
		Dispositions: dispositions,
		Commit:       commit,
	}, nil
}

// scaffoldSpec renders the draft feature spec's markdown content: I-10's
// no-magic placeholder title (derived from the spec name — no tracker
// lookup happens here, unlike `design start`'s title resolution, since
// commit-to-design's input is a board, not a story provider), the
// board's pinned refs as `context:`, one placeholder acceptance
// criterion (artifact.SpecFrontmatter.Validate requires at least one),
// and the dispositions block.
func scaffoldSpec(specRef, storyRef, specName string, pins []artifact.Pin, dispositions []artifact.Disposition) string {
	title := titleCase(specName)

	var b strings.Builder
	fmt.Fprintf(&b, "---\nid: %s\nkind: spec\ntitle: %q\nowners: [unassigned]\nclass: feature\nstatus: draft\nstory: %s\n", specRef, title, storyRef)

	if len(pins) > 0 {
		b.WriteString("context:\n")
		for _, p := range pins {
			fmt.Fprintf(&b, "  - %s\n", p.Ref)
		}
	}

	b.WriteString("acceptance_criteria:\n  - { id: ac-1, text: \"TODO: replace with real acceptance criteria before accept\", evidence: [static] }\n")

	if len(dispositions) > 0 {
		b.WriteString("dispositions:\n")
		for _, d := range dispositions {
			fmt.Fprintf(&b, "  - { sticky: %s, disposition: %s }\n", d.Sticky, d.Disposition)
		}
	}

	fmt.Fprintf(&b, "---\n# %s\n\nTODO: design notes.\n\nDrafted by commit-to-design from board %q. Every board sticky above is\ncarried as `open-question` until the commit-to-design skill (or a human)\npromotes it to `incorporated` or `contradicted` (I-5).\n", title, specRef)
	return b.String()
}

// titleCase turns a kebab-case spec name into a human-readable title
// placeholder ("stale-decline-v2" -> "Stale Decline V2").
func titleCase(name string) string {
	parts := strings.Split(name, "-")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}

// boardContent is the hashable content of a board snapshot — pins,
// stickies, and yarn only, never Frozen/Provenance (which would make the
// digest self-referential).
type boardContent struct {
	Pins     []artifact.Pin    `json:"pins"`
	Stickies []artifact.Sticky `json:"stickies"`
	Yarn     []artifact.Yarn   `json:"yarn"`
}

// freezeBoard builds the committed board.json snapshot: the mutable
// board's own pins/stickies/yarn (unmodified — "one frame, not a drag
// history"), a Frozen stamp, and Provenance with a digest recomputable
// from the pins (each already a pinned ref) plus the mutable board file
// itself, named as a path@commit input (02 §Generated artifacts and
// digests).
func freezeBoard(board *artifact.Board, boardPath, commit, at string) (*artifact.Board, error) {
	content := boardContent{Pins: board.Pins, Stickies: board.Stickies, Yarn: board.Yarn}
	canon, err := canonjson.Marshal(content)
	if err != nil {
		return nil, fmt.Errorf("computing board digest: %w", err)
	}
	sum := sha256.Sum256(canon)
	digest := "sha256:" + hex.EncodeToString(sum[:])

	inputs := []string{fmt.Sprintf("%s@%s", filepath.ToSlash(boardPath), commit)}
	seen := map[string]bool{}
	pinRefs := make([]string, 0, len(board.Pins))
	for _, p := range board.Pins {
		if !seen[p.Ref] {
			seen[p.Ref] = true
			pinRefs = append(pinRefs, p.Ref)
		}
	}
	sort.Strings(pinRefs)
	inputs = append(inputs, pinRefs...)

	return &artifact.Board{
		Schema:   "verdi.board/v1",
		Pins:     board.Pins,
		Stickies: board.Stickies,
		Yarn:     board.Yarn,
		Frozen:   &artifact.Frozen{At: at, Commit: commit},
		Provenance: &artifact.Provenance{
			Generator: "commit-to-design",
			Version:   "v0",
			Inputs:    inputs,
			Digest:    digest,
		},
	}, nil
}
