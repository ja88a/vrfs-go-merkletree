package rservice

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	rpcfile "github.com/ja88a/vrfs-go-merkletree/libs/rpcapi/file"
	pb "github.com/ja88a/vrfs-go-merkletree/libs/rpcapi/protos/v1/fileserver"
)

// File transfer service's client and context info
type FTService struct {
	// Protobuf client
	client pb.FileServiceClient

	// connection URL of the FileTransfer service
	addr string

	// Max size of data chunks while transferring
	chunkSize int

	// Toogle for verbose log output
	debug bool
}

// Init the file transfer service context info
func NewFileTransfer(addr string, chunkSize int, verbose bool) *FTService {
	return &FTService{
		addr:      addr,
		chunkSize: chunkSize,
		debug:     verbose,
	}
}

// Upload 1 file, using chunks with a max size, to the specified file storage's bucket
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
	buf := make([]byte, s.chunkSize)
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
		if s.debug {
			log.Printf("Sent chunk #%v of size %v\n", batchNumber, len(chunk))
		}
		batchNumber += 1
	}

	res, err := stream.CloseAndRecv()
	if err != nil {
		return err
	}
	if s.debug {
		log.Printf("Sent %6d bytes for file %s\n", res.GetSize(), res.GetFileName())
	}
	cancel()
	return nil
}

func (s *FTService) initClientConnection() (*grpc.ClientConn, error) {
	conn, err := grpc.Dial(s.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to FS grpc API at '%v'\n%v", s.addr, err)
	}

	s.client = pb.NewFileServiceClient(conn)

	return conn, nil
}

// Local file data transfer protocol based on gRPC streaming, in chunks
func (s *FTService) SendFile(bucketId string, filePath string) error {
	if s.debug {
		log.Printf("Sending file '%v' to FS bucket '%v' at '%v'", filePath, bucketId, s.addr)
	}

	interrupt := make(chan os.Signal, 1)
	shutdownSignals := []os.Signal{
		os.Interrupt,
		syscall.SIGTERM,
		syscall.SIGINT,
		syscall.SIGQUIT,
	}

	signal.Notify(interrupt, shutdownSignals...)

	conn, err := s.initClientConnection()
	if err != nil {
		return fmt.Errorf("failed to init upload of file '%v' to the FileStorage service \n%w", filePath, err)
	}
	defer conn.Close()

	s.client = pb.NewFileServiceClient(conn)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func(s *FTService) {
		if err := s.upload(ctx, cancel, bucketId, filePath); err != nil {
			log.Fatalf("Upload of file '%v' to FS bucket '%v' has failed\n%v", filePath, bucketId, err)
			cancel()
		}
	}(s)

	select {
	case killSignal := <-interrupt:
		log.Printf("Kill signal received (%v)\n", killSignal)
		cancel()
	case <-ctx.Done():
	}
	return nil
}

// Handle a file download request towards the FS server, based on a bucket ID (previously loaded) and a file index
// Consider the file index towards a lexically sorted list order of the target files directory
func (s *FTService) DownloadFile(bucketId string, fileIndex int) (*rpcfile.File, error) {
	if s.debug {
		log.Printf("Downloading file #%d from FS bucket '%v' at '%v'", fileIndex, bucketId, s.addr)
	}

	conn, err := s.initClientConnection()
	if err != nil {
		return nil, fmt.Errorf("failed to init the FileStorage download service \n%w", err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(2)*time.Second)
	defer cancel()

	stream, err := s.client.Download(ctx, &pb.FileDownloadRequest{BucketId: bucketId, FileIndex: int32(fileIndex)})
	if err != nil {
		return nil, fmt.Errorf("failed to download file #%d from FS bucket '%v' \n%w", fileIndex, bucketId, err)
	}

	md, err := stream.Header()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve download stream header for file #%d from bucket '%v'\n%w", fileIndex, bucketId, err)
	}

	pReader, pWriter := io.Pipe()
	rFile := rpcfile.NewFromMetadata(md, pReader)
	go copyFromResponse(pWriter, stream)

	return rFile, nil
}

func copyFromResponse(w *io.PipeWriter, stream pb.FileService_DownloadClient) {
	message := new(pb.FileDownloadResponse)
	var err error
	for {
		err = stream.RecvMsg(message)
		if err == io.EOF {
			_ = w.Close()
			break
		}
		if err != nil {
			_ = w.CloseWithError(err)
			break
		}
		if len(message.GetChunk()) > 0 {
			_, err = w.Write(message.Chunk)
			if err != nil {
				_ = stream.CloseSend()
				break
			}
		}
		message.Chunk = message.Chunk[:0]
	}
}
