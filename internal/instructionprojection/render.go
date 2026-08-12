package instructionprojection

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/policyauthority"
)

// Selection is the caller-declared set of policy ids one Render call
// includes in its rendered view. Render does not itself decide which
// policies are applicable (SI-87(c) — scope selection is the compiler
// caller's job); it only formats whichever ids the caller names here,
// omitting every unselected policy from the rendered content without
// interpreting why it was left out.
type Selection struct {
	PolicyIDs []string
}

// RenderedFile is one managed path's rendered content and content
// digest. Content is a fresh copy on every Render call: it never aliases
// another RenderedFile's Content, a previous or later call's Content, or
// any memory owned by the passed Store.
type RenderedFile struct {
	Path    string
	Content []byte
	Digest  string
}

// Rendered is one pure Render call's complete output: the rendering
// adapter's own identity, the resolved authority facts every managed
// file and the manifest were rendered from, every managed file this
// adapter's selected view produces, and the canonical projection
// manifest bytes/digest describing them — the same shape Generate writes
// and Verify recomputes.
type Rendered struct {
	AdapterID       string
	AdapterVersion  string
	AuthorityDigest string
	ProfileID       string
	ProfileDigest   string
	Files           []RenderedFile
	Manifest        []byte
	ManifestDigest  string
}

// Render is the one pure instruction-projection rendering entry point
// design §7 and SI-87(c) require: it performs no filesystem I/O, write,
// clock read, or randomness — every byte it returns derives only from
// store, effective, adapter, and selection. Generate and Verify both
// route through this single call (generate.go, verify.go); the context
// compiler will later drive it directly with a phase-filtered Selection.
//
// store/effective must be the genuine loaded/resolved pair: Render
// freshly resolves store (policyauthority.Resolve) and requires digest
// equality against the passed effective policy, refusing rather than
// silently substituting either side of a pair that does not genuinely
// match. Because policyauthority.EffectivePolicy.Digest() itself refuses
// a value Resolve did not seal, this one check also rejects a nil store
// (Resolve(nil) fails) and an unsealed/hand-built effective policy
// (Digest() fails) without a separate nil check for either.
//
// adapter must byte-exactly match one row of store's own constitution: a
// caller cannot render against a constitution the store never declared.
//
// selection's ids must be a subset of effective's own policy entries
// (every entry, not only ones carrying an instruction), with no
// duplicate and no unknown id. Render sorts a COPY of selection.PolicyIDs
// — the caller's own slice is never read after copying and never
// mutated.
func Render(store *policyauthority.Store, effective *policyauthority.EffectivePolicy, adapter policyartifact.Adapter, selection Selection) (Rendered, error) {
	fresh, err := policyauthority.Resolve(store)
	if err != nil {
		return Rendered{}, fmt.Errorf("instructionprojection: Render: %w", err)
	}
	freshDigest, err := fresh.Digest()
	if err != nil {
		return Rendered{}, fmt.Errorf("instructionprojection: Render: %w", err)
	}
	passedDigest, err := effective.Digest()
	if err != nil {
		return Rendered{}, fmt.Errorf("instructionprojection: Render: effective policy: %w", err)
	}
	if freshDigest != passedDigest {
		return Rendered{}, fmt.Errorf("instructionprojection: Render: store/effective policy pair does not genuinely match (fresh resolution digest %s, passed effective digest %s)", freshDigest, passedDigest)
	}

	if !adapterDeclaredByStore(store.Constitution, adapter) {
		return Rendered{}, fmt.Errorf("instructionprojection: Render: adapter %s/%s does not byte-exactly match a row of the store's own constitution", adapter.ID, adapter.Version)
	}

	sel, err := validatedSortedSelection(effective, selection)
	if err != nil {
		return Rendered{}, fmt.Errorf("instructionprojection: Render: %w", err)
	}

	in, err := buildProjectionInput(store.Policies, effective)
	if err != nil {
		return Rendered{}, fmt.Errorf("instructionprojection: Render: %w", err)
	}
	in.Policies = filterSelected(in.Policies, sel)

	content := renderProjection(adapter, in)
	contentDig := contentDigest(content)

	files := make([]RenderedFile, 0, len(adapter.Managed))
	digestFiles := make([]FileDigest, 0, len(adapter.Managed))
	for _, rel := range adapter.Managed {
		files = append(files, RenderedFile{
			Path:    rel,
			Content: append([]byte(nil), content...),
			Digest:  contentDig,
		})
		digestFiles = append(digestFiles, FileDigest{Path: rel, Digest: contentDig})
	}

	m := buildManifest(adapter, in, digestFiles)
	mBytes, err := manifestBytes(m)
	if err != nil {
		return Rendered{}, fmt.Errorf("instructionprojection: Render: canonicalizing manifest: %w", err)
	}

	return Rendered{
		AdapterID:       adapter.ID,
		AdapterVersion:  adapter.Version,
		AuthorityDigest: in.AuthorityDigest,
		ProfileID:       in.ProfileID,
		ProfileDigest:   in.ProfileDigest,
		Files:           files,
		Manifest:        append([]byte(nil), mBytes...),
		ManifestDigest:  contentDigest(mBytes),
	}, nil
}

// adapterDeclaredByStore reports whether adapter byte-exactly matches one
// row of c's own declared adapters.
func adapterDeclaredByStore(c *policyartifact.Constitution, adapter policyartifact.Adapter) bool {
	if c == nil {
		return false
	}
	for _, a := range c.Adapters {
		if reflect.DeepEqual(a, adapter) {
			return true
		}
	}
	return false
}

