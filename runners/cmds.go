package runners

import (
	"context"
	"errors"
	"github.com/gokins-main/core/common"
	"github.com/sirupsen/logrus"
	"io"
	"os/exec"
	"runtime/debug"
	"time"
)

type cmdExec struct {
	prt  *taskExec
	envs map[string]string

	ctx    context.Context
	cncl   context.CancelFunc
	cmd    *exec.Cmd
	cmdend bool
	spts   string

	cmdout io.ReadCloser
	cmderr io.ReadCloser
	cmdbuf io.WriteCloser
	linetm time.Time

	cmdind int
	//cmds   []*hbtpBean.CmdContent

}

func (c *cmdExec) stop() {
	defer func() {
		if err := recover(); err != nil {
			logrus.Warnf("cmdExec stop recover:%v", err)
			logrus.Warnf("Engine stack:%s", string(debug.Stack()))
		}
	}()
	if c.cmdbuf != nil {
		c.cmdbuf.Close()
		c.cmdbuf = nil
	}
	if c.cncl != nil {
		c.cncl()
		c.cncl = nil
	}
	/*if c.cmd.ProcessState != nil && !c.cmd.ProcessState.Exited() {
		if c.cmd != nil && c.cmd.Process != nil {
			core.Log.Debugf("cmdExec %s need kill!pid:%d", c.prt.job.Name, c.cmd.Process.Pid)
			// c.cmd.Process.Signal(syscall.SIGINT)
			// time.Sleep(time.Second)
			// c.cmd.Process.Kill()
		}
	}*/
}
func (c *cmdExec) start() error {
	c.cmdind = -1
	c.cmdend = false
	c.linetm = time.Now()
	c.ctx, c.cncl = context.WithCancel(c.prt.prt.ctx)
	if c.prt == nil {
		return errors.New("parent is nil")
	}
	defer func() {
		c.stop()
		logrus.Debugf("cmdExec end code:%d!!", c.prt.job.ExitCode)
		if err := recover(); err != nil {
			logrus.Warnf("cmdExec start recover:%v", err)
			logrus.Warnf("Engine stack:%s", string(debug.Stack()))
		}
	}()

	c.prt.job.Status = common.BuildStatusError
	return nil
}
