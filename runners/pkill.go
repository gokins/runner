package runners

import (
	"github.com/sirupsen/logrus"
	"runtime/debug"
)

func (c *procExec) killCmd() {
	defer func() {
		if err := recover(); err != nil {
			logrus.Warnf("procExec killCmd recover:%v", err)
			logrus.Warnf("Engine stack:%s", string(debug.Stack()))
		}
	}()

	if c.child == nil || c.child.Process == nil {
		return
	}
	err := c.child.Process.Kill()
	if err != nil {
		logrus.Debugf("killCmd err:%v", err)
	}
}
