package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/csmith/envflag"
	"github.com/go-chi/chi/v5"
	"github.com/greboid/dsp/internal"
	"log"
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
	p := internal.NewProxy(*permissibleKillSignals, *realSock)
	router := chi.NewRouter()
	router.Post("/containers/{id}/kill", p.ContainerKill)
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

	go func() {
		log.Printf("Starting: http://0.0.0.0:%d", *proxyPort)
		if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Error serving: %v", err)
		}
	}()

	<-sigChan
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Shutdown error: %v", err)
	}
	log.Println("Exiting")
}
