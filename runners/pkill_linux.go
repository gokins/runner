package runners

func (c *procExec) killCmd() {
	defer func() {
		if err := recover(); err != nil {
			logrus.Warnf("procExec killCmd recover:%v", err)
			logrus.Warnf("Engine stack:%s", string(debug.Stack()))
		}
	}()

	pid := c.child.Process.Pid
	err := syscall.Kill(-pid, syscall.SIGKILL)
	if err != nil {
		logrus.Debugf("killCmd err:%v", err)
	}
}
