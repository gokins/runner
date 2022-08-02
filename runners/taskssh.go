package runners

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"golang.org/x/crypto/ssh"

	"github.com/gokins/core/utils"
	hbtp "github.com/mgr9525/HyperByte-Transfer-Protocol"
	"github.com/pkg/sftp"
	"github.com/sirupsen/logrus"
)

func (c *taskExec) connSSH() (cli *ssh.Client, rterr error) {
	defer func() {
		if err := recover(); err != nil {
			rterr = fmt.Errorf("recover:%v", err)
			logrus.Warnf("taskExec connSSH recover:%v", err)
			logrus.Warnf("Engine stack:%s", string(debug.Stack()))
		}
	}()

	cfg := &ssh.ClientConfig{
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	host := ""
	if c.job.Input != nil {
		host = c.job.Input["host"]
		cfg.User = c.job.Input["user"]
		pass := c.job.Input["pass"]
		keyfl := c.job.Input["keyFile"]
		if pass != "" {
			cfg.Auth = []ssh.AuthMethod{
				ssh.Password(pass),
			}
		} else if keyfl != "" {
			if keyfl == "user_def_file" {
				keyfl = filepath.Join(utils.HomePath(), ".ssh", "id_rsa")
			}
			keybts, err := ioutil.ReadFile(keyfl)
			if err != nil {
				return nil, err
			}
			pkey, err := ssh.ParsePrivateKey(keybts)
			if err != nil {
				return nil, err
			}
			cfg.Auth = []ssh.AuthMethod{
				ssh.PublicKeys(pkey),
			}
		}
	}
	if host == "" {
		return nil, errors.New("ssh Host is empty")
	}
	if !strings.Contains(host, ":") {
		host = host + ":22"
	}

	return ssh.Dial("tcp", host, cfg)
}

func (c *taskExec) chkArtPathSSH(stpcli *sftp.Client, pth string) (string, error) {
	pths := pth
	if !strings.HasPrefix(pth, "/") {
		pths = filepath.Join(c.job.UsersRepo, pth)
	}
	stat, err := stpcli.Stat(utils.RepSeparators(pths))
	if err == nil {
		if stat.IsDir() {
			return pths, nil
		} else {
			return pths, errors.New("artifact path is not dir")
		}
	}
	return pths, nil //stpcli.MkdirAll(pths, 0750)
}

func (c *taskExec) copyServDirSSH(stpcli *sftp.Client, fs int, pth, root2s string, rmtPrefix ...string) error {
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
	stpcli.MkdirAll(utils.RepSeparators(tpth))
	logrus.Debugf("copyServDir MkdirAll:%s", tpth)
	for _, v := range fls {
		pths := filepath.Join(pth, v.Name)
		if v.IsDir {
			err = c.copyServDirSSH(stpcli, fs, pths, root2s, rmtPrefix...)
		} else {
			err = c.cprepoflSSH(stpcli, fs, pths, root2s, rmtPrefix...)
		}
		if err != nil {
			// stpcli.RemoveDirectory(tpth)
			return err
		}
	}
	return nil
}
func (c *taskExec) cprepoflSSH(stpcli *sftp.Client, fs int, pth, root2s string, rmtPrefix ...string) error {
	var err error
	for i := 0; i < 10; i++ {
		if hbtp.EndContext(c.cmdctx) {
			err = fmt.Errorf("ctx end")
			break
		}
		err = c.cprepoFileSSH(stpcli, fs, pth, root2s, rmtPrefix...)
		if err == nil {
			break
		}
	}
	return err
}
func (c *taskExec) cprepoFileSSH(stpcli *sftp.Client, fs int, pth, root2s string, rmtPrefix ...string) error {
	rpth := pth
	if len(rmtPrefix) > 0 && rmtPrefix[0] != "" {
		rpth = filepath.Join(rmtPrefix[0], pth)
	}
	flpth := filepath.Join(root2s, pth)
	stat, err := stpcli.Stat(utils.RepSeparators(flpth))
	// stat, err := os.Stat(flpth)
	ln := int64(0)
	if err == nil {
		ln = stat.Size()
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
	flw, err := stpcli.OpenFile(utils.RepSeparators(flpth), os.O_CREATE|os.O_RDWR|os.O_TRUNC)
	// flw, err := os.OpenFile(flpth, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0640)
	if err != nil {
		return err
	}
	flw.Chmod(0640)
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

func (c *taskExec) chkArtsPathSSH(stpcli *sftp.Client, pth string) (string, os.FileInfo, error) {
	pths := filepath.Join(c.job.UsersRepo, pth)
	stat, err := stpcli.Stat(utils.RepSeparators(pths))
	if err != nil {
		return pths, nil, err
	}
	return pths, stat, nil
}

func (c *taskExec) uploaddirSSH(stpcli *sftp.Client, fs int, dir, pth, flpth string) error {
	fls, err := ioutil.ReadDir(flpth)
	if err != nil {
		return err
	}
	for _, v := range fls {
		pths := filepath.Join(pth, v.Name())
		flpths := filepath.Join(flpth, v.Name())
		if v.IsDir() {
			err = c.uploaddirSSH(stpcli, fs, dir, pths, flpths)
		} else {
			err = c.uploadflSSH(stpcli, fs, dir, pths, flpths)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
func (c *taskExec) uploadflSSH(stpcli *sftp.Client, fs int, dir, pth, flpth string) error {
	var err error
	for i := 0; i < 10; i++ {
		if hbtp.EndContext(c.cmdctx) {
			err = fmt.Errorf("ctx end")
			break
		}
		err = c.uploadFileSSH(stpcli, fs, dir, pth, flpth)
		if err == nil {
			break
		}
	}
	return err
}
func (c *taskExec) uploadFileSSH(stpcli *sftp.Client, fs int, dir, pth, flpth string) error {
	stat, err := stpcli.Stat(utils.RepSeparators(flpth))
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
	fl, err := stpcli.Open(utils.RepSeparators(flpth))
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
