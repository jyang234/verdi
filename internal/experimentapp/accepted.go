package experimentapp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/jyang234/verdi/internal/experiment"
)

// DefaultBranch is one locally resolved accepted default-branch fact.
type DefaultBranch struct {
	Name string
	Ref  string
	Head string
}

func (b DefaultBranch) validate() error {
	if strings.TrimSpace(b.Name) == "" || strings.TrimSpace(b.Ref) == "" {
		return fmt.Errorf("experimentapp: default branch name and ref must be nonblank")
	}
	if err := experiment.ValidateCommit(b.Head); err != nil {
		return fmt.Errorf("experimentapp: default branch HEAD: %w", err)
	}
	return nil
}

// GitTreeEntry is one exact recursive tree leaf.
type GitTreeEntry struct {
	Mode   string
	Type   string
	Object string
	Path   string
}

type staleAcceptedHeadError struct {
	want string
	got  string
}

func (e *staleAcceptedHeadError) Error() string {
	return fmt.Sprintf("expected accepted HEAD %s, got %s", e.want, e.got)
}

type acceptedSnapshot struct {
	revision       DefaultBranch
	experimentPath string
	source         *snapshotFS
	// entries is the ORIGINAL complete accepted-tree enumeration the one
	// ListTree call returned, retained so lifecycle operations resolve
	// protected-input entries from the same snapshot without a second
	// enumeration (design §7: HEAD identity, Git enumeration, blob reads,
	// and state derivation each execute once).
	entries []GitTreeEntry
}

func resolveAcceptedHead(ctx context.Context, git AcceptedGit, identity Identity) (DefaultBranch, error) {
	revision, err := git.ResolveDefaultBranch(ctx, identity.CheckoutRoot)
	if err != nil {
		return DefaultBranch{}, fmt.Errorf("experimentapp: resolve default branch: %w", err)
	}
	if err := revision.validate(); err != nil {
		return DefaultBranch{}, err
	}
	if revision.Head != identity.ExpectedAcceptedHEAD {
		return DefaultBranch{}, &staleAcceptedHeadError{want: identity.ExpectedAcceptedHEAD, got: revision.Head}
	}
	return revision, nil
}

func resolveAccepted(ctx context.Context, git AcceptedGit, identity Identity) (acceptedSnapshot, error) {
	return resolveAcceptedExperiment(ctx, git, identity, true)
}

// resolveAcceptedBase permits an absent experiment at accepted HEAD so a new
// draft can be reviewed against the canonical empty artifact set.
func resolveAcceptedBase(ctx context.Context, git AcceptedGit, identity Identity) (acceptedSnapshot, error) {
	return resolveAcceptedExperiment(ctx, git, identity, false)
}

func resolveAcceptedExperiment(ctx context.Context, git AcceptedGit, identity Identity, requireExperiment bool) (acceptedSnapshot, error) {
	revision, err := resolveAcceptedHead(ctx, git, identity)
	if err != nil {
		return acceptedSnapshot{}, err
	}
	entries, err := git.ListTree(ctx, identity.CheckoutRoot, revision.Head)
	if err != nil {
		return acceptedSnapshot{}, fmt.Errorf("experimentapp: enumerate accepted tree at %s: %w", revision.Head, err)
	}
	spikeID := strings.TrimPrefix(identity.Spike, "spec/")
	active := path.Join(".verdi/specs/active", spikeID, "experiments", identity.ExperimentID)
	archive := path.Join(".verdi/specs/archive", spikeID, "experiments", identity.ExperimentID)
	grouped := map[string][]GitTreeEntry{active: {}, archive: {}}
	for _, entry := range entries {
		for _, prefix := range []string{active, archive} {
			if entry.Path == prefix || strings.HasPrefix(entry.Path, prefix+"/") {
				grouped[prefix] = append(grouped[prefix], entry)
			}
		}
	}
	present := make([]string, 0, 2)
	for _, prefix := range []string{active, archive} {
		if len(grouped[prefix]) > 0 {
			present = append(present, prefix)
		}
	}
	if len(present) == 0 && !requireExperiment {
		return acceptedSnapshot{revision: revision, source: newSnapshotFS(map[string][]byte{}), entries: entries}, nil
	}
	if len(present) != 1 {
		return acceptedSnapshot{}, fmt.Errorf("experimentapp: accepted experiment resolves in %d active/archive locations, want exactly one", len(present))
	}
	experimentPath := present[0]
	selected := grouped[experimentPath]
	sort.Slice(selected, func(i, j int) bool { return selected[i].Path < selected[j].Path })
	files := make(map[string][]byte, len(selected))
	definitionPresent := false
	for _, entry := range selected {
		if err := experiment.ValidateRepoRelativePath(entry.Path); err != nil {
			return acceptedSnapshot{}, fmt.Errorf("experimentapp: accepted tree path: %w", err)
		}
		if entry.Type != "blob" || entry.Mode != "100644" && entry.Mode != "100755" || strings.TrimSpace(entry.Object) == "" {
			return acceptedSnapshot{}, fmt.Errorf("experimentapp: accepted experiment entry %q is not a regular blob", entry.Path)
		}
		if _, duplicate := files[entry.Path]; duplicate {
			return acceptedSnapshot{}, fmt.Errorf("experimentapp: duplicate accepted experiment path %q", entry.Path)
		}
		data, err := git.ReadBlob(ctx, identity.CheckoutRoot, revision.Head, entry.Object, entry.Path)
		if err != nil {
			return acceptedSnapshot{}, fmt.Errorf("experimentapp: read accepted blob %s:%s: %w", revision.Head, entry.Path, err)
		}
		files[entry.Path] = append([]byte(nil), data...)
		definitionPresent = definitionPresent || entry.Path == path.Join(experimentPath, "experiment.yaml")
	}
	if !definitionPresent {
		return acceptedSnapshot{}, fmt.Errorf("experimentapp: accepted experiment has no experiment.yaml")
	}
	return acceptedSnapshot{revision: revision, experimentPath: experimentPath, source: newSnapshotFS(files), entries: entries}, nil
}

