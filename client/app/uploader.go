package app

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"strings"

	"path/filepath"

	srvctx "github.com/ja88a/vrfs-go-merkletree/client/rservice"
	mt "github.com/ja88a/vrfs-go-merkletree/libs/merkletree"
	hash "github.com/ja88a/vrfs-go-merkletree/libs/merkletree/hash"
)

const debug = true

// List all files (their path) found in the specified directory and its subdirs.
//
// Walks the file tree rooted at root, calling fn for each file or directory in the tree, including root.
// The files are walked in lexical order, which makes the output deterministic
func listDirFilePaths(rootDir string) ([]string, error) {
	var filePaths []string

	if _, err := os.Stat(rootDir); err != nil {
		return nil, fmt.Errorf("unsupported local directory: '%v'\n%w", rootDir, err)
	}

	err := filepath.WalkDir(rootDir, func(path string, info fs.DirEntry, err error) error {
		if !info.IsDir() {
			filePaths = append(filePaths, path)
		}
		return nil
	})

	return filePaths, err
}

//
// Initiate the verified upload protocol of all files found under the specified local directory path
//
func Upload(ctx *srvctx.ApiService, localDirPath string) error {

	// Get the list of available local file paths
	files, err := listDirFilePaths(localDirPath)
	if err != nil || len(files) == 0 {
		return fmt.Errorf("no local files found in dir '%v'\n%w", localDirPath, err)
	}

	log.Printf("Upload - Found %v files in local dir '%v'", len(files), localDirPath)

	// Compute the file hashes, converted to MT data blocks
	fileHashes, err := computeFileHashes(files)
	if err != nil {
		return fmt.Errorf("failure while computing file hashes for '%v'\n%w", localDirPath, err)
	}

	// Build the Merkle Tree with file hashes as leafs
	mtConfig := &mt.Config{
		HashFunc:           hash.DefaultHashFuncParallel,
		NumRoutines:        0,
		Mode:               mt.ModeTreeBuild,
		RunInParallel:      true,
		SortSiblingPairs:   false,
		DisableLeafHashing: true,
	}
	tree, err := mt.New(mtConfig, fileHashes)
	if err != nil {
		return fmt.Errorf("failure while computing the merkletree for files in '%v'\n%w", localDirPath, err)
	}

	// Obtain the MerkleTree root hash
	rootHash := tree.Root
	log.Printf("Computed fileset MerkleTree root: %x", rootHash)

	// Request to VRFS for a bucket into which files can be remotely stored
	fileSetId := fmt.Sprintf("%x", rootHash)
	status, bucketId, err := ctx.HandleFileBucketReq(srvctx.UserMock, fileSetId)
	if err != nil || status < 0 {
		return fmt.Errorf("missing a bucket ref to upload fileset '%x'\n%w", rootHash, err)
	}
	log.Printf("Bucket ID '%v' (%d) available for uploading files", bucketId, status)

	// Upload the files to the Files Server
	err = uploadFiles(ctx, bucketId, files)

	// Confirm to VRFS that the files upload is done

	return err
}

// Compute the hash for each file, read their content from the provided file path.
//
// File hashes are stored as a merkle tree's leaf/block.
func computeFileHashes(filePaths []string) ([]mt.IDataBlock, error) {
	fileCounter := 0
	var fileHashes []mt.IDataBlock

	filesPrefix := extractCommonPrefix(filePaths)

	for _, filePath := range filePaths {
		fileContent, err := os.ReadFile(filePath)
		if err != nil {
			return fileHashes, fmt.Errorf("Upload process failed on reading content of file '%v'\nError:\n%v", filePath, err)
		}

		// Append the unique file name in the fileset to enforce the computed hash unicity
		fileName := strings.TrimPrefix(filePath, filesPrefix)
		fileContentName := append(fileContent, fileName...)

		fileHash, err := hash.DefaultHashFuncParallel(fileContentName)
		if err != nil {
			return fileHashes, fmt.Errorf("Upload process failed on computing hash for file '%v'\nError:\n%v", filePath, err)
		}

		block := &mt.DataBlock{
			Data: fileHash,
		}
		fileHashes = append(fileHashes, block)

		if debug {
			log.Printf("File %3d Hash: %x File: %v", fileCounter, fileHash, fileName)
		}

		fileCounter += 1
	}

	return fileHashes, nil
}

func uploadFiles(ctx *srvctx.ApiService, bucketId string, localFilePaths []string) error {

	// filesPrefix := extractCommonPrefix(localFilePaths)
	upService := srvctx.NewFileTransfer(ctx.RfsEndpoint, ctx.UploadMaxBatchSize)

	for _, filePath := range localFilePaths {
		// filePath := localFilePaths[0]
		if err := upService.SendFile(bucketId, filePath); err != nil {
			return fmt.Errorf("failed to upload file `%v`\n%w", filePath, err)
		}
	}

	return nil
}

// Utility method for extracting the longest common prefix among a set of provided strings
func extractCommonPrefix(strs []string) string {
	lenStrs := len(strs)
	if lenStrs == 0 {
		return ""
	}

	firstString := strs[0]
	lenFirstString := len(firstString)

	commonPrefix := ""
	for i := 0; i < lenFirstString; i++ {
		firstStringChar := string(firstString[i])
		match := true
		for j := 1; j < lenStrs; j++ {
			if (len(strs[j]) - 1) < i {
				match = false
				break
			}
			if string(strs[j][i]) != firstStringChar {
				match = false
				break
			}
		}
		if match {
			commonPrefix += firstStringChar
		} else {
			break
		}
	}

	return commonPrefix
}
