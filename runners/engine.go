package runners

import (
	"context"
	"errors"
	"github.com/gokins-main/core/common"
	"github.com/gokins-main/core/utils"
	hbtp "github.com/mgr9525/HyperByte-Transfer-Protocol"
	"github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"time"
)

type Engine struct {
	ctx  context.Context
	cncl context.CancelFunc
	cfg  Config
	itr  IExecute

	sysEnv utils.EnvVal
	linelk sync.RWMutex
	lines  map[string]*taskExec
}

func NewEngine(cfg Config, itr IExecute) *Engine {
	return &Engine{
		cfg:   cfg,
		itr:   itr,
		lines: make(map[string]*taskExec),
	}
}
func (c *Engine) Start(ctx context.Context) error {
	if c.itr == nil {
		return errors.New("execute is nil")
	}
	/*if c.cfg.Name == "" {
		return errors.New("config server name is empty.")
	}
	if len(c.cfg.Name) > 20 {
		return errors.New("config server name is too long than 20.")
	}*/
	if c.cfg.Workspace == "" {
		return errors.New("config workspace is empty.")
	}
	if c.cfg.Limit <= 0 {
		c.cfg.Limit = 50
	}
	if len(c.cfg.Plugin) <= 0 {
		return errors.New("plugins is empty(please see --help)")
	}
	os.MkdirAll(filepath.Join(c.cfg.Workspace, common.PathBuild), 0755)
	if ctx == nil {
		ctx = context.Background()
	}
	c.ctx, c.cncl = context.WithCancel(ctx)
	go func() {
		c.sysEnv = utils.AllEnv()
		for !c.Stopd() {
			c.run()
			time.Sleep(time.Millisecond * 100)
		}
	}()
	return nil
}
func (c *Engine) Stopd() bool {
	return hbtp.EndContext(c.ctx)
}
func (c *Engine) Stop() {
	if c.cncl != nil {
		c.cncl()
	}

	c.linelk.RLock()
	defer c.linelk.RUnlock()
	for _, v := range c.lines {
		v.stop()
	}
}
func (c *Engine) run() {
	defer func() {
		if err := recover(); err != nil {
			logrus.Warnf("Engine run recover:%v", err)
			logrus.Warnf("Engine stack:%s", string(debug.Stack()))
		}
	}()

	c.linelk.RLock()
	ln := len(c.lines)
	c.linelk.RUnlock()
	if c.cfg.Limit > 0 && ln >= c.cfg.Limit {
		logrus.Debugf("job list out limit:%d", ln)
		return
	}

	job, err := c.itr.PullJob(c.cfg.Plugin)
	if err != nil {
		//logrus.Debugf("not pull job:%v", err)
		return
	}

	c.linelk.Lock()
	defer c.linelk.Unlock()
	e := &taskExec{
		prt: c,
		job: job,
	}
	c.lines[job.Id] = e
	go c.startTask(e)
}
func (c *Engine) startTask(tsk *taskExec) {
	defer func() {
		if err := recover(); err != nil {
			logrus.Warnf("Engine startTask recover:%v", err)
			logrus.Warnf("Engine stack:%s", string(debug.Stack()))
		}
	}()

	tsk.run()
	c.linelk.Lock()
	defer c.linelk.Unlock()
	tsk.RLock()
	id := tsk.job.Id
	tsk.RUnlock()
	delete(c.lines, id)
}
