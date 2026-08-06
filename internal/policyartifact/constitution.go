package policyartifact

import (
	"fmt"
	"sort"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/governanceprincipal"
)

// ConstitutionName is the fixed name (and filename stem) of the one
// constitution manifest a store carries at .verdi/policy/constitution.md.
const ConstitutionName = "constitution"

// Constitution is the store-level constitution manifest: it selects the
// canonical governance profile (AC-1: "The constitution also selects a
// canonical governance profile"), registers the project's governance
// catalog (the duplicate-free catalog the kernel's DecodeProfile
// requires, SI-18), registers the constraint-subject catalogs projects
// contribute under DC-5 (subjects per Verdi-owned family, plus the
// environment scope values), and declares the harness adapters whose
// instruction projections this store generates (DC-1's adapter-driven
// projection rules; the adapter boundary stays generic data — paths and
// filenames — never executable configuration).
type Constitution struct {
	Schema          string            `json:"schema"`
	ID              string            `json:"id"`
	Kind            string            `json:"kind"`
	Title           string            `json:"title"`
	Owners          []string          `json:"owners"`
	SelectedProfile string            `json:"selected_profile"`
	Environments    []string          `json:"environments"`
	Catalog         GovernanceCatalog `json:"catalog"`
	Subjects        SubjectCatalog    `json:"subjects"`
	Adapters        []Adapter         `json:"adapters"`
	Template        *TemplateRecord   `json:"template,omitempty"`
	Rationale       string            `json:"rationale"`

	seal string
}

// GovernanceCatalog registers the project's governance vocabulary — the
// injected duplicate-free catalog governance-profile validation resolves
// against (SI-18). The kernel owns the catalog TYPE's semantics
// (governanceprincipal.Catalog); this manifest owns its committed
// registration.
type GovernanceCatalog struct {
	Roles             []string `yaml:"roles" json:"roles"`
	Transitions       []string `yaml:"transitions" json:"transitions"`
	EvidenceSources   []string `yaml:"evidence_sources" json:"evidence_sources"`
	EscalationMetrics []string `yaml:"escalation_metrics" json:"escalation_metrics"`
}

// SubjectCatalog registers the concrete constraint subjects a project's
// policy claims may name, per Verdi-owned family (DC-5: projects
// register subjects; they cannot register executable semantics).
type SubjectCatalog struct {
	Action        []string `yaml:"action" json:"action"`
	Configuration []string `yaml:"configuration" json:"configuration"`
	Capability    []string `yaml:"capability" json:"capability"`
	Resource      []string `yaml:"resource" json:"resource"`
	Identity      []string `yaml:"identity" json:"identity"`
	Evidence      []string `yaml:"evidence" json:"evidence"`
}

// Family returns the registered subjects for family f.
func (s SubjectCatalog) Family(f Family) []string {
	switch f {
	case FamilyAction:
		return s.Action
	case FamilyConfiguration:
		return s.Configuration
	case FamilyCapability:
		return s.Capability
	case FamilyResource:
		return s.Resource
	case FamilyIdentity:
		return s.Identity
	case FamilyEvidence:
		return s.Evidence
	}
	return nil
}

// Has reports whether subject is registered for family f.
func (s SubjectCatalog) Has(f Family, subject string) bool {
	for _, m := range s.Family(f) {
		if m == subject {
			return true
		}
	}
	return false
}

// Adapter declares one harness adapter's projection surface: the managed
// projection files this store generates for it and the instruction
// filenames its harness discovers project-wide (AC-1: "The adapter
// enumerates the harness's effective project-level instruction discovery
// chain, including nested instruction files").
type Adapter struct {
	ID                 string   `yaml:"id" json:"id"`
	Version            string   `yaml:"version" json:"version"`
	Managed            []string `yaml:"managed" json:"managed"`
	DiscoveryFilenames []string `yaml:"discovery_filenames" json:"discovery_filenames"`
}

