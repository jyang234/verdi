package dex

import (
	"bytes"
	"fmt"
)

// The marker pair that delimits a workbench-only block in assets/style.css
// (ledger SI-322): the workbench serves the whole stylesheet, and the docs
// site's copy has every such block removed, so a workbench chrome change
// can live in the one shared stylesheet without changing a byte of the
// docs build (spec/chrome-and-tokens-v2 ac-4, dc-1). Each marker stands
// alone on its line; a block is the whole lines from its begin line
// through its end line.
const (
	workbenchOnlyBegin = "/* verdi:workbench-only:begin */"
	workbenchOnlyEnd   = "/* verdi:workbench-only:end */"
	// workbenchOnlyStem is what both markers share: a line carrying it in
	// any other form is a malformed marker, refused rather than left in
	// the docs build.
	workbenchOnlyStem = "verdi:workbench-only"
)

// docsStyleCSS returns the docs site's stylesheet: StyleCSS, the
// stylesheet the workbench serves, with every workbench-only block
// removed. Anything the docs build derives from its stylesheet's bytes
// derives from these.
func docsStyleCSS() ([]byte, error) {
	full, err := StyleCSS()
	if err != nil {
		return nil, err
	}
	return stripWorkbenchOnly(full)
}

// stripWorkbenchOnly returns css with every workbench-only block removed
// byte-exactly: the begin marker's line, every line up to the end marker's
// line, and that line with its newline. Every other byte is kept, so css
// with no block comes back unchanged. A marker that does not stand alone
// on its line, a begin inside an open block, an end with no open block,
// and a block never closed are each an error naming the line: the build
// fails rather than ship a stylesheet whose blocks it cannot delimit.
func stripWorkbenchOnly(css []byte) ([]byte, error) {
	var out bytes.Buffer
	out.Grow(len(css))
	open := 0 // the line the open block began on; 0 when none is open
	rest := css
	for n := 1; len(rest) > 0; n++ {
		line := rest
		if i := bytes.IndexByte(rest, '\n'); i >= 0 {
			line = rest[:i+1]
		}
		rest = rest[len(line):]

		if !bytes.Contains(line, []byte(workbenchOnlyStem)) {
			if open == 0 {
				out.Write(line)
			}
			continue
		}
		switch string(bytes.TrimSpace(line)) {
		case workbenchOnlyBegin:
			if open != 0 {
				return nil, fmt.Errorf("dex: style.css line %d: nested workbench-only begin marker (the block opened at line %d is still open)", n, open)
			}
			open = n
		case workbenchOnlyEnd:
			if open == 0 {
				return nil, fmt.Errorf("dex: style.css line %d: workbench-only end marker with no open block", n)
			}
			open = 0
		default:
			return nil, fmt.Errorf("dex: style.css line %d: malformed workbench-only marker: each of %q and %q must stand alone on its line", n, workbenchOnlyBegin, workbenchOnlyEnd)
		}
	}
	if open != 0 {
		return nil, fmt.Errorf("dex: style.css line %d: workbench-only block has no end marker", open)
	}
	return out.Bytes(), nil
}
