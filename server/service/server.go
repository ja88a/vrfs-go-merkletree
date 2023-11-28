package service

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	mt "github.com/ja88a/vrfs-go-merkletree/libs/merkletree"
	mtutils "github.com/ja88a/vrfs-go-merkletree/libs/merkletree/utils"

	pbfs "github.com/ja88a/vrfs-go-merkletree/libs/rpcapi/protos/v1/fileserver"
	pb "github.com/ja88a/vrfs-go-merkletree/libs/rpcapi/protos/v1/vrfs"

	config "github.com/ja88a/vrfs-go-merkletree/libs/config"
	cache "github.com/ja88a/vrfs-go-merkletree/libs/db/cache"
	logger "github.com/ja88a/vrfs-go-merkletree/libs/logger"
)

// Service execution context
type VerifiableRemoteFileStorageServer struct {
	pb.UnimplementedVerifiableRemoteFileStorageServer
	l        *logger.Logger
	cfg      *config.Config
	fsClient pbfs.FileServiceClient
	db       *cache.CacheClient
}

// Init the service execution context
func New(l *logger.Logger, cfg *config.Config) (*VerifiableRemoteFileStorageServer, error) {
	// VRFS server connection
	fsConn, err := grpc.Dial(cfg.FSAPI.Endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("VRFS failed to dial FS server at '%v'\n%v", cfg.FSAPI.Endpoint, err)
	}

	// Cache DB client
	kvdb := cache.NewCacheClient(context.Background(), l, cfg)

	return &VerifiableRemoteFileStorageServer{
		l:        l,
		cfg:      cfg,
		fsClient: pbfs.NewFileServiceClient(fsConn),
		db:       kvdb,
	}, nil
}

// Handle the requests for file bucket where files can be uploaded on the Remote File Storage server
func (g *VerifiableRemoteFileStorageServer) UploadBucket(ctx context.Context, in *pb.UploadBucketRequest) (*pb.UploadBucketResponse, error) {
	g.l.Debug("Received bucket req for fileset '%v' of Tenant: %v", in.GetFilesetId(), in.GetTenantId())
	bucketId := computeBucketId(in.GetTenantId(), in.GetFilesetId())
	return &pb.UploadBucketResponse{BucketId: bucketId}, nil
}

// Simple bucket ID generation method
func computeBucketId(userId string, filesetId string) string {
	return userId + "_" + filesetId
}

// Handle the requests for ping requests: SayHello supports a basic ping service
func (g *VerifiableRemoteFileStorageServer) SayHello(ctx context.Context, in *pb.HelloRequest) (*pb.HelloReply, error) {
	g.l.Debug("Received: %v", in.GetName())
	return &pb.HelloReply{Message: "Hello " + in.GetName()}, nil
}

// Handle the requests for notifying that a fileset has been uploaded, its consistency verified and confirmed to the client
func (g *VerifiableRemoteFileStorageServer) UploadDone(ctx context.Context, in *pb.UploadDoneRequest) (*pb.UploadDoneResponse, error) {
	g.l.Debug("Received UploadDone req from '%v' for fileset '%v' with MT root '%x'", in.GetTenantId(), in.GetFilesetId(), in.GetMtRoot())
	bucketId := computeBucketId(in.GetTenantId(), in.GetFilesetId())

	ctxFS, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Request to the FS for providing the list of the file hashes present in its bucket
	// Alternative: download all the bucket files and compute the file hashes here! But we'll avoid consuming unnecessary bandwidth
	resp, err := g.fsClient.BucketFileHashes(ctxFS, &pbfs.BucketFileHashesRequest{BucketId: bucketId})
	if err != nil {
		respErr := fmt.Errorf("failed to retrieve files hashes from FS for fileset '%v' (bucket: '%v')\n%w", in.GetFilesetId(), bucketId, err)
		g.l.Error(fmt.Sprint(respErr))
		return &pb.UploadDoneResponse{Status: 500, Message: fmt.Sprint(respErr)}, respErr
	}

	// Compute the MerkleTree root
	fileHashes := resp.GetFileHashes()
	tree, err := mtutils.GenerateMerkleTree(fileHashes, true)
	if err != nil {
		respErr := fmt.Errorf("failed to compute merkletree from files hashes of fileset '%v' (%v)\n%v", in.GetFilesetId(), bucketId, err)
		g.l.Error(fmt.Sprint(respErr))
		return &pb.UploadDoneResponse{Status: 500, Message: fmt.Sprint(respErr)}, respErr
	}
	if len(tree.Proofs) == 0 {
		respErr := fmt.Errorf("failed to generate merkletree proofs for fileset '%v' (bucket: %v)", in.GetFilesetId(), bucketId)
		g.l.Error(fmt.Sprint(respErr))
		return &pb.UploadDoneResponse{Status: 500, Message: fmt.Sprint(respErr)}, nil
	}

	// Persist the MerkleTree proofs for each leaf node, for later retrieval by clients
	dbKey := computeDbKeyMtProofs(in.GetTenantId(), in.GetFilesetId())
	mtProofsJson, err := json.Marshal(tree.Proofs)
	if err != nil {
		respMsg := fmt.Sprintf("Failed to marshall MT Proofs to JSON for fileset '%v' Key: #%v\n%v", in.FilesetId, dbKey, err)
		g.l.Error(respMsg)
	}
	err = g.db.Set(dbKey, mtProofsJson, 0)
	if err != nil {
		respMsg := fmt.Sprintf("Failed to persist MT Proofs in DB for fileset '%v' Key: #%v\n%v", in.FilesetId, dbKey, err)
		g.l.Error(respMsg)
		return &pb.UploadDoneResponse{Status: 500, Message: fmt.Sprint(respMsg)}, status.Error(codes.DataLoss, respMsg)
	}

	g.l.Debug("%d MerkleTree proofs persisted in DB for fileset '%v' - key: '%v'", len(tree.Proofs), in.GetFilesetId(), dbKey)

	// Compare the MerkleTree roots to confirm that filesets match, or not
	if hex.EncodeToString(tree.Root) != hex.EncodeToString(in.GetMtRoot()) {
		respErr := fmt.Errorf("VRFS MerkleTree roots differ for fileset '%v' (bucket: %v) - Generated root: '%x'", in.GetFilesetId(), bucketId, tree.Root)
		g.l.Error(fmt.Sprint(respErr))
		return &pb.UploadDoneResponse{Status: 419, Message: fmt.Sprint(respErr)}, nil
	}

	g.l.Info("Files in FS bucket '%v' for fileset '%v' have their merkle tree root matching the client one: %x", bucketId, in.FilesetId, tree.Root)

	return &pb.UploadDoneResponse{Status: 200, Message: "MerkleTree roots match - Files Upload successful"}, nil
}

