package internal

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"syscall"
	"time"
)

type Server struct {
	p          *Proxy
	port       int
	httpServer *http.Server
	signal     chan os.Signal
}

func NewServer(p *Proxy, port int, signal chan os.Signal) (*Server, error) {
	s := &Server{
		p:      p,
		port:   port,
		signal: signal,
	}
	r := s.createRouter(p)
	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf("0.0.0.0:%d", port),
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  180 * time.Second,
	}
	return s, nil
}

func (s *Server) createRouter(p *Proxy) *http.ServeMux {
	router := http.NewServeMux()
	router.HandleFunc("POST /containers/{id}/kill", p.ContainerKill)
	router.HandleFunc("POST /{apiversion}/containers/{id}/kill", p.ContainerKill)
	router.HandleFunc("GET /events", p.Events)
	router.HandleFunc("GET /{apiversion}/events", p.Events)
	router.HandleFunc("POST /", p.AccessDenied)
	router.HandleFunc("GET /", p.PassToSocket)
	return router
}

func (s *Server) Shutdown() {
	shutdownCtx, shutdownRelease := context.WithTimeout(context.Background(), 2*time.Second)
	defer shutdownRelease()
	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("Shutdown error", "error", err)
	}
}

func (s *Server) Start() {
	go func() {
		slog.Info("Starting server", "url", fmt.Sprintf("http://0.0.0.0:%d", s.port))
		if err := s.httpServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Server error", "error", err)
			s.signal <- syscall.SIGINT
		}
	}()
}
