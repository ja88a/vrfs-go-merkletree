package app

import (
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strings"

	mt "github.com/ja88a/vrfs-go-merkletree/libs/merkletree"
	mtutils "github.com/ja88a/vrfs-go-merkletree/libs/merkletree/utils"
)

// DownloadFile is the method for downloading a file, from its index as part of a known fileset, to the specified local directory
// from the Remote FS store and Verify its consistency, i.e. the hash of the downloaded file and
// the merkle proofs downloaded from VRFS are verified against the expected fileset's merkle tree root.
//
// The specified fileset ID is the one provided by the VRFS service when the fileset was initially uploaded.
// The local directory FS path for the file to be downloaded must also to specified, e.g. './localdowndir/'.
//
// An error is returned in case an issue is met.
func (ctx *ClientContext) DownloadFile(fileSetID string, fileIndex int, downDirPath string) error {
	// Inputs validation
	if len(fileSetID) == 0 {
		return fmt.Errorf("unsupported fileset ID `%v`: it must be specified", fileSetID)
	}
	if !strings.HasPrefix(fileSetID, "fs-") {
		return fmt.Errorf("unsupported fileset ID `%v`: it must be prefixed with `fs-` and made of its merkletree root hash value as provided by the VRFS when uploaded", fileSetID)
	}
	if fileIndex <= 0 {
		return fmt.Errorf("unsupported file index `%v`: it must be a positive integer >= 0", fileIndex)
	}
	if len(downDirPath) == 0 {
		return fmt.Errorf("unsupported local download directory path `%v`: it must be specified", downDirPath)
	}

	log.Printf("Downloading file #%v part of fileset '%v'", fileIndex, fileSetID)

	// 1. Retrieve the necessary download info & verification proofs from VRFS
	bucketID, mtProof, err := ctx.Vrfs.HandleDownloadFileInfoReq(TenantIDMock, fileSetID, fileIndex)
	if err != nil {
		return fmt.Errorf("failed at retrieving file download info from VRFS for file %d in fileset '%v'\n%w", fileIndex, fileSetID, err)
	}
	if mtProof == nil {
		// Trigger an actual error & end the download process as long as file is not verifiable
		return fmt.Errorf("missing the merkle tree proofs from VRFS to check for the consistency of file %4d in fileset '%v'", fileIndex, fileSetID)
	}

	// 2. Save the file locally, in the client download dir
	// Initiate the file download process from the File Storage server
	dFile, err := ctx.Nfs.DownloadFile(bucketID, fileIndex)
	if err != nil {
		return fmt.Errorf("download process has failed for file %d of fileset '%v'\n%w", fileIndex, fileSetID, err)
	}

	localDirPath := computeFilesetDownloadDir(downDirPath, fileSetID)
	if _, err := os.Stat(localDirPath); os.IsNotExist(err) {
		err = os.MkdirAll(localDirPath, os.ModePerm) // 511
		if err != nil {
			return fmt.Errorf("failed to create the target local download directory `%v`\n%w", localDirPath, err)
		}
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
	rootHashS := strings.TrimPrefix(fileSetID, FilesetNamePrefix)
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
	log.Printf("Downloaded file '%v': Successfully verified", localFilePath)

	return nil
}

// Compute the FS path where files of a fileset are locally stored
func computeFilesetDownloadDir(localFileDownloadRepo string, fileSetID string) string {
	return localFileDownloadRepo + "/" + fileSetID
}
