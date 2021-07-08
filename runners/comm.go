package runners

import (
	"github.com/gokins-main/core/runtime"
	"github.com/gokins-main/core/utils"
	"io"
)

type UpdateJobInfo struct {
	Id       string `json:"id"`
	Status   string `json:"status"`
	Error    string `json:"error"`
	ExitCode int    `json:"exit_code"`
}
type RunJob struct {
	Id           string                 `json:"id"`
	StageId      string                 `json:"stageId"`
	BuildId      string                 `json:"buildId"`
	StageName    string                 `json:"StageName"`
	Step         string                 `json:"step"`
	Name         string                 `json:"name"`
	Env          map[string]string      `json:"env"`
	Commands     []*CmdContent          `json:"commands"`
	Artifacts    []*runtime.Artifact    `json:"artifacts"`
	UseArtifacts []*runtime.UseArtifact `json:"useArtifacts"`
}
type CmdContent struct {
	Id string `json:"id"`
	//Gid string `json:"gid"`
	//Pid   string    `json:"pid"`
	Conts string `json:"conts"`
	//Times time.Time `json:"times"`
}

type DirEntry struct {
	Name  string
	IsDir bool
	Size  int64
}
type IExecute interface {
	PullJob(plugs []string) (*RunJob, error)
	Update(m *UpdateJobInfo) error
	CheckCancel(buildId string) bool
	UpdateCmd(jobid, cmdid string, fs, code int) error // fs:1:run,2:end
	PushOutLine(jobid, cmdid, bs string, iserr bool) error
	FindJobId(buildId, stgNm, stpNm string) (string, bool)

	ReadDir(fs int, buildId string, pth string) ([]*DirEntry, error)
	ReadFile(fs int, buildId string, pth string) (int64, io.ReadCloser, error)
	GetEnv(jobid, key string) (string, bool)

	FindArtPackId(jobid, idnt string, name string) (string, error)
	UploadFile(jobid string, name, pth string) (io.WriteCloser, error)
	GenEnv(jobid string, env utils.EnvVal) error
}
