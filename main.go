package main

import (
	"fmt"
	"os"
	"runtime"

	cli "github.com/urfave/cli/v2"

	"github.com/timerlau/goctl-typings/action"
)

var (
	version  = "20250201"
	commands = []*cli.Command{
		{
			Name:   "typings",
			Usage:  "generates typings.d.ts",
			Action: action.Generator,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "filename",
					Usage: "save file name",
					Value: "typings.d.ts",
				},
			},
		},
	}
)

func main() {
	app := cli.NewApp()
	app.Usage = "a plugin of goctl to generate typings.d.ts"
	app.Version = fmt.Sprintf("custom %s %s/%s", version, runtime.GOOS, runtime.GOARCH)
	app.Commands = commands
	if err := app.Run(os.Args); err != nil {
		fmt.Printf("goctl-typings: %+v\n", err)
	}
}
