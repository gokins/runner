package runners

import (
	"github.com/gokins-main/core/runtime"
)

type UpdateJobInfo struct {
	Id       string `json:"id"`
	Status   string `json:"status"`
	Error    string `json:"error"`
	ExitCode int    `json:"exit_code"`
}
type RunJob struct {
	Id              string                    `json:"id"`
	StageId         string                    `json:"stageId"`
	BuildId         string                    `json:"buildId"`
	Step            string                    `json:"step"`
	Name            string                    `json:"name"`
	Env             map[string]string         `json:"env"`
	Commands        []*CmdContent             `json:"commands"`
	Artifacts       []*runtime.Artifact       `json:"artifacts"`
	DependArtifacts []*runtime.DependArtifact `json:"dependArtifacts"`
	IsClone         bool                      `json:"isClone"`
	RepoPath        string                    `json:"repoPath"`
}
type CmdContent struct {
	Id string `json:"id"`
	//Gid string `json:"gid"`
	//Pid   string    `json:"pid"`
	Conts string `json:"conts"`
	//Times time.Time `json:"times"`
}
type IExecute interface {
	PullJob(plugs []string) (*RunJob, error)
	Update(m *UpdateJobInfo) error
	CheckCancel(buildId string) bool
	UpdateCmd(jobid, cmdid string, fs, code int) error // fs:1:run,2:end
	PushOutLine(jobid, cmdid, bs string, iserr bool) error
}
