package policyadopt

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing/fstest"

	"github.com/jyang234/verdi/internal/constitutionimpact"
	"github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/humanartifact"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/policyauthority"
)

// Input is Compose's caller-supplied selection: which starter profile to
// render (solo or team — R-W4-1's starter shape) and the owner every
// rendered artifact records. Subject is the checkout's own local git
// identity for the solo profile (the one subject it binds to every
// role); the team template maps no one, so Subject must be empty for it
// — Compose refuses either direction (Task 1's RenderProfile symmetry:
// a non-empty Subject the template binds to no one is refused, not
// silently discarded).
type Input struct {
	Profile governanceprincipal.Class // ClassSolo | ClassTeam
	Owner   string                    // kebab-case owner handle
	Subject string                    // solo: the checkout's local git identity; team: ""
}

// File is one composed starter artifact: its store-relative path (forward
// slashes), rendered content, and the resolved scaffold record it was
// created from.
type File struct {
	RelPath  string // store-relative, forward slashes
	Content  []byte
	Template policyartifact.TemplateRecord
}

// Plan is Compose's proven, not-yet-written output: the four files in
// constitution/profile/policy/inventory order, the selected profile's
// kernel id, the design_assistance mode the starter policy grants, and
// the effective-policy digest the composed tree resolves to (proven by
// prove, R-W4-2) — the same digest Write's on-disk result must resolve
// to again once every byte lands.
type Plan struct {
	Files                []File // constitution, profile, policy, inventory — in that order
	ProfileID            string
	DesignAssistanceMode string
	EffectiveDigest      string // policyauthority.EffectivePolicy.Digest() over the composed tree
}

// ErrAlreadyAdopted is Compose's refusal when root already carries a
// policy store or a consumers inventory: adopting again would silently
// recompose over an existing constitution.
var ErrAlreadyAdopted = errors.New("policyadopt: this checkout already carries .verdi/policy or .verdi/constitution/consumers.json")

const (
	ProfileSoloID = "starter-solo"
	ProfileTeamID = "starter-team"
	PolicyName    = "starter"
	// modeSolo/modeTeam are the accepted ASD specification's own
	// design_assistance mode enum values (policyartifact.
	// DesignAssistancePayload.Validate: "off, proposal-only,
	// draft-write") — R-W4-1's one real rule the starter policy renders.
	// vocab:identity — design_assistance mode enum value (ASD AC-3), not the lifecycle state word
	modeSolo = "draft-write"
	// vocab:identity — design_assistance mode enum value (ASD AC-3), not the lifecycle state word
	modeTeam = "proposal-only"

	constitutionRel = ".verdi/policy/constitution.md"
	policyRel       = ".verdi/policy/policies/" + PolicyName + ".md"
	// inventoryRel is constitutionimpact's own canonical inventory path —
	// named here rather than referenced inline throughout so refuseAdopted
	// and prove share one literal.
	inventoryRel = constitutionimpact.InventoryPath
)

// profileRel returns the store-relative path a stored governance profile
// named id lives at.
func profileRel(id string) string { return ".verdi/policy/profiles/" + id + ".md" }

