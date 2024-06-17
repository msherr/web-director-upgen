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

func getClientTgen() []byte {
	clientTgen, err := os.ReadFile("templates/client.tgen.graphml")
	if err != nil {
		log.Fatal(err)
	}
	return clientTgen
}
