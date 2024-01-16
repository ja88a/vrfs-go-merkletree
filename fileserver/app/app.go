package app

import (
	"context"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/ja88a/vrfs-go-merkletree/libs/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/ja88a/vrfs-go-merkletree/fileserver/service"

	pb "github.com/ja88a/vrfs-go-merkletree/libs/rpcapi/protos/v1/fileserver"
	config "github.com/ja88a/vrfs-go-merkletree/libs/config"
)

// Run creates objects via constructors.
func Run(cfg *config.Config) {
	logger := logger.New(cfg.Log.Level)
	logger.Debug("Starting FileStorage server")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	grpcServer := grpc.NewServer()
	reflection.Register(grpcServer)
	uploadServer := service.New(logger, cfg)
	pb.RegisterFileServiceServer(grpcServer, uploadServer)
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
		logger.Info("FileStorage server started on port " + cfg.GRPC.Port)
		if err := g.Serve(listen); err != nil {
			logger.Fatal(err)
		}
	}(grpcServer)
	select {
	case killSignal := <-interrupt:
		logger.Warn("Got ", killSignal)
		cancel()
	case <-ctx.Done():
	}
}
