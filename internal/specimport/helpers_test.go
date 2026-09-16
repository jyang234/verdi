package specimport

import "encoding/json"

// validMarkdownSource is a minimal, well-formed positive markdown-v1
// fixture: labeled Problem/Outcome (Problem spans two physical lines, a
// direct byte-for-byte reproduction of the plan's own core positive
// example) and a flat Acceptance Criteria list.
const validMarkdownSource = "# Sample Feature\n" +
	"\n" +
	"## Problem\n" +
	"\n" +
	"First line.\n" +
	"Second line.\n" +
	"\n" +
	"## Outcome\n" +
	"\n" +
	"Users get value.\n" +
	"\n" +
	"## Acceptance Criteria\n" +
	"\n" +
	"- Criterion one.\n" +
	"- Criterion two.\n"

// minimalRequest returns a well-formed markdown-v1 Request over
// validMarkdownSource, ready for a test to mutate one field at a time.
func minimalRequest() Request {
	return Request{
		Schema:  RequestSchema,
		Target:  Target{Slug: "sample-feature", Class: "feature", Title: "Sample Feature"},
		Format:  FormatMarkdownV1,
		Primary: "source",
		Sources: []Source{
			{ID: "source", Label: "sample.md", Data: []byte(validMarkdownSource)},
		},
		RetainUnmapped: true,
	}
}

// mustJSON marshals v with the standard encoder (not canonjson): these are
// wire-format decode inputs, and DecodeRequest must accept ordinary
// encoding/json output, not just canonjson's own byte-for-byte form.
func mustJSON(v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}
