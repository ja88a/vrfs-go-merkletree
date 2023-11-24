package service

import (
	"fmt"

	mt "github.com/ja88a/vrfs-go-merkletree/libs/merkletree"
)

// Build the Merkle Tree with file hashes as leaf values
//
// Specify if MerkleTree proofs are also to be generated via `generateProofs`
func GenerateMerkleTree(fileHashes []string, generateProofs bool) (*mt.MerkleTree, error) {
	// Convert to leafs
	var fileHashBlocks []mt.IDataBlock
	for _, fileHash := range fileHashes {
		block := &mt.DataBlock{
			Data: []byte(fileHash), // TODO No string conversion but hex bytes - Remove unecessary string conversions
		}
		fileHashBlocks = append(fileHashBlocks, block)
	}

	// Generate
	mtConfig := mt.MerkleTreeDefaultConfig(generateProofs)

	tree, err := mt.New(mtConfig, fileHashBlocks)
	if err != nil {
		return nil, fmt.Errorf("failure while computing merkletree from file hashes (%d) \n%w", len(fileHashes), err)
	}

	return tree, nil
}
