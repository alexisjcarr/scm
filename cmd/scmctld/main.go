package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	cpapp "github.com/alexisjcarr/scm/internal/controlplane/app"
	cpinfra "github.com/alexisjcarr/scm/internal/controlplane/infra"
	"github.com/alexisjcarr/scm/internal/platform/clock"
	"github.com/alexisjcarr/scm/internal/platform/config"
	"github.com/alexisjcarr/scm/internal/platform/grpcutil"
	"github.com/alexisjcarr/scm/internal/platform/logging"
	"github.com/alexisjcarr/scm/internal/platform/metrics"
	"github.com/alexisjcarr/scm/internal/version"
	scmv1 "github.com/alexisjcarr/scm/pkg/api/scm/v1"
)

func main() {
	configPath := flag.String("config", "/etc/scm/scmctld.yaml", "path to scmctld config")
	flag.Parse()

	cfg, err := config.LoadControlPlaneConfig(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if err := config.EnsureParentDir(cfg.DatabasePath); err != nil {
		log.Fatalf("prepare database path: %v", err)
	}

	logger := logging.New(logging.Options{Level: cfg.LogLevel, JSON: cfg.LogJSON})
	logger.Info("starting scmctld", "version", version.Version, "commit", version.Commit)

	repo, err := cpinfra.NewSQLiteRepository(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("open repository: %v", err)
	}
	defer repo.Close()

	service := cpapp.NewService(repo, clock.RealClock{}, cfg.LeaseDuration)
	grpcServer := grpcutil.NewServer(logger)
	handler := cpinfra.NewGRPCServer(service)
	scmv1.RegisterApplyServiceServer(grpcServer, handler)
	scmv1.RegisterAgentServiceServer(grpcServer, handler)

	lis, err := net.Listen("tcp", cfg.GRPCListenAddress)
	if err != nil {
		log.Fatalf("listen on gRPC address: %v", err)
	}

	reg := metrics.NewRegistry()
	uiHandler, err := cpinfra.NewHTTPHandler(service)
	if err != nil {
		log.Fatalf("build http handler: %v", err)
	}
	httpMux := http.NewServeMux()
	httpMux.Handle("/", uiHandler)
	httpMux.Handle("/metrics", metrics.Handler(reg))
	httpServer := &http.Server{
		Addr:              cfg.HTTPListenAddress,
		Handler:           httpMux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("serving gRPC", "addr", cfg.GRPCListenAddress)
		if err := grpcServer.Serve(lis); err != nil {
			logger.Error("gRPC server stopped", "error", err)
		}
	}()
	go func() {
		logger.Info("serving http", "addr", cfg.HTTPListenAddress)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server stopped", "error", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	logger.Info("shutting down scmctld")
	grpcServer.GracefulStop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("http shutdown failed", "error", err)
	}
	fmt.Println("scmctld stopped")
}
