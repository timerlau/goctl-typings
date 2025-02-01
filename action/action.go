package action

import (
	cli "github.com/urfave/cli/v2"
	"github.com/zeromicro/go-zero/tools/goctl/plugin"

	"github.com/timerlau/goctl-typings/generate"
)

// Generator 入口文件
func Generator(ctx *cli.Context) error {
	fileName := ctx.String("filename")
	if len(fileName) == 0 {
		fileName = "typings.d.ts"
	}
	p, err := plugin.NewPlugin()
	if err != nil {
		return err
	}
	return generate.Do(p, fileName)
}
