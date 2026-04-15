package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	agentapp "github.com/alexisjcarr/scm/internal/agent/app"
	"github.com/alexisjcarr/scm/internal/platform/config"
)

func main() {
	configPath := flag.String("config", "/etc/scm/scmctld-agent.yaml", "path to scmctld-agent config")
	flag.Parse()

	cfg, err := config.LoadAgentConfig(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := agentapp.RunDaemon(ctx, cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
