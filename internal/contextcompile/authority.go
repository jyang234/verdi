package contextcompile

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/policyauthority"
	"github.com/jyang234/verdi/internal/repositoryfacts"
	"github.com/jyang234/verdi/internal/specstate"
)

// PolicyAuthority is the one resolved constitution, effective-policy
// projection and exact requested adapter used by a compile.
type PolicyAuthority struct {
	Store           *policyauthority.Store
	Effective       *policyauthority.EffectivePolicy
	Adapter         policyartifact.Adapter
	EffectiveDigest string
}

// ResolvePolicyAuthority resolves the adopted store and the exact requested
// constitution adapter. Missing adoption and adapter mismatch are semantic
// refusals; malformed authority and loader failures remain operational.
func ResolvePolicyAuthority(loader AuthorityLoader, root string, requested AdapterRef) (PolicyAuthority, error) {
	if loader == nil {
		return PolicyAuthority{}, fmt.Errorf("contextcompile: policy authority loader is nil")
	}
	if err := requested.validate("adapter"); err != nil {
		return PolicyAuthority{}, err
	}

	store, err := loader.Load(root)
	if err != nil {
		if errors.Is(err, policyauthority.ErrNotAdopted) {
			return PolicyAuthority{}, &NoConstitutionRefusal{}
		}
		return PolicyAuthority{}, fmt.Errorf("contextcompile: load policy authority: %w", err)
	}
	if store == nil {
		return PolicyAuthority{}, fmt.Errorf("contextcompile: policy authority loader returned a nil store")
	}
	effective, err := loader.Resolve(store)
	if err != nil {
		return PolicyAuthority{}, fmt.Errorf("contextcompile: resolve effective policy: %w", err)
	}
	if effective == nil {
		return PolicyAuthority{}, fmt.Errorf("contextcompile: policy authority loader returned a nil effective policy")
	}
	effectiveDigest, err := effective.Digest()
	if err != nil {
		return PolicyAuthority{}, fmt.Errorf("contextcompile: digest effective policy: %w", err)
	}
	if store.Constitution == nil {
		return PolicyAuthority{}, fmt.Errorf("contextcompile: resolved policy store has no constitution")
	}

	registered := make([]AdapterRef, 0, len(store.Constitution.Adapters))
	for _, adapter := range store.Constitution.Adapters {
		ref := AdapterRef{ID: adapter.ID, Version: adapter.Version}
		registered = append(registered, ref)
		if ref == requested {
			return PolicyAuthority{
				Store:           store,
				Effective:       effective,
				Adapter:         adapter,
				EffectiveDigest: effectiveDigest,
			}, nil
		}
	}
	sort.Slice(registered, func(i, j int) bool {
		if registered[i].ID == registered[j].ID {
			return registered[i].Version < registered[j].Version
		}
		return registered[i].ID < registered[j].ID
	})
	return PolicyAuthority{}, &AdapterMismatchRefusal{Requested: requested, Registered: registered}
}

// ResolvedSpec is an exact HEAD-tree specification plus its merge-signaled
// accepted-baseline identity and strict-decoded frontmatter.
type ResolvedSpec struct {
	Ref           string
	Path          string
	Blob          string
	Commit        string
	ContentDigest string
	Content       []byte
	Spec          *artifact.SpecFrontmatter
	State         specstate.State
}

