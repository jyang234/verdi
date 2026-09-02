package workbench

// SI-168's structural asset budget: the ASD workbench's route-scoped
// JavaScript asset must not exceed 64 KiB uncompressed. A deterministic
// merge gate, never a wall-clock threshold.

import (
	"os"
	"testing"
)

func TestBoardSpecASDAssetBudget(t *testing.T) {
	const ceiling = 64 * 1024
	data, err := os.ReadFile("assets/boardspecasd.js")
	if err != nil {
		t.Fatal(err)
	}
	if len(data) > ceiling {
		t.Fatalf("assets/boardspecasd.js is %d bytes, over the %d-byte SI-168 ceiling", len(data), ceiling)
	}
	if len(data) == 0 {
		t.Fatal("assets/boardspecasd.js is empty")
	}
}
