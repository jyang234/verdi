package bundle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/upstream"
)

const svcfixDir = "../../testdata/svcfix"
const cannedDir = "../../testdata/svcfix-canned"

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return data
}

func loadSvcfixGraph(t *testing.T) *upstream.Graph {
	t.Helper()
	g, err := upstream.DecodeGraph(readFile(t, filepath.Join(cannedDir, "graph.json")))
	if err != nil {
		t.Fatalf("DecodeGraph: %v", err)
	}
	return g
}

func loadSvcfixBindings(t *testing.T) *artifact.Bindings {
	t.Helper()
	b, err := artifact.DecodeBindings(readFile(t, filepath.Join(svcfixDir, "verdi.bindings.yaml")))
	if err != nil {
		t.Fatalf("DecodeBindings: %v", err)
	}
	return b
}

// specACs mirrors spec/stale-decline's real acceptance_criteria ids
// (testdata/corpus/.verdi/specs/active/stale-decline/spec.md), the spec
// testdata/svcfix's verdi.bindings.yaml binds to.
func specACs() map[string]bool {
	return map[string]bool{"ac-1": true, "ac-2": true, "ac-3": true, "ac-4": true}
}

func passingTestSummary() *TestSummary {
	return &TestSummary{
		Schema:   testsSchema,
		Suite:    "pass",
		Packages: []PackageResult{{Package: "example.com/svcfix/internal/app", Status: "pass", Tests: 2}},
	}
}

