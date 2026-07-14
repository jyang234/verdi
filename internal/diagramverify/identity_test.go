package diagramverify

import "testing"

func TestShortName(t *testing.T) {
	cases := []struct {
		name string
		fqn  string
		want string
	}{
		{
			name: "pointer-receiver method",
			fqn:  "(*example.com/svcfix/internal/app.Service).GetRefund",
			want: "GetRefund",
		},
		{
			name: "value-receiver method",
			fqn:  "example.com/svcfix/internal/app.Service.GetRefund",
			want: "GetRefund",
		},
		{
			name: "bare function, no receiver",
			fqn:  "example.com/svcfix/internal/handler.NewServer",
			want: "NewServer",
		},
		{
			name: "no dot at all",
			fqn:  "GetRefund",
			want: "GetRefund",
		},
		{
			name: "empty",
			fqn:  "",
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ShortName(tc.fqn); got != tc.want {
				t.Errorf("ShortName(%q) = %q, want %q", tc.fqn, got, tc.want)
			}
		})
	}
}

func TestShortNameIndex_Collision(t *testing.T) {
	fqns := []string{
		"(*example.com/svcfix/internal/app.Service).GetRefund",
		"(*example.com/svcfix/internal/handler.Server).GetRefund",
		"(*example.com/svcfix/internal/app.Service).PublishRefund",
	}
	idx := shortNameIndex(fqns)
	if len(idx["GetRefund"]) != 2 {
		t.Fatalf("idx[GetRefund] = %v, want 2 colliding fqns", idx["GetRefund"])
	}
	if len(idx["PublishRefund"]) != 1 {
		t.Fatalf("idx[PublishRefund] = %v, want 1 fqn", idx["PublishRefund"])
	}
}
