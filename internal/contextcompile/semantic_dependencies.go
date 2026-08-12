package contextcompile

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"unicode/utf8"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/store"
)

// obligationPair is one accepted target acceptance-criterion/evidence-kind
// pair enumerated from the already-resolved target's decoded spec (authority
// design §8.2: "ac and kind are the exact declared pair").
type obligationPair struct {
	AC   string
	Kind artifact.EvidenceKind
}

// ResolveBoundObligations resolves every obligation bound to target's exact
// accepted AC/evidence-kind pairs from the exact HEAD tree only (SI-91's
// read-only, exact-byte resolution discipline). AC/evidence-kind pairs come
// only from the already-resolved target.Spec; ResolveBoundObligations never
// re-resolves the target and never accepts a caller-supplied pair list.
//
// For each pair the canonical store path is computed via
// store.ObligationPath("", <target name>, <ac>, <kind>) and looked up in one
// HEAD-tree listing (git.LsTreeEntries, called exactly once and reused
// across every pair). An absent canonical path is honest absence: the pair
// is skipped, never an error and never a placeholder row. A present
// candidate must be a regular blob (mode 100644/100755, type blob); must
// strict-decode via artifact.DecodeObligation; and its decoded id, for_kind,
// and sole verifies edge must agree exactly with the canonical grammar, the
// pair's declared kind, and target.Ref (whole-target, no fragment). Any
// mismatch is an operational error naming the offense — never a refusal,
// since a bound-obligation shape defect is not a state the caller declared
// and had refused.
//
// ResolveBoundObligations never reads working-tree content and never calls
// git.WorktreeChangedPaths.
func ResolveBoundObligations(ctx context.Context, git GitReader, root, head string, target ResolvedSpec) ([]BoundObligation, error) {
	if git == nil {
		return nil, fmt.Errorf("contextcompile: resolve bound obligations: git port is nil")
	}
	if err := validateGitHash("HEAD", head); err != nil {
		return nil, err
	}
	if err := validateSpecWholeRef("target.ref", target.Ref); err != nil {
		return nil, fmt.Errorf("contextcompile: resolve bound obligations: %w", err)
	}
	if target.Spec == nil {
		return nil, fmt.Errorf("contextcompile: resolve bound obligations: target %s has no decoded specification", target.Ref)
	}
	parsed, err := artifact.ParseRef(target.Ref)
	if err != nil {
		return nil, fmt.Errorf("contextcompile: resolve bound obligations: parse target ref: %w", err)
	}

	pairs := make([]obligationPair, 0, len(target.Spec.AcceptanceCriteria))
	for _, ac := range target.Spec.AcceptanceCriteria {
		for _, kind := range ac.Evidence {
			pairs = append(pairs, obligationPair{AC: ac.ID, Kind: kind})
		}
	}

	result := []BoundObligation{}
	if len(pairs) == 0 {
		return result, nil
	}

	entries, err := git.LsTreeEntries(ctx, root, head)
	if err != nil {
		return nil, fmt.Errorf("contextcompile: resolve bound obligations: list HEAD tree: %w", err)
	}
	byPath := make(map[string]gitx.TreeEntry, len(entries))
	for _, entry := range entries {
		byPath[entry.Path] = entry
	}

	for _, pair := range pairs {
		path := store.ObligationPath("", parsed.Name, pair.AC, string(pair.Kind))
		entry, ok := byPath[path]
		if !ok {
			// Honest absence: no obligation exists yet for this pair on the
			// exact HEAD tree. Skip, no error, no placeholder row.
			continue
		}
		if entry.Type != "blob" || (entry.Mode != "100644" && entry.Mode != "100755") || entry.Object == "" {
			return nil, fmt.Errorf("contextcompile: resolve bound obligations: %s is not a regular HEAD-tree blob: %+v", path, entry)
		}

		content, err := git.Show(ctx, root, head, path)
		if err != nil {
			return nil, fmt.Errorf("contextcompile: resolve bound obligations: read HEAD obligation %s: %w", path, err)
		}
		fmBytes, _, err := artifact.SplitFrontmatter(content)
		if err != nil {
			return nil, fmt.Errorf("contextcompile: resolve bound obligations: decode obligation %s: %w", path, err)
		}
		fm, err := artifact.DecodeObligation(fmBytes)
		if err != nil {
			return nil, fmt.Errorf("contextcompile: resolve bound obligations: decode obligation %s: %w", path, err)
		}

		if fm.ForKind != pair.Kind {
			return nil, fmt.Errorf("contextcompile: resolve bound obligations: obligation %s declares for_kind %q, want the declared pair's kind %q", path, fm.ForKind, pair.Kind)
		}
		wantID := fmt.Sprintf("obligation/%s--%s--%s", parsed.Name, pair.AC, pair.Kind)
		if fm.ID != wantID {
			return nil, fmt.Errorf("contextcompile: resolve bound obligations: obligation at %s declares id %q, want %q (spec/obligation-artifact DC-2 path/id agreement)", path, fm.ID, wantID)
		}
		if len(fm.Links) != 1 || fm.Links[0].Type != artifact.LinkVerifies || fm.Links[0].Ref != target.Ref {
			return nil, fmt.Errorf("contextcompile: resolve bound obligations: obligation %s must carry exactly one whole-target verifies edge to %s", path, target.Ref)
		}

		result = append(result, BoundObligation{
			Ref:           fm.ID,
			Path:          path,
			TargetRef:     target.Ref,
			AC:            pair.AC,
			Kind:          pair.Kind,
			ContentDigest: rawContentDigest(content),
			Content:       append([]byte(nil), content...),
		})
	}

	sort.Slice(result, func(i, j int) bool { return result[i].Ref < result[j].Ref })
	return result, nil
}

