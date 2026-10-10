// The index's view (spec/index-v2 ac-6; SI-366 (6), (7)): the pipeline —
// the four columns side by side — or the list, the same four sections
// restyled as rows on the same DOM, so every card renders exactly once
// whichever view is drawn. The view is chosen server-side from the
// request's query (`?view=list`), so the list view needs no JavaScript;
// the bar's toggle switches views in place with it (assets/index.js).
package workbench

import "net/url"

// indexView is the index's view — a closed enum whose zero value is the
// pipeline.
type indexView string

const (
	// viewPipeline draws the four columns side by side.
	viewPipeline indexView = "pipeline"
	// viewList draws the same four sections as rows.
	viewList indexView = "list"
)

// indexViewQuery is the query parameter that selects the view.
const indexViewQuery = "view"

// indexViewOf reads the request's view: `view=list` selects the list
// view, and anything else — the parameter absent, `pipeline`, or a value
// this enum does not name — fails closed to the pipeline, the view the
// page draws by default. The page never invents a view: the bar's toggle
// names the view actually drawn as current, whatever the query said.
func indexViewOf(q url.Values) indexView {
	if q.Get(indexViewQuery) == string(viewList) {
		return viewList
	}
	return viewPipeline
}

// href is the index address that draws v: the bare route for the
// pipeline, the route with its query for the list.
func (v indexView) href() string {
	if v == viewList {
		return "/?" + indexViewQuery + "=" + string(viewList)
	}
	return "/"
}
