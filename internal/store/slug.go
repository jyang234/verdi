package store

import (
	"fmt"
	"strings"
)

// RefSlug maps a git ref to its normative slug form (01 §notes:
// "Ref slugging is normative"): the ref lowercased, with "/" mapped to
// "--" and every remaining byte outside [a-z0-9._-] mapped to "-". Order
// matters: lowercasing and the "/" -> "--" substitution happen first, then
// every byte still outside the allowed set — including a literal
// underscore, which the allowed set excludes — is mapped to "-".
//
// RefSlug alone cannot detect a collision between two different refs that
// happen to map to the same slug; CheckSlugCollisions does that over a
// whole ref set.
func RefSlug(ref string) string {
	lower := strings.ToLower(ref)
	dashed := strings.ReplaceAll(lower, "/", "--")

	var b strings.Builder
	b.Grow(len(dashed))
	for _, r := range dashed {
		if isSlugByte(r) {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	return b.String()
}

// isSlugByte reports whether r is in the allowed slug alphabet
// [a-z0-9._-]. RefSlug already lowercases before calling this, but the
// check itself does not assume that, so it stays correct if ever reused
// standalone.
func isSlugByte(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z':
		return true
	case r >= '0' && r <= '9':
		return true
	case r == '.' || r == '-':
		return true
	default:
		// Everything else — including '_', which the allowed set
		// [a-z0-9._-] deliberately excludes — maps to '-'.
		return false
	}
}

// CheckSlugCollisions computes RefSlug over every ref in refs and returns
// an error naming both refs the first time two distinct refs map to the
// same slug (01 §notes: "Two refs that collide after mapping are a hard
// error naming both — never a silent merge"). refs are checked in the
// order given, so the error is deterministic for a given input order.
func CheckSlugCollisions(refs []string) error {
	bySlug := make(map[string]string, len(refs))
	for _, ref := range refs {
		slug := RefSlug(ref)
		if prior, ok := bySlug[slug]; ok && prior != ref {
			return fmt.Errorf("store: ref slug collision: %q and %q both map to slug %q", prior, ref, slug)
		}
		bySlug[slug] = ref
	}
	return nil
}
