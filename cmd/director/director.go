package main

import (
	"context"
	"crypto/tls"
	"datamodel"
	_ "embed"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type AuthTokenKeyType string
type ClientKeyType string
type URLEndpointType string

const AuthTokenKey = AuthTokenKeyType("authToken")
const ClientKey = ClientKeyType("client")
const URLEndpointKey = URLEndpointType("urlEndpoint")

type TransportType int

const (
	undefinedTransport TransportType = iota
	obfsTransport
	proteusTransport
)

var allJobs = struct {
	JobID int `json:"job"`
}{
	JobID: -1,
}

func (t TransportType) String() string {
	return [...]string{"undefined", "obfs", "proteus"}[t]
}

// starts the bridge
func startBridge(ctxBridge context.Context, transportType TransportType, configNum int, expName, tgenPath, ptAdapterPath string) {

	// send the server.tgen.graphml file to the bridge
	graphMLBytes := getServerTgen()
	if res := sendFile(ctxBridge, "server.tgen.graphml", graphMLBytes); res != http.StatusOK {
		log.Fatal("could not send server.tgen.graphml to bridge")
	}

	var ptAdapterConfigBytes []byte

	switch transportType {
	case undefinedTransport:
		log.Fatal("transport type not defined")
	case obfsTransport:
		ptAdapterConfigBytes = getObfsPTAdapterServerTemplate(configNum)
	case proteusTransport:
		log.Fatal("Proteus transport not implemented yet")
	}

	if res := sendFile(ctxBridge, "ptadapter.server.conf", ptAdapterConfigBytes); res != http.StatusOK {
		log.Fatal("could not send ptadapter.server.conf to bridge")
	}

	// next, cause the bridge to call tgen on that file
	tgenCmd := datamodel.JsonCommandStruct{
		TimeoutInSecs: 0,
		Cmd:           tgenPath,
		Args:          []string{"server.tgen.graphml"},
		StdoutFile:    fmt.Sprintf("tgen.bridge.%s.%d.log", expName, configNum),
		StderrFile:    fmt.Sprintf("tgen.bridge.%s.%d.err", expName, configNum),
	}
	if res := makeRequest(ctxBridge, "/runInBackground", tgenCmd); res != http.StatusOK {
		log.Fatal("could not start tgen on bridge")
	}

	// next, cause the bridge to call ptadapter
	ptAdapterCommand := datamodel.JsonCommandStruct{
		TimeoutInSecs: 0,
		Cmd:           ptAdapterPath,
		Args:          []string{"-S", "ptadapter.server.conf"},
		StdoutFile:    fmt.Sprintf("ptadapter.bridge.%s.%d.log", expName, configNum),
		StderrFile:    fmt.Sprintf("ptadapter.bridge.%s.%d.err", expName, configNum),
	}
	if res := makeRequest(ctxBridge, "/runInBackground", ptAdapterCommand); res != http.StatusOK {
		log.Fatal("could not start ptadapter on bridge")
	}

	// report out what's running
	makeRequest(ctxBridge, "/jobs", nil)
}

func startOpenGFW(ctxGFW context.Context, expName string) {

	startOpenGFWCommand := datamodel.JsonCommandStruct{
		TimeoutInSecs: 0,
		Cmd:           "../OpenGFW/OpenGFW",
		Args:          []string{"-c", "../OpenGFW/configs/config.yaml", "../OpenGFW/rules/ruleset.yaml"},
		StdoutFile:    "OpenGFW." + expName + ".log",
		StderrFile:    "OpenGFW." + expName + ".err",
	}
	log.Println("Starting OpenGFW")
	if res := makeRequest(ctxGFW, "/runInBackground", startOpenGFWCommand); res != http.StatusOK {
		log.Fatal("could not start OpenGFW")
	}
	time.Sleep(2 * time.Second)

	makeRequest(ctxGFW, "/jobs", nil)
}

func stopAllJobs(ctxs []*context.Context) {
	log.Println("stopping all jobs")
	for _, ctx := range ctxs {
		makeRequest(*ctx, "/kill", allJobs)
	}
}

func main() {
	// environment and command-line vars
	var (
		expName             string
		authToken           string
		censoredUrlEndpoint string
		gfwUrlEndpoint      string
		bridgeUrlEndpoint   string
		ptAdapterPath       string
		tgenPath            string
		insecure            bool
	)
	var ctxGFW, ctxCensoredVM, ctxBridge context.Context

	authToken = os.Getenv("SERVER_AUTH_TOKEN")
	if authToken == "" {
		log.Fatal("SERVER_AUTH_TOKEN not set")
	}

	flag.StringVar(&expName, "exp", "", "experiment name")
	flag.BoolVar(&insecure, "insecure", false, "Set to disable TLS verification (on all endpoints)")
	flag.StringVar(&gfwUrlEndpoint, "gfw_url", "", "Specify the URL endpoint for OpenGFW")
	flag.StringVar(&censoredUrlEndpoint, "censoredvm_url", "", "Specify the URL endpoint for censored VM")
	flag.StringVar(&bridgeUrlEndpoint, "bridge_url", "", "Specify the URL endpoint for the bridge")
	flag.StringVar(&ptAdapterPath, "ptadapter", "/usr/local/bin/ptadapter", "path to ptadapter on both bridge and censored VM")
	flag.StringVar(&tgenPath, "tgen", "/usr/local/bin/tgen", "path to tgen on both bridge and censored VM")
	flag.Parse()

	if expName == "" || gfwUrlEndpoint == "" ||
		censoredUrlEndpoint == "" || bridgeUrlEndpoint == "" {
		flag.Usage()
		os.Exit(1)
	}

	if insecure {
		log.Println("Warning: Skipping TLS verification")
	}

	tr := &http.Transport{
		MaxIdleConns:        10,
		IdleConnTimeout:     3 * time.Minute,
		DisableCompression:  true,
		TLSHandshakeTimeout: 5 * time.Second,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: insecure}, // DANGER
	}
	client := &http.Client{Transport: tr}
	defer client.CloseIdleConnections()

	// create our contexts
	ctxGFW = context.WithValue(context.Background(), AuthTokenKey, authToken)
	ctxGFW = context.WithValue(ctxGFW, ClientKey, client)
	ctxGFW = context.WithValue(ctxGFW, URLEndpointKey, gfwUrlEndpoint)
	ctxCensoredVM = context.WithValue(ctxGFW, URLEndpointKey, censoredUrlEndpoint)
	ctxBridge = context.WithValue(ctxGFW, URLEndpointKey, bridgeUrlEndpoint)

	// make sure everything is shut down
	stopAllJobs([]*context.Context{&ctxGFW, &ctxCensoredVM, &ctxBridge})
	time.Sleep(2 * time.Second)

	// start OpenGFW
	startOpenGFW(ctxGFW, expName)

	for ttype := range []TransportType{obfsTransport, proteusTransport} {
		for configNum := 1; configNum <= 10000; configNum++ {

			// make sure bridge and client aren't doing anything
			stopAllJobs([]*context.Context{&ctxCensoredVM, &ctxBridge})
			time.Sleep(2 * time.Second)

			// notify opengfw of our configuration
			digCmd := datamodel.JsonCommandStruct{
				TimeoutInSecs: 0,
				Cmd:           "dig",
				Args: []string{
					fmt.Sprintf("exp_%s.trans_%v.iter_%d.log.message",
						strings.ReplaceAll(expName, " ", ""),
						ttype,
						configNum),
				},
			}
			makeRequest(ctxCensoredVM, "/runToCompletion", digCmd)

			// start ptadapter and tgen on the bridge
			startBridge(ctxBridge, obfsTransport, configNum, expName, tgenPath, ptAdapterPath)

			// TODO: start ptadapter on the censored VM STOPPED HERE
			obsClientTemplate := getObsClientTemplate()
			sendFile(ctxCensoredVM, "ptadapter-obs-client", obsClientTemplate)

		}
	}

	time.Sleep(60 * time.Second)
	log.Println("Killing all jobs")
	makeRequest(ctxCensoredVM, "/kill", allJobs)
	makeRequest(ctxGFW, "/kill", allJobs)
	makeRequest(ctxBridge, "/kill", allJobs)
}