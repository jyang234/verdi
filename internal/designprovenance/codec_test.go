package designprovenance

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/governanceprincipal"
)

const (
	digestA = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	digestB = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	digestC = "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
)

func validEntry(t *testing.T) Entry {
	t.Helper()
	e := Entry{
		Schema:         Schema,
		Spec:           "spec/widget",
		PreviousDigest: digestA,
		ResultDigest:   digestB,
		Attribution:    governanceprincipal.NewUnauthenticatedAttribution(),
		Harness:        "codex",
		PolicyDigest:   digestC,
		Context:        UnavailableContext(),
		Operations: []Operation{{
			Op:     OpSetProblem,
			Text:   "customers cannot retry safely",
			Anchor: "problem",
		}},
		Changes:  []Change{{Target: "problem", Change: ChangeReplaced, BeforeDigest: digestA, AfterDigest: digestB}},
		Excerpts: []Excerpt{},
	}
	if err := e.Seal(); err != nil {
		t.Fatalf("Seal: %v", err)
	}
	return e
}

func TestEntryStrictDecode(t *testing.T) {
	e := validEntry(t)
	raw, err := EncodeEntry(e)
	if err != nil {
		t.Fatalf("EncodeEntry: %v", err)
	}
	got, err := DecodeEntry(bytes.TrimSuffix(raw, []byte("\n")))
	if err != nil {
		t.Fatalf("DecodeEntry: %v", err)
	}
	if got.Digest != e.Digest || got.Context.Reason != ContextUnavailableReason {
		t.Fatalf("decoded entry = %+v, want digest %q and fixed context reason", got, e.Digest)
	}

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"unknown envelope field", strings.Replace(string(raw), `"schema":`, `"mystery":true,"schema":`, 1), "mystery"},
		{"trailing data", string(raw) + `{}`, "trailing"},
		{"duplicate envelope key", strings.Replace(string(raw), `"schema":`, `"schema":"verdi.design-provenance/v1","schema":`, 1), "duplicate"},
		{"unknown operation field", strings.Replace(string(raw), `"text":"customers`, `"surprise":true,"text":"customers`, 1), "surprise"},
		{"noncanonical whitespace", strings.Replace(string(bytes.TrimSuffix(raw, []byte("\n"))), `{`, `{ `, 1), "canonical"},
		{"optional null", strings.Replace(string(bytes.TrimSuffix(raw, []byte("\n"))), `"spec":`, `"session":null,"spec":`, 1), "canonical"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := DecodeEntry([]byte(tt.raw)); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("DecodeEntry error = %v, want it to contain %q", err, tt.want)
			}
		})
	}
}

func TestEntryAttributionAndOwnDigest(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Entry)
		want   string
	}{
		{"both attribution arms", func(e *Entry) { e.Attribution.PrincipalID = "principal/github/YWxpY2U" }, "exactly one"},
		{"unauthenticated session missing harness", func(e *Entry) { e.Harness, e.Session = "", "session-1" }, "harness"},
		{"principal arm carries harness", func(e *Entry) {
			e.Attribution = governanceprincipal.Attribution{PrincipalID: "principal/github/YWxpY2U"}
		}, "harness"},
		{"tampered own digest", func(e *Entry) { e.ResultDigest = digestC }, "own digest"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := validEntry(t)
			tt.mutate(&e)
			if err := e.Validate(); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate error = %v, want it to contain %q", err, tt.want)
			}
		})
	}

	unauthenticatedResolved := validEntry(t)
	unauthenticatedResolved.Harness = ""
	unauthenticatedResolved.Session = ""
	if err := unauthenticatedResolved.Seal(); err != nil {
		t.Fatalf("Seal resolved unauthenticated attribution: %v", err)
	}
	if err := unauthenticatedResolved.Validate(); err != nil {
		t.Fatalf("Validate resolved unauthenticated attribution: %v", err)
	}

	e := validEntry(t)
	projection := e
	projection.Digest = ""
	want, err := canonjson.Digest(projection.digestProjection())
	if err != nil {
		t.Fatalf("canonjson.Digest: %v", err)
	}
	if e.Digest != want {
		t.Fatalf("entry digest = %q, want projection digest %q", e.Digest, want)
	}
}

