package matrixprojection

import (
	"encoding/json"
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

	requiredFields := []struct {
		name   string
		data   []byte
		object []any
		key    string
	}{
		{name: "envelope schema", data: storyJSON, key: "schema"},
		{name: "envelope target", data: storyJSON, key: "target"},
		{name: "envelope preview false", data: featureJSON, key: "preview"},
		{name: "envelope violated false", data: featureJSON, key: "violated"},
		{name: "target class", data: storyJSON, object: []any{"target"}, key: "class"},
		{name: "target spec ref", data: storyJSON, object: []any{"target"}, key: "spec_ref"},
		{name: "target effective state", data: storyJSON, object: []any{"target"}, key: "effective_state"},
		{name: "story body story ref", data: storyJSON, object: []any{"story"}, key: "story_ref"},
		{name: "story body eligible false", data: storyJSON, object: []any{"story"}, key: "eligible"},
		{name: "story body acs", data: storyJSON, object: []any{"story"}, key: "acs"},
		{name: "story AC id", data: storyJSON, object: []any{"story", "acs", 0}, key: "id"},
		{name: "story AC text", data: storyJSON, object: []any{"story", "acs", 0}, key: "text"},
		{name: "story AC status", data: storyJSON, object: []any{"story", "acs", 0}, key: "status"},
		{name: "story AC summary", data: storyJSON, object: []any{"story", "acs", 0}, key: "summary"},
		{name: "story AC kinds", data: storyJSON, object: []any{"story", "acs", 0}, key: "kinds"},
		{name: "kind kind", data: storyJSON, object: []any{"story", "acs", 0, "kinds", 0}, key: "kind"},
		{name: "kind satisfied false", data: storyJSON, object: []any{"story", "acs", 0, "kinds", 0}, key: "satisfied"},
		{name: "kind attestation state", data: storyJSON, object: []any{"story", "acs", 0, "kinds", 0}, key: "attestation_state"},
		{name: "kind violating witness empty", data: storyJSON, object: []any{"story", "acs", 0, "kinds", 1}, key: "violating_witness"},
		{name: "kind obligation quality", data: storyJSON, object: []any{"story", "acs", 0, "kinds", 0}, key: "obligation_quality"},
		{name: "obligation quality structural state", data: storyJSON, object: []any{"story", "acs", 0, "kinds", 0, "obligation_quality"}, key: "structural_state"},
		{name: "obligation quality match state", data: storyJSON, object: []any{"story", "acs", 0, "kinds", 0, "obligation_quality"}, key: "match_state"},
		{name: "obligation quality reason empty", data: storyJSON, object: []any{"story", "acs", 0, "kinds", 0, "obligation_quality"}, key: "reason"},
		{name: "obligation quality witness path", data: storyJSON, object: []any{"story", "acs", 0, "kinds", 0, "obligation_quality"}, key: "witness_path"},
		{name: "feature body acs", data: featureJSON, object: []any{"feature"}, key: "acs"},
		{name: "feature AC id", data: featureJSON, object: []any{"feature", "acs", 0}, key: "id"},
		{name: "feature AC text", data: featureJSON, object: []any{"feature", "acs", 0}, key: "text"},
		{name: "feature AC status", data: featureJSON, object: []any{"feature", "acs", 0}, key: "status"},
		{name: "feature AC summary", data: featureJSON, object: []any{"feature", "acs", 0}, key: "summary"},
		{name: "feature AC implementing stories", data: featureJSON, object: []any{"feature", "acs", 0}, key: "implementing_stories"},
		{name: "feature AC outcome floor", data: featureJSON, object: []any{"feature", "acs", 0}, key: "outcome_floor"},
		{name: "outcome floor satisfied false", data: featureJSON, object: []any{"feature", "acs", 0, "outcome_floor"}, key: "satisfied"},
		{name: "outcome floor declares attestation false", data: featureJSON, object: []any{"feature", "acs", 1, "outcome_floor"}, key: "declares_attestation"},
		{name: "outcome floor attestation state", data: featureJSON, object: []any{"feature", "acs", 0, "outcome_floor"}, key: "attestation_state"},
		{name: "outcome floor violating witness empty", data: featureJSON, object: []any{"feature", "acs", 0, "outcome_floor"}, key: "violating_witness"},
	}
	for _, tc := range requiredFields {
		t.Run("missing "+tc.name, func(t *testing.T) {
			withoutKey := removeJSONKey(t, tc.data, tc.object, tc.key)
			if _, err := Decode(withoutKey); err == nil {
				t.Fatalf("Decode accepted JSON missing required %s key %q: %s", tc.name, tc.key, withoutKey)
			}
		})
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

func removeJSONKey(t *testing.T, data []byte, objectPath []any, key string) []byte {
	t.Helper()
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("decoding required-field fixture: %v", err)
	}
	current := root
	for _, segment := range objectPath {
		switch value := segment.(type) {
		case string:
			object, ok := current.(map[string]any)
			if !ok {
				t.Fatalf("required-field fixture path %v segment %q is not an object", objectPath, value)
			}
			current, ok = object[value]
			if !ok {
				t.Fatalf("required-field fixture path %v is missing segment %q", objectPath, value)
			}
		case int:
			array, ok := current.([]any)
			if !ok || value < 0 || value >= len(array) {
				t.Fatalf("required-field fixture path %v index %d is invalid", objectPath, value)
			}
			current = array[value]
		default:
			t.Fatalf("required-field fixture path %v has unsupported segment %#v", objectPath, segment)
		}
	}
	object, ok := current.(map[string]any)
	if !ok {
		t.Fatalf("required-field fixture path %v is not an object", objectPath)
	}
	if _, ok := object[key]; !ok {
		t.Fatalf("required-field fixture path %v has no key %q", objectPath, key)
	}
	delete(object, key)
	out, err := json.Marshal(root)
	if err != nil {
		t.Fatalf("encoding required-field fixture: %v", err)
	}
	return out
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
