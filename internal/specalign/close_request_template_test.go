// Lane E4b (docs/superpowers/plans/2026-09-23-unsealed-exemption-phase-b.md
// R-PB-4; ledger SI-252, SI-258 as amended by ruling R-PBW1-7): the
// committed, reviewed context-request template that close.yml fills for the
// CI close. The template holds everything that is authority (adapter,
// grants, phase, scope); the workflow fills only `spec`, from the validated
// dispatch input, and `verdi close` computes `expected` itself
// (cmd/verdi/conflictgate.go runConflictGate). This file pins the template's
// exact bytes and proves what they mean through the real request codec
// (internal/contextcompile), never a hand-rolled reading of the JSON.
package specalign

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/policyartifact"
)

// closeRequestTemplateRel is the template's repository-relative path: the
// file close.yml's instantiation step reads.
const closeRequestTemplateRel = ".github/verdi/close-context-request.json"

// closeRequestTemplateWant is the template's exact content: the canonical
// request with `spec` empty and no `expected` member. Adapter codex at
// version "1" is ruling R-PBW1-7 (SI-258 as amended): the compiler accepts
// whatever version the adopted constitution registers, the L4 adoption
// registers this one, and lane E6 validates that the two agree.
const closeRequestTemplateWant = `{"adapter":{"id":"codex","version":"1"},"grants":{"grants":[],"schema":"verdi.execution-grants/v1"},"phase":"review","schema":"verdi.context-compile-request/v1","scope":{"environments":[],"paths":[],"phases":["review"],"refs":[]},"spec":""}` + "\n"

// closeRequestTemplateDigest is the SHA-256 of closeRequestTemplateWant. A
// change to the template is a change to what the CI close is authorized to
// compile, so it must update this digest in the same reviewed commit.
const closeRequestTemplateDigest = "sha256:f859f378b0b3a5cb634557216bdfc54a562b07d12809ad095b5dd41802625340"

func closeRequestTemplatePath(root string) string {
	return filepath.Join(root, filepath.FromSlash(closeRequestTemplateRel))
}

func readCloseRequestTemplate(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(closeRequestTemplatePath(verdiRepoRoot))
	if err != nil {
		t.Fatalf("reading the close context-request template (R-PB-4: the CI close request comes from a committed, reviewed template): %v", err)
	}
	return raw
}

// closeRequestFor is the request the template means once close.yml has
// filled spec: adapter codex version 1, the empty grant set, phase review,
// scope phases [review] with every other dimension empty (universal,
// SI-83), and no expected claim.
func closeRequestFor(spec string) contextcompile.Request {
	return contextcompile.Request{
		Schema:  contextcompile.RequestSchema,
		Adapter: contextcompile.AdapterRef{ID: "codex", Version: "1"},
		Grants:  execworkspace.GrantSet{Grants: []execworkspace.Grant{}},
		Phase:   contextcompile.PhaseReview,
		Scope: policyartifact.Scope{
			Phases:       []string{string(contextcompile.PhaseReview)},
			Environments: []string{},
			Paths:        []string{},
			Refs:         []string{},
		},
		Spec: spec,
	}
}

// swapSpecMember replaces the one member `"spec":<from>` in doc with
// `"spec":<to>`, each value written verbatim between quotes. It refuses a
// document that does not carry exactly one such member, and a value that
// strconv.Quote would escape (a quote, a backslash, a control or
// non-printable character), for which a verbatim write is not its JSON
// string: every value close.yml can supply matches the validation step's
// pattern (lowercase letters, digits, '-' and '/'), which needs no escape.
func swapSpecMember(doc []byte, from, to string) ([]byte, error) {
	for _, v := range []string{from, to} {
		if q := strconv.Quote(v); q != `"`+v+`"` {
			return nil, fmt.Errorf("spec value %q needs a JSON escape (%s), which no validated spec_ref does", v, q)
		}
	}
	old := []byte(`"spec":"` + from + `"`)
	if n := bytes.Count(doc, old); n != 1 {
		return nil, fmt.Errorf("document carries %d members %s, want exactly 1", n, old)
	}
	return bytes.Replace(doc, old, []byte(`"spec":"`+to+`"`), 1), nil
}

func TestSwapSpecMember(t *testing.T) {
	tests := []struct {
		name     string
		doc      string
		from, to string
		want     string
		wantErr  string
	}{
		{"fills the empty spec", `{"phase":"review","spec":""}`, "", "spec/a", `{"phase":"review","spec":"spec/a"}`, ""},
		{"empties a filled spec", `{"spec":"spec/a-b"}`, "spec/a-b", "", `{"spec":""}`, ""},
		{"no spec member", `{"phase":"review"}`, "", "spec/a", "", "carries 0 members"},
		{"two spec members", `{"spec":"","x":{"spec":""}}`, "", "spec/a", "", "carries 2 members"},
		{"value needing an escape", `{"spec":""}`, "", `spec/a"b`, "", "needs a JSON escape"},
		{"non-printable value", `{"spec":""}`, "", "spec/\u2028", "", "needs a JSON escape"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := swapSpecMember([]byte(tt.doc), tt.from, tt.to)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("swapSpecMember error = %v, want one containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("swapSpecMember: %v", err)
			}
			if string(got) != tt.want {
				t.Fatalf("swapSpecMember = %s, want %s", got, tt.want)
			}
		})
	}
}

