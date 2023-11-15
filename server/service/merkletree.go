package service

import (
	"fmt"

	mt "github.com/ja88a/vrfs-go-merkletree/libs/merkletree"
)

//
// Build the Merkle Tree with file hashes as leaf values
//
func GenerateMerkleTree(fileHashes []string) (*mt.MerkleTree, error) {
	// Convert to leafs
	var fileHashBlocks []mt.IDataBlock
	for _, fileHash := range fileHashes {
		block := &mt.DataBlock{
			Data: []byte(fileHash),
		}
		fileHashBlocks = append(fileHashBlocks, block)
	}
	
	// Generate
	mtConfig := mt.MerkleTreeDefaultConfig()

	tree, err := mt.New(mtConfig, fileHashBlocks)
	if err != nil {
		return nil, fmt.Errorf("failure while computing merkletree from file hashes (%d) \n%w", len(fileHashes), err)
	}

	return tree, nil
}
