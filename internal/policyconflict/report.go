package policyconflict

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/policyartifact"
)

func reportInput(view contextcompile.ConflictView, req Request, evaluatedOn string) (InputIdentity, string, error) {
	snapshot := view.Snapshot
	targetSource, err := exactTargetSource(snapshot, req)
	if err != nil {
		return InputIdentity{}, "", err
	}

	target := TargetIdentity{}
	switch req.Target.Kind {
	case TargetAcceptedContext:
		target = TargetIdentity{
			Kind:     TargetAcceptedContext,
			Accepted: &AcceptedIdentity{ManifestDigest: snapshot.ManifestDigest},
		}
	case TargetAcceptanceCandidate:
		if !snapshot.Repository.Branch.Known || !snapshot.Repository.Head.Known {
			return InputIdentity{}, "", fmt.Errorf("candidate snapshot has no proven branch/HEAD")
		}
		target = TargetIdentity{
			Kind: TargetAcceptanceCandidate,
			Candidate: &CandidateIdentity{
				Ref:           targetSource.Ref,
				Path:          targetSource.Path,
				Branch:        snapshot.Repository.Branch.Value,
				Head:          snapshot.Repository.Head.Value,
				Blob:          snapshot.CandidateBlob,
				ContentDigest: targetSource.ContentDigest,
				Scope:         snapshot.Scope,
				Adapter:       snapshot.Adapter,
				GrantDigest:   snapshot.GrantDigest,
			},
		}
	default:
		return InputIdentity{}, "", fmt.Errorf("unknown request target kind %q", req.Target.Kind)
	}

	entries := make([]PolicyEntryIdentity, len(snapshot.PolicyEntries))
	for i, entry := range snapshot.PolicyEntries {
		entries[i] = PolicyEntryIdentity{Kind: entry.Kind, ID: entry.ID, Digest: entry.Digest}
	}
	return InputIdentity{
		Target:                target,
		Repository:            snapshot.Repository,
		ConstitutionDigest:    snapshot.ConstitutionDigest,
		EffectivePolicyDigest: snapshot.EffectivePolicyDigest,
		PolicyEntries:         entries,
		Profile: ProfileIdentity{
			ID:     snapshot.ProfileID,
			Class:  string(view.Profile.Class),
			Digest: snapshot.ProfileDigest,
		},
		EvaluatedOn: evaluatedOn,
	}, targetSource.ContentDigest, nil
}

func exactTargetSource(snapshot contextcompile.SnapshotIdentity, req Request) (contextcompile.ConflictSourceIdentity, error) {
	wantRef := ""
	switch req.Target.Kind {
	case TargetAcceptedContext:
		wantRef = req.Target.AcceptedContext.Spec
	case TargetAcceptanceCandidate:
		wantRef = req.Target.AcceptanceCandidate.Spec
	default:
		return contextcompile.ConflictSourceIdentity{}, fmt.Errorf("unknown target kind %q", req.Target.Kind)
	}

	var found []contextcompile.ConflictSourceIdentity
	for _, source := range snapshot.Sources {
		if source.Ref == wantRef {
			found = append(found, source)
		}
	}
	if len(found) != 1 {
		return contextcompile.ConflictSourceIdentity{}, fmt.Errorf("sealed snapshot has %d sources for exact target ref %q, want one", len(found), wantRef)
	}
	source := found[0]
	if filepath.IsAbs(source.Path) {
		return contextcompile.ConflictSourceIdentity{}, fmt.Errorf("sealed target path is absolute")
	}
	if err := validateDigest("sealed target content digest", source.ContentDigest); err != nil {
		return contextcompile.ConflictSourceIdentity{}, err
	}
	if req.Target.Kind == TargetAcceptanceCandidate && source.ContentDigest != snapshot.CandidateDigest {
		return contextcompile.ConflictSourceIdentity{}, fmt.Errorf("candidate content digest %s does not match sealed target source digest %s", snapshot.CandidateDigest, source.ContentDigest)
	}
	return source, nil
}

func reportSemanticClaims(claims []contextcompile.ProseClaim) []policyartifact.SemanticClaimWitness {
	out := make([]policyartifact.SemanticClaimWitness, len(claims))
	for i, claim := range claims {
		out[i] = policyartifact.SemanticClaimWitness{
			ID:              claim.ID,
			Digest:          claim.TextDigest,
			Category:        claim.Category,
			AuthorityDigest: claim.AuthorityDigest,
			Scope:           claim.Scope,
			Values:          []string{},
		}
	}
	return out
}

func mergeReportDisclosures(sets ...[]Disclosure) ([]Disclosure, error) {
	byCode := make(map[DisclosureCode]map[string]bool)
	for _, set := range sets {
		for _, disclosure := range set {
			if err := validateDisclosure("report disclosure input", disclosure); err != nil {
				return nil, err
			}
			witnesses := byCode[disclosure.Code]
			if witnesses == nil {
				witnesses = make(map[string]bool)
				byCode[disclosure.Code] = witnesses
			}
			for _, witness := range disclosure.Witnesses {
				witnesses[witness] = true
			}
		}
	}
	codes := make([]DisclosureCode, 0, len(byCode))
	for code := range byCode {
		codes = append(codes, code)
	}
	sort.Slice(codes, func(i, j int) bool { return codes[i] < codes[j] })
	out := make([]Disclosure, 0, len(codes))
	for _, code := range codes {
		witnesses := make([]string, 0, len(byCode[code]))
		for witness := range byCode[code] {
			witnesses = append(witnesses, witness)
		}
		sort.Strings(witnesses)
		out = append(out, Disclosure{Code: code, Witnesses: witnesses})
	}
	return out, nil
}

func compilerDisclosures(codes []contextcompile.DisclosureCode) ([]Disclosure, error) {
	out := make([]Disclosure, len(codes))
	for i, code := range codes {
		if err := validateDisclosureCode(code); err != nil {
			return nil, err
		}
		out[i] = Disclosure{Code: code, Witnesses: []string{}}
	}
	return out, nil
}

func blockingCompilerDisclosure(code DisclosureCode) bool {
	switch code {
	case contextcompile.DisclosureApplicabilityUnknown,
		contextcompile.DisclosureReviewResultDiffUnproven,
		contextcompile.DisclosureReviewEvidenceBundleUnproven,
		contextcompile.DisclosureReviewBuilderReceiptUnproven:
		return true
	default:
		return false
	}
}

func reportVerdict(mechanical []MechanicalEvaluation, semantic []SemanticEvaluation, disclosures []Disclosure) Verdict {
	unproven := false
	for _, row := range mechanical {
		switch row.State {
		case ProofViolatedWithWitness:
			return VerdictBlockedViolated
		case ProofUnproven:
			unproven = true
		}
	}
	for _, row := range semantic {
		switch row.State {
		case ProofViolatedWithWitness:
			return VerdictBlockedViolated
		case ProofUnproven:
			unproven = true
		}
	}
	for _, disclosure := range disclosures {
		if blockingCompilerDisclosure(disclosure.Code) {
			unproven = true
		}
	}
	if unproven {
		return VerdictBlockedUnproven
	}
	return VerdictPass
}

func canonicalResult(report Report) (Result, error) {
	bytes, err := EncodeReport(report)
	if err != nil {
		return Result{}, err
	}
	decoded, err := DecodeReport(bytes)
	if err != nil {
		return Result{}, err
	}
	return Result{Report: decoded, ReportBytes: bytes}, nil
}
