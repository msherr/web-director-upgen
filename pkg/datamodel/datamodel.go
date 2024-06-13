package datamodel

import "os/exec"

type JobNoType int

// ProcessJobStruct represents a background process job.
type ProcessJobStruct struct {
	Cmd     *exec.Cmd
	JobNo   JobNoType
	CmdLine string
}

// JsonCommandStruct represents a command to be executed.
type JsonCommandStruct struct {
	TimeoutInSecs float32  `json:"timeout"` // anything positive will be considered a timeout
	Cmd           string   `json:"cmd"`
	Args          []string `json:"args"`
	StdoutFile    string   `json:"stdout"`
	StderrFile    string   `json:"stderr"`
}

// JsonKillStruct represents a job to be killed.
// note that  if job == -1, then all jobs will be killed
type JsonKillStruct struct {
	JobNo JobNoType `json:"job"`
}
