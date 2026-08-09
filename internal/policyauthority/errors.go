package policyauthority

import "errors"

// ErrNotAdopted is returned by Load when root/.verdi/policy/ does not
// exist. A legacy project without a constitution store is an EXPECTED
// state, never a constitution claim (a project that has not adopted a
// constitution has no policy to violate) — callers distinguish this from
// every other Load failure with errors.Is(err, ErrNotAdopted).
var ErrNotAdopted = errors.New("policyauthority: .verdi/policy/ does not exist (constitution store not adopted)")

// ErrIncompleteAdoption is returned by Load when root/.verdi/policy/
// exists but carries no constitution.md manifest. A policy/ directory
// with policies, overlays, or exemptions but no constitution can never
// resolve: there is no selected profile, catalog, or subject
// registration to validate against, so this state is a distinct,
// explicitly named failure rather than a silent empty result.
var ErrIncompleteAdoption = errors.New("policyauthority: .verdi/policy/ exists but constitution.md is missing (incomplete adoption)")