// TestCloseRequestTemplateBytesArePinned pins the template's exact bytes and
// their digest, so any edit to what the CI close may compile is a visible,
// reviewed change to this file (SI-252: "a workflow test pins the
// template").
func TestCloseRequestTemplateBytesArePinned(t *testing.T) {
	raw := readCloseRequestTemplate(t)
	if string(raw) != closeRequestTemplateWant {
		t.Errorf("%s:\n got %q\nwant %q", closeRequestTemplateRel, raw, closeRequestTemplateWant)
	}
	sum := sha256.Sum256(raw)
	if got := "sha256:" + hex.EncodeToString(sum[:]); got != closeRequestTemplateDigest {
		t.Errorf("%s: digest %s, want %s", closeRequestTemplateRel, got, closeRequestTemplateDigest)
	}
	wantSum := sha256.Sum256([]byte(closeRequestTemplateWant))
	if got := "sha256:" + hex.EncodeToString(wantSum[:]); got != closeRequestTemplateDigest {
		t.Errorf("closeRequestTemplateDigest %s is not the digest of closeRequestTemplateWant (%s)", closeRequestTemplateDigest, got)
	}
}

// TestCloseRequestTemplateIsTheCanonicalEncodingWithSpecEmpty proves the
// template is contextcompile.EncodeRequest's own output for the close request
// with only the spec value blanked, and that filling spec back in yields a
// request DecodeRequest accepts with exactly the intended meaning.
//
// EncodeRequest refuses an empty spec (validate.go: spec must be a whole
// spec/<name> ref), so the comparison runs the other way: encode a sample
// spec/<name> request, swap its spec value for "", and require the template
// bytes.
//
// Why close.yml's `jq -c -S --arg spec "$SPEC_REF" '.spec = $spec'` then
// produces EncodeRequest's exact bytes for every validated spec/<name>:
//   - sorted keys: -S sorts every object's keys by codepoint; canonjson
//     sorts them bytewise; for the ASCII keys here the two orders agree;
//   - compact: -c writes no whitespace, with ',' and ':' separators, as
//     canonjson does;
//   - no HTML escaping: jq never escapes '<', '>' or '&', and canonjson
//     disables that escaping;
//   - ASCII-only values that need no escape: every value is printable ASCII
//     without '"' or '\' (TestCloseRequestTemplateNeedsNoEscapesOrNumbers),
//     and the validated spec is lowercase letters, digits, '-' and '/', so
//     neither encoder escapes anything; the two differ only in escapes
//     (U+2028/U+2029, DEL, invalid UTF-8), which cannot arise;
//   - no numbers, booleans or nulls, whose renderings could differ: every
//     leaf is a string or an empty array;
//   - one trailing newline: jq ends each output with one, and canonjson
//     appends exactly one.
//
// So jq's output is the template with the spec value replaced, which is the
// swap this test proves equal to EncodeRequest's output.
func TestCloseRequestTemplateIsTheCanonicalEncodingWithSpecEmpty(t *testing.T) {
	template := readCloseRequestTemplate(t)
	accepted := regexp.MustCompile(specRefValidationPattern)
	for _, spec := range []string{"spec/a", "spec/vatc-machine-projections", "spec/a1-2b-c3"} {
		t.Run(spec, func(t *testing.T) {
			if !accepted.MatchString(spec) {
				t.Fatalf("sample %q is not a value close.yml's validation accepts (%s)", spec, specRefValidationPattern)
			}
			canonical, err := contextcompile.EncodeRequest(closeRequestFor(spec))
			if err != nil {
				t.Fatalf("EncodeRequest(close request for %s): %v", spec, err)
			}
			blanked, err := swapSpecMember(canonical, spec, "")
			if err != nil {
				t.Fatalf("blanking the canonical encoding's spec: %v", err)
			}
			if !bytes.Equal(blanked, template) {
				t.Fatalf("the template is not the canonical encoding with spec blanked:\n template %q\ncanonical %q", template, blanked)
			}

			filled, err := swapSpecMember(template, "", spec)
			if err != nil {
				t.Fatalf("filling the template's spec: %v", err)
			}
			if !bytes.Equal(filled, canonical) {
				t.Fatalf("the filled template %q is not EncodeRequest's output %q", filled, canonical)
			}
			decoded, err := contextcompile.DecodeRequest(filled)
			if err != nil {
				t.Fatalf("DecodeRequest refuses the filled template: %v", err)
			}
			if decoded.Schema != contextcompile.RequestSchema {
				t.Errorf("schema = %q, want %q", decoded.Schema, contextcompile.RequestSchema)
			}
			if want := (contextcompile.AdapterRef{ID: "codex", Version: "1"}); decoded.Adapter != want {
				t.Errorf("adapter = %+v, want %+v (ruling R-PBW1-7)", decoded.Adapter, want)
			}
			if len(decoded.Grants.Grants) != 0 {
				t.Errorf("grants = %+v, want the empty grant set (a CI close request authorizes no execution capability)", decoded.Grants.Grants)
			}
			if decoded.Phase != contextcompile.PhaseReview {
				t.Errorf("phase = %q, want %q (verdi close runs the review-phase conflict gate)", decoded.Phase, contextcompile.PhaseReview)
			}
			scope := decoded.Scope
			if !slices.Equal(scope.Phases, []string{"review"}) || len(scope.Environments) != 0 || len(scope.Paths) != 0 || len(scope.Refs) != 0 {
				t.Errorf("scope = %+v, want phases [review] and every other dimension empty (universal, SI-83)", scope)
			}
			if decoded.Spec != spec {
				t.Errorf("spec = %q, want %q", decoded.Spec, spec)
			}
			if decoded.Expected != nil {
				t.Errorf("expected = %+v, want absent (verdi close computes it from repository facts)", *decoded.Expected)
			}
		})
	}
	if bytes.Contains(template, []byte(`"expected"`)) {
		t.Errorf("%s carries an expected member; the lifecycle adapter computes expected, a template must not claim it", closeRequestTemplateRel)
	}
}

