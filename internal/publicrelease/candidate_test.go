package publicrelease

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// This runs the actual peer's unchanged helper and actual shipped run path.
// It does not manufacture a report or treat a claimed digest as a launched file.
func TestCandidateATCBuildBindsObservedRuntime(t *testing.T) {
	peer := os.Getenv("PUBLIC_RELEASE_SOURCECHECK_ATC")
	if peer == "" {
		t.Skip("UNPROVEN: actual candidate ATC runtime binding requires PUBLIC_RELEASE_SOURCECHECK_ATC")
	}
	root := t.TempDir()
	x := processExecutor{root: root}
	ctx := context.Background()
	env := []string{"GOPROXY=off", "GOWORK=off", "GOFLAGS="}
	ordinary := filepath.Join(root, "ordinary-bin")
	trimmed := filepath.Join(root, "trimmed-bin")
	for _, row := range []struct {
		name, path string
		args       []string
	}{
		{"ordinary", ordinary, []string{"go", "build", "-o", ordinary, "."}},
		{"trimmed", trimmed, []string{"go", "build", "-trimpath", "-o", trimmed, "."}},
	} {
		out, e := x.Run(ctx, command{Dir: filepath.Join(peer, "cmd/vatc"), Name: row.name, Args: row.args, Env: env})
		if e != nil || out.Exit != 0 {
			t.Fatalf("actual %s build: %+v %v", row.name, out, e)
		}
	}
	good, e := fileDigest(ordinary)
	if e != nil {
		t.Fatal(e)
	}
	bad, e := fileDigest(trimmed)
	if e != nil {
		t.Fatal(e)
	}
	if good == bad {
		t.Fatal("ordinary and trimpath candidates did not differ")
	}
	t.Logf("actual ordinary ATC sha256:%s; trimpath ATC sha256:%s", good, bad)
	for _, tc := range []struct {
		name, want string
		reject     bool
	}{{"different-executable", bad, true}, {"bound-executable", good, false}} {
		t.Run(tc.name, func(t *testing.T) {
			lane := filepath.Join(root, tc.name)
			if e := os.Mkdir(lane, 0700); e != nil {
				t.Fatal(e)
			}
			runner := processExecutor{root: lane}
			_, err := authenticateCandidate(ctx, runner, candidateBuild("atc", peer, filepath.Join(lane, "rebuilt"), "./cmd/vatc", env), tc.want)
			if (err != nil) != tc.reject {
				t.Fatalf("preflight rejection=%v: %v", tc.reject, err)
			}
			if err != nil {
				if _, e := os.Stat(filepath.Join(lane, "runtime")); !os.IsNotExist(e) {
					t.Fatalf("runtime launched before refusal: %v", e)
				}
				t.Log("different executable refused before shipped runtime invocation")
				return
			}
			name := "TestAssemblyContractMismatchBuiltBinaryHasNoEffects"
			out, err := runTests(ctx, runner, command{Dir: peer, Name: "runtime", Args: []string{"go", "test", "-json", "-count=1", "./cmd/vatc", "-run", "^" + name + "$"}, Env: env}, "github.com/jyang234/verdi-atc/cmd/vatc", []string{name, name + "/first-run-old-contract", name + "/second-run-changed-pin"})
			if err != nil {
				t.Fatal(err)
			}
			observed, err := candidateRuntimeBuilds(out, good, []string{name}, true)
			if err != nil || len(observed) != 1 || observed[0].BinarySHA != good {
				t.Fatalf("actual runtime binding %+v: %v", observed, err)
			}
			t.Logf("actual shipped runtime bound: %+v", observed)
			if _, err = candidateRuntimeBuilds(out, bad, []string{name}, true); err == nil {
				t.Fatal("actual runtime observation accepted a different bound binary")
			}
		})
	}
}
