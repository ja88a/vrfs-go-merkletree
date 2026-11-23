package rservice

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	rpcfile "github.com/ja88a/vrfs-go-merkletree/libs/rpcapi/file"
	pb "github.com/ja88a/vrfs-go-merkletree/libs/rpcapi/protos/v1/vrfs-fs"
)

// fakeFileServer is a real gRPC file service with scripted behaviour.
type fakeFileServer struct {
	pb.UnimplementedFileServiceServer

	uploadErr    error
	holdUpload   bool
	firstChunk   chan struct{}
	firstOnce    sync.Once
	downloadData []byte
	downloadErr  error
	holdDownload bool
	chunkDelay   time.Duration

	mu       sync.Mutex
	received map[string][]byte
}

func (s *fakeFileServer) Upload(stream pb.FileService_UploadServer) error {
	var name string
	var data []byte
	for {
		req, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		name = req.GetFileName()
		data = append(data, req.GetChunk()...)
		if s.firstChunk != nil {
			s.firstOnce.Do(func() { close(s.firstChunk) })
		}
		if s.holdUpload {
			<-stream.Context().Done()
			return stream.Context().Err()
		}
	}
	if s.uploadErr != nil {
		return s.uploadErr
	}
	if name == "" {
		return status.Error(codes.InvalidArgument, "no file metadata received")
	}
	s.mu.Lock()
	if s.received == nil {
		s.received = map[string][]byte{}
	}
	s.received[name] = data
	s.mu.Unlock()
	return stream.SendAndClose(&pb.FileUploadResponse{FileName: name, Size: uint32(len(data))})
}

func (s *fakeFileServer) Download(_ *pb.FileDownloadRequest, stream pb.FileService_DownloadServer) error {
	if s.downloadErr != nil {
		return s.downloadErr
	}
	file := rpcfile.NewFile("data.bin", ".bin", len(s.downloadData), bytes.NewReader(s.downloadData))
	if err := stream.SendHeader(file.Metadata()); err != nil {
		return err
	}
	if s.holdDownload {
		<-stream.Context().Done()
		return stream.Context().Err()
	}
	const chunkSize = 4096
	for offset := 0; offset < len(s.downloadData); offset += chunkSize {
		end := offset + chunkSize
		if end > len(s.downloadData) {
			end = len(s.downloadData)
		}
		time.Sleep(s.chunkDelay)
		if err := stream.Send(&pb.FileDownloadResponse{Chunk: s.downloadData[offset:end]}); err != nil {
			return err
		}
	}
	return nil
}

func startFileServer(t *testing.T, impl *fakeFileServer) FTService {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer()
	pb.RegisterFileServiceServer(server, impl)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	return NewFileTransfer(listener.Addr().String(), 1024, false)
}

