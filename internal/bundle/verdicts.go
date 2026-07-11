package bundle

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/canonjson"
	"github.com/OWNER/verdi/internal/upstream"
)

// JoinInput is what BuildVerdicts needs to join one service's bindings
// against its graph's obligation statuses and its behavioral test signal
// (I-3: "verdicts.json ... synthesized from graph obligation statuses +
// bindings join (static) and golden/test outcomes (behavioral)").
type JoinInput struct {
	// ServiceName names the service the graph/bindings belong to (used
	// only in error messages and witness text).
	ServiceName string
	// Graph is the service's strict-decoded flowmap graph. Required if
	// Bindings declares any static binding.
	Graph *upstream.Graph
	// Bindings is the service's verdi.bindings.yaml sidecar. A nil
	// Bindings produces no verdicts and is not an error — not every
	// service need bind evidence to a spec.
	Bindings *artifact.Bindings
	// KnownGoldenFlows is the set of golden flow names found on disk
	// under <service>/testdata/flows/ (basenames, extension stripped). A
	// behavioral binding whose producer is not in this set is a dangling
	// binding (unknown producer). Nil disables this check (callers that
	// cannot enumerate goldens accept the risk explicitly).
	KnownGoldenFlows map[string]bool
	// SpecACs is the set of AC ids the bound spec actually declares. A
	// binding naming an AC not in this set is a dangling binding
	// (03 §Declarations: "a misspelled ac-3 must never surface as a
	// silent no-signal"). Nil disables this check.
	SpecACs map[string]bool
	// TestSummary is the behavioral signal: every behavioral binding's
	// verdict is pass iff TestSummary.Suite == "pass" (03 §Declarations:
	// unit-test evidence stays coarse, suite-level). Required if Bindings
	// declares any behavioral binding.
	TestSummary *TestSummary
	// Provenance is stamped onto every produced record, set by the
	// caller (sync.go): source ci for a pulled CI bundle, source local
	// for `sync --or-regen`.
	Provenance artifact.EvidenceProvenance
}

// BuildVerdicts joins in.Bindings against in.Graph's obligations (static)
// and in.TestSummary (behavioral), producing one evidence record per
// matched graph obligation entry (static) or one per behavioral binding.
// It fails loudly — never a silent empty or abstained record — when:
//
//   - a binding names an AC the bound spec does not declare (SpecACs, if
//     given);
//   - a static binding's producer matches no obligation rule in the graph;
//   - a matched obligation's status is UNMATCHED (I-3: "hard error");
//   - a behavioral binding's producer is not a known golden flow
//     (KnownGoldenFlows, if given);
//   - a binding names a kind this package cannot assemble in v0
//     (runtime/attestation have no toolchain producer — OQ-2).
//
// Output order follows in.Bindings.Bindings's declared order (and, for a
// static binding matching multiple call sites, the graph's obligations[]
// order) — both already deterministic, so no additional sort is needed for
// byte-identical canonjson output.
func BuildVerdicts(in JoinInput) ([]artifact.Evidence, error) {
	if in.Bindings == nil {
		return nil, nil
	}

	obligationsByRule := make(map[string][]upstream.Obligation)
	if in.Graph != nil {
		for _, o := range in.Graph.Obligations {
			obligationsByRule[o.Rule] = append(obligationsByRule[o.Rule], o)
		}
	}

	var out []artifact.Evidence
	for _, b := range in.Bindings.Bindings {
		if in.SpecACs != nil {
			for _, ac := range b.ACs {
				if !in.SpecACs[ac] {
					return nil, fmt.Errorf("bundle: service %s: binding %q names ac %q, which spec %q does not declare (dangling binding, 03 §Declarations)", in.ServiceName, b.Producer, ac, in.Bindings.Spec)
				}
			}
		}

		switch b.Kind {
		case artifact.EvidenceStatic:
			recs, err := staticRecords(in, b)
			if err != nil {
				return nil, err
			}
			out = append(out, recs...)

		case artifact.EvidenceBehavioral:
			rec, err := behavioralRecord(in, b)
			if err != nil {
				return nil, err
			}
			out = append(out, rec)

		default:
			return nil, fmt.Errorf("bundle: service %s: binding %q: kind %q has no toolchain producer in v0 (runtime is deferred per OQ-2; attestation is file-existence, not a toolchain join)", in.ServiceName, b.Producer, b.Kind)
		}
	}
	return out, nil
}

