package signedapproval

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

// rowStates renders an artifact's rows for a failure message.
func rowStates(a Artifact) string {
	parts := make([]string, 0, len(a.Rows))
	for _, r := range a.Rows {
		parts = append(parts, fmt.Sprintf("(%s: %s %s)", r.Role, r.State, r.Reason))
	}
	return strings.Join(parts, " ")
}

// TestAuthenticate_NonstandardLineBreakAtHead pins the F1 review probes: an
// unsigned commit adds a row whose line carries a line break YAML counts and
// Git does not, so YAML's row span swallows the next Git line. Without the
// one line-break convention the signed row stayed authenticated over an
// injected frontmatter key (one break) or a replaced body (two lone CRs
// pushing the span past the closing delimiter). At the head such a
// frontmatter is an operational error (SI-256).
func TestAuthenticate_NonstandardLineBreakAtHead(t *testing.T) {
	const pre = "---\nschema: test/v1\nid: exemption/x\napprovals:\n"
	injected := func(brk string) func(p1, p2 string) string {
		return func(p1, p2 string) string {
			return pre + "  - role: " + ownerRole + "\n    principal: " + p1 + "\n" +
				"  - role: " + escalationRole + brk + "    principal: " + p2 + "\n" +
				"review_condition: injected after the approval\n---\n" + defaultBody
		}
	}
	tests := []struct {
		name string
		head func(p1, p2 string) string
	}{
		{name: "lone CR hides an added key", head: injected("\r")},
		{name: "NEL hides an added key", head: injected("\u0085")},
		{name: "LS hides an added key", head: injected("\u2028")},
		{name: "PS hides an added key", head: injected("\u2029")},
		{name: "two lone CRs hide a replaced body", head: func(p1, p2 string) string {
			return pre + "  - role: " + ownerRole + "\n    principal: " + p1 + "\n" +
				"  - role: " + escalationRole + "\r\r    principal: " + p2 + "\n" +
				"---\nEVIL RATIONALE: the departure is unbounded.\n---\n" + defaultBody
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := newTestRepo(t)
			p1, p2 := principalFor(t, signedSource, "1001"), principalFor(t, signedSource, "2002")
			r.write(artifactPath, pre+"---\n"+defaultBody)
			r.write("other.txt", "base\n")
			r.commit("draft")
			r.write(artifactPath, pre+"  - role: "+ownerRole+"\n    principal: "+p1+"\n---\n"+defaultBody)
			c := r.commit("approve (signed by 1001)")
			r.write(artifactPath, tc.head(p1, p2))
			u := r.commit("unsigned: add a row and hide a change")
			v := &fakeVerifier{facts: map[string]CommitVerification{c: verifiedBy(c, "1001"), u: unverified(u)}}
			a, err := Authenticate(context.Background(), Input{Root: r.dir, Head: u, Path: artifactPath, Profile: soloProfile(t), Verifier: v})
			if !errors.Is(err, artifact.ErrNonstandardLineBreak) {
				t.Fatalf("Authenticate = rows %s, error %v: want ErrNonstandardLineBreak", rowStates(a), err)
			}
			if len(v.calls) != 0 {
				t.Fatalf("verifier calls = %v, want none for a refused head", v.calls)
			}
		})
	}
}
