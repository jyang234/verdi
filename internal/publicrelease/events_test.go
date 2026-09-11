package publicrelease

import (
	"strings"
	"testing"
)

func TestTestEventsRequireActualCompleteNamedPass(t *testing.T) {
	good := `{"Action":"start","Package":"p"}
{"Action":"run","Package":"p","Test":"TestA"}
{"Action":"run","Package":"p","Test":"TestA/negative"}
{"Action":"pass","Package":"p","Test":"TestA/negative"}
{"Action":"pass","Package":"p","Test":"TestA"}
{"Action":"pass","Package":"p"}
`
	for _, tc := range []struct {
		name, input string
		exit        int
		required    []string
		bad         bool
	}{
		{"complete", good, 0, []string{"TestA", "TestA/negative"}, false},
		{"exit despite pass", good, 1, []string{"TestA"}, true},
		{"missing root", good, 0, []string{"TestMissing"}, true},
		{"missing child", good, 0, []string{"TestA/absent"}, true},
		{"skip", strings.Replace(good, `"pass","Package":"p","Test":"TestA/negative"`, `"skip","Package":"p","Test":"TestA/negative"`, 1), 0, []string{"TestA"}, true},
		{"truncated", strings.TrimSuffix(good, "{\"Action\":\"pass\",\"Package\":\"p\"}\n"), 0, []string{"TestA"}, true},
		{"terminal without run", `{"Action":"pass","Package":"p","Test":"TestA"}`, 0, []string{"TestA"}, true},
		{"unknown event", `{"Action":"invented","Package":"p"}`, 0, []string{"TestA"}, true},
		{"malformed", `{`, 0, []string{"TestA"}, true},
		{"supplied success", `{"pass":true}`, 0, []string{"TestA"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := readTestEvents(strings.NewReader(tc.input), tc.exit, "p", tc.required)
			if (err != nil) != tc.bad {
				t.Fatalf("error=%v want bad=%v", err, tc.bad)
			}
		})
	}
}
