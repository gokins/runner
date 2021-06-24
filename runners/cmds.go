package runners

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/gokins-main/core/common"
	"github.com/gokins-main/core/utils"
	hbtp "github.com/mgr9525/HyperByte-Transfer-Protocol"
	"github.com/sirupsen/logrus"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
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
	cmds   []*CmdContent
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
	if c.prt == nil {
		return errors.New("parent is nil")
	}
	c.ctx, c.cncl = context.WithCancel(c.prt.cmdctx)
	defer func() {
		c.stop()
		logrus.Debugf("cmdExec end code:%d!!", c.prt.ExitCode)
		if err := recover(); err != nil {
			logrus.Warnf("cmdExec start recover:%v", err)
			logrus.Warnf("Engine stack:%s", string(debug.Stack()))
		}
	}()

	c.prt.Status = common.BuildStatusError

	cmderr := make(chan error)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		cmderr <- c.runCmd()
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		linebuf := &bytes.Buffer{}
		for !hbtp.EndContext(c.prt.prt.ctx) && !c.runReadErr(linebuf) {
			time.Sleep(time.Millisecond)
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		linebuf := &bytes.Buffer{}
		for !hbtp.EndContext(c.prt.prt.ctx) && !c.runReadOut(linebuf) {
			time.Sleep(time.Millisecond)
		}
	}()

	for !hbtp.EndContext(c.prt.prt.ctx) && !c.cmdend {
		time.Sleep(time.Millisecond)
		if time.Since(c.linetm).Hours() > 2 {
			c.linetm = c.linetm.Add(time.Minute * 20)
			c.stop()
		}
	}
	wg.Wait()
	return <-cmderr
}

