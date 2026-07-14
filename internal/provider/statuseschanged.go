package provider

// StatusesChanged reports whether the (id, status) projection key extracts
// from a and b differ, comparing by id (order-independent) — 04
// §Semantics's "any AC status changed since the last publish" rule every
// StoryProvider adapter implements identically. An id appearing in one set
// but not the other counts as a change. The two adapters this package
// ships (fake, jira) each call it with their own projection: the fake
// compares CriterionStatus values directly, jira compares its wire-shaped
// criterionPayload — StatusesChanged owns the comparison, not the shape.
func StatusesChanged[T any](a, b []T, key func(T) (id, status string)) bool {
	statusesByID := func(cs []T) map[string]string {
		m := make(map[string]string, len(cs))
		for _, c := range cs {
			id, status := key(c)
			m[id] = status
		}
		return m
	}
	am, bm := statusesByID(a), statusesByID(b)
	if len(am) != len(bm) {
		return true
	}
	for id, st := range am {
		if bm[id] != st {
			return true
		}
	}
	return false
}
