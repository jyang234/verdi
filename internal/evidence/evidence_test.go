package evidence

import "github.com/OWNER/verdi/internal/artifact"

// hex64 is a 64-hex-character placeholder used to satisfy Evidence's
// sha256:<64 hex> digest shape check across this package's tests — the
// digest's actual value is never load-bearing for fold logic.
const hex64 = "ab12ab12ab12ab12ab12ab12ab12ab12ab12ab12ab12ab12ab12ab12ab12ab1"

// testEvidence builds a synthetic, already-decoded evidence record for
// fold/current unit tests (bypassing DecodeEvidence's strict-decode path,
// which is exercised separately by internal/artifact and by records_test.go's
// on-disk LoadRecords tests).
func testEvidence(kind artifact.EvidenceKind, verdict artifact.EvidenceVerdict, ac string, opts ...func(*artifact.Evidence)) artifact.Evidence {
	e := artifact.Evidence{
		Schema:      "verdi.evidence/v1",
		EvidenceFor: []string{ac},
		Kind:        kind,
		Verdict:     verdict,
		Witness:     "witness",
		Provenance:  artifact.EvidenceProvenance{Source: artifact.SourceCI, Pipeline: "1", Commit: "7f3c2a1"},
		Digest:      "sha256:" + hex64,
	}
	for _, opt := range opts {
		opt(&e)
	}
	return e
}

func withProducer(p string) func(*artifact.Evidence) { return func(e *artifact.Evidence) { e.Producer = p } }
func withWitness(w string) func(*artifact.Evidence)  { return func(e *artifact.Evidence) { e.Witness = w } }
func withPipeline(p string) func(*artifact.Evidence) {
	return func(e *artifact.Evidence) { e.Provenance.Pipeline = p }
}
func withJob(j string) func(*artifact.Evidence) { return func(e *artifact.Evidence) { e.Provenance.Job = j } }
func withCommit(c string) func(*artifact.Evidence) {
	return func(e *artifact.Evidence) { e.Provenance.Commit = c }
}
func withSource(s artifact.ProvenanceSource) func(*artifact.Evidence) {
	return func(e *artifact.Evidence) { e.Provenance.Source = s }
}
