package lint

import "testing"

func TestReadCIEnv(t *testing.T) {
	t.Run("gitlab MR pipeline", func(t *testing.T) {
		t.Setenv("CI", "true")
		t.Setenv("CI_DEFAULT_BRANCH", "main")
		t.Setenv("CI_MERGE_REQUEST_TARGET_BRANCH_NAME", "main")
		t.Setenv("GITHUB_ACTIONS", "")
		t.Setenv("GITHUB_BASE_REF", "")

		e := ReadCIEnv()
		if !e.InCI || e.DefaultBranch != "main" || e.TargetBranch != "main" {
			t.Fatalf("got %+v, want InCI=true DefaultBranch=main TargetBranch=main", e)
		}
	})

	t.Run("github PR workflow falls back to GITHUB_BASE_REF", func(t *testing.T) {
		t.Setenv("CI", "")
		t.Setenv("CI_DEFAULT_BRANCH", "")
		t.Setenv("CI_MERGE_REQUEST_TARGET_BRANCH_NAME", "")
		t.Setenv("GITHUB_ACTIONS", "true")
		t.Setenv("GITHUB_BASE_REF", "main")

		e := ReadCIEnv()
		if !e.InCI || e.TargetBranch != "main" {
			t.Fatalf("got %+v, want InCI=true TargetBranch=main", e)
		}
	})

	t.Run("no CI environment at all", func(t *testing.T) {
		t.Setenv("CI", "")
		t.Setenv("CI_DEFAULT_BRANCH", "")
		t.Setenv("CI_MERGE_REQUEST_TARGET_BRANCH_NAME", "")
		t.Setenv("GITHUB_ACTIONS", "")
		t.Setenv("GITHUB_BASE_REF", "")

		e := ReadCIEnv()
		if e.InCI || e.DefaultBranch != "" || e.TargetBranch != "" {
			t.Fatalf("got %+v, want all zero", e)
		}
	})
}
