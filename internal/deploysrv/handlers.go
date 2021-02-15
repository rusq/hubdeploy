package deploysrv

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/rusq/dlog"
)

const (
	webhooks = "webhooks/dockerhub"
	results  = webhooks + "/results"
)

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(path.Join(s.prefix, "/"), func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(stall * 2) // stall the motherfucker
		http.Error(w, "go away", http.StatusNotFound)
	})

	mux.HandleFunc(path.Join(s.prefix, webhooks)+"/", s.webhookHandler)
	if s.resultsDir != "" {
		mux.HandleFunc(path.Join(s.prefix, results)+"/", s.resultsHandler)
	}

	return mux
}

func (s *Server) webhookHandler(w http.ResponseWriter, r *http.Request) {
	var buf strings.Builder
	var wh DockerhubWebhook
	dec := json.NewDecoder(io.TeeReader(r.Body, &buf))
	if err := dec.Decode(&wh); err != nil {
		dlog.Println("invalid body: %s", buf.String())
		badRequest(w)
		return
	}

	dp, ok := s.mapping[wh.Repository.RepoName]
	if !ok || dp.Disabled {
		dlog.Printf("no deployment for repository: %q", dp.RepoName)
		http.Error(w, "no deployment for this repository", http.StatusNotAcceptable)
		return
	}
	if !tagExist(dp.Tags, wh.PushData.Tag) {
		dlog.Printf("[%s] no deployment for tag: %q", dp.RepoName, wh.PushData.Tag)
		http.Error(w, "no deployment for this tag", http.StatusNotAcceptable)
		return
	}
	s.jobs <- job{wh, dp}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(http.StatusText(http.StatusOK)))
}

// tagExist returns true if the tag is in the tags.  If the tags is empty, returns true (matches any tag).
// if the tag is empty - returns false (matches no tags).
func tagExist(tags []string, tag string) bool {
	if len(tags) == 0 {
		return true
	}
	if tag == "" {
		return false
	}
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}

func badRequest(w http.ResponseWriter, s ...string) {
	msg := http.StatusText(http.StatusBadRequest)
	if len(s) > 0 {
		msg = strings.Join(s, " ")
	}
	http.Error(w, msg, http.StatusBadRequest)
}

func (s *Server) resultsHandler(w http.ResponseWriter, r *http.Request) {
	if s.resultsDir == "" {
		time.Sleep(stall)
		http.NotFound(w, r)
		return
	}

	filename := path.Base(r.URL.Path)
	if filename == "" {
		time.Sleep(stall)
		http.NotFound(w, r)
		return
	}

	f, err := os.Open(filepath.Join(s.resultsDir, filename))
	if err != nil {
		dlog.Println(err)
		time.Sleep(stall)
		http.NotFound(w, r)
		return
	}
	defer f.Close()

	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, f); err != nil {
		dlog.Println(err)
		return
	}
}
