// Package main implements a client for the VRFS & the FileServer services
package main

import (
	"flag"
	"log"
	"strconv"

	srvctx "github.com/ja88a/vrfs-go-merkletree/client/rservice"
	"github.com/ja88a/vrfs-go-merkletree/client/app"
)

var (
	action      = flag.String("action", "", "The expected client action: `ping` (default); `upload` or `download` of files in the specified `dir`")
	dirpath     = flag.String("dir", "fs-playground/forupload/catyclops", "The local directory where to find files to upload or to download a file to, depending on the specified action")
	fileIndex   = flag.String("file", "0", "The index number of the file to be downloaded in the specified local dir")
	apiEndpoint = flag.String("api", "localhost:50051", "The gRPC endpoint (host & port) for the VRFS Service API")
	rfsEndpoint = flag.String("rfs", "localhost:9000", "The gRPC endpoint (host & port) for the Remote File Storage API")
	upMaxNb 		= flag.String("upnb", "5", "Max number of concurrent file uploads")
	upBatchSize = flag.String("upsize", "1048576", "Max data batch size for a file upload")
)

func main() {
	flag.Parse()

	// CLI params minimal checks
	batchUpNb, err := strconv.Atoi(*upMaxNb)
	if err != nil || batchUpNb < 1 {
		log.Fatalf("unsupported value %v for parameter 'upnb' - must be a positive integer >= 1", *upMaxNb)
	}
	upBatchSizeNb, err := strconv.Atoi(*upBatchSize)
	if err != nil || upBatchSizeNb < 1024 {
		log.Fatalf("unsupported value %v for parameter 'upsize' - must be a positive integer >= 1024", *upBatchSize)
	}

	// Initialize the client execution context
	clientCtx, err := srvctx.NewClientContext(*apiEndpoint, *rfsEndpoint, batchUpNb, upBatchSizeNb)
	if err != nil {
		log.Fatalf("Failed to establish a connection to the VRFS API\n%v", err)
	}

	// Run
	switch *action {
	case "upload":
		err := app.Upload(clientCtx, *dirpath)
		if err != nil {
			log.Fatalf("Failed to Upload files\n%v", err)
		}
	// case "download": fdownloader.Download(serverCtx, *dirpath, *fileIndex)
	default:
		log.Println("VRFS Client\nNo action specified.\nHelp Command: `./vrfs-client -h`")
		clientCtx.HandlePingVrfsReq()
	}
}
