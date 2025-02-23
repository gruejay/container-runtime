package container

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"syscall"
)

type Container struct {
	Namespaces NamespaceConfig `json:"namespaces"`
	Detach     bool            `json:"detach"`
	Command    string          `json:"command"`
	Args       []string        `json:"args"`
	Root       string          `json:"root"`
}

type NamespaceConfig struct {
	PID     bool `json:"pid"`     // Process ID namespace
	Network bool `json:"network"` // Network namespace
	Mount   bool `json:"mount"`   // Mount namespace
	UTS     bool `json:"uts"`     // Unix Timesharing System namespace
	IPC     bool `json:"ipc"`     // Inter-Process Communication namespace
	User    bool `json:"user"`    // User namespace
	Cgroup  bool `json:"cgroup"`  // Control Group namespace
}

func NewContainer() *Container {
	return &Container{
		Namespaces: NamespaceConfig{
			PID:     true,
			Network: false,
			Mount:   false,
			UTS:     true,
			IPC:     false,
			User:    false, // Disabled by default as it requires additional user mapping setup
			Cgroup:  false,
		},
	}
}

// Helper method to get clone flags based on namespace configuration
func (c *Container) getNamespaceFlags() uintptr {
	var flags uintptr

	if c.Namespaces.PID {
		flags |= syscall.CLONE_NEWPID
	}
	if c.Namespaces.Network {
		flags |= syscall.CLONE_NEWNET
	}
	if c.Namespaces.Mount {
		flags |= syscall.CLONE_NEWNS
	}
	if c.Namespaces.UTS {
		flags |= syscall.CLONE_NEWUTS
	}
	if c.Namespaces.IPC {
		flags |= syscall.CLONE_NEWIPC
	}
	if c.Namespaces.User {
		flags |= syscall.CLONE_NEWUSER
	}
	if c.Namespaces.Cgroup {
		flags |= syscall.CLONE_NEWCGROUP
	}

	return flags
}

func init() {
	// Configure structured JSON logger with timestamp and level
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)
}

func (c *Container) Run() error {

	slog.Info("starting container process",
		"command", c.Command,
		"args", c.Args,
	)
	fmt.Println(c.getNamespaceFlags())
	cmd := exec.Command(c.Command, c.Args...)

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: c.getNamespaceFlags(),
	}

	syscall.Chroot(c.Root)
	os.Chdir("/")

	if !c.Detach {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("command failed with %v", err)
		}
		slog.Info("started container process",
			"command", c.Command,
			"args", c.Args,
		)
	} else {
		cmd.Start()
		pid := cmd.Process.Pid
		slog.Info("started detached container process",
			"command", c.Command,
			"args", c.Args,
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
