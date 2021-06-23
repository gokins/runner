package runners

import "github.com/gokins-main/core/runtime"

type IExecute interface {
	PullJob(plugs []string) (*runtime.Step, error)
	Update(job *runtime.Step) error
}
