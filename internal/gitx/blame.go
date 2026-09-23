package gitx

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// BlameLine is one line of `git blame --line-porcelain` output: the commit
// the line is attributed to, where the line sat in that commit's version
// of the file, and whether blame stopped at a history boundary there.
type BlameLine struct {
	// Commit is the full lowercase object id the line is attributed to.
	Commit string
	// OrigLine is the line's 1-based number in Commit's version of
	// Filename.
	OrigLine int
	// FinalLine is the line's 1-based number in the blamed revision.
	FinalLine int
	// Boundary reports that blame could not look past Commit: a root
	// commit, or the grafted tip of a shallow clone. Such a commit is where
	// the available history ends, not proof the line was introduced there.
	Boundary bool
	// Filename is the path the file had in Commit; it differs from the
	// blamed path when blame followed a rename.
	Filename string
}

// blameObjectIDRe is a full lowercase SHA-1 or SHA-256 object id.
var blameObjectIDRe = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)

// Blame attributes lines start..end (1-based, inclusive) of path as of rev
// with `git blame --line-porcelain`. path is repo-relative.
//
// Repository configuration can change attribution, so Blame overrides the
// settings that would. blame.ignoreRevsFile re-attributes an ignored
// commit's lines to an older commit; the trailing empty --ignore-revs-file=
// clears that list. blame.showRoot=true hides root and shallow boundaries;
// -c blame.showRoot=false keeps both marked as boundaries. --no-textconv
// makes blame compare the stored bytes and not a configured conversion.
// core.quotePath=false keeps non-ASCII filenames unquoted, and any
// remaining C-style quoting is decoded.
//
// A range outside the file, a path missing at rev, or output that is not
// exactly one well-formed entry per requested line is an error, never a
// short or partial result.
func Blame(ctx context.Context, dir, rev, path string, start, end int) ([]BlameLine, error) {
	if rev == "" || strings.HasPrefix(rev, "-") {
		return nil, fmt.Errorf("gitx: Blame: invalid rev %q", rev)
	}
	if path == "" {
		return nil, fmt.Errorf("gitx: Blame: empty path")
	}
	if start < 1 || end < start {
		return nil, fmt.Errorf("gitx: Blame: invalid line range %d,%d", start, end)
	}
	out, err := run(ctx, dir,
		"-c", "blame.showRoot=false", "-c", "core.quotePath=false",
		"blame", "--line-porcelain", "--no-textconv", "--ignore-revs-file=",
		"-L", fmt.Sprintf("%d,%d", start, end), rev, "--", path)
	if err != nil {
		return nil, fmt.Errorf("gitx: Blame(%s:%s %d,%d): %w", rev, path, start, end, err)
	}
	lines, err := parseBlamePorcelain(out, start, end)
	if err != nil {
		return nil, fmt.Errorf("gitx: Blame(%s:%s %d,%d): %w", rev, path, start, end, err)
	}
	return lines, nil
}

// parseBlamePorcelain parses `--line-porcelain` output, which repeats the
// full header for every line: "<oid> <orig> <final> [<count>]", then
// key/value lines, then the line's content prefixed by a TAB. Only the
// boundary flag and filename are read from the key/value lines. The result
// must cover exactly start..end in order.
func parseBlamePorcelain(out []byte, start, end int) ([]BlameLine, error) {
	raw := bytes.Split(out, []byte("\n"))
	// A well-formed output ends with a newline, leaving one empty tail.
	if n := len(raw); n > 0 && len(raw[n-1]) == 0 {
		raw = raw[:n-1]
	}
	var lines []BlameLine
	i := 0
	for i < len(raw) {
		line, next, err := parseBlameEntry(raw, i)
		if err != nil {
			return nil, err
		}
		want := start + len(lines)
		if want > end {
			return nil, fmt.Errorf("blame output has more lines than the requested range %d,%d", start, end)
		}
		if line.FinalLine != want {
			return nil, fmt.Errorf("blame output line %d is out of order: want final line %d", line.FinalLine, want)
		}
		lines = append(lines, line)
		i = next
	}
	if len(lines) != end-start+1 {
		return nil, fmt.Errorf("blame output covers %d lines, want %d for range %d,%d", len(lines), end-start+1, start, end)
	}
	return lines, nil
}

// parseBlameEntry parses the one entry beginning at raw[i] and returns the
// index just past its content line.
func parseBlameEntry(raw [][]byte, i int) (BlameLine, int, error) {
	header := strings.Fields(string(raw[i]))
	if len(header) != 3 && len(header) != 4 {
		return BlameLine{}, 0, fmt.Errorf("malformed blame header %q", raw[i])
	}
	if !blameObjectIDRe.MatchString(header[0]) {
		return BlameLine{}, 0, fmt.Errorf("malformed blame object id %q", header[0])
	}
	orig, err := positiveLineNumber(header[1])
	if err != nil {
		return BlameLine{}, 0, err
	}
	final, err := positiveLineNumber(header[2])
	if err != nil {
		return BlameLine{}, 0, err
	}
	line := BlameLine{Commit: header[0], OrigLine: orig, FinalLine: final}
	haveFilename := false
	for j := i + 1; j < len(raw); j++ {
		text := string(raw[j])
		switch {
		case strings.HasPrefix(text, "\t"):
			if !haveFilename {
				return BlameLine{}, 0, fmt.Errorf("blame entry for final line %d has no filename", final)
			}
			return line, j + 1, nil
		case text == "boundary":
			line.Boundary = true
		case strings.HasPrefix(text, "filename "):
			if haveFilename {
				return BlameLine{}, 0, fmt.Errorf("blame entry for final line %d has two filenames", final)
			}
			name, err := unquoteBlameFilename(strings.TrimPrefix(text, "filename "))
			if err != nil {
				return BlameLine{}, 0, err
			}
			line.Filename = name
			haveFilename = true
		}
	}
	return BlameLine{}, 0, fmt.Errorf("blame entry for final line %d has no content line", final)
}

// positiveLineNumber parses a canonical 1-based line number.
func positiveLineNumber(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 || strconv.Itoa(n) != s {
		return 0, fmt.Errorf("malformed blame line number %q", s)
	}
	return n, nil
}

// unquoteBlameFilename decodes git's C-style quoting, which it applies to
// a filename carrying a quote, a backslash, or a control character.
func unquoteBlameFilename(s string) (string, error) {
	if strings.HasPrefix(s, `"`) {
		unquoted, err := strconv.Unquote(s)
		if err != nil {
			return "", fmt.Errorf("malformed quoted blame filename %s: %w", s, err)
		}
		s = unquoted
	}
	if s == "" {
		return "", fmt.Errorf("empty blame filename")
	}
	return s, nil
}
