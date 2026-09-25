package artifact

import "fmt"

// ValidateFor validates l as an entry of the `links:` block of an artifact
// of kind owner. It is Link.Validate with one widening, closed-spec object
// supersession's (02 §Link taxonomy: "A conflict's `challenges` links may
// also target object fragments, all naming objects of one spec"): when owner
// is a conflict, a `challenges` link may target a spec object fragment,
// spec/<name>#<object-id>. A conflict's `challenges` fragment naming any
// other kind fails closed, and so does a pinned one, spec/<name>@<sha>#<id>
// (02 §Common frontmatter: refs inside links are unpinned; SI-271); a pinned
// whole-artifact `challenges` ref keeps Link.Validate's verdict. For every
// other owner, link type, and ref it returns exactly what Link.Validate
// returns, so the closed five-value spec-object edge vocabulary is unchanged
// in every other context and a context-free caller of Link.Validate stays
// fail-closed on a fragment `challenges` link.
//
// This is the one definition of the kind-aware link check: validateBase uses
// it for every artifact kind, and a caller that decodes without the kind's
// own Validate (internal/lint's VL-003) can call it with the document's kind.
// Whether a conflict's fragments all name one spec is VL-026's, not this
// check's (design §7).
func (l Link) ValidateFor(owner Kind) error {
	if owner != KindConflict || l.Type != LinkChallenges {
		return l.Validate()
	}
	ref, err := ParseRef(l.Ref)
	if err != nil || !ref.Fragment() {
		return l.Validate()
	}
	if ref.Kind != KindSpec {
		return fmt.Errorf("artifact: conflict challenges link targets object fragment %q, but a conflict's fragment challenges name a spec object, spec/<name>#<object-id> (02 §Link taxonomy)", l.Ref)
	}
	if ref.Pinned() {
		return fmt.Errorf("artifact: conflict challenges link targets pinned object fragment %q (pinned at %q), but refs inside links are unpinned and a conflict's fragment challenges name a spec object as spec/<name>#<object-id> (02 §Common frontmatter, SI-271)", l.Ref, ref.Commit)
	}
	return nil
}

// ValidateResolvedBy checks a conflict's resolved_by against the decode scope
// SI-269 chose for 02 §Kind registry's field ("A superseded conflict whose
// `challenges` name object fragments also carries `resolved_by:
// spec/<name>`"): absent is always accepted here; present, it is accepted
// only on a conflict whose status is superseded and at least one of whose
// `challenges` links targets an object fragment, and only as an unpinned
// spec/<name> ref with no fragment. Anything else names a resolution that
// did not happen and fails closed. Its required presence and the named
// spec's existence are VL-026's, never this check's (design §7), so it reads
// no corpus.
//
// An explicitly empty value is indistinguishable from an absent one in the
// string field, so it decodes as absent.
func (fm ConflictFrontmatter) ValidateResolvedBy() error {
	if fm.ResolvedBy == "" {
		return nil
	}
	if fm.Status != "superseded" {
		// vocab:identity — strict-decode/schema diagnostic speaking status/field ids
		return fmt.Errorf("artifact: conflict resolved_by %q is accepted only on a conflict whose status is superseded, not %q (02 §Kind registry, SI-269)", fm.ResolvedBy, fm.Status)
	}
	if !fm.challengesObjectFragment() {
		return fmt.Errorf("artifact: conflict resolved_by %q is accepted only on a conflict whose challenges links name an object fragment, and none of this conflict's do (02 §Kind registry, SI-269)", fm.ResolvedBy)
	}
	ref, err := ParseRef(fm.ResolvedBy)
	if err != nil {
		return fmt.Errorf("artifact: conflict resolved_by %q must be an unpinned spec/<name> ref (02 §Kind registry): %w", fm.ResolvedBy, err)
	}
	if ref.Kind != KindSpec || ref.Pinned() || ref.Fragment() {
		return fmt.Errorf("artifact: conflict resolved_by %q must be an unpinned spec/<name> ref with no fragment (02 §Kind registry)", fm.ResolvedBy)
	}
	return nil
}

// challengesObjectFragment reports whether any of the conflict's
// `challenges` links targets an object fragment.
func (fm ConflictFrontmatter) challengesObjectFragment() bool {
	for _, l := range fm.Links {
		if l.Type != LinkChallenges {
			continue
		}
		if ref, err := ParseRef(l.Ref); err == nil && ref.Fragment() {
			return true
		}
	}
	return false
}
