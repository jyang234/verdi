package evidence

import (
	"testing"

	"github.com/OWNER/verdi/internal/artifact"
)

// TestCurrent_FlakeResolvesToLatestByPipelineJob proves the flake case:
// a same-commit retry with a higher job id wins, pass-after-fail included
// (03 §The fold: "the latest record per (kind, producer) wins, including
// across retries on the same commit").
func TestCurrent_FlakeResolvesToLatestByPipelineJob(t *testing.T) {
	fail := testEvidence(artifact.EvidenceStatic, artifact.VerdictFail, "ac-1",
		withProducer("retryWorker"), withPipeline("913"), withJob("1"), withCommit("7f3c2a1"))
	pass := testEvidence(artifact.EvidenceStatic, artifact.VerdictPass, "ac-1",
		withProducer("retryWorker"), withPipeline("913"), withJob("2"), withCommit("7f3c2a1"))

	got := Current([]artifact.Evidence{fail, pass})
	if len(got) != 1 {
		t.Fatalf("Current = %d records, want 1 (deduped to the latest retry)", len(got))
	}
	if got[0].Verdict != artifact.VerdictPass {
		t.Fatalf("Current()[0].Verdict = %q, want pass (the later job wins over the earlier fail)", got[0].Verdict)
	}

	// Order reversed in the input: result must not depend on input order.
	got = Current([]artifact.Evidence{pass, fail})
	if len(got) != 1 || got[0].Verdict != artifact.VerdictPass {
		t.Fatalf("Current (reversed input) = %v, want a single pass record regardless of input order", got)
	}
}

// TestCurrent_NumericPipelineOrdering proves pipeline/job comparison is
// numeric, not lexicographic — "10" must sort after "9" (I-25: monotonic
// ordering).
func TestCurrent_NumericPipelineOrdering(t *testing.T) {
	nine := testEvidence(artifact.EvidenceStatic, artifact.VerdictFail, "ac-1", withProducer("p"), withPipeline("9"))
	ten := testEvidence(artifact.EvidenceStatic, artifact.VerdictPass, "ac-1", withProducer("p"), withPipeline("10"))

	got := Current([]artifact.Evidence{nine, ten})
	if len(got) != 1 || got[0].Provenance.Pipeline != "10" {
		t.Fatalf("Current = %v, want the numerically-later pipeline 10 to win over 9 (not lexicographic, where \"10\" < \"9\")", got)
	}
}

// TestCurrent_DistinctProducersNotCollapsed proves two records of the same
// kind but different producers are two separate groups, not one deduped
// record — dedup is scoped to (kind, producer), not kind alone.
func TestCurrent_DistinctProducersNotCollapsed(t *testing.T) {
	a := testEvidence(artifact.EvidenceStatic, artifact.VerdictPass, "ac-1", withProducer("obligationA"))
	b := testEvidence(artifact.EvidenceStatic, artifact.VerdictFail, "ac-1", withProducer("obligationB"))

	got := Current([]artifact.Evidence{a, b})
	if len(got) != 2 {
		t.Fatalf("Current = %d records, want 2 (distinct producers kept separately)", len(got))
	}
}

// TestCurrent_WitnessFallback proves that when Producer is absent, records
// are grouped (and deduped) by witness text instead — the documented
// fallback for records whose producer identity was never recorded on
// disk.
func TestCurrent_WitnessFallback(t *testing.T) {
	t.Run("same witness dedups by pipeline/job", func(t *testing.T) {
		early := testEvidence(artifact.EvidenceBehavioral, artifact.VerdictFail, "ac-1", withWitness("golden: refund"), withPipeline("1"))
		later := testEvidence(artifact.EvidenceBehavioral, artifact.VerdictPass, "ac-1", withWitness("golden: refund"), withPipeline("2"))

		got := Current([]artifact.Evidence{early, later})
		if len(got) != 1 || got[0].Verdict != artifact.VerdictPass {
			t.Fatalf("Current = %v, want a single deduped pass record", got)
		}
	})

	t.Run("different witness stays distinct", func(t *testing.T) {
		a := testEvidence(artifact.EvidenceBehavioral, artifact.VerdictPass, "ac-1", withWitness("golden: refund"))
		b := testEvidence(artifact.EvidenceBehavioral, artifact.VerdictPass, "ac-1", withWitness("golden: partial-refund"))

		got := Current([]artifact.Evidence{a, b})
		if len(got) != 2 {
			t.Fatalf("Current = %d records, want 2 (distinct witness text kept separately)", len(got))
		}
	})
}

// TestCurrent_EmptyInput is Current's negative/degenerate case: no records
// in, no records out, no panic.
func TestCurrent_EmptyInput(t *testing.T) {
	got := Current(nil)
	if len(got) != 0 {
		t.Fatalf("Current(nil) = %v, want empty", got)
	}
	got = Current([]artifact.Evidence{})
	if len(got) != 0 {
		t.Fatalf("Current([]) = %v, want empty", got)
	}
}
