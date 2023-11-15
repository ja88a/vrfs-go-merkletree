package service

import (
	"context"

	pb "github.com/ja88a/vrfs-go-merkletree/libs/protos/v1/vrfs"
	config "github.com/ja88a/vrfs-go-merkletree/libs/utils/config"
	"github.com/ja88a/vrfs-go-merkletree/libs/utils/logger"
)

type VerifiableRemoteFileStorageServer struct {
	pb.UnimplementedVerifiableRemoteFileStorageServer
	l   *logger.Logger
	cfg *config.Config
}

func New(l *logger.Logger, cfg *config.Config) *VerifiableRemoteFileStorageServer {
	return &VerifiableRemoteFileStorageServer{
		l:   l,
		cfg: cfg,
	}
}

func (g *VerifiableRemoteFileStorageServer) UploadBucket(ctx context.Context, in *pb.UploadBucketRequest) (*pb.UploadBucketResponse, error) {
	g.l.Debug("Received bucket req from '%d' for fileset '%v'", in.UserId, in.FilesetId)
	return &pb.UploadBucketResponse{Status: 0, BucketId: in.UserId + "_" + in.FilesetId}, nil
}

// SayHello implements vrfs.GreeterServer
func (g *VerifiableRemoteFileStorageServer) SayHello(ctx context.Context, in *pb.HelloRequest) (*pb.HelloReply, error) {
	g.l.Debug("Received: %v", in.GetName())
	return &pb.HelloReply{Message: "Hello " + in.GetName()}, nil
}
