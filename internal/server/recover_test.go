package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/taigrr/crush/internal/proto"
	"github.com/stretchr/testify/require"
)

// TestRecoverHandler_PanicReturns500 verifies that a panicking handler
// surfaces as a structured 500 to the client, rather than closing the
// connection silently and producing an opaque EOF.
func TestRecoverHandler_PanicReturns500(t *testing.T) {
	t.Parallel()

	s := &Server{}
	h := s.recoverHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("kaboom")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	body, err := io.ReadAll(rec.Body)
	require.NoError(t, err)
	require.NotEmpty(t, body)

	var perr proto.Error
	require.NoError(t, json.Unmarshal(body, &perr))
	require.NotEmpty(t, perr.Message)
}

// TestRecoverHandler_NoPanicPassthrough verifies that the middleware
// does not interfere with successful responses.
func TestRecoverHandler_NoPanicPassthrough(t *testing.T) {
	t.Parallel()

	s := &Server{}
	h := s.recoverHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("ok"))
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusTeapot, rec.Code)
	require.Equal(t, "ok", rec.Body.String())
}

// TestRecoverHandler_PanicAfterWriteHeader verifies that if a handler
// panics after it has already started writing the response, the
// middleware does not attempt to overwrite the status (which would
// trigger a superfluous WriteHeader warning) but still logs and
// recovers.
func TestRecoverHandler_PanicAfterWriteHeader(t *testing.T) {
	t.Parallel()

	s := &Server{}
	h := s.recoverHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("partial"))
		panic("late panic")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	require.NotPanics(t, func() { h.ServeHTTP(rec, req) })
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "partial", rec.Body.String())
}

// TestRecoverHandler_AbortHandlerPropagates verifies that the documented
// http.ErrAbortHandler sentinel is re-panicked so the net/http server
// can handle it normally (suppress logging, close connection).
func TestRecoverHandler_AbortHandlerPropagates(t *testing.T) {
	t.Parallel()

	s := &Server{}
	h := s.recoverHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(http.ErrAbortHandler)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	require.PanicsWithValue(t, http.ErrAbortHandler, func() { h.ServeHTTP(rec, req) })
}
