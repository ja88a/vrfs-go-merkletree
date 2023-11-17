// Package main implements a server for the VerifiableRemoteFileStorage service API.
package main

import (
	"log"

	config "github.com/ja88a/vrfs-go-merkletree/libs/config"
	app "github.com/ja88a/vrfs-go-merkletree/server/app"
)

func main() {
	// Configuration
	cfg, err := config.NewConfig("server-vrfs")
	if err != nil {
		log.Fatalf("Config error: %s", err)
	}
	log.Println("VRFS API config: ", cfg)
	
	// Run the server
	err = app.Run(cfg)
	if err != nil {
		log.Fatalf("Failed at running VRFS API service\n%v", err)
	}
}