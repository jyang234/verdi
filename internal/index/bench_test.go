package index

import "testing"

// BenchmarkBuild is advisory (PLAN.md phase 3 test strategy: "benchmark
// asserting sub-second walk+index at fixture scale, advisory"). It is not
// a gate — `go test -bench` never fails a build — but `go test -bench=. -v`
// on this package reports ns/op for a human to eyeball against the
// sub-second budget (01 §Scale envelope).
func BenchmarkBuild(b *testing.B) {
	root := buildGoldenRepo(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Build(root); err != nil {
			b.Fatalf("Build: %v", err)
		}
	}
}
