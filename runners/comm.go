package runners

import (
	"github.com/gokins-main/core/runtime"
	"github.com/gokins-main/core/utils"
	"io"
)

type ServerInfo struct {
	WebHost   string
	DownToken string
}
type UpdateJobInfo struct {
	BuildId  string
	JobId    string
	Status   string
	Error    string
	ExitCode int
}
type RunJob struct {
	Id           string
	StageId      string
	BuildId      string
	StageName    string
	Step         string
	Name         string
	Input        map[string]string
	Env          map[string]string
	Commands     []*CmdContent
	Artifacts    []*runtime.Artifact
	UseArtifacts []*runtime.UseArtifact
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
	ServerInfo() ServerInfo
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