type snapshotFS struct {
	files map[string][]byte
	dirs  map[string]bool
}

func newSnapshotFS(files map[string][]byte) *snapshotFS {
	cloned := make(map[string][]byte, len(files))
	dirs := map[string]bool{".": true}
	for name, data := range files {
		cloned[name] = append([]byte(nil), data...)
		for dir := path.Dir(name); dir != "."; dir = path.Dir(dir) {
			dirs[dir] = true
		}
	}
	return &snapshotFS{files: cloned, dirs: dirs}
}

func (s *snapshotFS) Open(name string) (fs.File, error) {
	if data, ok := s.files[name]; ok {
		return &snapshotFile{Reader: bytes.NewReader(append([]byte(nil), data...)), info: snapshotInfo{name: path.Base(name), size: int64(len(data))}}, nil
	}
	if s.dirs[name] {
		return &snapshotFile{Reader: bytes.NewReader(nil), info: snapshotInfo{name: path.Base(name), dir: true}}, nil
	}
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

func (s *snapshotFS) ReadFile(name string) ([]byte, error) {
	data, ok := s.files[name]
	if !ok {
		return nil, &fs.PathError{Op: "read", Path: name, Err: fs.ErrNotExist}
	}
	return append([]byte(nil), data...), nil
}

func (s *snapshotFS) Stat(name string) (fs.FileInfo, error) {
	if data, ok := s.files[name]; ok {
		return snapshotInfo{name: path.Base(name), size: int64(len(data))}, nil
	}
	if s.dirs[name] {
		return snapshotInfo{name: path.Base(name), dir: true}, nil
	}
	return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
}

func (s *snapshotFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if !s.dirs[name] {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrNotExist}
	}
	prefix := ""
	if name != "." {
		prefix = name + "/"
	}
	entries := map[string]snapshotInfo{}
	for dir := range s.dirs {
		if dir == name || !strings.HasPrefix(dir, prefix) {
			continue
		}
		remainder := strings.TrimPrefix(dir, prefix)
		if !strings.Contains(remainder, "/") {
			entries[remainder] = snapshotInfo{name: remainder, dir: true}
		}
	}
	for file, data := range s.files {
		if !strings.HasPrefix(file, prefix) {
			continue
		}
		remainder := strings.TrimPrefix(file, prefix)
		if !strings.Contains(remainder, "/") {
			entries[remainder] = snapshotInfo{name: remainder, size: int64(len(data))}
		}
	}
	names := make([]string, 0, len(entries))
	for entry := range entries {
		names = append(names, entry)
	}
	sort.Strings(names)
	out := make([]fs.DirEntry, len(names))
	for index, entry := range names {
		out[index] = snapshotDirEntry{entries[entry]}
	}
	return out, nil
}

type snapshotInfo struct {
	name string
	size int64
	dir  bool
}

func (i snapshotInfo) Name() string { return i.name }
func (i snapshotInfo) Size() int64  { return i.size }
func (i snapshotInfo) Mode() fs.FileMode {
	if i.dir {
		return fs.ModeDir | 0o555
	}
	return 0o444
}
func (i snapshotInfo) ModTime() time.Time { return time.Time{} }
func (i snapshotInfo) IsDir() bool        { return i.dir }
func (i snapshotInfo) Sys() any           { return nil }

type snapshotDirEntry struct{ snapshotInfo }

func (e snapshotDirEntry) Type() fs.FileMode          { return e.Mode().Type() }
func (e snapshotDirEntry) Info() (fs.FileInfo, error) { return e.snapshotInfo, nil }

type snapshotFile struct {
	*bytes.Reader
	info snapshotInfo
}

func (f *snapshotFile) Close() error               { return nil }
func (f *snapshotFile) Stat() (fs.FileInfo, error) { return f.info, nil }

var _ io.Reader = (*snapshotFile)(nil)
var _ fs.ReadFileFS = (*snapshotFS)(nil)
var _ fs.ReadDirFS = (*snapshotFS)(nil)
var _ fs.StatFS = (*snapshotFS)(nil)
