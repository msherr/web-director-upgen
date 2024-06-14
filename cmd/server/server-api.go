package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"datamodel"
)

// HTTP Handlers

// handleVersion handles the "/version" endpoint and returns the server version.
func handleVersion(w http.ResponseWriter, r *http.Request) {
	res := struct {
		Version string `json:"version"`
	}{
		Version: version,
	}
	writeJson(res, w)
}

// handleExit handles the "/exit" endpoint and exits the server after a delay.
func handleExit(w http.ResponseWriter, r *http.Request) {
	writeJson(true, w)
	go func() {
		time.Sleep(1 * time.Second)
		os.Exit(0)
	}()
}

// handleRunToCompletion handles the "/runToCompletion" endpoint and executes a command synchronously.
func handleRunToCompletion(w http.ResponseWriter, r *http.Request) {
	var cmdFromForm datamodel.JsonCommandStruct
	if err := json.NewDecoder(r.Body).Decode(&cmdFromForm); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var cmd *exec.Cmd
	var output []byte
	var err error

	if cmdFromForm.TimeoutInSecs > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cmdFromForm.TimeoutInSecs)*time.Second)
		defer cancel()

		cmd = exec.CommandContext(ctx, cmdFromForm.Cmd, cmdFromForm.Args...)
	} else {
		cmd = exec.Command(cmdFromForm.Cmd, cmdFromForm.Args...)
	}
	if output, err = cmd.Output(); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			http.Error(w, "timeout", http.StatusRequestTimeout)
			return
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	writeJson(struct {
		Success  bool   `json:"success"`
		Output   string `json:"output"`
		ExitCode int    `json:"exitCode"`
	}{
		Success:  true,
		Output:   string(output),
		ExitCode: cmd.ProcessState.ExitCode(),
	}, w)
}

func writeToFile(wg *sync.WaitGroup, fileName string, s *io.ReadCloser) {
	defer wg.Done()
	wg.Add(1)
	file, err := os.Create(fileName)
	if err != nil {
		log.Printf("warning: cannot create file (%v): %v\n", fileName, err)
		return
	}
	defer file.Close()
	_, err = io.Copy(file, *s)
	if err != nil {
		log.Printf("warning: cannot write to file (%v): %v\n", fileName, err)
	}
}

// handleRunInBackground handles the "/runInBackground" endpoint and executes a command asynchronously.
func handleRunInBackground(w http.ResponseWriter, r *http.Request) {
	var cmdFromForm datamodel.JsonCommandStruct
	if err := json.NewDecoder(r.Body).Decode(&cmdFromForm); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var cmd *exec.Cmd
	var err error
	var stdout, stderr io.ReadCloser

	if cmdFromForm.TimeoutInSecs > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cmdFromForm.TimeoutInSecs)*time.Second)
		defer cancel()

		cmd = exec.CommandContext(ctx, cmdFromForm.Cmd, cmdFromForm.Args...)
	} else {
		cmd = exec.Command(cmdFromForm.Cmd, cmdFromForm.Args...)
	}

	// create pipes for stderr and stdout
	if cmdFromForm.StdoutFile != "" {
		stdout, err = cmd.StdoutPipe()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	if cmdFromForm.StderrFile != "" {
		stderr, err = cmd.StderrPipe()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// run the thing, in the background
	if err = cmd.Start(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// call wait on the thing we just started
	go func() {

		// first, save stdout and stderr to files, if necessary
		pipePointers := []struct {
			fileName string
			src      *io.ReadCloser
		}{
			{cmdFromForm.StdoutFile, &stdout},
			{cmdFromForm.StderrFile, &stderr},
		}
		var wg sync.WaitGroup
		for _, pipePointer := range pipePointers {
			if pipePointer.fileName != "" {
				// writeToFile adds 1 to wg
				go writeToFile(&wg, pipePointer.fileName, pipePointer.src)
			}
		}
		// wait for stderr and stdout to finish writing
		wg.Wait()

		// finally, wait for the process to finish (which is should have already)
		cmd.Wait()
	}()

	processChannel <- &datamodel.ProcessJobStruct{
		Cmd:        cmd,
		JobNo:      <-jobChannel,
		CmdLine:    cmdFromForm.Cmd + " " + strings.Join(cmdFromForm.Args, " "),
		StdoutFile: cmdFromForm.StdoutFile,
		StderrFile: cmdFromForm.StderrFile,
	}
	writeJson(true, w)
}

// handleJobList handles the "/jobs" endpoint and returns the list of running jobs.
func handleJobList(w http.ResponseWriter, r *http.Request) {
	jobListRequestChannel <- nil
	jobList := <-jobListResponseChannel

	writeJson(jobList, w)
}

// handleJobList handles the "/jobs" endpoint and returns the list of running jobs.
func handleKillJob(w http.ResponseWriter, r *http.Request) {
	var jsonKill datamodel.JsonKillStruct
	if err := json.NewDecoder(r.Body).Decode(&jsonKill); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	jobKillChannel <- jsonKill.JobNo
	writeJson(true, w)
}

func handleUploadFile(w http.ResponseWriter, r *http.Request) {
	// Check if the request method is POST
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the multipart form data
	err := r.ParseMultipartForm(32 << 20) // 32MB
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get the file from the form data
	file, handler, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Failed to get file from form data", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Create a new file on the server to store the uploaded file
	dst, err := os.Create(handler.Filename)
	if err != nil {
		http.Error(w, "Failed to create file on server", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	// Copy the contents of the uploaded file to the destination file
	_, err = io.Copy(dst, file)
	if err != nil {
		http.Error(w, "Failed to copy file contents", http.StatusInternalServerError)
		return
	}

	writeJson(true, w)
}
