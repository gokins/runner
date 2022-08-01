package runners

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"time"

	hbtp "github.com/mgr9525/HyperByte-Transfer-Protocol"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

type sshExec struct {
	prt    *taskExec
	ctx    context.Context
	cncl   context.CancelFunc
	cmd    *CmdContent
	client *ssh.Client

	child  *ssh.Session
	cmdend time.Time
	cmdout io.Reader
	cmderr io.Reader
}

func (c *sshExec) stop() {
	if c.cncl != nil {
		c.cncl()
		c.cncl = nil
	}
}
func (c *sshExec) close() {
	defer func() {
		if err := recover(); err != nil {
			logrus.Warnf("sshExec close recover:%v", err)
			logrus.Warnf("Engine stack:%s", string(debug.Stack()))
		}
	}()
	if c.child != nil {
		c.child.Close()
	}
}
func (c *sshExec) start() (rterr error) {
	defer func() {
		c.stop()
		logrus.Debugf("sshExec end code:%d!!", c.prt.ExitCode)
		if err := recover(); err != nil {
			rterr = fmt.Errorf("recover:%v", err)
			logrus.Warnf("sshExec start recover:%v", err)
			logrus.Warnf("Engine stack:%s", string(debug.Stack()))
		}
	}()

	if c.prt == nil {
		return nil
	}
	c.ctx, c.cncl = context.WithCancel(c.prt.cmdctx)
	session, err := c.client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()
	c.cmdout, err = session.StdoutPipe()
	if err != nil {
		return err
	}
	c.cmderr, err = session.StderrPipe()
	if err != nil {
		return err
	}

	c.child = session
	buf := &bytes.Buffer{}
	dirs := c.prt.job.UsersRepo
	/* if c.prt.job.Input != nil {
		dirs := c.prt.job.Input["workspace"]
		if dirs == "" {
			dirs = c.prt.job.UsersRepo
		}
	} */
	if dirs != "" {
		s := strings.ReplaceAll(dirs, "'", "")
		s = strings.ReplaceAll(s, "\n", " ")
		buf.WriteString(fmt.Sprintf("cd '%s'", s))
		buf.WriteString("\n")
	}
	c.prt.cmdenvlk.RLock()
	for k, v := range c.prt.cmdenvs {
		_, ok := c.prt.job.Env[k]
		if !ok && k != "" {
			s := strings.ReplaceAll(v, "'", "\\'")
			s = strings.ReplaceAll(s, "\n", " ")
			buf.WriteString(fmt.Sprintf("export %s='%s'", k, s))
			buf.WriteString("\n")
		}
	}
	c.prt.cmdenvlk.RUnlock()
	if c.prt.job.Env != nil && len(c.prt.job.Env) > 0 {
		for k, v := range c.prt.job.Env {
			if k != "" {
				s := strings.ReplaceAll(v, "'", "\\'")
				s = strings.ReplaceAll(s, "\n", " ")
				buf.WriteString(fmt.Sprintf("export %s='%s'", k, s))
				buf.WriteString("\n")
			}
		}
	}

	buf.WriteString(c.cmd.Conts)
	buf.WriteString("\n\n\n")
	cmds := buf.String()
	err = session.Start(cmds)
	if err != nil {
		return err
	}
	var cmderr error
	wtn := int32(3)
	go func() {
		cmderr = c.runCmd()
		// c.killCmd()
		logrus.Debugf("runCmd end!!!!")
		atomic.AddInt32(&wtn, -1)
		time.Sleep(time.Millisecond * 100)
	}()
	go func() {
		linebuf := &bytes.Buffer{}
		for !hbtp.EndContext(c.prt.egn.ctx) && c.runReadErr(linebuf) {
			time.Sleep(time.Millisecond)
		}
		logrus.Debugf("runReadErr end!!!!")
		atomic.AddInt32(&wtn, -1)
	}()
	go func() {
		linebuf := &bytes.Buffer{}
		for !hbtp.EndContext(c.prt.egn.ctx) && c.runReadOut(linebuf) {
			time.Sleep(time.Millisecond)
		}
		logrus.Debugf("runReadOut end!!!!")
		atomic.AddInt32(&wtn, -1)
	}()

	ln := 0
	for wtn > 0 {
		time.Sleep(time.Millisecond * 100)
		if hbtp.EndContext(c.ctx) && c.cmdend.IsZero() {
			ln++
			if ln <= 3 {
				c.child.Signal(ssh.SIGINT)
				time.Sleep(time.Second)
			} else {
				c.child.Signal(ssh.SIGKILL)
				break
			}
		}
		if !c.cmdend.IsZero() && time.Since(c.cmdend).Seconds() > 5 {
			time.Sleep(time.Second)
			break
		}
	}

	return cmderr
}
func (c *sshExec) runCmd() (rterr error) {
	defer func() {
		c.cmdend = time.Now()
		logrus.Debugf("sshExec end code:%d!!", c.prt.ExitCode)
		if err := recover(); err != nil {
			rterr = fmt.Errorf("recover:%v", err)
			logrus.Warnf("sshExec runCmd recover:%v", err)
			logrus.Warnf("Engine stack:%s", string(debug.Stack()))
		}
	}()

	return c.child.Wait()
}

//return false end thread
func (c *sshExec) runReadErr(linebuf *bytes.Buffer) bool {
	defer func() {
		if err := recover(); err != nil {
			logrus.Warnf("sshExec runReadErr recover:%v", err)
			logrus.Warnf("Engine stack:%s", string(debug.Stack()))
		}
	}()
	if c.cmderr == nil {
		return c.cmdend.IsZero()
	}
	bts := make([]byte, 1024)
	rn, err := c.cmderr.Read(bts)
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
			//logrus.Debugf("test errlog line:%s", bs)
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

//return false end thread
func (c *sshExec) runReadOut(linebuf *bytes.Buffer) bool {
	defer func() {
		if err := recover(); err != nil {
			logrus.Warnf("sshExec runReadOut recover:%v", err)
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
func (c *sshExec) pushCmdLine(bs string, iserr bool) {
	err := c.prt.egn.itr.PushOutLine(c.prt.job.BuildId, c.prt.job.Id, c.cmd.Id, bs, iserr)
	if err != nil {
		logrus.Errorf("sshExec PushOutLine err:%v", err)
	}
}
