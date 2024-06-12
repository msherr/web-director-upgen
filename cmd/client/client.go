package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
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

// jsonCommandStruct represents a command to be executed.
type jsonCommandStruct struct {
	TimeoutInSecs float32  `json:"timeout"` // anything positive will be considered a timeout
	Cmd           string   `json:"cmd"`
	Args          []string `json:"args"`
}

func makeRequest(ctx context.Context, f string, data any) {
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
	res, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	if res.StatusCode == http.StatusOK {
		b, err := io.ReadAll(res.Body)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(string(b))
	}
}

func main() {
	authToken := os.Getenv("SERVER_AUTH_TOKEN")
	if authToken == "" {
		log.Fatal("SERVER_AUTH_TOKEN not set")
	}

	var insecure bool
	var urlEndpoint string

	flag.BoolVar(&insecure, "insecure", false, "Set to true to disable TLS verification")
	flag.StringVar(&urlEndpoint, "url", "", "Specify the URL endpoint")
	flag.Parse()

	if urlEndpoint == "" {
		flag.Usage()
		os.Exit(1)
	}

	if insecure {
		log.Println("Warning: Skipping TLS verification")
	}

	ctx := context.WithValue(context.Background(), AuthTokenKey, authToken)
	ctx = context.WithValue(ctx, URLEndpointKey, urlEndpoint)

	tr := &http.Transport{
		MaxIdleConns:       10,
		IdleConnTimeout:    30 * time.Second,
		DisableCompression: true,
		TLSClientConfig:    &tls.Config{InsecureSkipVerify: insecure}, // DANGER
	}
	client := &http.Client{Transport: tr}
	ctx = context.WithValue(ctx, ClientKey, client)
	defer client.CloseIdleConnections()

	sleepCmd := jsonCommandStruct{
		TimeoutInSecs: 0,
		Cmd:           "sleep",
		Args:          []string{"2"},
	}
	for i := 0; i < 10; i++ {
		fmt.Println("Requesting sleep ")
		makeRequest(ctx, "/runInBackground", sleepCmd)
	}

	makeRequest(ctx, "/jobs", nil)

	makeRequest(ctx, "/kill",
		struct {
			JobID int `json:"job"`
		}{
			JobID: -1,
		})

	makeRequest(ctx, "/jobs", nil)

}
