package merkletree

import "github.com/ja88a/vrfs-go-merkletree/libs/merkletree/hash"

func MerkleTreeDefaultConfig() *Config {
	return &Config{
		HashFunc:           hash.DefaultHashFuncParallel,
		NumRoutines:        0,
		Mode:               ModeTreeBuild,
		RunInParallel:      true,
		SortSiblingPairs:   false,
		DisableLeafHashing: true,
	}
}