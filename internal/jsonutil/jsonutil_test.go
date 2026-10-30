package jsonutil

import "testing"

func TestUnmarshalSupportsCommentsWithoutChangingStrings(t *testing.T) {
	var value struct {
		URL  string `json:"url"`
		Name string `json:"name"`
	}

	data := []byte(`{
    // endpoint comment
    "url": "https://example.com/a//b",
    /* inline comment */ "name": "quoted \"//\""
}`)
	if err := Unmarshal(data, &value); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if value.URL != "https://example.com/a//b" {
		t.Fatalf("URL was changed: %q", value.URL)
	}
	if value.Name != `quoted "//"` {
		t.Fatalf("name was changed: %q", value.Name)
	}
}