func staticRecords(in JoinInput, b artifact.Binding) ([]artifact.Evidence, error) {
	matches := obligationsFor(in, b.Producer)
	if len(matches) == 0 {
		return nil, fmt.Errorf("bundle: service %s: binding %q (static): no graph obligation named %q was found (dangling binding: unknown producer, 03 §Declarations)", in.ServiceName, b.Producer, b.Producer)
	}

	recs := make([]artifact.Evidence, 0, len(matches))
	for _, o := range matches {
		verdict, err := staticVerdict(o)
		if err != nil {
			return nil, fmt.Errorf("bundle: service %s: binding %q: %w", in.ServiceName, b.Producer, err)
		}
		digest, err := recordDigest(o)
		if err != nil {
			return nil, err
		}
		recs = append(recs, artifact.Evidence{
			Schema:      "verdi.evidence/v1",
			EvidenceFor: append([]string(nil), b.ACs...),
			Kind:        artifact.EvidenceStatic,
			Verdict:     verdict,
			Witness:     staticWitness(o),
			Producer:    b.Producer,
			Provenance:  in.Provenance,
			Digest:      digest,
		})
	}
	return recs, nil
}

func obligationsFor(in JoinInput, producer string) []upstream.Obligation {
	if in.Graph == nil {
		return nil
	}
	var out []upstream.Obligation
	for _, o := range in.Graph.Obligations {
		if o.Rule == producer {
			out = append(out, o)
		}
	}
	return out
}

// staticVerdict maps a graph obligation's status to an evidence verdict
// per I-3's status map: SATISFIED -> pass, VIOLATED -> fail,
// CANT-PROVE -> abstain, UNMATCHED -> hard error (never a silent abstain —
// an UNMATCHED rule means the binding's producer never even fired, which
// is worse than "we couldn't prove it").
func staticVerdict(o upstream.Obligation) (artifact.EvidenceVerdict, error) {
	switch o.Status {
	case upstream.ObligationSatisfied:
		return artifact.VerdictPass, nil
	case upstream.ObligationViolated:
		return artifact.VerdictFail, nil
	case upstream.ObligationCantProve:
		return artifact.VerdictAbstain, nil
	case upstream.ObligationUnmatched:
		return "", fmt.Errorf("graph obligation %q status UNMATCHED (%s): the rule's anchor matched no call site — hard error per I-3, never a silent abstain", o.Rule, o.Detail)
	default:
		return "", fmt.Errorf("graph obligation %q: unrecognized status %q", o.Rule, o.Status)
	}
}

func staticWitness(o upstream.Obligation) string {
	switch {
	case o.Fn != "" && o.Site != "":
		return fmt.Sprintf("%s @ %s", o.Fn, o.Site)
	case o.Fn != "":
		return o.Fn
	default:
		return o.Rule
	}
}

func behavioralRecord(in JoinInput, b artifact.Binding) (artifact.Evidence, error) {
	if in.KnownGoldenFlows != nil && !in.KnownGoldenFlows[b.Producer] {
		return artifact.Evidence{}, fmt.Errorf("bundle: service %s: binding %q (behavioral): no golden flow named %q found under testdata/flows (dangling binding: unknown producer)", in.ServiceName, b.Producer, b.Producer)
	}
	if in.TestSummary == nil {
		return artifact.Evidence{}, fmt.Errorf("bundle: service %s: binding %q (behavioral): no test summary available", in.ServiceName, b.Producer)
	}

	verdict := artifact.VerdictPass
	if in.TestSummary.Suite != "pass" {
		verdict = artifact.VerdictFail
	}
	digest, err := recordDigest(in.TestSummary)
	if err != nil {
		return artifact.Evidence{}, err
	}
	return artifact.Evidence{
		Schema:      "verdi.evidence/v1",
		EvidenceFor: append([]string(nil), b.ACs...),
		Kind:        artifact.EvidenceBehavioral,
		Verdict:     verdict,
		Witness:     fmt.Sprintf("golden flow %q via go test suite (%s)", b.Producer, in.ServiceName),
		Producer:    b.Producer,
		Provenance:  in.Provenance,
		Digest:      digest,
	}, nil
}

// recordDigest hashes v's canonjson encoding: a deterministic
// content-address of the upstream fact (an obligation entry, or the test
// summary) that produced one evidence record, in the spirit of 02
// §Generated artifacts and digests's "recomputable from inputs" — a future
// `verdi verify-artifact` (out of v0 scope) could recompute it the same
// way.
func recordDigest(v interface{}) (string, error) {
	data, err := canonjson.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("bundle: computing record digest: %w", err)
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}
