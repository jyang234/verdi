package draftmutation

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

const baseSpec = `---
id: spec/sample
kind: spec
class: feature
title: Sample
owners: [platform-team]
links: [ { type: depends-on, ref: spec/base } ]
problem: { text: "old problem", anchor: "#problem" }
outcome: { text: "old outcome", anchor: "#outcome" }
context: [spec/base@abcdef0, adr/choice@abcdef1]
acceptance_criteria:
  - { id: ac-1, text: "first", evidence: [static], anchor: "#ac-1" }
  - { id: ac-2, text: "second", evidence: [behavioral], anchor: "#ac-2" }
constraints:
  - { id: co-1, text: "bounded", anchor: "#co-1" }
decisions:
  - { id: dc-1, text: "reuse base", anchor: "#dc-1", links: [ { type: depends-on, ref: spec/base } ] }
open_questions:
  - { id: oq-1, text: "which signal?", anchor: "#oq-1" }
stubs:
  - { slug: first-story, acceptance_criteria: [ac-1] }
  - { slug: second-story, acceptance_criteria: [ac-2] }
---
# Sample

## Problem

Old prose stays.

## Outcome

Old prose stays.

## ac-1

First.

## ac-2

Second.

## co-1

Constraint.

## dc-1

Decision.

## oq-1

Question.
`

func validRequestBytes(t *testing.T) []byte {
	t.Helper()
	digest := DigestBytes([]byte(baseSpec))
	request := map[string]any{
		"schema":        RequestSchema,
		"spec":          "spec/sample",
		"base_digest":   digest,
		"base_spec_b64": base64.StdEncoding.EncodeToString([]byte(baseSpec)),
		"expected": map[string]any{
			"checkout": "/tmp/repository",
			"branch":   "design/sample",
			"head":     strings.Repeat("a", 40),
		},
		"operations": []any{map[string]any{
			"op": "set-problem", "text": "new problem", "anchor": "#problem",
		}},
	}
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestDecodeRequestStrictAndBaseSnapshot(t *testing.T) {
	raw := validRequestBytes(t)
	req, err := DecodeRequest(raw)
	if err != nil {
		t.Fatalf("DecodeRequest: %v", err)
	}
	if req.Spec != "spec/sample" || !bytes.Equal(req.BaseSpec, []byte(baseSpec)) || len(req.Operations) != 1 {
		t.Fatalf("request = %+v", req)
	}

	tests := []struct {
		name string
		raw  []byte
		want string
	}{
		{"unknown", bytes.Replace(raw, []byte(`"schema":`), []byte(`"unknown":true,"schema":`), 1), "unknown"},
		{"duplicate", bytes.Replace(raw, []byte(`"schema":`), []byte(`"schema":"verdi.draftmutation/v1","schema":`), 1), "duplicate"},
		{"trailing", append(append([]byte{}, raw...), []byte(`{}`)...), "trailing"},
		{"request attribution", bytes.Replace(raw, []byte(`"operations":`), []byte(`"attribution":{"unauthenticated":true},"operations":`), 1), "attribution"},
		{"operation foreign field", bytes.Replace(raw, []byte(`"text":"new problem"`), []byte(`"id":"ac-forged","text":"new problem"`), 1), "id"},
		{"base spec identity", bytes.Replace(raw, []byte(`"spec":"spec/sample"`), []byte(`"spec":"spec/other"`), 1), "base spec id"},
		{"base mismatch", bytes.Replace(raw, []byte(DigestBytes([]byte(baseSpec))), []byte("sha256:"+strings.Repeat("b", 64)), 1), "base_digest"},
		{"noncanonical base64", bytes.Replace(raw, []byte(base64.StdEncoding.EncodeToString([]byte(baseSpec))), []byte(base64.StdEncoding.EncodeToString([]byte(baseSpec))[:len(base64.StdEncoding.EncodeToString([]byte(baseSpec)))-1]), 1), "base_spec_b64"},
		{"oversize", append(append([]byte{}, raw...), bytes.Repeat([]byte(" "), MaxRequestBytes)...), "1 MiB"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := DecodeRequest(tt.raw); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("DecodeRequest error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestDecodeRequestExcerptBounds(t *testing.T) {
	raw := validRequestBytes(t)
	excerpt := `"excerpts":[{"target":"problem","classification":"human-stated","representation":"verbatim","text":"one"},{"target":"problem","classification":"human-stated","representation":"verbatim","text":"two"},{"target":"problem","classification":"human-stated","representation":"verbatim","text":"three"},{"target":"problem","classification":"human-stated","representation":"verbatim","text":"four"}],`
	raw = bytes.Replace(raw, []byte(`"operations":`), []byte(excerpt+`"operations":`), 1)
	if _, err := DecodeRequest(raw); err == nil || !strings.Contains(err.Error(), "three") {
		t.Fatalf("DecodeRequest error = %v, want per-target excerpt bound", err)
	}

	badEvidence := bytes.Replace(validRequestBytes(t), []byte(`{"anchor":"#problem","op":"set-problem","text":"new problem"}`), []byte(`{"anchor":"#ac-1","evidence":["static","static"],"id":"ac-1","op":"edit-ac","text":"new"}`), 1)
	if _, err := DecodeRequest(badEvidence); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("DecodeRequest duplicate evidence error = %v", err)
	}

	if !artifact.ValidDigest(DigestBytes([]byte(baseSpec))) {
		t.Fatal("DigestBytes did not return a canonical digest")
	}
}