// TestBuildVerdicts_Happy joins svcfix's real bindings against its real
// graph: audit-before-publish (static, SATISFIED) -> ac-1/ac-2, and
// refund-flow (behavioral) -> ac-3.
func TestBuildVerdicts_Happy(t *testing.T) {
	in := JoinInput{
		ServiceName:      "svcfix",
		Graph:            loadSvcfixGraph(t),
		Bindings:         loadSvcfixBindings(t),
		KnownGoldenFlows: map[string]bool{"refund-flow": true},
		SpecACs:          specACs(),
		TestSummary:      passingTestSummary(),
		Provenance:       artifact.EvidenceProvenance{Source: artifact.SourceLocal, Commit: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"},
	}
	recs, err := BuildVerdicts(in)
	if err != nil {
		t.Fatalf("BuildVerdicts: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("BuildVerdicts = %d records, want 2 (one static, one behavioral)", len(recs))
	}

	static := recs[0]
	if static.Kind != artifact.EvidenceStatic || static.Verdict != artifact.VerdictPass {
		t.Errorf("static record = %+v, want kind static, verdict pass", static)
	}
	if len(static.EvidenceFor) != 2 || static.EvidenceFor[0] != "ac-1" || static.EvidenceFor[1] != "ac-2" {
		t.Errorf("static record EvidenceFor = %v, want [ac-1 ac-2]", static.EvidenceFor)
	}
	if err := static.Validate(); err != nil {
		t.Errorf("static record fails artifact.Evidence.Validate: %v", err)
	}

	behavioral := recs[1]
	if behavioral.Kind != artifact.EvidenceBehavioral || behavioral.Verdict != artifact.VerdictPass {
		t.Errorf("behavioral record = %+v, want kind behavioral, verdict pass", behavioral)
	}
	if len(behavioral.EvidenceFor) != 1 || behavioral.EvidenceFor[0] != "ac-3" {
		t.Errorf("behavioral record EvidenceFor = %v, want [ac-3]", behavioral.EvidenceFor)
	}
	if err := behavioral.Validate(); err != nil {
		t.Errorf("behavioral record fails artifact.Evidence.Validate: %v", err)
	}
}

func TestBuildVerdicts_NilBindings(t *testing.T) {
	recs, err := BuildVerdicts(JoinInput{ServiceName: "svcfix"})
	if err != nil {
		t.Fatalf("BuildVerdicts(nil bindings): %v", err)
	}
	if recs != nil {
		t.Fatalf("BuildVerdicts(nil bindings) = %v, want nil", recs)
	}
}

func TestBuildVerdicts_BehavioralSuiteFail(t *testing.T) {
	in := JoinInput{
		ServiceName:      "svcfix",
		Graph:            loadSvcfixGraph(t),
		Bindings:         loadSvcfixBindings(t),
		KnownGoldenFlows: map[string]bool{"refund-flow": true},
		SpecACs:          specACs(),
		TestSummary:      &TestSummary{Schema: testsSchema, Suite: "fail"},
		Provenance:       artifact.EvidenceProvenance{Source: artifact.SourceCI, Commit: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"},
	}
	recs, err := BuildVerdicts(in)
	if err != nil {
		t.Fatalf("BuildVerdicts: %v", err)
	}
	var found bool
	for _, r := range recs {
		if r.Kind == artifact.EvidenceBehavioral {
			found = true
			if r.Verdict != artifact.VerdictFail {
				t.Errorf("behavioral verdict = %q, want fail when the suite failed", r.Verdict)
			}
		}
	}
	if !found {
		t.Fatal("no behavioral record produced")
	}
}

// TestBuildVerdicts_DanglingAC proves a binding naming an AC the spec does
// not declare is a loud error, not an empty cell (03 §Declarations).
func TestBuildVerdicts_DanglingAC(t *testing.T) {
	bindings := loadSvcfixBindings(t)
	// Corrupt one binding's AC to a plausible misspelling not in specACs().
	bindings.Bindings[0].ACs = []string{"ac-99"}

	in := JoinInput{
		ServiceName: "svcfix",
		Graph:       loadSvcfixGraph(t),
		Bindings:    bindings,
		SpecACs:     specACs(),
		TestSummary: passingTestSummary(),
		Provenance:  artifact.EvidenceProvenance{Source: artifact.SourceLocal, Commit: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"},
	}
	_, err := BuildVerdicts(in)
	if err == nil {
		t.Fatal("BuildVerdicts with a dangling AC binding: want error, got nil")
	}
	if !strings.Contains(err.Error(), "ac-99") {
		t.Errorf("error = %v, want it to name the dangling ac-99", err)
	}
}

// TestBuildVerdicts_UnknownStaticProducer proves a static binding whose
// producer matches no graph obligation is a loud error (dangling binding:
// unknown producer), e.g. a misspelled obligation name.
func TestBuildVerdicts_UnknownStaticProducer(t *testing.T) {
	bindings := loadSvcfixBindings(t)
	bindings.Bindings[0].Producer = "audit-before-publish-typo"

	in := JoinInput{
		ServiceName: "svcfix",
		Graph:       loadSvcfixGraph(t),
		Bindings:    bindings,
		SpecACs:     specACs(),
		TestSummary: passingTestSummary(),
		Provenance:  artifact.EvidenceProvenance{Source: artifact.SourceLocal, Commit: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"},
	}
	_, err := BuildVerdicts(in)
	if err == nil {
		t.Fatal("BuildVerdicts with an unknown static producer: want error, got nil")
	}
	if !strings.Contains(err.Error(), "audit-before-publish-typo") {
		t.Errorf("error = %v, want it to name the unknown producer", err)
	}
}

// TestBuildVerdicts_UnknownBehavioralProducer proves a behavioral binding
// naming a golden flow that does not exist on disk is a loud error.
func TestBuildVerdicts_UnknownBehavioralProducer(t *testing.T) {
	bindings := loadSvcfixBindings(t)
	bindings.Bindings[1].Producer = "refund-flow-that-does-not-exist"

	in := JoinInput{
		ServiceName:      "svcfix",
		Graph:            loadSvcfixGraph(t),
		Bindings:         bindings,
		KnownGoldenFlows: map[string]bool{"refund-flow": true},
		SpecACs:          specACs(),
		TestSummary:      passingTestSummary(),
		Provenance:       artifact.EvidenceProvenance{Source: artifact.SourceLocal, Commit: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"},
	}
	_, err := BuildVerdicts(in)
	if err == nil {
		t.Fatal("BuildVerdicts with an unknown behavioral producer: want error, got nil")
	}
}

// TestBuildVerdicts_UnmatchedObligation proves an UNMATCHED graph
// obligation is a hard error when bound, never a silent abstain — the
// exact JSON shape spike S1 captured for svcfix's own UNMATCHED case (see
// testdata/svcfix-canned/README.md).
func TestBuildVerdicts_UnmatchedObligation(t *testing.T) {
	const unmatchedGraphJSON = `{"obligations":[{"rule":"audit-before-publish","kind":"must-precede","status":"UNMATCHED","detail":"anchor example.com/svcfix/internal/bus#Publish matches no call site — the rule is inert"}]}`
	g, err := upstream.DecodeGraph([]byte(unmatchedGraphJSON))
	if err != nil {
		t.Fatalf("DecodeGraph: %v", err)
	}

	in := JoinInput{
		ServiceName: "svcfix",
		Graph:       g,
		Bindings:    loadSvcfixBindings(t),
		SpecACs:     specACs(),
		TestSummary: passingTestSummary(),
		Provenance:  artifact.EvidenceProvenance{Source: artifact.SourceLocal, Commit: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"},
	}
	_, err = BuildVerdicts(in)
	if err == nil {
		t.Fatal("BuildVerdicts with an UNMATCHED obligation: want error, got nil")
	}
	if !strings.Contains(err.Error(), "UNMATCHED") {
		t.Errorf("error = %v, want it to mention UNMATCHED", err)
	}
}

func TestBuildVerdicts_ViolatedIsFail(t *testing.T) {
	const violatedGraphJSON = `{"obligations":[{"rule":"audit-before-publish","kind":"must-precede","fn":"(*example.com/svcfix/internal/app.Service).PublishRefund","site":"internal/app/app.go:33","status":"VIOLATED","detail":"no call to example.com/svcfix/internal/audit#Write dominates this call to example.com/svcfix/internal/bus#Publish"}]}`
	g, err := upstream.DecodeGraph([]byte(violatedGraphJSON))
	if err != nil {
		t.Fatalf("DecodeGraph: %v", err)
	}

	in := JoinInput{
		ServiceName:      "svcfix",
		Graph:            g,
		Bindings:         loadSvcfixBindings(t),
		KnownGoldenFlows: map[string]bool{"refund-flow": true},
		SpecACs:          specACs(),
		TestSummary:      passingTestSummary(),
		Provenance:       artifact.EvidenceProvenance{Source: artifact.SourceLocal, Commit: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"},
	}
	recs, err := BuildVerdicts(in)
	if err != nil {
		t.Fatalf("BuildVerdicts: %v", err)
	}
	if recs[0].Verdict != artifact.VerdictFail {
		t.Errorf("verdict = %q, want fail for a VIOLATED obligation", recs[0].Verdict)
	}
}

func TestBuildVerdicts_CantProveIsAbstain(t *testing.T) {
	const cantProveGraphJSON = `{"obligations":[{"rule":"audit-before-publish","kind":"must-release","fn":"example.com/svcfix/internal/app.Foo","site":"internal/app/app.go:1","status":"CANT-PROVE","detail":"acquired value is returned"}]}`
	g, err := upstream.DecodeGraph([]byte(cantProveGraphJSON))
	if err != nil {
		t.Fatalf("DecodeGraph: %v", err)
	}

	in := JoinInput{
		ServiceName:      "svcfix",
		Graph:            g,
		Bindings:         loadSvcfixBindings(t),
		KnownGoldenFlows: map[string]bool{"refund-flow": true},
		SpecACs:          specACs(),
		TestSummary:      passingTestSummary(),
		Provenance:       artifact.EvidenceProvenance{Source: artifact.SourceLocal, Commit: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"},
	}
	recs, err := BuildVerdicts(in)
	if err != nil {
		t.Fatalf("BuildVerdicts: %v", err)
	}
	if recs[0].Verdict != artifact.VerdictAbstain {
		t.Errorf("verdict = %q, want abstain for a CANT-PROVE obligation", recs[0].Verdict)
	}
}
