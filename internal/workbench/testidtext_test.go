package workbench

import "regexp"

// testIDElementText returns the inner text of the FIRST element carrying
// data-testid=id in html (tags stripped, entities decoded) and how many
// elements carry that testid — enough for the exact-text assertions the
// shell tests make without a DOM.
func testIDElementText(html, id string) (string, int) {
	re := regexp.MustCompile(`data-testid="` + regexp.QuoteMeta(id) + `"[^>]*>(.*?)</`)
	matches := re.FindAllStringSubmatch(html, -1)
	if len(matches) == 0 {
		return "", 0
	}
	return visibleText(matches[0][1]), len(matches)
}
