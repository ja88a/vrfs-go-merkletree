package rservice

import (
	"context"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/ja88a/vrfs-go-merkletree/libs/protos/v1/fileserver"
)

type FTService struct {
	addr      string
	batchSize int
	client    pb.FileServiceClient
}

func NewFileTransfer(addr string, batchSize int) *FTService {
	return &FTService{
		addr:      addr,
		batchSize: batchSize,
	}
}

func (s *FTService) SendFile(bucketId string, filePath string) error {
	log.Println(s.addr, filePath)
	conn, err := grpc.Dial(s.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()

	s.client = pb.NewFileServiceClient(conn)
	interrupt := make(chan os.Signal, 1)
	shutdownSignals := []os.Signal{
		os.Interrupt,
		syscall.SIGTERM,
		syscall.SIGINT,
		syscall.SIGQUIT,
	}

	signal.Notify(interrupt, shutdownSignals...)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func(s *FTService) {
		if err = s.upload(ctx, cancel, bucketId, filePath); err != nil {
			log.Fatal(err)
			cancel()
		}
	}(s)

	select {
	case killSignal := <-interrupt:
		log.Println("Got ", killSignal)
		cancel()
	case <-ctx.Done():
	}
	return nil
}

func (s *FTService) upload(ctx context.Context, cancel context.CancelFunc, bucketId string, filePath string) error {
	stream, err := s.client.Upload(ctx)
	if err != nil {
		return err
	}
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}

	fileName := filepath.Base(filePath)

	// Loop over the file data per the specified max batch size
	buf := make([]byte, s.batchSize)
	batchNumber := 1
	for {
		num, err := file.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		chunk := buf[:num]

		// Send the file batch over gRPC
		if err := stream.Send(&pb.FileUploadRequest{BucketId: bucketId, FileName: fileName, Chunk: chunk}); err != nil {
			return err
		}
		log.Printf("Sent - batch #%v - size - %v\n", batchNumber, len(chunk))
		batchNumber += 1
	}

	res, err := stream.CloseAndRecv()
	if err != nil {
		return err
	}
	log.Printf("Sent - %v bytes - %s\n", res.GetSize(), res.GetFileName())

	cancel()
	return nil
}
