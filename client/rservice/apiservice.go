package rservice

import (
	"context"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	mt "github.com/ja88a/vrfs-go-merkletree/libs/merkletree"
	pbvrfs "github.com/ja88a/vrfs-go-merkletree/libs/rpcapi/protos/v1/vrfs"
)

const UserMock = "umock"

// Client execution context to interact with its API services
type ApiService struct {
	// client to the VRFS GRPC service
	VrfsClient pbvrfs.VerifiableRemoteFileStorageClient

	// default connection timeout for VRFS
	vrfsTimeout time.Duration

	// RFS server endpoint
	RfsEndpoint string

	// Max number of concurrent file uploads
	UploadMaxConcurrent int

	// Max data batch size for a file upload
	UploadMaxBatchSize int

	// logger to replace global logging
	//log

	// Path of the local root repository where files are downloaded
	LocalFileDownloadRepo string
}

// Init the client's remote service / context
func NewClientContext(vrfsEndpoint string, nfsEndpoint string, upMaxConcurrent int, upBatchSize int, localDownloadRepo string) (*ApiService, error) {
	// VRFS server connection
	vrfsConn, err := grpc.Dial(vrfsEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return &ApiService{
		VrfsClient:  pbvrfs.NewVerifiableRemoteFileStorageClient(vrfsConn),
		vrfsTimeout: time.Second,

		RfsEndpoint:           nfsEndpoint,
		UploadMaxConcurrent:   upMaxConcurrent,
		UploadMaxBatchSize:    upBatchSize,
		LocalFileDownloadRepo: localDownloadRepo,
	}, nil
}

// Handle the request for a file storage bucket from the VRFS API, to upload files to the file storage service
func (clientCtx *ApiService) HandleFileBucketReq(userId string, fileSetId string) (int32, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	resp, err := clientCtx.VrfsClient.UploadBucket(ctx, &pbvrfs.UploadBucketRequest{UserId: userId, FilesetId: fileSetId})
	if err != nil {
		return -1, "", fmt.Errorf("failed to request file bucket for '%v'\n%w", fileSetId, err)
	}

	return 0, resp.GetBucketId(), nil
}

// Handle the request to VRFS for confirming the fileset has been correctly uploaded & stored
func (clientCtx *ApiService) HandleUploadDoneReq(userId string, fileSetId string, mtRootHash []byte) (int32, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	resp, err := clientCtx.VrfsClient.UploadDone(ctx, &pbvrfs.UploadDoneRequest{UserId: userId, FilesetId: fileSetId, MtRoot: mtRootHash})
	if err != nil {
		return -1, resp.GetMessage(), fmt.Errorf("failed to request for files upload correctness - fileset: '%v'\n%w", fileSetId, err)
	}

	return resp.GetStatus(), resp.GetMessage(), nil
}

// Handle the request to VRFS for retrieving the info to download a file and check/proove it is untampered
func (clientCtx *ApiService) HandleDownloadFileInfoReq(userId string, fileSetId string, fileIndex int) (string, *mt.Proof, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	resp, err := clientCtx.VrfsClient.DownloadFileInfo(ctx, &pbvrfs.DownloadFileInfoRequest{UserId: userId, FilesetId: fileSetId, FileIndex: int32(fileIndex)})
	if err != nil {
		return resp.GetBucketId(), nil, fmt.Errorf("failed to retrieve download info for file #%4d in fileset '%v'\n%w", fileIndex, fileSetId, err)
	}

	mtProof := &mt.Proof{
		Siblings: resp.GetMtProof().GetSiblings(),
		Path:     resp.GetMtProof().GetPath(),
	}

	return resp.GetBucketId(), mtProof, nil
}

// Handle the VRFS API ping request, to check for its availability
func (clientCtx *ApiService) HandlePingVrfsReq() error {
	// Contact the server and print out its response.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	resp, err := clientCtx.VrfsClient.SayHello(ctx, &pbvrfs.HelloRequest{Name: "VRFS User"})
	if err != nil {
		return fmt.Errorf("VFRS service ping request fails\n%w", err)
	}

	log.Printf("Greetings %s! Connection to VRFS server is operational", resp.GetMessage())
	return nil
}
