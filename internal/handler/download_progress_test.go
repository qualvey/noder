package handler

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStreamDownloadProgressSendsCurrentState(t *testing.T) {
	const id = "progress-test-id"
	publishDownloadProgress(id, 73, "正在生成", true)
	request := httptest.NewRequest("GET", "/dl/1?progress_id="+id, nil)
	response := httptest.NewRecorder()
	streamDownloadProgress(response, request, id)

	body := response.Body.String()
	if response.Code != 200 || !strings.Contains(body, `"percent":73`) || !strings.Contains(body, `"stage":"正在生成"`) || !strings.Contains(body, `"done":true`) {
		t.Fatalf("unexpected progress response: status=%d body=%s", response.Code, body)
	}
}

func TestProgressIDValidation(t *testing.T) {
	for _, test := range []struct {
		id    string
		valid bool
	}{
		{id: "abc-123", valid: true},
		{id: "", valid: false},
		{id: "bad/id", valid: false},
		{id: strings.Repeat("a", 65), valid: false},
	} {
		if got := validProgressID(test.id); got != test.valid {
			t.Errorf("validProgressID(%q) = %v, want %v", test.id, got, test.valid)
		}
	}
}
