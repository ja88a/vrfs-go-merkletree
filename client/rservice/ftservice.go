package rservice

import (
	"context"
	"errors"
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
	pb "github.com/ja88a/vrfs-go-merkletree/libs/rpcapi/protos/v1/vrfs-fs"
)

// Max time to wait for the next message of a download stream. The total transfer time is not limited
const downloadIdleTimeout = 2 * time.Second

// ErrUploadInterrupted is returned when a shutdown signal stops an upload
var ErrUploadInterrupted = errors.New("upload interrupted by a shutdown signal")

// FTService is the client API for FTService
type FTService interface {
	// Local file data transfer protocol based on gRPC streaming, in chunks
	UploadFile(bucketId string, localFilePath string) error

	// Handle a file download request towards the FS server, based on a bucket ID (previously loaded) and a file index
	// Consider the file index towards a lexically sorted list order of the target files directory
	DownloadFile(bucketId string, fileIndex int) (*rpcfile.File, error)
}

// File transfer service's client and context info
type fTService struct {
	// Endpoint of the FileTransfer service (host:port)
	endpoint string

	// Max size of data chunks while transferring
	dataChunkMaxSize int

	// Max time to wait for the next message of a download stream
	idleTimeout time.Duration

	// Toogle for verbose log output
	debug bool
}

// Init the file transfer service context info
func NewFileTransfer(addr string, chunkSize int, verbose bool) FTService {
	return &fTService{
		endpoint:         addr,
		dataChunkMaxSize: chunkSize,
		idleTimeout:      downloadIdleTimeout,
		debug:            verbose,
	}
}

// Init the client gRPC connection
func (s *fTService) initClientConnection() (*grpc.ClientConn, error) {
	conn, err := grpc.Dial(s.endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to FS grpc API at '%v'\n%v", s.endpoint, err)
	}

	return conn, nil
}

// Local file data transfer protocol based on gRPC streaming, in chunks
//
// The call is synchronous and safe for concurrent use. A shutdown signal stops it with ErrUploadInterrupted.
func (s *fTService) UploadFile(bucketId string, filePath string) error {
	if s.debug {
		log.Printf("Sending file '%v' to FS bucket '%v' at '%v'", filePath, bucketId, s.endpoint)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	conn, err := s.initClientConnection()
	if err != nil {
		return fmt.Errorf("failed to init upload of file '%v' to the FileStorage service \n%w", filePath, err)
	}
	defer conn.Close()

	if err := s.upload(ctx, pb.NewFileServiceClient(conn), bucketId, filePath); err != nil {
		if ctx.Err() != nil {
			err = ErrUploadInterrupted
		}
		return fmt.Errorf("upload of file '%v' to FS bucket '%v' has failed\n%w", filePath, bucketId, err)
	}
	return nil
}

// Upload 1 file, using chunks with a max size, to the specified file storage's bucket
func (s *fTService) upload(ctx context.Context, client pb.FileServiceClient, bucketId string, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	stream, err := client.Upload(ctx)
	if err != nil {
		return err
	}

	fileName := filepath.Base(filePath)

	// Loop over the file data per the specified max batch size
	buf := make([]byte, s.dataChunkMaxSize)
	sentChunks := 0
	for {
		num, err := file.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		chunk := buf[:num]

		// Send the file chunk over gRPC
		err = stream.Send(&pb.FileUploadRequest{BucketId: bucketId, FileName: fileName, Chunk: chunk})
		if err == io.EOF {
			// The server ended the stream: its status is returned by CloseAndRecv
			break
		}
		if err != nil {
			return err
		}
		sentChunks++
		if s.debug {
			log.Printf("Sent chunk #%v of size %v\n", sentChunks, len(chunk))
		}
	}

	// An empty file still needs its metadata to reach the server
	if sentChunks == 0 {
		err := stream.Send(&pb.FileUploadRequest{BucketId: bucketId, FileName: fileName})
		if err != nil && err != io.EOF {
			return err
		}
	}

	res, err := stream.CloseAndRecv()
	if err != nil {
		return err
	}
	if s.debug {
		log.Printf("Sent %6d bytes for file %s\n", res.GetSize(), res.GetFileName())
	}
	return nil
}

// Handle a file download request towards the FS server, based on a bucket ID (previously loaded) and a file index
// Consider the file index towards a lexically sorted list order of the target files directory
func (s *fTService) DownloadFile(bucketId string, fileIndex int) (*rpcfile.File, error) {
	if s.debug {
		log.Printf("Downloading file #%d from FS bucket '%v' at '%v'", fileIndex, bucketId, s.endpoint)
	}

	conn, err := s.initClientConnection()
	if err != nil {
		return nil, fmt.Errorf("failed to init the FileStorage download service \n%w", err)
	}

	// The connection must outlive this call: it is released once the stream has been consumed
	ctx, cancel := context.WithCancel(context.Background())
	release := func() {
		cancel()
		_ = conn.Close()
	}
	idle := time.AfterFunc(s.idleTimeout, cancel)

	stream, err := pb.NewFileServiceClient(conn).Download(ctx, &pb.FileDownloadRequest{BucketId: bucketId, FileIndex: int32(fileIndex)})
	if err != nil {
		idle.Stop()
		release()
		return nil, fmt.Errorf("failed to download file #%d from FS bucket '%v' \n%w", fileIndex, bucketId, err)
	}

	md, err := stream.Header()
	if err == nil && md == nil {
		// gRPC reports a stream that failed before its headers through Recv only
		_, err = stream.Recv()
		if err == nil || err == io.EOF {
			err = errors.New("stream ended without file metadata")
		}
	}
	if err != nil {
		idle.Stop()
		release()
		return nil, fmt.Errorf("failed to retrieve download stream header for file #%d from bucket '%v'\n%w", fileIndex, bucketId, err)
	}

	pReader, pWriter := io.Pipe()
	rFile := rpcfile.NewFromMetadata(md, pReader)
	go func() {
		defer release()
		copyFromResponse(pWriter, stream, idle, s.idleTimeout)
	}()

	return rFile, nil
}

// Aggregate the chunks of a data stream to a local file.
// The idle timer only runs while waiting for the server, so that a slow reader is not penalized.
func copyFromResponse(w *io.PipeWriter, stream pb.FileService_DownloadClient, idle *time.Timer, idleTimeout time.Duration) {
	defer idle.Stop()

	message := new(pb.FileDownloadResponse)
	for {
		idle.Reset(idleTimeout)
		err := stream.RecvMsg(message)
		idle.Stop()
		if err == io.EOF {
			_ = w.Close()
			return
		}
		if err != nil {
			_ = w.CloseWithError(err)
			return
		}
		if len(message.GetChunk()) > 0 {
			if _, err := w.Write(message.Chunk); err != nil {
				return
			}
		}
		message.Chunk = message.Chunk[:0]
	}
}
