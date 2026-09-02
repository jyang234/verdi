package gitx

import (
	"bytes"
	"context"
	"fmt"
)

// TreeEntry is one leaf (blob or gitlink) `git ls-tree -rz --full-tree`
// reports at a ref: its mode, object type, blob/commit object ID, and
// repo-root-relative path. -r recurses into subtrees and therefore never
// emits a "tree" entry itself — only the leaves it contains.
type TreeEntry struct {
	Mode   string
	Type   string
	Object string
	Path   string
}

// LsTreeEntries lists every leaf at ref via
// `git ls-tree -rz --full-tree <ref>`, parsed on the mode/type/object/TAB/
// path grammar without ever splitting on newline: -z NUL-terminates each
// entry so paths carrying whitespace, tabs, newlines, or non-ASCII bytes
// survive intact. --full-tree keeps paths repository-root-relative
// regardless of dir's own position inside the repository. LsTreeEntries
// never reads blob content.
func LsTreeEntries(ctx context.Context, dir, ref string) ([]TreeEntry, error) {
	out, err := run(ctx, dir, "ls-tree", "-rz", "--full-tree", ref)
	if err != nil {
		return nil, fmt.Errorf("gitx: LsTreeEntries(%s): %w", ref, err)
	}
	entries, err := parseTreeEntries(out)
	if err != nil {
		return nil, fmt.Errorf("gitx: LsTreeEntries(%s): %w", ref, err)
	}
	return entries, nil
}

// LsTreeEntriesIncludingTrees lists every tree and leaf at ref. The -t flag
// retains directory entries while -r continues through them, allowing closed
// tree grammars to reject an unexpected directory even when it is empty.
func LsTreeEntriesIncludingTrees(ctx context.Context, dir, ref string) ([]TreeEntry, error) {
	out, err := run(ctx, dir, "ls-tree", "-rtz", "--full-tree", ref)
	if err != nil {
		return nil, fmt.Errorf("gitx: LsTreeEntriesIncludingTrees(%s): %w", ref, err)
	}
	entries, err := parseTreeEntries(out)
	if err != nil {
		return nil, fmt.Errorf("gitx: LsTreeEntriesIncludingTrees(%s): %w", ref, err)
	}
	return entries, nil
}

// parseTreeEntries splits `git ls-tree -rz --full-tree` output on NUL bytes
// (never on newline, which is a legal path byte) and each entry on its
// first TAB, which separates the space-separated "<mode> <type> <object>"
// metadata from the path that follows.
func parseTreeEntries(out []byte) ([]TreeEntry, error) {
	var entries []TreeEntry
	for _, raw := range bytes.Split(out, []byte{0}) {
		if len(raw) == 0 {
			// The trailing NUL yields one empty final field.
			continue
		}
		tab := bytes.IndexByte(raw, '\t')
		if tab < 0 {
			return nil, fmt.Errorf("malformed `git ls-tree -rz` entry %q: no TAB separator", raw)
		}
		meta := bytes.Fields(raw[:tab])
		if len(meta) != 3 {
			return nil, fmt.Errorf("malformed `git ls-tree -rz` entry %q: want mode, type, object before TAB, got %d fields", raw, len(meta))
		}
		entries = append(entries, TreeEntry{
			Mode:   string(meta[0]),
			Type:   string(meta[1]),
			Object: string(meta[2]),
			Path:   string(raw[tab+1:]),
		})
	}
	return entries, nil
}
