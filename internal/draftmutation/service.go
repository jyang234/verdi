package draftmutation

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/designprovenance"
	"github.com/jyang234/verdi/internal/store"
)

// Response is the exact service result union. Stale is the only refusal with
// a structured public record; all other refusals are returned as typed errors.
type Response struct {
	Result *Result
	Stale  *StaleRefusal
}

// Service composes canonical identity, authority, pure mutation, provenance,
// and the coordinated checkout-wide transaction.
type Service struct {
	Identity    IdentityReader
	State       StateProjector
	Policy      PolicySource
	Coordinator Coordinator
}

// NewService returns the production service with Git-backed identity/state
// and constitution-backed policy authority.
func NewService() Service {
	return Service{
		Identity: GitIdentityReader{},
		State:    NewGitStateProjector(),
		Policy:   ConstitutionPolicySource{},
	}
}

// Mutate applies one already strict-decoded request. After construction,
// identity is copied unchanged into success, stale, and every typed error.
func (s Service) Mutate(ctx context.Context, start string, request Request, actor Actor) (Response, *Error) {
	identity, err := ResolveCanonicalIdentity(ctx, start, request.Spec, s.Identity)
	if err != nil {
		return Response{}, NewIdentityUnavailableError("constructing canonical identity", err)
	}
	if typed := VerifyExpected(identity, request.Expected); typed != nil {
		return Response{}, typed
	}
	ref, err := artifact.ParseRef(identity.Spec)
	if err != nil {
		return Response{}, WrapError(CodeAuthorityInvalid, identity, "parsing canonical spec identity", err)
	}

	var response Response
	transactionErr := WithWriterLock(ctx, filepath.FromSlash(identity.Checkout), s.Coordinator, func(writer *LockedWriter) error {
		if err := writer.Recover(ref.Name); err != nil {
			// vocab:identity — ASD protocol/transaction name in a machinery diagnostic
			return WrapError(CodeRecoveryInvalid, identity, "recovering prior draft mutation", err)
		}

		current, provenance, provenanceExists, typed := readMutationState(identity, ref.Name)
		if typed != nil {
			return typed
		}
		currentDigest := DigestBytes(current)
		if currentDigest != request.BaseDigest {
			targets, err := ChangedTargets(request.BaseSpec, current)
			if err != nil {
				return WrapError(CodeAuthorityInvalid, identity, "computing stale changed targets", err)
			}
			stale := StaleRefusal{
				Schema: RefusalSchema, Identity: identity, Code: CodeStaleBase,
				CurrentDigest: currentDigest, ChangedTargets: targets,
			}
			if err := stale.Validate(); err != nil {
				return WrapError(CodeResultInvalid, identity, "validating stale refusal", err)
			}
			response.Stale = &stale
			return NewError(CodeStaleBase, identity, "base digest does not match current spec bytes")
		}

		if _, typed := AuthorizeState(ctx, filepath.FromSlash(identity.Checkout), identity, current, s.State); typed != nil {
			return typed
		}
		policy, typed := authorizeMutationPolicy(ctx, filepath.FromSlash(identity.Checkout), identity, actor, s.Policy)
		if typed != nil {
			return typed
		}

		entries, err := designprovenance.DecodeLog(provenance)
		if err != nil {
			return WrapError(CodeAuthorityInvalid, identity, "decoding design provenance", err)
		}
		for i, entry := range entries {
			if entry.Spec != identity.Spec {
				return NewError(CodeAuthorityInvalid, identity, fmt.Sprintf("design provenance entry[%d] names %q", i, entry.Spec))
			}
		}

		var gap *designprovenance.UnclassifiedGap
		if len(entries) > 0 && entries[len(entries)-1].ResultDigest != currentDigest {
			gap = &designprovenance.UnclassifiedGap{
				FromDigest: entries[len(entries)-1].ResultDigest,
				ToDigest:   currentDigest,
			}
		}

		applied, err := Apply(current, request, identity)
		if err != nil {
			return WrapError(CodeOperationInvalid, identity, "applying ordered mutation batch", err)
		}
		if err := validateEffectiveResult(current, applied); err != nil {
			return WrapError(CodeResultInvalid, identity, "mutation result is not an effective semantic change", err)
		}
		if gap != nil {
			applied.Result.Disclosures = append([]Disclosure{{
				Code: DisclosureUnclassifiedDirectEdit, FromDigest: gap.FromDigest, ToDigest: gap.ToDigest,
			}}, applied.Result.Disclosures...)
		}
		if err := applied.Result.Validate(); err != nil {
			return WrapError(CodeResultInvalid, identity, "validating mutation result", err)
		}

		entry := designprovenance.Entry{
			Schema: designprovenance.SchemaV2, Spec: identity.Spec,
			PreviousDigest: currentDigest, ResultDigest: applied.Result.ResultDigest,
			UnclassifiedGap: gap, Attribution: actor.Attribution(), Harness: actor.Harness(), Session: actor.Session(),
			Policy: &policy, Context: designprovenance.UnavailableContext(),
			Operations: append(make([]designprovenance.Operation, 0, len(request.Operations)), request.Operations...),
			Changes:    append(make([]designprovenance.Change, 0, len(applied.Result.Changes)), applied.Result.Changes...),
			Excerpts:   append(make([]designprovenance.Excerpt, 0, len(applied.ProvenanceExcerpts)), applied.ProvenanceExcerpts...),
		}
		if err := entry.Seal(); err != nil {
			return WrapError(CodeResultInvalid, identity, "sealing design provenance entry", err)
		}
		encoded, err := designprovenance.EncodeEntry(entry)
		if err != nil {
			return WrapError(CodeResultInvalid, identity, "encoding design provenance entry", err)
		}
		newProvenance := make([]byte, 0, len(provenance)+len(encoded))
		newProvenance = append(newProvenance, provenance...)
		newProvenance = append(newProvenance, encoded...)
		if _, err := designprovenance.DecodeLog(newProvenance); err != nil {
			return WrapError(CodeResultInvalid, identity, "validating appended design provenance", err)
		}

		if err := writer.Commit(Transaction{
			Spec: identity.Spec, OldSpec: current, NewSpec: applied.Spec,
			OldProvenance: provenance, OldProvenanceExists: provenanceExists, NewProvenance: newProvenance,
		}); err != nil {
			// vocab:identity — ASD protocol/transaction name in a machinery diagnostic
			return WrapError(CodeIOFailure, identity, "committing coordinated draft mutation", err)
		}
		result := applied.Result
		response.Result = &result
		return nil
	})
	if transactionErr == nil {
		return response, nil
	}
	var typed *Error
	if errors.As(transactionErr, &typed) {
		return response, typed
	}
	// vocab:identity — ASD protocol/transaction name in a machinery diagnostic
	return response, WrapError(CodeIOFailure, identity, "running checkout-wide draft mutation transaction", transactionErr)
}

