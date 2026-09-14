package specimport

import (
	"bytes"
	"fmt"
)

// stripFrontmatter recognizes an optional leading YAML frontmatter block
// (spec-import-contract.md, "In external markdown-v1, a leading YAML
// frontmatter block (beginning with a standalone `---`, ending with a
// later standalone `---` or `...`) is retained-only opaque metadata, never
// native field authority. Recognize this block before goldmark parsing so
// its closing delimiter cannot become a setext heading. An opening
// delimiter without a closing delimiter is invalid-source.").
//
// It returns bodyStart, the byte offset in data where the remainder
// begins (0 when data has no frontmatter block at all). Every downstream
// span is computed relative to the ORIGINAL data
// (spec-import-contract.md: "Maintain offsets into the original selected
// bytes when parsing the remainder") — callers use bodyStart only to
// decide which prefix to exclude from goldmark parsing, never to rebase a
// reported offset.
//
// A delimiter line must be standalone: exactly "---" or "..." once its own
// line terminator is removed, with no other content on that physical line.
func stripFrontmatter(data []byte) (bodyStart int, err error) {
	offsets := physicalLineOffsets(data)
	total := len(offsets) - 1
	if total == 0 || lineContentAt(data, offsets, 0) != "---" {
		return 0, nil
	}
	for i := 1; i < total; i++ {
		content := lineContentAt(data, offsets, i)
		if content == "---" || content == "..." {
			return offsets[i+1], nil
		}
	}
	return 0, fmt.Errorf("%w: leading frontmatter delimiter on line 1 is never closed by a later standalone '---' or '...' line", ErrInvalidSource)
}

// lineContentAt returns physical line i's (0-based) content with any
// trailing line terminator (CRLF or LF) removed, for delimiter-line
// comparison only — it is never used to build reported field/span text.
func lineContentAt(data []byte, offsets []int, i int) string {
	line := data[offsets[i]:offsets[i+1]]
	line = bytes.TrimSuffix(line, []byte("\n"))
	line = bytes.TrimSuffix(line, []byte("\r"))
	return string(line)
}
