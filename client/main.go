// Package main implements a client for the VRFS & the FileServer services
package main

import (
	"context"
	"flag"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/ja88a/vrfs-go-merkletree/libs/protos/v1/vrfs"
	up "github.com/ja88a/vrfs-go-merkletree/client/upload"
)

const (
	defaultName = "VFRS"
)

var (
	action = flag.String("action", "upload", "The expected client action: upload or download of files in the specified `dir`")
	dirpath = flag.String("dir", "fs-playground/forupload/catyclops", "The local directory where to find files to upload or to download a file to")
	vrfs = flag.String("vrfs", "localhost:50051", "The host & port of the VRFS Service API")
	nfs = flag.String("nfs", "localhost:9000", "The host & port of the Network Files Server")
	name = flag.String("name", defaultName, "Name to greet")
)

func main() {
	flag.Parse()

	up.Upload(*dirpath)

	// Set up a connection to the server.
	conn, err := grpc.Dial(*vrfs, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	
	c := pb.NewVerifiableRemoteFileStorageClient(conn)

	// Contact the server and print out its response.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r, err := c.SayHello(ctx, &pb.HelloRequest{Name: *name})
	if err != nil {
		log.Fatalf("could not greet: %v", err)
	}
	log.Printf("Greeting: %s", r.GetMessage())
}