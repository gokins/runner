package cmd

import (
	"github.com/gokins-main/core/utils"
	hbtp "github.com/mgr9525/HyperByte-Transfer-Protocol"
	"net/url"
	"strconv"
	"time"
)

const (
	RPCHostCode = 10
)

func (c *HbtpRunner) newHbtpReq(cmds string, tmots ...time.Duration) *hbtp.Request {
	times := strconv.FormatInt(time.Now().Unix(), 10)
	random := utils.RandomString(20)
	pars := url.Values{}
	pars.Set("cmds", cmds)
	pars.Set("times", times)
	pars.Set("random", random)
	sign := utils.Md5String(cmds + random + times + c.cfg.Secret)
	pars.Set("sign", sign)
	return hbtp.NewRequest(c.cfg.Host, RPCHostCode, tmots...).Command(cmds).Args(pars)
}
func (c *HbtpRunner) doHbtpJson(method string, in, out interface{}, hd ...hbtp.Map) error {
	req := c.newHbtpReq(method)
	return hbtp.DoJson(req, in, out, hd...)
}
func (c *HbtpRunner) doHbtpString(method string, in interface{}, hd ...hbtp.Map) (int32, []byte, error) {
	req := c.newHbtpReq(method)
	return hbtp.DoString(req, in, hd...)
}