// authorizeMutationPolicy is Mutate's one policy-authorization dispatch: the
// explicit browser-human actor (isExplicitBrowserHuman) routes through
// AuthorizeBrowserHuman, independent of design_assistance mode/adoption
// (§4.1, SI-176); every other actor keeps AuthorizePolicy's existing,
// unchanged matrix. Both outcomes project onto the same v2 policy union so
// every current writer emits exactly one closed arm: a resolved digest, or
// the explicit browser-human's honest not-applicable declaration.
func authorizeMutationPolicy(ctx context.Context, root string, identity Identity, actor Actor, source PolicySource) (designprovenance.Policy, *Error) {
	if actor.isExplicitBrowserHuman() {
		posture, typed := AuthorizeBrowserHuman(ctx, root, identity, actor, source)
		if typed != nil {
			return designprovenance.Policy{}, typed
		}
		if !posture.Adopted {
			return designprovenance.Policy{State: designprovenance.PolicyNotApplicable}, nil
		}
		return designprovenance.Policy{State: designprovenance.PolicyResolved, Digest: posture.Digest}, nil
	}
	grant, typed := AuthorizePolicy(ctx, root, identity, actor, source)
	if typed != nil {
		return designprovenance.Policy{}, typed
	}
	return designprovenance.Policy{State: designprovenance.PolicyResolved, Digest: grant.Digest}, nil
}

func readMutationState(identity Identity, name string) ([]byte, []byte, bool, *Error) {
	root := filepath.FromSlash(identity.Checkout)
	specPath := store.SpecPath(root, store.ZoneActive, name)
	if err := validateDirectoryChain(root, filepath.Dir(specPath)); err != nil {
		return nil, nil, false, WrapError(CodeIOFailure, identity, "validating spec directory", err)
	}
	if err := validateRegularDestination(specPath, true); err != nil {
		return nil, nil, false, WrapError(CodeIOFailure, identity, "validating spec destination", err)
	}
	current, err := os.ReadFile(specPath)
	if err != nil {
		return nil, nil, false, WrapError(CodeIOFailure, identity, "reading current spec", err)
	}
	provenance, exists, err := readOptionalRegular(store.DesignProvenancePath(root, store.ZoneActive, name))
	if err != nil {
		return nil, nil, false, WrapError(CodeIOFailure, identity, "reading design provenance", err)
	}
	return current, provenance, exists, nil
}

func validateEffectiveResult(current []byte, applied Applied) error {
	if bytes.Equal(current, applied.Spec) || DigestBytes(current) == applied.Result.ResultDigest {
		return fmt.Errorf("result bytes are unchanged")
	}
	if len(applied.Result.Changes) == 0 {
		return fmt.Errorf("result contains no changes")
	}
	for i, change := range applied.Result.Changes {
		if change.Change == designprovenance.ChangeReplaced && change.BeforeDigest == change.AfterDigest {
			return fmt.Errorf("change[%d] leaves target %q unchanged", i, change.Target)
		}
	}
	return nil
}
