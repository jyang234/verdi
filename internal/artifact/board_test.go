package artifact

import "testing"

func TestDecodeBoard_Happy(t *testing.T) {
	cases := map[string]string{
		"live": `{"schema":"verdi.board/v1",
			"pins":[{"ref":"spec/stale-decline@7f3c2a1","x":10,"y":10}],
			"stickies":[{"id":"a-01J8Z0K3AAAAAAAAAAAAAAAAAA","x":20,"y":20}],
			"yarn":[{"from":"pin:spec/stale-decline","to":"sticky:a-01J8Z0K3AAAAAAAAAAAAAAAAAA","label":"relates"}]}`,
		"frozen": `{"schema":"verdi.board/v1",
			"pins":[{"ref":"spec/stale-decline@7f3c2a1","x":10,"y":10}],
			"stickies":[],
			"yarn":[],
			"frozen":{"at":"2026-05-14","commit":"3e91ab2"},
			"provenance":{"generator":"commit-to-design","version":"v0","inputs":["spec/stale-decline@7f3c2a1"],"digest":"sha256:` + hex64 + `"}}`,
	}
	for name, y := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeBoard([]byte(y)); err != nil {
				t.Fatalf("DecodeBoard: %v", err)
			}
		})
	}
}

func TestDecodeBoard_Negative(t *testing.T) {
	cases := map[string]string{
		"wrong schema":              `{"schema":"bogus","pins":[],"stickies":[],"yarn":[]}`,
		"unpinned pin ref":          `{"schema":"verdi.board/v1","pins":[{"ref":"spec/foo","x":0,"y":0}],"stickies":[],"yarn":[]}`,
		"bad sticky id":             `{"schema":"verdi.board/v1","pins":[],"stickies":[{"id":"not-a-ulid","x":0,"y":0}],"yarn":[]}`,
		"yarn missing to":           `{"schema":"verdi.board/v1","pins":[],"stickies":[],"yarn":[{"from":"a","to":"","label":"x"}]}`,
		"frozen without provenance": `{"schema":"verdi.board/v1","pins":[],"stickies":[],"yarn":[],"frozen":{"at":"2026-05-14","commit":"3e91ab2"}}`,
		"provenance without frozen": `{"schema":"verdi.board/v1","pins":[],"stickies":[],"yarn":[],"provenance":{"generator":"g","version":"v0","inputs":["spec/foo@3e91ab2"],"digest":"sha256:` + hex64 + `"}}`,
		"unknown field":             `{"schema":"verdi.board/v1","pins":[],"stickies":[],"yarn":[],"bogus":true}`,
	}
	for name, y := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeBoard([]byte(y)); err == nil {
				t.Fatalf("DecodeBoard(%s): want error, got nil", name)
			}
		})
	}
}
