package publicrelease

import (
	"fmt"
	"regexp"

	"github.com/jyang234/verdi/internal/artifact"
)

type workflowContext struct {
	Source       artifact.ProvenanceSource `json:"source"`
	Job          string                    `json:"job"`
	Run          string                    `json:"run"`
	Attempt      string                    `json:"attempt"`
	WorkflowSHA  string                    `json:"workflow_sha"`
	WorkflowRef  string                    `json:"workflow_ref"`
	Authenticity string                    `json:"authenticity"`
}

func provenance(workflow bool, commit string, getenv func(string) string) (workflowContext, error) {
	out := workflowContext{Source: artifact.SourceLocal, Job: Job, Authenticity: "Local execution is advisory. Environment values do not attest a genuine CI run."}
	if !workflow {
		return out, nil
	}
	digits := regexp.MustCompile(`^[1-9][0-9]*$`)
	if getenv("GITHUB_ACTIONS") != "true" || getenv("GITHUB_JOB") != Job || getenv("GITHUB_EVENT_NAME") != "workflow_dispatch" || !digits.MatchString(getenv("GITHUB_RUN_ID")) || !digits.MatchString(getenv("GITHUB_RUN_ATTEMPT")) || getenv("PUBLIC_RELEASE_VERDI_COMMIT") != commit {
		return out, fmt.Errorf("misbound release workflow context")
	}
	out.WorkflowSHA = getenv("GITHUB_SHA")
	out.WorkflowRef = getenv("GITHUB_WORKFLOW_REF")
	if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(out.WorkflowSHA) || out.WorkflowRef == "" {
		return out, fmt.Errorf("missing workflow source identity")
	}
	out.Source = artifact.SourceCI
	out.Run = getenv("GITHUB_RUN_ID")
	out.Attempt = getenv("GITHUB_RUN_ATTEMPT")
	out.Authenticity = "Unattested in isolation. CI authenticity requires this artifact to be obtained from the actual matching GitHub workflow run/job; local forged environment values cannot establish CI provenance."
	return out, nil
}
