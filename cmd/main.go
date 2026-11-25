package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/csmith/envflag/v2"
	"github.com/csmith/slogflags"
	"github.com/greboid/dsp/internal"
)

var (
	realSock               = flag.String("socket", "/var/run/docker.sock", "Docker socket")
	proxyPort              = flag.Int("proxyPort", 8080, "Proxy port")
	permissibleKillSignals = flag.String("killSignals", "HUP", "Space separated list of permissible kill signals, if blank none are allowed")
	allowPulls             = flag.Bool("allowPulls", false, "Allow pulling images (POST /images/create)")
	allowPrune             = flag.Bool("allowPrune", false, "Allow pruning containers, images, networks, volumes and build cache")
	allowExec              = flag.Bool("allowExec", false, "Allow running commands in and attaching to containers")
	allowCreate            = flag.Bool("allowCreate", false, "Allow creating, starting, waiting on and deleting containers")
	allowAll               = flag.Bool("allowAll", false, "Allow all requests, equivalent to full socket access")
)

func main() {
	envflag.Parse()
	slogflags.Logger(slogflags.WithSetDefault(true))

	if _, err := os.Stat(*realSock); err != nil {
		slog.Error("unable to access socket", "socket", *realSock, "error", err)
		os.Exit(1)
	}
	if *allowAll {
		slog.Warn("allowAll is enabled, all requests will be passed to the socket")
	}

	p := internal.NewProxy(internal.Options{
		KillSignals: *permissibleKillSignals,
		AllowPulls:  *allowPulls,
		AllowPrune:  *allowPrune,
		AllowExec:   *allowExec,
		AllowCreate: *allowCreate,
		AllowAll:    *allowAll,
	}, *realSock, nil)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := internal.NewServer(p, *proxyPort).Run(ctx); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
	slog.Info("Exiting")
}
