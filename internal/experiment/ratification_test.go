package experiment

import (
	"strings"
	"testing"
)

// validActor is a canonical governanceprincipal.PrincipalID literal
// (governanceprincipal's own TestPrincipalIDValidate fixture): "github" ⇒
// trust source, decoding to subject "user-123".
const validActor = "principal/github/dXNlci0xMjM"

func validRatificationYAML() string {
	return "schema: verdi.experiment-ratification/v1\n" +
		"result_digest: " + digestOf("a") + "\n" +
		"actor: " + validActor + "\n" +
		"disposition: select-recommended\n"
}

func mutateRatification(t *testing.T, old, replacement string) string {
	t.Helper()
	doc := validRatificationYAML()
	if !strings.Contains(doc, old) {
		t.Fatalf("fixture does not contain %q", old)
	}
	return strings.Replace(doc, old, replacement, 1)
}

func TestDecodeRatificationHappyPath(t *testing.T) {
	r, err := DecodeRatification([]byte(validRatificationYAML()))
	if err != nil {
		t.Fatalf("DecodeRatification() unexpected error: %v", err)
	}
	if r.Disposition != DispositionSelectRecommended {
		t.Errorf("r.Disposition = %q, want %q", r.Disposition, DispositionSelectRecommended)
	}
}

func TestDecodeRatificationSelectOtherHappyPath(t *testing.T) {
	doc := "schema: verdi.experiment-ratification/v1\n" +
		"result_digest: " + digestOf("a") + "\n" +
		"actor: " + validActor + "\n" +
		"disposition: select-other\n" +
		"candidate: baseline\n" +
		"reason: lower operational risk than the recommended candidate\n"
	r, err := DecodeRatification([]byte(doc))
	if err != nil {
		t.Fatalf("DecodeRatification() unexpected error: %v", err)
	}
	if r.Candidate != "baseline" {
		t.Errorf("r.Candidate = %q, want baseline", r.Candidate)
	}
}

// bindingRatification returns a grammar-valid Ratification with the given
// disposition and candidate — the record ValidateRatificationBinding then
// judges against a definition and result.
func bindingRatification(disposition Disposition, candidate string) Ratification {
	r := Ratification{
		Schema:       RatificationSchema,
		ResultDigest: digestOf("a"),
		Actor:        validActor,
		Disposition:  disposition,
	}
	if candidate != "" {
		r.Candidate = candidate
		r.Reason = "lower operational risk than the recommended candidate"
	}
	return r
}

// bindingResult returns a Result carrying just the two fields
// ValidateRatificationBinding reads: the verdict and the winner.
func bindingResult(verdict Verdict, winner string) Result {
	return Result{Verdict: verdict, Winner: winner}
}

// TestValidateRatificationBinding is SI-45's table: AC-5's disposition
// list carries semantic preconditions its grammar cannot express, and the
// binding check is where they hold. Grammar validity is assumed by none of
// these cases — every record here is grammar-valid, and only the
// def/result binding separates the accepted ones from the rejected.
func TestValidateRatificationBinding(t *testing.T) {
	def := mustDecodeDefinition(t, validDefinitionYAML()) // candidates: baseline, facts-cache

	tests := []struct {
		name    string
		res     Result
		r       Ratification
		wantErr bool
	}{
		{
			name: "select-recommended over a proven winner",
			res:  bindingResult(VerdictProvenWinner, "facts-cache"),
			r:    bindingRatification(DispositionSelectRecommended, ""),
		},
		{
			name:    "select-recommended over an unproven result",
			res:     bindingResult(VerdictDisclosedUnproven, ""),
			r:       bindingRatification(DispositionSelectRecommended, ""),
			wantErr: true,
		},
		{
			name:    "select-recommended over a violated result",
			res:     bindingResult(VerdictViolatedWithWitness, ""),
			r:       bindingRatification(DispositionSelectRecommended, ""),
			wantErr: true,
		},
		{
			name: "select-other naming a different registered candidate",
			res:  bindingResult(VerdictProvenWinner, "facts-cache"),
			r:    bindingRatification(DispositionSelectOther, "baseline"),
		},
		{
			name: "select-other against a result with no winner",
			res:  bindingResult(VerdictDisclosedUnproven, ""),
			r:    bindingRatification(DispositionSelectOther, "facts-cache"),
		},
		{
			name:    "select-other naming the recommended winner",
			res:     bindingResult(VerdictProvenWinner, "facts-cache"),
			r:       bindingRatification(DispositionSelectOther, "facts-cache"),
			wantErr: true,
		},
		{
			name:    "select-other naming an unregistered candidate",
			res:     bindingResult(VerdictProvenWinner, "facts-cache"),
			r:       bindingRatification(DispositionSelectOther, "nonexistent"),
			wantErr: true,
		},
		{
			name: "reject-all over a proven winner",
			res:  bindingResult(VerdictProvenWinner, "facts-cache"),
			r:    bindingRatification(DispositionRejectAll, ""),
		},
		{
			name: "misframed over an unproven result",
			res:  bindingResult(VerdictDisclosedUnproven, ""),
			r:    bindingRatification(DispositionMisframed, ""),
		},
		{
			name: "request-new-revision over a violated result",
			res:  bindingResult(VerdictViolatedWithWitness, ""),
			r:    bindingRatification(DispositionRequestNewRevision, ""),
		},
		{
			name:    "grammar-invalid record",
			res:     bindingResult(VerdictProvenWinner, "facts-cache"),
			r:       Ratification{Schema: RatificationSchema},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRatificationBinding(def, tt.res, tt.r)
			if tt.wantErr && err == nil {
				t.Fatalf("ValidateRatificationBinding(%s) = nil error, want error", tt.name)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("ValidateRatificationBinding(%s) unexpected error: %v", tt.name, err)
			}
			if err != nil && !strings.HasPrefix(err.Error(), "experiment: ") {
				t.Errorf("ValidateRatificationBinding() error = %q, want the %q prefix", err.Error(), "experiment: ")
			}
		})
	}
}

