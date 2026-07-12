package artifact

import "testing"

const boardLayoutHappyJSON = `{
  "schema": "verdi.boardlayout/v1",
  "positions": { "ac-1": { "x": 40, "y": 20 }, "dc-1": { "x": 40, "y": 180 } }
}`

func TestDecodeBoardLayout_Happy(t *testing.T) {
	bl, err := DecodeBoardLayout([]byte(boardLayoutHappyJSON))
	if err != nil {
		t.Fatalf("DecodeBoardLayout: %v", err)
	}
	if len(bl.Positions) != 2 {
		t.Fatalf("Positions = %+v, want 2 entries", bl.Positions)
	}
	if bl.Positions["ac-1"].X != 40 || bl.Positions["ac-1"].Y != 20 {
		t.Fatalf("Positions[ac-1] = %+v, want {40 20}", bl.Positions["ac-1"])
	}
}

// TestDecodeBoardLayout_EmptyPositions proves the absent-key fallback path
// still decodes: an empty (or partial) positions map is valid — VL-018's
// dangling-key check is out of scope here, but "no stored position at all"
// must not be an error.
func TestDecodeBoardLayout_EmptyPositions(t *testing.T) {
	const y = `{"schema": "verdi.boardlayout/v1", "positions": {}}`
	bl, err := DecodeBoardLayout([]byte(y))
	if err != nil {
		t.Fatalf("DecodeBoardLayout: %v", err)
	}
	if len(bl.Positions) != 0 {
		t.Fatalf("Positions = %+v, want empty", bl.Positions)
	}
}

func TestDecodeBoardLayout_Negative(t *testing.T) {
	cases := map[string]string{
		"wrong schema":         `{"schema": "verdi.boardlayout/v2", "positions": {}}`,
		"bad position key":     `{"schema": "verdi.boardlayout/v1", "positions": {"Not-An-Id": {"x": 1, "y": 2}}}`,
		"unknown field":        `{"schema": "verdi.boardlayout/v1", "positions": {}, "bogus": true}`,
		"positions wrong type": `{"schema": "verdi.boardlayout/v1", "positions": []}`,
		"empty stub slug":      `{"schema": "verdi.boardlayout/v1", "positions": {"stub:": {"x": 1, "y": 2}}}`,
		"uppercase stub slug":  `{"schema": "verdi.boardlayout/v1", "positions": {"stub:Not-Kebab": {"x": 1, "y": 2}}}`,
	}
	for name, y := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeBoardLayout([]byte(y)); err == nil {
				t.Fatalf("DecodeBoardLayout(%s): want error, got nil", name)
			}
		})
	}
}

// TestDecodeBoardLayout_StubKey proves a "stub:<slug>" positions key
// decodes (round 5.5 dc-6 amendment: a stub is now a legal position-key
// namespace alongside object ids) — shape only; VL-018 resolves it against
// the sibling spec's declared stubs.
func TestDecodeBoardLayout_StubKey(t *testing.T) {
	const y = `{"schema": "verdi.boardlayout/v1", "positions": {"ac-1": {"x": 40, "y": 20}, "stub:borrower-update-api": {"x": 990, "y": 40}}}`
	bl, err := DecodeBoardLayout([]byte(y))
	if err != nil {
		t.Fatalf("DecodeBoardLayout: %v", err)
	}
	if len(bl.Positions) != 2 {
		t.Fatalf("Positions = %+v, want 2 entries", bl.Positions)
	}
	if p := bl.Positions["stub:borrower-update-api"]; p.X != 990 || p.Y != 40 {
		t.Fatalf("Positions[stub:borrower-update-api] = %+v, want {990 40}", p)
	}
}
