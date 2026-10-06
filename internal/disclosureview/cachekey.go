package disclosureview

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/store"
)

// The cache key covers every input lintDisclosures reads (SI-295). The
// inventory, followed from Current through lint.BuildContext,
// lint.Engine.Run, lint.BuildSnapshot and every rule:
//
// Process state
//   - Environment: CI_DEFAULT_BRANCH, CI_MERGE_REQUEST_TARGET_BRANCH_NAME,
//     GITHUB_BASE_REF, CI, GITHUB_ACTIONS (lint/cienv.go:30,
//     specstate/defaultbranch.go:60); git's own GIT_*, HOME and
//     XDG_CONFIG_HOME (config and repository discovery); PATH (which git
//     runs). Keyed: the whole sorted os.Environ().
//   - Working directory, when root is relative (store/discovery.go:83
//     filepath.Abs). Keyed: the absolute root.
//   - gitx's process-wide shallow memo (gitx/reachable.go:151). Not keyed:
//     a cache miss clears it (gitx.ResetShallowCache) so the enumeration
//     reads the shallow state the key read.
//
// Git (every read is a git subcommand run in root)
//   - HEAD's commit and symbolic name: gitx.CurrentBranch (lint/context.go:87,
//     gitx/branch.go:18), and HEAD as a revision in merge-base, diff, show
//     and ancestry. Keyed: rev-parse HEAD and --symbolic-full-name HEAD.
//   - Ref storage: under git's reftable backend HEAD and every ref live in
//     reftable/ tables the store guard cannot stamp, so such a repository
//     (extensions.refStorage, or a reftable directory) is an uncomputable
//     key (SI-295).
//   - Refs: refs/remotes/origin/HEAD's target (gitx/branch.go:38),
//     refs/remotes/origin/{main,master,<name>} and refs/heads/<name>
//     (specstate/defaultbranch.go:81,112,120), the resolved default
//     branch's commit (lint/context.go:94; specstate/resolve.go:356 and the
//     reads under it), refs/replace/* (git applies them to every object
//     read). Keyed: every ref, its object and its symref target
//     (for-each-ref).
//   - The index: VL-013's gitx.LsFiles (lint/vl013.go:17). Keyed: the same
//     gitx.LsFiles result.
//   - Objects outside the ref closure: a 7- to 40-hex pin resolves through
//     `rev-parse --verify -q <pin>^{commit}` (lint/vl003.go:184,
//     lint/vl009.go:51 via gitx/reachable.go:98), so an object added
//     anywhere can make a short pin ambiguous; VL-015 shows a file at a
//     frozen commit that no ref may reach (lint/vl015.go:188). Keyed: the
//     name of every loose object and pack file. Alternates (a second
//     object store) are not keyed: their presence makes the key
//     uncomputable.
//   - History shape: the shallow boundary (git rev-parse
//     --is-shallow-repository, gitx/reachable.go:174) and grafts. Keyed:
//     the shallow flag and the bytes of shallow and info/grafts.
//   - Configuration: diff rename limits for `diff -M` (gitx/diff.go:43),
//     core.disambiguate for short pins, core.quotePath for ls-files. Keyed:
//     `git config --list -z` (every scope, includes resolved) and
//     `git version`.
//
// Working tree (read directly, gitignored or not)
//   - .verdi/ except .verdi/data/: the document walk (lint/walk.go:86), the
//     top-level listing (lint/walk.go:235), board.json and layout.json
//     (lint/snapshot.go:162,200), .verdi/verdi.yaml (lint/snapshot.go:146,
//     store/open.go:42), .verdi/model.yaml (store/open.go:81), the spec files
//     VL-004, VL-015, VL-019 and VL-022 read again (lint/vl004.go:81,
//     lint/vl015.go:238, storyresolve/resolve.go:371). Keyed: every entry's
//     path and type and every file's bytes. A symbolic link that does not
//     resolve to a regular file makes the key uncomputable (lint reads
//     through linked directories that the walk does not descend).
//   - The mutable zone: .verdi/data/mutable's presence (lint/vl017.go:221),
//     the annotations/ listing and every *.jsonl file's bytes
//     (lint/vl017.go:235). Keyed as read.
//   - Service discovery (lint/snapshot.go:104, store/discovery.go:82): the
//     whole tree below root except .git, node_modules, testdata, examples
//     and .verdi/data (store/discovery.go:47); every .flowmap.yaml's bytes,
//     and for each service root the presence of
//     .flowmap/boundary-contract.json and api/openapi.{yaml,yml,json} and
//     the bytes of verdi.bindings.yaml (store/discovery.go:128). Keyed as
//     read.
//   - Root files: .gitattributes and verdi.bindings.yaml
//     (lint/snapshot.go:127,134). Keyed: presence and bytes.
//
// Not inputs: the wall clock, randomness and the network (no rule reads
// them); the specstate corpus cache, keyed on (root, commit) already
// (specstate/resolve.go:73).
//
// Alongside the key, readInputs records a stamp — mode, size, modification
// and change times, inode and device — of every file and directory it
// reads and of git's own HEAD, index, refs, packed-refs and config files.
// The key says WHAT the inputs are; the stamps say whether any of them was
// written, which is what Cache uses to refuse storing a result computed
// while an input changed, even if it changed back.

