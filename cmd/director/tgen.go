package main

import (
	_ "embed"
)

//go:embed templates/server.tgen.graphml
var serverTGenBytes []byte

//go:embed templates/client.tgen.graphml
var clientTGenBytes []byte

func getServerTgen() []byte {
	return serverTGenBytes
}

func getClientTgen() []byte {
	return clientTGenBytes
}
