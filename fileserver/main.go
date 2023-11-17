package main

import (
	"log"

	"github.com/ja88a/vrfs-go-merkletree/libs/config"
	"github.com/ja88a/vrfs-go-merkletree/fileserver/app"
)

func main() {
	// Configuration
	cfg, err := config.NewConfig("server-fs")
	if err != nil {
		log.Fatalf("Config error: %s", err)
	}

	log.Println("FileServer config: ", cfg)

	// Run the service
	app.Run(cfg)
}