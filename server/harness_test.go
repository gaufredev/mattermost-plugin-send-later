package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
)

// TestHarness serves dev/harness.html with the real plugin code behind a fake Mattermost,
// so the web part can be tried in a browser without a Mattermost server:
//
//	SL_HARNESS=1 SL_REACT_DIR=/path/to/node_modules go test ./server -run TestHarness -v
//
// then open http://127.0.0.1:8765/. POST /__quit stops it.
func TestHarness(t *testing.T) {
	if os.Getenv("SL_HARNESS") == "" {
		t.Skip("set SL_HARNESS=1 to run the browser harness")
	}
	reactDir := os.Getenv("SL_REACT_DIR")
	lang := os.Getenv("SL_LANG")

	f := newFixture(t, time.Now())
	f.p.now = time.Now
	if lang == "en" {
		f.api.users[userKo].Locale = "en"
	}
	f.api.files["file1"] = &model.FileInfo{Id: "file1", CreatorId: userKo, ChannelId: chanID, Name: "회의자료.pdf"}

	root, _ := filepath.Abs("..")
	quit := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(root, "dev", "harness.html"))
	})
	mux.HandleFunc("/main.js", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(root, "webapp", "main.js"))
	})
	mux.Handle("/react/", http.StripPrefix("/react/", http.FileServer(http.Dir(reactDir))))
	mux.HandleFunc("/plugins/"+pluginID+"/", func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = strings.TrimPrefix(r.URL.Path, "/plugins/"+pluginID)
		r.Header.Set("Mattermost-User-Id", userKo)
		rec := httptest.NewRecorder()
		f.p.router.ServeHTTP(rec, r)
		if rec.Code >= 400 {
			t.Logf("%s %s → %d %s", r.Method, r.URL.Path, rec.Code, strings.TrimSpace(rec.Body.String()))
		}
		for k, v := range rec.Header() {
			w.Header()[k] = v
		}
		w.WriteHeader(rec.Code)
		_, _ = w.Write(rec.Body.Bytes())
	})
	mux.HandleFunc("/__state", func(w http.ResponseWriter, _ *http.Request) {
		f.api.mu.Lock()
		defer f.api.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{"created": f.api.created, "ephemeral": f.api.ephemeral})
	})
	mux.HandleFunc("/__file", func(w http.ResponseWriter, _ *http.Request) {
		// Gives the harness a fresh unattached file to put in the draft.
		id := model.NewId()
		f.api.mu.Lock()
		f.api.files[id] = &model.FileInfo{Id: id, CreatorId: userKo, ChannelId: chanID, Name: "회의자료.pdf"}
		f.api.mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]string{"id": id, "name": "회의자료.pdf"})
	})
	mux.HandleFunc("/__quit", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		close(quit)
	})

	// Listen before serving, so a port still held by an earlier harness fails loudly here
	// instead of letting the browser test talk to the old server.
	ln, err := net.Listen("tcp", "127.0.0.1:8765")
	if err != nil {
		t.Fatalf("port 8765 is busy (an earlier harness still running?): %v", err)
	}
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	t.Log("harness on http://127.0.0.1:8765/")
	select {
	case <-quit:
	case <-time.After(20 * time.Minute):
	}
	_ = srv.Shutdown(context.Background())
}
