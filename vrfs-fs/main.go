package main

import (
	"log"

	"github.com/ja88a/vrfs-go-merkletree/libs/config"
	"github.com/ja88a/vrfs-go-merkletree/vrfs-fs/app"
)

func main() {
	// Configuration
	cfg, err := config.NewConfig("vrfs-fs")
	if err != nil {
		log.Fatalf("Config error: %s", err)
	}

	log.Println("FileServer config: ", cfg)

	// Run the service
	app.Run(cfg)
}
