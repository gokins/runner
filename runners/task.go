package runners

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gokins/core/utils"

	"github.com/gokins/core/common"
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
	repocpd bool   //是否不需要copy

	cmdctx   context.Context
	cmdcncl  context.CancelFunc
	cmdend   bool
	cmdenv   utils.EnvVal
	cmdenvs  utils.EnvVal
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
	c.wrkpth = filepath.Join(c.egn.cfg.Workspace, common.PathJobs, c.job.Id)
	c.repopth = filepath.Join(c.wrkpth, common.PathRepo)
	if c.job.OriginRepo != "" {
		_, err := os.Stat(c.job.OriginRepo)
		logrus.Debugf("task(%s) in OriginRepo err:%s,err=%v", c.job.Id, c.job.OriginRepo, err)
		if err == nil {
			c.repopth = c.job.OriginRepo
			c.repocpd = true
		}
	}
	if c.job.UsersRepo != "" {
		_, err := os.Stat(c.job.UsersRepo)
		logrus.Debugf("task(%s) in UsersRepo err:%s,err=%v", c.job.Id, c.job.UsersRepo, err)
		if err == nil {
			c.repopth = c.job.UsersRepo
			c.repocpd = true
		}
	}
	logrus.Debugf("taskExec run job:%s(OriginRepo:%s)", c.job.Name, c.job.OriginRepo)
	defer os.RemoveAll(c.wrkpth)

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
		BuildId:  c.job.BuildId,
		JobId:    c.job.Id,
		Status:   c.Status,
		Error:    c.Error,
		ExitCode: c.ExitCode,
	})
}
func (c *taskExec) checkStop() bool {
	return c.egn.itr.CheckCancel(c.job.BuildId)
}
func (c *taskExec) initCmdEnv() {
	c.cmdenvlk.Lock()
	defer c.cmdenvlk.Unlock()
	c.cmdenv = utils.EnvVal{}
	c.cmdenvs = utils.EnvVal{}
	for k, v := range c.egn.sysEnv {
		c.cmdenv[k] = v
	}
	for _, v := range c.egn.cfg.Env {
		i := strings.Index(v, "=")
		if i > 0 {
			k := v[:i]
			val := v[i+1:]
			if val != "" {
				c.cmdenv[k] = val
				logrus.Debug("cmd env:", k, "=", val)
			}
		}
	}
	c.cmdenv["WORKPATH"] = c.wrkpth
	c.cmdenv["REPOPATH"] = c.repopth
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
	c.initCmdEnv()
	err := c.checkRepo()
	if err != nil {
		c.status(common.BuildStatusError, fmt.Sprintf("check repo:%v", err))
		return
	}

	err = c.getArts()
	if err != nil {
		c.status(common.BuildStatusError, fmt.Sprintf("use artifacts:%v", err))
		return
	}

	var scli *ssh.Client
	if c.job.Step == "shell@ssh" {
		scli, err = c.connSSH()
		if err != nil {
			c.status(common.BuildStatusError, "SSH connect err:"+err.Error())
			return
		}
		defer scli.Close()
	}
	err = c.runProcs(scli)
	if err != nil {
		if hbtp.EndContext(c.cmdctx) {
			c.status(common.BuildStatusCancel, err.Error())
		} else {
			c.status(common.BuildStatusError, err.Error())
		}
		return
	}

	err = c.genArts()
	if err != nil {
		c.status(common.BuildStatusError, fmt.Sprintf("put artifacts:%v", err))
		return
	}

	c.status(common.BuildStatusOk, "")
}
