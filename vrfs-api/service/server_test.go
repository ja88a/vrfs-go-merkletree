package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	config "github.com/ja88a/vrfs-go-merkletree/libs/config"
	logger "github.com/ja88a/vrfs-go-merkletree/libs/logger"
	mtutils "github.com/ja88a/vrfs-go-merkletree/libs/merkletree/utils"
	pb "github.com/ja88a/vrfs-go-merkletree/libs/rpcapi/protos/v1/vrfs-api"
	pbfs "github.com/ja88a/vrfs-go-merkletree/libs/rpcapi/protos/v1/vrfs-fs"
)

type fakeFSClient struct {
	pbfs.FileServiceClient
	hashes [][]byte
}

func (f *fakeFSClient) BucketFileHashes(context.Context, *pbfs.BucketFileHashesRequest, ...grpc.CallOption) (*pbfs.BucketFileHashesResponse, error) {
	return &pbfs.BucketFileHashesResponse{FileHashes: f.hashes}, nil
}

type memoryStore struct {
	mu   sync.Mutex
	data map[string]string
}

func (m *memoryStore) GetString(key string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.data[key], nil
}

func (m *memoryStore) Set(key string, val interface{}, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.data == nil {
		m.data = map[string]string{}
	}
	m.data[key] = string(val.([]byte))
	return nil
}

func newTestServer(t *testing.T) (*VerifiableRemoteFileStorageServer, *memoryStore, []byte) {
	t.Helper()
	hashes := [][]byte{
		[]byte("00000000000000000000000000000001"),
		[]byte("00000000000000000000000000000002"),
	}
	tree, err := mtutils.GenerateMerkleTree(hashes, false)
	if err != nil {
		t.Fatal(err)
	}
	store := &memoryStore{}
	return &VerifiableRemoteFileStorageServer{
		l:        logger.New("error"),
		cfg:      &config.Config{Log: config.Log{Level: "error"}},
		fsClient: &fakeFSClient{hashes: hashes},
		db:       store,
	}, store, tree.Root
}

func TestUploadDoneDoesNotPersistProofsOnRootMismatch(t *testing.T) {
	server, store, _ := newTestServer(t)

	resp, err := server.UploadDone(context.Background(), &pb.UploadDoneRequest{
		TenantId: "tenant", FilesetId: "fs-1", MtRoot: []byte("a-different-root"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetStatus() != 419 {
		t.Fatalf("status = %d, want 419", resp.GetStatus())
	}
	if got, _ := store.GetString(computeDbKeyMtProofs("tenant", "fs-1")); got != "" {
		t.Fatal("proofs were persisted for a fileset that failed verification")
	}
}

func TestUploadDonePersistsProofsOnMatch(t *testing.T) {
	server, store, root := newTestServer(t)

	resp, err := server.UploadDone(context.Background(), &pb.UploadDoneRequest{
		TenantId: "tenant", FilesetId: "fs-1", MtRoot: root,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetStatus() != 200 {
		t.Fatalf("status = %d, want 200: %s", resp.GetStatus(), resp.GetMessage())
	}
	if got, _ := store.GetString(computeDbKeyMtProofs("tenant", "fs-1")); got == "" {
		t.Fatal("proofs were not persisted after a successful verification")
	}
}

func TestDownloadFileInfoRejectsOutOfRangeIndex(t *testing.T) {
	server, _, root := newTestServer(t)
	if _, err := server.UploadDone(context.Background(), &pb.UploadDoneRequest{
		TenantId: "tenant", FilesetId: "fs-1", MtRoot: root,
	}); err != nil {
		t.Fatal(err)
	}

	for _, index := range []int32{-1, 2, 99} {
		_, err := server.DownloadFileInfo(context.Background(), &pb.DownloadFileInfoRequest{
			TenantId: "tenant", FilesetId: "fs-1", FileIndex: index,
		})
		if status.Code(err) != codes.OutOfRange {
			t.Fatalf("index %d: code = %v, want OutOfRange (err: %v)", index, status.Code(err), err)
		}
	}
}

func TestDownloadFileInfoReturnsProof(t *testing.T) {
	server, _, root := newTestServer(t)
	if _, err := server.UploadDone(context.Background(), &pb.UploadDoneRequest{
		TenantId: "tenant", FilesetId: "fs-1", MtRoot: root,
	}); err != nil {
		t.Fatal(err)
	}

	resp, err := server.DownloadFileInfo(context.Background(), &pb.DownloadFileInfoRequest{
		TenantId: "tenant", FilesetId: "fs-1", FileIndex: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.GetMtProof().GetSiblings()) == 0 {
		t.Fatal("expected a Merkle proof")
	}
}
