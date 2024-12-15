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
	switch t {
	case undefinedTransport:
		return "undefined"
	case obfsTransport:
		return "obfs"
	case proteusTransport:
		return "proteus"
	default:
		return "unknown"
	}
}

// starts the bridge
func startBridge(ctxBridge context.Context, transportType TransportType, configNum int, expName, tgenPath, ptAdapterPath string) {

	// send the server.tgen.graphml file to the bridge
	graphMLBytes := getServerTgen()
	if res := sendFile(ctxBridge, "server.tgen.graphml", graphMLBytes); res != http.StatusOK {
		log.Fatal("could not send server.tgen.graphml to bridge")
	}
	// extract certificate files and send them to bridge
	m := getObsCertificates(configNum)
	for k, v := range m {
		if res := sendFile(ctxBridge, k, v); res != http.StatusOK {
			log.Fatalf("could not send %s to bridge", k)
		}
	}

	var ptAdapterConfigBytes []byte

	switch transportType {
	case undefinedTransport:
		log.Fatal("transport type not defined")
	case obfsTransport:
		ptAdapterConfigBytes = getObfsPTAdapterServerTemplate()
	case proteusTransport:
		ptAdapterConfigBytes = getProteusPTAdapterServerTemplate("FIXME")
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

// starts the censored client
func startClient(ctxCensoredVM context.Context, transportType TransportType, configNum int, expName, tgenPath, ptAdapterPath, bridgeHostname string) {

	// make sure that all tgen processes are really dead
	killTGen := datamodel.JsonCommandStruct{
		Cmd:  "/usr/bin/killall",
		Args: []string{"tgen"},
	}
	makeRequest(ctxCensoredVM, "/runToCompletion", killTGen)

	// send the client.tgen.graphml file to the bridge
	graphMLBytes := getClientTgen()
	if res := sendFile(ctxCensoredVM, "client.tgen.graphml", graphMLBytes); res != http.StatusOK {
		log.Fatal("could not send client.tgen.graphml to bridge")
	}

	var ptAdapterConfigBytes []byte

	switch transportType {
	case undefinedTransport:
		log.Fatal("transport type not defined")
	case obfsTransport:
		// extract certificate
		m := getObsCertificates(configNum)
		certString := getObsCertificatePart(m["obfs4_bridgeline.txt"])
		ptAdapterConfigBytes = getObfsPTAdapterClientTemplate(certString, bridgeHostname)
	case proteusTransport:
		ptAdapterConfigBytes = getProteusPTAdapterClientTemplate("FIXME", bridgeHostname)
	}

	if res := sendFile(ctxCensoredVM, "ptadapter.client.conf", ptAdapterConfigBytes); res != http.StatusOK {
		log.Fatal("could not send ptadapter.client.conf to client")
	}

	// next, cause the censoredVM to call ptadapter
	ptAdapterCommand := datamodel.JsonCommandStruct{
		TimeoutInSecs: 0,
		Cmd:           ptAdapterPath,
		Args:          []string{"-C", "ptadapter.client.conf"},
		StdoutFile:    fmt.Sprintf("ptadapter.client.%s.%d.log", expName, configNum),
		StderrFile:    fmt.Sprintf("ptadapter.client.%s.%d.err", expName, configNum),
	}
	if res := makeRequest(ctxCensoredVM, "/runInBackground", ptAdapterCommand); res != http.StatusOK {
		log.Fatal("could not start ptadapter on client")
	}

	// next, cause the bridge to call tgen on that file
	tgenCmd := datamodel.JsonCommandStruct{
		TimeoutInSecs: 0,
		Cmd:           tgenPath,
		Args:          []string{"client.tgen.graphml"},
		StdoutFile:    fmt.Sprintf("tgen.client.%s.%d.log", expName, configNum),
		StderrFile:    fmt.Sprintf("tgen.client.%s.%d.err", expName, configNum),
	}
	log.Println("running tgen on censored VM...")
	if res := makeRequest(ctxCensoredVM, "/runInBackground", tgenCmd); res != http.StatusOK {
		log.Fatal("could not start tgen on client")
	}
	time.Sleep(3500 * time.Millisecond)
}

func startOpenGFW(ctxGFW context.Context, expName, gfwExecPath string) {

	startOpenGFWCommand := datamodel.JsonCommandStruct{
		TimeoutInSecs: 0,
		Cmd:           gfwExecPath + "/OpenGFW",
		Args: []string{
			"-c",
			gfwExecPath + "/configs/config.yaml",
			gfwExecPath + "/rules/ruleset.yaml",
		},
		StdoutFile: "OpenGFW." + expName + ".log",
		StderrFile: "OpenGFW." + expName + ".err",
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
		gfwExecPath         string
		bridgeUrlEndpoint   string
		ptAdapterPath       string
		tgenPath            string
		bridgeByIP          string
		iterations          int
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
	flag.StringVar(&gfwExecPath, "gfw_exec", "../../OpenGFW/", "Specify the path to OpenGFW")
	flag.StringVar(&censoredUrlEndpoint, "censoredvm_url", "", "Specify the URL endpoint for censored VM")
	flag.StringVar(&bridgeUrlEndpoint, "bridge_url", "", "Specify the URL endpoint for the bridge")
	flag.StringVar(&bridgeByIP, "bridge_ip", "", "Bridge's IP address")
	flag.StringVar(&ptAdapterPath, "ptadapter", "/usr/local/bin/ptadapter", "path to ptadapter on both bridge and censored VM")
	flag.StringVar(&tgenPath, "tgen", "/usr/local/bin/tgen", "path to tgen on both bridge and censored VM")
	flag.IntVar(&iterations, "iterations", 1000, "Number of iterations to run")
	flag.Parse()

	if expName == "" || gfwUrlEndpoint == "" || bridgeByIP == "" ||
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
	startOpenGFW(ctxGFW, expName, gfwExecPath)
	time.Sleep(time.Second)

	for configNum := 1; configNum <= iterations; configNum++ {
		for _, ttype := range []TransportType{obfsTransport, proteusTransport} {

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
					"@retry=0",
					"@" + bridgeByIP,
				},
			}
			makeRequest(ctxCensoredVM, "/runToCompletion", digCmd)

			// start ptadapter and tgen on the bridge
			startBridge(ctxBridge, ttype, configNum, expName, tgenPath, ptAdapterPath)

			// start tgen and ptadapter on the censored VM
			startClient(ctxCensoredVM, ttype, configNum, expName, tgenPath, ptAdapterPath, bridgeByIP)

		}
	}

	time.Sleep(60 * time.Second)
	log.Println("Killing all jobs")
	makeRequest(ctxCensoredVM, "/kill", allJobs)
	makeRequest(ctxGFW, "/kill", allJobs)
	makeRequest(ctxBridge, "/kill", allJobs)
}
