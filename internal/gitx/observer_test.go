package gitx

import (
	"context"
	"reflect"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

type recordingObserver struct{ calls [][]string }

func (r *recordingObserver) Observe(dir string, args []string) {
	r.calls = append(r.calls, append([]string{dir}, args...))
}

func TestObserver_SeesRunAndConfigValue(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: map[string]string{"a.txt": "a\n"}, Message: "a"}})
	obs := &recordingObserver{}
	ctx := WithObserver(context.Background(), obs)
	if _, err := RevParse(ctx, repo.Dir, "HEAD"); err != nil {
		t.Fatal(err)
	}
	if _, err := ConfigValue(ctx, repo.Dir, "core.bare"); err != nil {
		t.Fatal(err)
	}
	want := [][]string{{repo.Dir, "rev-parse", "--verify", "HEAD"}, {repo.Dir, "config", "--local", "--get-all", "core.bare"}}
	if !reflect.DeepEqual(obs.calls, want) {
		t.Fatalf("observed %v, want %v", obs.calls, want)
	}
}

func TestObserver_AbsentIsNoop(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: map[string]string{"a.txt": "a\n"}, Message: "a"}})
	if _, err := RevParse(context.Background(), repo.Dir, "HEAD"); err != nil {
		t.Fatal(err)
	}
	ctx := WithObserver(context.Background(), nil)
	if _, err := RevParse(ctx, repo.Dir, "HEAD"); err != nil {
		t.Fatal(err)
	}
}