type governanceCatalogDoc struct {
	Roles             *[]string `yaml:"roles"`
	Transitions       *[]string `yaml:"transitions"`
	EvidenceSources   *[]string `yaml:"evidence_sources"`
	EscalationMetrics *[]string `yaml:"escalation_metrics"`
}

type subjectCatalogDoc struct {
	Action        *[]string `yaml:"action"`
	Configuration *[]string `yaml:"configuration"`
	Capability    *[]string `yaml:"capability"`
	Resource      *[]string `yaml:"resource"`
	Identity      *[]string `yaml:"identity"`
	Evidence      *[]string `yaml:"evidence"`
}

type adapterDoc struct {
	ID                 *string   `yaml:"id"`
	Version            *string   `yaml:"version"`
	Managed            *[]string `yaml:"managed"`
	DiscoveryFilenames *[]string `yaml:"discovery_filenames"`
}

type constitutionDoc struct {
	kernelDoc       `yaml:",inline"`
	SelectedProfile *string               `yaml:"selected_profile"`
	Environments    *[]string             `yaml:"environments"`
	Catalog         *governanceCatalogDoc `yaml:"catalog"`
	Subjects        *subjectCatalogDoc    `yaml:"subjects"`
	Adapters        *[]adapterDoc         `yaml:"adapters"`
}

