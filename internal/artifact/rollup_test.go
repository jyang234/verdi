package artifact

import "testing"

func TestDecodeRollup_Happy(t *testing.T) {
	y := `{"schema":"verdi.rollup/v1","story":"jira:LOAN-1482","ref":"spec/stale-decline","commit":"7f3c2a1",
		"criteria":[
			{"id":"ac-1","text":"static check","status":"evidenced","summary":"3/3 obligations pass"},
			{"id":"ac-4","text":"runtime","status":"waived","summary":"waived pending OQ-2"}
		],
		"eligible":true,"digest":"sha256:` + hex64 + `"}`
	r, err := DecodeRollup([]byte(y))
	if err != nil {
		t.Fatalf("DecodeRollup: %v", err)
	}
	if !r.Eligible {
		t.Fatal("Eligible = false")
	}
}

func TestDecodeRollup_CountersignProjection(t *testing.T) {
	y := `{"schema":"verdi.rollup/v1","story":"jira:LOAN-1482","ref":"spec/stale-decline","commit":"7f3c2a1",
		"criteria":[{"id":"ac-1","text":"static check","status":"evidenced","summary":"3/3 obligations pass"}],
		"eligible":true,
		"countersign":{"record_digest":"sha256:` + hex64 + `","verdict":"proven",
			"approvals":[{"approval_id":"17","approval_ref":"gid://review/17","principal_id":"forge-live:101","principal_state":"authenticated"}],
			"eligible_approval_ids":["17"],"distinct_principal_ids":["forge-live:101"],
			"witnesses":["countersign-verdict:value=proven"]},
		"digest":"sha256:` + hex64 + `"}`
	r, err := DecodeRollup([]byte(y))
	if err != nil {
		t.Fatalf("DecodeRollup: %v", err)
	}
	if r.Countersign == nil || r.Countersign.RecordDigest != "sha256:"+hex64 || len(r.Countersign.Approvals) != 1 || r.Countersign.Approvals[0].ApprovalRef != "gid://review/17" {
		t.Fatalf("Countersign = %+v", r.Countersign)
	}

	for _, tc := range []struct {
		name string
		edit func(*Rollup)
	}{
		{"non-proven verdict", func(r *Rollup) { r.Countersign.Verdict = "unproven" }},
		{"missing record digest", func(r *Rollup) { r.Countersign.RecordDigest = "" }},
		{"null approvals", func(r *Rollup) { r.Countersign.Approvals = nil }},
		{"missing approval ref", func(r *Rollup) { r.Countersign.Approvals[0].ApprovalRef = "" }},
		{"unknown principal state", func(r *Rollup) { r.Countersign.Approvals[0].PrincipalState = "mystery" }},
		{"null eligible ids", func(r *Rollup) { r.Countersign.EligibleApprovalIDs = nil }},
		{"null principals", func(r *Rollup) { r.Countersign.DistinctPrincipalIDs = nil }},
		{"null witnesses", func(r *Rollup) { r.Countersign.Witnesses = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			copy := *r
			projection := *r.Countersign
			projection.Approvals = append([]RollupCountersignApproval{}, projection.Approvals...)
			projection.EligibleApprovalIDs = append([]string{}, projection.EligibleApprovalIDs...)
			projection.DistinctPrincipalIDs = append([]string{}, projection.DistinctPrincipalIDs...)
			projection.Witnesses = append([]string{}, projection.Witnesses...)
			copy.Countersign = &projection
			tc.edit(&copy)
			if err := copy.Validate(); err == nil {
				t.Fatalf("Validate(%s): want error", tc.name)
			}
		})
	}
}