// ============================================================================
// SI-92: exact pinned declared-context artifact and fragment resolution
// ============================================================================

// DeclaredContextItem is one exact pinned declared-context artifact or
// fragment resolution result (docs/superpowers/invention-ledger.md SI-92).
// Ref is the caller's complete pinned ref exactly as declared (optionally
// carrying a fragment) — the item's own identity. LogicalRef is the
// unpinned, unfragmented "<kind>/<name>" form SI-92 fixes as the
// declared-context-ref payload's logical id; two distinct Refs that share
// one LogicalRef are rejected before ResolveDeclaredContext returns (SI-91
// dedupes only identical exact Refs). Content is the exact whole
// valid-UTF-8 source-file bytes at Path, at the ref's pinned commit — never
// a fragment projection, even when Ref carries a #object-id.
type DeclaredContextItem struct {
	Ref           string
	LogicalRef    string
	Path          string
	Kind          artifact.Kind
	Name          string
	ContentDigest string
	Content       []byte
}

// DeclaredContextResult is ResolveDeclaredContext's complete output.
//
// Lift maps each item's resolved store path to its LogicalRef — the
// head-tree path -> declared-context-ref shape BuildUniverse's
// UniverseInput.LiftedContextPaths consumes directly (universe.go already
// implements authority design §5's source precedence over this shape: an
// overlapping store-authority lift for the same path suppresses the
// declared-context lift, and a surviving declared-context lift suppresses
// the head-tree candidate for that path even when pinned and HEAD bytes
// differ — ResolveDeclaredContext does not reimplement that precedence,
// it only produces the map universe.go's existing resolveLifts already
// expects).
//
// Lift's values are deliberately the UNPINNED LogicalRef, not each item's
// complete pinned Ref: universe.go's liftedCandidates turns a lift map's
// value directly into a Candidate.Ref, and payload.go's BuildDataItem
// requires (for SourceDeclaredContext) candidate.ID == "ref:"+candidate.Ref
// — the only way that invariant can equal SI-92's fixed logical id
// "ref:<kind>/<name>" is for candidate.Ref itself to already be the
// unpinned "<kind>/<name>" form. Neither universe.go nor payload.go is
// reachable from this package's authorized surface, so this shape is a
// deliberate design point, not a rediscovery of an existing contract.
type DeclaredContextResult struct {
	Items []DeclaredContextItem
	Lift  map[string]string
}

