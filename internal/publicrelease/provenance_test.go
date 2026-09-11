package publicrelease

import (
	"github.com/jyang234/verdi/internal/artifact"
	"strings"
	"testing"
)

func TestWorkflowContextDoesNotPromoteLocalExecution(t *testing.T) {
	env := map[string]string{"GITHUB_ACTIONS": "true", "GITHUB_JOB": Job, "GITHUB_EVENT_NAME": "workflow_dispatch", "GITHUB_RUN_ID": "123", "GITHUB_RUN_ATTEMPT": "1", "PUBLIC_RELEASE_VERDI_COMMIT": "commit", "GITHUB_SHA": strings.Repeat("a", 40), "GITHUB_WORKFLOW_REF": "repo/workflow@ref"}
	get := func(k string) string { return env[k] }
	local, err := provenance(false, "commit", get)
	if err != nil || local.Source != artifact.SourceLocal {
		t.Fatalf("local: %+v %v", local, err)
	}
	ci, err := provenance(true, "commit", get)
	if err != nil || ci.Source != artifact.SourceCI || ci.Job != Job || ci.Authenticity == "" {
		t.Fatalf("workflow: %+v %v", ci, err)
	}
	for key := range env {
		t.Run(key, func(t *testing.T) {
			old := env[key]
			env[key] = ""
			defer func() { env[key] = old }()
			if _, err := provenance(true, "commit", get); err == nil {
				t.Fatal("accepted misbound workflow")
			}
		})
	}
}
