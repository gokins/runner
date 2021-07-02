package runners

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gokins-main/core/utils"
	hbtp "github.com/mgr9525/HyperByte-Transfer-Protocol"
	"github.com/sirupsen/logrus"
)

type procExec struct {
	prt  *taskExec
	cmd  *CmdContent
	ctx  context.Context
	cncl context.CancelFunc
	envs map[string]string

	child  *exec.Cmd
	cmdend time.Time
	cmdinr io.WriteCloser
	cmdout io.ReadCloser
	cmderr io.ReadCloser
	spts   string
	sptck  bool
}

func (c *procExec) stop() {
	if c.cmdinr != nil {
		c.cmdinr.Close()
		c.cmdinr = nil
	}
	if c.cncl != nil {
		c.cncl()
		c.cncl = nil
	}
}
func (c *procExec) close() {
	defer func() {
		if err := recover(); err != nil {
			logrus.Warnf("procExec close recover:%v", err)
			logrus.Warnf("Engine stack:%s", string(debug.Stack()))
		}
	}()
	if c.cmdinr != nil {
		c.cmdinr.Close()
		c.cmdinr = nil
	}
	if c.cmdout != nil {
		c.cmdout.Close()
		c.cmdout = nil
	}
	if c.cmderr != nil {
		c.cmderr.Close()
		c.cmderr = nil
	}
}
func (c *procExec) start() (rterr error) {
	defer func() {
		c.stop()
		logrus.Debugf("procExec end code:%d!!", c.prt.ExitCode)
		if err := recover(); err != nil {
			rterr = fmt.Errorf("recover:%v", err)
			logrus.Warnf("procExec start recover:%v", err)
			logrus.Warnf("Engine stack:%s", string(debug.Stack()))
		}
	}()
	var err error
	var cmderr error

	if c.prt == nil || c.cmd == nil || c.cmd.Conts == "" {
		return nil
	}
	c.ctx, c.cncl = context.WithCancel(c.prt.cmdctx)
	bins, err := os.Executable()
	if err != nil {
		return err
	}

	name := "sh"
	pars := []string{"-c"}
	rands := utils.RandomString(10)
	c.spts = childprefix + rands
	if c.prt.job.Step == "shell@bash" {
		name = "bash"
	}
	/*if c.prt.job.Step == "shell@docker" {
		name = "docker"
		pars := []string{"run","--rm","","-c"}
	}*/
	ends := fmt.Sprintf("%s %s %s %s", bins, childcmd, "$?", rands)
	cmds := fmt.Sprintf("%s\n\n\n%s", c.cmd.Conts, ends)

	if c.prt.job.Step == "shell@cmd" {
		name = "cmd"
		pars = []string{"/c"}

		cmds = c.cmd.Conts
		//ends = fmt.Sprintf("%s %s %s %s", bins, childcmd, "%ERRORLEVEL%", rands)
		//cmds = fmt.Sprintf("%s\n\n\n%s", c.cmd.Conts, ends)
		//cmds = strings.ReplaceAll(cmds, "\r\n", "`r`n")
		//cmds = strings.ReplaceAll(cmds, "\n", "`n")
	}
	if c.prt.job.Step == "shell@powershell" {
		name = "powershell"
		pars = []string{"/c"}

		cmds = c.cmd.Conts
		//ends = fmt.Sprintf("%s %s %s %s", bins, childcmd, "$LASTEXITCODE", rands)
		//cmds = fmt.Sprintf("%s\n\n\n%s", c.cmd.Conts, ends)
		//cmds = strings.ReplaceAll(cmds, "\r\n", "`r`n")
		//cmds = strings.ReplaceAll(cmds, "\n", "`n")
	}

	pars = append(pars, cmds)
	cmd := exec.CommandContext(c.ctx, name, pars...)
	c.cmdinr, err = cmd.StdinPipe()
	if err != nil {
		return err
	}
	c.cmdout, err = cmd.StdoutPipe()
	if err != nil {
		return err
	}
	c.cmderr, err = cmd.StderrPipe()
	if err != nil {
		return err
	}

	cmd.Dir = c.prt.repopth
	if c.prt.cmdenv != nil && len(c.prt.cmdenv) > 0 {
		envs := []string{}
		c.prt.cmdenvlk.RLock() /*
			for k, v := range c.prt.cmdenv {
				if k != "" && v != "" {
					logrus.Debugf("put env[%s]:%s", k, v)
					envs = append(envs, k+"="+v)
				}
			} */
		for _, v := range c.prt.cmdenv {
			if v != "" {
				envs = append(envs, v)
			}
		}
		c.prt.cmdenvlk.RUnlock()
		if len(envs) > 0 {
			cmd.Env = envs
		}
	}
	err = cmd.Start()
	if err != nil {
		c.close()
		//c.prt.job.ExitCode=-1
		//c.prt.job.Error = fmt.Sprintf("command run err:%v", err)
		return err
	}

	c.child = cmd
	c.cmdend = time.Time{}
	c.sptck = false

	wtn := int32(3)
	go func() {
		cmderr = c.runCmd()
		c.killCmd()
		logrus.Debugf("runCmd end!!!!")
		atomic.AddInt32(&wtn, -1)
		time.Sleep(time.Millisecond * 100)
		c.close()
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
				c.child.Process.Signal(syscall.SIGINT)
				time.Sleep(time.Second)
			} else {
				c.stop()
				c.child.Process.Kill()
				break
			}
		}
		if !c.cmdend.IsZero() && time.Since(c.cmdend).Seconds() > 5 {
			c.close()
			time.Sleep(time.Second)
		}
	}
	return cmderr
}
func (c *procExec) runCmd() (rterr error) {
	defer func() {
		c.cmdend = time.Now()
		/*if c.cmdinr != nil {
			c.cmdinr.Close()
		}*/
		logrus.Debugf("procExec end code:%d!!", c.prt.ExitCode)
		if err := recover(); err != nil {
			rterr = fmt.Errorf("recover:%v", err)
			logrus.Warnf("procExec runCmd recover:%v", err)
			logrus.Warnf("Engine stack:%s", string(debug.Stack()))
		}
	}()

	err := c.child.Wait()
	if c.child.ProcessState != nil {
		c.prt.ExitCode = c.child.ProcessState.ExitCode()
	}
	logrus.Debugf("runCmd end(code:%d)!!!!", c.prt.ExitCode)
	if err != nil {
		return err
	}
	if c.prt.ExitCode != 0 {
		return fmt.Errorf("cmd err:%d", c.prt.ExitCode)
	}
	return nil
}

