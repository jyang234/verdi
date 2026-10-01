package strictfixture

// counter is package-level mutable state: gochecknoglobals' one finding.
var counter int

// Next increments the counter.
func Next() int {
	counter++
	return counter
}
