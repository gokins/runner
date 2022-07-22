package cmd

import (
	"fmt"
	"io"
	"strconv"

	"github.com/gokins/core/utils"
	"github.com/gokins/runner/runners"
	hbtp "github.com/mgr9525/HyperByte-Transfer-Protocol"
)

type HbtpRunner struct {
	cfg Config
}

func (c *HbtpRunner) ServerInfo() (*runners.ServerInfo, error) {
	info := &runners.ServerInfo{}
	err := c.doHbtpJson("ServerInfo", nil, info)
	if err != nil {
		return nil, err
	}
	return info, nil
}
func (c *HbtpRunner) PullJob(name string, plugs []string) (*runners.RunJob, error) {
	rt := &runners.RunJob{}
	err := c.doHbtpJson("PullJob", &runners.ReqPullJob{
		Name:  name,
		Plugs: plugs,
	}, rt)
	return rt, err
}
func (c *HbtpRunner) CheckCancel(buildId string) bool {
	code, bts, err := c.doHbtpString("CheckCancel", nil, hbtp.Map{
		"buildId": buildId,
	})
	rts := string(bts)
	if err != nil || code != hbtp.ResStatusOk {
		return false
	}
	return rts == "true"
}
func (c *HbtpRunner) Update(m *runners.UpdateJobInfo) error {
	code, bts, err := c.doHbtpString("Update", m)
	if err != nil {
		return err
	}
	if code != hbtp.ResStatusOk {
		return fmt.Errorf("%s", string(bts))
	}
	return nil
}
func (c *HbtpRunner) UpdateCmd(buildId, jobId, cmdId string, fs, codes int) error {
	code, bts, err := c.doHbtpString("UpdateCmd", nil, hbtp.Map{
		"buildId": buildId,
		"jobId":   jobId,
		"cmdId":   cmdId,
		"fs":      fs,
		"code":    codes,
	})
	if err != nil {
		return err
	}
	if code != hbtp.ResStatusOk {
		return fmt.Errorf("%s", string(bts))
	}
	return nil
}
func (c *HbtpRunner) PushOutLine(buildId, jobId, cmdId, bs string, iserr bool) error {
	code, bts, err := c.doHbtpString("PushOutLine", nil, hbtp.Map{
		"buildId": buildId,
		"jobId":   jobId,
		"cmdId":   cmdId,
		"bs":      bs,
		"iserr":   iserr,
	})
	if err != nil {
		return err
	}
	if code != hbtp.ResStatusOk {
		return fmt.Errorf("%s", string(bts))
	}
	return nil
}
func (c *HbtpRunner) FindJobId(buildId, stgNm, stpNm string) (string, bool) {
	code, bts, err := c.doHbtpString("FindJobId", nil, hbtp.Map{
		"buildId": buildId,
	})
	rts := string(bts)
	if err != nil {
		return "", false
	}
	if code != hbtp.ResStatusOk {
		return "", false
	}
	return rts, true
}
func (c *HbtpRunner) ReadDir(fs int, buildId string, pth string) ([]*runners.DirEntry, error) {
	var rts []*runners.DirEntry
	err := c.doHbtpJson("ReadDir", nil, &rts, hbtp.Map{
		"buildId": buildId,
		"pth":     pth,
		"fs":      fs,
	})
	if err != nil {
		return nil, err
	}
	return rts, nil
}
func (c *HbtpRunner) ReadFile(fs int, buildId string, pth string, start int64) (int64, io.ReadCloser, error) {
	req := c.newHbtpReq("ReadFile")
	req.Header().Set("buildId", buildId)
	req.Header().Set("pth", pth)
	req.Header().Set("fs", fs)
	req.Header().Set("start", start)
	res, err := req.Do(nil, nil)
	if err != nil {
		return 0, nil, err
	}
	defer res.Close()
	rs := string(res.BodyBytes())
	if res.Code() != hbtp.ResStatusOk {
		return 0, nil, fmt.Errorf("%s", rs)
	}
	sz, err := strconv.ParseInt(rs, 10, 64)
	if err != nil {
		return 0, nil, err
	}
	return sz, res.Conn(true), nil
}
func (c *HbtpRunner) GetEnv(buildId, jobId, key string) (string, bool) {
	code, bts, err := c.doHbtpString("GetEnv", nil, hbtp.Map{
		"buildId": buildId,
		"jobId":   jobId,
		"key":     key,
	})
	rts := string(bts)
	if err != nil {
		return "", false
	}
	if code != hbtp.ResStatusOk {
		return "", false
	}
	return rts, true
}
func (c *HbtpRunner) GenEnv(buildId, jobId string, env utils.EnvVal) error {
	code, bts, err := c.doHbtpString("GenEnv", env, hbtp.Map{
		"buildId": buildId,
		"jobId":   jobId,
	})
	if err != nil {
		return err
	}
	if code != hbtp.ResStatusOk {
		return fmt.Errorf("%s", string(bts))
	}
	return nil
}
func (c *HbtpRunner) StatFile(fs int, buildId, jobId string, dir, pth string) (*runners.FileStat, error) {
	req := c.newHbtpReq("StatFile")
	req.Header().Set("buildId", buildId)
	req.Header().Set("jobId", jobId)
	req.Header().Set("dir", dir)
	req.Header().Set("pth", pth)
	req.Header().Set("fs", fs)
	res, err := req.Do(nil, nil)
	if err != nil {
		return nil, err
	}
	defer res.Close()
	rs := string(res.BodyBytes())
	if res.Code() != hbtp.ResStatusOk {
		return nil, fmt.Errorf("%s", rs)
	}
	stat := &runners.FileStat{}
	err = res.BodyJson(stat)
	return stat, err
}
func (c *HbtpRunner) UploadFile(fs int, buildId, jobId string, dir, pth string, start int64) (io.WriteCloser, error) {
	req := c.newHbtpReq("UploadFile")
	req.Header().Set("buildId", buildId)
	req.Header().Set("jobId", jobId)
	req.Header().Set("dir", dir)
	req.Header().Set("pth", pth)
	req.Header().Set("fs", fs)
	req.Header().Set("start", start)
	res, err := req.Do(nil, nil)
	if err != nil {
		return nil, err
	}
	defer res.Close()
	rs := string(res.BodyBytes())
	if res.Code() != hbtp.ResStatusOk {
		return nil, fmt.Errorf("%s", rs)
	}
	return res.Conn(true), nil
}
func (c *HbtpRunner) FindArtVersionId(buildId, idnt string, name string) (string, error) {
	code, bts, err := c.doHbtpString("FindArtVersionId", nil, hbtp.Map{
		"buildId": buildId,
		"idnt":    idnt,
		"name":    name,
	})
	rts := string(bts)
	if err != nil {
		return "", err
	}
	if code != hbtp.ResStatusOk {
		return "", fmt.Errorf("%s", string(bts))
	}
	return rts, nil
}
func (c *HbtpRunner) NewArtVersionId(buildId, idnt string, name string) (string, error) {
	code, bts, err := c.doHbtpString("NewArtVersionId", nil, hbtp.Map{
		"buildId": buildId,
		"idnt":    idnt,
		"name":    name,
	})
	rts := string(bts)
	if err != nil {
		return "", err
	}
	if code != hbtp.ResStatusOk {
		return "", fmt.Errorf("%s", string(bts))
	}
	return rts, nil
}
