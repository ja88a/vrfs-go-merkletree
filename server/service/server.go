package service

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	pbfs "github.com/ja88a/vrfs-go-merkletree/libs/rpcapi/protos/v1/fileserver"
	pb "github.com/ja88a/vrfs-go-merkletree/libs/rpcapi/protos/v1/vrfs"

	config "github.com/ja88a/vrfs-go-merkletree/libs/config"
	logger "github.com/ja88a/vrfs-go-merkletree/libs/logger"
	cache "github.com/ja88a/vrfs-go-merkletree/libs/redis/client"
)

// Service execution context
type VerifiableRemoteFileStorageServer struct {
	pb.UnimplementedVerifiableRemoteFileStorageServer
	l        	*logger.Logger
	cfg      	*config.Config
	fsClient 	pbfs.FileServiceClient
	db				*cache.CacheClient
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
		db: 			kvdb,
	}, nil
}

// Handle the requests for file bucket where files can be uploaded on the Remote File Storage server
func (g *VerifiableRemoteFileStorageServer) UploadBucket(ctx context.Context, in *pb.UploadBucketRequest) (*pb.UploadBucketResponse, error) {
	g.l.Debug("Received bucket req from '%d' for fileset '%v'", in.UserId, in.FilesetId)
	bucketId := computeBucketId(in.UserId, in.FilesetId)
	return &pb.UploadBucketResponse{Status: 200, BucketId: bucketId}, nil
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

// Handle the requests for notifying that a fileset has been uploaded and its consistency verified and confirmed to the client
func (g *VerifiableRemoteFileStorageServer) UploadDone(ctx context.Context, in *pb.UploadDoneRequest) (*pb.UploadDoneResponse, error) {
	g.l.Debug("Received UploadDone req from '%d' for fileset '%v' with MT root '%v'", in.UserId, in.FilesetId, in.FilesetMtRoot)
	bucketId := computeBucketId(in.UserId, in.FilesetId)

	ctxFS, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Request to the FS for providing the list of the file hashes present in its bucket
	// Alternative: download all the bucket files and compute the file hashes here! But we'll avoid consuming unnecessary bandwidth
	resp, err := g.fsClient.BucketFileHashes(ctxFS, &pbfs.BucketFileHashesRequest{BucketId: bucketId})
	if err != nil {
		respErr := fmt.Errorf("VRFS failed to retrieve files hashes from FS for fileset '%v' (%v)\n%w", in.GetFilesetId(), bucketId, err)
		g.l.Error(fmt.Sprint(respErr))
		return &pb.UploadDoneResponse{Status: 500, Message: fmt.Sprint(respErr)}, respErr
	}

	// Compute the MerkleTree root
	fileHashes := resp.GetFileHashes()
	tree, err := GenerateMerkleTree(fileHashes)
	if err != nil {
		respErr := fmt.Errorf("VRFS failed to compute merkletree from files hashes of fileset '%v' (%v)\n%v", in.GetFilesetId(), bucketId, err)
		g.l.Error(fmt.Sprint(respErr))
		return &pb.UploadDoneResponse{Status: 500, Message: fmt.Sprint(respErr)}, respErr
	}

	// Persist the MerkleTree proofs for each leaf node, for later retrieval by the client
	//STORE_fileSetTreeProofs[in.FilesetId] = tree.Proofs
	dbKey := computeDbKeyMtProofs(in.UserId, in.FilesetId)
	g.db.Set(dbKey, tree.Proofs, 0)

	// Compare the MerkleTree roots to confirm that file sets match, or not
	treeRoot := fmt.Sprintf("%x", tree.Root)
	if treeRoot != in.GetFilesetMtRoot() {
		respErr := fmt.Errorf("VRFS MerkleTree roots differ for fileset '%v' (bucket: %v) - Generated root: '%v'", in.GetFilesetId(), bucketId, treeRoot)
		g.l.Warn(fmt.Sprint(respErr))
		return &pb.UploadDoneResponse{Status: 419, Message: fmt.Sprint(respErr)}, nil
	}

	return &pb.UploadDoneResponse{Status: 200, Message: "MerkleTree roots match!"}, nil
}

// Utility method for computing the KV store's entry key for a set of MerkleTree proofs
func computeDbKeyMtProofs(userId string, fileSetId string) string {
	return userId +"_"+ fileSetId +"_mtproofs"
}

// Temporary solution for testing E2E -> External Persistence MANDATORY
//var STORE_fileSetTreeProofs = make(map[string][]*merkletree.Proof)

// Get the download info to retrieve a file from the files storage server as well as 
// the MerkleTree proofs to confirm it has not been tampered while stored
func (g *VerifiableRemoteFileStorageServer) DownloadFileInfo(ctx context.Context, in *pb.DownloadFileInfoRequest) (*pb.DownloadFileInfoResponse, error) {
	g.l.Debug("Received DownloadFileInfo req from '%d' for fileset '%v' file #%v", in.UserId, in.FilesetId, in.FileIndex)

	bucketId := computeBucketId(in.UserId, in.FilesetId)
	
	//mtProofs := STORE_fileSetTreeProofs[in.FilesetId]
	dbKey := computeDbKeyMtProofs(in.UserId, in.FilesetId)
	mtProofs, err := g.db.Get(dbKey)
	if err != nil {
		respMsg := fmt.Sprintf("failed to retrieve mt proofs for fileset '%v' file #%d from db \n%v", in.FilesetId, in.FileIndex, err)
		g.l.Error(respMsg)
		return &pb.DownloadFileInfoResponse{BucketId: bucketId, MtProofs: nil}, status.Error(codes.DataLoss, respMsg)
	}
	if mtProofs == nil {
		respMsg := fmt.Sprintf("No MerkleTree Proofs available for fileset %v", in.FilesetId)
		g.l.Warn(respMsg)
		return &pb.DownloadFileInfoResponse{BucketId: bucketId, MtProofs: nil}, status.Error(codes.FailedPrecondition, respMsg)
	}

	// TODO Implement the Serialization of []merkletree.Proof{ Siblings: [][]byte, Path: uint32 }
	//fileMtProofs := mtProofs[in.FileIndex]
	return &pb.DownloadFileInfoResponse{BucketId: bucketId, MtProofs: nil}, nil
}