// serviceWalkSkipDirs mirrors store's skipDirNames (store/discovery.go:47):
// the directory names service discovery never descends into. A drift
// guard test pins the two sets equal.
var serviceWalkSkipDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"testdata":     true,
	"examples":     true,
}

// serviceCompanions are the fixed paths, relative to a service root,
// whose presence (and, for verdi.bindings.yaml, bytes) discovery reads
// (store/discovery.go:128). A drift guard test pins the unexported two
// against store's own bindingsFile and openAPICandidates.
var serviceCompanions = []string{
	store.BoundaryContractRelPath,
	"verdi.bindings.yaml",
	"api/openapi.yaml",
	"api/openapi.yml",
	"api/openapi.json",
}

// errUncomputable marks a key that cannot be proven complete. A caller
// that gets it enumerates afresh and caches nothing.
var errUncomputable = errors.New("disclosureview: cache key uncomputable")

// inputs is one reading of every input lintDisclosures reads.
type inputs struct {
	// key is a digest of the inputs' contents: two readings with the same
	// key read identical bytes, refs, listings and environment.
	key string
	// stamps is a digest of the stat stamps of everything read.
	stamps string
	// newest is the latest modification or change time among the stamps.
	newest time.Time
}

// same reports whether two readings saw the same contents with nothing
// written in between.
func (in inputs) same(other inputs) bool {
	return in.key == other.key && in.stamps == other.stamps
}

// readInputs reads every input lintDisclosures reads for root. Any error
// means the key cannot be proven complete; it wraps errUncomputable.
func readInputs(ctx context.Context, root string) (inputs, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return inputs{}, uncomputable("resolving root", err)
	}
	r := &inputReader{key: sha256.New(), stamps: sha256.New()}
	r.field("root", []byte(abs))
	r.environment()
	steps := []func() error{
		func() error { return r.git(ctx, abs) },
		func() error { return r.verdiTree(abs) },
		func() error { return r.mutableZone(abs) },
		func() error { return r.services(abs) },
		func() error { return r.rootFiles(abs) },
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return inputs{}, err
		}
	}
	return inputs{
		key:    hex.EncodeToString(r.key.Sum(nil)),
		stamps: hex.EncodeToString(r.stamps.Sum(nil)),
		newest: r.newest,
	}, nil
}

func uncomputable(what string, err error) error {
	return fmt.Errorf("%w: %s: %v", errUncomputable, what, err)
}

// inputReader accumulates the key and stamp digests.
type inputReader struct {
	key    hash.Hash
	stamps hash.Hash
	newest time.Time
}

// field writes one labeled, length-prefixed value into the key, so no two
// different input sets serialize alike.
func (r *inputReader) field(label string, value []byte) {
	writeFramed(r.key, []byte(label))
	writeFramed(r.key, value)
}

// contentField keys a file's bytes by their digest.
func (r *inputReader) contentField(label string, data []byte) {
	sum := sha256.Sum256(data)
	r.field(label, sum[:])
}

