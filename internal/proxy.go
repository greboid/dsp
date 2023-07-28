package internal

import (
	"context"
	"encoding/json"
	"errors"
	"golang.org/x/exp/slices"
	"golang.org/x/exp/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"regexp"
	"time"
)

type Proxy struct {
	killSignals []string
	rp          *httputil.ReverseProxy
}

func NewProxy(permissibleKillSignals string, realSock string) (*Proxy, error) {
	if _, err := os.Stat(realSock); errors.Is(err, os.ErrNotExist) {
		return nil, errors.New("socket does not exist")
	}
	d := net.Dialer{
		Timeout: 5 * time.Second,
	}
	return &Proxy{
		killSignals: regexp.MustCompile("\\S+").FindAllString(permissibleKillSignals, -1),
		rp: &httputil.ReverseProxy{
			Director: func(request *http.Request) {
				request.URL.Scheme = "http"
				request.URL.Host = "localhost"
			},
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					return d.DialContext(context.Background(), "unix", realSock)
				},
			},
		},
	}, nil
}

func (p *Proxy) ContainerKill(writer http.ResponseWriter, request *http.Request) {
	signal := request.URL.Query().Get("signal")
	if len(p.killSignals) > 0 && slices.Contains(p.killSignals, signal) {
		slog.Debug("Kill allowed", "url", request.URL, "signal", signal)
		p.rp.ServeHTTP(writer, request)
		return
	}
	slog.Error("Kill not allowed", "path", "url", request.URL)
	_ = json.NewEncoder(writer).Encode(struct {
		Message string `json:"message"`
	}{"Access Denied"})
}

func (p *Proxy) AccessDenied(writer http.ResponseWriter, request *http.Request) {
	slog.Error("Access denied", "path", "url", request.URL)
	_ = json.NewEncoder(writer).Encode(struct {
		Message string `json:"message"`
	}{"Access Denied"})
}

func (p *Proxy) PassToSocket(writer http.ResponseWriter, request *http.Request) {
	slog.Debug("Passing to socket", "url", request.URL)
	p.rp.ServeHTTP(writer, request)
}
