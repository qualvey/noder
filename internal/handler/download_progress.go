package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

type downloadProgress struct {
	Percent int    `json:"percent"`
	Stage   string `json:"stage"`
	Done    bool   `json:"done"`
}

type downloadProgressState struct {
	mu      sync.Mutex
	current downloadProgress
	changed chan struct{}
}

var downloadProgressStates sync.Map

func validProgressID(id string) bool {
	if len(id) == 0 || len(id) > 64 {
		return false
	}
	for _, r := range id {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' {
			return false
		}
	}
	return true
}

func getDownloadProgress(id string) *downloadProgressState {
	if !validProgressID(id) {
		return nil
	}
	state := &downloadProgressState{
		current: downloadProgress{Percent: 0, Stage: "等待开始"},
		changed: make(chan struct{}, 1),
	}
	actual, loaded := downloadProgressStates.LoadOrStore(id, state)
	if !loaded {
		time.AfterFunc(10*time.Minute, func() {
			downloadProgressStates.Delete(id)
		})
	}
	return actual.(*downloadProgressState)
}

func publishDownloadProgress(id string, percent int, stage string, done bool) {
	state := getDownloadProgress(id)
	if state == nil {
		return
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	state.mu.Lock()
	state.current = downloadProgress{Percent: percent, Stage: stage, Done: done}
	state.mu.Unlock()
	select {
	case state.changed <- struct{}{}:
	default:
	}
}

func streamDownloadProgress(w http.ResponseWriter, r *http.Request, id string) {
	state := getDownloadProgress(id)
	if state == nil {
		http.Error(w, "invalid progress id", http.StatusBadRequest)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming is not supported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")

	writeCurrent := func() bool {
		state.mu.Lock()
		current := state.current
		state.mu.Unlock()
		payload, _ := json.Marshal(current)
		if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
			return false
		}
		flusher.Flush()
		return !current.Done
	}

	if !writeCurrent() {
		return
	}
	keepAlive := time.NewTicker(20 * time.Second)
	defer keepAlive.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-state.changed:
			if !writeCurrent() {
				return
			}
		case <-keepAlive.C:
			if _, err := fmt.Fprint(w, ": keep-alive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func progressIDFromRequest(r *http.Request) string {
	id := strings.TrimSpace(r.URL.Query().Get("progress_id"))
	if !validProgressID(id) {
		return ""
	}
	return id
}
