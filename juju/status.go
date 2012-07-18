package main

import (
	"launchpad.net/gnuflag"
	"launchpad.net/juju-core/cmd"
	"launchpad.net/juju-core/juju"
)

type StatusCommand struct {
	EnvName string
	Out	cmd.Output
}

var statusDoc = "This command will report on the runtime state of various system entities."

func (c *StatusCommand) Info() *cmd.Info {
	return &cmd.Info{
		"status", "", "Output status information about an environment.", statusDoc,
	}
}

func (c *StatusCommand) Init(f *gnuflag.FlagSet, args []string) error {
	addEnvironFlags(&c.EnvName, f)
	c.Out.AddFlags(f, "yaml", cmd.DefaultFormatters)
	if err := f.Parse(true, args); err != nil {
		return err
	}
	return cmd.CheckEmpty(f.Args())
}

func (c *StatusCommand) Run(_ *cmd.Context) error {
	conn, err := juju.NewConn(c.EnvName)
	if err != nil {
		return err
	}
	defer conn.Close()
	return nil
}
