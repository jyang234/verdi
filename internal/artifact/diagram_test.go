package artifact

import "testing"

func TestDecodeDiagram_Happy(t *testing.T) {
	y := "id: diagram/loansvc-topology\nkind: diagram\ntitle: LoanSvc topology\nstatus: active\nowners: [platform-team]\n"
	fm, err := DecodeDiagram([]byte(y))
	if err != nil {
		t.Fatalf("DecodeDiagram: %v", err)
	}
	if fm.Status != "active" {
		t.Fatalf("Status = %q", fm.Status)
	}
}

func TestDecodeDiagram_Negative(t *testing.T) {
	cases := map[string]string{
		"unknown status": "id: diagram/foo\nkind: diagram\ntitle: Foo\nstatus: draft\nowners: [x]\n",
		"frozen present": "id: diagram/foo\nkind: diagram\ntitle: Foo\nstatus: active\nowners: [x]\nfrozen: { at: 2026-01-01, commit: 3e91ab2 }\n",
		"missing status": "id: diagram/foo\nkind: diagram\ntitle: Foo\nowners: [x]\n",
	}
	for name, y := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeDiagram([]byte(y)); err == nil {
				t.Fatalf("DecodeDiagram(%s): want error, got nil", name)
			}
		})
	}
}
