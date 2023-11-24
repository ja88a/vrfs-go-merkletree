package utils

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/ja88a/vrfs-go-merkletree/libs/merkletree/hash"
)

// List all files (their path) found in the specified directory and its subdirs.
//
// Walks the file tree rooted at root, calling fn for each file or directory in the tree, including root.
// The files are walked in lexical order, which makes the output deterministic
func ListDirFilePaths(rootDir string) ([]string, error) {
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

// TODO Return [][]byte instead - Remove unecessary string conversions
// Compute the content hash of all the provided files
func ComputeFileHashes(filePaths []string) ([]string, error) {
	fileCounter := 0
	var fileHashes []string

	// Loop over the dir files
	for _, filePath := range filePaths {
		fileContent, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("files hashing process failed on reading content of file '%v'\nError:\n%v", filePath, err)
		}
		
		// Append the unique file name in the fileset to enforce the computed hash unicity
		fileName := filepath.Base(filePath)
		fileContentName := append(fileContent, fileName...)

		// Hash computation
		fileHash, err := hash.DefaultHashFuncParallel(fileContentName)
		if err != nil {
			return nil, fmt.Errorf("upload process failed on computing hash for file '%v'\n%w", filePath, err)
		}
		fileHashes = append(fileHashes, fmt.Sprintf("%x", fileHash))

		fileCounter += 1
	}

	return fileHashes, nil
}

// Utility method for extracting the longest common prefix among a set of provided strings
// Use that technique instead of filepath.Base(filePath) to enable the support for subdirs
// Once: filesPrefix := extractCommonPrefix(filePaths)
// Then: strings.TrimPrefix(filePath, filesPrefix)
// func ExtractCommonPrefix(strs []string) string {
// 	lenStrs := len(strs)
// 	if lenStrs == 0 {
// 		return ""
// 	}
// 	firstString := strs[0]
// 	lenFirstString := len(firstString)
// 	commonPrefix := ""
// 	for i := 0; i < lenFirstString; i++ {
// 		firstStringChar := string(firstString[i])
// 		match := true
// 		for j := 1; j < lenStrs; j++ {
// 			if (len(strs[j]) - 1) < i {
// 				match = false
// 				break
// 			}
// 			if string(strs[j][i]) != firstStringChar {
// 				match = false
// 				break
// 			}
// 		}
// 		if match {
// 			commonPrefix += firstStringChar
// 		} else {
// 			break
// 		}
// 	}

// 	return commonPrefix
// }
