package specdoc

import (
	"strings"
)

// bodySections splits a spec body on its level-two headings. The scaffold
// and every authored spec key object bodies as "## <id>" (02 §Object
// model anchors), so the id under a heading is the map key; a prose
// heading such as "## Problem" is keyed by its lowercased text. Text is
// trimmed of surrounding blank lines; level-three headings and below stay
// inside their section. A "## " line inside a fenced code block (a line
// opening with ``` or ~~~) is fence content, never a heading (fix round,
// F9 — a detail body quoting a "## " line inside its own fenced example,
// e.g. documenting this very package's own object-anchor convention,
// must not fracture the section it belongs to).
func bodySections(body []byte) map[string]string {
	out := map[string]string{}
	if len(body) == 0 {
		return out
	}
	lines := strings.Split(strings.ReplaceAll(string(body), "\r\n", "\n"), "\n")
	key := ""
	var buf []string
	flush := func() {
		if key != "" {
			out[key] = strings.Trim(strings.Join(buf, "\n"), "\n")
		}
		buf = buf[:0]
	}
	inFence := false
	var fenceChar byte
	fenceLen := 0
	for _, line := range lines {
		if marker, n := fenceMarker(line); n > 0 {
			switch {
			case !inFence:
				inFence, fenceChar, fenceLen = true, marker, n
			case marker == fenceChar && n >= fenceLen:
				inFence = false
			}
		} else if !inFence && strings.HasPrefix(line, "## ") {
			flush()
			key = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(line, "## ")))
			continue
		}
		if key != "" {
			buf = append(buf, line)
		}
	}
	flush()
	return out
}

// fenceMarker reports whether line opens or closes a fenced code block: a
// run of three or more identical backtick or tilde characters, optionally
// preceded by leading whitespace and followed by anything (an opening
// fence's info string, or a closing fence's trailing whitespace — this
// package only needs to track fence boundaries, not validate them against
// the full CommonMark fenced-code-block grammar). Returns a zero byte and
// 0 when line is not a fence marker.
func fenceMarker(line string) (byte, int) {
	trimmed := strings.TrimLeft(line, " \t")
	if trimmed == "" {
		return 0, 0
	}
	c := trimmed[0]
	if c != '`' && c != '~' {
		return 0, 0
	}
	n := 0
	for n < len(trimmed) && trimmed[n] == c {
		n++
	}
	if n < 3 {
		return 0, 0
	}
	return c, n
}