// ResolveDeclaredContext resolves a target's exact pinned declared-context
// refs into whole-artifact payloads (SI-92), reading only:
//
//   - the exact pinned commit each ref names, for the artifact/fragment
//     itself (never worktree, never a different commit's bytes); and
//   - the exact target HEAD tree, only to re-verify (TOCTOU) each governing
//     parent feature's own context: list for a story/spike target (SI-91).
//
// A class:feature target resolves its own Spec.Context list and must not
// receive parents (feature-only field; SI-91 (c)). A class:story target
// (spike included) has no context: field of its own — SpecFrontmatter.
// Validate rejects one — so it resolves the sorted, de-duplicated union of
// every governing parent feature's exact pinned context: refs, each parent
// re-read and re-verified at exact HEAD rather than trusted from the
// caller's already-resolved FeatureFragment (guards a stale/tampered
// caller-supplied fragment from silently widening declared context).
//
// Every kind in the closed artifact registry is accepted. Non-spec kinds
// resolve their single fixed path via store.NonSpecArtifactPath; spec
// resolves exactly one matching active- or archive-zone path from the
// PINNED tree (an archived or later-superseded pin still resolves — SI-92:
// "explicit declared-context pins remain included"). The matching bytes
// are strict-decoded and the decoded unpinned whole id must equal the
// ref's own kind/name. A fragment on a spec ref must name an id in
// artifact.DeclaredObjectIDs (so #problem/#outcome never resolve); a
// fragment on any non-spec kind is invalid authority. The payload is
// always the WHOLE file's exact bytes, regardless of any fragment.
//
// Every failure mode here — an unresolvable path or fragment, an identity
// mismatch, a duplicate candidate, malformed frontmatter, invalid UTF-8/
// NUL content, or a malformed pinned ref — is an operational error
// (IsRefusal reports false), never a refusal: SI-92's declared-context
// material carries universal applicability, so no state-refusal or
// phase/scope exclusion can apply to it.
func ResolveDeclaredContext(ctx context.Context, git GitReader, root, head string, target ResolvedSpec, parents []FeatureFragment) (DeclaredContextResult, error) {
	if git == nil {
		return DeclaredContextResult{}, fmt.Errorf("contextcompile: resolve declared context: git port is nil")
	}
	if err := validateGitHash("HEAD", head); err != nil {
		return DeclaredContextResult{}, fmt.Errorf("contextcompile: resolve declared context: %w", err)
	}
	if err := validateSpecWholeRef("target.ref", target.Ref); err != nil {
		return DeclaredContextResult{}, fmt.Errorf("contextcompile: resolve declared context: %w", err)
	}
	if target.Spec == nil {
		return DeclaredContextResult{}, fmt.Errorf("contextcompile: resolve declared context: target %s has no decoded specification", target.Ref)
	}

	rawRefs, err := declaredContextRawRefs(ctx, git, root, head, target, parents)
	if err != nil {
		return DeclaredContextResult{}, err
	}

	treeCache := make(map[string][]gitx.TreeEntry)
	items := make([]DeclaredContextItem, 0, len(rawRefs))
	logicalOwner := make(map[string]string, len(rawRefs)) // LogicalRef -> owning raw ref
	lift := make(map[string]string, len(rawRefs))
	for _, raw := range rawRefs {
		item, err := resolveDeclaredContextItem(ctx, git, root, raw, treeCache)
		if err != nil {
			return DeclaredContextResult{}, err
		}
		if owner, seen := logicalOwner[item.LogicalRef]; seen {
			// vocab:identity — SI-92 duplicate-candidate diagnostic naming the
			// fixed declared-context-ref logical id
			return DeclaredContextResult{}, fmt.Errorf("contextcompile: resolve declared context: %q and %q are distinct exact refs that both resolve to logical id %q (duplicate candidate)", owner, raw, item.LogicalRef)
		}
		logicalOwner[item.LogicalRef] = raw
		items = append(items, item)
		lift[item.Path] = item.LogicalRef
	}

	return DeclaredContextResult{Items: items, Lift: lift}, nil
}

