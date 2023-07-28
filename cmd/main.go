package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/csmith/envflag"
	"github.com/go-chi/chi/v5"
	"github.com/greboid/dsp/internal"
	"golang.org/x/exp/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	realSock               = flag.String("socket", "/var/run/docker.sock", "Docket socket")
	proxyPort              = flag.Int("proxyPort", 8080, "Proxy port")
	permissibleKillSignals = flag.String("killSignals", "HUP", "Comma separated list of permissible kill signals, if blank all are allowed")
)

func main() {
	envflag.Parse()
	var p *internal.Proxy
	var err error
	if p, err = internal.NewProxy(*permissibleKillSignals, *realSock); err != nil {
		slog.Error("socket does not exist", "socket", *realSock)
		return
	}
	router := chi.NewRouter()
	router.Post("/containers/{id}/kill", p.ContainerKill)
	router.Post("/{apiversion}/containers/{id}/kill", p.ContainerKill)
	router.Post("/*", p.AccessDenied)
	router.Get("/*", p.PassToSocket)
	server := &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%d", *proxyPort),
		Handler: router,
	}
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	shutdownCtx, shutdownRelease := context.WithTimeout(context.Background(), 2*time.Second)
	defer shutdownRelease()

	go runServer(server, sigChan)

	<-sigChan
	if err = server.Shutdown(shutdownCtx); err != nil {
		slog.Error("Shutdown error", "error", err)
	}
	slog.Info("Exiting")
}

func runServer(server *http.Server, c chan os.Signal) {
	slog.Info("Starting server", "url", fmt.Sprintf("http://0.0.0.0:%d", *proxyPort))
	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		slog.Error("Server error", "error", err)
		c <- syscall.SIGINT
	}
}
