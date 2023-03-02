package runners

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime/debug"
	"sync/atomic"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	ghttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/gokins/runner/util"
	hbtp "github.com/mgr9525/HyperByte-Transfer-Protocol"
	"github.com/sirupsen/logrus"
)

type gitExec struct {
	prt  *taskExec
	ctx  context.Context
	cncl context.CancelFunc
	cmd  *CmdContent

	cmdend time.Time
	cmdout *bytes.Buffer
}

func (c *gitExec) stop() {
	if c.cncl != nil {
		c.cncl()
		c.cncl = nil
	}
}
func (c *gitExec) start() (rterr error) {
	defer func() {
		c.stop()
		logrus.Debugf("gitExec end code:%d!!", c.prt.ExitCode)
		if err := recover(); err != nil {
			rterr = fmt.Errorf("recover:%v", err)
			logrus.Warnf("gitExec start recover:%v", err)
			logrus.Warnf("Engine stack:%s", string(debug.Stack()))
		}
	}()

	if c.prt == nil {
		return nil
	}
	c.ctx, c.cncl = context.WithCancel(c.prt.cmdctx)
	c.cmdout = &bytes.Buffer{}

	var cmderr error
	wtn := int32(3)
	go func() {
		cmderr = c.runCmd()
		logrus.Debugf("runCmd end!!!!")
		atomic.AddInt32(&wtn, -1)
		time.Sleep(time.Millisecond * 100)
	}()
	go func() {
		linebuf := &bytes.Buffer{}
		for !hbtp.EndContext(c.prt.egn.ctx) && c.runReadOut(linebuf) {
			time.Sleep(time.Millisecond)
		}
		logrus.Debugf("runReadOut end!!!!")
		atomic.AddInt32(&wtn, -1)
	}()

	for wtn > 0 {
		time.Sleep(time.Millisecond * 100)
		if hbtp.EndContext(c.ctx) && c.cmdend.IsZero() {
			break
		}
		if !c.cmdend.IsZero() && time.Since(c.cmdend).Seconds() > 5 {
			time.Sleep(time.Second)
			break
		}
	}

	return cmderr
}

func (c *gitExec) runCmd() (rterr error) {
	defer func() {
		c.cmdend = time.Now()
		logrus.Debugf("gitExec end code:%d!!", c.prt.ExitCode)
		if err := recover(); err != nil {
			rterr = fmt.Errorf("recover:%v", err)
			logrus.Warnf("gitExec runCmd recover:%v", err)
			logrus.Warnf("Engine stack:%s", string(debug.Stack()))
		}
	}()

	gopt := &git.CloneOptions{
		Progress:     c.cmdout,
		SingleBranch: true,
	}
	shas := ""
	dirs := c.prt.repopth
	if c.prt.job.Input != nil {
		gopt.URL = c.prt.job.Input["url"]
		user := c.prt.job.Input["user"]
		token := c.prt.job.Input["token"]
		branch := c.prt.job.Input["branch"]
		shas = c.prt.job.Input["sha"]
		dir := c.prt.job.Input["directory"]
		logrus.Debugf("gitExec.runCmd input: user=%s,token=%d,branch=%s,shas=%s,dir=%s",user,len(token),branch,shas,dir)
		if dir == "" {
			dir = c.prt.job.Input["dir"]
		}
		if shas == "" {
			shas = branch
		}
		if dir != "" {
			dirs = filepath.Join(dirs, dir)
		}
		if token != "" {
			if user != "" {
				gopt.Auth = &ghttp.BasicAuth{
					Username: user,
					Password: token,
				}
			} else {
				gopt.Auth = &ghttp.TokenAuth{
					Token: token,
				}
			}
		}
	}
	if gopt.URL == "" {
		return errors.New("git urls is empty")
	}
	if shas != "" && !plumbing.IsHash(shas) {
		gopt.ReferenceName = plumbing.NewBranchReferenceName(shas)
	}
	rpy, err := util.CloneRepo(dirs, gopt, c.ctx)
	if err != nil {
		return fmt.Errorf("cloneRepo err:%v", err)
	}
	if plumbing.IsHash(shas) {
		err = util.CheckOutHash(rpy, shas)
		if err != nil {
			return fmt.Errorf("CheckOutHash [%s] err:%v", shas, err)
		}
	}

	return nil
}

//return false end thread
func (c *gitExec) runReadOut(linebuf *bytes.Buffer) bool {
	defer func() {
		if err := recover(); err != nil {
			logrus.Warnf("gitExec runReadOut recover:%v", err)
			logrus.Warnf("Engine stack:%s", string(debug.Stack()))
		}
	}()
	if c.cmdout == nil {
		return c.cmdend.IsZero()
	}
	bts := make([]byte, 1024)
	rn, err := c.cmdout.Read(bts)
	if rn <= 0 && !c.cmdend.IsZero() {
		if linebuf.Len() <= 0 {
			return false
		}
		//linebuf.WriteByte('\n')
		bts[0] = '\n'
		rn = 1
		err = nil
	}
	if err != nil {
		return c.cmdend.IsZero()
	}

	for i := 0; !hbtp.EndContext(c.prt.egn.ctx) && i < rn; i++ {
		if bts[i] == '\r' || bts[i] == '\n' {
			bs := linebuf.String()
			//logrus.Debugf("test log line:%s", bs)
			if bs != "" {
				c.pushCmdLine(bs, false)
			}
			linebuf.Reset()
		} else {
			linebuf.WriteByte(bts[i])
		}
	}
	return true
}
func (c *gitExec) pushCmdLine(bs string, iserr bool) {
	err := c.prt.egn.itr.PushOutLine(c.prt.job.BuildId, c.prt.job.Id, c.cmd.Id, bs, iserr)
	if err != nil {
		logrus.Errorf("gitExec PushOutLine err:%v", err)
	}
}