// declaredContextRawRefs computes the exact set of raw declared-context ref
// strings ResolveDeclaredContext must resolve, sorted and de-duplicated by
// exact string (SI-91's "identical exact refs dedupe").
func declaredContextRawRefs(ctx context.Context, git GitReader, root, head string, target ResolvedSpec, parents []FeatureFragment) ([]string, error) {
	var all []string
	switch target.Spec.Class {
	case artifact.ClassFeature:
		if len(parents) != 0 {
			// vocab:identity — "feature" names the fixed target-class identity this class-dispatch branch handles (SI-91/SI-92)
			return nil, fmt.Errorf("contextcompile: resolve declared context: a feature target resolves its own context: list and must not receive parent fragments")
		}
		all = target.Spec.Context
	case artifact.ClassStory:
		if len(parents) == 0 {
			// vocab:identity — "feature" names the fixed governing-parent artifact class this class-dispatch branch requires (SI-91/SI-92)
			return nil, fmt.Errorf("contextcompile: resolve declared context: a story/spike target requires at least one governing parent feature")
		}
		for _, p := range parents {
			spec, err := reverifyGoverningFeature(ctx, git, root, head, p.Feature)
			if err != nil {
				return nil, err
			}
			all = append(all, spec.Context...)
		}
	default:
		// vocab:identity — "feature" names the fixed target-class identity this class-dispatch default enumerates (SI-91/SI-92)
		return nil, fmt.Errorf("contextcompile: resolve declared context: target class %q does not declare context (only feature and story/spike do)", target.Spec.Class)
	}

	seen := make(map[string]bool, len(all))
	unique := make([]string, 0, len(all))
	for _, r := range all {
		if seen[r] {
			continue
		}
		seen[r] = true
		unique = append(unique, r)
	}
	sort.Strings(unique)
	return unique, nil
}

// reverifyGoverningFeature re-reads ff.Path at exact HEAD and re-verifies
// (TOCTOU) that its strict-decoded id, class, and content digest still
// agree with the already-resolved FeatureFragment the caller supplied,
// before this resolver ever trusts that fragment's Context list. It never
// reads the ref the caller supplied blindly — only Path, which addresses a
// fixed on-disk location regardless of what Ref claims.
func reverifyGoverningFeature(ctx context.Context, git GitReader, root, head string, ff FragmentFeature) (*artifact.SpecFrontmatter, error) {
	if ff.Path == "" {
		// vocab:identity — "feature" names the fixed governing-parent artifact class this path-presence check guards (SI-92)
		return nil, fmt.Errorf("contextcompile: resolve declared context: governing parent feature has no path")
	}
	content, err := git.Show(ctx, root, head, ff.Path)
	if err != nil {
		// vocab:identity — "feature" names the fixed governing-parent artifact class this HEAD re-read targets (SI-92)
		return nil, fmt.Errorf("contextcompile: resolve declared context: read HEAD parent feature %s: %w", ff.Path, err)
	}
	fmBytes, _, err := artifact.SplitFrontmatter(content)
	if err != nil {
		// vocab:identity — "feature" names the fixed governing-parent artifact class this frontmatter split targets (SI-92)
		return nil, fmt.Errorf("contextcompile: resolve declared context: decode parent feature %s: %w", ff.Path, err)
	}
	spec, err := artifact.DecodeSpec(fmBytes)
	if err != nil {
		// vocab:identity — "feature" names the fixed governing-parent artifact class this spec decode targets (SI-92)
		return nil, fmt.Errorf("contextcompile: resolve declared context: decode parent feature %s: %w", ff.Path, err)
	}
	if spec.ID != ff.Ref || spec.Class != artifact.ClassFeature {
		// vocab:identity — "feature" names the fixed governing-parent and re-decoded class identities this TOCTOU diagnostic reports (SI-92)
		return nil, fmt.Errorf("contextcompile: resolve declared context: parent feature %s at %s re-decoded as %s/%s (TOCTOU mismatch)", ff.Ref, ff.Path, spec.ID, spec.Class)
	}
	if rawContentDigest(content) != ff.SourceDigest {
		// vocab:identity — "feature" names the fixed governing-parent artifact class this source-digest TOCTOU diagnostic reports (SI-92)
		return nil, fmt.Errorf("contextcompile: resolve declared context: parent feature %s content at exact HEAD no longer matches its resolved source digest (TOCTOU mismatch)", ff.Ref)
	}
	return spec, nil
}

