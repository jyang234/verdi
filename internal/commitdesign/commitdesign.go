package commitdesign

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/boardio"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/designscaffold"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/store"
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
	// ModelDigest is the resolved operating model's canonical-JSON sha256
	// digest (model.Model.Digest(), spec/model-digest ledger L-M5) — the
	// caller (cmd/verdi/board.go or internal/workbench's boardCommitHandler)
	// resolves it once via store.Open(Root).Model.Digest() ahead of calling
	// Run, since this package never imports internal/model itself (that
	// package already imports internal/artifact, so the reverse import
	// would cycle). Threaded straight to freezeBoard's artifact.
	// StampProvenance call; empty reaches StampProvenance's own panic, the
	// same fail-closed posture Frozen's at/commit already have.
	ModelDigest string
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
			// vocab:identity — story: FIELD validation (scheme:key form)
			return nil, fmt.Errorf("commitdesign: no story ref given, and board key %q is not itself scheme:key-shaped; pass StoryRef explicitly", in.BoardKey)
		}
		storyRef = in.BoardKey
	}
	if !storyRefShapeRe.MatchString(storyRef) {
		// vocab:identity — story: FIELD validation (scheme:key form)
		return nil, fmt.Errorf("commitdesign: story ref %q must be scheme:key form (e.g. jira:LOAN-1482)", storyRef)
	}

	specDir := store.ActiveSpecDir(in.Root, in.SpecName)
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
	// judged-ac5-board-freeze-wallclock: the frozen board.json stamp pairs
	// `at` with `commit` (preCommit — the content-final sha this snapshot pins
	// to, threaded into Frozen and the provenance inputs below), so `at` is
	// that commit's own committer date, never wall clock — the obligationauthor
	// precedent (L-M4, internal/artifact.NewFrozen's doc) that this seam exists
	// to enforce everywhere a frozen artifact is minted.
	at, err := gitx.CommitDateOnly(ctx, in.Root, preCommit)
	if err != nil {
		return nil, fmt.Errorf("commitdesign: %w", err)
	}
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
	tmpl, err := resolveTemplate(in.Root)
	if err != nil {
		return nil, err
	}
	content, err := scaffoldSpec(tmpl, specRef, storyRef, in.SpecName, board.Pins, dispositions)
	if err != nil {
		return nil, fmt.Errorf("commitdesign: rendering scaffold: %w", err)
	}

	// Self-validate before writing anything to disk (CLAUDE.md: "never
	// fake success").
	fm, _, splitErr := artifact.SplitFrontmatter([]byte(content))
	if splitErr != nil {
		return nil, fmt.Errorf("commitdesign: internal error: scaffold failed self-validation: %w", splitErr)
	}
	spec, decErr := artifact.DecodeSpec(fm)
	if decErr != nil {
		return nil, fmt.Errorf("commitdesign: internal error: scaffold failed self-validation: %w", decErr)
	}
	// K1, inherited from every other scaffold consumer (spec/creation-form
	// ac-4): a store's feature.md override can hardcode the wrong class:
	// literal and still strict-decode clean — caught here, before any
	// write.
	if ccErr := designscaffold.CheckClass(spec, artifact.ClassFeature); ccErr != nil {
		return nil, fmt.Errorf("commitdesign: template for class %s failed self-validation: %w", artifact.ClassFeature, ccErr)
	}

	frozenBoard, err := freezeBoard(board, relBoardPath, preCommit, at, in.ModelDigest)
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

// resolveTemplate resolves commit-to-design's scaffold template in the
// two layers spec/creation-form ac-4 ratified (the L-M12 discharge): a
// store's own override of the FEATURE class's declared Class.Template —
// the exact file L-M12's witness named as silently ignored on this path
// — wins; absent one, the embedded commit-to-design canonical template
// (designscaffold templates/commitdesign.md), whose render reproduces
// the retired strings.Builder output byte-for-byte for every input the
// old producer handled (TestScaffoldSpec_BytePin). The embedded
// fallback is deliberately NOT the canonical feature.md: byte-stability
// and override-honoring cannot both hold over it — the legacy
// commit-to-design shape predates problem:/outcome: and carries
// context:/dispositions: blocks feature.md has no slots for.
func resolveTemplate(root string) ([]byte, error) {
	cfg, err := store.Open(root)
	if err != nil {
		return nil, fmt.Errorf("commitdesign: resolving store config: %w", err)
	}
	class, ok := cfg.Model.Classes[string(artifact.ClassFeature)]
	if !ok {
		return nil, fmt.Errorf("commitdesign: internal error: resolved model has no %q class", artifact.ClassFeature)
	}
	tmpl, overridden, err := designscaffold.LoadOverride(root, class.Template)
	if err != nil {
		return nil, fmt.Errorf("commitdesign: %w", err)
	}
	if overridden {
		return tmpl, nil
	}
	tmpl, err = designscaffold.Canonical("commitdesign.md")
	if err != nil {
		return nil, fmt.Errorf("commitdesign: %w", err)
	}
	return tmpl, nil
}

// scaffoldSpec renders the draft feature spec's markdown content through
// the shared designscaffold producer (spec/creation-form ac-4 — the
// retired strings.Builder body was the L-M12 third producer): I-10's
// no-magic placeholder title (derived from the spec name — no tracker
// lookup happens here, unlike `design start`'s title resolution, since
// commit-to-design's input is a board, not a story provider), the
// board's pinned refs as `context:`, one placeholder acceptance
// criterion (artifact.SpecFrontmatter.Validate requires at least one),
// and the dispositions block — all through the template's own
// content-carrying fields (ScaffoldData.Pins/.Dispositions). The
// statement defaults are passed for override templates that reference
// {{.Problem}}/{{.Outcome}}; the embedded canonical ignores them, as the
// legacy shape always did.
func scaffoldSpec(tmpl []byte, specRef, storyRef, specName string, pins []artifact.Pin, dispositions []artifact.Disposition) (string, error) {
	return designscaffold.Render(tmpl, designscaffold.ScaffoldData{
		Ref:          specRef,
		Title:        designscaffold.HumanizeName(specName),
		StoryRef:     storyRef,
		Owners:       designscaffold.DefaultOwners,
		Problem:      designscaffold.DefaultProblem,
		Outcome:      designscaffold.DefaultOutcome,
		Pins:         pins,
		Dispositions: dispositions,
	})
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
// digests). modelDigest is stamped via artifact.StampProvenance
// (spec/model-digest ac-2: never set inline in the Provenance{...}
// literal below, the same way Digest is).
func freezeBoard(board *artifact.Board, boardPath, commit, at, modelDigest string) (*artifact.Board, error) {
	content := boardContent{Pins: board.Pins, Stickies: board.Stickies, Yarn: board.Yarn}
	digest, err := canonjson.Digest(content)
	if err != nil {
		return nil, fmt.Errorf("computing board digest: %w", err)
	}

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

	frozen := artifact.NewFrozen(at, commit)
	prov := &artifact.Provenance{
		Generator: "commit-to-design",
		Version:   "v0",
		Inputs:    inputs,
		Digest:    digest,
	}
	artifact.StampProvenance(prov, modelDigest)
	return &artifact.Board{
		Schema:     "verdi.board/v1",
		Pins:       board.Pins,
		Stickies:   board.Stickies,
		Yarn:       board.Yarn,
		Frozen:     &frozen,
		Provenance: prov,
	}, nil
}