// Compose resolves the four starter scaffolds under root (a store
// override at .verdi/templates/<name> winning over the embedded
// canonical default, humanartifact.ResolveScaffold), renders each
// against in through Task 1's shared humanartifact renderers, and proves
// the composed tree as a whole in memory (prove, R-W4-2) before
// returning — Compose itself never writes a byte; Write does that.
//
// It refuses with ErrAlreadyAdopted when root already carries
// .verdi/policy or .verdi/constitution/consumers.json (refuseAdopted),
// and with a plain error naming the offending field for every other
// input or rendering failure — an unsupported profile class, a subject
// supplied for (or missing from) the wrong profile, or a store override
// that fails Task 1's render/strict-decode kernel round trip.
func Compose(root string, in Input) (*Plan, error) {
	if err := refuseAdopted(root); err != nil {
		return nil, err
	}

	var profileID, profileTemplateName, mode string
	switch in.Profile {
	case governanceprincipal.ClassSolo:
		if in.Subject == "" {
			return nil, fmt.Errorf("policyadopt: the solo profile binds the checkout's own git identity; none was supplied")
		}
		profileID, profileTemplateName, mode = ProfileSoloID, humanartifact.ProfileSoloTemplate, modeSolo
	case governanceprincipal.ClassTeam:
		if in.Subject != "" {
			return nil, fmt.Errorf("policyadopt: the team profile maps no subjects; refusing the supplied subject %q", in.Subject)
		}
		profileID, profileTemplateName, mode = ProfileTeamID, humanartifact.ProfileTeamTemplate, modeTeam
	default:
		return nil, fmt.Errorf("policyadopt: profile class %q is not a starter profile (solo or team)", in.Profile)
	}

	// Resolve all four scaffolds first so a missing/unsafe override is
	// reported before any render is attempted.
	cs, err := humanartifact.ResolveScaffold(root, humanartifact.ConstitutionTemplate)
	if err != nil {
		return nil, err
	}
	ps, err := humanartifact.ResolveScaffold(root, profileTemplateName)
	if err != nil {
		return nil, err
	}
	pol, err := humanartifact.ResolveScaffold(root, humanartifact.StarterPolicyTemplate)
	if err != nil {
		return nil, err
	}
	inv, err := humanartifact.ResolveScaffold(root, humanartifact.InventoryTemplate)
	if err != nil {
		return nil, err
	}

	constitution, err := humanartifact.RenderConstitution(cs, humanartifact.ConstitutionScaffoldData{
		Title:            "Starter constitution",
		Owners:           []string{in.Owner},
		ProfileID:        profileID,
		TemplateIdentity: cs.Identity,
		TemplateDigest:   cs.Digest,
	})
	if err != nil {
		return nil, err
	}
	// Decode the just-rendered constitution to get at ITS catalog: the
	// vocabulary the profile renderer below validates the profile
	// against. Task 1's own TestRenderConstitution_RoundTrip pins that
	// this catalog is exactly the vocabulary the starter profiles need.
	decodedC, err := policyartifact.DecodeConstitution([]byte(constitution))
	if err != nil {
		return nil, err
	}
	catalog, err := decodedC.GovernanceCatalog()
	if err != nil {
		return nil, err
	}

	profile, err := humanartifact.RenderProfile(ps, humanartifact.ProfileScaffoldData{
		ProfileID:        profileID,
		Class:            in.Profile,
		Subject:          in.Subject,
		TemplateIdentity: ps.Identity,
		TemplateDigest:   ps.Digest,
	}, catalog)
	if err != nil {
		return nil, err
	}

	policy, err := humanartifact.RenderStarterPolicy(pol, humanartifact.StarterPolicyScaffoldData{
		Name:                 PolicyName,
		Title:                "Starter policy",
		Owners:               []string{in.Owner},
		DesignAssistanceMode: mode,
		TemplateIdentity:     pol.Identity,
		TemplateDigest:       pol.Digest,
	})
	if err != nil {
		return nil, err
	}

	inventory, err := humanartifact.RenderConsumersInventory(inv, humanartifact.InventoryScaffoldData{
		TemplateIdentity: inv.Identity,
		TemplateDigest:   inv.Digest,
	})
	if err != nil {
		return nil, err
	}

	files := []File{
		{RelPath: constitutionRel, Content: []byte(constitution), Template: policyartifact.TemplateRecord{Identity: cs.Identity, Digest: cs.Digest}},
		{RelPath: profileRel(profileID), Content: []byte(profile), Template: policyartifact.TemplateRecord{Identity: ps.Identity, Digest: ps.Digest}},
		{RelPath: policyRel, Content: []byte(policy), Template: policyartifact.TemplateRecord{Identity: pol.Identity, Digest: pol.Digest}},
		{RelPath: inventoryRel, Content: inventory, Template: policyartifact.TemplateRecord{Identity: inv.Identity, Digest: inv.Digest}},
	}

	digest, err := prove(files, mode)
	if err != nil {
		return nil, err
	}
	return &Plan{Files: files, ProfileID: profileID, DesignAssistanceMode: mode, EffectiveDigest: digest}, nil
}

// prove loads the composed tree exactly as the store would
// (policyauthority.LoadFromSource over an in-memory FS), resolves it, and
// requires exactly one valid design_assistance payload with the planned
// mode — the same lookup draftmutation.ResolvePolicyGrant performs at
// authorization time (draftmutation/policy.go:289-310), re-implemented
// here rather than called: ResolvePolicyGrant needs identity/checkout
// plumbing this in-memory, pre-write proof does not have. cmd/verdi's own
// showcase/build-binary tests run the REAL ResolvePolicyGrant against the
// written store afterward and pin its answer equal to this one.
func prove(files []File, mode string) (string, error) {
	tree := fstest.MapFS{}
	for _, f := range files {
		tree[f.RelPath] = &fstest.MapFile{Data: f.Content, Mode: 0o644}
	}

	store, err := policyauthority.LoadFromSource(tree)
	if err != nil {
		return "", fmt.Errorf("policyadopt: the composed starter tree does not load: %w", err)
	}
	eff, err := policyauthority.Resolve(store)
	if err != nil {
		return "", fmt.Errorf("policyadopt: the composed starter tree does not resolve: %w", err)
	}

	var found *policyartifact.DesignAssistancePayload
	for _, entry := range eff.Policies {
		raw, ok := entry.Payloads[policyartifact.DesignAssistancePayloadKind]
		if !ok {
			continue
		}
		if found != nil {
			return "", fmt.Errorf("policyadopt: the composed tree carries two design_assistance payloads")
		}
		typed, ok := raw.(*policyartifact.DesignAssistancePayload)
		if !ok {
			return "", fmt.Errorf("policyadopt: design_assistance payload is not the registered typed payload")
		}
		found = typed
	}
	if found == nil || found.Mode != mode {
		return "", fmt.Errorf("policyadopt: the composed tree grants design_assistance mode %v, want %q", found, mode)
	}

	// Belt and suspenders over the inventory too, even though
	// RenderConsumersInventory already strict-decoded it: prove reads
	// back exactly the bytes about to be written, from the same
	// in-memory tree LoadFromSource just proved, before any of it
	// reaches disk (R-W4-2).
	if _, err := constitutionimpact.DecodeInventory(tree[inventoryRel].Data); err != nil {
		return "", err
	}

	return eff.Digest()
}

// refuseAdopted reports ErrAlreadyAdopted, wrapped with the offending
// path, when root already carries .verdi/policy or the consumers
// inventory at inventoryRel — Compose must never silently recompose over
// an existing (or partial) adoption. Any os.Stat failure other than
// not-exist is operational, propagated as such rather than folded into
// ErrAlreadyAdopted.
func refuseAdopted(root string) error {
	for _, rel := range []string{".verdi/policy", inventoryRel} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("%w: %s", ErrAlreadyAdopted, rel)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("policyadopt: checking %s: %w", rel, err)
		}
	}
	return nil
}
