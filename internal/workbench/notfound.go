// The disclosed 404 surface (spec/directory-home dc-5): every path the
// mux's "/" catch-all receives besides "/" itself renders as a legible
// notice page — HTTP 404 as the honest status, a human-readable body, and
// a link back to the directory — never a bare http.NotFound, never a blank
// response. The load-bearing case is the stale directory entry (ac-3): a
// design branch deleted between the directory render and the click
// resolves here (the draft-boards routing story is not yet registered, and
// its own dc-4 routes the no-ref case to this same surface once it is), so
// the page names what vanished.
package workbench

import (
	"context"
	stdhtml "html"
	"html/template"
	"net/http"
	"net/url"
	"strings"

	"github.com/jyang234/verdi/internal/refindex"
)

// renderCatchAllNotFound answers every non-"/" path that fell through to
// the "/" catch-all. When the path carries the draft-boards story's
// per-branch board grammar (/b/<branch-escaped>/board/spec/<name>) and
// that branch no longer resolves to any ref — local or remote-tracking —
// it renders the stale-entry notice naming the vanished branch (ac-3);
// every other unserved path gets the generic disclosed notice. Both are
// HTTP 404 with a working link back to the directory.
func renderCatchAllNotFound(w http.ResponseWriter, r *http.Request, root string, git refindex.GitRunner) {
	if branch, ok := draftBoardBranch(r.URL.EscapedPath()); ok {
		exists, err := designBranchExists(r.Context(), git, root, branch)
		if err == nil && !exists {
			renderStaleEntryNotice(w, branch)
			return
		}
	}
	renderPathNotFound(w, r.URL.Path)
}

// draftBoardBranch parses the draft-boards address grammar
// /b/<branch-escaped>/board/spec/<name> (draft-boards dc-1: the branch
// rides one path segment, slashes percent-encoded) out of an ESCAPED
// request path, returning the decoded branch name. It parses the grammar
// only — whether the branch resolves is the caller's question.
func draftBoardBranch(escapedPath string) (branch string, ok bool) {
	parts := strings.Split(strings.TrimPrefix(escapedPath, "/"), "/")
	if len(parts) != 5 || parts[0] != "b" || parts[2] != "board" || parts[3] != "spec" {
		return "", false
	}
	branch, err := url.PathUnescape(parts[1])
	if err != nil || branch == "" {
		return "", false
	}
	return branch, true
}

// designBranchExists reports whether branch still resolves as a local or
// remote-tracking design ref — the same two enumerations the directory
// index reads, through the same read-only port (co-1: a 404 probe mutates
// nothing).
func designBranchExists(ctx context.Context, git refindex.GitRunner, root, branch string) (bool, error) {
	local, err := git.LocalDesignBranches(ctx, root)
	if err != nil {
		return false, err
	}
	for _, b := range local {
		if b == branch {
			return true, nil
		}
	}
	remote, err := git.RemoteDesignBranches(ctx, root)
	if err != nil {
		return false, err
	}
	for _, b := range remote {
		if b == branch {
			return true, nil
		}
	}
	return false, nil
}

// renderStaleEntryNotice renders ac-3's deleted-mid-session shape: a
// rendered, disclosed notice page — HTTP 404 as the honest status, a body
// naming the branch that vanished, and a working link back to the
// directory (dc-5). Never a bare NotFound.
func renderStaleEntryNotice(w http.ResponseWriter, branch string) {
	var body strings.Builder
	body.WriteString(`<div class="error-page" role="alert" data-testid="stale-entry-notice">`)
	body.WriteString(`<p class="error-message"><strong>This entry&rsquo;s design branch is gone.</strong></p>`)
	body.WriteString(`<p>The design branch <code>`)
	body.WriteString(stdhtml.EscapeString(branch))
	body.WriteString(`</code> no longer resolves to any ref in this store &mdash; neither a local branch nor a remote-tracking one. It may have been deleted or merged after the directory page was rendered.</p>`)
	writeBackToDirectory(&body)
	body.WriteString(`</div>`)
	writeNotFoundPage(w, body.String())
}

// renderPathNotFound renders the generic disclosed 404 for any path this
// workbench serves nothing at — still a legible page with a way back,
// never a bare NotFound (dc-5's posture applied to the whole catch-all).
func renderPathNotFound(w http.ResponseWriter, path string) {
	var body strings.Builder
	body.WriteString(`<div class="error-page" role="alert" data-testid="path-not-found">`)
	body.WriteString(`<p class="error-message"><strong>No page is served at this address.</strong></p>`)
	body.WriteString(`<pre class="error-detail">`)
	body.WriteString(stdhtml.EscapeString(path))
	body.WriteString(`</pre>`)
	writeBackToDirectory(&body)
	body.WriteString(`</div>`)
	writeNotFoundPage(w, body.String())
}

func writeBackToDirectory(body *strings.Builder) {
	body.WriteString(`<p class="error-hint"><a href="/" data-testid="back-to-directory">Back to the directory</a></p>`)
}

// writeNotFoundPage writes bodyHTML through the shared shell at HTTP 404,
// mirroring errorpage.go's posture: the status stays loud and honest, only
// the body becomes legible. If the shell itself fails, it falls back to a
// bare text 404 rather than a blank success.
func writeNotFoundPage(w http.ResponseWriter, bodyHTML string) {
	out, err := renderPage(pageData{
		Title:    "Not found",
		Nav:      template.HTML(`<span class="current">not found</span>`),
		BodyHTML: template.HTML(bodyHTML),
	})
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write(out) // response body write; post-header error is unactionable
}