// TestRatificationValidateStaysGrammarScoped pins the split SI-45 chose:
// the record's own Validate never consults a definition or result, so a
// disposition that is semantically impossible against its bound result
// still DECODES — it is the binding check, wherever a ratification meets
// its context, that refuses it.
func TestRatificationValidateStaysGrammarScoped(t *testing.T) {
	r := bindingRatification(DispositionSelectRecommended, "")
	if err := r.Validate(); err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}
	def := mustDecodeDefinition(t, validDefinitionYAML())
	if err := ValidateRatificationBinding(def, bindingResult(VerdictDisclosedUnproven, ""), r); err == nil {
		t.Errorf("ValidateRatificationBinding() on a select-recommended against an unproven result = nil error, want error")
	}
}

func TestDecodeRatificationRejects(t *testing.T) {
	selectOtherDoc := "schema: verdi.experiment-ratification/v1\n" +
		"result_digest: " + digestOf("a") + "\n" +
		"actor: " + validActor + "\n" +
		"disposition: select-other\n"

	tests := []struct {
		name string
		doc  string
	}{
		{"unknown schema", mutateRatification(t, "schema: verdi.experiment-ratification/v1", "schema: verdi.experiment-ratification/v3")},
		// v2 is a known schema now, but its actor is a block: a v1-shaped
		// scalar actor under the v2 schema is malformed, so v1 actor text
		// can never ride into a v2 record.
		{"v2 schema with v1 scalar actor", mutateRatification(t, "schema: verdi.experiment-ratification/v1", "schema: verdi.experiment-ratification/v2")},
		{"unknown field", validRatificationYAML() + "unknown_field: true\n"},
		// A bare trailing scalar the parser cannot place; a second "---"
		// document is covered by strictdecode_test.go's trailing-document
		// probes.
		{"trailing data", validRatificationYAML() + "trailing-garbage-not-a-key\n"},
		{"yaml anchor", mutateRatification(t, "actor: "+validActor, "actor: &a "+validActor)},
		{"yaml alias", validRatificationYAML() + "alias_ref: *nonexistent\n"},
		{"custom tag", mutateRatification(t, "disposition: select-recommended", "disposition: !custom select-recommended")},
		{"bad result digest", mutateRatification(t, "result_digest: "+digestOf("a"), "result_digest: not-a-digest")},
		{"unknown disposition", mutateRatification(t, "disposition: select-recommended", "disposition: select-everyone")},
		{"bare name actor", mutateRatification(t, "actor: "+validActor, "actor: alice")},
		{"unauthenticated marker actor", mutateRatification(t, "actor: "+validActor, "actor: unauthenticated")},
		{"empty actor", mutateRatification(t, "actor: "+validActor, "actor: \"\"")},
		{"malformed principal actor", mutateRatification(t, "actor: "+validActor, "actor: principal/GitHub/dXNlci0xMjM")},
		{"select-other missing candidate", selectOtherDoc + "reason: because\n"},
		{"select-other missing reason", selectOtherDoc + "candidate: baseline\n"},
		// A PRESENT reason must carry content. select-other is the one
		// disposition where the rule has observable force: an explicitly
		// empty reason and an absent one both decode to "", and for every
		// other disposition an absent reason is legitimate.
		{"select-other empty reason", selectOtherDoc + "candidate: baseline\nreason: \"\"\n"},
		{"candidate present on non-select-other", validRatificationYAML() + "candidate: baseline\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := DecodeRatification([]byte(tt.doc)); err == nil {
				t.Errorf("DecodeRatification(%s) = nil error, want error", tt.name)
			}
		})
	}
}

// validRatificationActorBlock is the exact v2 actor block whose principal
// id is the kernel derivation of its own claim (github/user-123).
func validRatificationActorBlock() string {
	return "actor:\n" +
		"  trust_source: github\n" +
		"  subject: \"user-123\"\n" +
		"  principal_id: " + validActor + "\n"
}

func validRatificationV2YAML() string {
	return "schema: verdi.experiment-ratification/v2\n" +
		"result_digest: " + digestOf("a") + "\n" +
		validRatificationActorBlock() +
		"disposition: select-recommended\n"
}

