//go:build ignore

package backup

import (
	"context"
	"io"
	"os"
	"os/exec"
)

type CommandExecutor interface {
	LookPath(file string) (string, error)
	Run(ctx context.Context, name string, args []string, env map[string]string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error
}

type OSCommandExecutor struct{}

func NewOSCommandExecutor() *OSCommandExecutor {
	return &OSCommandExecutor{}
}

func (e *OSCommandExecutor) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

func (e *OSCommandExecutor) Run(ctx context.Context, name string, args []string, env map[string]string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	command := exec.CommandContext(ctx, name, args...)
	command.Stdin = stdin
	command.Stdout = stdout
	command.Stderr = stderr
	command.Env = os.Environ()
	for key, value := range env {
		command.Env = append(command.Env, key+"="+value)
	}
	return command.Run()
}
