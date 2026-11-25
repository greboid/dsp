package internal

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

type Server struct {
	httpServer *http.Server
}

func NewServer(p *Proxy, port int) *Server {
	return &Server{
		httpServer: &http.Server{
			Addr:              fmt.Sprintf("0.0.0.0:%d", port),
			Handler:           newRouter(p),
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       180 * time.Second,
		},
	}
}

func newRouter(p *Proxy) *http.ServeMux {
	router := http.NewServeMux()
	if p.AllowAll {
		router.HandleFunc("/", p.Stream)
		return router
	}
	// ServeMux has no optional segments, so each route is registered with and without the API version prefix.
	handle := func(method, path string, h http.HandlerFunc) {
		router.HandleFunc(method+" "+path, h)
		router.HandleFunc(method+" /{apiversion}"+path, h)
	}
	handle("POST", "/containers/{id}/kill", p.ContainerKill)
	handle("GET", "/events", p.Stream)
	handle("GET", "/containers/{id}/stats", p.Stream)
	handle("GET", "/containers/{id}/logs", p.Stream)
	handle("GET", "/containers/{id}/archive", p.Stream)
	handle("POST", "/images/create", p.Guard(p.AllowPulls))
	for _, kind := range []string{"containers", "images", "networks", "volumes", "build"} {
		handle("POST", "/"+kind+"/prune", p.Guard(p.AllowPrune))
	}
	for _, path := range []string{"/containers/{id}/attach", "/containers/{id}/exec", "/exec/{id}/start", "/exec/{id}/resize"} {
		handle("POST", path, p.Guard(p.AllowExec))
	}
	for _, path := range []string{"/containers/create", "/containers/{id}/start", "/containers/{id}/wait"} {
		handle("POST", path, p.Guard(p.AllowCreate))
	}
	handle("DELETE", "/containers/{id}", p.Guard(p.AllowCreate))
	router.HandleFunc("GET /", p.PassToSocket)
	// Any other method or path (POST, PUT, DELETE, ...) is denied.
	router.HandleFunc("/", p.AccessDenied)
	return router
}

// Run serves until ctx is cancelled, then shuts down gracefully.
func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		slog.Info("Starting server", "addr", s.httpServer.Addr)
		errCh <- s.httpServer.ListenAndServe()
	}()
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if err := <-errCh; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
