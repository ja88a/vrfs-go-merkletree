package app

import (
	"fmt"
	"log"
	"os"

	"path/filepath"

	srvctx "github.com/ja88a/vrfs-go-merkletree/client/rservice"
	mt "github.com/ja88a/vrfs-go-merkletree/libs/merkletree"
	hash "github.com/ja88a/vrfs-go-merkletree/libs/merkletree/hash"
	mtfiles "github.com/ja88a/vrfs-go-merkletree/libs/merkletree/files"
)

// Initiate the verified upload protocol of all files found under the specified local directory path
func Upload(ctx *srvctx.ApiService, localDirPath string) error {
	// Get the list of available local file paths
	files, err := mtfiles.ListDirFilePaths(localDirPath)
	if err != nil || len(files) == 0 {
		return fmt.Errorf("no local files found in dir '%v'\n%w", localDirPath, err)
	}

	log.Printf("Upload - Found %v files in local dir '%v'", len(files), localDirPath)

	// Compute the file hashes, converted to MT data blocks
	fileHashes, err := computeFileHashBlocks(files)
	if err != nil {
		return fmt.Errorf("failure while computing file hashes for '%v'\n%w", localDirPath, err)
	}

	// Build the Merkle Tree with file hashes as leaf values
	mtConfig := mt.MerkleTreeDefaultConfig()
	tree, err := mt.New(mtConfig, fileHashes)
	if err != nil {
		return fmt.Errorf("failure while computing the merkletree for files in '%v'\n%w", localDirPath, err)
	}

	// Obtain the MerkleTree root hash
	rootHash := fmt.Sprintf("%x", tree.Root)
	if rootHash == "" {
		return fmt.Errorf("invalid empty MerkleTree root for fileset in '%v'", localDirPath)
	}
	log.Printf("Computed fileset MerkleTree root: %v", rootHash)

	// Request to VRFS for a bucket into which files can be remotely stored
	fileSetId := "fs-" + rootHash
	status, bucketId, err := ctx.HandleFileBucketReq(srvctx.UserMock, fileSetId)
	if err != nil || status < 0 {
		return fmt.Errorf("missing a bucket ref to upload the fileset '%v'\n%w", rootHash, err)
	}
	log.Printf("Bucket ID '%v' (%d) available for uploading files", bucketId, status)

	// Upload the local files to the Remote Files Server
	err = uploadFiles(ctx, bucketId, files)
	if err != nil || status < 0 {
		return fmt.Errorf("failed to upload all local files to bucket '%v'\n%w", bucketId, err)
	}

	// Confirm from VRFS that the files have been correctly uploaded, by comparing the file hashes' MerkleTree roots
	filesMatch, err := confirmFilesUploadIsDoneAndCorrect(ctx, fileSetId, rootHash)
	if err != nil {
		return fmt.Errorf("failed to verify the remotely stored files for fileset '%v' with MT root %v\n%w", fileSetId, rootHash, err)
	}
	if !filesMatch {
		return fmt.Errorf("remotely stored files could not be verified: failure or mismatch - bucket '%v'\n%w", bucketId, err)
	}

	// Delete the client files since now stored remotely & verified
	removeLocalFiles := false
	if filesMatch && removeLocalFiles {
		log.Println("Removing local files")
		for _, filePath := range files {
			err := os.RemoveAll(filePath)
			if err != nil {
				return fmt.Errorf("deletion of local files failed on file '%v'\n%w", filePath, err)
			}
		}
	}

	return nil
}

// Compute the hash for each file, read their content from the provided file path.
//
// File hashes are stored as a merkle tree's leaf/block.
func computeFileHashBlocks(filePaths []string) ([]mt.IDataBlock, error) {
	fileCounter := 0
	var fileHashBlocks []mt.IDataBlock

	// Loop over the dir files
	for _, filePath := range filePaths {
		fileContent, err := os.ReadFile(filePath)
		if err != nil {
			return fileHashBlocks, fmt.Errorf("Upload process failed on reading content of file '%v'\nError:\n%v", filePath, err)
		}

		// Append the unique file name in the fileset to enforce the computed hash unicity
		fileName := filepath.Base(filePath)
		fileContentName := append(fileContent, fileName...)

		// Hash computation
		fileHash, err := hash.DefaultHashFuncParallel(fileContentName)
		if err != nil {
			return fileHashBlocks, fmt.Errorf("Upload process failed on computing hash for file '%v'\nError:\n%v", filePath, err)
		}
		fileHashS := fmt.Sprintf("%x", fileHash)

		if debug {
			//log.Printf("File %3d Hash: %x File: %v", fileCounter, fileHash, fileName)
			fmt.Printf("File %3d Hash: %v File: %v\n", fileCounter, fileHashS, fileName)
			fileCounter += 1
		}

		// Convert
		block := &mt.DataBlock{
			Data: []byte(fileHashS), // fileHash,
		}
		fileHashBlocks = append(fileHashBlocks, block)
	}

	return fileHashBlocks, nil
}

// Batch upload of local files to the Remote FS store
func uploadFiles(ctx *srvctx.ApiService, bucketId string, localFilePaths []string) error {
	upService := srvctx.NewFileTransfer(ctx.RfsEndpoint, ctx.UploadMaxBatchSize)

	// Loop over the local files
	// TODO handle the parallel uploads of files
	for _, filePath := range localFilePaths {
		if err := upService.SendFile(bucketId, filePath); err != nil {
			return fmt.Errorf("failed to batch upload the file `%v`\n%w", filePath, err)
		}
	}

	log.Printf("%d files uploaded to the remote FS bucket '%v'", len(localFilePaths), bucketId)

	return nil
}

// Confirm from the VRFS API that all local files have been properly uploaded to the File Storage server
//
// Provide the locally generated MerkleTree root so that the VRFS API compares it with its own generated
// MerkleTree root hash for the remotely stored fileset.
func confirmFilesUploadIsDoneAndCorrect(ctx *srvctx.ApiService, fileSetId string, rootHash string) (bool, error) {
	// Notify VRFS that files upload is done
	status, message, err := ctx.HandleUploadDoneReq(srvctx.UserMock, fileSetId, rootHash)
	if err != nil {
		return false, fmt.Errorf("failed to confirm that remotely stored files for fileset '%v' match with local ones (root: %v)\n%w", fileSetId, rootHash, err)
	}

	// Check that the filesets' MerkleTree roots are confirmed to match
	if status == 500 {
		return false, fmt.Errorf("VRFS failed at confirming that remotely stored files for fileset '%v' match with local ones (root: %x)\nStatus %d : %v", fileSetId, rootHash, status, message)
	} else if status == 419 {
		return false, fmt.Errorf("remotely stored files for fileset '%v' do not match with local ones (root: %v)\nStatus %d : %v", fileSetId, rootHash, status, message)
	} else if status == 200 {
		log.Printf("Remote storage of fileset '%v' verified as untampered - Status %d : %v", fileSetId, status, message)
		return true, nil
	}
	return false, fmt.Errorf("unsupported status return by VRFS while checking for the remotely stored files consistency for fileset '%v' (%x)\nStatus %d : %v", fileSetId, rootHash, status, message)
}
