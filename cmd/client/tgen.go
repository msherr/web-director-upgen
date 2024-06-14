package main

import (
	"log"
	"os"
)

func getServerTgen() []byte {
	serverTgen, err := os.ReadFile("templates/server.tgen.graphml")
	if err != nil {
		log.Fatal(err)
	}
	return serverTgen
}
