package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
)

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
	log.Printf("Making request to %v: %v\n", url+f, data)
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
		log.Printf("Unexpected status code from %s%s: %d", url, f, res.StatusCode)
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
	if resp.StatusCode == http.StatusOK {
		log.Printf("sent file %s to %s\n", fileName, url)
	} else {
		log.Printf("could not send file %s to %s: status code %v", fileName, url, resp.Status)
	}
	return resp.StatusCode
}