// resolveDeclaredContextItem resolves one raw pinned ref string into a
// DeclaredContextItem, reading only the exact pinned commit the ref names.
// treeCache is keyed by commit so multiple refs pinned to the same commit
// share one tree listing.
func resolveDeclaredContextItem(ctx context.Context, git GitReader, root, raw string, treeCache map[string][]gitx.TreeEntry) (DeclaredContextItem, error) {
	parsed, err := artifact.ParsePinnedRef(raw)
	if err != nil {
		// vocab:identity — SI-92 diagnostic naming the fixed declared-context
		// ref grammar (kind/name@commit[#object-id])
		return DeclaredContextItem{}, fmt.Errorf("contextcompile: resolve declared context: %w", err)
	}

	entries, ok := treeCache[parsed.Commit]
	if !ok {
		entries, err = git.LsTreeEntries(ctx, root, parsed.Commit)
		if err != nil {
			return DeclaredContextItem{}, fmt.Errorf("contextcompile: resolve declared context: list pinned tree for %s: %w", raw, err)
		}
		treeCache[parsed.Commit] = entries
	}

	var path string
	if parsed.Kind == artifact.KindSpec {
		path, err = specZonePath(entries, parsed.Name)
	} else {
		path, err = store.NonSpecArtifactPath(parsed.Kind, parsed.Name)
	}
	if err != nil {
		return DeclaredContextItem{}, fmt.Errorf("contextcompile: resolve declared context: %s: %w", raw, err)
	}

	if err := requireRegularBlob(entries, path); err != nil {
		return DeclaredContextItem{}, fmt.Errorf("contextcompile: resolve declared context: %s: %w", raw, err)
	}

	content, err := git.Show(ctx, root, parsed.Commit, path)
	if err != nil {
		return DeclaredContextItem{}, fmt.Errorf("contextcompile: resolve declared context: read pinned %s at %s: %w", path, parsed.Commit, err)
	}
	if !utf8.Valid(content) || bytes.IndexByte(content, 0) >= 0 {
		// vocab:identity — SI-92 diagnostic naming the fixed declared-
		// context-ref invalid-authority boundary (strict decode precedes
		// any non-text classification, so invalid UTF-8/NUL never reaches
		// the data-item text predicate)
		return DeclaredContextItem{}, fmt.Errorf("contextcompile: resolve declared context: %s: content at %s is not valid UTF-8 text (invalid authority)", raw, path)
	}

	id, spec, err := decodeArtifactIdentity(parsed.Kind, content)
	if err != nil {
		return DeclaredContextItem{}, fmt.Errorf("contextcompile: resolve declared context: decode %s: %w", raw, err)
	}
	wantID := artifact.Ref{Kind: parsed.Kind, Name: parsed.Name}.String()
	if id != wantID {
		// vocab:identity — SI-92 kind-dispatch identity-check diagnostic
		return DeclaredContextItem{}, fmt.Errorf("contextcompile: resolve declared context: %s: decoded id %q, want %q", raw, id, wantID)
	}

	if parsed.Fragment() {
		if parsed.Kind != artifact.KindSpec {
			// vocab:identity — SI-92 diagnostic naming the fixed non-spec
			// fragment invalid-authority rule
			return DeclaredContextItem{}, fmt.Errorf("contextcompile: resolve declared context: %s: a fragment on a non-spec artifact is invalid authority", raw)
		}
		if !artifact.DeclaredObjectIDs(spec)[parsed.Object] {
			// vocab:identity — SI-92 diagnostic naming the fixed
			// DeclaredObjectIDs fragment-resolution rule (#problem/#outcome
			// are deliberately excluded from that set and so never resolve)
			return DeclaredContextItem{}, fmt.Errorf("contextcompile: resolve declared context: %s: fragment object %q is not declared", raw, parsed.Object)
		}
	}

	return DeclaredContextItem{
		Ref:           parsed.String(),
		LogicalRef:    wantID,
		Path:          path,
		Kind:          parsed.Kind,
		Name:          parsed.Name,
		ContentDigest: rawContentDigest(content),
		Content:       append([]byte(nil), content...),
	}, nil
}

