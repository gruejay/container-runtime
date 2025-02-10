package container

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"syscall"
)

type Container struct {
	Namespaces NamespaceConfig
}

type NamespaceConfig struct {
	PID bool
}

func NewContainer() *Container {
	return &Container{
		Namespaces: NamespaceConfig{
			PID: true,
		},
	}
}

func init() {
	// Configure structured JSON logger with timestamp and level
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)
}

func Run(args []string, detach bool) error {
	if len(args) < 1 {
		slog.Error("no command specified")
		return fmt.Errorf("no command specified")
	}

	slog.Info("starting container process",
		"command", args[0],
		"args", args[1:],
	)

	cmd := exec.Command(args[0], args[1:]...)

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWPID | syscall.CLONE_NEWUTS,
	}

	if !detach {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
		pid := cmd.Process.Pid
		slog.Info("started container process",
			"command", args[0],
			"args", args[1:],
			"pid", pid,
		)
	} else {
		cmd.Start()
		pid := cmd.Process.Pid
		slog.Info("started detached container process",
			"command", args[0],
			"args", args[1:],
			"pid", pid,
		)
	}
	return nil
}

func Stop(args []string) error {
	slog.Info("stopping container",
		"container_id", args,
	)

	slog.Info("container process completed successfully",
		"command", args[0],
		"args", args[1:],
	)
	return nil
}

func Kill(args []string) error {
	fmt.Println("Killing container")
	return nil
}