func writeFramed(h hash.Hash, b []byte) {
	var n [8]byte
	binary.BigEndian.PutUint64(n[:], uint64(len(b)))
	h.Write(n[:])
	h.Write(b)
}

// stamp records path's stat stamp, following a symbolic link when follow
// is set. An absent path is stamped as absent.
func (r *inputReader) stamp(path string, follow bool) error {
	stat := os.Lstat
	if follow {
		stat = os.Stat
	}
	fi, err := stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		writeFramed(r.stamps, []byte("absent "+path))
		return nil
	}
	if err != nil {
		return uncomputable("stat "+path, err)
	}
	return r.stampInfo(path, fi)
}

func (r *inputReader) stampInfo(path string, fi fs.FileInfo) error {
	ctime, ino, dev, ok := sysStamp(fi)
	if !ok {
		return uncomputable("stat "+path, errors.New("no change time, inode or device on this platform"))
	}
	mtime := fi.ModTime()
	writeFramed(r.stamps, []byte(fmt.Sprintf("%s %o %d %d %d %d %d", path, fi.Mode(), fi.Size(), mtime.UnixNano(), ctime.UnixNano(), ino, dev)))
	for _, t := range []time.Time{mtime, ctime} {
		if t.After(r.newest) {
			r.newest = t
		}
	}
	return nil
}

func (r *inputReader) environment() {
	env := os.Environ()
	sort.Strings(env)
	r.field("environment", []byte(strings.Join(env, "\x00")))
}

// git keys the repository state lint's git reads depend on. Every read
// runs through gitx (spec/gitx-recorder-seam dc-3), whose readers return
// git's exact stdout, so the key is byte for byte what it was when the key
// ran these commands itself.
func (r *inputReader) git(ctx context.Context, root string) error {
	if v := os.Getenv("GIT_ALTERNATE_OBJECT_DIRECTORIES"); v != "" {
		return uncomputable("object store", errors.New("GIT_ALTERNATE_OBJECT_DIRECTORIES names a second object store"))
	}
	version, err := gitx.Version(ctx, root)
	if err != nil {
		return uncomputable("git version", err)
	}
	r.field("git version", version)

	layout, err := gitx.RepositoryLayout(ctx, root)
	if err != nil {
		return uncomputable("git rev-parse", err)
	}
	gitDir, commonDir, objectsDir, shallowPath, graftsPath := layout.GitDir, layout.CommonDir, layout.ObjectsDir, layout.ShallowFile, layout.GraftsFile
	r.field("git rev-parse", layout.Output)

	refs, err := gitx.RefList(ctx, root)
	if err != nil {
		return uncomputable("git for-each-ref", err)
	}
	r.field("git refs", refs)

	config, err := gitx.ConfigList(ctx, root)
	if err != nil {
		return uncomputable("git config --list", err)
	}
	r.field("git config", config)
	if err := refsAreStampable(config, gitDir, commonDir); err != nil {
		return err
	}

	tracked, err := gitx.LsFiles(ctx, root)
	if err != nil {
		return uncomputable("git ls-files", err)
	}
	r.field("git ls-files", []byte(strings.Join(tracked, "\x00")))

	if err := r.optionalFile("git shallow", shallowPath); err != nil {
		return err
	}
	if err := r.optionalFile("git grafts", graftsPath); err != nil {
		return err
	}
	if err := r.objects(objectsDir); err != nil {
		return err
	}

	for _, p := range []string{
		filepath.Join(gitDir, "HEAD"),
		filepath.Join(gitDir, "index"),
		filepath.Join(gitDir, "config.worktree"),
		filepath.Join(commonDir, "packed-refs"),
		filepath.Join(commonDir, "config"),
	} {
		if err := r.stamp(p, false); err != nil {
			return err
		}
	}
	refDirs := []string{filepath.Join(commonDir, "refs")}
	if gitDir != commonDir {
		refDirs = append(refDirs, filepath.Join(gitDir, "refs"))
	}
	for _, dir := range refDirs {
		if err := r.stampTree(dir); err != nil {
			return err
		}
	}
	return nil
}

