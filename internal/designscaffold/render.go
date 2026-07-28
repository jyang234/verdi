package designscaffold

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/jyang234/verdi/internal/artifact"
)

// embeddedTemplates carries the canonical scaffold templates (spec/
// scaffold-templates ac-1) — the shipped default every class's Class.
// Template filename resolves to when a store carries no override of its
// own, precedent internal/dex/assets.go / internal/model/embed.go's
// Canonical().
//
//go:embed templates/*.md
var embeddedTemplates embed.FS

// ScaffoldData is Render's one input shape (fixed signature, docs/design/
// plans/2026-07-17-extensibility-phase1-plan.md Task 8): every field a
// class's template might reference when scaffolding a fresh spec. Not
// every template uses every field — the canonical feature.md ignores
// Spike/Links/ParentRef entirely, and a plain (non-spike) story ignores
// nothing story.md itself defines — a template failing to reference a
// field is never an error; only referencing an UNDEFINED one is (Render's
// missingkey=error posture, mirroring struct field access semantics: an
// unknown field name is already a template execution error by
// construction, since ScaffoldData is a struct, not a map).
type ScaffoldData struct {
	Ref       string
	Title     string
	Owners    string
	Problem   string
	Outcome   string
	StoryRef  string
	ParentRef string
	Links     []StoryLink
	Spike     bool
	// Pins and Dispositions are commit-to-design's content-carrying
	// fields (spec/creation-form ac-4 — the "content-carrying
	// template-contract extension" ledger L-M12's ratification
	// predicted): the board's pinned refs render as context: entries,
	// the sticky dispositions as the dispositions: block. Every other
	// consumer leaves them zero; a template failing to reference them
	// is never an error (the struct posture above).
	Pins         []artifact.Pin
	Dispositions []artifact.Disposition
}

