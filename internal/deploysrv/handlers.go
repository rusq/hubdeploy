package deploysrv

import (
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/rusq/dlog"
)

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
