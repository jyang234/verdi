package artifact

import "testing"

var (
	hashOld = "sha256:" + hex64
	hashNew = "sha256:" + hexFF64
)

var hexFF64 = repeatHex("cd", 32)

func repeatHex(pair string, n int) string {
	s := ""
	for i := 0; i < n; i++ {
		s += pair
	}
	return s
}

var reaffirmationHappyYAML = `
id: reaffirmation/jira-loan-1482--ac-2
kind: reaffirmation
title: "re-affirm ac-2 as amended for jira:LOAN-1482"
schema: verdi.reaffirmation/v1
owners: [loansvc-team]
frozen: { at: 2026-07-14, commit: 3e91ab2 }
object: spec/loan-update@3e91ab2#ac-2
hash: { old: ` + hashOld + `, new: ` + hashNew + ` }
`

// guide-claim: 8.4-reaffirmations-kind
func TestDecodeReaffirmation_Happy(t *testing.T) {
	fm, err := DecodeReaffirmation([]byte(reaffirmationHappyYAML))
	if err != nil {
		t.Fatalf("DecodeReaffirmation: %v", err)
	}
	if fm.Object != "spec/loan-update@3e91ab2#ac-2" {
		t.Fatalf("Object = %q", fm.Object)
	}
	if fm.Hash.Old != hashOld || fm.Hash.New != hashNew {
		t.Fatalf("Hash = %+v", fm.Hash)
	}
}

func TestDecodeReaffirmation_Negative(t *testing.T) {
	cases := map[string]string{
		"missing frozen": `
id: reaffirmation/jira-loan-1482--ac-2
kind: reaffirmation
title: "t"
owners: [loansvc-team]
object: spec/loan-update@3e91ab2#ac-2
hash: { old: ` + hashOld + `, new: ` + hashNew + ` }
`,
		"unpinned object": `
id: reaffirmation/jira-loan-1482--ac-2
kind: reaffirmation
title: "t"
owners: [loansvc-team]
frozen: { at: 2026-07-14, commit: 3e91ab2 }
object: spec/loan-update#ac-2
hash: { old: ` + hashOld + `, new: ` + hashNew + ` }
`,
		"non-fragment object": `
id: reaffirmation/jira-loan-1482--ac-2
kind: reaffirmation
title: "t"
owners: [loansvc-team]
frozen: { at: 2026-07-14, commit: 3e91ab2 }
object: spec/loan-update@3e91ab2
hash: { old: ` + hashOld + `, new: ` + hashNew + ` }
`,
		"identical hash pair": `
id: reaffirmation/jira-loan-1482--ac-2
kind: reaffirmation
title: "t"
owners: [loansvc-team]
frozen: { at: 2026-07-14, commit: 3e91ab2 }
object: spec/loan-update@3e91ab2#ac-2
hash: { old: ` + hashOld + `, new: ` + hashOld + ` }
`,
		"status field present (existence is the record)": `
id: reaffirmation/jira-loan-1482--ac-2
kind: reaffirmation
title: "t"
owners: [loansvc-team]
status: active
frozen: { at: 2026-07-14, commit: 3e91ab2 }
object: spec/loan-update@3e91ab2#ac-2
hash: { old: ` + hashOld + `, new: ` + hashNew + ` }
`,
		"simple (non-compound) name": `
id: reaffirmation/loan-1482
kind: reaffirmation
title: "t"
owners: [loansvc-team]
frozen: { at: 2026-07-14, commit: 3e91ab2 }
object: spec/loan-update@3e91ab2#ac-2
hash: { old: ` + hashOld + `, new: ` + hashNew + ` }
`,
	}
	for name, y := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeReaffirmation([]byte(y)); err == nil {
				t.Fatalf("DecodeReaffirmation(%s): want error, got nil", name)
			}
		})
	}
}

func TestHashPair_Validate_Happy(t *testing.T) {
	hp := HashPair{Old: hashOld, New: hashNew}
	if err := hp.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestHashPair_Validate_Negative(t *testing.T) {
	cases := []HashPair{
		{Old: "not-sha256", New: hashNew},
		{Old: hashOld, New: "not-sha256"},
		{Old: hashOld, New: hashOld},
	}
	for i, hp := range cases {
		if err := hp.Validate(); err == nil {
			t.Fatalf("case %d Validate(%+v): want error, got nil", i, hp)
		}
	}
}
