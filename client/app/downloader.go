package app

import (
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strings"

	srvctx "github.com/ja88a/vrfs-go-merkletree/client/rservice"
	mt "github.com/ja88a/vrfs-go-merkletree/libs/merkletree"
	mtutils "github.com/ja88a/vrfs-go-merkletree/libs/merkletree/utils"
)

// Batch upload of local files to the Remote FS store
func DownloadFile(ctx *srvctx.ApiService, fileSetId string, fileIndex int) error {
	log.Printf("Downloading file #%v part of fileset '%v'", fileIndex, fileSetId)

	// 1. Retrieve necessary download & verification proofs from VRFS
	bucketId, mtProof, err := ctx.HandleDownloadFileInfoReq(srvctx.UserMock, fileSetId, fileIndex)
	if err != nil {
		return fmt.Errorf("failed at retrieving file download info from VRFS for file %d in fileset '%v'\n%w", fileIndex, fileSetId, err)
	}
	if mtProof == nil {
		// Trigger an actual error & end the download process as long as file is not verifiable
		return fmt.Errorf("missing the merkle tree proofs from VRFS to check for the consistency of file %4d in fileset '%v'", fileIndex, fileSetId)
	}

	// 2. Save the file locally, in the client download dir
	// Initiate the file download process from the File Storage server
	ftService := srvctx.NewFileTransfer(ctx.RfsEndpoint, ctx.UploadMaxBatchSize, DEBUG)
	dFile, err := ftService.DownloadFile(bucketId, fileIndex)
	if err != nil {
		return fmt.Errorf("download process has failed for file %d of fileset '%v'\n%w", fileIndex, fileSetId, err)
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
	fileHashes, err := mtutils.ComputeFileHashes([]string{localFilePath})
	if err != nil {
		return fmt.Errorf("failed to compute hash for file '%v' \n%w", localFilePath, err)
	}
	log.Printf("File '%v' Downloaded. Hash: %x", localFilePath, fileHashes[0])

	// Trick for avoiding the local storage of the fileset's MT root by the client
	rootHashS := strings.TrimPrefix(fileSetId, FILESET_PREFIX)
	rootHash, err := hex.DecodeString(rootHashS)
	if err != nil {
		return fmt.Errorf("failed to convert root hash to hex '%v'\n%w", rootHashS, err)
	}

	fileBlock := &mt.DataBlock{
		Data: fileHashes[0],
	}
	mtConfig := mt.MerkleTreeDefaultConfig(false)
	fileValid, err := mt.Verify(fileBlock, mtProof, rootHash, mtConfig)
	if err != nil {
		return fmt.Errorf("failed to verify the downloaded file '%v' \n%w", localFilePath, err)
	}
	if !fileValid {
		return fmt.Errorf("downloaded file '%v' fails the verification process - Root: %x  ProofSiblings: %d  Hash: %x", localFilePath, rootHash, len(mtProof.Siblings), fileHashes[0])
	} 
	log.Printf("Downloaded file '%v' Successfully verified as untampered!", localFilePath)

	return nil
}

func computeFilesetDownloadDir(ctx *srvctx.ApiService, fileSetId string) string {
	return ctx.LocalFileDownloadRepo + "/" + fileSetId
}
