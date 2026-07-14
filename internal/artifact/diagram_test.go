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

// hex64 (64 lowercase hex chars, a syntactically well-formed sha256 digest
// body) is declared once in common_test.go and reused here.

// TestDecodeDiagram_Proposal is spec/proposal-artifact obligation
// ac-1--behavioral's table: every case it names, in order — (1) a clean
// proposed proposal with no frozen/derived_from, (2) a clean accepted
// proposal carrying frozen + a well-formed derived_from, (3)-(6) the four
// named negatives, and (7) the pre-existing incumbent fixture proving the
// class-absent branch is unaffected (already covered by
// TestDecodeDiagram_Happy/_Negative above, re-asserted here as one
// obligation-shaped table).
func TestDecodeDiagram_Proposal(t *testing.T) {
	cases := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{
			name: "1: proposed, no frozen, no derived_from",
			yaml: "id: diagram/loansvc-target\nkind: diagram\ntitle: Target\nclass: proposal\nstatus: proposed\nowners: [platform-team]\n",
		},
		{
			name: "2: accepted, frozen present, well-formed derived_from",
			yaml: "id: diagram/loansvc-target\nkind: diagram\ntitle: Target\nclass: proposal\nstatus: accepted\nowners: [platform-team]\n" +
				"frozen: { at: 2026-07-14, commit: 3e91ab2 }\n" +
				"derived_from: { ref: diagram/loansvc-topology, digest: sha256:" + hex64 + " }\n",
		},
		{
			name:    "3: class proposal, status active (incumbent-only status leaking in)",
			yaml:    "id: diagram/loansvc-target\nkind: diagram\ntitle: Target\nclass: proposal\nstatus: active\nowners: [platform-team]\n",
			wantErr: true,
		},
		{
			name:    "4: accepted missing frozen",
			yaml:    "id: diagram/loansvc-target\nkind: diagram\ntitle: Target\nclass: proposal\nstatus: accepted\nowners: [platform-team]\n",
			wantErr: true,
		},
		{
			name: "5: proposed illegally carrying frozen",
			yaml: "id: diagram/loansvc-target\nkind: diagram\ntitle: Target\nclass: proposal\nstatus: proposed\nowners: [platform-team]\n" +
				"frozen: { at: 2026-07-14, commit: 3e91ab2 }\n",
			wantErr: true,
		},
		{
			name:    "6: unknown frontmatter field on a class: proposal diagram",
			yaml:    "id: diagram/loansvc-target\nkind: diagram\ntitle: Target\nclass: proposal\nstatus: proposed\nowners: [platform-team]\nbogus: true\n",
			wantErr: true,
		},
		{
			name: "7: incumbent diagram (class absent), status active — unaffected",
			yaml: "id: diagram/loansvc-topology\nkind: diagram\ntitle: LoanSvc topology\nstatus: active\nowners: [platform-team]\n",
		},
		{
			name: "7b: incumbent diagram (class absent), status superseded — unaffected",
			yaml: "id: diagram/loansvc-topology\nkind: diagram\ntitle: LoanSvc topology\nstatus: superseded\nowners: [platform-team]\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DecodeDiagram([]byte(tc.yaml))
			if tc.wantErr && err == nil {
				t.Fatalf("DecodeDiagram: want error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("DecodeDiagram: %v", err)
			}
		})
	}
}

// TestDiagramDerivedFrom_Negative covers DiagramDerivedFrom.Validate's own
// negatives directly (missing ref, unparseable ref, missing digest) —
// deliberately NOT a malformed-digest-format or dangling-ref case, since
// both of those must decode cleanly and are VL-021's job instead (see
// diagram.go's doc comment on DiagramDerivedFrom).
func TestDiagramDerivedFrom_Negative(t *testing.T) {
	base := "id: diagram/loansvc-target\nkind: diagram\ntitle: Target\nclass: proposal\nstatus: proposed\nowners: [platform-team]\n"
	cases := map[string]string{
		"missing ref":     base + "derived_from: { digest: sha256:" + hex64 + " }\n",
		"unparseable ref": base + "derived_from: { ref: \"not a ref\", digest: sha256:" + hex64 + " }\n",
		"missing digest":  base + "derived_from: { ref: diagram/loansvc-topology }\n",
	}
	for name, y := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeDiagram([]byte(y)); err == nil {
				t.Fatalf("DecodeDiagram(%s): want error, got nil", name)
			}
		})
	}
}

// TestDiagramDerivedFrom_DanglingRefAndMalformedDigestDecodeClean proves the
// split diagram.go's doc comment promises: a structurally well-formed but
// corpus-dangling ref, and a structurally well-formed but pattern-invalid
// digest, both decode without error — VL-021 (internal/lint) is the rule
// that must catch them, not decode.
func TestDiagramDerivedFrom_DanglingRefAndMalformedDigestDecodeClean(t *testing.T) {
	base := "id: diagram/loansvc-target\nkind: diagram\ntitle: Target\nclass: proposal\nstatus: proposed\nowners: [platform-team]\n"
	cases := map[string]string{
		"dangling ref":     base + "derived_from: { ref: diagram/does-not-exist, digest: sha256:" + hex64 + " }\n",
		"malformed digest": base + "derived_from: { ref: diagram/loansvc-topology, digest: not-a-digest }\n",
	}
	for name, y := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeDiagram([]byte(y)); err != nil {
				t.Fatalf("DecodeDiagram(%s): want no error (VL-021's job, not decode's), got %v", name, err)
			}
		})
	}
}

// TestDiagramDisclosedStatus covers spec/proposal-artifact obligation
// ac-4--behavioral's full input table.
func TestDiagramDisclosedStatus(t *testing.T) {
	proposed := DiagramFrontmatter{Status: "proposed"}
	accepted := DiagramFrontmatter{Status: "accepted"}

	cases := []struct {
		name     string
		fm       DiagramFrontmatter
		residual *ResidualDiff
		want     Status
	}{
		{"proposed, nil residual -> proposed", proposed, nil, "proposed"},
		{"accepted, nil residual -> accepted", accepted, nil, "accepted"},
		{"accepted, empty residual -> realized", accepted, &ResidualDiff{}, DiagramStatusRealized},
		{"accepted, non-empty residual -> stale", accepted, &ResidualDiff{Elements: []string{"node/foo"}}, DiagramStatusStale},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DiagramDisclosedStatus(tc.fm, tc.residual)
			if got != tc.want {
				t.Fatalf("DiagramDisclosedStatus = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestDecodeDiagram_RealizedStaleRejected proves realized/stale are
// decode-rejected as authored frontmatter values under class: proposal —
// the enforcement mechanism for "never written" (ac-4/dc-3): they are
// simply absent from proposalStatuses.
func TestDecodeDiagram_RealizedStaleRejected(t *testing.T) {
	for _, status := range []string{"realized", "stale"} {
		t.Run(status, func(t *testing.T) {
			y := "id: diagram/loansvc-target\nkind: diagram\ntitle: Target\nclass: proposal\nstatus: " + status + "\nowners: [platform-team]\n"
			_, err := DecodeDiagram([]byte(y))
			if err == nil {
				t.Fatalf("DecodeDiagram(status: %s): want a %q not a known status error, got nil", status, status)
			}
		})
	}
}