// ResolveAcceptedSpec resolves ref only from exact HEAD tree bytes and accepts
// it only when the shared state projector proves an exact accepted baseline.
func ResolveAcceptedSpec(ctx context.Context, git GitReader, states StateResolver, root, head, ref string) (ResolvedSpec, error) {
	if git == nil || states == nil {
		return ResolvedSpec{}, fmt.Errorf("contextcompile: accepted-spec ports must be non-nil")
	}
	if err := validateGitHash("HEAD", head); err != nil {
		return ResolvedSpec{}, err
	}
	if err := validateSpecWholeRef("spec", ref); err != nil {
		return ResolvedSpec{}, err
	}
	parsed, err := artifact.ParseRef(ref)
	if err != nil {
		return ResolvedSpec{}, fmt.Errorf("contextcompile: parse spec ref: %w", err)
	}
	activePath := ".verdi/specs/active/" + parsed.Name + "/spec.md"
	archivePath := ".verdi/specs/archive/" + parsed.Name + "/spec.md"

	entries, err := git.LsTreeEntries(ctx, root, head)
	if err != nil {
		return ResolvedSpec{}, fmt.Errorf("contextcompile: list HEAD tree: %w", err)
	}
	var matches []gitx.TreeEntry
	for _, entry := range entries {
		if entry.Path == activePath || entry.Path == archivePath {
			matches = append(matches, entry)
		}
	}
	if len(matches) == 0 {
		return ResolvedSpec{}, &AcceptedSpecRefusal{
			Ref: ref, State: specstate.Unproven, Relation: specstate.RelationUnproven,
			Disclosures: []string{"spec is absent from the exact HEAD active/archive tree"},
		}
	}
	if len(matches) != 1 {
		return ResolvedSpec{}, fmt.Errorf("contextcompile: %s appears in both active and archive zones at HEAD", ref)
	}
	entry := matches[0]
	if entry.Type != "blob" || (entry.Mode != "100644" && entry.Mode != "100755") || entry.Object == "" {
		return ResolvedSpec{}, fmt.Errorf("contextcompile: accepted spec %s is not a regular HEAD-tree blob: %+v", ref, entry)
	}
	if err := validateGitHash("accepted spec blob", entry.Object); err != nil {
		return ResolvedSpec{}, err
	}
	content, err := git.Show(ctx, root, head, entry.Path)
	if err != nil {
		return ResolvedSpec{}, fmt.Errorf("contextcompile: read HEAD spec %s: %w", entry.Path, err)
	}
	fmBytes, _, err := artifact.SplitFrontmatter(content)
	if err != nil {
		return ResolvedSpec{}, fmt.Errorf("contextcompile: decode accepted spec %s: %w", ref, err)
	}
	spec, err := artifact.DecodeSpec(fmBytes)
	if err != nil {
		return ResolvedSpec{}, fmt.Errorf("contextcompile: decode accepted spec %s: %w", ref, err)
	}
	if spec.ID != ref {
		return ResolvedSpec{}, fmt.Errorf("contextcompile: accepted spec path for %s declares id %q", ref, spec.ID)
	}

	result, err := states.Resolve(ctx, root, specstate.Candidate{Path: entry.Path, Content: append([]byte(nil), content...)})
	if err != nil {
		return ResolvedSpec{}, fmt.Errorf("contextcompile: resolve accepted state for %s: %w", ref, err)
	}
	accepted := result.State == specstate.AcceptedPendingBuild || result.State == specstate.Closed
	if !accepted || result.Relation != specstate.RelationExact {
		return ResolvedSpec{}, &AcceptedSpecRefusal{
			Ref:         ref,
			State:       result.State,
			Relation:    result.Relation,
			Disclosures: append([]string(nil), result.Disclosures...),
		}
	}
	if result.Baseline == nil {
		return ResolvedSpec{}, fmt.Errorf("contextcompile: state projector accepted %s without a baseline", ref)
	}
	if (result.State == specstate.AcceptedPendingBuild && entry.Path != activePath) ||
		(result.State == specstate.Closed && entry.Path != archivePath) {
		return ResolvedSpec{}, fmt.Errorf("contextcompile: state projector returned %s for incompatible path %s", result.State, entry.Path)
	}
	baseline := result.Baseline
	if baseline.Path != entry.Path || baseline.Blob != entry.Object {
		return ResolvedSpec{}, fmt.Errorf("contextcompile: state projector baseline for %s differs from exact HEAD tree", ref)
	}
	if err := validateGitHash("accepted spec landing commit", baseline.LandingCommit); err != nil {
		return ResolvedSpec{}, err
	}

	return ResolvedSpec{
		Ref:           ref,
		Path:          entry.Path,
		Blob:          entry.Object,
		Commit:        baseline.LandingCommit,
		ContentDigest: rawContentDigest(content),
		Content:       append([]byte(nil), content...),
		Spec:          spec,
		State:         result.State,
	}, nil
}

// ResolveExpectedRepository compares the optional caller expectation to
// computed facts. Expectations are assertions only; they never fill unknown
// computed values.
func ResolveExpectedRepository(expected *Expected, facts repositoryfacts.Facts) error {
	if expected == nil {
		return nil
	}
	if err := facts.Validate(); err != nil {
		return fmt.Errorf("contextcompile: computed repository facts: %w", err)
	}
	if err := validateNonEmpty("expected.branch", expected.Branch); err != nil {
		return err
	}
	if err := validateGitHash("expected.head", expected.Head); err != nil {
		return err
	}
	if !facts.Branch.Known || !facts.Head.Known || facts.Branch.Value != expected.Branch || facts.Head.Value != expected.Head {
		return &ExpectedRepositoryMismatchRefusal{
			Expected:       *expected,
			ComputedBranch: facts.Branch.Value,
			ComputedHead:   facts.Head.Value,
			BranchKnown:    facts.Branch.Known,
			HeadKnown:      facts.Head.Known,
		}
	}
	return nil
}

func wholeSpecRef(fragmentRef string) (artifact.Ref, error) {
	parsed, err := artifact.ParseRef(fragmentRef)
	if err != nil {
		return artifact.Ref{}, err
	}
	if parsed.Kind != artifact.KindSpec || parsed.Pinned() || !parsed.Fragment() {
		return artifact.Ref{}, fmt.Errorf("fragment ref %q must be an unpinned spec object ref", fragmentRef)
	}
	if strings.TrimSpace(parsed.Object) == "" {
		return artifact.Ref{}, fmt.Errorf("fragment ref %q has no object", fragmentRef)
	}
	return parsed, nil
}
