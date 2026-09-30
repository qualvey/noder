package loading

import (
	"strings"
	"testing"

	"noder/internal/model"
)

func TestRenderLoadingPageIncludesProgressUIAndDownload(t *testing.T) {
	filename := "client.zip"
	html := RenderLoadingPage(&model.DistFile{OriginalName: filename}, &model.User{Name: "Alice"})
	for _, expected := range []string{
		"id=\"progress-fill\"",
		"new EventSource(progressUrl.toString())",
		"response.body.getReader()",
		"Math.min(99, 90 + Math.floor",
		"url.searchParams.set('download', '1')",
		"url.searchParams.set('progress_id', progressId)",
		"配置包生成完成！",
		"style.width = percent + '%'",
	} {
		if !strings.Contains(html, expected) {
			t.Errorf("loading page does not contain %q", expected)
		}
	}
	if strings.Contains(html, "%!") {
		t.Fatalf("loading page contains an unexpanded fmt format directive")
	}
	if strings.Index(html, "updateProgress(100") < strings.Index(html, "a.click();") {
		t.Fatalf("loading page marks completion before triggering the download")
	}
}