func (c *cmdExec) writeEnvs(k, v string) {
	//vs := strings.ReplaceAll(v, "\t", ``)
	vs := strings.ReplaceAll(v, "\n", `\n`)
	vs = strings.ReplaceAll(vs, `"`, `\"`)
	//vs = strings.ReplaceAll(vs, `'`, `\'`)
	c.cmdWriteString("\n")
	c.cmdWriteString(fmt.Sprintf(`export %s="%s"`, k, vs))
	c.cmdWriteString("\n")
	logrus.Debugf(`cmdExec write env:%s="%s"`, k, vs)
}
func (c *cmdExec) writeSplit() {
	c.cmdWriteString("\n")
	c.cmdWriteString("\n")
	c.cmdWriteString(`echo ""`)
	// c.cmdWriteString(`echo -e "\n"`)
	c.cmdWriteString("\n")
	c.cmdWriteString(fmt.Sprintf(`echo "%s"`, c.spts))
	c.cmdWriteString("\n")
}
func (c *cmdExec) cmdWriteString(s string) {
	if c.cmdbuf != nil {
		c.cmdbuf.Write([]byte(s))
	}
}
func (c *cmdExec) runCmd() error {
	defer func() {
		c.cmdend = true
		/*if c.cmdind >= 0 && c.cmdind < len(c.cmds) {
			it := c.cmds[c.cmdind]
			m.Gid = it.Gid
			m.Pid = it.Id
			//m.Name = it.name
		}*/
		if err := recover(); err != nil {
			logrus.Warnf("cmdExec runCmd recover:%v", err)
			logrus.Warnf("Engine stack:%s", string(debug.Stack()))
		}
	}()

	name := "sh"
	if c.prt.job.Step == "shell@bash" {
		name = "bash"
	}
	if c.prt.job.Step == "shell@cmd" {
		name = "cmd"
	}
	if c.prt.job.Step == "shell@powershell" {
		name = "powershell"
	}
	c.spts = fmt.Sprintf("*********%s", utils.RandomString(6))
	cmd := exec.CommandContext(c.ctx, name)
	iner, err := cmd.StdinPipe()
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

	c.cmd = cmd
	c.cmdbuf = iner

	cmd.Dir = c.prt.repopth
	err = cmd.Start()
	if err != nil {
		//c.prt.job.ExitCode=-1
		//c.prt.job.Error = fmt.Sprintf("command run err:%v", err)
		return err
	}
	c.writeEnvs("SYSPATH", "$PATH")
	c.writeEnvs("WORKDIR", c.prt.wrkpth)
	if c.prt.job.Environments != nil {
		for k, v := range c.prt.job.Environments {
			c.writeEnvs(k, v)
		}
	}
	if c.envs != nil {
		for k, v := range c.envs {
			c.writeEnvs(k, v)
		}
	}
	c.cmdWriteString("\n")
	c.cmdWriteString(`echo "ready"`)
	c.cmdWriteString("\n")
	c.writeSplit()
	err = cmd.Wait()
	if cmd.ProcessState != nil {
		c.prt.ExitCode = cmd.ProcessState.ExitCode()
	}
	logrus.Debugf("job %s cmd end code:%d", c.prt.job.Name, c.prt.ExitCode)
	time.Sleep(time.Second)
	if err != nil || c.prt.ExitCode != 0 {
		//c.job.Status = common.BUILD_STATUS_ERROR
		//c.job.Error = fmt.Sprintf("command run err(code:%d):%v", c.job.ExitCode, err)
		return fmt.Errorf("(code:%d):%v", c.prt.ExitCode, err)
	}
	return nil
}
func (c *cmdExec) runReadErr(linebuf *bytes.Buffer) bool {
	defer func() {
		if err := recover(); err != nil {
			logrus.Warnf("cmdExec runReadErr recover:%v", err)
			logrus.Warnf("Engine stack:%s", string(debug.Stack()))
		}
	}()
	if c.cmderr == nil {
		return c.cmdend
	}
	bts := make([]byte, 1024)
	rn, err := c.cmderr.Read(bts)
	if rn <= 0 && c.cmdend {
		if linebuf.Len() <= 0 {
			return true
		}
		linebuf.WriteByte('\n')
		err = nil
	}
	if err != nil {
		return c.cmdend
	}
	for i := 0; !hbtp.EndContext(c.prt.prt.ctx) && i < rn; i++ {
		if bts[i] == '\n' {
			c.linetm = time.Now()
			bs := string(linebuf.Bytes())
			//logrus.Debugf("test log line:%s",bs)
			if bs != "" {
				c.pushCmdLine(bs, true)
			}
			linebuf.Reset()
		} else {
			linebuf.WriteByte(bts[i])
		}
	}
	return false
}
func (c *cmdExec) runReadOut(linebuf *bytes.Buffer) bool {
	defer func() {
		if err := recover(); err != nil {
			logrus.Warnf("cmdExec runReadOut recover:%v", err)
			logrus.Warnf("Engine stack:%s", string(debug.Stack()))
		}
	}()
	if c.cmdout == nil {
		return c.cmdend
	}
	bts := make([]byte, 1024)
	rn, err := c.cmdout.Read(bts)
	if rn <= 0 && c.cmdend {
		if linebuf.Len() <= 0 {
			return true
		}
		linebuf.WriteByte('\n')
		err = nil
	}
	if err != nil {
		return c.cmdend
	}

	for i := 0; !hbtp.EndContext(c.prt.prt.ctx) && i < rn; i++ {
		if bts[i] == '\n' {
			c.linetm = time.Now()
			bs := string(linebuf.Bytes())
			if bs == "" {

			} else if strings.Contains(bs, c.spts) {
				c.runCmdNext()
			} else {
				c.pushCmdLine(bs, false)
			}
			linebuf.Reset()
		} else {
			linebuf.WriteByte(bts[i])
		}
	}
	return false
}
func (c *cmdExec) runCmdNext() {
	defer func() {
		if err := recover(); err != nil {
			logrus.Warnf("cmdExec runCmdNext recover:%v", err)
			logrus.Warnf("Engine stack:%s", string(debug.Stack()))
		}
	}()

	ln := len(c.cmds)
	if c.cmdind >= 0 && c.cmdind < ln {
		it := c.cmds[c.cmdind]
		err := c.prt.prt.itr.UpdateCmd(c.prt.job.Id, it.Id, 2)
		if err != nil {
			logrus.Errorf("cmdExec runCmdNext UpdateCmd err:%v", err)
		}
	}
	c.cmdind++
	if c.cmdind >= ln {
		cmds, err := os.Executable()
		flpth := filepath.Join(c.prt.wrkpth, "envs")
		if err != nil {
			logrus.Debugf("cmdExec run Executable err:%v", err)
			return
		}
		cmdenvs := fmt.Sprintf("%s envs %s", cmds, flpth)
		// logrus.Debugf("cmdExec run Executable envs:%s", cmdenvs)
		c.cmdWriteString("\n")
		c.cmdWriteString(cmdenvs)
		c.cmdWriteString("\n")
		c.cmdWriteString("sleep 1s\nexit $?")
		//logrus.Debugf("job exec exit!!!")
		time.Sleep(time.Second)
		c.cmdbuf.Close()
		c.cmdbuf = nil
		time.Sleep(time.Second)
		c.stop()
	} else {
		it := c.cmds[c.cmdind]
		err := c.prt.prt.itr.UpdateCmd(c.prt.job.Id, it.Id, 1)
		if err != nil {
			logrus.Errorf("cmdExec runCmdNext UpdateCmd err:%v", err)
		}
		c.cmdWriteString("\n")
		c.cmdWriteString(`echo ""`)
		c.cmdWriteString("\n")
		c.cmdWriteString(it.Conts)
		c.cmdWriteString("\n")
		c.cmdWriteString(`
GiteeGO_TEMP_CODE=$?
if [ $GiteeGO_TEMP_CODE -ne 0 ];then
	sleep 1s
	exit $GiteeGO_TEMP_CODE
fi
		`)
		//echo "command status err:$GiteeGO_TEMP_CODE" 1>&2
		c.cmdWriteString("\n")
		c.writeSplit()
		logrus.Debugf("cmdExec write cmdLine:%s", it.Conts)
	}
}
func (c *cmdExec) pushCmdLine(bs string, iserr bool) {
	cmdid := ""
	if c.cmdind >= 0 {
		it := c.cmds[c.cmdind]
		cmdid = it.Id
	}
	err := c.prt.prt.itr.PushOutLine(c.prt.job.Id, cmdid, bs, iserr)
	if err != nil {
		logrus.Errorf("cmdExec PushOutLine err:%v", err)
	}
}
