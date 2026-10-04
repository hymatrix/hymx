package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	nodeSchema "github.com/hymatrix/hymx/node/schema"
	"github.com/hymatrix/hymx/schema"
	"github.com/urfave/cli/v2"
)

var (
	cmds = []*cli.Command{
		{
			Name:  "start",
			Usage: "run server in deamon mode",
			Flags: flags,
			Action: func(c *cli.Context) error {
				configPath := c.String("config")
				if configPath == "" {
					configPath = DefaultConfig
				}

				// if existing daemon
				if _, err := os.Stat(Pid); err == nil {
					return errors.New("daemon is already running")
				}

				// generate cmd
				path, err := os.Executable()
				if err != nil {
					return err
				}
				command := exec.Command(path, "--config", configPath, "--mode", c.String("mode"))

				// log
				logName := fmt.Sprintf("%s_%s_%d.log", schema.DataProtocol, nodeSchema.NodeVersion, time.Now().Unix())
				logFile, err := os.OpenFile(logName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
				if err != nil {
					return err
				}
				command.Stdout = logFile
				command.Stderr = logFile

				// run cmd
				if err := command.Start(); err != nil {
					return err
				}
				if err := os.WriteFile(Pid, []byte(fmt.Sprintf("%d", command.Process.Pid)), 0666); err != nil {
					return err
				}

				os.Exit(0)
				return nil
			},
		},
		{
			Name:  "stop",
			Usage: "gracefully stop server; skip VM checkpoints by default",
			Flags: []cli.Flag{
				&cli.BoolFlag{Name: "checkpoint", Usage: "save all running VM checkpoints before shutdown"},
			},
			Action: func(c *cli.Context) error {
				strb, err := os.ReadFile(Pid)
				if err != nil {
					log.Error("stop server failed", "err", err)
					return err
				}
				pid, err := strconv.Atoi(strings.TrimSpace(string(strb)))
				if err != nil || pid <= 0 {
					return fmt.Errorf("invalid PID in %s", Pid)
				}
				process, err := os.FindProcess(pid)
				if err != nil {
					return err
				}
				defer process.Release()
				sig := syscall.SIGTERM
				if c.Bool("checkpoint") {
					sig = syscall.SIGUSR1
				}
				if err := process.Signal(sig); err != nil {
					return err
				}
				log.Info("shutdown requested", "checkpoint", c.Bool("checkpoint"))

				return nil
			},
		},
	}
)