// validatedSortedSelection proves every id in selection.PolicyIDs names a
// policy effective carries, with no duplicate, and returns a freshly
// sorted copy — selection.PolicyIDs itself is never mutated.
func validatedSortedSelection(effective *policyauthority.EffectivePolicy, selection Selection) ([]string, error) {
	known := make(map[string]bool, len(effective.Policies))
	for _, e := range effective.Policies {
		known[e.PolicyID] = true
	}

	sel := append([]string(nil), selection.PolicyIDs...)
	sort.Strings(sel)

	seen := make(map[string]bool, len(sel))
	for _, id := range sel {
		if !known[id] {
			return nil, fmt.Errorf("selected policy id %q is not a member of the effective policy", id)
		}
		if seen[id] {
			return nil, fmt.Errorf("selected policy id %q is selected more than once", id)
		}
		seen[id] = true
	}
	return sel, nil
}

// filterSelected returns the subsequence of policies whose ID is a member
// of selected, preserving policies' own order (already sorted by id from
// buildProjectionInput). It performs no scope interpretation of its own —
// selected is exactly the caller's (already-validated) chosen id set.
func filterSelected(policies []policyProjection, selected []string) []policyProjection {
	set := make(map[string]bool, len(selected))
	for _, id := range selected {
		set[id] = true
	}
	out := make([]policyProjection, 0, len(policies))
	for _, p := range policies {
		if set[p.ID] {
			out = append(out, p)
		}
	}
	return out
}

// projectionInput is the one resolved-authority view every adapter's
// content and manifest are rendered from. Generate and Verify each build
// exactly one projectionInput per Load+Resolve pair (Verify's is a fresh
// recomputation, never the stored manifest — see verify.go), so the two
// call sites can never drift in what they consider "the current
// authority".
type projectionInput struct {
	AuthorityDigest string
	ProfileID       string
	ProfileDigest   string
	// Policies holds only the policies that have at least one
	// instruction, sorted by policy id (the contract's "one section per
	// policy that has at least one instruction, sorted by policy id"
	// rule) — a policy with zero instructions never reaches this slice,
	// so renderProjection never needs to re-check for emptiness.
	Policies []policyProjection
}

// policyProjection is one policy's rendered identity and ordered
// instruction content.
type policyProjection struct {
	ID           string
	Title        string
	Digest       string
	Instructions []string
}

// buildProjectionInput resolves ep and policies (the loaded Store's own
// policy map, keyed by full policy id) into the one projectionInput
// every adapter's rendering reads. It fails only if ep was not produced
// by policyauthority.Resolve (an unsealed or hand-built value) or if the
// resolved effective policy names a policy id the Store never loaded —
// both cases policyauthority's own cross-validation already makes
// unreachable for a genuine Load+Resolve pair, so this is defense in
// depth, not a path this package's own callers can normally reach.
func buildProjectionInput(policies map[string]*policyartifact.Policy, ep *policyauthority.EffectivePolicy) (*projectionInput, error) {
	authorityDigest, err := ep.Digest()
	if err != nil {
		return nil, fmt.Errorf("instructionprojection: resolving authority digest: %w", err)
	}

	var entries []policyProjection
	for _, e := range ep.Policies {
		if len(e.Instructions) == 0 {
			continue
		}
		p, ok := policies[e.PolicyID]
		if !ok {
			return nil, fmt.Errorf("instructionprojection: effective policy entry %s has no matching loaded policy", e.PolicyID)
		}
		entries = append(entries, policyProjection{
			ID:           e.PolicyID,
			Title:        p.Title,
			Digest:       e.PolicyDigest,
			Instructions: append([]string{}, e.Instructions...),
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })

	return &projectionInput{
		AuthorityDigest: authorityDigest,
		ProfileID:       ep.ProfileID,
		ProfileDigest:   ep.ProfileDigest,
		Policies:        entries,
	}, nil
}

// renderProjection renders adapter's one deterministic content body
// (the v1 rule: one adapter projects one content body to every one of
// its managed paths, so this is called exactly once per adapter
// regardless of how many files it manages). Every line derives only
// from in, which itself derives only from resolved authority — no
// timestamp, username, absolute path, or random identifier ever appears
// (CO-3).
func renderProjection(adapter policyartifact.Adapter, in *projectionInput) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- verdi:generated-projection adapter=%s adapter-version=%s -->\n", adapter.ID, adapter.Version)
	fmt.Fprintf(&b, "<!-- verdi:authority-digest %s -->\n", in.AuthorityDigest)
	fmt.Fprintf(&b, "<!-- verdi:governance-profile id=%s digest=%s -->\n", in.ProfileID, in.ProfileDigest)
	b.WriteString("<!-- verdi: this is a generated projection; edits here never change authority, and any difference is reported as drift until this file is regenerated. -->\n")

	for _, p := range in.Policies {
		b.WriteString("\n")
		fmt.Fprintf(&b, "## %s (%s)\n", p.Title, p.ID)
		fmt.Fprintf(&b, "policy-digest: %s\n\n", p.Digest)
		for _, ins := range p.Instructions {
			fmt.Fprintf(&b, "- %s\n", ins)
		}
	}

	return []byte(b.String())
}
