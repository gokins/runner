package runners

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gokins/core/common"
	"github.com/gokins/core/runtime"
	"github.com/gokins/core/utils"
	hbtp "github.com/mgr9525/HyperByte-Transfer-Protocol"
	"github.com/pkg/sftp"
	"github.com/sirupsen/logrus"
)

func (c *taskExec) runProcs() (rterr error) {
	defer func() {
		if err := recover(); err != nil {
			rterr = fmt.Errorf("recover:%v", err)
			logrus.Warnf("taskExec runProcs recover:%v", err)
			logrus.Warnf("Engine stack:%s", string(debug.Stack()))
		}
	}()

	var err error
	for _, v := range c.job.Commands {
		if err != nil {
			errs := c.egn.itr.UpdateCmd(c.job.BuildId, c.job.Id, v.Id, 3, c.ExitCode)
			if errs != nil {
				logrus.Errorf("cmdExec runCmdNext UpdateCmd err:%v", errs)
			}
			continue
		}

		errs := c.egn.itr.UpdateCmd(c.job.BuildId, c.job.Id, v.Id, 1, 0)
		if errs != nil {
			logrus.Errorf("cmdExec runCmdNext UpdateCmd err:%v", errs)
		}
		if c.sshcli != nil {
			ex := &sshExec{
				prt:    c,
				cmd:    v,
				client: c.sshcli,
			}
			err = ex.start()
		} else {
			ex := &procExec{
				prt: c,
				cmd: v,
			}
			err = ex.start()
		}
		fs := 2
		if err != nil {
			if hbtp.EndContext(c.cmdctx) {
				fs = 3
			} else {
				fs = -1
			}
		}
		errs = c.egn.itr.UpdateCmd(c.job.BuildId, c.job.Id, v.Id, fs, c.ExitCode)
		if errs != nil {
			logrus.Errorf("cmdExec runCmdNext UpdateCmd err:%v", errs)
		}
	}
	return err
}
func (c *taskExec) checkRepo() (rterr error) {
	defer func() {
		if err := recover(); err != nil {
			rterr = fmt.Errorf("recover:%v", err)
			logrus.Warnf("taskExec getArts recover:%v", err)
			logrus.Warnf("Engine stack:%s", string(debug.Stack()))
		}
	}()
	if c.repocpd {
		return nil
	}
	_, err := os.Stat(c.repopth)
	logrus.Debugf("taskExec.checkRepo(%s) err:%v", c.repopth, err)
	if err != nil {
		return c.copyServDir(1, "/", c.repopth)
	}
	return nil
}
func (c *taskExec) copyServDir(fs int, pth, root2s string, rmtPrefix ...string) error {
	defer func() {
		if err := recover(); err != nil {
			logrus.Warnf("taskExec getArts recover:%v", err)
			logrus.Warnf("Engine stack:%s", string(debug.Stack()))
		}
	}()
	rpth := pth
	if len(rmtPrefix) > 0 && rmtPrefix[0] != "" {
		rpth = filepath.Join(rmtPrefix[0], pth)
	}
	fls, err := c.egn.itr.ReadDir(fs, c.job.BuildId, rpth)
	if err != nil {
		return err
	}
	tpth := filepath.Join(root2s, pth)
	os.MkdirAll(tpth, 0750)
	logrus.Debugf("copyServDir MkdirAll:%s", tpth)
	for _, v := range fls {
		pths := filepath.Join(pth, v.Name)
		if v.IsDir {
			err = c.copyServDir(fs, pths, root2s, rmtPrefix...)
		} else {
			err = c.cprepofl(fs, pths, root2s, rmtPrefix...)
		}
		if err != nil {
			// os.RemoveAll(tpth)
			return err
		}
	}
	return nil
}
func (c *taskExec) cprepofl(fs int, pth, root2s string, rmtPrefix ...string) error {
	var err error
	for i := 0; i < 10; i++ {
		if hbtp.EndContext(c.cmdctx) {
			err = fmt.Errorf("ctx end")
			break
		}
		err = c.cprepoFile(i, fs, pth, root2s, rmtPrefix...)
		if err == nil {
			break
		}
	}
	return err
}
func (c *taskExec) cprepoFile(idx int, fs int, pth, root2s string, rmtPrefix ...string) error {
	rpth := pth
	if len(rmtPrefix) > 0 && rmtPrefix[0] != "" {
		rpth = filepath.Join(rmtPrefix[0], pth)
	}
	ln := int64(0)
	flpth := filepath.Join(root2s, pth)
	if idx <= 0 {
		os.Remove(flpth)
	} else {
		stat, err := os.Stat(flpth)
		if err == nil {
			ln = stat.Size()
		}
	}
	sz, flr, err := c.egn.itr.ReadFile(fs, c.job.BuildId, rpth, ln)
	if err != nil {
		return err
	}
	if sz == ln {
		return nil
	}
	if ln > sz {
		os.RemoveAll(flpth)
	}
	defer flr.Close()
	logrus.Debugf("cprepofl copy:(%s)%s->%s", c.job.BuildId, rpth, flpth)
	flw, err := os.OpenFile(flpth, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0640)
	if err != nil {
		return err
	}
	defer flw.Close()
	if ln > 0 {
		flw.Seek(ln, io.SeekStart)
	}

	bts := make([]byte, 10240)
	for {
		if hbtp.EndContext(c.cmdctx) {
			return errors.New("ctx end")
		}
		rn, err := flr.Read(bts)
		if rn <= 0 {
			break
		}
		wn, _ := flw.Write(bts[:rn])
		ln += int64(wn)
		if err != nil {
			break
		}
	}
	logrus.Debugf("cp file size end:%d/%d", ln, sz)
	if ln != sz {
		return fmt.Errorf("cp file size err:%d/%d", ln, sz)
	}
	return nil
}
func (c *taskExec) chkArtPath(pth string) (string, error) {
	pths := pth
	if pth == "." {
		pths = c.repopth
	} else if !strings.HasPrefix(pth, "/") {
		pths = filepath.Join(c.repopth, pth)
	}
	stat, err := os.Stat(pths)
	if err == nil {
		if stat.IsDir() {
			return pths, nil
		} else {
			return pths, errors.New("artifact path is not dir")
		}
	}
	return pths, os.MkdirAll(pths, 0750)
}
func (c *taskExec) getArtsFiles(v *runtime.UseArtifact, fs int, rmtPrefix string) error {
	if c.sshcli == nil {
		pths, err := c.chkArtPath(v.Path)
		if err != nil {
			return err
		}
		err = c.copyServDir(2, "/", pths, rmtPrefix)
		if err != nil {
			return err
		}
	} else {
		stpcli, err := sftp.NewClient(c.sshcli)
		if err != nil {
			return err
		}
		defer stpcli.Close()
		pths, err := c.chkArtPathSSH(stpcli, v.Path)
		if err != nil {
			return err
		}
		err = c.copyServDirSSH(stpcli, fs, "/", pths, rmtPrefix)
		if err != nil {
			return err
		}
	}
	return nil
}
func (c *taskExec) getArts() (rterr error) {
	defer func() {
		if err := recover(); err != nil {
			rterr = fmt.Errorf("recover:%v", err)
			logrus.Warnf("taskExec getArts recover:%v", err)
			logrus.Warnf("Engine stack:%s", string(debug.Stack()))
		}
	}()
	servinfo, err := c.egn.itr.ServerInfo()
	if err != nil {
		return err
	}
	for _, v := range c.job.UseArtifacts {
		switch v.Scope {
		case common.ArtsArchive, common.ArtsRepo:
			verid, err := c.egn.itr.FindArtVersionId(c.job.BuildId, v.Repository, v.Name)
			if err != nil {
				return err
			}
			if v.IsUrl || v.Path == "" {
				tms := time.Now().Format(time.RFC3339Nano)
				random := utils.RandomString(20)
				sign := utils.Md5String(verid + tms + random + servinfo.DownToken)
				/*pths:=v.Path
				if pths[0]=='/'{
					pths=pths[1:]
				}*/
				ul := fmt.Sprintf("%s/api/art/pub/down/%s/%s?times=%s&random=%s&sign=%s",
					servinfo.WebHost, verid, v.Path, url.QueryEscape(tms), random, sign)
				als := v.Alias
				if als == "" {
					als = v.Name
				}
				c.cmdenvlk.Lock()
				c.cmdenvs["ARTIFACT_DOWNURL_"+als] = ul
				c.cmdenvlk.Unlock()
			} else {
				err = c.getArtsFiles(v, 2, verid)
				if err != nil {
					return err
				}
				/* pths, err := c.chkArtPath(v.Path)
				if err != nil {
					return err
				}
				err = c.copyServDir(2, "/", pths, verid)
				if err != nil {
					return err
				} */
			}
		case common.ArtsPipeline, common.ArtsPipe:
			if v.SourceStage == "" {
				v.SourceStage = c.job.StageName
			}
			if v.SourceStep == "" {
				return fmt.Errorf("'%s' fromStep is empty", c.job.Name)
			}
			jid, ok := c.egn.itr.FindJobId(c.job.BuildId, v.SourceStage, v.SourceStep)
			if !ok {
				return fmt.Errorf("'%s' Not Found fromStep '%s->%s'", c.job.Name, v.SourceStage, v.SourceStep)
			}
			if v.IsUrl || v.Path == "" {
				tms := time.Now().Format(time.RFC3339Nano)
				random := utils.RandomString(20)
				sign := utils.Md5String(jid + v.Name + tms + random + servinfo.DownToken)
				/*pths:=v.Path
				if pths[0]=='/'{
					pths=pths[1:]
				}*/
				ul := fmt.Sprintf("%s/api/art/pub/downs/%s/%s/%s?times=%s&random=%s&sign=%s",
					servinfo.WebHost, jid, v.Name, v.Path, url.QueryEscape(tms), random, sign)
				als := v.Alias
				if als == "" {
					als = v.Name
				}
				c.cmdenvlk.Lock()
				c.cmdenvs["ARTIFACT_DOWNURL_"+als] = ul
				c.cmdenvlk.Unlock()
			} else {
				err = c.getArtsFiles(v, 3, filepath.Join("/", jid, common.PathArts, v.Name))
				if err != nil {
					return err
				}
				/* pths, err := c.chkArtPath(v.Path)
				if err != nil {
					return err
				}
				err = c.copyServDir(3, "/", pths, filepath.Join("/", jid, common.PathArts, v.Name))
				if err != nil {
					return err
				} */
			}
		case common.ArtsVariable, common.ArtsVar:
			if v.SourceStage == "" {
				v.SourceStage = c.job.StageName
			}
			if v.SourceStep == "" {
				return fmt.Errorf("'%s' fromStep is empty", c.job.Name)
			}
			jid, ok := c.egn.itr.FindJobId(c.job.BuildId, v.SourceStage, v.SourceStep)
			if !ok {
				return fmt.Errorf("'%s' Not Found fromStep '%s->%s'", c.job.Name, v.SourceStage, v.SourceStep)
			}
			val, ok := c.egn.itr.GetEnv(c.job.BuildId, jid, v.Name)
			if ok {
				c.cmdenvlk.Lock()
				c.cmdenvs[v.Name] = val
				c.cmdenvlk.Unlock()
			}
		}
	}
	return nil
}
func (c *taskExec) chkArtsPath(pth string) (string, os.FileInfo, error) {
	pths := filepath.Join(c.repopth, pth)
	stat, err := os.Stat(pths)
	if err != nil {
		return pths, nil, err
	}
	return pths, stat, nil
}
func (c *taskExec) genArtsFiles(v *runtime.Artifact, fs int, name string) error {
	if c.sshcli == nil {
		pths, stat, err := c.chkArtsPath(v.Path)
		if err != nil {
			return err
		}
		if stat.IsDir() {
			err = c.uploaddir(fs, name, stat.Name(), pths)
		} else {
			err = c.uploadfl(fs, name, stat.Name(), pths)
		}
		if err != nil {
			return err
		}
	} else {
		stpcli, err := sftp.NewClient(c.sshcli)
		if err != nil {
			return err
		}
		defer stpcli.Close()
		pths, stat, err := c.chkArtsPathSSH(stpcli, v.Path)
		if err != nil {
			return err
		}
		if stat.IsDir() {
			err = c.uploaddirSSH(stpcli, fs, name, stat.Name(), pths)
		} else {
			err = c.uploadflSSH(stpcli, fs, name, stat.Name(), pths)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
func (c *taskExec) genArts() (rterr error) {
	defer func() {
		if err := recover(); err != nil {
			rterr = fmt.Errorf("recover:%v", err)
			logrus.Warnf("taskExec genArts recover:%v", err)
			logrus.Warnf("Engine stack:%s", string(debug.Stack()))
		}
	}()

	env := utils.EnvVal{}
	for _, v := range c.job.Artifacts {
		switch v.Scope {
		case common.ArtsArchive, common.ArtsRepo:
			verid, err := c.egn.itr.NewArtVersionId(c.job.BuildId, v.Repository, v.Name)
			if err != nil {
				return err
			}
			err = c.genArtsFiles(v, 1, verid)
			if err != nil {
				return err
			}
			/* pths, stat, err := c.chkArtsPath(v.Path)
			if err != nil {
				return err
			}
			if stat.IsDir() {
				err = c.uploaddir(1, verid, stat.Name(), pths)
			} else {
				err = c.uploadfl(1, verid, stat.Name(), pths)
			}
			if err != nil {
				return err
			} */
		case common.ArtsPipeline, common.ArtsPipe:
			err := c.genArtsFiles(v, 2, v.Name)
			if err != nil {
				return err
			}
			/* pths, stat, err := c.chkArtsPath(v.Path)
			if err != nil {
				return err
			}
			if stat.IsDir() {
				err = c.uploaddir(2, v.Name, stat.Name(), pths)
			} else {
				err = c.uploadfl(2, v.Name, stat.Name(), pths)
			}
			if err != nil {
				return err
			} */
		case common.ArtsVariable, common.ArtsVar:
			c.cmdenvlk.RLock()
			env[v.Name] = c.cmdenv[v.Name]
			c.cmdenvlk.RUnlock()
		}
	}
	if len(env) > 0 {
		err := c.egn.itr.GenEnv(c.job.BuildId, c.job.Id, env)
		if err != nil {
			return err
		}
	}
	return nil
}
func (c *taskExec) uploaddir(fs int, dir, pth, flpth string) error {
	fls, err := ioutil.ReadDir(flpth)
	if err != nil {
		return err
	}
	for _, v := range fls {
		pths := filepath.Join(pth, v.Name())
		flpths := filepath.Join(flpth, v.Name())
		if v.IsDir() {
			err = c.uploaddir(fs, dir, pths, flpths)
		} else {
			err = c.uploadfl(fs, dir, pths, flpths)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
func (c *taskExec) uploadfl(fs int, dir, pth, flpth string) error {
	var err error
	for i := 0; i < 10; i++ {
		if hbtp.EndContext(c.cmdctx) {
			err = fmt.Errorf("ctx end")
			break
		}
		err = c.uploadFile(fs, dir, pth, flpth)
		if err == nil {
			break
		}
	}
	return err
}
func (c *taskExec) uploadFile(fs int, dir, pth, flpth string) error {
	stat, err := os.Stat(flpth)
	if err != nil {
		return err
	}
	ln := int64(0)
	sz := stat.Size()
	stats, err := c.egn.itr.StatFile(fs, c.job.BuildId, c.job.Id, dir, pth)
	if err == nil {
		ln = stats.Size
	}
	if sz == ln {
		return nil
	}
	if ln > sz {
		ln = 0
	}
	fl, err := os.Open(flpth)
	if err != nil {
		return err
	}
	defer fl.Close()
	if ln > 0 {
		fl.Seek(ln, io.SeekStart)
	}
	wt, err := c.egn.itr.UploadFile(fs, c.job.BuildId, c.job.Id, dir, pth, ln)
	if err != nil {
		return err
	}
	defer wt.Close()

	bts := make([]byte, 10240)
	for {
		if hbtp.EndContext(c.cmdctx) {
			return errors.New("ctx end")
		}
		rn, err := fl.Read(bts)
		if rn <= 0 {
			break
		}
		wn, _ := wt.Write(bts[:rn])
		ln += int64(wn)
		if err != nil {
			break
		}
	}
	if ln != stat.Size() {
		return fmt.Errorf("cp file size err:%d/%d", ln, stat.Size())
	}
	return nil
}
