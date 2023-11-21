package app

import (
	"fmt"
	"log"
	"os"
	// "strings"

	srvctx "github.com/ja88a/vrfs-go-merkletree/client/rservice"
	// mt "github.com/ja88a/vrfs-go-merkletree/libs/merkletree"
	mtfiles "github.com/ja88a/vrfs-go-merkletree/libs/merkletree/files"
)

// Batch upload of local files to the Remote FS store
func DownloadFile(ctx *srvctx.ApiService, fileSetId string, fileIndex int) error {
	log.Printf("Downloading file #%v part of fileset '%v'", fileIndex, fileSetId)

	// 1. Retrieve necessary download & verification proofs from VRFS
	bucketId, mtProofs, err := ctx.HandleDownloadFileInfoReq(srvctx.UserMock, fileSetId, fileIndex)
	if err != nil {
		return fmt.Errorf("failed at retrieving file download info from VRFS for file %d in fileset '%v'\n%w", fileIndex, fileSetId, err)
	}
	if mtProofs == nil {
		// FIXME Trigger an actual error & end the download process as long as file is not verifiable
		//return fmt.Errorf("missing the merkle proofs from VFS to check the consistency of file '%d' in fileset '%v'", fileIndex, fileSetId)
		log.Printf("ERROR missing the merkle proofs from VFS to check the consistency of file '%d' in fileset '%v'", fileIndex, fileSetId)
	}

	// 2. Save the file locally, in the client download dir
	// Initiate the file download process from the File Storage server
	ftService := srvctx.NewFileTransfer(ctx.RfsEndpoint, ctx.UploadMaxBatchSize)
	dFile, err := ftService.DownloadFile(bucketId, fileIndex)
	if err != nil {
		return fmt.Errorf("download process of file %d in FS bucket '%v'\n%w", fileIndex, bucketId, err)
	}

	localDirPath := computeFilesetDownloadDir(ctx, fileSetId)
	if _, err := os.Stat(localDirPath); os.IsNotExist(err) { 
    os.MkdirAll(localDirPath, os.ModePerm) // 511
	}
	localFilePath := localDirPath + "/" + dFile.Name
	localFile, err := os.Create(localFilePath)
	if err != nil {
		return fmt.Errorf("failed to create local file for download '%v' \n%w", localFilePath, err)
	}
	defer localFile.Close()

	_, err = localFile.ReadFrom(dFile)
	if err != nil {
		return fmt.Errorf("failed to stream file data to '%v' \n%w", localFilePath, err)
	}

	// 3. Verify the downloaded file's hash based on the fileset's MerkleTree root 
	// and the MerkleTree proofs retrieved from VRFS
	fileHashes, err := mtfiles.ComputeFileHashes([]string{localFilePath})
	if err != nil {
		return fmt.Errorf("failed to compute hash for file '%v' \n%w", localFilePath, err)
	}
	log.Printf("File '%v' downloaded Hash: %v", localFilePath, fileHashes[0])

	// rootHash := strings.TrimPrefix("fs-", fileSetId)
	// fileBlock := &mt.DataBlock{
	// 	Data: []byte(fileHashes[0]),
	// }
	// mtDefaultConfig := mt.MerkleTreeDefaultConfig()
	// ok, err := mt.Verify(fileBlock, mtProofs, []byte(rootHash), mtDefaultConfig.HashFunc)

	return nil
}

func computeFilesetDownloadDir(ctx *srvctx.ApiService, fileSetId string) string {
	return ctx.LocalFileDownloadRepo + "/" + fileSetId
}
