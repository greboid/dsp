package internal

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"slices"
	"strings"
	"time"
)

type Options struct {
	// KillSignals is a whitespace separated list of signals permitted for container kill.
	KillSignals string
	AllowPulls  bool
	AllowPrune  bool
	AllowExec   bool
	AllowCreate bool
	AllowAll    bool
}

type Proxy struct {
	Options
	killSignals []string
	rp          *httputil.ReverseProxy
}

// NewProxy creates a proxy to the docker socket at realSock; transport overrides the socket connection if not nil.
func NewProxy(opts Options, realSock string, transport http.RoundTripper) *Proxy {
	if transport == nil {
		d := net.Dialer{Timeout: 5 * time.Second}
		transport = &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return d.DialContext(ctx, "unix", realSock)
			},
		}
	}
	return &Proxy{
		Options:     opts,
		killSignals: strings.Fields(opts.KillSignals),
		rp: &httputil.ReverseProxy{
			Rewrite: func(r *httputil.ProxyRequest) {
				r.Out.URL.Scheme = "http"
				r.Out.URL.Host = "localhost"
			},
			Transport: transport,
		},
	}
}

func (p *Proxy) ContainerKill(writer http.ResponseWriter, request *http.Request) {
	signal := request.URL.Query().Get("signal")
	if p.AllowAll || slices.Contains(p.killSignals, signal) {
		slog.Debug("Kill allowed", "url", request.URL, "signal", signal)
		p.rp.ServeHTTP(writer, request)
		return
	}
	p.AccessDenied(writer, request)
}

// Guard returns a handler that streams the request to the socket if allowed (or AllowAll is set), otherwise denies it.
func (p *Proxy) Guard(allowed bool) http.HandlerFunc {
	if allowed || p.AllowAll {
		return p.Stream
	}
	return p.AccessDenied
}

func (p *Proxy) AccessDenied(writer http.ResponseWriter, request *http.Request) {
	slog.Error("Access denied", "method", request.Method, "url", request.URL)
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(writer).Encode(struct {
		Message string `json:"message"`
	}{"Access Denied"})
}

func (p *Proxy) PassToSocket(writer http.ResponseWriter, request *http.Request) {
	slog.Debug("Passing to socket", "method", request.Method, "url", request.URL)
	p.rp.ServeHTTP(writer, request)
}

// Stream forwards the request with the server read and write timeouts removed, for long-running requests.
func (p *Proxy) Stream(writer http.ResponseWriter, request *http.Request) {
	slog.Debug("Streaming to socket", "method", request.Method, "url", request.URL)
	rc := http.NewResponseController(writer)
	if err := rc.SetReadDeadline(time.Time{}); err != nil {
		slog.Error("Failed to set read deadline", "error", err)
	}
	if err := rc.SetWriteDeadline(time.Time{}); err != nil {
		slog.Error("Failed to set write deadline", "error", err)
	}
	p.rp.ServeHTTP(writer, request)
}
