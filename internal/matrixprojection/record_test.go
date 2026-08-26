package matrixprojection

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/evidence"
)

func TestMatrixProjectionContract_Static(t *testing.T) {
	t.Parallel()

	failing := &artifact.Evidence{Witness: "ci://verify/static/ac-2"}
	storyResult := evidence.StoryResult{
		Story:   "jira:VATC-1",
		SpecRef: "spec/vatc-story",
		ACs: []evidence.ACResult{
			{
				ID:      "ac-2",
				Text:    "second in declaration order",
				Status:  evidence.StatusViolated,
				Summary: "static:fail; attestation:absent",
				Kinds: []evidence.KindResult{
					{
						Kind:      artifact.EvidenceStatic,
						Satisfied: false,
						Violating: failing,
						ObligationQuality: evidence.ObligationQualityProjection{
							StructuralState: evidence.ObligationElaborated,
							MatchState:      evidence.ObligationViolatedWithWitness,
							WitnessPath:     ".verdi/obligations/vatc-story/ac-2--static.md",
						},
					},
					{
						Kind:        artifact.EvidenceAttestation,
						Satisfied:   false,
						Attestation: evidence.AttestationAbsent,
						ObligationQuality: evidence.ObligationQualityProjection{
							StructuralState: evidence.ObligationMissing,
							MatchState:      evidence.ObligationUnproven,
							WitnessPath:     ".verdi/obligations/vatc-story/ac-2--attestation.md",
						},
					},
				},
			},
			{ID: "ac-1", Text: "first id but second declaration row", Status: evidence.StatusNoSignal, Summary: "runtime:awaited", Kinds: []evidence.KindResult{}},
		},
		Violated: true,
		Eligible: false,
	}

	story, err := NewStory(Target{Class: ClassStory, SpecRef: storyResult.SpecRef, EffectiveState: StateAcceptedPendingBuild}, true, storyResult)
	if err != nil {
		t.Fatalf("NewStory: %v", err)
	}
	storyJSON, err := Marshal(story)
	if err != nil {
		t.Fatalf("Marshal(story): %v", err)
	}
	wantStory := `{"preview":true,"schema":"verdi.matrix/v1","story":{"acs":[{"id":"ac-2","kinds":[{"attestation_state":"not-applicable","kind":"static","obligation_quality":{"match_state":"violated-with-witness","reason":"","structural_state":"elaborated","witness_path":".verdi/obligations/vatc-story/ac-2--static.md"},"satisfied":false,"violating_witness":"ci://verify/static/ac-2"},{"attestation_state":"absent","kind":"attestation","obligation_quality":{"match_state":"unproven","reason":"","structural_state":"missing","witness_path":".verdi/obligations/vatc-story/ac-2--attestation.md"},"satisfied":false,"violating_witness":""}],"status":"violated","summary":"static:fail; attestation:absent","text":"second in declaration order"},{"id":"ac-1","kinds":[],"status":"no-signal","summary":"runtime:awaited","text":"first id but second declaration row"}],"eligible":false,"story_ref":"jira:VATC-1"},"target":{"class":"story","effective_state":"accepted-pending-build","spec_ref":"spec/vatc-story"},"violated":true}` + "\n"
	if string(storyJSON) != wantStory {
		t.Fatalf("story canonical JSON mismatch:\n got: %s\nwant: %s", storyJSON, wantStory)
	}
	decodedStory, err := Decode(storyJSON)
	if err != nil {
		t.Fatalf("Decode(story): %v", err)
	}
	if decodedStory.Story == nil || decodedStory.Feature != nil {
		t.Fatalf("decoded story arms = story:%#v feature:%#v, want exactly story", decodedStory.Story, decodedStory.Feature)
	}

	featureResult := evidence.FeatureResult{
		SpecRef: "spec/vatc-feature",
		ACs: []evidence.FeatureACResult{
			{
				ID:                  "ac-3",
				Text:                "feature outcome",
				Status:              evidence.StatusPending,
				Summary:             "attestation:unauthored",
				ImplementingStories: []string{"spec/a", "spec/z"},
				Floor: evidence.FloorResult{
					Satisfied:           false,
					DeclaresAttestation: true,
					Attestation:         evidence.AttestationUnauthored,
				},
			},
			{
				ID:                  "ac-1",
				Text:                "empty implementers remain an array",
				Status:              evidence.StatusNoSignal,
				Summary:             "attestation:absent",
				ImplementingStories: nil,
				Floor:               evidence.FloorResult{},
			},
		},
	}
	feature, err := NewFeature(Target{Class: ClassFeature, SpecRef: featureResult.SpecRef, EffectiveState: StateProposed}, false, featureResult)
	if err != nil {
		t.Fatalf("NewFeature: %v", err)
	}
	featureJSON, err := Marshal(feature)
	if err != nil {
		t.Fatalf("Marshal(feature): %v", err)
	}
	wantFeature := `{"feature":{"acs":[{"id":"ac-3","implementing_stories":["spec/a","spec/z"],"outcome_floor":{"attestation_state":"unauthored","declares_attestation":true,"satisfied":false,"violating_witness":""},"status":"pending","summary":"attestation:unauthored","text":"feature outcome"},{"id":"ac-1","implementing_stories":[],"outcome_floor":{"attestation_state":"not-applicable","declares_attestation":false,"satisfied":false,"violating_witness":""},"status":"no-signal","summary":"attestation:absent","text":"empty implementers remain an array"}]},"preview":false,"schema":"verdi.matrix/v1","target":{"class":"feature","effective_state":"proposed","spec_ref":"spec/vatc-feature"},"violated":false}` + "\n"
	if string(featureJSON) != wantFeature {
		t.Fatalf("feature canonical JSON mismatch:\n got: %s\nwant: %s", featureJSON, wantFeature)
	}
	decodedFeature, err := Decode(featureJSON)
	if err != nil {
		t.Fatalf("Decode(feature): %v", err)
	}
	if decodedFeature.Feature == nil || decodedFeature.Story != nil {
		t.Fatalf("decoded feature arms = story:%#v feature:%#v, want exactly feature", decodedFeature.Story, decodedFeature.Feature)
	}

	invalid := []struct {
		name string
		json string
	}{
		{"unknown field", strings.Replace(wantStory, `"preview":true`, `"extra":1,"preview":true`, 1)},
		{"trailing data", wantStory + `{}`},
		{"unknown class", strings.Replace(wantStory, `"class":"story"`, `"class":"component"`, 1)},
		{"unknown effective state", strings.Replace(wantStory, `"effective_state":"accepted-pending-build"`, `"effective_state":"mystery"`, 1)},
		{"unknown status", strings.Replace(wantStory, `"status":"violated"`, `"status":"mystery"`, 1)},
		{"unknown kind", strings.Replace(wantStory, `"kind":"static"`, `"kind":"mystery"`, 1)},
		{"unknown attestation state", strings.Replace(wantStory, `"attestation_state":"absent"`, `"attestation_state":"mystery"`, 1)},
		{"unknown structural state", strings.Replace(wantStory, `"structural_state":"elaborated"`, `"structural_state":"mystery"`, 1)},
		{"unknown match state", strings.Replace(wantStory, `"match_state":"violated-with-witness"`, `"match_state":"mystery"`, 1)},
		{"unknown reason", strings.Replace(wantStory, `"reason":""`, `"reason":"mystery"`, 1)},
		{"discriminator mismatch", strings.Replace(wantStory, `"class":"story"`, `"class":"feature"`, 1)},
		{"both arms", strings.Replace(wantStory, `"preview":true`, `"feature":{"acs":[]},"preview":true`, 1)},
		{"neither arm", `{"preview":false,"schema":"verdi.matrix/v1","target":{"class":"story","effective_state":"accepted-pending-build","spec_ref":"spec/x"},"violated":false}`},
		{"null story arm", `{"preview":false,"schema":"verdi.matrix/v1","story":null,"target":{"class":"story","effective_state":"accepted-pending-build","spec_ref":"spec/x"},"violated":false}`},
		{"null feature arm", `{"feature":null,"preview":false,"schema":"verdi.matrix/v1","target":{"class":"feature","effective_state":"accepted-pending-build","spec_ref":"spec/x"},"violated":false}`},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Decode([]byte(tc.json)); err == nil {
				t.Fatalf("Decode accepted invalid %s JSON: %s", tc.name, tc.json)
			}
		})
	}
}

