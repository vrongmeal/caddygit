package client

import (
	"context"
	"os/exec"
	"syscall"
)

// Commander runs the given command in order. If a command throws an error,
// it terminates the execution of further commands if `ExitOnError` is set
// true. An error func can also be provided which is run when there's an
// error in running command.
type Commander struct {
	Commands []Command

	OnError func(error)
	OnStart func(Command)
}

// Run runs the commands.
func (c *Commander) Run(ctx context.Context) error {
	for _, cmd := range c.Commands {
		if cmd.String() == "" {
			continue
		}

		if c.OnStart != nil {
			c.OnStart(cmd)
		}

		if err := cmd.Execute(ctx); err != nil {
			if c.OnError != nil {
				c.OnError(err)
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()

		default:
			continue
		}
	}

	return nil
}

// Command is the representation of a shell command that can be run asynchronously
// or synchronously depending on the async parameter.
type Command struct {
	Args  []string `json:"command,omitempty"`
	Async bool     `json:"async,omitempty"`
}

func (c *Command) cmd() *exec.Cmd {
	var name string
	var args []string

	if len(c.Args) == 0 {
		return nil
	}

	name = c.Args[0]
	if len(c.Args) > 1 {
		args = c.Args[1:]
	}

	command := exec.Command(name, args...) // nolint:gosec
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	return command
}

// String returns the command in a string format.
func (c *Command) String() string {
	return c.cmd().String()
}

// Execute runs the command with the given context. The process is killed when the
// context is canceled.
func (c *Command) Execute(ctx context.Context) error {
	stream := make(chan error)

	cmd := c.cmd()

	if err := cmd.Start(); err != nil {
		return err
	}

	if c.Async {
		// exit if the process is run asynchronously
		return nil
	}

	go func(ex *exec.Cmd, err chan<- error) {
		err <- ex.Wait()
	}(cmd, stream)

	select {
	case <-ctx.Done():
		// Elegantly close the parent along-with the children.
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if err != nil {
			return err
		}

		return ctx.Err()

	case err := <-stream:
		return err
	}
}
