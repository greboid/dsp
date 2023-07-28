package main

import (
	"flag"
	"github.com/csmith/envflag"
	"github.com/greboid/dsp/internal"
	"golang.org/x/exp/slog"
	"os"
	"os/signal"
	"syscall"
)

var (
	realSock               = flag.String("socket", "/var/run/docker.sock", "Docket socket")
	proxyPort              = flag.Int("proxyPort", 8080, "Proxy port")
	permissibleKillSignals = flag.String("killSignals", "HUP", "Comma separated list of permissible kill signals, if blank all are allowed")
)

func main() {
	envflag.Parse()
	var p *internal.Proxy
	var s *internal.Server
	var err error

	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, syscall.SIGINT, syscall.SIGTERM)

	if p, err = internal.NewProxy(*permissibleKillSignals, *realSock); err != nil {
		slog.Error("socket does not exist", "socket", *realSock)
		return
	}

	if s, err = internal.NewServer(p, *proxyPort, shutdownSignal); err != nil {
		slog.Error("unable to create server", "error", err)
		return
	}

	s.Start()
	<-shutdownSignal
	s.Shutdown()
	slog.Info("Exiting")
}
