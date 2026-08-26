package artifact

import "testing"

func TestRawNodeStringScalar(t *testing.T) {
	doc := func(t *testing.T, body string) *RawNode {
		t.Helper()
		var root struct {
			Actor RawNode `yaml:"actor"`
		}
		if err := DecodeStrict([]byte(body), &root); err != nil {
			t.Fatalf("fixture decode: %v", err)
		}
		return &root.Actor
	}
	if value, ok := RawNodeStringScalar(doc(t, "actor: principal/github/abc\n")); !ok || value != "principal/github/abc" {
		t.Fatalf("RawNodeStringScalar(plain string) = %q/%v, want value/true", value, ok)
	}
	for name, body := range map[string]string{
		"null":     "actor: null\n",
		"integer":  "actor: 7\n",
		"boolean":  "actor: true\n",
		"mapping":  "actor: {a: b}\n",
		"sequence": "actor: [a]\n",
	} {
		if _, ok := RawNodeStringScalar(doc(t, body)); ok {
			t.Errorf("RawNodeStringScalar(%s) = true, want false", name)
		}
	}
	if _, ok := RawNodeStringScalar(nil); ok {
		t.Errorf("RawNodeStringScalar(nil) = true, want false")
	}
}

func TestRawNodeStringMapping(t *testing.T) {
	doc := func(t *testing.T, body string) *RawNode {
		t.Helper()
		var root struct {
			Actor RawNode `yaml:"actor"`
		}
		if err := DecodeStrict([]byte(body), &root); err != nil {
			t.Fatalf("fixture decode: %v", err)
		}
		return &root.Actor
	}

	fields, ok, err := RawNodeStringMapping(doc(t, "actor:\n  a: \"1\"\n  b: two\n"))
	if err != nil || !ok || fields["a"] != "1" || fields["b"] != "two" || len(fields) != 2 {
		t.Fatalf("RawNodeStringMapping(valid) = %v/%v/%v", fields, ok, err)
	}

	if _, ok, err := RawNodeStringMapping(doc(t, "actor: scalar\n")); ok || err != nil {
		t.Fatalf("RawNodeStringMapping(scalar) = %v/%v, want not-a-mapping without error", ok, err)
	}
	if _, ok, _ := RawNodeStringMapping(nil); ok {
		t.Fatalf("RawNodeStringMapping(nil) = true, want false")
	}

	for name, body := range map[string]string{
		"null value":     "actor:\n  a: null\n",
		"integer value":  "actor:\n  a: 7\n",
		"mapping value":  "actor:\n  a: {b: c}\n",
		"sequence value": "actor:\n  a: [b]\n",
		"integer key":    "actor:\n  7: b\n",
	} {
		if _, ok, err := RawNodeStringMapping(doc(t, body)); !ok || err == nil {
			t.Errorf("RawNodeStringMapping(%s) = ok=%v err=%v, want mapping error", name, ok, err)
		}
	}
}
