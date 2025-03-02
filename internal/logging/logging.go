package logging

import (
	"fmt"
	"log/slog"
	"os"
	"syscall"
)

// logNamespaceInfo logs detailed information about the current process namespaces
func LogNamespaceInfo(context string) {
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
