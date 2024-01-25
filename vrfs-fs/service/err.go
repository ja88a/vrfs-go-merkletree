package service

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (g *FileStorageService) logError(err error) error {
	if err != nil {
		g.l.Error(err)
	}
	return err
}

func (g *FileStorageService) contextError(ctx context.Context) error {
	switch ctx.Err() {
	case context.Canceled:
		return g.logError(status.Error(codes.Canceled, "request is canceled"))
	case context.DeadlineExceeded:
		return g.logError(status.Error(codes.DeadlineExceeded, "deadline is exceeded"))
	default:
		return nil
	}
}
