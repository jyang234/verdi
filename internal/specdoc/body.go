package specdoc

import (
	"strings"
)

// bodySections splits a spec body on its level-two headings. The scaffold
// and every authored spec key object bodies as "## <id>" (02 §Object
// model anchors), so the id under a heading is the map key; a prose
// heading such as "## Problem" is keyed by its lowercased text. Text is
// trimmed of surrounding blank lines; level-three headings and below stay
// inside their section.
func bodySections(body []byte) map[string]string {
	out := map[string]string{}
	if len(body) == 0 {
		return out
	}
	lines := strings.Split(strings.ReplaceAll(string(body), "\r\n", "\n"), "\n")
	key := ""
	var buf []string
	flush := func() {
		if key != "" {
			out[key] = strings.Trim(strings.Join(buf, "\n"), "\n")
		}
		buf = buf[:0]
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			flush()
			key = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(line, "## ")))
			continue
		}
		if key != "" {
			buf = append(buf, line)
		}
	}
	flush()
	return out
}
