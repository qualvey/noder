package handler

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/go-chi/chi/v5"
	"noder/internal/config"
	"noder/internal/db"
	"noder/internal/model"
	"noder/internal/service/file"
)

func TestHandleDownloadRecordsOnlyUserTokenDownloads(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "noder-download-log-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	config.Init(tmpDir)
	database, err := db.InitDB()
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer database.Close()

	user := &model.User{Name: "Download Test", Token: "download-test-user-token", IsActive: true}
	if _, err := database.NewInsert().Model(user).Exec(t.Context()); err != nil {
		t.Fatal(err)
	}
	dist := &model.DistFile{
		Name:         "download-log-test",
		FileType:     "text",
		OriginalName: "download-log-test.txt",
		StoredName:   "download-log-test.txt",
		IsActive:     true,
	}
	if _, err := database.NewInsert().Model(dist).Exec(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := file.SaveFileContent(dist, []byte("test download")); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(filepath.Join(config.FilesDir, dist.StoredName))

	router := chi.NewRouter()
	router.Get("/dl/{id}", HandleDownload)

	userRequest := httptest.NewRequest("GET", "/dl/"+formatID(dist.ID)+"?token="+user.Token, nil)
	userResponse := httptest.NewRecorder()
	router.ServeHTTP(userResponse, userRequest)
	if userResponse.Code != 200 || userResponse.Body.String() != "test download" {
		t.Fatalf("unexpected user download response: status=%d body=%q", userResponse.Code, userResponse.Body.String())
	}

	stats := loadUserDownloadStats(t.Context(), []int64{user.ID})
	if got := stats[user.ID].Count; got != 1 {
		t.Fatalf("user download count = %d, want 1", got)
	}
	if stats[user.ID].Last == nil {
		t.Fatal("expected latest download timestamp")
	}

	sharedToken := getSharedTokenFromDB(t.Context())
	sharedRequest := httptest.NewRequest("GET", "/dl/"+formatID(dist.ID)+"?token="+sharedToken, nil)
	sharedResponse := httptest.NewRecorder()
	router.ServeHTTP(sharedResponse, sharedRequest)
	if sharedResponse.Code != 200 {
		t.Fatalf("unexpected shared download response: status=%d", sharedResponse.Code)
	}

	stats = loadUserDownloadStats(t.Context(), []int64{user.ID})
	if got := stats[user.ID].Count; got != 1 {
		t.Fatalf("shared token changed user download count to %d, want 1", got)
	}
}

func formatID(id int64) string {
	return strconv.FormatInt(id, 10)
}
