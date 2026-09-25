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
	// Files lists every entry under dir that is not a directory,
	// recursively, as repo-relative slash paths; dir itself is listed when
	// it is not a directory. A symlink, to a file or a directory, is listed
	// as not Regular and never followed. A dir absent from the tree lists
	// nothing.
	Files(ctx context.Context, dir string) ([]TreeFile, error)
	// ReadFile returns a repo-relative path's bytes.
	ReadFile(ctx context.Context, path string) ([]byte, error)
}

// TreeFile is one listed entry: its repo-relative path, and whether it is a
// regular file rather than a symlink or another special entry.
type TreeFile struct {
	Path    string
	Regular bool
}

// errNotRegular is the Failure of a record path, or a directory above one,
// that is not a regular file: both readers report it alike and read
// through neither (lane L3 review a M-5).
var errNotRegular = errors.New("not a regular file; a record is read only from a regular file, never through a link")

// WorkTree reads the working tree under Root, as `verdi align` does.
type WorkTree struct{ Root string }

// Files implements TreeReader.
func (w WorkTree) Files(_ context.Context, dir string) ([]TreeFile, error) {
	base := filepath.Join(w.Root, filepath.FromSlash(dir))
	var out []TreeFile
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
			out = append(out, TreeFile{Path: filepath.ToSlash(rel), Regular: d.Type().IsRegular()})
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
func (c CommitTree) Files(ctx context.Context, dir string) ([]TreeFile, error) {
	entries, err := gitx.LsTreeEntries(ctx, c.Root, c.Commit)
	if err != nil {
		return nil, err
	}
	var out []TreeFile
	for _, e := range entries {
		if e.Path == dir || strings.HasPrefix(e.Path, dir+"/") {
			out = append(out, TreeFile{Path: e.Path, Regular: e.Mode == "100644" || e.Mode == "100755"})
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

// Closed reports the match's reading of "closed" (SI-277): the spec's
// document sits in the archive zone. A `status: closed` spec in the active
// zone is not closed; the match refuses it as target not closed.
func (s *Spec) Closed() bool { return s.Archived }

// Conflict is one decoded conflict of a tree.
type Conflict struct {
	Name string
	Path string
	FM   *artifact.ConflictFrontmatter
}

// superseded reports whether the conflict's own claim is resolved: its
// frontmatter status (open -> superseded | dismissed, 02 §Kind registry)
// is superseded. It is the package's one read of a conflict's status
// (SI-277); it is never a spec's lifecycle state, which comes from
// internal/specstate and first-parent history (SI-270).
func (c *Conflict) superseded() bool { return c.FM.Status == "superseded" }

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
// failure is an error; a record that fails decode, whose id disagrees with
// its path, or that is listed under the two directories as a symlink or
// other non-regular entry, is a Failure.
func ReadRecords(ctx context.Context, tr TreeReader) (*Records, error) {
	recs := &Records{Specs: map[string]*Spec{}}
	fail := func(p string, err error) { recs.Failures = append(recs.Failures, fmt.Sprintf("%s: %v", p, err)) }

	specFiles, err := tr.Files(ctx, specsDir)
	if err != nil {
		return nil, fmt.Errorf("objsupersede: %w", err)
	}
	for _, f := range specFiles {
		p := f.Path
		if !f.Regular {
			fail(p, errNotRegular)
			continue
		}
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

	conflictFiles, err := tr.Files(ctx, conflictsDir)
	if err != nil {
		return nil, fmt.Errorf("objsupersede: %w", err)
	}
	for _, f := range conflictFiles {
		p := f.Path
		if !f.Regular {
			fail(p, errNotRegular)
			continue
		}
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