// DecodeConstitution strictly decodes data as the store's
// verdi.policy-constitution/v1 manifest, validates it, normalizes its
// semantic sets, and seals the result.
func DecodeConstitution(data []byte) (*Constitution, error) {
	fm, body, err := artifact.SplitFrontmatter(data)
	if err != nil {
		return nil, fmt.Errorf("policyartifact: %w", err)
	}
	var doc constitutionDoc
	if err := artifact.DecodeStrict(fm, &doc); err != nil {
		return nil, err
	}
	k, err := doc.kernelDoc.toKernel(SchemaConstitution, KindConstitution)
	if err != nil {
		return nil, err
	}
	if nameOf(k.ID) != ConstitutionName {
		return nil, fmt.Errorf("policyartifact: constitution id must be %s/%s, got %q", KindConstitution, ConstitutionName, k.ID)
	}
	missing := func(field string) error {
		return fmt.Errorf("policyartifact: constitution field %s is missing: every constitution field is mandatory (an explicitly empty set is [])", field)
	}
	if doc.SelectedProfile == nil {
		return nil, missing("selected_profile")
	}
	if doc.Environments == nil {
		return nil, missing("environments")
	}
	if doc.Catalog == nil {
		return nil, missing("catalog")
	}
	if doc.Subjects == nil {
		return nil, missing("subjects")
	}
	if doc.Adapters == nil {
		return nil, missing("adapters")
	}

	// The selected profile id follows the kernel's own id grammar; the
	// kernel decides everything else about profiles.
	if err := governanceprincipal.ValidateID(*doc.SelectedProfile); err != nil {
		return nil, fmt.Errorf("policyartifact: constitution selected_profile: %w", err)
	}

	if err := uniqueSet("constitution.environments", emptyIfNil(*doc.Environments), func(e string) error {
		if !kebabRe.MatchString(e) {
			return fmt.Errorf("environment %q must be kebab-case", e)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	catalog, err := doc.Catalog.toCatalog()
	if err != nil {
		return nil, err
	}
	subjects, err := doc.Subjects.toCatalog()
	if err != nil {
		return nil, err
	}

	adapters := make([]Adapter, 0, len(*doc.Adapters))
	seenAdapter := make(map[string]bool, len(*doc.Adapters))
	for i, ad := range *doc.Adapters {
		a, err := ad.toAdapter(i)
		if err != nil {
			return nil, err
		}
		if seenAdapter[a.ID] {
			return nil, fmt.Errorf("policyartifact: constitution adapters: duplicate adapter id %q", a.ID)
		}
		seenAdapter[a.ID] = true
		adapters = append(adapters, a)
	}

	rationale, err := requireRationale(KindConstitution, body)
	if err != nil {
		return nil, err
	}

	c := &Constitution{
		Schema:          k.Schema,
		ID:              k.ID,
		Kind:            k.Kind,
		Title:           k.Title,
		Owners:          k.Owners,
		SelectedProfile: *doc.SelectedProfile,
		Environments:    emptyIfNil(*doc.Environments),
		Catalog:         catalog,
		Subjects:        subjects,
		Adapters:        adapters,
		Template:        k.Template,
		Rationale:       rationale,
	}
	normalizeConstitution(c)
	seal, err := canonjson.Digest(c)
	if err != nil {
		return nil, err
	}
	c.seal = seal
	return c, nil
}

func (d governanceCatalogDoc) toCatalog() (GovernanceCatalog, error) {
	missing := func(field string) error {
		return fmt.Errorf("policyartifact: constitution catalog.%s is missing (an explicitly empty set is [])", field)
	}
	switch {
	case d.Roles == nil:
		return GovernanceCatalog{}, missing("roles")
	case d.Transitions == nil:
		return GovernanceCatalog{}, missing("transitions")
	case d.EvidenceSources == nil:
		return GovernanceCatalog{}, missing("evidence_sources")
	case d.EscalationMetrics == nil:
		return GovernanceCatalog{}, missing("escalation_metrics")
	}
	c := GovernanceCatalog{
		Roles:             emptyIfNil(*d.Roles),
		Transitions:       emptyIfNil(*d.Transitions),
		EvidenceSources:   emptyIfNil(*d.EvidenceSources),
		EscalationMetrics: emptyIfNil(*d.EscalationMetrics),
	}
	fields := []struct {
		name string
		set  []string
	}{
		{"catalog.roles", c.Roles},
		{"catalog.transitions", c.Transitions},
		{"catalog.evidence_sources", c.EvidenceSources},
		{"catalog.escalation_metrics", c.EscalationMetrics},
	}
	for _, f := range fields {
		if err := uniqueSet("constitution."+f.name, f.set, governanceprincipal.ValidateID); err != nil {
			return GovernanceCatalog{}, err
		}
	}
	return c, nil
}

func (d subjectCatalogDoc) toCatalog() (SubjectCatalog, error) {
	missing := func(field string) error {
		return fmt.Errorf("policyartifact: constitution subjects.%s is missing (an explicitly empty set is [])", field)
	}
	switch {
	case d.Action == nil:
		return SubjectCatalog{}, missing("action")
	case d.Configuration == nil:
		return SubjectCatalog{}, missing("configuration")
	case d.Capability == nil:
		return SubjectCatalog{}, missing("capability")
	case d.Resource == nil:
		return SubjectCatalog{}, missing("resource")
	case d.Identity == nil:
		return SubjectCatalog{}, missing("identity")
	case d.Evidence == nil:
		return SubjectCatalog{}, missing("evidence")
	}
	s := SubjectCatalog{
		Action:        emptyIfNil(*d.Action),
		Configuration: emptyIfNil(*d.Configuration),
		Capability:    emptyIfNil(*d.Capability),
		Resource:      emptyIfNil(*d.Resource),
		Identity:      emptyIfNil(*d.Identity),
		Evidence:      emptyIfNil(*d.Evidence),
	}
	fields := []struct {
		name string
		set  []string
	}{
		{"subjects.action", s.Action},
		{"subjects.configuration", s.Configuration},
		{"subjects.capability", s.Capability},
		{"subjects.resource", s.Resource},
		{"subjects.identity", s.Identity},
		{"subjects.evidence", s.Evidence},
	}
	for _, f := range fields {
		if err := uniqueSet("constitution."+f.name, f.set, func(m string) error {
			if !kebabRe.MatchString(m) {
				return fmt.Errorf("subject %q must be kebab-case", m)
			}
			return nil
		}); err != nil {
			return SubjectCatalog{}, err
		}
	}
	return s, nil
}

func (d adapterDoc) toAdapter(i int) (Adapter, error) {
	missing := func(field string) error {
		return fmt.Errorf("policyartifact: constitution adapters[%d].%s is missing", i, field)
	}
	switch {
	case d.ID == nil:
		return Adapter{}, missing("id")
	case d.Version == nil:
		return Adapter{}, missing("version")
	case d.Managed == nil:
		return Adapter{}, missing("managed")
	case d.DiscoveryFilenames == nil:
		return Adapter{}, missing("discovery_filenames")
	}
	a := Adapter{
		ID:                 *d.ID,
		Version:            *d.Version,
		Managed:            emptyIfNil(*d.Managed),
		DiscoveryFilenames: emptyIfNil(*d.DiscoveryFilenames),
	}
	if !kebabRe.MatchString(a.ID) {
		return Adapter{}, fmt.Errorf("policyartifact: constitution adapters[%d]: id %q must be kebab-case", i, a.ID)
	}
	if a.Version == "" {
		return Adapter{}, fmt.Errorf("policyartifact: constitution adapters[%d] (%s): version must be non-empty", i, a.ID)
	}
	if len(a.Managed) == 0 {
		return Adapter{}, fmt.Errorf("policyartifact: constitution adapters[%d] (%s): managed must name at least one projection file", i, a.ID)
	}
	if err := uniqueSet(fmt.Sprintf("constitution.adapters[%d].managed", i), a.Managed, validateRelPath); err != nil {
		return Adapter{}, err
	}
	if len(a.DiscoveryFilenames) == 0 {
		return Adapter{}, fmt.Errorf("policyartifact: constitution adapters[%d] (%s): discovery_filenames must name at least one instruction filename", i, a.ID)
	}
	if err := uniqueSet(fmt.Sprintf("constitution.adapters[%d].discovery_filenames", i), a.DiscoveryFilenames, func(f string) error {
		if !artifact.IsBareFilename(f) {
			return fmt.Errorf("discovery filename %q must be a bare filename (the harness discovers it by name at any depth)", f)
		}
		return nil
	}); err != nil {
		return Adapter{}, err
	}
	return a, nil
}

// normalizeConstitution sorts every semantic set.
func normalizeConstitution(c *Constitution) {
	sort.Strings(c.Environments)
	sort.Strings(c.Catalog.Roles)
	sort.Strings(c.Catalog.Transitions)
	sort.Strings(c.Catalog.EvidenceSources)
	sort.Strings(c.Catalog.EscalationMetrics)
	sort.Strings(c.Subjects.Action)
	sort.Strings(c.Subjects.Configuration)
	sort.Strings(c.Subjects.Capability)
	sort.Strings(c.Subjects.Resource)
	sort.Strings(c.Subjects.Identity)
	sort.Strings(c.Subjects.Evidence)
	for i := range c.Adapters {
		sort.Strings(c.Adapters[i].Managed)
		sort.Strings(c.Adapters[i].DiscoveryFilenames)
	}
	sort.Slice(c.Adapters, func(i, j int) bool { return c.Adapters[i].ID < c.Adapters[j].ID })
}

// GovernanceCatalog converts the registered governance vocabulary into
// the kernel's own catalog type for DecodeProfile injection.
func (c *Constitution) GovernanceCatalog() governanceprincipal.Catalog {
	return governanceprincipal.Catalog{
		Roles:             c.Catalog.Roles,
		Transitions:       c.Catalog.Transitions,
		EvidenceSources:   c.Catalog.EvidenceSources,
		EscalationMetrics: c.Catalog.EscalationMetrics,
	}
}

// Digest returns the constitution's canonical content address after
// proving the value is unmodified DecodeConstitution output.
func (c *Constitution) Digest() (string, error) {
	if err := c.checkSeal(); err != nil {
		return "", err
	}
	return c.seal, nil
}

func (c *Constitution) checkSeal() error {
	return checkSealed("constitution", c.ID, c.seal, c)
}
