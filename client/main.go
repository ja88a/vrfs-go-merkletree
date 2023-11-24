// Package main implements a client for the VRFS & the FileServer services
package main

import (
	"flag"
	"fmt"
	"log"
	"strconv"

	"github.com/ja88a/vrfs-go-merkletree/client/app"
	srvctx "github.com/ja88a/vrfs-go-merkletree/client/rservice"
)

var (
	action      = flag.String("action", "", "The expected client action: `ping` (default); `upload` or `download` of files in the specified `dir`")
	updir       = flag.String("updir", "fs-playground/forupload/catyclops", "Upload - The local directory where to find files to upload or to download a file to, depending on the specified action")
	downdir     = flag.String("downdir", "fs-playground/downloaded", "Download - The local directory where client files are downloaded to")
	fileSet     = flag.String("fileset", "fs-c32c7fac4685a43c2e7806d13bdd737a12f56322a65d23e29d81ae0a34a13019", "Download - The fileset ID to request for a file download")
	index       = flag.String("index", "0", "Download - The index number of the file to be downloaded in the specified local dir")
	apiEndpoint = flag.String("api", "localhost:50051", "The gRPC endpoint (host & port) for the VRFS Service API")
	rfsEndpoint = flag.String("fs", "localhost:9000", "The gRPC endpoint (host & port) for the Remote File Storage service")
	batchSize   = flag.String("batch", "5", "Upload - Batch size, the max number of concurrent file uploads")
	chunkSize   = flag.String("chunk", "1048576", "Upload - Max size of data chunks transfer for each file upload")
)

// Client CLI entry point
func main() {
	flag.Parse()

	// CLI params minimal checks
	batchSizeNb, err := strconv.Atoi(*batchSize)
	if err != nil || batchSizeNb < 1 {
		log.Fatalf("Unsupported value %v for parameter 'batch' - must be a positive integer >= 1", *batchSize)
	}
	chunkSizeNb, err := strconv.Atoi(*chunkSize)
	if err != nil || chunkSizeNb < 1024 {
		log.Fatalf("Unsupported value %v for parameter 'chunk' - must be a positive integer >= 1024", *chunkSize)
	}

	// Initialize the client execution context
	clientCtx, err := srvctx.NewClientContext(*apiEndpoint, *rfsEndpoint, batchSizeNb, chunkSizeNb, *downdir)
	if err != nil {
		log.Fatalf("Failed to establish a connection to the VRFS API\n%v", err)
	}

	// Run
	switch *action {
	case "upload": // Upload the files of a fileset and confirm its consistency
		err = app.Upload(clientCtx, *updir)
		if err != nil {
			log.Fatalf("Failed to remotely store and verify the fileset\n%v", err)
		}
	case "download": // Download one file of a fileset and verify it is unaltered
		fileIndex, err := strconv.Atoi(*index)
		if err != nil || fileIndex < 0 {
			log.Fatalf("Unsupported file 'index' specified %v - Must be a positive integer or 0\n%v", *index, err)
		}
		err = app.DownloadFile(clientCtx, *fileSet, fileIndex)
		if err != nil {
			log.Fatalf("File download & verify process has failed\n%v", err)
		}
	default: // Default command line info
		fmt.Printf("VRFS Client v0.1.0 2023-11\n\nNo action specified.\n\nHelp command: `./vrfs-client -h`\n\n")
		clientCtx.HandlePingVrfsReq()
	}
}
