package merkletree

import "github.com/ja88a/vrfs-go-merkletree/libs/merkletree/hash"

// Commonly shared default config for generating Merkle Trees from filesets' file hashes
func MerkleTreeDefaultConfig(proofsGen bool) *Config {
	treeGenMode := ModeTreeBuild
	if proofsGen {
		treeGenMode = ModeProofGenAndTreeBuild
	}
	return &Config{
		HashFunc:           hash.DefaultHashFuncParallel,
		NumRoutines:        0,
		Mode:               treeGenMode,
		RunInParallel:      true,
		SortSiblingPairs:   false,
		DisableLeafHashing: true,
	}
}
