package skillpack

import (
	"bytes"
	"fmt"
	"strings"
)

// Step is one line of a template's verdi-sequence block.
//
// Grammar (one step per line, blank lines ignored):
//
//	call <tool> [key=value ...]   a tool call the skill makes
//	show                          the skill shows the human a proposal or result
//	confirm                       the skill waits for the human's confirmation
//	loop                          opens a body repeated once per item
//	end                           closes the body
type Step struct {
	Kind string
	Tool string
	Args map[string]string
}

// Sequence is a template's declared tool sequence (R-W3-6).
type Sequence []Step

var (
	fenceOpen  = []byte("```verdi-sequence\n")
	fenceClose = []byte("```\n")
	// fenceCloseBare is the fenceClose fallback for a block whose closing
	// fence is the template's very last bytes, with no trailing newline.
	fenceCloseBare = []byte("```")
)

// ParseSequence extracts and parses the single verdi-sequence block of a
// template. Zero or two blocks, an empty block, an unknown step, a call
// without a tool, a call with a repeated argument key, or an unbalanced
// loop is an error.
//
// Line endings are normalized (CRLF tolerated, stripped to LF) and a
// closing fence at end-of-input with no trailing newline is accepted, so
// a checked-out SKILL.md a human or a text editor has lightly touched
// still parses (task-1-review.md finding 6); the embedded templates
// Template() serves are always LF and newline-terminated, so this only
// widens what a copy on disk may look like, never what this package
// itself produces.
func ParseSequence(template []byte) (Sequence, error) {
	template = bytes.ReplaceAll(template, []byte("\r"), nil)
	start := bytes.Index(template, fenceOpen)
	if start < 0 {
		return nil, fmt.Errorf("skillpack: template has no verdi-sequence block")
	}
	rest := template[start+len(fenceOpen):]
	stop, closeLen := -1, 0
	if i := bytes.Index(rest, fenceClose); i >= 0 {
		stop, closeLen = i, len(fenceClose)
	} else if bytes.HasSuffix(rest, fenceCloseBare) {
		stop, closeLen = len(rest)-len(fenceCloseBare), len(fenceCloseBare)
	}
	if stop < 0 {
		// vocab:identity — "closed" names the unterminated code fence, not the lifecycle state
		return nil, fmt.Errorf("skillpack: verdi-sequence block is not closed")
	}
	if bytes.Contains(rest[stop+closeLen:], fenceOpen) {
		return nil, fmt.Errorf("skillpack: template has more than one verdi-sequence block")
	}
	var seq Sequence
	depth := 0
	for _, line := range strings.Split(string(rest[:stop]), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "call":
			if len(fields) < 2 {
				return nil, fmt.Errorf("skillpack: verdi-sequence: call without a tool name")
			}
			st := Step{Kind: "call", Tool: fields[1], Args: map[string]string{}}
			for _, kv := range fields[2:] {
				k, v, ok := strings.Cut(kv, "=")
				if !ok || k == "" || v == "" {
					return nil, fmt.Errorf("skillpack: verdi-sequence: bad argument %q", kv)
				}
				if _, dup := st.Args[k]; dup {
					return nil, fmt.Errorf("skillpack: verdi-sequence: duplicate argument %q", k)
				}
				st.Args[k] = v
			}
			seq = append(seq, st)
		case "show", "confirm":
			if len(fields) != 1 {
				return nil, fmt.Errorf("skillpack: verdi-sequence: %s takes no arguments", fields[0])
			}
			seq = append(seq, Step{Kind: fields[0]})
		case "loop":
			depth++
			seq = append(seq, Step{Kind: "loop"})
		case "end":
			depth--
			if depth < 0 {
				return nil, fmt.Errorf("skillpack: verdi-sequence: end without loop")
			}
			seq = append(seq, Step{Kind: "end"})
		default:
			return nil, fmt.Errorf("skillpack: verdi-sequence: unknown step %q", fields[0])
		}
	}
	if depth != 0 {
		return nil, fmt.Errorf("skillpack: verdi-sequence: loop without end")
	}
	if len(seq) == 0 {
		return nil, fmt.Errorf("skillpack: verdi-sequence block is empty")
	}
	return seq, nil
}
