package ritualwitness

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

// The parsers below read git's machine output strictly: every malformed
// line is an error naming it, never a silently shorter reading, so a sensor
// that cannot read its source fails the run instead of passing it.

// isObjectID reports whether s is a full object id: 40 (SHA-1) or 64
// (SHA-256) lowercase hex digits.
func isObjectID(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// nulFields splits NUL-terminated output into its fields. Output that is
// not NUL-terminated, or that has an empty field, is malformed.
func nulFields(out []byte, what string) ([]string, error) {
	if len(out) == 0 {
		return nil, nil
	}
	if out[len(out)-1] != 0 {
		return nil, fmt.Errorf("%s: output is not NUL-terminated", what)
	}
	raw := bytes.Split(out[:len(out)-1], []byte{0})
	fields := make([]string, 0, len(raw))
	for _, f := range raw {
		if len(f) == 0 {
			return nil, fmt.Errorf("%s: an empty field", what)
		}
		fields = append(fields, string(f))
	}
	return fields, nil
}

// lines splits newline-terminated output into its non-empty lines.
func lines(out []byte) []string {
	var ls []string
	for _, l := range strings.Split(string(out), "\n") {
		if l != "" {
			ls = append(ls, l)
		}
	}
	return ls
}

// parseRefs reads `for-each-ref --format=%(objectname)%00%(refname)%00%(symref)`.
// Refnames can hold neither NUL nor newline (git check-ref-format), so one
// line per ref with NUL-separated fields is unambiguous.
func parseRefs(out []byte) (map[string]Ref, error) {
	refs := map[string]Ref{}
	for _, l := range lines(out) {
		f := strings.Split(l, "\x00")
		if len(f) != 3 {
			return nil, fmt.Errorf("for-each-ref: malformed line %q", l)
		}
		if !isObjectID(f[0]) {
			return nil, fmt.Errorf("for-each-ref: malformed object id in %q", l)
		}
		if !strings.HasPrefix(f[1], "refs/") {
			return nil, fmt.Errorf("for-each-ref: refname %q is outside refs/", f[1])
		}
		if _, dup := refs[f[1]]; dup {
			return nil, fmt.Errorf("for-each-ref: ref %q listed twice", f[1])
		}
		refs[f[1]] = Ref{Object: f[0], Symref: f[2]}
	}
	return refs, nil
}

// parseIndex reads `ls-files -s -z`: "<mode> <object> <stage>\t<path>"
// fields, sorted by path and then stage.
func parseIndex(out []byte) ([]IndexEntry, error) {
	fields, err := nulFields(out, "ls-files -s -z")
	if err != nil {
		return nil, err
	}
	var entries []IndexEntry
	for _, f := range fields {
		meta, path, ok := strings.Cut(f, "\t")
		if !ok || path == "" {
			return nil, fmt.Errorf("ls-files -s -z: malformed entry %q", f)
		}
		parts := strings.Split(meta, " ")
		if len(parts) != 3 || !isObjectID(parts[1]) {
			return nil, fmt.Errorf("ls-files -s -z: malformed metadata %q", meta)
		}
		stage, err := strconv.Atoi(parts[2])
		if err != nil || stage < 0 || stage > 3 {
			return nil, fmt.Errorf("ls-files -s -z: malformed stage %q", parts[2])
		}
		entries = append(entries, IndexEntry{Mode: parts[0], Object: parts[1], Stage: stage, Path: path})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Path != entries[j].Path {
			return entries[i].Path < entries[j].Path
		}
		return entries[i].Stage < entries[j].Stage
	})
	return entries, nil
}

// parseTree reads `ls-tree -r -z --full-tree`: "<mode> <type> <object>\t<path>"
// fields.
func parseTree(out []byte) (map[string]TreeEntry, error) {
	fields, err := nulFields(out, "ls-tree -r -z")
	if err != nil {
		return nil, err
	}
	tree := map[string]TreeEntry{}
	for _, f := range fields {
		meta, path, ok := strings.Cut(f, "\t")
		if !ok || path == "" {
			return nil, fmt.Errorf("ls-tree -r -z: malformed entry %q", f)
		}
		parts := strings.Split(meta, " ")
		if len(parts) != 3 || !isObjectID(parts[2]) || (parts[1] != "blob" && parts[1] != "commit") {
			return nil, fmt.Errorf("ls-tree -r -z: malformed metadata %q", meta)
		}
		if _, dup := tree[path]; dup {
			return nil, fmt.Errorf("ls-tree -r -z: path %q listed twice", path)
		}
		tree[path] = TreeEntry{Mode: parts[0], Object: parts[2]}
	}
	return tree, nil
}

