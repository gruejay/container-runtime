package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/gruejay/container-runtime/pkg/container"
	"github.com/gruejay/container-runtime/pkg/reexec"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "boxr",
	Short: "Boxr is a simple container runtime",
	Long:  `A simple container runtime implementation written in Go.`,
}

var detach bool
var root string

var runCmd = &cobra.Command{
	Use:   "run [command]",
	Short: "Run a container",
	Long: `Run a container with the specified command.
Examples:
  boxr run /bin/bash           # Run interactively
  boxr run -d sleep 1000       # Run in background
  boxr run --detach sleep 1000 # Run in background`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Initialize a new container with default settings
		c := container.NewContainer()
		// Set the command and arguments
		c.Command = args[0]
		c.Args = args[1:]
		// Convert root to absolute path
		absRoot, err := filepath.Abs(root)
		if err != nil {
			fmt.Printf("Error getting absolute path: %v\n", err)
			os.Exit(1)
		}
		c.Root = absRoot
		// Set detach mode from flag
		c.Detach = detach

		// For detached mode, fork before reexec
		if c.Detach && os.Getenv("_CONTAINER_INIT") != "1" && os.Getenv("_CONTAINER_DETACH") != "1" {
			// First fork - create a new process
			// Reconstruct args with absolute path
			args := []string{"run", "--detach", "--root", c.Root, c.Command}
			args = append(args, c.Args...)
			cmd := exec.Command(os.Args[0], args...)
			cmd.Env = append(os.Environ(), "_CONTAINER_DETACH=1")
			cmd.SysProcAttr = &syscall.SysProcAttr{
				Setsid: true, // Create new session
			}

			// Start the first fork
			if err := cmd.Start(); err != nil {
				fmt.Printf("Error starting detached container: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Started detached container (PID: %d)\n", cmd.Process.Pid)
			fmt.Printf("Logs will be written to: /var/log/boxr/container-%d.log\n", cmd.Process.Pid)
			os.Exit(0)
		}

		// Second fork - this runs in the first forked process
		if os.Getenv("_CONTAINER_DETACH") == "1" && os.Getenv("_CONTAINER_INIT") != "1" {
			// Pass log file path to the final process
			logDir := "/var/log/boxr"
			os.MkdirAll(logDir, 0755)
			logFile := fmt.Sprintf("%s/container-%d.log", logDir, os.Getpid())

			// Just do a simple exec without double fork
			// The process is already detached from the first fork
			// Change directory to /
			if err := os.Chdir("/"); err != nil {
				os.Exit(1)
			}

			// Clear umask
			syscall.Umask(0)

			// Set up environment with log file path
			os.Setenv("_CONTAINER_LOG", logFile)

			// Continue with normal flow - the logging will be set up after reexec
		}

		// Normal reexec flow
		if os.Getenv("_CONTAINER_INIT") != "1" {
			err := reexec.Reexec(c, cmd)
			if err != nil {
				os.Exit(1)
			}
			os.Exit(0)
		}

		// Run the container (this will exec into the user command)
		// For detached mode, redirect output to log file
		if logFile := os.Getenv("_CONTAINER_LOG"); logFile != "" {
			// Open log file for output
			logFd, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err == nil {
				// Redirect stdout and stderr to log file
				os.Stdout = logFd
				os.Stderr = logFd
				// Also redirect for child processes
				syscall.Dup2(int(logFd.Fd()), 1)
				syscall.Dup2(int(logFd.Fd()), 2)
			}

			// Redirect stdin to /dev/null
			devNull, err := os.Open("/dev/null")
			if err == nil {
				os.Stdin = devNull
				syscall.Dup2(int(devNull.Fd()), 0)
			}
		}
		if err := c.Run(); err != nil {
			fmt.Printf("Error running container: %v\n", err)
			os.Exit(1)
		}
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop a running container",
	Run: func(cmd *cobra.Command, args []string) {
		if err := container.Stop(args); err != nil {
			fmt.Printf("Error stopping container: %v\n", err)
			os.Exit(1)
		}
	},
}

var killCmd = &cobra.Command{
	Use:   "kill",
	Short: "Kill a running container",
	Run: func(cmd *cobra.Command, args []string) {
		if err := container.Kill(args); err != nil {
			fmt.Printf("Error killing container: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	runCmd.Flags().BoolVarP(&detach, "detach", "d", false, "Run container in background")
	runCmd.Flags().StringVarP(&root, "root", "r", "rootfs", "Root directory of container")
	runCmd.MarkFlagRequired("root")
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(killCmd)
}

func main() {

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
