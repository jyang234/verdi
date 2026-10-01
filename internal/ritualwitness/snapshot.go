package ritualwitness

// Snapshot is everything the sensors cover (SI-325 (7)), taken at one
// instant from a fixture's repository and its bare remote. Every path in
// it is canonical: absolute paths have their symlinks resolved, and
// repository paths are repository-relative, slash-separated, and
// byte-exact (every git listing is read with -z).
type Snapshot struct {
	// Root is the repository's main worktree.
	Root string
	// StoreRoot is the directory Capture was given: the store a ritual
	// runs against. Declared paths are relative to it.
	StoreRoot string
	// Prefix is StoreRoot relative to Root, slash-separated with a
	// trailing "/", or "" when the store is the repository root.
	Prefix string
	// CommonDir is the repository's git common directory.
	CommonDir string

	// Refs is every ref under refs/ in the repository; RemoteRefs every
	// ref under refs/ in the bare remote.
	Refs       map[string]Ref
	RemoteRefs map[string]Ref
	// Head is the main worktree's HEAD.
	Head Head
	// HeadTree is the main worktree's HEAD tree (`ls-tree -r -z`), by
	// repository-relative path: what a HEAD switch or a move of the
	// checked-out branch changes.
	HeadTree map[string]TreeEntry
	// Index is the main worktree's index (`ls-files -s -z`).
	Index []IndexEntry
	// Status is the main worktree's `status --porcelain -z`.
	Status []StatusEntry
	// Files is every non-ignored file of the main worktree, tracked or
	// untracked, by repository-relative path. A path that is absent is
	// not on disk.
	Files map[string]FileState
	// Worktrees is every linked worktree registered in the common
	// directory's worktrees/ administrative directory, sorted by path.
	Worktrees []Worktree
	// Config is the repository's local configuration (`config --local
	// --list -z`) by key; each value is "=" plus the value, or "(no value)"
	// for a key set with no value.
	Config map[string][]string
	// GitFiles is every file under the common directory's hooks/ and
	// info/, by path relative to the common directory.
	GitFiles map[string]FileState
	// Commits is every commit object in the repository or the remote,
	// reachable or not.
	Commits map[string]CommitObject
}

// Ref is one ref's value: the object it resolves to, and for a symbolic
// ref the ref it points at.
type Ref struct {
	Object string
	Symref string
}

// Head is a worktree's HEAD: attached to a branch (Ref, a full refname)
// or detached, and the commit it resolves to.
type Head struct {
	Ref      string
	Detached bool
	Commit   string
}

// IndexEntry is one `git ls-files -s -z` entry.
type IndexEntry struct {
	Mode   string
	Object string
	Stage  int
	Path   string
}

// TreeEntry is one blob (or gitlink) of a tree: its mode and object.
type TreeEntry struct {
	Mode   string
	Object string
}

// StatusEntry is one `git status --porcelain -z` entry: X compares the
// index with HEAD, Y the working tree with the index. OrigPath is a
// rename's or copy's source, "" otherwise.
type StatusEntry struct {
	X, Y           byte
	Path, OrigPath string
}

// FileKind is what a path is on disk.
type FileKind string

// The file kinds the sensors record. A directory is never recorded: see
// doc.go.
const (
	KindFile    FileKind = "file"
	KindSymlink FileKind = "symlink"
)

// FileState is a file's content as the sensors read it: its kind, its
// executable bit, and the SHA-256 of its bytes (of its target, for a
// symlink).
type FileState struct {
	Kind   FileKind
	Exec   bool
	Digest string
}

// Worktree is one linked worktree, read from its administrative entry
// ($GIT_COMMON_DIR/worktrees/<ID>/).
type Worktree struct {
	ID   string
	Path string
	// Head is the linked worktree's HEAD, from its administrative entry;
	// for an attached HEAD, Commit is the branch's tip.
	Head Head
	// Locked is whether the entry holds a lock; LockReason is the lock
	// file's content.
	Locked     bool
	LockReason string
	// Present is whether the worktree's directory exists. Index is its
	// index, read only when it does.
	Present bool
	Index   []IndexEntry
}

// CommitObject is one commit: its parents, in order, and its file list
// against its first parent (against the empty tree for a root commit).
type CommitObject struct {
	Parents []string
	Files   []string
}
