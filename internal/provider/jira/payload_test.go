package jira_test

import "encoding/json"

// wirePayload mirrors the machine field's compact JSON wire schema (04
// §Jira adapter, verbatim): "{ commit, eligible, criteria: [{id, status}] }".
// Decoding it directly here (rather than reaching into the jira package's
// unexported rollupPayload type) keeps this test package honest: it only
// gets to assert on the same wire contract 04 documents, nothing internal.
type wirePayload struct {
	Commit   string          `json:"commit"`
	Eligible bool            `json:"eligible"`
	Criteria []wireCriterion `json:"criteria"`
}

type wireCriterion struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func decodeRollupPayload(raw string) (wirePayload, error) {
	var p wirePayload
	err := json.Unmarshal([]byte(raw), &p)
	return p, err
}
