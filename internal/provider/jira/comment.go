package jira

import (
	"fmt"

	"github.com/OWNER/verdi/internal/provider"
)

// ciLink reads a link to the current MR/pipeline from CI-provided
// environment variables (04 §Jira adapter: "a link to the MR/pipeline"),
// preferring GitLab's merge-request URL, then GitLab's pipeline URL, then
// GitHub Actions' workflow-run URL, so whichever forge's CI runs this
// still gets a usable link. Returns "" when none of these are present —
// the comment then simply omits the link line rather than inventing one.
func ciLink(getenv func(string) string) string {
	if projectURL := getenv("CI_PROJECT_URL"); projectURL != "" {
		if iid := getenv("CI_MERGE_REQUEST_IID"); iid != "" {
			return projectURL + "/-/merge_requests/" + iid
		}
	}
	if u := getenv("CI_PIPELINE_URL"); u != "" {
		return u
	}
	if server, repo, run := getenv("GITHUB_SERVER_URL"), getenv("GITHUB_REPOSITORY"), getenv("GITHUB_RUN_ID"); server != "" && repo != "" && run != "" {
		return server + "/" + repo + "/actions/runs/" + run
	}
	return ""
}

// renderCommentLines renders the human comment's content (04 §Jira
// adapter: "the criteria table plus a link to the MR/pipeline"): a summary
// line, one line per AC, and the CI link when present. Blank lines are
// paragraph separators for buildCommentADF.
func renderCommentLines(r provider.Rollup, getenv func(string) string) []string {
	lines := []string{
		fmt.Sprintf("Rollup published for commit %s (eligible=%t)", r.Commit, r.Eligible),
		"",
	}
	for _, c := range r.Criteria {
		line := fmt.Sprintf("%s: %s", c.ID, c.Status)
		if c.Summary != "" {
			line += " — " + c.Summary
		}
		lines = append(lines, line)
	}
	if link := ciLink(getenv); link != "" {
		lines = append(lines, "", "MR/pipeline: "+link)
	}
	return lines
}

// buildCommentADF wraps renderCommentLines's content in a minimal, valid
// Atlassian Document Format document (Jira Cloud API v3 comment bodies are
// ADF, not plain strings): one paragraph per non-empty line.
func buildCommentADF(r provider.Rollup, getenv func(string) string) map[string]interface{} {
	var content []interface{}
	for _, line := range renderCommentLines(r, getenv) {
		if line == "" {
			continue
		}
		content = append(content, map[string]interface{}{
			"type": "paragraph",
			"content": []interface{}{
				map[string]interface{}{"type": "text", "text": line},
			},
		})
	}
	return map[string]interface{}{
		"type":    "doc",
		"version": 1,
		"content": content,
	}
}
