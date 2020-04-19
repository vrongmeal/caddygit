package commander

import (
	"context"
	"os/exec"
)

// CommandOpts are the options to create new command.
type CommandOpts struct {
	Args  []string `json:"command,omitempty"`
	Async bool     `json:"async,omitempty"`
}

// Command is the representation of a shell command that can be run asynchronously
// or synchronously depending on the async parameter.
type Command struct {
	Name  string
	Args  []string
	Async bool
}

// NewCommand creates a command from given args.
func NewCommand(co *CommandOpts) Command {
	if len(co.Args) == 0 {
		return Command{}
	}

	if len(co.Args) == 1 {
		return Command{
			Name:  co.Args[0],
			Async: co.Async,
		}
	}

	return Command{
		Name:  co.Args[0],
		Args:  co.Args[1:],
		Async: co.Async,
	}
}

// String returns the command in a string format.
func (c *Command) String() string {
	return exec.Command(c.Name, c.Args...).String() // nolint:gosec
}

// Execute runs the command with the given context. The process is killed when the
// context is canceled.
func (c *Command) Execute(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, c.Name, c.Args...) // nolint:gosec

	if err := cmd.Start(); err != nil {
		return err
	}

	if c.Async {
		return nil
	}

	return cmd.Wait()
}
