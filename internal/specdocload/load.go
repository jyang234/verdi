package specdocload

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/matrixprojection"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/readinesspilot"
	"github.com/jyang234/verdi/internal/specdoc"
	"github.com/jyang234/verdi/internal/specstate"
	"github.com/jyang234/verdi/internal/store"
)

// Mode selects which bytes a document is rendered from.
type Mode int

const (
	// ModeAccepted reads the default branch's bytes; the stamp commit is
	// that branch's head. This is the accepted reading.
	ModeAccepted Mode = iota
	// ModeAt reads a pinned commit's bytes.
	ModeAt
	// ModeWorkingTree reads the checkout's bytes, stamped with HEAD; the
	// stamp is marked proposed unless specstate proves the bytes are the
	// exact accepted bytes on the default branch (ruling R-W2-4).
	ModeWorkingTree
)

// Request names the spec, the mode, and the optional facts a caller owns.
type Request struct {
	Root      string
	Name      string
	Mode      Mode
	At        string
	Kind      specdoc.Kind
	Model     *model.Model
	Readiness *readinesspilot.Snapshot
}

// Result carries the assembled Input plus what a consumer may want to
// disclose or reuse.
type Result struct {
	Input       specdoc.Input
	RelPath     string
	Head        string
	Disclosures []string
}

// Load assembles the Input. Errors are operational (unreadable store,
// unknown spec, unresolvable commit or default branch, invalid kind);
// degraded facts (status not resolved, evidence not computed) are
// disclosed in Result.Disclosures and never fail the load.
func Load(ctx context.Context, req Request) (Result, error) {
	if req.Name == "" {
		return Result{}, errors.New("specdocload: spec name is required")
	}
	if _, err := artifact.ParseRef("spec/" + req.Name); err != nil {
		return Result{}, fmt.Errorf("specdocload: spec name %q: %w", req.Name, err)
	}
	if _, err := specdoc.ParseKind(string(req.Kind)); err != nil {
		return Result{}, err
	}
	if _, err := os.Stat(filepath.Join(req.Root, ".verdi", "verdi.yaml")); err != nil {
		return Result{}, fmt.Errorf("specdocload: %s is not a verdi store root: %w", req.Root, err)
	}
	ref := "spec/" + req.Name

	src, err := loadSource(ctx, req)
	if err != nil {
		return Result{}, err
	}
	// Split first, decode the frontmatter bytes only — the module's one
	// DecodeSpec convention. Decoding the whole file let the YAML parser
	// peek past the closing "---" into the Markdown body to find the
	// document end, so a body opening with "*" (bold) failed as an alias.
	fmBytes, body, err := artifact.SplitFrontmatter(src.content)
	if err != nil {
		return Result{}, fmt.Errorf("specdocload: %s at %s: %w", ref, src.commit, err)
	}
	fm, err := artifact.DecodeSpec(fmBytes)
	if err != nil {
		return Result{}, fmt.Errorf("specdocload: %s at %s: %w", ref, src.commit, err)
	}

	var disclosures []string
	status := ""
	proposed := false
	if res, rerr := specstate.NewProjector().Resolve(ctx, req.Root, specstate.Candidate{Path: src.relPath, Content: src.content}); rerr == nil {
		status = string(res.ArtifactStatus())
		if req.Mode == ModeWorkingTree {
			proposed = res.Relation != specstate.RelationExact
		}
	} else {
		disclosures = append(disclosures, "status not resolved: "+rerr.Error())
		if req.Mode == ModeWorkingTree {
			proposed = true
		}
	}

	// HEAD is resolved separately from src.commit (which, under ModeAt or
	// ModeAccepted, names a possibly different — older or newer — commit
	// than HEAD): the matrix always evaluates the working tree's current
	// HEAD. Fix-round-1 F1: a HEAD that cannot be resolved (e.g. a
	// checkout whose HEAD points at a branch that was never created,
	// while the default branch itself still resolves cleanly via a
	// remote-tracking ref) must not block a ModeAccepted/ModeAt render —
	// only evidence, which depends on HEAD, degrades: Result.Head stays
	// empty and WithMatrix is skipped, exactly Wave 1's CLI behavior
	// before this package existed. ModeWorkingTree is different:
	// loadSource above already resolved this exact HEAD to read the
	// working tree's bytes, so a failure here can only mean HEAD moved
	// between the two calls — stay conservative and fail the whole
	// render rather than stamp facts with a HEAD that no longer means
	// anything.
	var head string
	if h, herr := gitx.RevParse(ctx, req.Root, "HEAD"); herr != nil {
		if req.Mode == ModeWorkingTree {
			return Result{}, fmt.Errorf("specdocload: resolving HEAD: %w", herr)
		}
		disclosures = append(disclosures, "evidence not computed: resolving HEAD: "+herr.Error())
	} else {
		head = h
	}

	facts := specdoc.FactsFromSpec(fm)
	if head != "" {
		// The matrix's preview flag is the derived `proposed` value, never
		// the raw req.Mode == ModeWorkingTree test (controller ruling,
		// spec-documents wave 2 task 2): a working-tree render of the exact
		// accepted bytes (proposed == false) must fold evidence identically
		// to the accepted reading, so every consumer that lands on the
		// accepted bytes — whichever mode it asked in — sees the same
		// evidence.
		preview := proposed
		if proj, perr := matrixprojection.Project(ctx, req.Root, ref, preview, req.Model); perr == nil {
			facts = specdoc.WithMatrix(facts, proj.Record, head)
		} else {
			disclosures = append(disclosures, "evidence not computed: "+perr.Error())
		}
	}
	if req.Readiness != nil {
		facts = specdoc.WithReadiness(facts, *req.Readiness, ref)
	}

	return Result{
		Input: specdoc.Input{
			Spec:   fm,
			Body:   body,
			Status: status,
			Stamp:  specdoc.Stamp{Ref: ref, Commit: src.commit, Proposed: proposed},
			Facts:  facts,
			Model:  req.Model,
			Kind:   req.Kind,
		},
		RelPath:     src.relPath,
		Head:        head,
		Disclosures: disclosures,
	}, nil
}

