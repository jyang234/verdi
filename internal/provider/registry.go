package provider

import (
	"context"
	"errors"
	"fmt"
)

// ErrUnknownScheme means no adapter is registered for a StoryRef's
// scheme. VL-005 (02 §Lint) rejects unconfigured schemes at lint time;
// this is the runtime counterpart for anything that reaches Resolve or
// PublishRollup without having gone through lint first.
var ErrUnknownScheme = errors.New("provider: unknown scheme")

// Registry selects a StoryProvider by a StoryRef's scheme (04 §Reference
// scheme: "the scheme selects the adapter at runtime from verdi.yaml's
// providers: map"). Registry itself implements StoryProvider, dispatching
// each call to the adapter registered for the ref's scheme, so it can be
// used anywhere a single StoryProvider is expected (including wrapped in
// a CachingProvider).
//
// Registry is built from a plain map so this package never imports the
// store/config package that decodes verdi.yaml; callers translate the
// providers: map into a map[string]StoryProvider (scheme -> adapter)
// themselves.
type Registry struct {
	providers map[string]StoryProvider
}

// NewRegistry builds a Registry from scheme -> adapter. The map is copied,
// so later mutation of the argument does not affect the Registry.
func NewRegistry(providers map[string]StoryProvider) *Registry {
	cp := make(map[string]StoryProvider, len(providers))
	for k, v := range providers {
		cp[k] = v
	}
	return &Registry{providers: cp}
}

// Provider returns the adapter registered for scheme, or ErrUnknownScheme.
func (r *Registry) Provider(scheme string) (StoryProvider, error) {
	p, ok := r.providers[scheme]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownScheme, scheme)
	}
	return p, nil
}

// Resolve parses ref's scheme and delegates to the registered adapter.
func (r *Registry) Resolve(ctx context.Context, ref StoryRef) (Story, error) {
	scheme, _, err := ParseStoryRef(ref)
	if err != nil {
		return Story{}, err
	}
	p, err := r.Provider(scheme)
	if err != nil {
		return Story{}, err
	}
	return p.Resolve(ctx, ref)
}

// PublishRollup parses roll.Story's scheme and delegates to the
// registered adapter.
func (r *Registry) PublishRollup(ctx context.Context, roll Rollup) error {
	scheme, _, err := ParseStoryRef(roll.Story)
	if err != nil {
		return err
	}
	p, err := r.Provider(scheme)
	if err != nil {
		return err
	}
	return p.PublishRollup(ctx, roll)
}

var _ StoryProvider = (*Registry)(nil)
