package middleware

import (
"log"
"net/http"
"time"
)

type statusRecorder struct {
http.ResponseWriter
status int
}

func (r *statusRecorder) WriteHeader(status int) {
r.status = status
r.ResponseWriter.WriteHeader(status)
}

func RequestLogger(next http.Handler) http.Handler {
return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
started := time.Now()
recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
next.ServeHTTP(recorder, req)
log.Printf("method=%s path=%s status=%d duration=%s", req.Method, req.URL.Path, recorder.status, time.Since(started).Round(time.Millisecond))
})
}
