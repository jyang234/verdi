package policyartifact

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Artifact kind names and schema IDs for the constitution store's own
// kinds. These are CI-owned constitution kinds (SI-6 assigns the
// directory's internal grammar, including its kind rows, to this unit);
// they are deliberately NOT rows in 02 §Kind registry's frozen table and
// never enter internal/artifact's knownKinds — their IDs are validated
// here, by their own closed grammar.
const (
	KindPolicy       = "policy"
	KindOverlay      = "policy-overlay"
	KindExemption    = "policy-exemption"
	KindConstitution = "policy-constitution"

	SchemaPolicy       = "verdi.policy/v1"
	SchemaOverlay      = "verdi.policy-overlay/v1"
	SchemaExemption    = "verdi.policy-exemption/v1"
	SchemaConstitution = "verdi.policy-constitution/v1"
)

var sha256Re = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// TemplateRecord is the resolved-scaffold provenance a created human
// artifact records (AC-1: "A created artifact records the resolved
// template identity and digest"). Identity is the shared scaffold
// resolver's source identity (internal/humanartifact); Digest is the
// sha256 of the resolved template bytes.
type TemplateRecord struct {
	Identity string `yaml:"identity" json:"identity"`
	Digest   string `yaml:"digest" json:"digest"`
}

// Validate checks the record's grammar.
func (t TemplateRecord) Validate() error {
	if t.Identity == "" {
		return fmt.Errorf("policyartifact: template.identity is required")
	}
	if !sha256Re.MatchString(t.Digest) {
		return fmt.Errorf("policyartifact: template.digest %q is not sha256:<64 hex> form", t.Digest)
	}
	return nil
}

// kernelDoc is the strict decode target for the immutable kernel fields
// every constitution artifact shares (AC-1/DC-4: identity, authority —
// expressed by the kind and its governing relationships — ownership, and
// template provenance; scope is per-kind).
//
// LIFECYCLE is deliberately not a frontmatter field: the constitution
// kinds are statusless, following the store's ratified statusless
// direction (VL-015's merge-signaled supersession for specs; the
// attestation kinds' "existence is the record"). An artifact committed
// under .verdi/policy/ IS active authority — its lifecycle state is
// presence on the default branch, derived from git, never an authorable
// status enum a hand edit could flip. Supersession and retirement flows
// are later-wave governance work over git history (DC-14/DC-15); the
// effective-policy resolver therefore treats every loaded artifact as
// live, by contract rather than omission.
type kernelDoc struct {
	Schema   *string         `yaml:"schema"`
	ID       *string         `yaml:"id"`
	Kind     *string         `yaml:"kind"`
	Title    *string         `yaml:"title"`
	Owners   *[]string       `yaml:"owners"`
	Template *TemplateRecord `yaml:"template"`
}

// kernel is the validated, normalized kernel value.
type kernel struct {
	Schema   string
	ID       string
	Kind     string
	Title    string
	Owners   []string
	Template *TemplateRecord
}

// toKernel enforces presence, schema/kind/id agreement, and the shared
// kernel grammar. wantSchema and wantKind pin the artifact's declared
// identity; unknown or mismatched values fail closed.
func (d kernelDoc) toKernel(wantSchema, wantKind string) (kernel, error) {
	missing := func(field string) error {
		return fmt.Errorf("policyartifact: %s field %s is missing: every kernel field is mandatory", wantKind, field)
	}
	switch {
	case d.Schema == nil:
		return kernel{}, missing("schema")
	case d.ID == nil:
		return kernel{}, missing("id")
	case d.Kind == nil:
		return kernel{}, missing("kind")
	case d.Title == nil:
		return kernel{}, missing("title")
	case d.Owners == nil:
		return kernel{}, missing("owners")
	}
	k := kernel{
		Schema:   *d.Schema,
		ID:       *d.ID,
		Kind:     *d.Kind,
		Title:    *d.Title,
		Owners:   *d.Owners,
		Template: d.Template,
	}
	if k.Schema != wantSchema {
		return kernel{}, fmt.Errorf("policyartifact: schema %q is not %q (unknown schemas fail closed)", k.Schema, wantSchema)
	}
	if k.Kind != wantKind {
		return kernel{}, fmt.Errorf("policyartifact: kind field %q does not match expected kind %q", k.Kind, wantKind)
	}
	if _, err := parseKindedID(k.ID, wantKind); err != nil {
		return kernel{}, err
	}
	if strings.TrimSpace(k.Title) == "" {
		return kernel{}, fmt.Errorf("policyartifact: title is required and must not be blank")
	}
	// A title is single-line prose. It is rendered verbatim into
	// generated instruction projections (internal/instructionprojection
	// writes a per-policy "## <title> (<id>)" line), so a newline or
	// carriage return inside one would emit extra, header-shaped lines
	// into a generated file that reviewing the artifact's own title
	// would not reveal. Every control character is rejected by one
	// uniform rule (rune < 0x20, or 0x7f) rather than an enumerated
	// blocklist: a tab is not meaningful in single-line prose either,
	// and a uniform rule has no forgotten character to probe for.
	for _, r := range k.Title {
		if r < 0x20 || r == 0x7f {
			return kernel{}, fmt.Errorf("policyartifact: title %q contains a control character (U+%04X); a title is single-line prose", k.Title, r)
		}
	}
	if len(k.Owners) == 0 {
		return kernel{}, fmt.Errorf("policyartifact: owners must list at least one owner")
	}
	// Owners carry the store's kebab-case owner-handle grammar (the
	// convention every committed artifact already follows: platform-team,
	// service-team). DC-17 resolves an owner handle to an authenticated
	// principal downstream; an ungrammatical handle could never resolve.
	if err := uniqueSet("owners", k.Owners, func(o string) error {
		if !kebabRe.MatchString(o) {
			return fmt.Errorf("owner %q must be a kebab-case owner handle", o)
		}
		return nil
	}); err != nil {
		return kernel{}, err
	}
	sort.Strings(k.Owners)
	if k.Template != nil {
		if err := k.Template.Validate(); err != nil {
			return kernel{}, err
		}
	}
	return k, nil
}

// parseKindedID validates an "<kind>/<name>" constitution artifact id
// and returns its name half.
func parseKindedID(id, wantKind string) (string, error) {
	prefix := wantKind + "/"
	if !strings.HasPrefix(id, prefix) {
		return "", fmt.Errorf("policyartifact: id %q must have the form %s<name>", id, prefix)
	}
	name := strings.TrimPrefix(id, prefix)
	if !kebabRe.MatchString(name) {
		return "", fmt.Errorf("policyartifact: id %q name %q must be kebab-case", id, name)
	}
	return name, nil
}

// nameOf returns the name half of an already-validated kinded id.
func nameOf(id string) string {
	if i := strings.LastIndex(id, "/"); i >= 0 {
		return id[i+1:]
	}
	return id
}

// requireRationale enforces the kernel's rationale requirement: the
// artifact body must carry non-blank prose (AC-1's kernel records
// rationale; a rule with no stated why is not reviewable authority).
func requireRationale(kind string, body []byte) (string, error) {
	text := string(body)
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("policyartifact: %s body must carry a non-empty rationale", kind)
	}
	return text, nil
}
