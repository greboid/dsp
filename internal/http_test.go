package internal

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// serve routes one request through a router built with opts and reports the status and whether the socket was called.
func serve(t *testing.T, opts Options, method, path string) (int, bool) {
	t.Helper()
	called := false
	p := newTestProxy(t, opts, func(http.ResponseWriter, *http.Request) { called = true })
	w := httptest.NewRecorder()
	newRouter(p).ServeHTTP(w, httptest.NewRequest(method, path, nil))
	return w.Code, called
}

func TestRouter(t *testing.T) {
	tests := []struct {
		request string
		allowBy Options
	}{
		{"POST /containers/prune", Options{AllowPrune: true}},
		{"POST /v1.47/images/prune", Options{AllowPrune: true}},
		{"POST /volumes/prune", Options{AllowPrune: true}},
		{"POST /containers/abc/exec", Options{AllowExec: true}},
		{"POST /v1.47/exec/abc/start", Options{AllowExec: true}},
		{"POST /exec/abc/resize", Options{AllowExec: true}},
		{"POST /v1.47/containers/abc/attach", Options{AllowExec: true}},
		{"POST /containers/create", Options{AllowCreate: true}},
		{"POST /v1.47/containers/abc/start", Options{AllowCreate: true}},
		{"POST /containers/abc/wait", Options{AllowCreate: true}},
		{"DELETE /containers/abc", Options{AllowCreate: true}},
		{"DELETE /v1.47/containers/abc", Options{AllowCreate: true}},
		{"POST /images/create", Options{AllowPulls: true}},
		{"DELETE /images/abc", Options{}},
		{"PUT /containers/abc/archive", Options{}},
		{"POST /containers/abc/stop", Options{}},
	}
	for _, tt := range tests {
		t.Run(tt.request, func(t *testing.T) {
			method, path, _ := strings.Cut(tt.request, " ")

			code, called := serve(t, Options{}, method, path)
			assert.Equal(t, http.StatusForbidden, code, "denied by default")
			assert.False(t, called)

			code, called = serve(t, Options{AllowAll: true}, method, path)
			assert.Equal(t, http.StatusOK, code, "allowed by allowAll")
			assert.True(t, called)

			if tt.allowBy != (Options{}) {
				code, called = serve(t, tt.allowBy, method, path)
				assert.Equal(t, http.StatusOK, code, "allowed by flag")
				assert.True(t, called)
			}
		})
	}
}

func TestRouter_GetPassesThrough(t *testing.T) {
	code, called := serve(t, Options{}, "GET", "/v1.47/containers/abc/archive?path=/data")
	assert.Equal(t, http.StatusOK, code)
	assert.True(t, called)
}

func TestRouter_KillAllowAll(t *testing.T) {
	code, called := serve(t, Options{AllowAll: true}, "POST", "/containers/abc/kill?signal=KILL")
	assert.Equal(t, http.StatusOK, code)
	assert.True(t, called)
}
