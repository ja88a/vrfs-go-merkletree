package app

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/ja88a/vrfs-go-merkletree/libs/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/ja88a/vrfs-go-merkletree/vrfs-api/service"

	config "github.com/ja88a/vrfs-go-merkletree/libs/config"
	pbvrfs "github.com/ja88a/vrfs-go-merkletree/libs/rpcapi/protos/v1/vrfs-api"
)

// Initialize the VRFS Service run by loading its config and initializing the gRPC API
func Run(cfg *config.Config) error {
	logger := logger.New(cfg.Log.Level)
	logger.Debug("Starting up the VRFS API")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Init the gRPC API service
	grpcServer := grpc.NewServer()
	reflection.Register(grpcServer)

	vrfsServer, err := service.New(logger, cfg)
	if err != nil {
		return fmt.Errorf("failed to init the VRFS service\n%w", err)
	}
	pbvrfs.RegisterVerifiableRemoteFileStorageServer(grpcServer, vrfsServer)

	listen, err := net.Listen("tcp", cfg.GRPC.Port)
	if err != nil {
		logger.Fatal(err)
	}

	interrupt := make(chan os.Signal, 1)
	shutdownSignals := []os.Signal{
		os.Interrupt,
		syscall.SIGTERM,
		syscall.SIGINT,
		syscall.SIGQUIT,
	}
	signal.Notify(interrupt, shutdownSignals...)

	go func(g *grpc.Server) {
		logger.Info("VRFS gRPC server started on port " + cfg.GRPC.Port)
		if err := g.Serve(listen); err != nil {
			logger.Fatal(err)
		}
	}(grpcServer)
	select {
	case killSignal := <-interrupt:
		logger.Warn("Received kill signal: %v", killSignal)
		cancel()
	case <-ctx.Done():
	}

	return nil
}
