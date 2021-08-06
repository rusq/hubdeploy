package deploysrv

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/rusq/dlog"
)

type statusRecorder struct {
	http.ResponseWriter
	Status int
}

func (sr *statusRecorder) WriteHeader(status int) {
	sr.Status = status
	sr.ResponseWriter.WriteHeader(status)
}

type reqID int

var reqIDkey reqID

type requestID string

func newReqIDContext(parent context.Context, id requestID) context.Context {
	return context.WithValue(parent, reqIDkey, id)
}

func reqIDFromContext(ctx context.Context) (requestID, bool) {
	id, ok := ctx.Value(reqIDkey).(requestID)
	return id, ok
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wr := &statusRecorder{w, 200}
		reqID := requestID(uuid.New().String())
		r = r.WithContext(newReqIDContext(r.Context(), reqID))
		next.ServeHTTP(wr, r)
		dlog.Printf("[%s] HTTP %s %s - %d %5dms (%s)", reqID, r.Method, r.URL.Path, wr.Status, time.Since(start).Milliseconds(), getIP(r))
	})
}

func getIP(r *http.Request) string {
	return r.RemoteAddr // for now
}
