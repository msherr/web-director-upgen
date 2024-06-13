/*
*
A web service for running and managing background processes

Micah Sherr <msherr@cs.georgetown.edu>

Provides a server implementation for running and managing background processes.
The server exposes several HTTP endpoints for executing commands, managing jobs,
and retrieving job information. It also includes an authentication middleware
for validating requests using a session token.

WARNING: This program is extremely dangerous and you probably don't want to run
it on any machine you care about.  It allows arbitrary command execution, and is
intended for managing experiments.
*/
package main

import (
	"datamodel"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
)

// Constants
const version = "0.0.1"

// jobList represents a list of jobs.
type jobList []struct {
	JobNo      datamodel.JobNoType `json:"jobNo"`
	CmdLine    string              `json:"cmdLine"`
	StdoutFile string              `json:"stdoutFile"`
	StderrFile string              `json:"stderrFile"`
}

// Variables

var processChannel = make(chan *datamodel.ProcessJobStruct)
var jobChannel = make(chan datamodel.JobNoType)

// Channels for job list management
var jobListRequestChannel = make(chan any)
var jobListResponseChannel = make(chan jobList)
var jobKillChannel = make(chan datamodel.JobNoType)

// Helper Functions

// produceNextJobNumber generates the next job number and sends it to the jobChannel.
func produceNextJobNumber() {
	jobNo := datamodel.JobNoType(0)
	for {
		jobChannel <- jobNo
		jobNo++
	}
}

// writeJson writes the given value as JSON to the response writer.
func writeJson(v any, w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}
	w.Write(b)
	w.Write([]byte("\n"))
	return nil
}

// `jobManager` periodically checks the status of running processes and updates
// the job list. It also responds to requests to list the jobs or delete/kill a
// job.
func jobManager() {
	ticker := time.NewTicker(500 * time.Millisecond)
	processJobs := make(map[*datamodel.ProcessJobStruct]any)
	for {
		select {
		case cmd := <-processChannel:
			// new job, add it to our map
			processJobs[cmd] = nil

		case <-ticker.C:
			for p := range processJobs {
				log.Printf("Checking process %v: %v", p.JobNo, p.Cmd)
				if p.Cmd.ProcessState != nil {
					p.Cmd.Wait()
					delete(processJobs, p)
				}
			}

		case <-jobListRequestChannel:
			jobList := make(jobList, 0, len(processJobs))
			for p := range processJobs {
				jobList = append(jobList, struct {
					JobNo      datamodel.JobNoType `json:"jobNo"`
					CmdLine    string              `json:"cmdLine"`
					StdoutFile string              `json:"stdoutFile"`
					StderrFile string              `json:"stderrFile"`
				}{
					JobNo:      p.JobNo,
					CmdLine:    p.CmdLine,
					StdoutFile: p.StdoutFile,
					StderrFile: p.StderrFile,
				})
			}
			jobListResponseChannel <- jobList

		case jobNo := <-jobKillChannel:
			for p := range processJobs {
				if p.JobNo == jobNo || jobNo == -1 {
					// kill the process
					log.Printf("Killing process %v: %v", p.JobNo, p.Cmd)
					p.Cmd.Process.Kill()
					//p.Cmd.Wait()
					delete(processJobs, p)
				}
			}
		}
	}
}

// Middleware

// authenticationMiddleware is a middleware for validating requests using a session token.
type authenticationMiddleware struct {
	authToken string
}

// Init initializes the authentication middleware by grabbing the token from the environment.
func (amw *authenticationMiddleware) Init() {
	amw.authToken = os.Getenv("SERVER_AUTH_TOKEN")
	if amw.authToken == "" {
		log.Fatal("SERVER_AUTH_TOKEN not set")
	}
}

// Middleware is the middleware function that will be called for each request.
func (amw *authenticationMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-Session-Token")

		if token == amw.authToken {
			log.Printf("token valid")
			// Pass down the request to the next middleware (or final handler)
			next.ServeHTTP(w, r)
		} else {
			// Write an error and stop the handler chain
			http.Error(w, "Forbidden", http.StatusForbidden)
		}
	})
}

// Main Function

func main() {
	var (
		certPath string
		keyPath  string
		port     string
	)

	flag.StringVar(&certPath, "certpath", "", "Path to the certificate file")
	flag.StringVar(&keyPath, "keypath", "", "Path to the key file")
	flag.StringVar(&port, "port", "8888", "Port number to listen on")
	flag.Parse()

	if certPath == "" || keyPath == "" {
		flag.Usage()
		os.Exit(1)
	}

	log.Println("Certificate Path:", certPath)
	log.Println("Key Path:", keyPath)

	go jobManager()
	go produceNextJobNumber()

	r := mux.NewRouter()

	r.HandleFunc("/version", handleVersion)
	r.HandleFunc("/exit", handleExit)
	r.HandleFunc("/runToCompletion", handleRunToCompletion)
	r.HandleFunc("/runInBackground", handleRunInBackground)
	r.HandleFunc("/jobs", handleJobList)
	r.HandleFunc("/kill", handleKillJob)

	amw := authenticationMiddleware{}
	amw.Init()
	r.Use(amw.Middleware)

	log.Println("Server starting on port " + port)
	log.Fatal(http.ListenAndServeTLS(":"+port, certPath, keyPath, r))
}
