package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	config "github.com/ja88a/vrfs-go-merkletree/libs/config"
	logger "github.com/ja88a/vrfs-go-merkletree/libs/logger"
	rpcfile "github.com/ja88a/vrfs-go-merkletree/libs/rpcapi/file"
	pb "github.com/ja88a/vrfs-go-merkletree/libs/rpcapi/protos/v1/vrfs-fs"
)

type testServer struct {
	client  pb.FileServiceClient
	storage string
	handled chan struct{}
}

func startServer(t *testing.T) *testServer {
	t.Helper()
	storage := filepath.Join(t.TempDir(), "storage")
	if err := os.MkdirAll(storage, 0o750); err != nil {
		t.Fatal(err)
	}

	handled := make(chan struct{}, 16)
	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer(grpc.StreamInterceptor(func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		defer func() { handled <- struct{}{} }()
		return handler(srv, ss)
	}))
	cfg := &config.Config{FilesStorage: config.FilesStorage{Location: storage}}
	pb.RegisterFileServiceServer(server, New(logger.New("error"), cfg))
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)

	conn, err := grpc.Dial("bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	return &testServer{client: pb.NewFileServiceClient(conn), storage: storage, handled: handled}
}

func (s *testServer) upload(t *testing.T, bucket, name string, chunks ...[]byte) (*pb.FileUploadResponse, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stream, err := s.client.Upload(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, chunk := range chunks {
		if err := stream.Send(&pb.FileUploadRequest{BucketId: bucket, FileName: name, Chunk: chunk}); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
	}
	return stream.CloseAndRecv()
}

func (s *testServer) waitHandled(t *testing.T) {
	t.Helper()
	select {
	case <-s.handled:
	case <-time.After(5 * time.Second):
		t.Fatal("server handler did not finish")
	}
}

func TestUploadStoresFileAndReportsSize(t *testing.T) {
	s := startServer(t)

	resp, err := s.upload(t, "bucket-1", "a.txt", []byte("hello "), []byte("world"))
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetFileName() != "a.txt" || resp.GetSize() != 11 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	got, err := os.ReadFile(filepath.Join(s.storage, "bucket-1", "a.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello world" {
		t.Fatalf("stored %q", got)
	}
}

func TestUploadStoresEmptyFile(t *testing.T) {
	s := startServer(t)

	if _, err := s.upload(t, "bucket-1", "empty.txt", nil); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(s.storage, "bucket-1", "empty.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != 0 {
		t.Fatalf("size = %d, want 0", info.Size())
	}
}

func TestUploadFailsWhenFileCannotBeStored(t *testing.T) {
	s := startServer(t)
	// A directory at the target path makes the file impossible to store.
	if err := os.MkdirAll(filepath.Join(s.storage, "bucket-1", "a.txt"), 0o750); err != nil {
		t.Fatal(err)
	}

	if _, err := s.upload(t, "bucket-1", "a.txt", []byte("hello")); err == nil {
		t.Fatal("upload was acknowledged although the file was not stored")
	}
}

func TestUploadRejectsStreamWithoutMetadata(t *testing.T) {
	s := startServer(t)

	if _, err := s.upload(t, "", ""); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code = %v, want InvalidArgument (err: %v)", status.Code(err), err)
	}
}

func TestUploadRejectsMetadataChangeMidStream(t *testing.T) {
	s := startServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stream, err := s.client.Upload(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.txt", "b.txt"} {
		if err := stream.Send(&pb.FileUploadRequest{BucketId: "bucket-1", FileName: name, Chunk: []byte("x")}); err != nil && !errors.Is(err, io.EOF) {
			t.Fatal(err)
		}
	}
	if _, err := stream.CloseAndRecv(); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code = %v, want InvalidArgument (err: %v)", status.Code(err), err)
	}
	assertNoFiles(t, filepath.Join(s.storage, "bucket-1"))
}

func TestUploadRejectsPathTraversal(t *testing.T) {
	cases := []struct{ name, bucket, file string }{
		{"file name", "bucket-1", "../../escaped.txt"},
		{"bucket", "../../escaped-bucket", "a.txt"},
		{"dot bucket", "..", "escaped.txt"},
		{"absolute file", "bucket-1", "/tmp/escaped.txt"},
		{"reserved bucket", uploadTmpDirName, "a.txt"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := startServer(t)
			if _, err := s.upload(t, tc.bucket, tc.file, []byte("x")); status.Code(err) != codes.InvalidArgument {
				t.Fatalf("code = %v, want InvalidArgument (err: %v)", status.Code(err), err)
			}
			assertNoFiles(t, filepath.Dir(s.storage))
		})
	}
}

