// Package reexec provides functionality to re-execute the current binary
// with specific arguments and namespace configurations.
package reexec

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/gruejay/container-runtime/pkg/container"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Reexec executes the current binary with the given container configuration.
// It reconstructs the command with all flags and arguments from the original command.
func Reexec(c *container.Container, cmd *cobra.Command, args ...string) error {
	fmt.Println("Hello from pre-exec world!")

	// Rebuild the command with all flags and arguments
	execCmd, err := BuildCommand(cmd, args...)
	if err != nil {
		return fmt.Errorf("failed to rebuild command: %w", err)
	}

	// Configure command execution environment
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr
	execCmd.Stdin = os.Stdin
	execCmd.Env = append(os.Environ(), "_CONTAINER_INIT=1")

	// Set namespace flags for containerization
	execCmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: c.GetNamespaceFlags(),
	}

	// Execute the command
	if err := execCmd.Run(); err != nil {
		return fmt.Errorf("failed to invoke self: %w", err)
	}

	return nil
}

// BuildCommand creates an exec.Cmd that reproduces the given cobra command.
// It preserves all explicitly set flags and arguments.
func BuildCommand(cmd *cobra.Command, args ...string) (*exec.Cmd, error) {
	// Get the path to the current binary
	binary, err := os.Readlink("/proc/self/exe")
	if err != nil {
		return nil, fmt.Errorf("failed to read /proc/self/exe: %w", err)
	}

	// Build command arguments
	cmdArgs := buildCommandArgs(cmd, args)

	fmt.Printf("Reconstructed command: %s %v\n", binary, cmdArgs)

	return exec.Command(binary, cmdArgs...), nil
}

// buildCommandArgs constructs the command arguments from a cobra command.
func buildCommandArgs(cmd *cobra.Command, args []string) []string {
	var cmdArgs []string

	// Add the subcommand name if available
	if cmd.CalledAs() != "" {
		cmdArgs = append(cmdArgs, cmd.CalledAs())
	}

	// Add all flags that have been explicitly set
	cmd.Flags().Visit(func(flag *pflag.Flag) {
		// Skip help flag
		if flag.Name == "help" {
			return
		}

		cmdArgs = append(cmdArgs, formatFlag(flag))
	})

	// Add the command arguments
	return append(cmdArgs, args...)
}

// formatFlag formats a flag as a command-line argument.
func formatFlag(flag *pflag.Flag) string {
	// Handle boolean flags specially
	if flag.Value.Type() == "bool" {
		if flag.Value.String() == "true" {
			return "--" + flag.Name
		}
		return "--" + flag.Name + "=false"
	}

	// Format non-boolean flags
	return "--" + flag.Name + "=" + flag.Value.String()
}
