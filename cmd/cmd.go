package cmd

import (
	"context"
	"errors"
	"fmt"
	"github.com/gokins/core"
	utils2 "github.com/gokins/core/utils"
	"github.com/gokins/runner/runners"
	hbtp "github.com/mgr9525/HyperByte-Transfer-Protocol"
	"github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
)

const Version = "0.1.1"

var app = kingpin.New("gokins runner", "A golang workflow application.")
var cfg = Config{}

func Run() {
	core.IsRunner = true
	regs()
	kingpin.Version(Version)
	kingpin.MustParse(app.Parse(os.Args[1:]))
}

func regs() {
	app.Flag("name", "runner name").StringVar(&cfg.Name)
	app.Flag("workdir", "runner work path").Short('w').StringVar(&cfg.WorkPath)
	app.Flag("host", "registered server host").Short('h').StringVar(&cfg.Host)
	app.Flag("secret", "registered server secret").Short('s').StringVar(&cfg.Secret)
	app.Flag("plugin", "runner supported plugins").Short('p').StringsVar(&cfg.Plugin)
	app.Flag("env", "runner default env").Short('e').StringsVar(&cfg.Env)
	app.Flag("limit", "runner task limit").Short('l').IntVar(&cfg.Limit)

	cmd := app.Command("run", "run process").Default().
		Action(run)
	cmd.Flag("debug", "debug log show").BoolVar(&core.Debug)

	cmd = app.Command("daemon", "run process background").
		Action(start)
}
func getArgs() []string {
	args := make([]string, 0)
	args = append(args, "run")
	if cfg.Name != "" {
		args = append(args, "--name")
		args = append(args, cfg.Name)
	}
	if cfg.WorkPath != "" {
		args = append(args, "--workdir")
		args = append(args, cfg.WorkPath)
	}
	if cfg.Host != "" {
		args = append(args, "--host")
		args = append(args, cfg.Host)
	}
	if cfg.Secret != "" {
		args = append(args, "--secret")
		args = append(args, cfg.Secret)
	}
	if cfg.Limit > 0 {
		args = append(args, "--limit")
		args = append(args, fmt.Sprintf("%d", cfg.Limit))
	}
	for _, v := range cfg.Plugin {
		if v != "" {
			args = append(args, "--plugin")
			args = append(args, v)
		}
	}
	for _, v := range cfg.Env {
		if v != "" {
			args = append(args, "--env")
			args = append(args, v)
		}
	}
	return args
}
func start(pc *kingpin.ParseContext) error {
	args := getArgs()
	fullpth, err := os.Executable()
	if err != nil {
		return err
	}
	println("start process")
	cmd := exec.Command(fullpth, args...)
	err = cmd.Start()
	if err != nil {
		return err
	}
	return nil
}
func run(pc *kingpin.ParseContext) error {
	var runr *runners.Engine
	csig := make(chan os.Signal, 1)
	signal.Notify(csig, os.Interrupt, syscall.SIGALRM)
	go func() {
		s := <-csig
		hbtp.Debugf("get signal(%d):%s", s, s.String())
		runr.Stop()
	}()
	if core.Debug {
		hbtp.Debug = true
	}
	if cfg.WorkPath == "" {
		pth := filepath.Join(utils2.HomePath(), ".gokins")
		cfg.WorkPath = utils2.EnvDefault("GOKINS_WORKPATH", pth)
	}
	if cfg.Host == "" {
		cfg.Host = utils2.EnvDefault("GOKINS_SERVHOST", "localhost:8031")
	}
	if cfg.Secret == "" {
		cfg.Secret = utils2.EnvDefault("GOKINS_SERVSECRET")
	}
	if len(cfg.Plugin) <= 0 {
		plgs := strings.TrimSpace(utils2.EnvDefault("GOKINS_PLUGIN"))
		if plgs != "" {
			cfg.Plugin = append(cfg.Plugin, plgs)
		}
	}
	if cfg.Host == "" {
		return errors.New("host err")
	}
	if len(cfg.Plugin) <= 0 {
		return errors.New("the runner supported plugins empty")
	}
	core.InitLog(cfg.WorkPath)
	itr := &HbtpRunner{cfg: cfg}
	info, err := itr.ServerInfo()
	if err != nil {
		logrus.Errorf("check server err.Please check your host,secret!")
		return err
	}
	logrus.Infof("check server host ok:%s", info.WebHost)
	runr = runners.NewEngine(runners.Config{
		Name:      cfg.Name,
		Workspace: cfg.WorkPath,
		Limit:     cfg.Limit,
		Plugin:    cfg.Plugin,
		Env:       cfg.Env,
	}, itr)
	return runr.Run(context.Background())
}