func TestConstructorsRejectMismatchedTargets(t *testing.T) {
	t.Parallel()

	if _, err := NewStory(Target{Class: ClassFeature, SpecRef: "spec/x", EffectiveState: StateProposed}, false, evidence.StoryResult{Story: "jira:X-1", SpecRef: "spec/x", ACs: []evidence.ACResult{}}); err == nil {
		t.Fatal("NewStory accepted feature target")
	}
	if _, err := NewFeature(Target{Class: ClassStory, SpecRef: "spec/x", EffectiveState: StateProposed}, false, evidence.FeatureResult{SpecRef: "spec/x", ACs: []evidence.FeatureACResult{}}); err == nil {
		t.Fatal("NewFeature accepted story target")
	}
}

func TestMarshalRejectsInvalidRecord(t *testing.T) {
	t.Parallel()

	if _, err := Marshal(Record{}); err == nil {
		t.Fatal("Marshal accepted invalid zero record")
	}
}

func TestProjectAttestationState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		in      evidence.AttestationState
		want    AttestationState
		wantErr bool
	}{
		{name: "absent", in: evidence.AttestationAbsent, want: AttestationAbsent},
		{name: "unauthored", in: evidence.AttestationUnauthored, want: AttestationUnauthored},
		{name: "authored", in: evidence.AttestationAuthored, want: AttestationAuthored},
		{name: "unknown", in: evidence.AttestationState(99), wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := projectAttestationState(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("projectAttestationState(%v) error = %v, wantErr %t", tc.in, err, tc.wantErr)
			}
			if got != tc.want {
				t.Fatalf("projectAttestationState(%v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
