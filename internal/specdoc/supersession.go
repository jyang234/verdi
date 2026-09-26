package specdoc

import (
	"fmt"
	"strings"

	"github.com/jyang234/verdi/internal/objsupersede"
)

// SupersessionFacts are the closed-spec object supersession views a
// consumer computed for this spec (design §6; SI-263, SI-279), supplied
// like every other fact: the document reports them and never derives
// them. Objects holds, by object id, the view of each of the spec's own
// criteria and decisions that a supersession touches — an id absent here
// is not superseded and renders as before. Decisions holds, by decision
// id, the views of each fragment `supersedes` edge the decision carries
// to a closed spec's object, in link order. Links maps a canonical ref a
// view names ("spec/S#dc-x", "spec/S_n", "conflict/<name>", "spec/T#o")
// to its URL on the consumer's surface; a ref with no entry renders as
// plain text. A nil SupersessionFacts means "not supplied for this
// render", and every object renders exactly as before.
type SupersessionFacts struct {
	Objects   map[string]objsupersede.ObjectView
	Decisions map[string][]objsupersede.DecisionView
	Links     map[string]string
}

// Supersession is what the document adds beside one object: the view's
// state id, the object a decision view's edge names (empty for an object
// view), and the view's §6 lines, each verbatim with the refs it names
// linked. Build fills it; a malformed view is Build's error, never a
// rendering (objsupersede's Lines contract).
type Supersession struct {
	State  string
	Object string
	Lines  []SupersessionLine
}

// SupersessionLine is one §6 line. Text is the view's line, verbatim.
// Kind names the line's place in its view, for surfaces and tests:
// governed, since, carry, or unproven for an object view; edge, carries,
// or not-established for a decision view. Disclosure marks a line that
// is a disclosure rather than a §6 text (SI-279: an unproven
// supersession or carry, a not-established reason), so surfaces can
// render it in the muted disclosure register. Links are the refs the
// text names, linked in place; Trailing are links rendered after the
// text — an object view's establishing conflict beside the line naming
// its deciding decision (§6: "linking to S's decision and to the
// conflict").
type SupersessionLine struct {
	Kind       string
	Text       string
	Disclosure bool
	Links      []RefLink
	Trailing   []RefLink
}

// RefLink is a canonical ref and its URL on the rendering surface.
type RefLink struct{ Ref, URL string }

// ObjectSupersession converts an object view; nil for an object the
// records do not supersede.
func ObjectSupersession(v objsupersede.ObjectView, links map[string]string) (*Supersession, error) {
	if v.State == objsupersede.ObjectNotSuperseded {
		return nil, nil
	}
	texts, err := v.Lines()
	if err != nil {
		return nil, err
	}
	var kinds []string
	switch v.State {
	case objsupersede.ObjectUnproven:
		kinds = []string{"unproven"}
	case objsupersede.ObjectSuperseded:
		kinds = []string{"governed", "since", "carry"}
	}
	s := &Supersession{State: string(v.State)}
	for i, text := range texts {
		if i >= len(kinds) {
			return nil, fmt.Errorf("specdoc: the %s view of %s rendered %d lines, more than its state has", v.State, v.Object, len(texts))
		}
		line := SupersessionLine{Kind: kinds[i], Text: text}
		// Links ride the §6 texts only: the deciding decision and the
		// conflict on the since line, the revision on a carried or
		// dropped carry line. A disclosure (an unproven supersession or
		// carry; SI-279) is left as it reads and marked as one.
		line.Disclosure = kinds[i] == "unproven" || (kinds[i] == "carry" && v.Carry == objsupersede.CarryUnproven)
		switch {
		case kinds[i] == "since":
			line.Links = namedLinks(text, links, v.By)
			if url := links[v.Conflict]; v.Conflict != "" && url != "" {
				line.Trailing = []RefLink{{Ref: v.Conflict, URL: url}}
			}
		case kinds[i] == "carry" && v.Carry != objsupersede.CarryUnproven:
			line.Links = namedLinks(text, links, v.Revision)
		}
		s.Lines = append(s.Lines, line)
	}
	return s, nil
}

// DecisionSupersession converts one decision edge's view.
func DecisionSupersession(v objsupersede.DecisionView, links map[string]string) (Supersession, error) {
	texts, err := v.Lines()
	if err != nil {
		return Supersession{}, err
	}
	kinds := []string{"edge", "carries"}
	if v.State == objsupersede.DecisionNotEstablished {
		kinds = []string{"not-established"}
	}
	s := Supersession{State: string(v.State), Object: v.Object}
	for i, text := range texts {
		if i >= len(kinds) {
			return Supersession{}, fmt.Errorf("specdoc: the %s view of %s's edge %s rendered %d lines, more than its state has", v.State, v.Decision, v.Edge, len(texts))
		}
		line := SupersessionLine{Kind: kinds[i], Text: text, Disclosure: kinds[i] == "not-established"}
		// Links ride the §6 and §5 texts only: the object on the edge
		// line, the establishing successor and its conflict on the
		// carries line. A not-established reason (SI-279) is left as it
		// reads and marked as a disclosure.
		switch kinds[i] {
		case "edge":
			line.Links = namedLinks(text, links, v.Object)
		case "carries":
			line.Links = namedLinks(text, links, v.Establisher, v.Conflict)
		}
		s.Lines = append(s.Lines, line)
	}
	return s, nil
}

