package runners

import (
	"github.com/gokins-main/core/runtime"
	"github.com/gokins-main/core/utils"
	"io"
)

type UpdateJobInfo struct {
	BuildId  string `json:"buildId"`
	JobId    string `json:"jobId"`
	Status   string `json:"status"`
	Error    string `json:"error"`
	ExitCode int    `json:"exitCode"`
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
	UpdateCmd(buildId, jobId, cmdid string, fs, code int) error // fs:1:run,2:end
	PushOutLine(buildId, jobId, cmdid, bs string, iserr bool) error
	FindJobId(buildId, stgNm, stpNm string) (string, bool)

	ReadDir(fs int, buildId string, pth string) ([]*DirEntry, error)
	ReadFile(fs int, buildId string, pth string) (int64, io.ReadCloser, error)
	GetEnv(buildId, jobId, key string) (string, bool)
	FindArtVersionId(buildId, idnt string, name string) (string, error)

	NewArtVersionId(buildId, idnt string, name string) (string, error)
	UploadFile(fs int, buildId, jobId string, dir, pth string) (io.WriteCloser, error)
	GenEnv(buildId, jobId string, env utils.EnvVal) error
}