// TestCloseRequestTemplateFillsOnlyIntoAValidRequest proves the negative
// side: the template is not a request until close.yml fills spec with a
// spec/<name> ref, a tracker ref cannot fill it (why close.yml refuses
// tracker refs, SI-258), and a rendering jq would produce without -c, or
// with -j (no trailing newline), is not the canonical encoding DecodeRequest
// requires.
func TestCloseRequestTemplateFillsOnlyIntoAValidRequest(t *testing.T) {
	template := readCloseRequestTemplate(t)
	fill := func(t *testing.T, spec string) []byte {
		t.Helper()
		filled, err := swapSpecMember(template, "", spec)
		if err != nil {
			t.Fatalf("filling the template's spec with %q: %v", spec, err)
		}
		return filled
	}
	canonical := fill(t, "spec/a")
	var indented bytes.Buffer
	if err := json.Indent(&indented, canonical, "", "  "); err != nil {
		t.Fatalf("indenting the filled template: %v", err)
	}
	tests := []struct {
		name string
		doc  []byte
	}{
		{"the unfilled template (spec is empty)", template},
		{"a tracker ref in spec", fill(t, "jira:LOAN-1482")},
		{"a feature ref in spec", fill(t, "feature/a")},
		{"a spec ref with a fragment", fill(t, "spec/a#ac-1")},
		{"an indented rendering (jq without -c)", indented.Bytes()},
		{"no trailing newline (jq -j)", bytes.TrimSuffix(canonical, []byte("\n"))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := contextcompile.DecodeRequest(tt.doc); err == nil {
				t.Fatalf("DecodeRequest accepted %q, want a refusal", tt.doc)
			}
		})
	}
}

// TestCloseRequestTemplateNeedsNoEscapesOrNumbers checks the facts the jq
// equivalence above rests on: every byte of the template is printable ASCII
// apart from its one final newline, no byte is '\' (no string holds an
// escape), and every leaf value is a string (no numbers, booleans or nulls).
func TestCloseRequestTemplateNeedsNoEscapesOrNumbers(t *testing.T) {
	raw := readCloseRequestTemplate(t)
	body, ok := bytes.CutSuffix(raw, []byte("\n"))
	if !ok || bytes.Contains(body, []byte("\n")) {
		t.Fatalf("%s must end with exactly one newline and contain no other", closeRequestTemplateRel)
	}
	for i, b := range body {
		if b < 0x20 || b > 0x7e || b == '\\' {
			t.Errorf("%s byte %d is %q: only printable ASCII without '\\' keeps jq's and canonjson's renderings identical", closeRequestTemplateRel, i, b)
		}
	}
	var generic any
	if err := json.Unmarshal(body, &generic); err != nil {
		t.Fatalf("%s is not JSON: %v", closeRequestTemplateRel, err)
	}
	var walk func(path string, v any)
	walk = func(path string, v any) {
		switch val := v.(type) {
		case map[string]any:
			for k, child := range val {
				walk(path+"."+k, child)
			}
		case []any:
			for i, child := range val {
				walk(fmt.Sprintf("%s[%d]", path, i), child)
			}
		case string:
		default:
			t.Errorf("%s: %s is %T (%v); only strings, arrays and objects keep the two encoders byte-identical", closeRequestTemplateRel, path, v, v)
		}
	}
	walk("$", generic)
}
