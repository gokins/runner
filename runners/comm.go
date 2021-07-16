package runners

import (
	"github.com/gokins-main/core/utils"
	"io"
)

type IExecute interface {
	ServerInfo() (*ServerInfo, error)
	PullJob(name string, plugs []string) (*RunJob, error)
	CheckCancel(buildId string) bool
	Update(m *UpdateJobInfo) error
	UpdateCmd(buildId, jobId, cmdId string, fs, code int) error // fs:1:run,2:end
	PushOutLine(buildId, jobId, cmdId, bs string, iserr bool) error
	FindJobId(buildId, stgNm, stpNm string) (string, bool)

	ReadDir(fs int, buildId string, pth string) ([]*DirEntry, error)
	ReadFile(fs int, buildId string, pth string) (int64, io.ReadCloser, error)
	GetEnv(buildId, jobId, key string) (string, bool)
	FindArtVersionId(buildId, idnt string, name string) (string, error)

	NewArtVersionId(buildId, idnt string, name string) (string, error)
	UploadFile(fs int, buildId, jobId string, dir, pth string) (io.WriteCloser, error)
	GenEnv(buildId, jobId string, env utils.EnvVal) error
}
