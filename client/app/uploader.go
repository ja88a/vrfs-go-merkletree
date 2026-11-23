package app

import (
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"sync/atomic"

	"github.com/ja88a/vrfs-go-merkletree/client/rservice"
	mtutils "github.com/ja88a/vrfs-go-merkletree/libs/merkletree/utils"
)

// UploadFileset is the method for initiating the verified upload protocol of all files found under the specified local directory path
// The maximum batch size of files to be concurrently uploaded is specified, it must be greater than 0
func (ctx *ClientContext) UploadFileset(localDirPath string, concurrencyMax int) error {
	// Inputs validation
	if len(localDirPath) == 0 {
		return fmt.Errorf("unsupported local upload directory path `%v`: it must be specified", localDirPath)
	}
	if concurrencyMax < 1 {
		return fmt.Errorf("unsupported max batch size value `%v`: it must be a positive integer >= 1", concurrencyMax)
	}

	// Get the list of available local file paths
	files, err := mtutils.ListDirFilePaths(localDirPath)
	if err != nil {
		return fmt.Errorf("failed to list local files in dir '%v'\n%w", localDirPath, err)
	}
	if len(files) == 0 {
		return fmt.Errorf("no local files found in dir '%v'", localDirPath)
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

	// Request to VRFS for a FS bucket into which files can be remotely stored
	fileSetID := FilesetNamePrefix + hex.EncodeToString(tree.Root)
	status, bucketID, err := ctx.Vrfs.HandleFileBucketReq(TenantIDMock, fileSetID)
	if err != nil || status < 0 {
		return fmt.Errorf("missing a bucket ref to upload the fileset '%v'\n%w", fileSetID, err)
	}
	log.Printf("Bucket '%v' available for uploading files (%d)", bucketID, status)

	// Upload the local files to the Remote Files Server
	err = ctx.uploadFilesConcurrently(bucketID, files, concurrencyMax)
	if err != nil || status < 0 {
		return fmt.Errorf("failed to upload all local files to bucket '%v'\n%w", bucketID, err)
	}

	// Confirm from VRFS that the files have been correctly uploaded, by comparing the file hashes' MerkleTree roots
	filesMatch, err := ctx.confirmAndVerifyFilesetUpload(fileSetID, tree.Root)
	if err != nil {
		return fmt.Errorf("failed to verify the remotely stored files for fileset '%v' with MT root %x\n%w", fileSetID, tree.Root, err)
	}
	if !filesMatch {
		return fmt.Errorf("remotely stored files could not be verified: failure or mismatch - fileset '%v' bucket '%v'\n%w", fileSetID, bucketID, err)
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

// uploadFile is a client method for uploading a local file to the remote FileStore bucket
func (ctx *ClientContext) uploadFile(bucketID string, filePath string) error {
	if err := ctx.Nfs.UploadFile(bucketID, filePath); err != nil {
		return fmt.Errorf("failed to upload the file `%v`\n%w", filePath, err)
	}

	return nil
}

// uploadFilesConcurrently uploads the files with a bounded concurrency and waits for all of them.
// Every failure is reported. No new upload is started once a shutdown signal has interrupted one.
func (ctx *ClientContext) uploadFilesConcurrently(bucketID string, localFilePaths []string, maxConcurrentUploads int) error {
	// Buffered channel for concurrency control to upload the files
	sem := make(chan struct{}, maxConcurrentUploads)

	errs := make([]error, len(localFilePaths))
	var interrupted atomic.Bool
	var wg sync.WaitGroup

	// Loop over the files
	for i, file := range localFilePaths {
		// Acquire a slot
		sem <- struct{}{}
		if interrupted.Load() {
			<-sem
			break
		}

		// Start a new goroutine to upload this file
		wg.Add(1)
		go func(i int, file string) {
			defer wg.Done()
			defer func() {
				// Release the slot in the semaphore
				<-sem
			}()

			errs[i] = ctx.uploadFile(bucketID, file)
			if errors.Is(errs[i], rservice.ErrUploadInterrupted) {
				interrupted.Store(true)
			}
		}(i, file)
	}
	wg.Wait()

	if err := errors.Join(errs...); err != nil {
		return err
	}

	log.Printf("%d files uploaded to the remote FS bucket '%v'", len(localFilePaths), bucketID)

	return nil
}

// Confirm from the VRFS API that all local files have been properly uploaded to the File Storage server
//
// Provide the locally generated MerkleTree root so that the VRFS API compares it with its own generated
// MerkleTree root hash for the remotely stored fileset.
func (ctx *ClientContext) confirmAndVerifyFilesetUpload(fileSetID string, rootHash []byte) (bool, error) {
	// Notify VRFS that files upload is done
	status, message, err := ctx.Vrfs.HandleUploadDoneReq(TenantIDMock, fileSetID, rootHash)
	if err != nil {
		return false, fmt.Errorf("failed to confirm that remotely stored files for fileset '%v' match with local ones (root: %v)\n%w", fileSetID, rootHash, err)
	}

	// Check that the filesets' MerkleTree roots are confirmed to match
	if status == 500 {
		return false, fmt.Errorf("VRFS failed at confirming that remotely stored files for fileset '%v' match with local ones (root: %x)\nStatus %d : %v", fileSetID, rootHash, status, message)
	} else if status == 419 {
		return false, fmt.Errorf("remotely stored files for fileset '%v' do not match with local ones (root: %x)\nStatus %d : %v", fileSetID, rootHash, status, message)
	} else if status == 200 {
		log.Printf("Remote storage of fileset '%v' verified as untampered - Status %d : %v", fileSetID, status, message)
		return true, nil
	}
	return false, fmt.Errorf("unsupported status return by VRFS while checking for the remotely stored files consistency for fileset '%v' (root: %x)\nStatus %d : %v", fileSetID, rootHash, status, message)
}
