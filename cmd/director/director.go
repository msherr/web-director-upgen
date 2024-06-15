package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"datamodel"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"time"
)

type AuthTokenKeyType string
type ClientKeyType string
type URLEndpointType string

const AuthTokenKey = AuthTokenKeyType("authToken")
const ClientKey = ClientKeyType("client")
const URLEndpointKey = URLEndpointType("urlEndpoint")

var allJobs = struct {
	JobID int `json:"job"`
}{
	JobID: -1,
}

func makeRequest(ctx context.Context, f string, data any) int {
	var err error
	var req *http.Request

	url := ctx.Value(URLEndpointKey).(string)

	if data != nil {
		var j []byte
		j, err = json.Marshal(data)
		if err != nil {
			log.Fatal(err)
		}
		req, err = http.NewRequest("GET", url+f, bytes.NewBuffer(j))
	} else {
		req, err = http.NewRequest("GET", url+f, nil)
	}
	if err != nil {
		log.Fatal(err)
	}
	client := ctx.Value(ClientKey).(*http.Client)
	token := ctx.Value(AuthTokenKey).(string)
	req.Header = http.Header{
		"X-Session-Token": {token},
		"Content-Type":    {"application/json"},
	}
	log.Printf("Making request to %v: %v\n", url+f, req)
	res, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	if res.StatusCode == http.StatusOK {
		b, err := io.ReadAll(res.Body)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Response from %s%s:\n\n", url, f)
		fmt.Println(string(b))
	} else {
		log.Fatalf("Unexpected status code from %s%s: %d", url, f, res.StatusCode)
	}
	return res.StatusCode
}

func sendFile(ctx context.Context, fileName string, fileContents []byte) int {
	client := ctx.Value(ClientKey).(*http.Client)
	token := ctx.Value(AuthTokenKey).(string)
	url := ctx.Value(URLEndpointKey).(string)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		log.Fatal(err)
	}
	_, err = io.Copy(part, bytes.NewReader(fileContents))
	if err != nil {
		log.Fatal(err)
	}

	if err = writer.Close(); err != nil {
		log.Fatal(err)
	}

	req, err := http.NewRequest(http.MethodPost, url+"/upload", body)
	if err != nil {
		log.Fatal(err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-Session-Token", token)

	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	log.Printf("sent file %s to %s\n", fileName, url)
	return resp.StatusCode
}

// starts the bridge
func startBridge(ctxBridge context.Context, expName, tgenPath string) {
	// step 0: make sure that nothing else is running
	makeRequest(ctxBridge, "/kill", allJobs)
	time.Sleep(2 * time.Second)

	// next, send the server.tgen.graphml file to the bridge
	graphMLBytes := getServerTgen()
	if res := sendFile(ctxBridge, "server.tgen.graphml", graphMLBytes); res != http.StatusOK {
		log.Fatal("could not send server.tgen.graphml to bridge")
	}

	// next, cause the bridge to call tgen on that file
	tgenCmd := datamodel.JsonCommandStruct{
		TimeoutInSecs: 0,
		Cmd:           tgenPath,
		Args:          []string{"server.tgen.graphml"},
		StdoutFile:    "tgen.bridge." + expName + ".log",
		StderrFile:    "tgen.bridge." + expName + ".err",
	}
	if res := makeRequest(ctxBridge, "/runInBackground", tgenCmd); res != http.StatusOK {
		log.Fatal("could not start tgen on bridge")
	}

	// report out what's running
	makeRequest(ctxBridge, "/jobs", nil)
}

func startOpenGFW(ctxGFW context.Context, expName string) {
	// step 0: make sure that nothing else is running
	makeRequest(ctxGFW, "/kill", allJobs)
	time.Sleep(2 * time.Second)

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

	// --- start the experiment ---
	startBridge(ctxBridge, expName, tgenPath) // start the bridge
	startOpenGFW(ctxGFW, expName)             // start OpenGFW

	obsClientTemplate := getObsClientTemplate()
	sendFile(ctxCensoredVM, "ptadapter-obs-client", obsClientTemplate)

	/*
		for i := 0; i < 150; i++ {
			digCmd := datamodel.JsonCommandStruct{
				TimeoutInSecs: 0,
				Cmd:           "dig",
				Args: []string{
					fmt.Sprintf("thisisatest.%d.log.message", i),
					"@10.128.0.1",
				},
			}
			makeRequest(ctxCensoredVM, "/runToCompletion", digCmd)
		}
	*/

	time.Sleep(60 * time.Second)
	log.Println("Killing all jobs")
	makeRequest(ctxCensoredVM, "/kill", allJobs)
	makeRequest(ctxGFW, "/kill", allJobs)
	makeRequest(ctxBridge, "/kill", allJobs)
}
