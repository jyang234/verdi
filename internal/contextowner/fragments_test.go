package contextowner_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/contextevent"
	. "github.com/jyang234/verdi/internal/contextowner"
)

// These 64 expected outcomes were measured against both fragment implementations
// at e671a326 before consolidation. They describe typed fragment validation;
// they do not replace enclosing artifact or wire admission checks.
func sharedFragmentFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "public-contract", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func sharedFragmentJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	b, err := canonjson.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return bytes.TrimSuffix(b, []byte("\n"))
}

func TestSharedFragmentSegments(t *testing.T) {
	f := "store-redacted-segment.call.v2.json"
	var frame struct {
		Payload struct {
			Segment RedactedSegment `json:"segment"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(sharedFragmentFixture(t, f), &frame); err != nil {
		t.Fatal(err)
	}
	base := frame.Payload.Segment
	rows := []struct {
		name string
		ok   bool
		mut  func(*RedactedSegment)
	}{
		{"frozen", true, func(*RedactedSegment) {}}, {"schema", false, func(s *RedactedSegment) { s.Schema = "wrong" }}, {"media", false, func(s *RedactedSegment) { s.MediaType = "text/plain" }}, {"redaction", false, func(s *RedactedSegment) { s.RedactionProfile = "wrong" }}, {"nil-bytes", false, func(s *RedactedSegment) { s.Bytes = nil; s.ByteCount = 0; s.Digest = fixtureDigestOf(nil) }}, {"empty-bytes", false, func(s *RedactedSegment) { s.Bytes = []byte{}; s.ByteCount = 0; s.Digest = fixtureDigestOf(nil) }}, {"count", false, func(s *RedactedSegment) { s.ByteCount++ }}, {"digest-syntax", false, func(s *RedactedSegment) { s.Digest = "sha256:ABC" }}, {"digest-authentication", false, func(s *RedactedSegment) { s.Digest = "sha256:" + strings.Repeat("0", 64) }},
	}
	for _, x := range []struct {
		name string
		data string
		ok   bool
	}{{"noncanonical-space", "{\"a\": 1}", false}, {"canonical-object", "{\"a\":1}", true}, {"trailing-lf", "{\"a\":1}\n", false}, {"duplicate-member", "{\"a\":1,\"a\":2}", false}, {"trailing-document", "{}{}", false}, {"invalid-json", "{", false}, {"canonical-null", "null", true}, {"canonical-array", "[]", true}} {
		x := x
		rows = append(rows, struct {
			name string
			ok   bool
			mut  func(*RedactedSegment)
		}{x.name, x.ok, func(s *RedactedSegment) {
			s.Bytes = []byte(x.data)
			s.ByteCount = uint64(len(s.Bytes))
			s.Digest = fixtureDigestOf(s.Bytes)
		}})
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			s := base
			s.Bytes = append([]byte{}, base.Bytes...)
			r.mut(&s)
			err := ValidateRedactedSegment(s)
			if (err == nil) != r.ok {
				t.Fatalf("accepted = %v, want %v: %v", err == nil, r.ok, err)
			}
		})
	}
}
func TestSharedFragmentReferences(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	valid := SegmentReferencePrefix + strings.Repeat("a", 64)
	for _, r := range []struct {
		name, value string
		ok          bool
	}{{"frozen-shape", valid, true}, {"wrong-prefix", "controller-segments/sha256/" + strings.Repeat("a", 64), false}, {"short", valid[:len(valid)-1], false}, {"uppercase", SegmentReferencePrefix + strings.Repeat("A", 64), false}, {"suffix", valid + "/x", false}, {"empty", "", false}} {
		t.Run(r.name, func(t *testing.T) {
			err := ValidateSegmentReference(r.value)
			if (err == nil) != r.ok {
				t.Fatalf("accepted = %v, want %v: %v", err == nil, r.ok, err)
			}
		})
	}
	for _, r := range []struct {
		name, value string
		ok          bool
	}{{"canonical", digest, true}, {"bad-prefix", "SHA256:" + strings.Repeat("a", 64), false}, {"uppercase", "sha256:" + strings.Repeat("A", 64), false}, {"short", digest[:len(digest)-1], false}} {
		t.Run("derive-"+r.name, func(t *testing.T) {
			got, err := SegmentReference(r.value)
			if (err == nil) != r.ok {
				t.Fatalf("accepted = %v, want %v: %v", err == nil, r.ok, err)
			}
			if r.ok && got != valid {
				t.Fatalf("reference = %q, want %q", got, valid)
			}
		})
	}
}
func TestSharedFragmentKeys(t *testing.T) {
	base := ExecutionKey{Flight: "flight", Lane: "lane", Epoch: "epoch"}
	for _, field := range []string{"flight", "lane", "epoch"} {
		for _, r := range []struct {
			name, value string
			ok          bool
		}{{"valid", "value", true}, {"empty", "", false}, {"leading-space", " value", false}, {"trailing-space", "value ", false}, {"invalid-utf8", string([]byte{0xff}), false}, {"internal-space", "two words", true}, {"internal-control", "two\x01words", true}} {
			t.Run(field+"-"+r.name, func(t *testing.T) {
				s := base
				switch field {
				case "flight":
					s.Flight = r.value
				case "lane":
					s.Lane = r.value
				case "epoch":
					s.Epoch = r.value
				}
				err := ValidateExecutionKey(s)
				if (err == nil) != r.ok {
					t.Fatalf("accepted = %v, want %v: %v", err == nil, r.ok, err)
				}
			})
		}
	}
}
func TestSharedFragmentSnapshots(t *testing.T) {
	var request struct {
		Check EpochCheck `json:"check"`
	}
	if err := json.Unmarshal(sharedFragmentFixture(t, "verify-epoch.request.json"), &request); err != nil {
		t.Fatal(err)
	}
	base := request.Check.Snapshot
	rows := []struct {
		name string
		ok   bool
		mut  func(*FlightStateSnapshot)
	}{
		{"frozen", true, func(*FlightStateSnapshot) {}}, {"key-empty", false, func(s *FlightStateSnapshot) {
			s.Key.Flight = ""
		}}, {"workspace-empty", false, func(s *FlightStateSnapshot) {
			s.WorkspaceID = ""
		}}, {"workspace-whitespace", false, func(s *FlightStateSnapshot) {
			s.WorkspaceID = " x"
		}}, {"commit-bad", false, func(s *FlightStateSnapshot) {
			s.CandidateCommit = "bad"
		}}, {"tree-uppercase", false, func(s *FlightStateSnapshot) {
			s.CandidateTree = strings.Repeat("A", 40)
		}}, {"oid-64", true, func(s *FlightStateSnapshot) {
			s.CandidateCommit = strings.Repeat("a", 64)
			s.CandidateTree = s.CandidateCommit
		}}, {"manifest-digest", false, func(s *FlightStateSnapshot) {
			s.ManifestDigest = "bad"
		}}, {"projection-digest", false, func(s *FlightStateSnapshot) {
			s.ProjectionDigest = "bad"
		}}, {"expansion-empty", true, func(s *FlightStateSnapshot) {
			s.ExpansionRoot = ""
		}}, {"expansion-bad", false, func(s *FlightStateSnapshot) {
			s.ExpansionRoot = "bad"
		}}, {"sequence-zero", false, func(s *FlightStateSnapshot) {
			s.NextSourceSequence = 0
		}}, {"prior-digest-empty", true, func(s *FlightStateSnapshot) {
			s.PriorEventDigest = ""
		}}, {"prior-digest-bad", false, func(s *FlightStateSnapshot) {
			s.PriorEventDigest = "bad"
		}},
		{"unbound-metadata-control", true, func(s *FlightStateSnapshot) {
			s.Revision = 999
			s.LastGlobalSequence = 0
			s.Invalidated = true
			s.PriorRevision = &contextevent.PriorRevision{ManifestDigest: "bad"}
		}},
		{"nested-request-schema", false, func(s *FlightStateSnapshot) {
			var doc map[string]json.RawMessage
			if e := json.Unmarshal(s.Request, &doc); e != nil {
				t.Fatal(e)
			}
			doc["schema"] = json.RawMessage(`"wrong"`)
			s.Request = sharedFragmentJSON(t, doc)
		}},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			s := base
			s.Request = append(json.RawMessage{}, base.Request...)
			r.mut(&s)
			err := ValidateFlightStateSnapshot(s)
			if (err == nil) != r.ok {
				t.Fatalf("accepted = %v, want %v: %v", err == nil, r.ok, err)
			}
		})
	}

}
