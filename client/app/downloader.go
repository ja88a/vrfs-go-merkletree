package app

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	mt "github.com/ja88a/vrfs-go-merkletree/libs/merkletree"
	mtutils "github.com/ja88a/vrfs-go-merkletree/libs/merkletree/utils"
	rpcfile "github.com/ja88a/vrfs-go-merkletree/libs/rpcapi/file"
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
	if fileIndex < 0 {
		return fmt.Errorf("unsupported file index `%v`: it must be an integer >= 0", fileIndex)
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

	// 2. Save the file in a temporary location of the client download dir
	// Initiate the file download process from the File Storage server
	dFile, err := ctx.Nfs.DownloadFile(bucketID, fileIndex)
	if err != nil {
		return fmt.Errorf("download process has failed for file %d of fileset '%v'\n%w", fileIndex, fileSetID, err)
	}
	defer dFile.Close()

	// The file name is supplied by the server: it must not be able to leave the download directory
	if !rpcfile.IsSafeName(dFile.Name) {
		return fmt.Errorf("unsafe file name %q received for file %d of fileset '%v'", dFile.Name, fileIndex, fileSetID)
	}

	localDirPath := computeFilesetDownloadDir(downDirPath, fileSetID)
	if err := os.MkdirAll(localDirPath, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create the target local download directory `%v`\n%w", localDirPath, err)
	}

	// A temporary directory lets the file keep its name, which is part of its hash,
	// and being on the same filesystem makes the final rename atomic
	tmpDirPath, err := os.MkdirTemp(localDirPath, ".download-*")
	if err != nil {
		return fmt.Errorf("failed to create a temporary download directory in `%v`\n%w", localDirPath, err)
	}
	defer os.RemoveAll(tmpDirPath)

	tmpFilePath := filepath.Join(tmpDirPath, dFile.Name)
	if err := saveToFile(tmpFilePath, dFile); err != nil {
		return fmt.Errorf("failed to stream file data to '%v' \n%w", tmpFilePath, err)
	}

	// 3. Verify the downloaded file's hash based on the fileset's MerkleTree root
	// and the MerkleTree proofs retrieved from VRFS
	localFilePath := filepath.Join(localDirPath, dFile.Name)
	fileHashes, err := mtutils.ComputeFileHashes([]string{tmpFilePath})
	if err != nil {
		return fmt.Errorf("failed to compute hash for file '%v' \n%w", tmpFilePath, err)
	}
	log.Printf("File '%v' Downloaded. Hash: %x", dFile.Name, fileHashes[0])

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
		return fmt.Errorf("failed to verify the downloaded file '%v' \n%w", dFile.Name, err)
	}
	if !fileValid {
		return fmt.Errorf("downloaded file '%v' fails the verification process - Root: %x  ProofSiblings: %d  Hash: %x", dFile.Name, rootHash, len(mtProof.Siblings), fileHashes[0])
	}

	// 4. Only a verified file reaches its final path, replacing any previous copy atomically
	if err := os.Rename(tmpFilePath, localFilePath); err != nil {
		return fmt.Errorf("failed to move the verified file to '%v' \n%w", localFilePath, err)
	}
	log.Printf("Downloaded file '%v': Successfully verified", localFilePath)

	return nil
}

// saveToFile writes the content of r to a new file at the specified path
func saveToFile(path string, r io.Reader) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	_, err = file.ReadFrom(r)
	return errors.Join(err, file.Close())
}

// Compute the FS path where files of a fileset are locally stored
func computeFilesetDownloadDir(localFileDownloadRepo string, fileSetID string) string {
	return localFileDownloadRepo + "/" + fileSetID
}