// refsAreStampable refuses a repository whose refs use git's reftable
// backend (SI-295): HEAD and every ref then live in reftable/ tables that
// the store guard does not stamp, so a ref changed and changed back during
// an enumeration would go unseen. It is detected by extensions.refStorage
// in the configuration (git config --list -z entries are "key\nvalue")
// or by a reftable directory in the repository's own or common git
// directory.
func refsAreStampable(config []byte, gitDir, commonDir string) error {
	for _, entry := range strings.Split(string(config), "\x00") {
		key, value, _ := strings.Cut(entry, "\n")
		if strings.EqualFold(key, "extensions.refstorage") && !strings.EqualFold(strings.TrimSpace(value), "files") {
			return uncomputable("ref storage", fmt.Errorf("extensions.refStorage = %q: refs the store guard cannot stamp", value))
		}
	}
	for _, dir := range []string{gitDir, commonDir} {
		_, err := os.Lstat(filepath.Join(dir, "reftable"))
		switch {
		case err == nil:
			return uncomputable("ref storage", fmt.Errorf("%s holds a reftable directory: refs the store guard cannot stamp", dir))
		case !errors.Is(err, fs.ErrNotExist):
			return uncomputable("ref storage", err)
		}
	}
	return nil
}

// optionalFile keys a file's bytes, or its absence.
func (r *inputReader) optionalFile(label, path string) error {
	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		r.field(label, []byte("absent"))
		return r.stamp(path, true)
	case err != nil:
		return uncomputable("reading "+path, err)
	}
	r.contentField(label, data)
	return r.stamp(path, true)
}

// objects keys the object store's membership: the name of every loose
// object and every pack file.
func (r *inputReader) objects(dir string) error {
	if _, err := os.Lstat(filepath.Join(dir, "info", "alternates")); err == nil {
		return uncomputable("object store", errors.New("objects/info/alternates names a second object store"))
	} else if !errors.Is(err, fs.ErrNotExist) {
		return uncomputable("object store", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return uncomputable("reading "+dir, err)
	}
	if err := r.stamp(dir, false); err != nil {
		return err
	}
	for _, e := range entries {
		name := e.Name()
		r.field("objects entry", []byte(name+" "+e.Type().String()))
		if !e.IsDir() || (name != "pack" && !isFanoutDir(name)) {
			continue
		}
		sub := filepath.Join(dir, name)
		names, err := os.ReadDir(sub)
		if err != nil {
			return uncomputable("reading "+sub, err)
		}
		list := make([]string, len(names))
		for i, n := range names {
			list[i] = n.Name()
		}
		r.field("objects "+name, []byte(strings.Join(list, "\x00")))
		if err := r.stamp(sub, false); err != nil {
			return err
		}
	}
	return nil
}

func isFanoutDir(name string) bool {
	if len(name) != 2 {
		return false
	}
	for _, c := range name {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}

// stampTree stamps every entry under dir (git's loose refs), without
// keying it: for-each-ref already keys the refs' values.
func (r *inputReader) stampTree(dir string) error {
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}
		return r.stampInfo(path, fi)
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return uncomputable("walking "+dir, err)
	}
	return nil
}

// verdiTree keys .verdi/ except .verdi/data/: every entry's path and type
// and every file's bytes.
func (r *inputReader) verdiTree(root string) error {
	verdi := filepath.Join(root, ".verdi")
	fi, err := os.Lstat(verdi)
	if err != nil {
		return uncomputable("stat .verdi", err)
	}
	if !fi.IsDir() {
		return uncomputable(".verdi", errors.New("not a directory (a symbolic link is not followed)"))
	}
	err = filepath.WalkDir(verdi, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(verdi, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		if err := r.stampInfo(path, info); err != nil {
			return err
		}
		if d.IsDir() {
			r.field("verdi dir", []byte(rel))
			if path != verdi && filepath.Dir(path) == verdi && d.Name() == "data" {
				return filepath.SkipDir
			}
			return nil
		}
		return r.regularFile("verdi file "+rel, path, d)
	})
	if err != nil {
		if errors.Is(err, errUncomputable) {
			return err
		}
		return uncomputable("walking .verdi", err)
	}
	return nil
}

