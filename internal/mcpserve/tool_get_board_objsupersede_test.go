package mcpserve

import (
	"context"
	"testing"

	"github.com/jyang234/verdi/internal/objsupersede/scenario"
)

// get_board's closed-spec object supersession fields (SI-278; design §6,
// §8): additive and omitted when empty. A reference card whose target is
// a closed spec's superseded object carries `object` — the original text
// and the default-branch view with its lines — and a decision card whose
// edges supersede a closed spec's objects carries `supersessions`, from
// the board's own tree; every other card and reference card is
// byte-for-byte what it was. The tool inventory is unchanged.

// getBoardMaps decodes a get_board result loosely, so a key's presence
// or absence is itself observable.
func getBoardMaps(t *testing.T, b *Backend, ref string) (cards, refCards map[string]map[string]any) {
	t.Helper()
	var out struct {
		Cards    []map[string]any `json:"cards"`
		RefCards []map[string]any `json:"ref_cards"`
	}
	toolResultJSON(t, b.GetBoard(context.Background(), mustArgs(t, map[string]any{"ref": ref})), &out)
	cards, refCards = map[string]map[string]any{}, map[string]map[string]any{}
	for _, c := range out.Cards {
		cards[c["id"].(string)] = c
	}
	for _, rc := range out.RefCards {
		refCards[rc["ref"].(string)] = rc
	}
	return cards, refCards
}

func neutralizeCIEnvForScenario(t *testing.T) {
	t.Helper()
	for _, key := range []string{"CI", "GITHUB_ACTIONS", "GITHUB_BASE_REF", "CI_DEFAULT_BRANCH", "CI_MERGE_REQUEST_TARGET_BRANCH_NAME"} {
		t.Setenv(key, "")
	}
}

func TestGetBoard_ClosedSpecObjectSupersession(t *testing.T) {
	neutralizeCIEnvForScenario(t)
	t.Run("after acceptance: object on the reference card, in-force view on the decision", func(t *testing.T) {
		repo := scenario.Build(t, "accepted")
		cards, refCards := getBoardMaps(t, &Backend{Root: repo.Dir}, "spec/successor")

		obj, ok := refCards["spec/closed-feature#dc-1"]["object"].(map[string]any)
		if !ok {
			t.Fatalf("ref card lacks object: %+v", refCards["spec/closed-feature#dc-1"])
		}
		if obj["text"] != "the governed records are listed newest first" {
			t.Errorf("object.text = %v", obj["text"])
		}
		sup := obj["supersession"].(map[string]any)
		if sup["state"] != "superseded" || sup["by"] != "spec/successor#dc-1" || sup["conflict"] != "conflict/successor-closed-feature" || sup["since"] != "2024-02-15" || sup["closed"] != "2024-01-10" {
			t.Errorf("supersession structured fields = %+v", sup)
		}
		lines := sup["lines"].([]any)
		if len(lines) != 2 {
			t.Fatalf("lines = %+v", lines)
		}
		since := lines[1].(map[string]any)
		if since["kind"] != "since" || since["text"] != "superseded since 2024-02-15 by spec/successor#dc-1" {
			t.Errorf("since line = %+v", since)
		}
		if link := since["links"].([]any)[0].(map[string]any); link["ref"] != "spec/successor#dc-1" || link["href"] != "#obj-dc-1" {
			t.Errorf("since link = %+v", link)
		}
		if trailing := since["trailing"].([]any)[0].(map[string]any); trailing["ref"] != "conflict/successor-closed-feature" || trailing["href"] != "/a/conflict/successor-closed-feature" {
			t.Errorf("since trailing = %+v", trailing)
		}
		for _, key := range []string{"carry", "revision", "witness", "heads", "reason", "establisher", "object", "edge"} {
			if _, present := sup[key]; present {
				t.Errorf("empty field %q reached the wire: %+v", key, sup)
			}
		}

		dc1 := cards["dc-1"]["supersessions"].([]any)
		if len(dc1) != 1 {
			t.Fatalf("dc-1 supersessions = %+v", dc1)
		}
		view := dc1[0].(map[string]any)
		if view["state"] != "in-force" || view["object"] != "spec/closed-feature#dc-1" || view["edge"] != "spec/closed-feature#dc-1" || view["establisher"] != "spec/successor" || view["conflict"] != "conflict/successor-closed-feature" {
			t.Errorf("dc-1 view = %+v", view)
		}
		if _, present := view["carried"]; present {
			t.Errorf("carried=false reached the wire: %+v", view)
		}
		edge := view["lines"].([]any)[0].(map[string]any)
		if edge["kind"] != "edge" || edge["text"] != "supersedes spec/closed-feature#dc-1" {
			t.Errorf("edge line = %+v", edge)
		}
		if link := edge["links"].([]any)[0].(map[string]any); link["href"] != "/a/spec/closed-feature#dc-1" {
			t.Errorf("edge link = %+v", link)
		}
		// Omitted when empty: a criterion card, and no `object` beyond the
		// two closed targets.
		if _, present := cards["ac-1"]["supersessions"]; present {
			t.Errorf("ac-1 carries supersessions: %+v", cards["ac-1"])
		}
		if _, ok := refCards["spec/closed-story#ac-1"]["object"]; !ok {
			t.Errorf("closed-story#ac-1 ref card lacks object: %+v", refCards["spec/closed-story#ac-1"])
		}
	})

	t.Run("not yet accepted: proposed on the decision, nothing on the reference cards", func(t *testing.T) {
		repo := scenario.Build(t, "proposed")
		cards, refCards := getBoardMaps(t, &Backend{Root: repo.Dir}, "spec/successor")
		for ref, rc := range refCards {
			if _, present := rc["object"]; present {
				t.Errorf("%s carries object before acceptance: %+v", ref, rc)
			}
		}
		view := cards["dc-1"]["supersessions"].([]any)[0].(map[string]any)
		if view["state"] != "proposed" {
			t.Errorf("dc-1 view = %+v", view)
		}
		if line := view["lines"].([]any)[0].(map[string]any); line["text"] != "proposed — supersedes spec/closed-feature#dc-1 when spec/successor is accepted" {
			t.Errorf("proposed line = %+v", line)
		}
	})

	t.Run("not established: the reason, never a supersession", func(t *testing.T) {
		repo := scenario.Build(t, "no-conflict")
		cards, refCards := getBoardMaps(t, &Backend{Root: repo.Dir}, "spec/successor")
		if _, present := refCards["spec/closed-feature#dc-1"]["object"]; present {
			t.Errorf("closed-feature#dc-1 carries object: %+v", refCards["spec/closed-feature#dc-1"])
		}
		view := cards["dc-1"]["supersessions"].([]any)[0].(map[string]any)
		if view["state"] != "not-established" || view["reason"] != "no conflict challenges spec/closed-feature#dc-1" {
			t.Errorf("dc-1 view = %+v", view)
		}
		if line := view["lines"].([]any)[0].(map[string]any); line["kind"] != "not-established" || line["text"] != "supersession not established: no conflict challenges spec/closed-feature#dc-1" {
			t.Errorf("not-established line = %+v", line)
		}
	})
}