// specZonePath searches entries (a listing of one exact pinned commit's
// tree) for name's spec.md in exactly one of the active or archive zones
// (SI-92: "spec/* resolves exactly one matching active- or archive-zone
// path from the pinned tree"). Zero matches and two matches are both
// invalid authority — an archived or later-superseded pin still resolves
// as long as it is present in exactly one zone at the PINNED commit.
func specZonePath(entries []gitx.TreeEntry, name string) (string, error) {
	activePath := store.ActiveSpecRelPath(name)
	archivePath := store.SpecRelPath(store.ZoneArchive, name)
	var matches []string
	for _, e := range entries {
		if e.Path == activePath || e.Path == archivePath {
			matches = append(matches, e.Path)
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("spec %q is absent from both active and archive zones at the pinned commit", name)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("spec %q matches both active and archive zones at the pinned commit (ambiguous)", name)
	}
}

// requireRegularBlob fails closed unless path names a regular file blob
// (mode 100644/100755, type blob) in entries — the same non-regular-entry
// check ResolveBoundObligations already applies to its own HEAD-tree
// candidates, generalized to an arbitrary pinned tree listing.
func requireRegularBlob(entries []gitx.TreeEntry, path string) error {
	for _, e := range entries {
		if e.Path != path {
			continue
		}
		if e.Type != "blob" || (e.Mode != "100644" && e.Mode != "100755") || e.Object == "" {
			return fmt.Errorf("%s is not a regular blob: %+v", path, e)
		}
		return nil
	}
	return fmt.Errorf("%s is absent from the pinned tree", path)
}

// decodeArtifactIdentity strict-decodes content per kind and returns the
// decoded unpinned whole id (Base.ID — validated unpinned by every kind's
// own validateBase) plus, for kind spec only, the decoded
// *artifact.SpecFrontmatter a fragment resolution needs. This is SI-92's
// kind-dispatch identity check: every existing per-kind Decode* seam
// already embeds Base and so already exposes ID after a successful
// decode, so no new internal/artifact decoder was needed for this
// dispatch.
func decodeArtifactIdentity(kind artifact.Kind, content []byte) (string, *artifact.SpecFrontmatter, error) {
	fmBytes, _, err := artifact.SplitFrontmatter(content)
	if err != nil {
		return "", nil, err
	}
	switch kind {
	case artifact.KindSpec:
		spec, err := artifact.DecodeSpec(fmBytes)
		if err != nil {
			return "", nil, err
		}
		return spec.ID, spec, nil
	case artifact.KindADR:
		fm, err := artifact.DecodeADR(fmBytes)
		if err != nil {
			return "", nil, err
		}
		return fm.ID, nil, nil
	case artifact.KindDiagram:
		fm, err := artifact.DecodeDiagram(fmBytes)
		if err != nil {
			return "", nil, err
		}
		return fm.ID, nil, nil
	case artifact.KindAttestation:
		fm, err := artifact.DecodeAttestation(fmBytes)
		if err != nil {
			return "", nil, err
		}
		return fm.ID, nil, nil
	case artifact.KindWaiver:
		fm, err := artifact.DecodeWaiver(fmBytes)
		if err != nil {
			return "", nil, err
		}
		return fm.ID, nil, nil
	case artifact.KindConflict:
		fm, err := artifact.DecodeConflict(fmBytes)
		if err != nil {
			return "", nil, err
		}
		return fm.ID, nil, nil
	case artifact.KindReaffirmation:
		fm, err := artifact.DecodeReaffirmation(fmBytes)
		if err != nil {
			return "", nil, err
		}
		return fm.ID, nil, nil
	case artifact.KindObligation:
		fm, err := artifact.DecodeObligation(fmBytes)
		if err != nil {
			return "", nil, err
		}
		return fm.ID, nil, nil
	default:
		return "", nil, fmt.Errorf("unknown artifact kind %q", kind)
	}
}
