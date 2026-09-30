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
		"downloadUrl.searchParams.set('download', '1')",
		"downloadUrl.searchParams.set('progress_id', progressId)",
		"downloadButton.href = downloadUrl.toString()",
		"downloadFrame.src = downloadUrl.toString()",
		"浏览器下载已启动",
		"style.width = percent + '%'",
	} {
		if !strings.Contains(html, expected) {
			t.Errorf("loading page does not contain %q", expected)
		}
	}
	if strings.Contains(html, "%!") {
		t.Fatalf("loading page contains an unexpanded fmt format directive")
	}
	if strings.Contains(html, "fetch(") || strings.Contains(html, "new Blob(") {
		t.Fatalf("loading page must use the browser's direct download instead of buffering a Blob")
	}
}
