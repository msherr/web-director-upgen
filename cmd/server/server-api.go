package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Structs

// jsonCommandStruct represents a command to be executed.
type jsonCommandStruct struct {
	TimeoutInSecs float32  `json:"timeout"` // anything positive will be considered a timeout
	Cmd           string   `json:"cmd"`
	Args          []string `json:"args"`
}

// jsonKillStruct represents a job to be killed.
// note that  if job == -1, then all jobs will be killed
type jsonKillStruct struct {
	JobNo JobNoType `json:"job"`
}

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
	var cmdFromForm jsonCommandStruct
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

// handleRunInBackground handles the "/runInBackground" endpoint and executes a command asynchronously.
func handleRunInBackground(w http.ResponseWriter, r *http.Request) {
	var cmdFromForm jsonCommandStruct
	if err := json.NewDecoder(r.Body).Decode(&cmdFromForm); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var cmd *exec.Cmd
	var err error

	if cmdFromForm.TimeoutInSecs > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cmdFromForm.TimeoutInSecs)*time.Second)
		defer cancel()

		cmd = exec.CommandContext(ctx, cmdFromForm.Cmd, cmdFromForm.Args...)
	} else {
		cmd = exec.Command(cmdFromForm.Cmd, cmdFromForm.Args...)
	}

	if err = cmd.Start(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// call wait on the thing we just started
	go func() {
		cmd.Wait()
	}()
	processChannel <- &processJobStruct{
		cmd:     cmd,
		jobNo:   <-jobChannel,
		cmdLine: cmdFromForm.Cmd + " " + strings.Join(cmdFromForm.Args, " "),
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
	var jsonKill jsonKillStruct
	if err := json.NewDecoder(r.Body).Decode(&jsonKill); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	jobKillChannel <- jsonKill.JobNo
	writeJson(true, w)
}
