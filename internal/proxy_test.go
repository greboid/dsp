package internal

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestProxy returns a proxy whose socket is replaced by handler.
func newTestProxy(t *testing.T, opts Options, handler http.HandlerFunc) *Proxy {
	t.Helper()
	backend := httptest.NewServer(handler)
	t.Cleanup(backend.Close)
	transport := &http.Transport{
		Proxy: func(*http.Request) (*url.URL, error) {
			return url.Parse(backend.URL)
		},
	}
	return NewProxy(opts, "/mock/socket", transport)
}

func failIfCalled(t *testing.T) http.HandlerFunc {
	return func(http.ResponseWriter, *http.Request) {
		t.Error("socket was called, but the request should have been denied")
	}
}

func assertDenied(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	assert.Equal(t, http.StatusForbidden, w.Code)
	var response struct {
		Message string `json:"message"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&response))
	assert.Equal(t, "Access Denied", response.Message)
}

func TestNewProxy_KillSignals(t *testing.T) {
	p := NewProxy(Options{KillSignals: " HUP\tTERM "}, "/mock/socket", nil)
	assert.Equal(t, []string{"HUP", "TERM"}, p.killSignals)
}

func TestProxy_ContainerKill_AllowedSignal(t *testing.T) {
	p := newTestProxy(t, Options{KillSignals: "HUP TERM"}, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/containers/123/kill", r.URL.Path)
		assert.Equal(t, "HUP", r.URL.Query().Get("signal"))
		w.WriteHeader(http.StatusOK)
	})
	w := httptest.NewRecorder()
	p.ContainerKill(w, httptest.NewRequest("POST", "/containers/123/kill?signal=HUP", nil))
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestProxy_ContainerKill_DisallowedSignal(t *testing.T) {
	p := newTestProxy(t, Options{KillSignals: "HUP"}, failIfCalled(t))
	w := httptest.NewRecorder()
	p.ContainerKill(w, httptest.NewRequest("POST", "/containers/123/kill?signal=TERM", nil))
	assertDenied(t, w)
}

func TestProxy_ContainerKill_BlankSignalsDenyAll(t *testing.T) {
	p := newTestProxy(t, Options{}, failIfCalled(t))
	w := httptest.NewRecorder()
	p.ContainerKill(w, httptest.NewRequest("POST", "/containers/123/kill", nil))
	assertDenied(t, w)
}

func TestProxy_AccessDenied(t *testing.T) {
	p := newTestProxy(t, Options{}, failIfCalled(t))
	w := httptest.NewRecorder()
	p.AccessDenied(w, httptest.NewRequest("POST", "/", nil))
	assertDenied(t, w)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
}

func TestProxy_PassToSocket(t *testing.T) {
	p := newTestProxy(t, Options{}, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/", r.URL.Path)
		_, _ = w.Write([]byte("success"))
	})
	w := httptest.NewRecorder()
	p.PassToSocket(w, httptest.NewRequest("GET", "/", nil))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "success", w.Body.String())
}

func TestProxy_Guard(t *testing.T) {
	p := newTestProxy(t, Options{}, failIfCalled(t))
	w := httptest.NewRecorder()
	p.Guard(false)(w, httptest.NewRequest("POST", "/images/create?fromImage=alpine", nil))
	assertDenied(t, w)

	p = newTestProxy(t, Options{}, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "alpine", r.URL.Query().Get("fromImage"))
	})
	w = httptest.NewRecorder()
	p.Guard(true)(w, httptest.NewRequest("POST", "/images/create?fromImage=alpine", nil))
	assert.Equal(t, http.StatusOK, w.Code)
}
