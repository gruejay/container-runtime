package main

import (
	"fmt"
	"os"

	"github.com/gruejay/container-runtime/pkg/container"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "boxr",
	Short: "Grocker is a simple container runtime",
	Long:  `A simple container runtime implementation written in Go.`,
}

var detach bool

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
		c.Args = args

		// Set detach mode from flag
		c.Detach = detach

		// Run the container
		if err := container.Run(*c); err != nil {
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
