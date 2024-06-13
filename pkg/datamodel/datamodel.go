package datamodel

import "os/exec"

type JobNoType int

// ProcessJobStruct represents a background process job.
type ProcessJobStruct struct {
	Cmd     *exec.Cmd
	JobNo   JobNoType
	CmdLine string
}
