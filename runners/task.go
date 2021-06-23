package runners

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"time"

	"github.com/gokins-main/core/common"
	"github.com/gokins-main/core/runtime"
	hbtp "github.com/mgr9525/HyperByte-Transfer-Protocol"
	"github.com/sirupsen/logrus"
)

type taskExec struct {
	sync.RWMutex
	prt *Engine
	job *runtime.Step

	bngtm time.Time
	endtm time.Time

	wrkpth  string //工作地址
	repopth string //仓库地址
	jobpth  string //仓库地址

	cmd    *cmdExec
	cmdend bool
}

func (c *taskExec) status(stat, errs string, event ...string) {
	c.Lock()
	defer c.Unlock()
	c.job.Status = stat
	c.job.Error = errs
	if len(event) > 0 {
		c.job.Event = event[0]
	}
}
func (c *taskExec) run() {
	defer func() {
		if err := recover(); err != nil {
			logrus.Warnf("taskExec run recover:%v", err)
			logrus.Warnf("Engine stack:%s", string(debug.Stack()))
		}
	}()
	c.wrkpth = filepath.Join(c.prt.cfg.Workspace, common.PathBuild, c.job.BuildId)
	c.repopth = filepath.Join(c.wrkpth, common.PathRepo)
	c.jobpth = filepath.Join(c.wrkpth, common.PathJobs, c.job.Id)

	c.cmdend = false
	c.bngtm = time.Now()
	defer func() {
		c.endtm = time.Now()
		os.RemoveAll(c.jobpth)
	}()
	os.RemoveAll(c.jobpth)
	os.MkdirAll(c.jobpth, 0750)
	err := c.check()
	if err != nil {
		c.status(common.BuildStatusError, fmt.Sprintf("check err:%v", err))
		goto ends
	}
	if c.checkStop() {
		c.status(common.BuildStatusError, "manual stop!!")
		goto ends
	}
	err = c.checkRepo()
	if err != nil {
		c.status(common.BuildStatusError, fmt.Sprintf("check repo err:%v", err))
		goto ends
	}
	c.status(common.BuildStatusRunning, "")
	c.update()
	go c.runJob()
	for !hbtp.EndContext(c.prt.ctx) && !c.cmdend {
		time.Sleep(time.Millisecond * 100)
		if c.checkStop() {
			c.stop()
		}
	}
ends:
	c.update()
}
func (c *taskExec) stop() {
	if c.cmd != nil {
		c.cmd.stop()
	}
}
func (c *taskExec) check() error {
	if c.job.Name == "" {
		//c.update(common.BUILD_STATUS_ERROR,"build Job name is empty")
		return errors.New("build Job name is empty")
	}
	return nil
}
func (c *taskExec) checkRepo() error {
	stat, err := os.Stat(c.repopth)
	if err == nil {
		if stat.IsDir() {
			return nil
		} else {
			return errors.New("repo path is not dir")
		}
	} /* else {
		//download
	} */
	return errors.New("mores")
}
func (c *taskExec) update() {
	for {
		err := c.updates()
		if err == nil {
			break
		}
		logrus.Errorf("ExecTask update err:%v", err)
		time.Sleep(time.Millisecond * 100)
		if hbtp.EndContext(c.prt.ctx) {
			break
		}
	}
}
func (c *taskExec) updates() error {
	c.RLock()
	defer func() {
		c.RUnlock()
		if err := recover(); err != nil {
			logrus.Warnf("taskExec update recover:%v", err)
			logrus.Warnf("Engine stack:%s", string(debug.Stack()))
		}
	}()
	return c.prt.itr.Update(&UpdateJobInfo{
		Id:       c.job.Id,
		Status:   c.job.Status,
		Error:    c.job.Error,
		ExitCode: c.job.ExitCode,
	})
}
func (c *taskExec) checkStop() bool {
	return c.prt.itr.CheckCancel(c.job.BuildId)
}
func (c *taskExec) runJob() {
	defer func() {
		c.cmdend = true
		if err := recover(); err != nil {
			logrus.Warnf("taskExec runJob recover:%v", err)
			logrus.Warnf("Engine stack:%s", string(debug.Stack()))
		}
	}()

	var envs map[string]string
	c.cmd = &cmdExec{prt: c, envs: envs}
	err := c.cmd.start()
	if err != nil {
		c.status(common.BuildStatusError, err.Error())
		return
	}
	if c.job.Status != common.BuildStatusOk {
		logrus.Errorf("cmdExec start err(%d):%s", c.job.ExitCode, c.job.Error)
		return
	}

	c.status(common.BuildStatusOk, "")
}
