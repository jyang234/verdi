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
		"pin with a why": `{"id":"a-01J8Z0K6DDDDDDDDDDDDDDDDDD","ts":"2026-07-10T14:05:00Z","author":"john",
			"target":{"ref":"adr/0001-outbox-events@7f3c2a1","selector":{"heading":"","quote":"","line":null}},
			"board":{"story":"refi-decline-flow","x":50,"y":60},
			"type":"pin","body":"background for the outbox decision","status":"open"}`,
		"pin with no body (body optional for pins)": `{"id":"a-01J8Z0K7EEEEEEEEEEEEEEEEEE","ts":"2026-07-10T14:06:00Z","author":"john",
			"target":{"ref":"diagram/loansvc-topology@7f3c2a1","selector":{"heading":"","quote":"","line":null}},
			"board":{"story":"refi-decline-flow","x":70,"y":80},
			"type":"pin","body":"","status":"open"}`,
		"graduated pin": `{"id":"a-01J8Z0K8FFFFFFFFFFFFFFFFFF","ts":"2026-07-10T14:07:00Z","author":"john",
			"target":{"ref":"adr/0002-outbox-events@7f3c2a1","selector":{"heading":"","quote":"","line":null}},
			"board":{"story":"refi-decline-flow","x":90,"y":100},
			"type":"pin","body":"","status":"graduated"}`,
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

		// The pin type's own closed shape (02 §Record schemas, round-5.2
		// amendment): a pin REQUIRES a target (the pinned artifact) AND a
		// board position; a selector-bearing target, a target_b, or a
		// fragment target all fail closed.
		"pin without target":        `{"id":"a-01J8Z0K3AAAAAAAAAAAAAAAAAA","ts":"2026-07-10T14:02:11Z","author":"j","board":{"story":"S","x":0,"y":0},"type":"pin","body":"","status":"open"}`,
		"pin without board":         `{"id":"a-01J8Z0K3AAAAAAAAAAAAAAAAAA","ts":"2026-07-10T14:02:11Z","author":"j","target":{"ref":"adr/0001-x@7f3c2a1","selector":{"heading":"","quote":"","line":null}},"type":"pin","body":"","status":"open"}`,
		"pin with a selector":       `{"id":"a-01J8Z0K3AAAAAAAAAAAAAAAAAA","ts":"2026-07-10T14:02:11Z","author":"j","target":{"ref":"adr/0001-x@7f3c2a1","selector":{"heading":"h","quote":"","line":null}},"board":{"story":"S","x":0,"y":0},"type":"pin","body":"","status":"open"}`,
		"pin with a target_b":       `{"id":"a-01J8Z0K3AAAAAAAAAAAAAAAAAA","ts":"2026-07-10T14:02:11Z","author":"j","target":{"ref":"adr/0001-x@7f3c2a1","selector":{"heading":"","quote":"","line":null}},"target_b":{"ref":"adr/0002-x@7f3c2a1","selector":{"heading":"","quote":"","line":null}},"board":{"story":"S","x":0,"y":0},"type":"pin","body":"","status":"open"}`,
		"pin with a fragment target": `{"id":"a-01J8Z0K3AAAAAAAAAAAAAAAAAA","ts":"2026-07-10T14:02:11Z","author":"j","target":{"ref":"spec/foo@7f3c2a1#ac-1","selector":{"heading":"","quote":"","line":null}},"board":{"story":"S","x":0,"y":0},"type":"pin","body":"","status":"open"}`,
	}
	for name, y := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeAnnotation([]byte(y)); err == nil {
				t.Fatalf("DecodeAnnotation(%s): want error, got nil", name)
			}
		})
	}
}
