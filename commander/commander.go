// Package commander defines commander that runs a set of commands which
// can be executed with context.
package commander

import (
	"context"
)

// Opts are the options to create a new commander.
type Opts []CommandOpts

// Commander runs the given command in order. If a command throws an error,
// it terminates the execution of further commands if `ExitOnError` is set
// true. An error func can also be provided which is run when there's an
// error in running command.
type Commander struct {
	Commands []Command

	OnError func(error)
	OnStart func(Command)

	ctx context.Context
}

// New creates a new commander from given options.
func New(ctx context.Context, opts Opts) *Commander {
	cmdr := &Commander{
		ctx: ctx,
	}

	for i := 0; i < len(opts); i++ {
		cmd := NewCommand(&opts[i])
		if cmd.Name != "" {
			cmdr.Commands = append(cmdr.Commands, cmd)
		}
	}

	return cmdr
}

// Run runs the commands.
func (c *Commander) Run() error {
	for _, cmd := range c.Commands {
		select {
		case <-c.ctx.Done():
			return c.ctx.Err()

		default:
			if cmd.Name == "" {
				continue
			}

			if c.OnStart != nil {
				c.OnStart(cmd)
			}

			if err := cmd.Execute(c.ctx); err != nil {
				if c.OnError != nil {
					c.OnError(err)
				}
			}
		}
	}

	return nil
}
