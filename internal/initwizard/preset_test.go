package initwizard

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/model"
)

func TestPlainPreset_ValidatesAgainstTheCanonicalModelAndIsFresh(t *testing.T) {
	p := PlainPreset()
	if p.Classes["story"] != "planned story" || p.Classes["spike"] != "research spike" || len(p.Classes) != 2 || len(p.States) != 0 || len(p.Verbs) != 0 {
		t.Fatalf("preset = %+v", p)
	}
	if _, err := model.DecodeModel(RenderModelYAML(p)); err != nil {
		t.Fatalf("preset model.yaml does not decode: %v", err)
	}
	p.Classes["story"] = "mutated"
	if PlainPreset().Classes["story"] != "planned story" {
		t.Fatal("PlainPreset shares state between calls")
	}
	// Display chain: the renamed words flow through the model.
	m := CandidateModel(PlainPreset())
	if m.DisplayClass("story") != "planned story" || m.DisplayClassPlural("spike") != "research spikes" || m.DisplayClass("feature") != "feature" {
		t.Fatalf("display: %s %s %s", m.DisplayClass("story"), m.DisplayClassPlural("spike"), m.DisplayClass("feature"))
	}
}

func TestParseVocabularyPreset(t *testing.T) {
	for _, tc := range []struct {
		in        string
		wantEmpty bool
		wantErr   bool
	}{
		{"plain", false, false},
		{"canonical", true, false},
		{"", false, true},
		{"Plain", false, true},
		{"fancy", false, true},
	} {
		v, err := ParseVocabularyPreset(tc.in)
		if (err != nil) != tc.wantErr || (err == nil && VocabularyEmpty(v) != tc.wantEmpty) {
			t.Fatalf("%q: v=%+v err=%v", tc.in, v, err)
		}
		if err != nil && !strings.Contains(err.Error(), "plain") {
			t.Fatalf("%q: error must name the legal values: %v", tc.in, err)
		}
	}
}
