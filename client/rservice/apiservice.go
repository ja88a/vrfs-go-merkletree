package rservice

import (
	"context"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	mt "github.com/ja88a/vrfs-go-merkletree/libs/merkletree"
	pbvrfs "github.com/ja88a/vrfs-go-merkletree/libs/rpcapi/protos/v1/vrfs-api"
)

// The client API for the Verifiable Remote File Storage service
type VrfsService interface {
	// Handle the request for a file storage bucket from the VRFS API, to upload files to the file storage service
	HandleFileBucketReq(tenantId string, fileSetId string) (int32, string, error)

	// Handle the request to VRFS for confirming the fileset has been correctly uploaded & stored
	HandleUploadDoneReq(tenantId string, fileSetId string, mtRootHash []byte) (int32, string, error)

	// Handle the request to VRFS for retrieving the info to download a file and check/proove it is untampered
	HandleDownloadFileInfoReq(tenantId string, fileSetId string, fileIndex int) (string, *mt.Proof, error)

	// Handle a VRFS API ping request, to check for the service availability
	HandlePingReq() error
}

// Client execution context to interact with its API services
type vrfsService struct {
	// client to the VRFS GRPC service
	client pbvrfs.VerifiableRemoteFileStorageClient

	// default connection timeout for VRFS
	vrfsTimeout time.Duration
}

// Init the client's remote service / context
func NewVrfsClient(vrfsEndpoint string) (VrfsService, error) {
	// VRFS server connection
	vrfsConn, err := grpc.Dial(vrfsEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return &vrfsService{
		client:      pbvrfs.NewVerifiableRemoteFileStorageClient(vrfsConn),
		vrfsTimeout: time.Second,
	}, nil
}

// Handle the request for a file storage bucket from the VRFS API, to upload files to the file storage service
func (apiCtx *vrfsService) HandleFileBucketReq(tenantId string, fileSetId string) (int32, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), apiCtx.vrfsTimeout)
	defer cancel()

	resp, err := apiCtx.client.UploadBucket(ctx, &pbvrfs.UploadBucketRequest{TenantId: tenantId, FilesetId: fileSetId})
	if err != nil {
		return -1, "", fmt.Errorf("failed to request file bucket for '%v'\n%w", fileSetId, err)
	}

	return 0, resp.GetBucketId(), nil
}

// Handle the request to VRFS for confirming the fileset has been correctly uploaded & stored
func (apiCtx *vrfsService) HandleUploadDoneReq(tenantId string, fileSetId string, mtRootHash []byte) (int32, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), apiCtx.vrfsTimeout)
	defer cancel()

	resp, err := apiCtx.client.UploadDone(ctx, &pbvrfs.UploadDoneRequest{TenantId: tenantId, FilesetId: fileSetId, MtRoot: mtRootHash})
	if err != nil {
		return -1, resp.GetMessage(), fmt.Errorf("failed to request for files upload correctness - fileset: '%v'\n%w", fileSetId, err)
	}

	return resp.GetStatus(), resp.GetMessage(), nil
}

// Handle the request to VRFS for retrieving the info to download a file and check/proove it is untampered
func (apiCtx *vrfsService) HandleDownloadFileInfoReq(tenantId string, fileSetId string, fileIndex int) (string, *mt.Proof, error) {
	ctx, cancel := context.WithTimeout(context.Background(), apiCtx.vrfsTimeout)
	defer cancel()

	resp, err := apiCtx.client.DownloadFileInfo(ctx, &pbvrfs.DownloadFileInfoRequest{TenantId: tenantId, FilesetId: fileSetId, FileIndex: int32(fileIndex)})
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
func (apiCtx *vrfsService) HandlePingReq() error {
	// Contact the server and print out its response.
	ctx, cancel := context.WithTimeout(context.Background(), apiCtx.vrfsTimeout)
	defer cancel()

	resp, err := apiCtx.client.Ping(ctx, &pbvrfs.PingRequest{Name: "VRFS User"})
	if err != nil {
		return fmt.Errorf("VFRS service ping request fails\n%w", err)
	}

	log.Printf("%s - VRFS service is UP", resp.GetMessage())
	return nil
}
