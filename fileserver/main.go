package main

import (
	"log"

	"github.com/ja88a/vrfs-go-merkletree/libs/utils/config"
	"github.com/ja88a/vrfs-go-merkletree/fileserver/app"
)

func main() {
	// Configuration
	cfg, err := config.NewConfig("server-rfs")
	if err != nil {
		log.Fatalf("Config error: %s", err)
	}

	log.Println("FileServer config: ", cfg)

	// Run the service
	app.Run(cfg)
}