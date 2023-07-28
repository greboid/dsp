package main

import (
	"flag"
	"fmt"
	"github.com/csmith/envflag"
	"github.com/go-chi/chi/v5"
	"github.com/greboid/dsp/internal"
	"log"
	"net/http"
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
	if err := http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", *proxyPort), router); err != http.ErrServerClosed {
		log.Fatalf("Failed to serve http: %v", err)
		return
	}
}
