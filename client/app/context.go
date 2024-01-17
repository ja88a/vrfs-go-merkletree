package app

import (
	"fmt"

	"github.com/ja88a/vrfs-go-merkletree/client/rservice"
)

// Client application context grouping the accesses to remote services
type ClientContext struct {
	// VRFS API service
	Vrfs rservice.VrfsService

	// File Transfer service
	Nfs rservice.FTService
}

// Init the application context
func NewClientContext(vrfsEndpoint string, nfsEndpoint string, chunkSize int) (*ClientContext, error) {
	// Init the VRFS service
	vrfsService, err := rservice.NewVrfsClient(vrfsEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to establish a connection to the VRFS API\n%v", err)
	}

	// Init the File Transfer service
	ftService := rservice.NewFileTransfer(nfsEndpoint, chunkSize, DEBUG)

	return &ClientContext{
		Vrfs: vrfsService,
		Nfs:  ftService,
	}, nil
}