func TestInterruptedUploadLeavesNoFile(t *testing.T) {
	s := startServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream, err := s.client.Upload(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := stream.Send(&pb.FileUploadRequest{BucketId: "bucket-1", FileName: "a.txt", Chunk: []byte("partial")}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	cancel()
	s.waitHandled(t)

	assertNoFiles(t, s.storage)
}

func TestDownloadReturnsStoredFile(t *testing.T) {
	s := startServer(t)
	content := bytes.Repeat([]byte("abc"), 5000)
	if _, err := s.upload(t, "bucket-1", "a.txt", content); err != nil {
		t.Fatal(err)
	}

	stream, err := s.client.Download(context.Background(), &pb.FileDownloadRequest{BucketId: "bucket-1", FileIndex: 0})
	if err != nil {
		t.Fatal(err)
	}
	var got []byte
	for {
		msg, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, msg.GetChunk()...)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("got %d bytes, want %d", len(got), len(content))
	}
}

func TestDownloadSupportsNonASCIIFileNames(t *testing.T) {
	for _, name := range []string{"café.txt", "日本語.データ", "with space.txt", "100%.txt"} {
		t.Run(name, func(t *testing.T) {
			s := startServer(t)
			if _, err := s.upload(t, "bucket-1", name, []byte("content")); err != nil {
				t.Fatal(err)
			}

			stream, err := s.client.Download(context.Background(), &pb.FileDownloadRequest{BucketId: "bucket-1", FileIndex: 0})
			if err != nil {
				t.Fatal(err)
			}
			md, err := stream.Header()
			if err != nil {
				t.Fatal(err)
			}
			msg, err := stream.Recv()
			if err != nil {
				t.Fatalf("download failed: %v", err)
			}
			if got := rpcfile.NewFromMetadata(md, nil).Name; got != name {
				t.Fatalf("name = %q, want %q", got, name)
			}
			if string(msg.GetChunk()) != "content" {
				t.Fatalf("content = %q", msg.GetChunk())
			}
		})
	}
}

func TestDownloadRejectsIndexPastTheEnd(t *testing.T) {
	s := startServer(t)
	if _, err := s.upload(t, "bucket-1", "a.txt", []byte("x")); err != nil {
		t.Fatal(err)
	}

	stream, err := s.client.Download(context.Background(), &pb.FileDownloadRequest{BucketId: "bucket-1", FileIndex: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stream.Recv(); status.Code(err) != codes.NotFound {
		t.Fatalf("code = %v, want NotFound (err: %v)", status.Code(err), err)
	}
}

func TestReadsRejectPathTraversal(t *testing.T) {
	s := startServer(t)
	outside := filepath.Join(filepath.Dir(s.storage), "outside")
	if err := os.MkdirAll(outside, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := s.client.BucketFileHashes(context.Background(), &pb.BucketFileHashesRequest{BucketId: "../outside"}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("hashes: code = %v, want InvalidArgument (err: %v)", status.Code(err), err)
	}
	stream, err := s.client.Download(context.Background(), &pb.FileDownloadRequest{BucketId: "../outside", FileIndex: 0})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stream.Recv(); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("download: code = %v, want InvalidArgument (err: %v)", status.Code(err), err)
	}
}

// assertNoFiles fails if any regular file exists below root.
func assertNoFiles(t *testing.T, root string) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if !entry.IsDir() {
			t.Errorf("unexpected file left behind: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
