// Package main implements the client for the VRFS & the FileServer services
package main

import (
	"flag"
	"fmt"
	"log"
	"strconv"
	"strings"

	app "github.com/ja88a/vrfs-go-merkletree/client/app"
	srvctx "github.com/ja88a/vrfs-go-merkletree/client/rservice"
)

// CLI command parameters
var (
	action      = flag.String("action", "", "Expected client action: `ping` (default); `upload` or `download` of files in the specified `dir`")
	updir       = flag.String("updir", "fs-playground/forupload/catyclops", "Upload - The local directory where to find the files to upload")
	downdir     = flag.String("downdir", "fs-playground/downloaded", "Download - The local directory where client files are downloaded to")
	fileSet     = flag.String("fileset", "fs-10f652e11f5e2f799481ed02d45a74bcf3d62dea3200ad08120bba43c242f5fb", "Download* - The fileset ID to request for a file download")
	index       = flag.String("index", "0", "Download* - The index number of the file to be downloaded in the specified local dir")
	apiEndpoint = flag.String("api", "localhost:50051", "The gRPC endpoint (host & port) for the VRFS Service API")
	rfsEndpoint = flag.String("fs", "localhost:9000", "The gRPC endpoint (host & port) for the Remote File Storage service")
	batchSize   = flag.String("batch", "5", "Upload - Batch size, the max number of concurrent file uploads")
	chunkSize   = flag.String("chunk", "1048576", "Max size of data chunks transfer for each file upload or download")
)

// Client CLI entry point
func main() {
	flag.Parse()

	// Initialize the client execution context
	clientCtx, err := srvctx.NewClientContext(*apiEndpoint, *rfsEndpoint)
	if err != nil {
		log.Fatalf("Failed to establish a connection to the VRFS API\n%v", err)
	}

	// Run
	switch *action {
	case "upload":
		// CLI params minimal checks
		batchSizeNb, err := strconv.Atoi(*batchSize)
		if err != nil || batchSizeNb < 1 {
			log.Fatalf("Unsupported value %v for parameter 'batch' - must be a positive integer >= 1", *batchSize)
		}
		chunkSizeNb, err := strconv.Atoi(*chunkSize)
		if err != nil || chunkSizeNb < 1024 {
			log.Fatalf("Unsupported value %v for parameter 'chunk' - must be a positive integer >= 1024", *chunkSize)
		}

		// Upload the files of a fileset and confirm its consistency
		err = app.Upload(clientCtx, *updir, batchSizeNb, chunkSizeNb)
		if err != nil {
			log.Fatalf("Failed to remotely store and verify the fileset\n%v", err)
		}

	case "download":
		// Download one file of a given fileset and verify it is unaltered
		if *fileSet == "" || !strings.HasPrefix(*fileSet, "fs-") {
			log.Fatalf("A `fileset` parameter must be specified using the pattern 'fs-HexFilesetRootHash'. The HexFilesetRootHash, and the complete fileset ID are provided in logs of previous fileset upload process")
		}
		fileIndex, err := strconv.Atoi(*index)
		if err != nil || fileIndex < 0 {
			log.Fatalf("Unsupported file index specified '%v' - Must be a positive integer or 0\n%v", *index, err)
		}
		chunkSizeNb, err := strconv.Atoi(*chunkSize)
		if err != nil || chunkSizeNb < 1024 {
			log.Fatalf("Unsupported value %v for parameter 'chunk' - must be a positive integer >= 1024", *chunkSize)
		}
		err = app.DownloadFile(clientCtx, *fileSet, fileIndex, *downdir, chunkSizeNb)
		if err != nil {
			log.Fatalf("File download & verification process has failed\n%v", err)
		}

	default:
		// Default command line info
		fmt.Printf("VRFS Client v0.1.0 2023-11\n\nNo action specified.\n\nHelp command: `vrfs-client -h`\n\n")
		clientCtx.HandlePingVrfsReq()
	}
}
