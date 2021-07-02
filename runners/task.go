package runners

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"time"

	"github.com/gokins-main/core/common"
	hbtp "github.com/mgr9525/HyperByte-Transfer-Protocol"
	"github.com/sirupsen/logrus"
)

type taskExec struct {
	sync.RWMutex
	egn      *Engine
	job      *RunJob
	Status   string
	Event    string
	Error    string
	ExitCode int

	bngtm time.Time
	endtm time.Time

	wrkpth  string //工作地址
	repopth string //仓库地址

	cmdctx   context.Context
	cmdcncl  context.CancelFunc
	cmdend   bool
	cmdenv   []string
	cmdenvlk sync.RWMutex
}

func (c *taskExec) status(stat, errs string, event ...string) {
	c.Lock()
	defer c.Unlock()
	c.Status = stat
	c.Error = errs
	if len(event) > 0 {
		c.Event = event[0]
	}
}
func (c *taskExec) run() {
	defer func() {
		if err := recover(); err != nil {
			logrus.Warnf("taskExec run recover:%v", err)
			logrus.Warnf("Engine stack:%s", string(debug.Stack()))
		}
	}()
	logrus.Debugf("taskExec run job:%s", c.job.Name)
	c.wrkpth = filepath.Join(c.egn.cfg.Workspace, c.job.BuildId)
	c.repopth = filepath.Join(c.wrkpth, common.PathRepo)

	c.cmdend = false
	c.bngtm = time.Now()
	defer func() {
		c.endtm = time.Now()
	}()
	err := c.check()
	if err != nil {
		c.status(common.BuildStatusError, fmt.Sprintf("check err:%v", err))
		goto ends
	}
	if c.checkStop() {
		c.status(common.BuildStatusCancel, "manual stop!!")
		goto ends
	}
	// c.egn.sysEnv.SetOs() //重设环境变量
	// c.cmdenv = utils.EnvVal{}
	c.cmdctx, c.cmdcncl = context.WithCancel(c.egn.ctx)
	c.status(common.BuildStatusRunning, "")
	c.update()
	go c.runJob()
	for !hbtp.EndContext(c.egn.ctx) && !c.cmdend {
		time.Sleep(time.Millisecond * 100)
		if c.checkStop() {
			c.stop()
		}
	}
ends:
	c.update()
}
func (c *taskExec) stop() {
	if c.cmdcncl != nil {
		c.cmdcncl()
		c.cmdcncl = nil
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
	if !c.job.IsClone {
		stat, err := os.Stat(c.job.RepoPath)
		if err == nil && stat.IsDir() {
			c.repopth = c.job.RepoPath
		}
	}
	stat, err := os.Stat(c.repopth)
	if err == nil {
		if stat.IsDir() {
			return nil
		} else {
			return errors.New("path is not dir")
		}
	} /* else {
		//TODO: download

	}*/
	return errors.New("not found err")
}
func (c *taskExec) update() {
	for {
		err := c.updates()
		if err == nil {
			break
		}
		logrus.Errorf("ExecTask update err:%v", err)
		time.Sleep(time.Millisecond * 100)
		if hbtp.EndContext(c.egn.ctx) {
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
	return c.egn.itr.Update(&UpdateJobInfo{
		Id:       c.job.Id,
		Status:   c.Status,
		Error:    c.Error,
		ExitCode: c.ExitCode,
	})
}
func (c *taskExec) checkStop() bool {
	return c.egn.itr.CheckCancel(c.job.BuildId)
}
func (c *taskExec) runJob() {
	defer func() {
		c.cmdend = true
		if err := recover(); err != nil {
			logrus.Warnf("taskExec runJob recover:%v", err)
			logrus.Warnf("Engine stack:%s", string(debug.Stack()))
		}
	}()

	if len(c.job.Commands) <= 0 {
		c.status(common.BuildStatusError, "not found any commands")
		return
	}

	err := c.checkRepo()
	if err != nil {
		c.status(common.BuildStatusError, fmt.Sprintf("check repo:%v", err))
		return
	}

	for _, v := range c.job.Commands {
		proc := &procExec{
			prt:  c,
			cmd:  v,
			envs: c.job.Env,
		}
		err = c.egn.itr.UpdateCmd(c.job.Id, v.Id, 1, 0)
		if err != nil {
			logrus.Errorf("cmdExec runCmdNext UpdateCmd err:%v", err)
		}
		err = proc.start()
		if err != nil {
			code := -1
			if hbtp.EndContext(c.cmdctx) {
				c.status(common.BuildStatusCancel, err.Error())
				code = 3
			} else {
				c.status(common.BuildStatusError, err.Error())
				code = -1
			}
			err = c.egn.itr.UpdateCmd(c.job.Id, v.Id, code, c.ExitCode)
			if err != nil {
				logrus.Errorf("cmdExec runCmdNext UpdateCmd err:%v", err)
			}
			return
		}
		err = c.egn.itr.UpdateCmd(c.job.Id, v.Id, 2, c.ExitCode)
		if err != nil {
			logrus.Errorf("cmdExec runCmdNext UpdateCmd err:%v", err)
		}
	}

	/*if c.Status != common.BuildStatusOk {
		logrus.Debugf("cmdExec start err(%d):%s", c.ExitCode, c.Error)
		return
	}*/
	c.status(common.BuildStatusOk, "")
}