func writeTempFile(t *testing.T, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestUploadFileReturnsServerError(t *testing.T) {
	impl := &fakeFileServer{uploadErr: status.Error(codes.Internal, "disk full")}
	client := startFileServer(t, impl)

	err := client.UploadFile("bucket", writeTempFile(t, "a.txt", []byte("hello")))
	if status.Code(err) != codes.Internal {
		t.Fatalf("code = %v, want Internal (err: %v)", status.Code(err), err)
	}
}

func TestUploadFileReturnsLocalReadError(t *testing.T) {
	client := startFileServer(t, &fakeFileServer{})

	if err := client.UploadFile("bucket", filepath.Join(t.TempDir(), "missing.txt")); err == nil {
		t.Fatal("expected an error for a missing local file")
	}
}

func TestUploadFileSendsEmptyFile(t *testing.T) {
	impl := &fakeFileServer{}
	client := startFileServer(t, impl)

	if err := client.UploadFile("bucket", writeTempFile(t, "empty.txt", nil)); err != nil {
		t.Fatalf("empty file upload failed: %v", err)
	}
	impl.mu.Lock()
	defer impl.mu.Unlock()
	if data, ok := impl.received["empty.txt"]; !ok || len(data) != 0 {
		t.Fatalf("empty file not received as empty (present=%v, %d bytes)", ok, len(data))
	}
}

func TestUploadFileIsSafeForConcurrentUse(t *testing.T) {
	impl := &fakeFileServer{}
	client := startFileServer(t, impl)

	const uploads = 16
	paths := make([]string, uploads)
	for i := range paths {
		paths[i] = writeTempFile(t, fmt.Sprintf("file-%02d.txt", i), bytes.Repeat([]byte{byte(i)}, 5000))
	}

	var wg sync.WaitGroup
	errs := make([]error, uploads)
	for i, path := range paths {
		wg.Add(1)
		go func(i int, path string) {
			defer wg.Done()
			errs[i] = client.UploadFile("bucket", path)
		}(i, path)
	}
	wg.Wait()

	if err := errors.Join(errs...); err != nil {
		t.Fatal(err)
	}
	impl.mu.Lock()
	defer impl.mu.Unlock()
	for i := 0; i < uploads; i++ {
		want := bytes.Repeat([]byte{byte(i)}, 5000)
		if got := impl.received[fmt.Sprintf("file-%02d.txt", i)]; !bytes.Equal(got, want) {
			t.Fatalf("file %d corrupted: %d bytes received", i, len(got))
		}
	}
}

func TestUploadFileReturnsErrorWhenInterrupted(t *testing.T) {
	impl := &fakeFileServer{holdUpload: true, firstChunk: make(chan struct{})}
	client := startFileServer(t, impl)

	result := make(chan error, 1)
	go func() {
		result <- client.UploadFile("bucket", writeTempFile(t, "slow.txt", []byte("data")))
	}()

	select {
	case <-impl.firstChunk:
	case <-time.After(5 * time.Second):
		t.Fatal("server never received the first chunk")
	}
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-result:
		if !errors.Is(err, ErrUploadInterrupted) {
			t.Fatalf("an interrupted upload must report it, got: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("upload did not stop after the signal")
	}
}

func TestDownloadFileStreamsAfterReturning(t *testing.T) {
	data := bytes.Repeat([]byte("verifiable-remote-file-storage-"), 2000)
	client := startFileServer(t, &fakeFileServer{downloadData: data, chunkDelay: 2 * time.Millisecond})

	file, err := client.DownloadFile("bucket", 0)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("stream was cut before the end of the file: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("got %d bytes, want %d", len(got), len(data))
	}
}

func TestDownloadFileOutlivesIdleTimeout(t *testing.T) {
	data := bytes.Repeat([]byte("x"), 8*4096)
	client := startFileServer(t, &fakeFileServer{downloadData: data, chunkDelay: 30 * time.Millisecond})
	client.(*fTService).idleTimeout = 150 * time.Millisecond

	file, err := client.DownloadFile("bucket", 0)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("a transfer that keeps progressing must not time out: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("got %d bytes, want %d", len(got), len(data))
	}
}

func TestDownloadFileFailsWhenStreamStalls(t *testing.T) {
	client := startFileServer(t, &fakeFileServer{holdDownload: true})
	client.(*fTService).idleTimeout = 50 * time.Millisecond

	file, err := client.DownloadFile("bucket", 0)
	if err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() {
		_, err := io.ReadAll(file)
		result <- err
	}()

	select {
	case err := <-result:
		if err == nil {
			t.Fatal("a stalled transfer must fail")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("stalled transfer was not stopped")
	}
}

func TestDownloadFileReturnsServerStatus(t *testing.T) {
	client := startFileServer(t, &fakeFileServer{downloadErr: status.Error(codes.NotFound, "no such file")})

	_, err := client.DownloadFile("bucket", 0)
	if status.Code(err) != codes.NotFound {
		t.Fatalf("code = %v, want NotFound (err: %v)", status.Code(err), err)
	}
}
