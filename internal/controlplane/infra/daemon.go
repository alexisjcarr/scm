package infra

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	cpapp "github.com/alexisjcarr/scm/internal/controlplane/app"
	"github.com/alexisjcarr/scm/internal/platform/clock"
	"github.com/alexisjcarr/scm/internal/platform/config"
	"github.com/alexisjcarr/scm/internal/platform/grpcutil"
	"github.com/alexisjcarr/scm/internal/platform/logging"
	platformmetrics "github.com/alexisjcarr/scm/internal/platform/metrics"
	"github.com/alexisjcarr/scm/internal/version"
	scmv1 "github.com/alexisjcarr/scm/pkg/api/scm/v1"
)

// RunDaemon assembles and runs scmctld until the context is cancelled.
func RunDaemon(ctx context.Context, cfg config.ControlPlaneConfig) error {
	if err := config.EnsureParentDir(cfg.DatabasePath); err != nil {
		return fmt.Errorf("prepare database path: %w", err)
	}

	logger := logging.New(logging.Options{Level: cfg.LogLevel, JSON: cfg.LogJSON})
	logger.Info("starting scmctld", "version", version.Version, "commit", version.Commit)

	repo, err := NewSQLiteRepository(cfg.DatabasePath)
	if err != nil {
		return fmt.Errorf("open repository: %w", err)
	}
	defer repo.Close()

	reg := platformmetrics.NewRegistry()
	service := cpapp.NewService(repo, clock.RealClock{}, cfg.LeaseDuration, platformmetrics.NewControlPlaneMetrics(reg))
	grpcServer := grpcutil.NewServer(logger)
	handler := NewGRPCServer(service)
	scmv1.RegisterApplyServiceServer(grpcServer, handler)
	scmv1.RegisterAgentServiceServer(grpcServer, handler)

	lis, err := net.Listen("tcp", cfg.GRPCListenAddress)
	if err != nil {
		return fmt.Errorf("listen on gRPC address: %w", err)
	}

	uiHandler, err := NewHTTPHandler(service)
	if err != nil {
		return fmt.Errorf("build http handler: %w", err)
	}
	httpMux := http.NewServeMux()
	httpMux.Handle("/", uiHandler)
	httpMux.Handle("/metrics", platformmetrics.Handler(reg))
	httpServer := &http.Server{
		Addr:              cfg.HTTPListenAddress,
		Handler:           httpMux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErr := make(chan error, 2)
	go func() {
		logger.Info("serving gRPC", "addr", cfg.GRPCListenAddress)
		serverErr <- grpcServer.Serve(lis)
	}()
	go func() {
		logger.Info("serving http", "addr", cfg.HTTPListenAddress)
		err := httpServer.ListenAndServe()
		if err == http.ErrServerClosed {
			err = nil
		}
		serverErr <- err
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutting down scmctld")
		grpcServer.GracefulStop()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			logger.Error("http shutdown failed", "error", err)
		}
		return nil
	case err := <-serverErr:
		return err
	}
}