// parseStatus reads `status --porcelain -z`: "XY <path>" fields, a rename
// or copy followed by its source path as the next field.
func parseStatus(out []byte) ([]StatusEntry, error) {
	fields, err := nulFields(out, "status --porcelain -z")
	if err != nil {
		return nil, err
	}
	var entries []StatusEntry
	for i := 0; i < len(fields); i++ {
		f := fields[i]
		if len(f) < 4 || f[2] != ' ' {
			return nil, fmt.Errorf("status --porcelain -z: malformed entry %q", f)
		}
		e := StatusEntry{X: f[0], Y: f[1], Path: f[3:]}
		if e.X == 'R' || e.X == 'C' {
			i++
			if i >= len(fields) {
				return nil, fmt.Errorf("status --porcelain -z: rename or copy %q has no source field", f)
			}
			e.OrigPath = fields[i]
		}
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries, nil
}

// parsePaths reads a NUL-terminated path list (`ls-files -z`, `diff-tree
// -z --name-only`), keeping each path verbatim and the first occurrence of
// a repeated one.
func parsePaths(out []byte) ([]string, error) {
	fields, err := nulFields(out, "path list")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var paths []string
	for _, f := range fields {
		if !seen[f] {
			seen[f] = true
			paths = append(paths, f)
		}
	}
	return paths, nil
}

// parseCommitObjects reads `cat-file --batch-all-objects
// --batch-check=%(objectname) %(objecttype)` and returns the commits.
func parseCommitObjects(out []byte) ([]string, error) {
	var commits []string
	for _, l := range lines(out) {
		f := strings.Split(l, " ")
		if len(f) != 2 || !isObjectID(f[0]) {
			return nil, fmt.Errorf("cat-file --batch-check: malformed line %q", l)
		}
		switch f[1] {
		case "commit":
			commits = append(commits, f[0])
		case "tree", "blob", "tag":
		default:
			return nil, fmt.Errorf("cat-file --batch-check: unknown object type in %q", l)
		}
	}
	return commits, nil
}

// parseParents reads `rev-list --no-walk --parents`: "<commit>
// <parent>..." lines.
func parseParents(out []byte) (map[string][]string, error) {
	parents := map[string][]string{}
	for _, l := range lines(out) {
		f := strings.Split(l, " ")
		for _, id := range f {
			if !isObjectID(id) {
				return nil, fmt.Errorf("rev-list --parents: malformed line %q", l)
			}
		}
		if _, dup := parents[f[0]]; dup {
			return nil, fmt.Errorf("rev-list --parents: commit %s listed twice", f[0])
		}
		var ps []string
		if len(f) > 1 {
			ps = append(ps, f[1:]...)
		}
		parents[f[0]] = ps
	}
	return parents, nil
}

// noValue marks a configuration key set with no value ("[core] flag"),
// distinct from one set to the empty string.
const noValue = "(no value)"

// parseConfig reads `config --local --list -z`: "<key>\n<value>" fields,
// or "<key>" alone for a key with no value. Each value is kept as "="
// plus its text, so an empty value and no value stay distinct.
func parseConfig(out []byte) (map[string][]string, error) {
	fields, err := nulFields(out, "config --list -z")
	if err != nil {
		return nil, err
	}
	config := map[string][]string{}
	for _, f := range fields {
		key, value, hasValue := strings.Cut(f, "\n")
		if key == "" {
			return nil, fmt.Errorf("config --list -z: an entry with no key: %q", f)
		}
		v := noValue
		if hasValue {
			v = "=" + value
		}
		config[key] = append(config[key], v)
	}
	return config, nil
}

// parseHeadFile reads a HEAD file: "ref: <refname>" for an attached HEAD,
// or a full object id for a detached one.
func parseHeadFile(content []byte) (ref, oid string, err error) {
	s := strings.TrimSuffix(string(content), "\n")
	if target, ok := strings.CutPrefix(s, "ref: "); ok {
		if !strings.HasPrefix(target, "refs/") {
			return "", "", fmt.Errorf("HEAD file: symbolic ref %q is outside refs/", target)
		}
		return target, "", nil
	}
	if isObjectID(s) {
		return "", s, nil
	}
	return "", "", fmt.Errorf("HEAD file: malformed content %q", s)
}

// headFromSymbolicRef reads `symbolic-ref -q HEAD`'s result: an attached
// HEAD's full refname, or detached when (and only when) git exits 1. Any
// other failure is operational, never read as detached.
func headFromSymbolicRef(out []byte, runErr error) (ref string, detached bool, err error) {
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) && exitErr.ExitCode() == 1 {
			return "", true, nil
		}
		return "", false, fmt.Errorf("symbolic-ref -q HEAD: %w", runErr)
	}
	ref = strings.TrimSpace(string(out))
	if !strings.HasPrefix(ref, "refs/") {
		return "", false, fmt.Errorf("symbolic-ref -q HEAD: %q is not a full refname", ref)
	}
	return ref, false, nil
}
