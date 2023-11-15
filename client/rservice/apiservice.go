package rservice

import (
	"context"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pbvrfs "github.com/ja88a/vrfs-go-merkletree/libs/protos/v1/vrfs"
)

const UserMock = "umock"

// this type contains state of the server
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
}

// Constructor for the client's remote service / context
func NewClientContext(vrfsEndpoint string, nfsEndpoint string, upMaxConcurrent int, upBatchSize int) (*ApiService, error) {
	// VRFS server connection
	vrfsConn, err := grpc.Dial(vrfsEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return &ApiService{
		VrfsClient:  pbvrfs.NewVerifiableRemoteFileStorageClient(vrfsConn),
		vrfsTimeout: time.Second,

		RfsEndpoint:         nfsEndpoint,
		UploadMaxConcurrent: upMaxConcurrent,
		UploadMaxBatchSize:  upBatchSize,
	}, nil
}

// Handle the request to the VRFS API for a file bucket to upload files to the file storage server
func (clientCtx *ApiService) HandleFileBucketReq(userId string, fileSetId string) (int32, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	resp, err := clientCtx.VrfsClient.UploadBucket(ctx, &pbvrfs.UploadBucketRequest{UserId: userId, FilesetId: fileSetId})
	if err != nil {
		return -1, "", fmt.Errorf("Failed to request File Bucket for '%v'\n%w", fileSetId, err)
	}
	//return -1, "", fmt.Errorf("Failed to request File Bucket for '%v'\n%v\n%w", fileSetId, resp, fmt.Errorf("root cause"))
	return resp.Status, resp.BucketId, err
}

func (clientCtx *ApiService) HandlePingVrfsReq() {
	// Contact the server and print out its response.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	r, err := clientCtx.VrfsClient.SayHello(ctx, &pbvrfs.HelloRequest{Name: "VRFS User"})
	if err != nil {
		log.Fatalf("VFRS service ping req fails\n%v", err)
	}

	log.Printf("Greetings, %s!", r.GetMessage())
}
