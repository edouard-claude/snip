package engine

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"
)

// Result holds the output of a command execution.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
}

// shellBuiltins lists commands that are shell built-ins and cannot be
// executed directly via exec.Command.
var shellBuiltins = map[string]bool{
	"export":   true,
	"unset":    true,
	"source":   true,
	"alias":    true,
	"unalias":  true,
	"eval":     true,
	"set":      true,
	"shopt":    true,
	"declare":  true,
	"local":    true,
	"readonly": true,
	"typeset":  true,
	"ulimit":   true,
	"umask":    true,
}

// makeCommand creates an exec.Cmd, wrapping shell built-ins with sh -c
// so they can be executed. Shell built-ins like "export" have no binary
// in $PATH and would fail with exec.Command directly.
func makeCommand(command string, args []string) *exec.Cmd {
	if shellBuiltins[command] {
		shPath, err := lookPath("sh")
		if err != nil {
			shPath = "/bin/sh"
		}
		shArgs := make([]string, 0, len(args)+3)
		shArgs = append(shArgs, "-c", command+` "$@"`, "_")
		shArgs = append(shArgs, args...)
		return &exec.Cmd{
			Path: shPath,
			Args: append([]string{"sh"}, shArgs...),
		}
	}
	cmdPath, err := lookPath(command)
	if err != nil {
		cmdPath = command
	}
	return &exec.Cmd{
		Path: cmdPath,
		Args: append([]string{command}, args...),
	}
}

// Execute runs a command, capturing stdout and stderr concurrently via goroutines.
func Execute(command string, args []string) (*Result, error) {
	start := time.Now()

	cmd := makeCommand(command, args)
	// Don't connect stdin for captured commands — prevents blocking on
	// commands that don't read stdin (most filtered commands).
	// Passthrough commands still get stdin via the Passthrough function.

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start command: %w", err)
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, _ = stdoutBuf.ReadFrom(stdoutPipe)
	}()
	go func() {
		defer wg.Done()
		_, _ = stderrBuf.ReadFrom(stderrPipe)
	}()

	wg.Wait()

	exitCode := 0
	err = cmd.Wait()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("wait command: %w", err)
		}
	}

	return &Result{
		Stdout:   stdoutBuf.String(),
		Stderr:   stderrBuf.String(),
		ExitCode: exitCode,
		Duration: time.Since(start),
	}, nil
}

// Passthrough runs a command with inherited stdio (no capture).
func Passthrough(command string, args []string) (int, error) {
	cmd := makeCommand(command, args)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil
		}
		return 1, fmt.Errorf("passthrough: %w", err)
	}
	return 0, nil
}
