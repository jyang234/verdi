package skillpack

import (
	"bytes"
	"context"
	"fmt"
	"regexp"

	"github.com/jyang234/verdi/internal/gitx"
)

// Rendered is one skill rendered for one host.
type Rendered struct {
	Host    Host
	Skill   string
	Path    string
	Content []byte
	Digest  string
}

var renderCommitRe = regexp.MustCompile(`^[0-9a-f]{40}$`)

// noRenderCommit is the render-commit stamp value when the root is not
// inside a git repository. vocab:identity — stamp grammar (identity)
const noRenderCommit = "none"

// RenderCommit is the 40-hex HEAD of the repository containing root, or
// "none" when root is not inside a git repository (R-W3-7).
func RenderCommit(ctx context.Context, root string) string {
	head, err := gitx.RevParse(ctx, root, "HEAD")
	if err != nil || !renderCommitRe.MatchString(head) {
		return noRenderCommit
	}
	return head
}

// Render renders skill for host: the embedded template with the
// generated-skill marker and the three stamps inserted immediately after
// the frontmatter's closing delimiter. renderCommit is 40 hex or "none".
func Render(h Host, skill, renderCommit string) (Rendered, error) {
	if h.dir() == "" {
		return Rendered{}, fmt.Errorf("skillpack: unknown host %q", h)
	}
	if renderCommit != noRenderCommit && !renderCommitRe.MatchString(renderCommit) {
		return Rendered{}, fmt.Errorf("skillpack: render commit %q must be 40 lowercase hex or %q", renderCommit, noRenderCommit)
	}
	tmpl, err := Template(skill)
	if err != nil {
		return Rendered{}, err
	}
	head, body, err := splitFrontmatter(tmpl)
	if err != nil {
		return Rendered{}, fmt.Errorf("skillpack: template %s: %w", skill, err)
	}
	var b bytes.Buffer
	b.Write(head)
	fmt.Fprintf(&b, "<!-- verdi:generated-skill host=%s skill=%s -->\n", h, skill)
	fmt.Fprintf(&b, "<!-- verdi:engine-digest %s -->\n", EngineDigest())
	fmt.Fprintf(&b, "<!-- verdi:template-digest %s -->\n", contentDigest(tmpl))
	fmt.Fprintf(&b, "<!-- verdi:render-commit %s -->\n", renderCommit)
	b.WriteString("<!-- verdi: this is a generated skill; edits here never change verdi, and any difference is reported as drift by `verdi harness check` until this file is regenerated with `verdi harness render`. -->\n")
	b.Write(body)
	content := b.Bytes()
	return Rendered{Host: h, Skill: skill, Path: Path(h, skill), Content: content, Digest: contentDigest(content)}, nil
}

var frontmatterClose = []byte("\n---\n")

// splitFrontmatter returns the frontmatter including its closing "---\n"
// line, and the body that follows. A template must open with "---\n" and
// carry a closing delimiter line.
func splitFrontmatter(tmpl []byte) (head, body []byte, err error) {
	if !bytes.HasPrefix(tmpl, []byte("---\n")) {
		return nil, nil, fmt.Errorf("template must open with a frontmatter delimiter")
	}
	i := bytes.Index(tmpl[3:], frontmatterClose)
	if i < 0 {
		return nil, nil, fmt.Errorf("template frontmatter has no closing delimiter")
	}
	cut := 3 + i + len(frontmatterClose)
	return tmpl[:cut], tmpl[cut:], nil
}
