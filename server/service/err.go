package service

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (g *VerifiableRemoteFileStorageServer) logError(err error) error {
	if err != nil {
		g.l.Debug(err)
	}
	return err
}

func (g *VerifiableRemoteFileStorageServer) contextError(ctx context.Context) error {
	switch ctx.Err() {
	case context.Canceled:
		return g.logError(status.Error(codes.Canceled, "request is canceled"))
	case context.DeadlineExceeded:
		return g.logError(status.Error(codes.DeadlineExceeded, "deadline is exceeded"))
	default:
		return nil
	}
}
