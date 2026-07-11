package index

import "testing"

func TestTokenize(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"Hello, World!", []string{"hello", "world"}},
		{"adr/0001-a", []string{"adr", "0001", "a"}},
		{"", nil},
		{"   ", nil},
	}
	for _, tc := range cases {
		got := tokenize(tc.in)
		if len(got) != len(tc.want) {
			t.Fatalf("tokenize(%q) = %v, want %v", tc.in, got, tc.want)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatalf("tokenize(%q) = %v, want %v", tc.in, got, tc.want)
			}
		}
	}
}

func TestIndex_Search_Happy(t *testing.T) {
	ix, err := Build(buildSyntheticStore(t))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	// "zephyrtoken" appears only in adr/0001-a's body.
	results := ix.Search("zephyrtoken")
	if len(results) != 1 || results[0].Ref != "adr/0001-a" {
		t.Fatalf("Search(zephyrtoken) = %+v, want exactly [adr/0001-a]", results)
	}
	if results[0].Score < 1 {
		t.Fatalf("Search(zephyrtoken) score = %d, want >= 1", results[0].Score)
	}

	// "svcfix" appears in multiple entries' refs/bodies; relevance ordering
	// must be deterministic across repeated calls.
	first := ix.Search("svcfix")
	second := ix.Search("svcfix")
	if len(first) == 0 {
		t.Fatal("Search(svcfix): want at least one hit")
	}
	if len(first) != len(second) {
		t.Fatalf("Search(svcfix) not deterministic: %d hits then %d", len(first), len(second))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("Search(svcfix) not deterministic at index %d: %+v vs %+v", i, first[i], second[i])
		}
	}
	// Tie-break by score desc, then ref asc.
	for i := 1; i < len(first); i++ {
		if first[i-1].Score < first[i].Score {
			t.Fatalf("Search(svcfix) not sorted by score desc: %+v then %+v", first[i-1], first[i])
		}
		if first[i-1].Score == first[i].Score && first[i-1].Ref >= first[i].Ref {
			t.Fatalf("Search(svcfix) tie not broken by ref asc: %+v then %+v", first[i-1], first[i])
		}
	}
}

func TestIndex_Search_Negative(t *testing.T) {
	ix, err := Build(buildSyntheticStore(t))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if results := ix.Search("thistermdoesnotexistanywhereinthisfixture"); len(results) != 0 {
		t.Fatalf("Search(bogus term) = %+v, want none", results)
	}
	if results := ix.Search(""); results != nil {
		t.Fatalf("Search(\"\") = %+v, want nil", results)
	}
	if results := ix.Search("!!!"); results != nil {
		t.Fatalf("Search(no tokenizable chars) = %+v, want nil", results)
	}
}

func TestIndex_AllTokens_Happy(t *testing.T) {
	ix, err := Build(buildSyntheticStore(t))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	tokens := ix.AllTokens()
	if len(tokens) == 0 {
		t.Fatal("AllTokens: got none, want a non-empty vocabulary")
	}
	found := false
	for i, tok := range tokens {
		if tok == "zephyrtoken" {
			found = true
		}
		if i > 0 && tokens[i-1] >= tokens[i] {
			t.Fatalf("AllTokens not sorted: %q then %q", tokens[i-1], tok)
		}
	}
	if !found {
		t.Fatal(`AllTokens: "zephyrtoken" missing from vocabulary`)
	}
}

func TestIndex_Postings_Happy(t *testing.T) {
	ix, err := Build(buildSyntheticStore(t))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	postings := ix.Postings("zephyrtoken")
	if len(postings) != 1 || postings[0].Ref != "adr/0001-a" {
		t.Fatalf("Postings(zephyrtoken) = %+v, want exactly [adr/0001-a]", postings)
	}
	if postings[0].Score < 1 {
		t.Fatalf("Postings(zephyrtoken) score = %d, want >= 1", postings[0].Score)
	}
}

func TestIndex_Postings_Negative(t *testing.T) {
	ix, err := Build(buildSyntheticStore(t))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if postings := ix.Postings("thistermdoesnotexistanywhereinthisfixture"); postings != nil {
		t.Fatalf("Postings(bogus term) = %+v, want nil", postings)
	}
}
