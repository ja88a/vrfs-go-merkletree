// Package main implements the client for the VRFS & the FileServer services
package main

import (
	"flag"
	"fmt"
	"log"
	"strconv"
	"strings"

	app "github.com/ja88a/vrfs-go-merkletree/client/app"
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

	chunkSizeNb, err := strconv.Atoi(*chunkSize)
	if err != nil || chunkSizeNb < 1024 {
		log.Fatalf("Unsupported value %v for parameter 'chunk' - must be a positive integer >= 1024", *chunkSize)
	}

	// Initialize the client execution context
	appCtx, err := app.NewClientContext(*apiEndpoint, *rfsEndpoint, chunkSizeNb)
	if err != nil {
		log.Fatalf("Failed to establish a connection to the VRFS API\n%v", err)
	}

	// Run
	switch *action {
	case "upload":
		// CLI parameters minimal checks
		batchSizeNb, err := strconv.Atoi(*batchSize)
		if err != nil || batchSizeNb < 1 {
			log.Fatalf("Unsupported value %v for parameter 'batch' - must be a positive integer >= 1", *batchSize)
		}

		// Upload the files of a fileset and confirm its consistency
		err = appCtx.UploadFileset(*updir, batchSizeNb)
		if err != nil {
			log.Fatalf("Failed to remotely store and verify the fileset\n%v", err)
		}

	case "download":
		// CLI parameters minimal checks
		if *fileSet == "" || !strings.HasPrefix(*fileSet, "fs-") {
			log.Fatalf("A `fileset` parameter must be specified using the pattern 'fs-HexFilesetRootHash'. The HexFilesetRootHash and the complete fileset ID are provided in logs of previous fileset upload process")
		}
		fileIndex, err := strconv.Atoi(*index)
		if err != nil || fileIndex < 0 {
			log.Fatalf("Unsupported file index specified '%v' - Must be a positive integer or 0\n%v", *index, err)
		}
		chunkSizeNb, err := strconv.Atoi(*chunkSize)
		if err != nil || chunkSizeNb < 1024 {
			log.Fatalf("Unsupported value %v for parameter 'chunk' - must be a positive integer >= 1024", *chunkSize)
		}

		// Download one file of a given fileset and verify it is unaltered
		err = appCtx.DownloadFile(*fileSet, fileIndex, *downdir)
		if err != nil {
			log.Fatalf("File download & verification process has failed\n%v", err)
		}

	default:
		// Default command line info
		fmt.Printf("VRFS Client v0.1.0 2023-11\n\nNo action specified.\n\nHelp command: `vrfs-client -h`\n\n")
		appCtx.ServiceVrfs.HandlePingVrfsReq()
	}
}
