package main

import (
	"github.com/BurntSushi/toml"
	"github.com/jmalloc/grit/src/grit"
	"github.com/jmalloc/grit/src/grit/pathutil"
	"github.com/urfave/cli"
)

func configShow(cfg grit.Config, c *cli.Context) error {
	file, err := pathutil.Resolve(c.GlobalString("config"))
	if err != nil {
		return err
	}

	write(c, "Config file: %s", file)
	write(c, "")

	return toml.NewEncoder(c.App.Writer).Encode(cfg)
}
