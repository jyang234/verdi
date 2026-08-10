package instructionprojection

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/policyauthority"
)

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
