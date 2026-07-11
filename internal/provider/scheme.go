package provider

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// ErrInvalidRef means a StoryRef is not well-formed scheme:key.
var ErrInvalidRef = errors.New("provider: invalid story ref")

// schemeRe matches the scheme prefix of a StoryRef: lowercase, starting
// with a letter (04 §Reference scheme's "jira", "gitlab" examples).
var schemeRe = regexp.MustCompile(`^[a-z][a-z0-9]*$`)

// ParseStoryRef splits a StoryRef into its scheme and key (04 §Reference
// scheme), e.g.:
//
//	"jira:LOAN-1482"     -> ("jira", "LOAN-1482")
//	"gitlab:platform#482" -> ("gitlab", "platform#482")
//
// The key is adapter-defined and unconstrained beyond being non-empty;
// only the scheme prefix is validated here. The scheme is matched on the
// first ':' so keys may contain their own punctuation (e.g. gitlab's
// "#482" issue suffix).
func ParseStoryRef(ref StoryRef) (scheme, key string, err error) {
	s := string(ref)
	scheme, key, ok := strings.Cut(s, ":")
	if !ok {
		return "", "", fmt.Errorf("%w: %q: missing ':' scheme separator", ErrInvalidRef, s)
	}
	if !schemeRe.MatchString(scheme) {
		return "", "", fmt.Errorf("%w: %q: scheme %q must match %s", ErrInvalidRef, s, scheme, schemeRe.String())
	}
	if key == "" {
		return "", "", fmt.Errorf("%w: %q: key is empty", ErrInvalidRef, s)
	}
	return scheme, key, nil
}
