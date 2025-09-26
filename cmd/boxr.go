package main

import (
	"fmt"
	"os"
	"os/exec"
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
		c.Root = root
		// Set detach mode from flag
		c.Detach = detach

		// For detached mode, fork before reexec
		if c.Detach && os.Getenv("_CONTAINER_INIT") != "1" {
			// Re-run ourselves with a special detach flag
			args := os.Args[1:]
			cmd := exec.Command(os.Args[0], args...)
			cmd.Env = append(os.Environ(), "_CONTAINER_DETACH=1")
			cmd.SysProcAttr = &syscall.SysProcAttr{
				Setsid: true,
			}

			// Redirect to /dev/null for detached mode
			devNull, err := os.Open("/dev/null")
			if err != nil {
				fmt.Printf("Error opening /dev/null: %v\n", err)
				os.Exit(1)
			}
			cmd.Stdin = devNull
			cmd.Stdout = devNull
			cmd.Stderr = devNull

			if err := cmd.Start(); err != nil {
				fmt.Printf("Error starting detached container: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Started detached container (PID: %d)\n", cmd.Process.Pid)
			os.Exit(0)
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