type source struct {
	relPath string
	content []byte
	commit  string
}

// loadSource picks the bytes for the mode: active zone first, archive
// second, in every mode (fix-round-1 F2 pins this precedence with tests
// covering both the working-tree read below and the git-show read after
// it, including a both-zones fixture proving the active zone wins).
func loadSource(ctx context.Context, req Request) (source, error) {
	name := req.Name
	switch req.Mode {
	case ModeWorkingTree:
		for _, p := range []struct{ abs, rel string }{
			{store.ActiveSpecPath(req.Root, name), store.ActiveSpecRelPath(name)},
			{store.ArchiveSpecPath(req.Root, name), store.SpecRelPath(store.ZoneArchive, name)},
		} {
			data, rerr := os.ReadFile(p.abs)
			if rerr == nil {
				head, err := gitx.RevParse(ctx, req.Root, "HEAD")
				if err != nil {
					return source{}, fmt.Errorf("specdocload: resolving HEAD: %w", err)
				}
				return source{relPath: p.rel, content: data, commit: head}, nil
			}
			if !os.IsNotExist(rerr) {
				return source{}, fmt.Errorf("specdocload: reading %s: %w", p.abs, rerr)
			}
		}
		// fix-round-1 F6: name both paths actually tried, not just the
		// zone-agnostic claim that neither held the spec.
		return source{}, fmt.Errorf("specdocload: spec/%s not found in either zone of the working tree (tried %s and %s)", name, store.ActiveSpecPath(req.Root, name), store.ArchiveSpecPath(req.Root, name))
	case ModeAccepted, ModeAt:
		// handled below — both read via git-show at a resolved commit.
	default:
		// fix-round-1 F9: an unknown Mode value fails closed, named,
		// rather than silently falling through to ModeAt's git-show path
		// with whatever req.At happens to hold (CLAUDE.md: "unknown enum
		// values fail closed").
		return source{}, fmt.Errorf("specdocload: unknown mode %d", req.Mode)
	}

	rev := req.At
	if req.Mode == ModeAt && rev == "" {
		// fix-round-1 F9: named refusal instead of letting an empty
		// revision string reach gitx.RevParse and fail with a generic
		// git error that never names which field was missing.
		return source{}, errors.New("specdocload: ModeAt requires a commit")
	}
	if req.Mode == ModeAccepted {
		branch, ok := specstate.ResolveDefaultBranch(ctx, req.Root)
		if !ok {
			return source{}, errors.New("specdocload: the default branch could not be resolved; use a pinned commit or the working tree")
		}
		rev = branch.Ref
	}
	commit, err := gitx.RevParse(ctx, req.Root, rev)
	if err != nil {
		return source{}, fmt.Errorf("specdocload: resolving %q: %w", rev, err)
	}
	for _, rel := range []string{store.ActiveSpecRelPath(name), store.SpecRelPath(store.ZoneArchive, name)} {
		content, err := gitx.Show(ctx, req.Root, commit, rel)
		if err == nil {
			return source{relPath: rel, content: content, commit: commit}, nil
		}
	}
	return source{}, fmt.Errorf("specdocload: spec/%s not found at %s in either zone", name, commit)
}
