package app

import (
	"encoding/hex"
	"fmt"
	"log"
	"os"

	srvctx "github.com/ja88a/vrfs-go-merkletree/client/rservice"
	mtutils "github.com/ja88a/vrfs-go-merkletree/libs/merkletree/utils"
)

// Initiate the verified upload protocol of all files found under the specified local directory path
func Upload(ctx *srvctx.ApiService, localDirPath string) error {
	// Get the list of available local file paths
	files, err := mtutils.ListDirFilePaths(localDirPath)
	if err != nil || len(files) == 0 {
		return fmt.Errorf("no local files found in dir '%v'\n%w", localDirPath, err)
	}

	log.Printf("Upload - Found %d files in local dir '%v'", len(files), localDirPath)

	// Compute the local file hashes
	fileHashes, err := mtutils.ComputeFileHashes(files)
	if err != nil {
		return fmt.Errorf("failure while computing file hashes for '%v'\n%w", localDirPath, err)
	}

	// Build the Merkle Tree with the file hashes as leaf values
	tree, err := mtutils.GenerateMerkleTree(fileHashes, false)
	if err != nil {
		return fmt.Errorf("failure while computing the merkletree for files in '%v'\n%w", localDirPath, err)
	}

	// Obtain the MerkleTree root hash
	if tree.Root == nil || len(tree.Root) == 0 {
		return fmt.Errorf("invalid empty MerkleTree root '%v' for fileset in '%v'", tree.Root, localDirPath)
	}
	log.Printf("Computed fileset MerkleTree root: %x", tree.Root)

	// Request to VRFS for a bucket into which files can be remotely stored
	fileSetId := FILESET_PREFIX + hex.EncodeToString(tree.Root)
	status, bucketId, err := ctx.HandleFileBucketReq(srvctx.UserMock, fileSetId)
	if err != nil || status < 0 {
		return fmt.Errorf("missing a bucket ref to upload the fileset '%v'\n%w", fileSetId, err)
	}
	log.Printf("Bucket '%v' (%d) available for uploading files", bucketId, status)

	// Upload the local files to the Remote Files Server
	err = uploadFiles(ctx, bucketId, files)
	if err != nil || status < 0 {
		return fmt.Errorf("failed to upload all local files to bucket '%v'\n%w", bucketId, err)
	}

	// Confirm from VRFS that the files have been correctly uploaded, by comparing the file hashes' MerkleTree roots
	filesMatch, err := confirmFilesUploadIsDoneAndCorrect(ctx, fileSetId, tree.Root)
	if err != nil {
		return fmt.Errorf("failed to verify the remotely stored files for fileset '%v' with MT root %x\n%w", fileSetId, tree.Root, err)
	}
	if !filesMatch {
		return fmt.Errorf("remotely stored files could not be verified: failure or mismatch - fileset '%v' bucket '%v'\n%w", fileSetId, bucketId, err)
	}

	// Delete the client files since now stored remotely & verified
	if filesMatch {
		log.Printf("Removing local files in %v", localDirPath)
		err := os.RemoveAll(localDirPath)
		if err != nil {
			return fmt.Errorf("failed to remove the uploaded local fileset %v\n%w", localDirPath, err)
		}
	}

	return nil
}

// Batch upload of local files to the Remote FS store
func uploadFiles(ctx *srvctx.ApiService, bucketId string, localFilePaths []string) error {
	upService := srvctx.NewFileTransfer(ctx.RfsEndpoint, ctx.UploadMaxBatchSize, DEBUG)

	// Loop over the local files to trigger their parallel upload
	// TODO Batch upload of files: no more than 5 in parallel
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
func confirmFilesUploadIsDoneAndCorrect(ctx *srvctx.ApiService, fileSetId string, rootHash []byte) (bool, error) {
	// Notify VRFS that files upload is done
	status, message, err := ctx.HandleUploadDoneReq(srvctx.UserMock, fileSetId, rootHash)
	if err != nil {
		return false, fmt.Errorf("failed to confirm that remotely stored files for fileset '%v' match with local ones (root: %v)\n%w", fileSetId, rootHash, err)
	}

	// Check that the filesets' MerkleTree roots are confirmed to match
	if status == 500 {
		return false, fmt.Errorf("VRFS failed at confirming that remotely stored files for fileset '%v' match with local ones (root: %x)\nStatus %d : %v", fileSetId, rootHash, status, message)
	} else if status == 419 {
		return false, fmt.Errorf("remotely stored files for fileset '%v' do not match with local ones (root: %x)\nStatus %d : %v", fileSetId, rootHash, status, message)
	} else if status == 200 {
		log.Printf("Remote storage of fileset '%v' verified as untampered - Status %d : %v", fileSetId, status, message)
		return true, nil
	}
	return false, fmt.Errorf("unsupported status return by VRFS while checking for the remotely stored files consistency for fileset '%v' (root: %x)\nStatus %d : %v", fileSetId, rootHash, status, message)
}
