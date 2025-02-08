package main

import (
	"fmt"
	"os"

	"github.com/gruejay/container-runtime/pkg/container"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "grocker",
	Short: "Grocker is a simple container runtime",
	Long:  `A simple container runtime implementation written in Go.`,
}

var detach bool

var runCmd = &cobra.Command{
	Use:   "run [command]",
	Short: "Run a container",
	Long: `Run a container with the specified command.
Examples:
  grocker run /bin/bash           # Run interactively
  grocker run -d sleep 1000       # Run in background
  grocker run --detach sleep 1000 # Run in background`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := container.Run(args, detach); err != nil {
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
