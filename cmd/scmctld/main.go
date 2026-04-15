package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	cpinfra "github.com/alexisjcarr/scm/internal/controlplane/infra"
	"github.com/alexisjcarr/scm/internal/platform/config"
)

func main() {
	configPath := flag.String("config", "/etc/scm/scmctld.yaml", "path to scmctld config")
	flag.Parse()

	cfg, err := config.LoadControlPlaneConfig(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := cpinfra.RunDaemon(ctx, cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