// Utility method for computing the KV store's entry key for a set of MerkleTree proofs
func computeDbKeyMtProofs(userId string, fileSetId string) string {
	return userId + "_" + fileSetId + "_mtproofs"
}

// Get the download info to retrieve a file from the files storage server as well as
// the MerkleTree proofs to confirm it has not been tampered while being stored
func (g *VerifiableRemoteFileStorageServer) DownloadFileInfo(ctx context.Context, in *pb.DownloadFileInfoRequest) (*pb.DownloadFileInfoResponse, error) {
	g.l.Info("Handle a DownloadFileInfo req from '%v' for fileset '%v' file #%v", in.GetTenantId(), in.GetFilesetId(), in.GetFileIndex())

	// Target FS fileset bucket
	bucketId := computeBucketId(in.GetTenantId(), in.GetFilesetId())

	// Retrieve MT Proofs from the DB
	dbKey := computeDbKeyMtProofs(in.GetTenantId(), in.GetFilesetId())
	mtProofsDb, err := g.db.GetString(dbKey)
	if err != nil {
		respMsg := fmt.Sprintf("Failed to retrieve MT Proofs for fileset '%v' file #%d from db \n%v", in.FilesetId, in.FileIndex, err)
		g.l.Error(respMsg)
		return &pb.DownloadFileInfoResponse{BucketId: bucketId, MtProof: nil}, status.Error(codes.DataLoss, respMsg)
	}
	if mtProofsDb == "" {
		respMsg := fmt.Sprintf("No MerkleTree Proofs available in DB for fileset '%v' Tenant: '%v'", in.GetFilesetId(), in.GetTenantId())
		g.l.Error(respMsg)
		return &pb.DownloadFileInfoResponse{BucketId: bucketId, MtProof: nil}, status.Error(codes.FailedPrecondition, respMsg)
	}

	// Convert the MT Proof
	var mtProofs []*mt.Proof
	err = json.Unmarshal([]byte(mtProofsDb), &mtProofs)
	if err != nil {
		respMsg := fmt.Sprintf("Failed to unmarshall JSON MerkleTree Proofs from DB for fileset '%v' Tenant: '%v'", in.GetFilesetId(), in.GetTenantId())
		g.l.Error(respMsg)
		return &pb.DownloadFileInfoResponse{BucketId: bucketId, MtProof: nil}, status.Error(codes.Internal, respMsg)
	}
	fileMtProof := mtProofs[in.FileIndex]

	if g.cfg.Log.Level == "debug" {
		for index, sibling := range fileMtProof.Siblings {
			g.l.Debug("Sibling #%d Hex: '%x' for file #%d of '%v' (%v)", index, sibling, in.GetFileIndex(), in.GetFilesetId(), in.GetTenantId())
		}
	}

	pbMtProof := &pb.MTProof{
		Siblings: fileMtProof.Siblings,
		Path:     fileMtProof.Path,
	}

	return &pb.DownloadFileInfoResponse{BucketId: bucketId, MtProof: pbMtProof}, nil
}