// Render instantiates tmpl (a text/template source: an embedded canonical
// template, or a store's own .verdi/templates/<name>.md override) against
// data, failing closed on any template syntax error or undefined-field
// access rather than producing a scaffold silently missing content. Every
// real caller self-validates the result afterward (SplitFrontmatter +
// DecodeSpec, exactly as design start and stub-instantiate already did
// before this story); `verdi model check` (ac-3) runs the identical
// instantiate-then-strict-decode round trip proactively, over every
// resolved template, so a broken template is caught at check time rather
// than at a scaffold consumer's first use.
//
// The "safe" function (K4, verified latent at the round's final review) is
// registered here so it is available to the embedded canonical templates
// AND any store override alike — see safeScalar's own doc comment for what
// it guards and why it is a conditional guard rather than an unconditional
// %q.
func Render(tmpl []byte, data ScaffoldData) (string, error) {
	t, err := template.New("scaffold").Funcs(template.FuncMap{"safe": safeScalar}).Option("missingkey=error").Parse(string(tmpl))
	if err != nil {
		return "", fmt.Errorf("designscaffold: parsing template: %w", err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("designscaffold: rendering template: %w", err)
	}
	return buf.String(), nil
}

// safeScalar guards a template position that renders a caller-supplied
// string BARE into YAML frontmatter — id:, owners:, story: in the
// canonical templates — against the newline-smuggle path K4 (final-review
// residual, verified latent) named: with no escaping at all, a value
// containing an embedded "\nsomekey: value" line renders straight through
// as a second, illegitimate frontmatter key, and a value containing a bare
// double quote or a ": " sequence can prematurely end or corrupt the
// surrounding YAML plain scalar. No current caller can trigger this (every
// real Ref/StoryRef is a validated kebab ref or scheme-prefixed tracker
// key, and Owners is always the fixed "[unassigned]" flow-sequence
// literal) — it is closed here defensively, at the mechanism, rather than
// left as a latent trap for the next caller.
//
// It is deliberately NOT an unconditional %q the way title: already uses
// ({{printf "%q" .Title}}, the established precedent this mirrors):
// quoting unconditionally would change the rendered BYTES for every
// current safe input (a bare kebab ref like "spec/foo-bar" becomes
// "\"spec/foo-bar\"" under %q — a different byte sequence that still
// decodes to the identical Go string, but breaks TestByteForByte's pin,
// spec/scaffold-templates ac-1's stronger-than-decode-equivalence floor),
// and for owners: specifically it would be actively WRONG: that position
// holds a YAML flow-SEQUENCE literal ("[unassigned]", "[platform-team,
// qa-lead]"), not a scalar — %q-wrapping it would turn a list into a
// string and change the decoded TYPE, not merely the bytes. So: pass s
// through completely unchanged when it cannot corrupt or extend the
// surrounding YAML (no newline, no double quote, no ": " that would open a
// new mapping pair or end a plain scalar early); %q-quote it (Go's own
// backslash/quote/newline escaping) the rest of the time. Byte-identical
// to today's bare rendering for every current input; fails closed (a
// smuggled value decodes as a quoted STRING as the affected field's own
// declared type — a list field like owners: then fails to decode as
// []string — never silently as a second key) the moment one ever isn't.
func safeScalar(s string) string {
	if strings.ContainsAny(s, "\n\"") || strings.Contains(s, ": ") {
		return fmt.Sprintf("%q", s)
	}
	return s
}

// LoadTemplate resolves filename's template bytes: a store override at
// .verdi/templates/<filename> under root when one exists, else the
// embedded canonical template of the same name (spec/scaffold-templates
// outcome: "a store with no templates/ directory at all changes
// nothing" — the same absence-changes-nothing posture store.Open's own
// model.yaml resolution already established, internal/store/open.go's
// loadModel). filename is a class's own Class.Template value (model.Class,
// kernel-required non-empty) — callers resolve it from the store's
// already-open model, never hardcode a class-to-filename mapping here.
//
// LoadTemplate composes the two halves below (spec/creation-form ac-4
// exposed them): LoadOverride for the store's own layer, Canonical for
// the embedded fallback of the same name. A consumer whose canonical
// default is NOT the same-named embedded class template — commit-to-
// design, whose no-override shape is the byte-pinned legacy scaffold —
// composes them differently; everyone else keeps calling this.
func LoadTemplate(root, filename string) ([]byte, error) {
	data, ok, err := LoadOverride(root, filename)
	if err != nil {
		return nil, err
	}
	if ok {
		return data, nil
	}
	return Canonical(filename)
}

// LoadOverride reads the store's own template override for filename at
// .verdi/templates/<filename> under root: (bytes, true, nil) when one
// exists, (nil, false, nil) when the store carries none — absence is
// never an error (the absence-changes-nothing posture) — and a real
// read failure or an unsafe filename fails closed.
func LoadOverride(root, filename string) ([]byte, bool, error) {
	// Defense-in-depth on the containment invariant internal/model's
	// Model.Validate kernel rule already enforces (judged-template-filename-
	// escapes-templates-dir): filename must be a BARE filename, so the join
	// below cannot escape .verdi/templates/. Rejecting a separator-carrying,
	// absolute, or . / .. value HERE keeps the invariant even for a caller
	// that reaches LoadOverride without a Validate-clean model — a specific,
	// fail-closed refusal naming the rule rather than an incidental
	// file-not-found on a resolved-elsewhere path. One shared definition
	// (artifact.IsBareFilename) backs both layers.
	if !artifact.IsBareFilename(filename) {
		return nil, false, fmt.Errorf("designscaffold: template filename %q must be a bare filename under .verdi/templates/ (no path separator, absolute path, or . / ..)", filename)
	}
	override := filepath.Join(root, ".verdi", "templates", filename)
	data, err := os.ReadFile(override)
	if err == nil {
		return data, true, nil
	}
	if !os.IsNotExist(err) {
		return nil, false, fmt.Errorf("designscaffold: reading template override %s: %w", override, err)
	}
	return nil, false, nil
}

// Canonical returns the embedded canonical template named filename —
// the shipped default a store with no override of its own resolves to.
func Canonical(filename string) ([]byte, error) {
	data, err := embeddedTemplates.ReadFile("templates/" + filename)
	if err != nil {
		return nil, fmt.Errorf("designscaffold: no embedded canonical template named %q: %w", filename, err)
	}
	return data, nil
}
