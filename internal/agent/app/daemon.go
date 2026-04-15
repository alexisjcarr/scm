package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	agentinfra "github.com/alexisjcarr/scm/internal/agent/infra"
	agentruntime "github.com/alexisjcarr/scm/internal/agent/runtime"
	"github.com/alexisjcarr/scm/internal/platform/config"
	"github.com/alexisjcarr/scm/internal/platform/grpcutil"
	"github.com/alexisjcarr/scm/internal/platform/logging"
	platformmetrics "github.com/alexisjcarr/scm/internal/platform/metrics"
	"github.com/alexisjcarr/scm/internal/version"
	scmv1 "github.com/alexisjcarr/scm/pkg/api/scm/v1"
	"google.golang.org/grpc"
)

// RunDaemon assembles and runs scmctld-agent until the context is cancelled.
func RunDaemon(ctx context.Context, cfg config.AgentConfig) error {
	if err := config.EnsureParentDir(cfg.StateDatabasePath); err != nil {
		return fmt.Errorf("prepare state database path: %w", err)
	}

	logger := logging.New(logging.Options{Level: cfg.LogLevel, JSON: cfg.LogJSON})
	logger.Info("starting scmctld-agent", "version", version.Version, "commit", version.Commit)

	conn, err := grpc.Dial(cfg.ControlPlaneAddress, grpcutil.DialOptions()...)
	if err != nil {
		return fmt.Errorf("dial control plane: %w", err)
	}
	defer conn.Close()

	repo, err := agentinfra.NewSQLiteRepository(cfg.StateDatabasePath)
	if err != nil {
		return fmt.Errorf("open local repository: %w", err)
	}
	defer repo.Close()

	reg := platformmetrics.NewRegistry()
	agentMetrics := platformmetrics.NewAgentMetrics(reg)
	client := scmv1.NewAgentServiceClient(conn)
	runner := agentruntime.NewRunner(repo, agentinfra.NewLinuxBackend(), agentMetrics)
	service := NewService(client, runner, logger, agentMetrics, cfg.AgentID, cfg.HostID, version.Version, cfg.Labels, []string{"packages", "files", "services"}, cfg.ManifestCacheDir)
	if err := service.Register(context.Background()); err != nil {
		return fmt.Errorf("register agent: %w", err)
	}

	httpServer := &http.Server{
		Addr:              cfg.MetricsListenAddress,
		Handler:           NewDiagnosticsHandler(reg, service),
		ReadHeaderTimeout: 5 * time.Second,
	}
	serverErr := make(chan error, 1)
	go func() {
		logger.Info("serving agent diagnostics", "addr", cfg.MetricsListenAddress)
		err := httpServer.ListenAndServe()
		if err == http.ErrServerClosed {
			err = nil
		}
		serverErr <- err
	}()

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
			return nil
		case err := <-serverErr:
			return err
		case <-ticker.C:
			runCtx, cancel := context.WithTimeout(ctx, cfg.PollInterval)
			if err := service.RunOnce(runCtx); err != nil {
				logger.Error("agent loop failed", "error", err)
			}
			cancel()
		}
	}
}