//return false end thread
func (c *procExec) runReadErr(linebuf *bytes.Buffer) bool {
	defer func() {
		if err := recover(); err != nil {
			logrus.Warnf("procExec runReadErr recover:%v", err)
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
		if bts[i] == '\n' {
			bs := linebuf.String()
			//logrus.Debugf("test errlog line:%s", bs)
			if bs == "" {

			} else if strings.Contains(bs, c.spts) {
				c.sptck = true
			} else if c.sptck {
				var env []string
				// env := utils.EnvVal{}
				err = json.Unmarshal(linebuf.Bytes(), &env)
				if err != nil {
					logrus.Debugf("end spts check err:%v", err)
				} else if len(env) > 0 {
					c.prt.cmdenvlk.Lock()
					c.prt.cmdenv = env
					/* for k, v := range env {
						c.prt.cmdenv[k] = v
					} */
					c.prt.cmdenvlk.Unlock()
				}
			} else {
				c.pushCmdLine(bs, true)
			}
			linebuf.Reset()
		} else {
			linebuf.WriteByte(bts[i])
		}
	}
	return true
}

//return false end thread
func (c *procExec) runReadOut(linebuf *bytes.Buffer) bool {
	defer func() {
		if err := recover(); err != nil {
			logrus.Warnf("procExec runReadOut recover:%v", err)
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
		if bts[i] == '\n' {
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
func (c *procExec) pushCmdLine(bs string, iserr bool) {
	err := c.prt.egn.itr.PushOutLine(c.prt.job.Id, c.cmd.Id, bs, iserr)
	if err != nil {
		logrus.Errorf("procExec PushOutLine err:%v", err)
	}
}
