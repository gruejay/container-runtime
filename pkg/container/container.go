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
			Mount:   true,
			UTS:     true,
			IPC:     false,
			User:    false, // Disabled by default as it requires additional user mapping setup
			Cgroup:  false,
		},
	}
}

// Helper method to get clone flags based on namespace configuration
func (c *Container) GetNamespaceFlags() uintptr {
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

// logNamespaceInfo logs detailed information about the current process namespaces
func logNamespaceInfo(context string) {
	pid := os.Getpid()

	// Get namespace inode numbers
	nsTypes := []struct {
		name string
		path string
	}{
		{"pid", fmt.Sprintf("/proc/%d/ns/pid", pid)},
		{"net", fmt.Sprintf("/proc/%d/ns/net", pid)},
		{"mnt", fmt.Sprintf("/proc/%d/ns/mnt", pid)},
		{"uts", fmt.Sprintf("/proc/%d/ns/uts", pid)},
		{"ipc", fmt.Sprintf("/proc/%d/ns/ipc", pid)},
		{"user", fmt.Sprintf("/proc/%d/ns/user", pid)},
		{"cgroup", fmt.Sprintf("/proc/%d/ns/cgroup", pid)},
	}

	nsInfo := make(map[string]string)
	for _, ns := range nsTypes {
		if info, err := os.Readlink(ns.path); err == nil {
			nsInfo[ns.name] = info
		} else {
			nsInfo[ns.name] = fmt.Sprintf("error: %v", err)
		}
	}

	// Get parent PID
	ppid := syscall.Getppid()

	// Log namespace information
	slog.Info("namespace information",
		"context", context,
		"pid", pid,
		"ppid", ppid,
		"pid_ns", nsInfo["pid"],
		"net_ns", nsInfo["net"],
		"mnt_ns", nsInfo["mnt"],
		"uts_ns", nsInfo["uts"],
		"ipc_ns", nsInfo["ipc"],
		"user_ns", nsInfo["user"],
		"cgroup_ns", nsInfo["cgroup"],
	)

	// Log process group information
	pgid, err := syscall.Getpgid(pid)
	if err == nil {
		slog.Info("process group information",
			"pid", pid,
			"pgid", pgid,
		)
	}

	// Log additional process information from /proc
	if cmdline, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid)); err == nil {
		slog.Info("process cmdline",
			"pid", pid,
			"cmdline", string(cmdline),
		)
	}
}

func (c *Container) Run() error {

	// Log current process information
	hostPid := os.Getpid()
	slog.Info("starting container process",
		"host_pid", hostPid,
		"command", c.Command,
		"args", c.Args,
		"detached", c.Detach,
	)

	// Create command
	cmd := exec.Command(c.Command, c.Args...)

	// Set up filesystem
	flags := uintptr(syscall.MS_PRIVATE | syscall.MS_REC)
	if err := syscall.Mount("none", "/", "", flags, ""); err != nil {
		return fmt.Errorf("failed to remount root as private: %w", err)
	}

	if err := syscall.Chroot(c.Root); err != nil {
		return fmt.Errorf("chroot failed: %w", err)
	}

	if err := os.Chdir("/"); err != nil {
		return fmt.Errorf("chdir to new root failed: %w", err)
	}
	if err := syscall.Mount("proc", "/proc", "proc", 0x0, ""); err != nil {
		return fmt.Errorf("failed to remount root as private: %w", err)
	}
	// Log namespace information before container setup
	logNamespaceInfo("before container setup")

	// Configure I/O
	if !c.Detach {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("command failed: %w", err)
		}

		slog.Info("container process completed",
			"command", c.Command,
			"args", c.Args,
		)
	} else {
		// For detached mode, redirect output to files
		// Set process group to prevent termination when parent exits
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setpgid: true,
		}

		if err := cmd.Start(); err != nil {
			return fmt.Errorf("failed to start detached process: %w", err)
		}

		pid := cmd.Process.Pid
		slog.Info("started detached container process",
			"command", c.Command,
			"args", c.Args,
			"container_pid", pid,
			"process_group", pid,
		)
	}

	return nil
}

func Stop(args []string) error {
	fmt.Println("Stopping container")
	return nil
}

func Kill(args []string) error {
	fmt.Println("Killing container")
	return nil
}
