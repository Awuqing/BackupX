package backup

import (
	"context"
	"io"
	"os"
	"os/exec"
)

type CommandOptions struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	Env    []string
}

type CommandExecutor interface {
	LookPath(file string) (string, error)
	Run(ctx context.Context, name string, args []string, options CommandOptions) error
}

type OSCommandExecutor struct{}

func NewOSCommandExecutor() *OSCommandExecutor {
	return &OSCommandExecutor{}
}

func (e *OSCommandExecutor) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

func (e *OSCommandExecutor) Run(ctx context.Context, name string, args []string, options CommandOptions) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = options.Stdin
	cmd.Stdout = options.Stdout
	cmd.Stderr = options.Stderr
	if len(options.Env) > 0 {
		cmd.Env = append(os.Environ(), options.Env...)
	}
	return cmd.Run()
}
