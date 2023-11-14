// Package provides an implementation of the DataBlock interface.
package merkletree

// IDataBlock is the interface for input data blocks used to generate the Merkle Tree.
//
// Implementations of IDataBlock should provide a serialization method
// that converts the data block into a byte slice for hashing purposes.
type IDataBlock interface {
	// Serialize converts the data block into a byte slice.
	// It returns the serialized byte slice and an error, if any occurs during the serialization process.
	Serialize() ([]byte, error)
}

// DataBlock is a mock implementation of the DataBlock interface.
type DataBlock struct {
	Data []byte
}

// Serialize returns the serialized form of the DataBlock.
func (t *DataBlock) Serialize() ([]byte, error) {
	return t.Data, nil
}
