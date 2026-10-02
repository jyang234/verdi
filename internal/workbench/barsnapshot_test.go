package workbench

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

// TestWallSnapshot_CarriesThePostureFragment (SI-323 (3)): the wall's
// snapshot carries the bar's posture group rendered from the bar's facts
// as its own field — the one posture, which the region no longer carries
// — and the page's embedded revision is the snapshot's.
func TestWallSnapshot_CarriesThePostureFragment(t *testing.T) {
	root := newBoardFixture(t)
	h := newBoardTestHandler(root)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/board/spec/"+boardFixtureName+"/snapshot", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET snapshot = %d\n%s", rec.Code, rec.Body.String())
	}
	var snap asdSnapshot
	if err := artifact.DecodeStrictJSON(rec.Body.Bytes(), &snap); err != nil {
		t.Fatalf("strict-decoding the snapshot: %v", err)
	}
	if !strings.HasPrefix(snap.Posture, `<section class="topbar-posture-group" id="asd-posture"`) || !strings.HasSuffix(snap.Posture, `</section>`) {
		t.Fatalf("snapshot posture = %q, want the bar's posture group fragment", snap.Posture)
	}
	if strings.Contains(snap.HTML, `id="asd-posture"`) {
		t.Fatal("the region carries a posture of its own: the bar's group is the one posture (SI-323 (3))")
	}
	for _, want := range []string{"Accepted HEAD", "Ahead/behind", gitOut(t, root, "rev-parse", "main"), "1 ahead, 0 behind main"} {
		if !strings.Contains(snap.Posture, want) {
			t.Errorf("snapshot posture lacks %q", want)
		}
	}

	page := httptest.NewRecorder()
	h.ServeHTTP(page, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/board/spec/"+boardFixtureName, nil))
	if page.Code != http.StatusOK {
		t.Fatalf("GET page = %d", page.Code)
	}
	var state struct {
		Asd struct {
			Revision string `json:"revision"`
		} `json:"asd"`
	}
	body := page.Body.String()
	const marker = "window.__BOARDV2__ = "
	start := strings.Index(body, marker)
	end := strings.Index(body[start:], ";\n</script>")
	if start < 0 || end < 0 {
		t.Fatal("page embeds no window.__BOARDV2__ state")
	}
	if err := json.Unmarshal([]byte(body[start+len(marker):start+end]), &state); err != nil {
		t.Fatalf("decoding page state: %v", err)
	}
	if state.Asd.Revision == "" || state.Asd.Revision != snap.Revision {
		t.Fatalf("page revision %q != snapshot revision %q", state.Asd.Revision, snap.Revision)
	}
}

// TestSnapshotRevision_HashesTheBarPosture: the revision token hashes the
// bar's posture facts explicitly, beside the rendered HTML — so a later
// move of the posture out of the region cannot drop the accepted head or
// ahead/behind from what a refresh compares.
func TestSnapshotRevision_HashesTheBarPosture(t *testing.T) {
	snapWith := func(mutate func(*barPosture)) *asdSnapshot {
		bar := barFacts{Title: "T", Spec: &barSpec{Name: "s"}, Posture: barPosture{
			AcceptedHead: provenFact("aaa"), AheadBehind: provenFact("1 ahead, 0 behind main"),
		}}
		mutate(&bar.Posture)
		return &asdSnapshot{HTML: "<main>same</main>", Posture: "<section>same</section>", BaseDigest: "d", Git: &boardGitState{}, bar: bar}
	}
	base := snapshotRevision(snapWith(func(*barPosture) {}))
	if again := snapshotRevision(snapWith(func(*barPosture) {})); again != base {
		t.Fatalf("revision is not deterministic: %s vs %s", base, again)
	}
	for name, mutate := range map[string]func(*barPosture){
		"accepted head moves": func(p *barPosture) { p.AcceptedHead = provenFact("bbb") },
		"ahead/behind moves":  func(p *barPosture) { p.AheadBehind = provenFact("1 ahead, 2 behind main") },
		"accepted head lost":  func(p *barPosture) { p.AcceptedHead = unprovenFact("gone") },
	} {
		if got := snapshotRevision(snapWith(mutate)); got == base {
			t.Errorf("%s with identical HTML leaves the revision unchanged: the token does not hash the bar's posture", name)
		}
	}
}

// TestMutationResponse_CarriesThePosture (F1A-B2): a mutation response's
// fresh projection carries the posture fragment from the same snapshot
// as its revision, so a client adopting that revision also has the
// posture it covers — equal to what /snapshot serves for the same state.
func TestMutationResponse_CarriesThePosture(t *testing.T) {
	root := newBoardFixture(t)
	h := newBoardTestHandler(root)
	rec, _ := postMutate(t, h, root, boardFixtureName, []map[string]any{
		{"op": "edit-ac", "id": "ac-1", "text": "a declined applicant sees the current reason, today", "evidence": []string{"attestation"}, "anchor": "#ac-1"},
	}, nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("mutate_draft = %d\n%s", rec.Code, rec.Body.String())
	}
	var out struct {
		Projection *mutationProjection `json:"projection"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.Projection == nil {
		t.Fatalf("decoding the mutation response: %v\n%s", err, rec.Body.String())
	}
	snapRec := httptest.NewRecorder()
	h.ServeHTTP(snapRec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/board/spec/"+boardFixtureName+"/snapshot", nil))
	var snap asdSnapshot
	if err := artifact.DecodeStrictJSON(snapRec.Body.Bytes(), &snap); err != nil {
		t.Fatalf("strict-decoding the snapshot: %v", err)
	}
	if out.Projection.Posture == "" || out.Projection.Posture != snap.Posture {
		t.Fatalf("mutation posture =\n%q\n/snapshot posture =\n%q", out.Projection.Posture, snap.Posture)
	}
	if out.Projection.Revision != snap.Revision || !strings.Contains(out.Projection.Posture, `data-dirty="dirty"`) {
		t.Fatalf("mutation revision %s vs snapshot %s; posture %q, want the fresh, dirty working tree", out.Projection.Revision, snap.Revision, out.Projection.Posture)
	}
}
