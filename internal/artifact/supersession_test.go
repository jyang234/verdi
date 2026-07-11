package artifact

import "testing"

func TestSupersession_Validate_Happy(t *testing.T) {
	s := Supersession{
		Carried:         []string{"ac-1", "co-3"},
		Amended:         []SupersessionNote{{ID: "ac-2", Note: "reworded for clarity"}},
		AmendedAdvisory: []SupersessionNote{{ID: "dc-4", Note: "non-reaffirming rewording"}},
		Removed:         []SupersessionNote{{ID: "ac-5", Note: "descoped"}},
		Added:           []string{"ac-6", "ac-7"},
	}
	if err := s.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestSupersession_Validate_Negative(t *testing.T) {
	cases := []Supersession{
		{Carried: []string{"Not-Kebab"}},
		{Amended: []SupersessionNote{{ID: "ac-2", Note: ""}}},
		{Amended: []SupersessionNote{{ID: "bad id", Note: "x"}}},
		{AmendedAdvisory: []SupersessionNote{{ID: "dc-4", Note: ""}}},
		{Removed: []SupersessionNote{{ID: "ac-5", Note: ""}}},
		{Added: []string{"not valid!"}},
	}
	for i, s := range cases {
		if err := s.Validate(); err == nil {
			t.Fatalf("case %d Validate(%+v): want error, got nil", i, s)
		}
	}
}
