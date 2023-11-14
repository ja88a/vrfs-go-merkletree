package main

import (
	"log"

	"github.com/ja88a/vrfs-go-merkletree/libs/utils/config"
	"github.com/ja88a/vrfs-go-merkletree/fileserver/app"
)

func main() {
	// Configuration
	cfg, err := config.NewConfig()
	if err != nil {
		log.Fatalf("Config error: %s", err)
	}

	log.Println("FileServer config: ", cfg)

	// Run
	app.Run(cfg)
}