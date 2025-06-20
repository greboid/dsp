package internal

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func setupTestServer(handler http.HandlerFunc) *httptest.Server {
	server := httptest.NewServer(handler)
	return server
}

func createTestTransport(server *httptest.Server) *http.Transport {
	return &http.Transport{
		Proxy: func(req *http.Request) (*url.URL, error) {
			return url.Parse(server.URL)
		},
	}
}

func TestNewProxy_Success(t *testing.T) {
	server := setupTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()
	transport := createTestTransport(server)
	proxy, err := NewProxy("HUP TERM", "/mock/socket", transport)

	assert.NoError(t, err)
	assert.NotNil(t, proxy)
	assert.Equal(t, []string{"HUP", "TERM"}, proxy.killSignals)
	assert.NotNil(t, proxy.rp)
	assert.Equal(t, transport, proxy.rp.Transport)
}

func TestProxy_ContainerKill_AllowedSignal(t *testing.T) {
	server := setupTestServer(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/containers/123/kill", r.URL.Path)
		assert.Equal(t, "HUP", r.URL.Query().Get("signal"))
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()
	proxy, err := NewProxy("HUP TERM", "/mock/socket", createTestTransport(server))
	require.NoError(t, err)

	w := httptest.NewRecorder()

	proxy.ContainerKill(w, httptest.NewRequest("POST", "/containers/123/kill?signal=HUP", nil))

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestProxy_ContainerKill_DisallowedSignal(t *testing.T) {
	server := setupTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Server handler was called, but the request should have been denied")
	})
	defer server.Close()
	proxy, err := NewProxy("HUP", "/mock/socket", createTestTransport(server))
	require.NoError(t, err)

	w := httptest.NewRecorder()

	proxy.ContainerKill(w, httptest.NewRequest("POST", "/containers/123/kill?signal=TERM", nil))
	assert.Equal(t, http.StatusForbidden, w.Code)

	var response struct {
		Message string `json:"message"`
	}
	err = json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, "Access Denied", response.Message)
}

func TestProxy_AccessDenied(t *testing.T) {
	server := setupTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Server handler was called, but AccessDenied doesn't make any requests")
	})
	defer server.Close()

	proxy, err := NewProxy("HUP", "/mock/socket", createTestTransport(server))
	require.NoError(t, err)

	w := httptest.NewRecorder()

	proxy.AccessDenied(w, httptest.NewRequest("POST", "/", nil))

	var response struct {
		Message string `json:"message"`
	}
	err = json.NewDecoder(w.Body).Decode(&response)
	assert.Equal(t, http.StatusForbidden, w.Code)
	require.NoError(t, err)
	assert.Equal(t, "Access Denied", response.Message)
}

func TestProxy_PassToSocket(t *testing.T) {
	server := setupTestServer(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	})
	defer server.Close()

	proxy, err := NewProxy("HUP", "/mock/socket", createTestTransport(server))
	require.NoError(t, err)

	w := httptest.NewRecorder()

	proxy.PassToSocket(w, httptest.NewRequest("GET", "/", nil))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "success", w.Body.String())
}