func TestJSONLChain(t *testing.T) {
	first := validEntry(t)
	second := validEntry(t)
	second.PreviousDigest = first.ResultDigest
	second.ResultDigest = digestC
	if err := second.Seal(); err != nil {
		t.Fatalf("Seal second: %v", err)
	}
	firstRaw, _ := EncodeEntry(first)
	secondRaw, _ := EncodeEntry(second)
	log := append(append([]byte{}, firstRaw...), secondRaw...)
	entries, err := DecodeLog(log)
	if err != nil {
		t.Fatalf("DecodeLog: %v", err)
	}
	if len(entries) != 2 || !bytes.HasSuffix(log, []byte("\n")) {
		t.Fatalf("DecodeLog entries = %d, want 2 and JSONL newline", len(entries))
	}

	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"missing final newline", bytes.TrimSuffix(log, []byte("\n")), "newline"},
		{"duplicate digest", append(append([]byte{}, firstRaw...), firstRaw...), "duplicate"},
		{"unexplained chain break", func() []byte {
			broken := second
			broken.PreviousDigest = "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
			broken.UnclassifiedGap = nil
			_ = broken.Seal()
			raw, _ := EncodeEntry(broken)
			return append(append([]byte{}, firstRaw...), raw...)
		}(), "chain"},
		{"invalid explained gap", func() []byte {
			broken := second
			broken.PreviousDigest = "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
			broken.UnclassifiedGap = &UnclassifiedGap{FromDigest: digestC, ToDigest: broken.PreviousDigest}
			_ = broken.Seal()
			raw, _ := EncodeEntry(broken)
			return append(append([]byte{}, firstRaw...), raw...)
		}(), "gap"},
		{"first entry cannot claim gap", func() []byte {
			broken := first
			broken.UnclassifiedGap = &UnclassifiedGap{FromDigest: digestC, ToDigest: broken.PreviousDigest}
			_ = broken.Seal()
			raw, _ := EncodeEntry(broken)
			return raw
		}(), "first"},
		{"mixed spec identities", func() []byte {
			broken := second
			broken.Spec = "spec/other"
			_ = broken.Seal()
			raw, _ := EncodeEntry(broken)
			return append(append([]byte{}, firstRaw...), raw...)
		}(), "spec"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := DecodeLog(tt.data); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("DecodeLog error = %v, want it to contain %q", err, tt.want)
			}
		})
	}
}

func TestOperationAndExcerptValidation(t *testing.T) {
	e := validEntry(t)
	e.Operations[0].ID = "ac-forged"
	if err := e.Seal(); err == nil || !strings.Contains(err.Error(), "field") {
		t.Fatalf("Seal operation with foreign field error = %v, want field-set refusal", err)
	}

	e = validEntry(t)
	e.Excerpts = []Excerpt{{
		Target:         "problem",
		TargetDigest:   digestB,
		Classification: ClassificationHumanStated,
		Representation: RepresentationVerbatim,
		Text:           strings.Repeat("é", MaxExcerptScalars+1),
	}}
	if err := e.Seal(); err == nil || !strings.Contains(err.Error(), "600") {
		t.Fatalf("Seal oversized excerpt error = %v, want scalar bound", err)
	}

	e.Excerpts[0].Target = "link/spec/depends-on/spec%2Fbase"
	e.Excerpts[0].Text = "bounded"
	if err := e.Seal(); err == nil || !strings.Contains(err.Error(), "object ID") {
		t.Fatalf("Seal relationship excerpt target error = %v, want object target refusal", err)
	}

	for _, tt := range []struct {
		name string
		op   Operation
		want string
	}{
		{"reorder stub after slug", Operation{Op: OpReorderStub, Slug: "second", AfterSlug: "first"}, ""},
		{"unknown link type", Operation{Op: OpAddLink, Source: "spec", Type: "invented", Ref: "spec/other"}, "link type"},
		{"duplicate plain stub criterion", Operation{Op: OpAddStub, Slug: "story", AcceptanceCriteria: []string{"one", "one"}}, "duplicate"},
		{"duplicate spike resolution", Operation{Op: OpAddStub, Slug: "story", Spike: boolPtr(true), Resolves: []string{"question/q-1", "question/q-1"}}, "duplicate"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.op.Validate()
			if tt.want == "" && err != nil {
				t.Fatalf("Validate: %v", err)
			}
			if tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want)) {
				t.Fatalf("Validate error = %v, want %q", err, tt.want)
			}
		})
	}

	for _, raw := range []string{
		`{"op":"reorder-ac","id":"ac-1","after_id":""}`,
		`{"op":"reorder-stub","slug":"story","after_slug":""}`,
		`{"op":"add-link","source":"spec","type":"depends-on","ref":"spec/other","note":""}`,
	} {
		var operation Operation
		if err := decodeStrictJSON([]byte(raw), &operation); err == nil || !strings.Contains(err.Error(), "nonempty") {
			t.Fatalf("empty optional operation field accepted: %s (%v)", raw, err)
		}
	}
}

func TestEntryRejectsBlankHarnessAndSession(t *testing.T) {
	for _, mutate := range []func(*Entry){
		func(entry *Entry) { entry.Harness = "   " },
		func(entry *Entry) { entry.Session = "   " },
	} {
		entry := validEntry(t)
		mutate(&entry)
		if err := entry.Seal(); err == nil || !strings.Contains(err.Error(), "nonblank") {
			t.Fatalf("Seal blank harness/session error = %v", err)
		}
	}
}

func boolPtr(v bool) *bool { return &v }

func TestCanonicalEntryHasArraysAndNewline(t *testing.T) {
	e := validEntry(t)
	e.Changes = []Change{}
	e.Excerpts = []Excerpt{}
	if err := e.Seal(); err != nil {
		t.Fatalf("Seal: %v", err)
	}
	raw, err := EncodeEntry(e)
	if err != nil {
		t.Fatalf("EncodeEntry: %v", err)
	}
	if !bytes.HasSuffix(raw, []byte("\n")) || !bytes.Contains(raw, []byte(`"changes":[]`)) || !bytes.Contains(raw, []byte(`"excerpts":[]`)) {
		t.Fatalf("canonical entry = %q, want newline and explicit empty arrays", raw)
	}
	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatalf("json.Unmarshal canonical entry: %v", err)
	}
}
