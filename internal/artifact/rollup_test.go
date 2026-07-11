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