// TestDecodeRollup_Happy_FeatureNoStory proves a feature rollup with no
// story: tracker ref at all (empty string) is a valid rollup.json — R4-I-2:
// a feature spec's story: field is OPTIONAL (spec/true-closure is a real
// example carrying none), so the closure ritual must still be able to
// write and validate its rollup.json quartet member even though there is
// nowhere honest to publish it (cmd/verdi/closefeature.go skips the
// tracker publish step in exactly this case, never fabricating a ref).
func TestDecodeRollup_Happy_FeatureNoStory(t *testing.T) {
	y := `{"schema":"verdi.rollup/v1","story":"","ref":"spec/close-feature-fixture","commit":"7f3c2a1",
		"criteria":[
			{"id":"ac-1","text":"outcome check","status":"evidenced","summary":"attestation:present"}
		],
		"eligible":true,"digest":"sha256:` + hex64 + `"}`
	r, err := DecodeRollup([]byte(y))
	if err != nil {
		t.Fatalf("DecodeRollup: %v", err)
	}
	if r.Story != "" {
		t.Fatalf("Story = %q, want empty", r.Story)
	}
	if !r.Eligible {
		t.Fatal("Eligible = false")
	}
}

func TestDecodeRollup_Negative(t *testing.T) {
	base := `{"schema":"verdi.rollup/v1","story":"jira:LOAN-1482","ref":"spec/stale-decline","commit":"7f3c2a1","criteria":[{"id":"ac-1","text":"t","status":"pending","summary":"s"}],"eligible":true,"digest":"sha256:` + hex64 + `"}`
	cases := map[string]string{
		"wrong schema":             `{"schema":"bogus","story":"jira:LOAN-1482","ref":"spec/stale-decline","commit":"7f3c2a1","criteria":[{"id":"ac-1","text":"t","status":"evidenced","summary":"s"}],"eligible":true,"digest":"sha256:` + hex64 + `"}`,
		"bad story":                `{"schema":"verdi.rollup/v1","story":"LOAN-1482","ref":"spec/stale-decline","commit":"7f3c2a1","criteria":[{"id":"ac-1","text":"t","status":"evidenced","summary":"s"}],"eligible":true,"digest":"sha256:` + hex64 + `"}`,
		"ref not spec kind":        `{"schema":"verdi.rollup/v1","story":"jira:LOAN-1482","ref":"adr/0001-foo","commit":"7f3c2a1","criteria":[{"id":"ac-1","text":"t","status":"evidenced","summary":"s"}],"eligible":true,"digest":"sha256:` + hex64 + `"}`,
		"bad commit":               `{"schema":"verdi.rollup/v1","story":"jira:LOAN-1482","ref":"spec/stale-decline","commit":"xyz","criteria":[{"id":"ac-1","text":"t","status":"evidenced","summary":"s"}],"eligible":true,"digest":"sha256:` + hex64 + `"}`,
		"bad digest":               `{"schema":"verdi.rollup/v1","story":"jira:LOAN-1482","ref":"spec/stale-decline","commit":"7f3c2a1","criteria":[{"id":"ac-1","text":"t","status":"evidenced","summary":"s"}],"eligible":true,"digest":"nope"}`,
		"no criteria":              `{"schema":"verdi.rollup/v1","story":"jira:LOAN-1482","ref":"spec/stale-decline","commit":"7f3c2a1","criteria":[],"eligible":true,"digest":"sha256:` + hex64 + `"}`,
		"duplicate criterion":      `{"schema":"verdi.rollup/v1","story":"jira:LOAN-1482","ref":"spec/stale-decline","commit":"7f3c2a1","criteria":[{"id":"ac-1","text":"t","status":"evidenced","summary":"s"},{"id":"ac-1","text":"t2","status":"pending","summary":"s2"}],"eligible":true,"digest":"sha256:` + hex64 + `"}`,
		"unknown criterion status": `{"schema":"verdi.rollup/v1","story":"jira:LOAN-1482","ref":"spec/stale-decline","commit":"7f3c2a1","criteria":[{"id":"ac-1","text":"t","status":"bogus","summary":"s"}],"eligible":true,"digest":"sha256:` + hex64 + `"}`,
		"eligible disagrees with criteria (pending)": base,
		"unknown field": `{"schema":"verdi.rollup/v1","story":"jira:LOAN-1482","ref":"spec/stale-decline","commit":"7f3c2a1","criteria":[{"id":"ac-1","text":"t","status":"evidenced","summary":"s"}],"eligible":true,"digest":"sha256:` + hex64 + `","bogus":true}`,
	}
	for name, y := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeRollup([]byte(y)); err == nil {
				t.Fatalf("DecodeRollup(%s): want error, got nil", name)
			}
		})
	}
}
