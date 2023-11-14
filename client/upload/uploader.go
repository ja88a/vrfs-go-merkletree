package upload

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"strings"

	"path/filepath"

	mt "github.com/ja88a/vrfs-go-merkletree/libs/merkletree"
	hash "github.com/ja88a/vrfs-go-merkletree/libs/merkletree/hash"
)

const debug = true

// List all files (their path) found in the specified directory and its subdirs.
//
// Walks the file tree rooted at root, calling fn for each file or directory in the tree, including root.
// The files are walked in lexical order, which makes the output deterministic
func GetListDirFiles(root string) ([]string, error) {
	var filePaths []string

	if _, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
		log.Fatalf("Unknown Directory: '%v'", root)
	}

	err := filepath.WalkDir(root, func(path string, info fs.DirEntry, err error) error {
		if !info.IsDir() {
			filePaths = append(filePaths, path)
		}
		return nil
	})

	return filePaths, err
}

// Initiate the verified upload protocol of all files found under the specified local directory path
func Upload(localDirPath string) {

	// Get the list of available local file paths
	files, err := GetListDirFiles(localDirPath)
	if err != nil || len(files) == 0 {
		log.Fatalf("No local files found in dir '%v'\n%v", localDirPath, err)
	}

	log.Printf("Upload - Found %v files in local dir '%v'", len(files), localDirPath)

	// Compute the file hashes, converted to MT data blocks
	fileHashes, err := computeFileHashes(files)
	if err != nil {
		log.Fatalf("Failure while computing File Hashes for '%v'\n%v", localDirPath, err)
	}

	// Build the Merkle Tree with file hashes as leafs
	mtConfig := &mt.Config{
		HashFunc: hash.DefaultHashFuncParallel,
		NumRoutines: 0,
		Mode: mt.ModeTreeBuild,
		RunInParallel: true,
		SortSiblingPairs: false,
		DisableLeafHashing: true,
	}
	tree, err := mt.New(mtConfig, fileHashes)
	if err != nil {
		log.Fatalf("Failure while computing the MerkleTree for files in '%v'\n%v", localDirPath, err)
	}

	// Obtain the MerkleTree root hash
	rootHash := tree.Root
	log.Printf("Computed fileset MerkleTree root: %x", rootHash)

	// Request to VRFS for a bucket into which files can be stored

	// Upload the files to the Files Server

	// Confirm to VRFS that the files upload is done

}

// Compute the hash for each file, read their content from the provided file path
// File hashes are stored as a merkle tree's leaf/block
// 
// TODO Multi-threading: compute files' hash in parallel instead of sequentially
func computeFileHashes(filePaths []string) ([]mt.IDataBlock, error) {
	fileCounter := 0
	var fileHashes []mt.IDataBlock

	filePrefix := longestCommonPrefix(filePaths)

	for _, filePath := range filePaths {
		fileContent, err := os.ReadFile(filePath)
		if err != nil {
			return fileHashes, fmt.Errorf("Upload process failed on reading content of file '%v'\nError:\n%v", filePath, err)
		}

		fileName := strings.TrimPrefix(filePath, filePrefix)
		uniqueFile := append(fileContent, fileName...)

		fileHash, err := hash.DefaultHashFuncParallel(uniqueFile)
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

func longestCommonPrefix(strs []string) string {
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
