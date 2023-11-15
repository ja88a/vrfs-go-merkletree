package app

import (
	"fmt"
	"log"
	"os"

	srvctx "github.com/ja88a/vrfs-go-merkletree/client/rservice"
	fileutils "github.com/ja88a/vrfs-go-merkletree/libs/utils/files"
)

// Batch upload of local files to the Remote FS store
func DownloadFile(ctx *srvctx.ApiService, fileSetId string, fileIndex int) error {
	log.Printf("Downloading file #%v part of fileset '%v'", fileIndex, fileSetId)

	// Retrieve necessary download & verification proofs from VRFS
	bucketId, _, err := ctx.HandleDownloadFileInfoReq(srvctx.UserMock, fileSetId, fileIndex)
	if err != nil {
		return fmt.Errorf("failed at retrieving file download info from VRFS for file %d in fileset '%v'\n%w", fileIndex, fileSetId, err)
	}

	// Initiate the file download from the File Store server
	ftService := srvctx.NewFileTransfer(ctx.RfsEndpoint, ctx.UploadMaxBatchSize)

	dFile, err := ftService.DownloadFile(bucketId, fileIndex)
	if err != nil {
		return fmt.Errorf("download process of file %d in FS bucket '%v'\n%w", fileIndex, bucketId, err)
	}

	// Save the file in a local dir
	fileName := dFile.Name
	localFilePath := computeFilesetDownloadDir(ctx, fileSetId) + fileName
	localFile, err := os.Create(localFilePath)
	if err != nil {
		return fmt.Errorf("failed to create local download dir '%v' \n%w", localFilePath, err)
	}
	defer localFile.Close()

	_, _ = localFile.ReadFrom(dFile)

	// Verify the file hash to confirm it is not tampered, based on the local MerkleTree root and the retrieved proofs from VRFS
	// Append the unique file name in the fileset to enforce the computed hash unicity
	fileHashes, err := fileutils.ComputeFileHashes([]string{localFilePath})
	if err != nil {
		return fmt.Errorf("failed to compute hash for file '%v' \n%w", localFilePath, err)
	}

	log.Printf("File #%d\tName : %v\tHash : %v", fileIndex, fileName, fileHashes[0])

	return nil
}

func computeFilesetDownloadDir(ctx *srvctx.ApiService, fileSetId string) string {
	return ctx.LocalFileDownloadRepo + "/" + fileSetId + "/"
}