func TestDecodeRatificationV2HappyPath(t *testing.T) {
	r, err := DecodeRatification([]byte(validRatificationV2YAML()))
	if err != nil {
		t.Fatalf("DecodeRatification(v2) unexpected error: %v", err)
	}
	if r.Schema != RatificationSchemaV2 || r.ActorV2 == nil {
		t.Fatalf("r = %+v, want v2 record with actor block", r)
	}
	if r.ActorV2.TrustSource != "github" || r.ActorV2.Subject != "user-123" || r.ActorV2.PrincipalID != validActor {
		t.Fatalf("r.ActorV2 = %+v", r.ActorV2)
	}
	if r.Actor != "" {
		t.Fatalf("r.Actor = %q, want empty v1 field on a v2 record", r.Actor)
	}
}

func TestDecodeRatificationV2Rejects(t *testing.T) {
	mutate := func(t *testing.T, old, replacement string) string {
		t.Helper()
		doc := validRatificationV2YAML()
		if !strings.Contains(doc, old) {
			t.Fatalf("v2 fixture does not contain %q", old)
		}
		return strings.Replace(doc, old, replacement, 1)
	}
	tests := []struct {
		name string
		doc  string
	}{
		{"null actor", mutate(t, validRatificationActorBlock(), "actor: null\n")},
		{"empty actor block", mutate(t, validRatificationActorBlock(), "actor: {}\n")},
		{"sequence actor", mutate(t, validRatificationActorBlock(), "actor: [github]\n")},
		{"unknown actor field", mutate(t, "  principal_id: "+validActor+"\n", "  principal_id: "+validActor+"\n  role: admin\n")},
		{"missing trust_source", mutate(t, "  trust_source: github\n", "")},
		{"missing subject", mutate(t, "  subject: \"user-123\"\n", "")},
		{"missing principal_id", mutate(t, "  principal_id: "+validActor+"\n", "")},
		{"duplicate actor field", mutate(t, "  subject: \"user-123\"\n", "  subject: \"user-123\"\n  subject: \"user-456\"\n")},
		{"null actor field", mutate(t, "  subject: \"user-123\"\n", "  subject: null\n")},
		{"non-string actor field", mutate(t, "  subject: \"user-123\"\n", "  subject: [user-123]\n")},
		{"bad trust source grammar", mutate(t, "  trust_source: github\n", "  trust_source: GitHub\n")},
		{"empty subject", mutate(t, "  subject: \"user-123\"\n", "  subject: \"\"\n")},
		{"malformed principal id", mutate(t, "  principal_id: "+validActor+"\n", "  principal_id: alice\n")},
		{"principal id not derived from claim", mutate(t, "  subject: \"user-123\"\n", "  subject: \"user-456\"\n")},
		{"v1 schema with v2 actor block", mutate(t, "schema: verdi.experiment-ratification/v2", "schema: verdi.experiment-ratification/v1")},
		{"actor anchor", mutate(t, "  trust_source: github\n", "  trust_source: &a github\n")},
		{"unknown top-level field", validRatificationV2YAML() + "unknown_field: true\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := DecodeRatification([]byte(tt.doc)); err == nil {
				t.Errorf("DecodeRatification(%s) = nil error, want error", tt.name)
			}
		})
	}
}

func TestEncodeRatificationEmitsV2Only(t *testing.T) {
	v1 := bindingRatification(DispositionSelectRecommended, "")
	if _, err := EncodeRatification(v1); err == nil {
		t.Fatalf("EncodeRatification(v1) = nil error, want v1 decode-only refusal")
	}

	record := Ratification{
		Schema: RatificationSchemaV2, ResultDigest: digestOf("a"),
		ActorV2:     &RatificationActor{TrustSource: "github", Subject: "user-123", PrincipalID: validActor},
		Disposition: DispositionSelectOther, Candidate: "baseline",
		Reason: "operational risk: \"quoted\" text\nand a newline",
	}
	first, err := EncodeRatification(record)
	if err != nil {
		t.Fatalf("EncodeRatification(v2) error: %v", err)
	}
	second, err := EncodeRatification(record)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("EncodeRatification is not deterministic:\n%q\n%q", first, second)
	}
	decoded, err := DecodeRatification(first)
	if err != nil {
		t.Fatalf("emitted v2 bytes do not strict-decode: %v\n%s", err, first)
	}
	if decoded.Schema != RatificationSchemaV2 || decoded.ActorV2 == nil ||
		*decoded.ActorV2 != *record.ActorV2 ||
		decoded.ResultDigest != record.ResultDigest ||
		decoded.Disposition != record.Disposition ||
		decoded.Candidate != record.Candidate || decoded.Reason != record.Reason {
		t.Fatalf("round trip changed the record:\nin:  %+v\nout: %+v", record, decoded)
	}
	// Defensive copy: the emitted bytes and decoded record share no actor
	// storage with the input.
	record.ActorV2.PrincipalID = "principal/github/mutated"
	redecoded, err := DecodeRatification(first)
	if err != nil {
		t.Fatal(err)
	}
	if redecoded.ActorV2.PrincipalID != validActor {
		t.Fatalf("emitted bytes aliased the caller's actor block")
	}
}