// regularFile keys a walked file's bytes. A symbolic link is followed
// when it names a regular file; anything else is uncomputable.
func (r *inputReader) regularFile(label, path string, d fs.DirEntry) error {
	switch {
	case d.Type()&fs.ModeSymlink != 0:
		target, err := os.Stat(path)
		if err != nil {
			return uncomputable("following "+path, err)
		}
		if !target.Mode().IsRegular() {
			return uncomputable(path, errors.New("symbolic link to something other than a regular file"))
		}
		if err := r.stampInfo(path+" (target)", target); err != nil {
			return err
		}
	case !d.Type().IsRegular():
		return uncomputable(path, fmt.Errorf("not a regular file (%s)", d.Type()))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return uncomputable("reading "+path, err)
	}
	r.contentField(label, data)
	return nil
}

// mutableZone keys what VL-017 reads: whether .verdi/data/mutable is a
// directory, and when it is, the annotations/ listing and each *.jsonl
// file's bytes.
func (r *inputReader) mutableZone(root string) error {
	if err := r.stamp(filepath.Join(root, ".verdi", "data"), false); err != nil {
		return err
	}
	mutable := filepath.Join(root, ".verdi", "data", "mutable")
	info, err := os.Stat(mutable)
	present := err == nil && info.IsDir()
	r.field("mutable zone present", []byte(fmt.Sprint(present)))
	if err := r.stamp(mutable, true); err != nil {
		return err
	}
	if !present {
		return nil
	}
	annotations := filepath.Join(mutable, "annotations")
	entries, err := os.ReadDir(annotations)
	if errors.Is(err, fs.ErrNotExist) {
		r.field("annotations", []byte("absent"))
		return r.stamp(annotations, true)
	}
	if err != nil {
		return uncomputable("reading "+annotations, err)
	}
	if err := r.stamp(annotations, true); err != nil {
		return err
	}
	for _, e := range entries {
		r.field("annotations entry", []byte(fmt.Sprintf("%s %t", e.Name(), e.IsDir())))
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		path := filepath.Join(annotations, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return uncomputable("reading "+path, err)
		}
		r.contentField("annotations file "+e.Name(), data)
		if err := r.stamp(path, true); err != nil {
			return err
		}
	}
	return nil
}

// services keys what service discovery reads: the walk's every directory
// (stamped, since a .flowmap.yaml may appear in any of them), every
// .flowmap.yaml's bytes, and each service root's companions.
func (r *inputReader) services(root string) error {
	verdiData := filepath.Join(root, ".verdi", "data")
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && (serviceWalkSkipDirs[d.Name()] || path == verdiData) {
				return filepath.SkipDir
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			return r.stampInfo(path, info)
		}
		if d.Name() != ".flowmap.yaml" {
			return nil
		}
		dir := filepath.Dir(path)
		rel, err := filepath.Rel(root, dir)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		data, err := os.ReadFile(path)
		if err != nil {
			return uncomputable("reading "+path, err)
		}
		r.contentField("service "+rel, data)
		if err := r.stamp(path, true); err != nil {
			return err
		}
		return r.companions(rel, dir)
	})
	if err != nil {
		if errors.Is(err, errUncomputable) {
			return err
		}
		return uncomputable("walking for services", err)
	}
	return nil
}

func (r *inputReader) companions(rel, dir string) error {
	for _, c := range serviceCompanions {
		path := filepath.Join(dir, filepath.FromSlash(c))
		info, err := os.Stat(path)
		exists := err == nil && !info.IsDir()
		r.field("service companion "+rel+"/"+c, []byte(fmt.Sprint(exists)))
		if err := r.stamp(path, true); err != nil {
			return err
		}
		if exists && c == "verdi.bindings.yaml" {
			data, err := os.ReadFile(path)
			if err != nil {
				return uncomputable("reading "+path, err)
			}
			r.contentField("service bindings "+rel, data)
		}
	}
	return nil
}

// rootFiles keys the two files BuildSnapshot reads at the store root.
func (r *inputReader) rootFiles(root string) error {
	for _, name := range []string{".gitattributes", "verdi.bindings.yaml"} {
		if err := r.optionalFile("root file "+name, filepath.Join(root, name)); err != nil {
			return err
		}
	}
	return nil
}
