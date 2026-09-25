// Package objsupersede computes closed-spec object supersession from records
// (design docs/superpowers/specs/2026-09-24-closed-spec-object-supersession-
// design.md §3-§5; 03 §Challenging closed decisions, closed-spec object
// supersession): it reads one tree's specs and conflicts, evaluates a
// successor's decision-level fragment `supersedes` edges against the match
// (SI-265, SI-272-SI-274), finds acceptance facts on the default branch's
// first-parent history (SI-270), and renders every result in one reason
// vocabulary. It is the one home align, the gate, the docs site, and the
// board call; none of them re-derives the match.
package objsupersede

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/store"
)

// TreeReader reads one tree's files: a working tree or a commit's tree.
type TreeReader interface {
	// Files lists every file under dir, recursively, as repo-relative slash
	// paths. A dir absent from the tree lists nothing.
	Files(ctx context.Context, dir string) ([]string, error)
	// ReadFile returns a repo-relative path's bytes.
	ReadFile(ctx context.Context, path string) ([]byte, error)
}

// WorkTree reads the working tree under Root, as `verdi align` does.
type WorkTree struct{ Root string }

// Files implements TreeReader.
func (w WorkTree) Files(_ context.Context, dir string) ([]string, error) {
	base := filepath.Join(w.Root, filepath.FromSlash(dir))
	var out []string
	err := filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == base && errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if !d.IsDir() {
			rel, err := filepath.Rel(w.Root, p)
			if err != nil {
				return err
			}
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("objsupersede: listing %s: %w", base, err)
	}
	return out, nil
}

// ReadFile implements TreeReader.
func (w WorkTree) ReadFile(_ context.Context, p string) ([]byte, error) {
	return os.ReadFile(filepath.Join(w.Root, filepath.FromSlash(p)))
}

// CommitTree reads the tree of Commit (any revision git resolves) in the
// repository at Root, through internal/gitx.
type CommitTree struct{ Root, Commit string }

// Files implements TreeReader through a NUL-terminated listing, so a path
// git would quote in a plain listing (a non-ASCII byte, a quote, a control
// character) is listed as written, never skipped (lane L3 review a I-1).
func (c CommitTree) Files(ctx context.Context, dir string) ([]string, error) {
	entries, err := gitx.LsTreeEntries(ctx, c.Root, c.Commit)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.Path == dir || strings.HasPrefix(e.Path, dir+"/") {
			out = append(out, e.Path)
		}
	}
	return out, nil
}

// ReadFile implements TreeReader.
func (c CommitTree) ReadFile(ctx context.Context, p string) ([]byte, error) {
	return gitx.Show(ctx, c.Root, c.Commit, p)
}

// Spec is one decoded spec of a tree.
type Spec struct {
	Name     string // bare name, from its directory
	Path     string // repo-relative spec.md path
	Archived bool   // in the archive zone
	Raw      []byte // the file's bytes
	FM       *artifact.SpecFrontmatter
}

// Closed reports the target reading of "closed": the archive zone, or an
// explicit legacy `status: closed` (VL-002's zone reading).
func (s *Spec) Closed() bool { return s.Archived || s.FM.Status == "closed" }

// Conflict is one decoded conflict of a tree.
type Conflict struct {
	Name string
	Path string
	FM   *artifact.ConflictFrontmatter
}

// Records is one tree's specs (both zones) and conflicts. Failures lists,
// sorted, every record that failed strict decode or disagrees with its path:
// a reported fact, never a skipped file (SI-274(6)).
type Records struct {
	Specs     map[string]*Spec
	Conflicts []*Conflict // sorted by name
	Failures  []string
}

// specsDir and conflictsDir are the store directories ReadRecords lists,
// derived from internal/store's layout accessors.
var (
	specsDir     = path.Dir(path.Dir(store.SpecDirRelPath(store.ZoneActive, "x")))
	conflictsDir = filepath.ToSlash(filepath.Dir(store.ConflictPath("", "x")))
)

// ReadRecords reads and strict-decodes, through internal/artifact, every
// spec.md in both zones and every conflict of the tree. An operational read
// failure is an error; a record that fails decode, or whose id disagrees
// with its path, is a Failure.
func ReadRecords(ctx context.Context, tr TreeReader) (*Records, error) {
	recs := &Records{Specs: map[string]*Spec{}}
	fail := func(p string, err error) { recs.Failures = append(recs.Failures, fmt.Sprintf("%s: %v", p, err)) }

	specPaths, err := tr.Files(ctx, specsDir)
	if err != nil {
		return nil, fmt.Errorf("objsupersede: %w", err)
	}
	for _, p := range specPaths {
		parts := strings.Split(p, "/")
		if len(parts) != 5 || p != store.SpecRelPath(parts[2], parts[3]) || (parts[2] != store.ZoneActive && parts[2] != store.ZoneArchive) {
			continue
		}
		name := parts[3]
		raw, err := tr.ReadFile(ctx, p)
		if err != nil {
			return nil, fmt.Errorf("objsupersede: reading %s: %w", p, err)
		}
		fm, err := decode(raw, artifact.DecodeSpec)
		switch {
		case err != nil:
			fail(p, err)
		case fm.ID != "spec/"+name:
			fail(p, fmt.Errorf("id %s disagrees with its directory", fm.ID))
		case recs.Specs[name] != nil:
			fail(p, fmt.Errorf("spec/%s is in both zones", name))
		default:
			recs.Specs[name] = &Spec{Name: name, Path: p, Archived: parts[2] == store.ZoneArchive, Raw: raw, FM: fm}
		}
	}

	conflictPaths, err := tr.Files(ctx, conflictsDir)
	if err != nil {
		return nil, fmt.Errorf("objsupersede: %w", err)
	}
	for _, p := range conflictPaths {
		name := strings.TrimSuffix(path.Base(p), ".md")
		if p != filepath.ToSlash(store.ConflictPath("", name)) {
			continue
		}
		raw, err := tr.ReadFile(ctx, p)
		if err != nil {
			return nil, fmt.Errorf("objsupersede: reading %s: %w", p, err)
		}
		fm, err := decode(raw, artifact.DecodeConflict)
		switch {
		case err != nil:
			fail(p, err)
		case fm.ID != "conflict/"+name:
			fail(p, fmt.Errorf("id %s disagrees with its file name", fm.ID))
		default:
			recs.Conflicts = append(recs.Conflicts, &Conflict{Name: name, Path: p, FM: fm})
		}
	}
	sort.Slice(recs.Conflicts, func(i, j int) bool { return recs.Conflicts[i].Name < recs.Conflicts[j].Name })
	sort.Strings(recs.Failures)
	return recs, nil
}

// decode splits a record's frontmatter and strict-decodes it.
func decode[T any](raw []byte, dec func([]byte) (*T, error)) (*T, error) {
	fm, _, err := artifact.SplitFrontmatter(raw)
	if err != nil {
		return nil, err
	}
	return dec(fm)
}
