package main

import "github.com/urfave/cli/v2"

var (
	flagVersion = &cli.BoolFlag{
		Name:    "version",
		Aliases: []string{"v", "V"},
		Usage:   "print version information",
	}

	flags = []cli.Flag{
		// for config
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c", "C"},
			Usage:   "configure path",
		},
	}
)
