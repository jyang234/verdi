package artifact

import "testing"

func TestDecodeAnnotation_Happy(t *testing.T) {
	cases := map[string]string{
		"targeted": `{"id":"a-01J8Z0K3AAAAAAAAAAAAAAAAAA","ts":"2026-07-10T14:02:11Z","author":"john",
			"target":{"ref":"spec/stale-decline@7f3c2a1","selector":{"heading":"ac-2","quote":"charge API","line":null}},
			"type":"comment","body":"needs a note","status":"open"}`,
		"board-only": `{"id":"a-01J8Z0K4BBBBBBBBBBBBBBBBBB","ts":"2026-07-10T14:03:00Z","author":"jane",
			"board":{"story":"STORY-1482","x":10,"y":20},
			"type":"question","body":"what about retries?","status":"open"}`,
		"agent-task": `{"id":"a-01J8Z0K5CCCCCCCCCCCCCCCCCC","ts":"2026-07-10T14:04:00Z","author":"claude",
			"board":{"story":"STORY-1482","x":30,"y":40},
			"type":"agent-task","body":"wire up the retry worker","status":"open"}`,
	}
	for name, y := range cases {
		t.Run(name, func(t *testing.T) {
			a, err := DecodeAnnotation([]byte(y))
			if err != nil {
				t.Fatalf("DecodeAnnotation: %v", err)
			}
			if a.ID == "" {
				t.Fatal("empty id")
			}
		})
	}
}

func TestDecodeAnnotation_Negative(t *testing.T) {
	cases := map[string]string{
		"bad id":                   `{"id":"not-a-ulid","ts":"2026-07-10T14:02:11Z","author":"j","board":{"story":"S","x":0,"y":0},"type":"comment","body":"x","status":"open"}`,
		"bad ts":                   `{"id":"a-01J8Z0K3AAAAAAAAAAAAAAAAAA","ts":"not-a-date","author":"j","board":{"story":"S","x":0,"y":0},"type":"comment","body":"x","status":"open"}`,
		"missing author":           `{"id":"a-01J8Z0K3AAAAAAAAAAAAAAAAAA","ts":"2026-07-10T14:02:11Z","board":{"story":"S","x":0,"y":0},"type":"comment","body":"x","status":"open"}`,
		"neither target nor board": `{"id":"a-01J8Z0K3AAAAAAAAAAAAAAAAAA","ts":"2026-07-10T14:02:11Z","author":"j","type":"comment","body":"x","status":"open"}`,
		"unpinned target ref":      `{"id":"a-01J8Z0K3AAAAAAAAAAAAAAAAAA","ts":"2026-07-10T14:02:11Z","author":"j","target":{"ref":"spec/foo","selector":{"heading":"h","quote":"q","line":null}},"type":"comment","body":"x","status":"open"}`,
		"unknown type":             `{"id":"a-01J8Z0K3AAAAAAAAAAAAAAAAAA","ts":"2026-07-10T14:02:11Z","author":"j","board":{"story":"S","x":0,"y":0},"type":"bogus","body":"x","status":"open"}`,
		"unknown status":           `{"id":"a-01J8Z0K3AAAAAAAAAAAAAAAAAA","ts":"2026-07-10T14:02:11Z","author":"j","board":{"story":"S","x":0,"y":0},"type":"comment","body":"x","status":"bogus"}`,
		"empty body":               `{"id":"a-01J8Z0K3AAAAAAAAAAAAAAAAAA","ts":"2026-07-10T14:02:11Z","author":"j","board":{"story":"S","x":0,"y":0},"type":"comment","body":"","status":"open"}`,
		"unknown field":            `{"id":"a-01J8Z0K3AAAAAAAAAAAAAAAAAA","ts":"2026-07-10T14:02:11Z","author":"j","board":{"story":"S","x":0,"y":0},"type":"comment","body":"x","status":"open","bogus":true}`,
	}
	for name, y := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeAnnotation([]byte(y)); err == nil {
				t.Fatalf("DecodeAnnotation(%s): want error, got nil", name)
			}
		})
	}
}
