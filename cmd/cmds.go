package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
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
				command := exec.Command(path, "--config", c.String("config"))

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
			Usage: "stop server in deamon mode",
			Action: func(c *cli.Context) error {
				strb, err := os.ReadFile(Pid)
				if err != nil {
					log.Error("stop server failed", "err", err)
					return err
				}
				command := exec.Command("kill", string(strb))
				if err := command.Start(); err != nil {
					return err
				}
				if err := os.Remove(Pid); err != nil {
					return err
				}
				log.Info("server is stopped")

				return nil
			},
		},
	}
)
