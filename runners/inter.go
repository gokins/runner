package runners

import "github.com/gokins-main/core/runtime"

type UpdateJobInfo struct {
	Id       string `json:"id"`
	Status   string `json:"status"`
	Error    string `json:"error"`
	ExitCode int    `json:"exit_code"`
}
type IExecute interface {
	PullJob(plugs []string) (*runtime.Step, error)
	Update(m *UpdateJobInfo) error
	CheckCancel(buildId string) bool
}
