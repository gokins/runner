package runners

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/gokins-main/core/common"
	"github.com/gokins-main/core/utils"
	hbtp "github.com/mgr9525/HyperByte-Transfer-Protocol"
	"github.com/sirupsen/logrus"
)

func (c *taskExec) checkRepo() (rterr error) {
	defer func() {
		if err := recover(); err != nil {
			rterr = fmt.Errorf("recover:%v", err)
			logrus.Warnf("taskExec getArts recover:%v", err)
			logrus.Warnf("Engine stack:%s", string(debug.Stack()))
		}
	}()
	_, err := os.Stat(c.repopth)
	if err == nil {
		return errors.New("path is exist")
	}
	return c.copyServDir(1, "/", c.repopth)
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
	os.MkdirAll(filepath.Join(root2s, pth), 0750)
	for _, v := range fls {
		pths := filepath.Join(pth, v.Name)
		if v.IsDir {
			err = c.copyServDir(fs, pths, root2s, rmtPrefix...)
		} else {
			err = c.cprepofl(fs, pths, root2s, rmtPrefix...)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
func (c *taskExec) cprepofl(fs int, pth, root2s string, rmtPrefix ...string) error {
	rpth := pth
	if len(rmtPrefix) > 0 && rmtPrefix[0] != "" {
		rpth = filepath.Join(rmtPrefix[0], pth)
	}
	sz, flr, err := c.egn.itr.ReadFile(fs, c.job.BuildId, rpth)
	if err != nil {
		return err
	}
	defer flr.Close()
	flpth := filepath.Join(root2s, pth)
	flw, err := os.OpenFile(flpth, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0640)
	if err != nil {
		return err
	}
	defer flr.Close()

	ln := int64(0)
	bts := make([]byte, 10240)
	for {
		if hbtp.EndContext(c.cmdctx) {
			return errors.New("ctx end")
		}
		rn, err := flr.Read(bts)
		if rn > 0 {
			wn, _ := flw.Write(bts[:rn])
			ln += int64(wn)
		}
		if err != nil {
			break
		}
	}
	if ln != sz {
		return fmt.Errorf("cp file size err:%d/%d", ln, sz)
	}
	return nil
}
func (c *taskExec) chkArtPath(pth string) (string, error) {
	pths := filepath.Join(c.repopth, pth)
	stat, err := os.Stat(pths)
	if err == nil {
		if stat.IsDir() {
			return "", nil
		} else {
			return "", errors.New("artifact path is not dir")
		}
	}
	return pths, os.MkdirAll(pths, 0750)
}
func (c *taskExec) getArts() (rterr error) {
	defer func() {
		if err := recover(); err != nil {
			rterr = fmt.Errorf("recover:%v", err)
			logrus.Warnf("taskExec getArts recover:%v", err)
			logrus.Warnf("Engine stack:%s", string(debug.Stack()))
		}
	}()
	for _, v := range c.job.UseArtifacts {
		switch v.Scope {
		case common.ArtsArchive, common.ArtsRepo:
			pths, err := c.chkArtPath(v.Path)
			if err != nil {
				return err
			}
			verid, err := c.egn.itr.FindArtVersionId(c.job.BuildId, v.Repository, v.Name)
			if err != nil {
				return err
			}
			err = c.copyServDir(2, "/", pths, verid)
			if err != nil {
				return err
			}
		case common.ArtsPipeline, common.ArtsPipe:
			pths, err := c.chkArtPath(v.Path)
			if err != nil {
				return err
			}
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
			err = c.copyServDir(3, "/", pths, filepath.Join("/", jid, common.PathArts, v.Name))
			if err != nil {
				return err
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
				c.cmdenv[v.Name] = val
			}
		}
	}
	return nil
}
func (c *taskExec) chkArtsPath(pth string) (string, os.FileInfo, error) {
	pths := filepath.Join(c.repopth, pth)
	stat, err := os.Stat(pths)
	if err != nil {
		return "", nil, err
	}
	return pths, stat, nil
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
			pths, stat, err := c.chkArtsPath(v.Path)
			if err != nil {
				return err
			}
			verid, err := c.egn.itr.NewArtVersionId(c.job.BuildId, v.Repository, v.Name)
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
			}
		case common.ArtsPipeline, common.ArtsPipe:
			pths, stat, err := c.chkArtsPath(v.Path)
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
			}
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
	stat, err := os.Stat(flpth)
	if err != nil {
		return err
	}
	fl, err := os.Open(flpth)
	if err != nil {
		return err
	}
	defer fl.Close()
	wt, err := c.egn.itr.UploadFile(fs, c.job.BuildId, c.job.Id, dir, pth)
	if err != nil {
		return err
	}
	defer wt.Close()

	ln := int64(0)
	bts := make([]byte, 10240)
	for {
		if hbtp.EndContext(c.cmdctx) {
			return errors.New("ctx end")
		}
		rn, err := fl.Read(bts)
		if rn > 0 {
			wn, _ := wt.Write(bts[:rn])
			ln += int64(wn)
		}
		if err != nil {
			break
		}
	}
	if ln != stat.Size() {
		return fmt.Errorf("cp file size err:%d/%d", ln, stat.Size())
	}
	return nil
}
