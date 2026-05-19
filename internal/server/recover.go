package server

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

// recoverHandler wraps the next handler in a panic-recovery middleware.
// If a handler panics, the panic is logged with a stack trace and a 500
// JSON error is written to the client (when no response has been started
// yet). Without this, a panicking handler closes the connection silently
// and surfaces as an opaque EOF on the client side.
func (s *Server) recoverHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rrw := &recoverResponseWriter{ResponseWriter: w}
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			// http.ErrAbortHandler is the documented way to abort a
			// handler without logging; preserve that contract.
			if rec == http.ErrAbortHandler {
				panic(rec)
			}
			s.logError(
				r, "Panic in handler",
				slog.Any("panic", rec),
				slog.String("stack", string(debug.Stack())),
			)
			if !rrw.wroteHeader {
				jsonError(rrw, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(rrw, r)
	})
}

// recoverResponseWriter tracks whether the response has been started so
// the recovery middleware knows if it can still write a 500 error.
type recoverResponseWriter struct {
	http.ResponseWriter
	wroteHeader bool
}

func (rrw *recoverResponseWriter) WriteHeader(code int) {
	rrw.wroteHeader = true
	rrw.ResponseWriter.WriteHeader(code)
}

func (rrw *recoverResponseWriter) Write(b []byte) (int, error) {
	rrw.wroteHeader = true
	return rrw.ResponseWriter.Write(b)
}

func (rrw *recoverResponseWriter) Unwrap() http.ResponseWriter {
	return rrw.ResponseWriter
}
