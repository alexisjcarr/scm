package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	agentapp "github.com/alexisjcarr/scm/internal/agent/app"
	agentinfra "github.com/alexisjcarr/scm/internal/agent/infra"
	"github.com/alexisjcarr/scm/internal/platform/config"
	"github.com/alexisjcarr/scm/internal/platform/grpcutil"
	"github.com/alexisjcarr/scm/internal/platform/logging"
	"github.com/alexisjcarr/scm/internal/platform/metrics"
	"github.com/alexisjcarr/scm/internal/version"
	scmv1 "github.com/alexisjcarr/scm/pkg/api/scm/v1"
	"google.golang.org/grpc"
)

func main() {
	configPath := flag.String("config", "/etc/scm/scmctld-agent.yaml", "path to scmctld-agent config")
	flag.Parse()

	cfg, err := config.LoadAgentConfig(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if err := config.EnsureParentDir(cfg.StateDatabasePath); err != nil {
		log.Fatalf("prepare state database path: %v", err)
	}

	logger := logging.New(logging.Options{Level: cfg.LogLevel, JSON: cfg.LogJSON})
	logger.Info("starting scmctld-agent", "version", version.Version, "commit", version.Commit)

	conn, err := grpc.Dial(cfg.ControlPlaneAddress, grpcutil.DialOptions()...)
	if err != nil {
		log.Fatalf("dial control plane: %v", err)
	}
	defer conn.Close()

	repo, err := agentinfra.NewSQLiteRepository(cfg.StateDatabasePath)
	if err != nil {
		log.Fatalf("open local repository: %v", err)
	}
	defer repo.Close()

	client := scmv1.NewAgentServiceClient(conn)
	service := agentapp.NewService(client, repo, agentinfra.LinuxBackend{}, logger, cfg.AgentID, cfg.HostID, version.Version, cfg.Labels, []string{"packages", "files", "services"}, cfg.ManifestCacheDir)
	if err := service.Register(context.Background()); err != nil {
		log.Fatalf("register agent: %v", err)
	}

	reg := metrics.NewRegistry()
	httpServer := &http.Server{
		Addr:              cfg.MetricsListenAddress,
		Handler:           metrics.Handler(reg),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		logger.Info("serving metrics", "addr", cfg.MetricsListenAddress)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("metrics server stopped", "error", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := httpServer.Shutdown(shutdownCtx); err != nil {
				logger.Error("metrics shutdown failed", "error", err)
			}
			return
		case <-ticker.C:
			runCtx, cancel := context.WithTimeout(ctx, cfg.PollInterval)
			if err := service.RunOnce(runCtx); err != nil {
				logger.Error("agent loop failed", "error", err)
			}
			cancel()
		}
	}
}
