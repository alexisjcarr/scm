package grpcutil

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// NewServer returns a gRPC server with shared interceptors and the project
// codec registered.
func NewServer(logger *slog.Logger) *grpc.Server {
	codec := JSONCodec{}
	return grpc.NewServer(
		grpc.ForceServerCodec(codec),
		grpc.ChainUnaryInterceptor(unaryLoggingInterceptor(logger)),
	)
}

// DialOptions returns the default client options for this repository's gRPC transport.
func DialOptions() []grpc.DialOption {
	codec := JSONCodec{}
	return []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(codec)),
	}
}

func unaryLoggingInterceptor(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		if err != nil {
			logger.Error("grpc request failed", "method", info.FullMethod, "duration", time.Since(start), "error", err)
			return resp, err
		}
		logger.Debug("grpc request completed", "method", info.FullMethod, "duration", time.Since(start))
		return resp, nil
	}
}