// namedLinks returns, in the given order, each non-empty ref that text
// names as a whole token and that links resolves.
func namedLinks(text string, links map[string]string, refs ...string) []RefLink {
	var out []RefLink
	for _, ref := range refs {
		if url := links[ref]; ref != "" && url != "" && containsToken(text, ref) {
			out = append(out, RefLink{Ref: ref, URL: url})
		}
	}
	return out
}

// containsToken reports whether text names ref as a whole token: an
// occurrence bounded on both sides by the text's edge or a character no
// ref contains, so "spec/s1" is not found inside "spec/s1-v2" or
// "spec/s1#dc-1".
func containsToken(text, ref string) bool {
	if ref == "" {
		return false
	}
	for start := 0; ; {
		i := strings.Index(text[start:], ref)
		if i < 0 {
			return false
		}
		i += start
		end := i + len(ref)
		if (i == 0 || !refByte(text[i-1])) && (end == len(text) || !refByte(text[end])) {
			return true
		}
		start = i + 1
	}
}

// refByte reports whether b can occur inside a canonical ref (a kind, a
// slug, a pin, or an object fragment: 02 §Object model).
func refByte(b byte) bool {
	switch {
	case b >= 'a' && b <= 'z', b >= 'A' && b <= 'Z', b >= '0' && b <= '9':
		return true
	}
	return strings.IndexByte("-_./#@:", b) >= 0
}

// SupersessionLineMarkup renders one line as inline HTML for the Markdown
// text form (the store's engine passes inline HTML through, exactly as
// the object anchors already rely on): the text verbatim, each named ref
// replaced in place by its link, inside a span that names the object, the
// line's kind, and the view's state; trailing links follow the span.
// Markup only — no word is added to or taken from the line.
func SupersessionLineMarkup(stem string, s Supersession, line SupersessionLine) string {
	return supersessionLineMarkup(stem, s, line, false)
}

// SupersessionLineMarkdown is SupersessionLineMarkup for the Markdown
// text form: the same inline HTML, with the text's Markdown-significant
// punctuation backslash-escaped so a witness that carries backticks,
// asterisks, underscores, brackets or backslashes renders literally
// through the store's engine, never as code, emphasis or a link (the L5
// docs review's M-2). The engine unescapes them, so the HTML it produces
// equals SupersessionLineMarkup's.
func SupersessionLineMarkdown(stem string, s Supersession, line SupersessionLine) string {
	return supersessionLineMarkup(stem, s, line, true)
}

func supersessionLineMarkup(stem string, s Supersession, line SupersessionLine, markdown bool) string {
	escape := textEscaper.Replace
	if markdown {
		escape = func(t string) string { return markdownEscaper.Replace(textEscaper.Replace(t)) }
	}
	text := escape(line.Text)
	for _, l := range line.Links {
		text = replaceToken(text, escape(l.Ref), `<a href="`+escapeAttr(l.URL)+`">`+escape(l.Ref)+`</a>`)
	}
	class := "objsupersede objsupersede--" + escapeAttr(s.State)
	if line.Disclosure {
		class += " objsupersede--disclosure"
	}
	var b strings.Builder
	b.WriteString(`<span class="` + class + `" data-testid="objsupersede-` + escapeAttr(stem) + `-` + escapeAttr(line.Kind) + `" data-state="` + escapeAttr(s.State) + `">` + text + `</span>`)
	for _, l := range line.Trailing {
		b.WriteString(` <a class="objsupersede-conflict" data-testid="objsupersede-` + escapeAttr(stem) + `-conflict" href="` + escapeAttr(l.URL) + `">` + escape(l.Ref) + `</a>`)
	}
	return b.String()
}

// textEscaper guards a line's text inside the inline markup: only the
// three characters that could open or close a tag or an entity, so an
// apostrophe or a quote in a §6 text ("spec/T's completed work") stays
// verbatim in the Markdown form.
var textEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

// markdownEscaper backslash-escapes the ASCII punctuation CommonMark and
// GFM read as inline syntax — code spans, emphasis, links, images,
// strikethrough, and the escape character itself — so a line's text is
// rendered as written. A canonical ref carries none of these characters,
// so the token links stay whole.
var markdownEscaper = strings.NewReplacer(`\`, `\\`, "`", "\\`", "*", `\*`, "_", `\_`, "[", `\[`, "]", `\]`, "~", `\~`)

// replaceToken replaces the first whole-token occurrence of ref in text
// (containsToken's boundary rule) with repl; text is returned unchanged
// when it names no such token.
func replaceToken(text, ref, repl string) string {
	if ref == "" {
		return text
	}
	for start := 0; ; {
		i := strings.Index(text[start:], ref)
		if i < 0 {
			return text
		}
		i += start
		end := i + len(ref)
		if (i == 0 || !refByte(text[i-1])) && (end == len(text) || !refByte(text[end])) {
			return text[:i] + repl + text[end:]
		}
		start = i + 1
	}
}

// SupersessionStem is the data-testid stem of one Supersession beside
// object id: the id alone for the object's own view, the id and the
// edge's object (its "/" and "#" flattened to "-", as the board's
// ref-card testids are) for a decision view.
func SupersessionStem(id string, s Supersession) string {
	if s.Object == "" {
		return id
	}
	return id + "-" + strings.NewReplacer("/", "-", "#", "-").Replace(s.Object)
}
