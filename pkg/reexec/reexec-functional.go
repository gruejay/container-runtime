//go:build exclude
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

// Reexec executes the current binary with the given container configuration
func ReexecFunctional(c *container.Container, oldCommand *cobra.Command, args ...string) error {
	fmt.Println("Hello from pre-exec world!")

	// Pipeline of transformations
	return getBinaryPath().
		flatMap(func(binary string) Result[*exec.Cmd] {
			return buildCommand(binary, oldCommand, args)
		}).
		map(configureCommand(c)).
		flatMap(runCommand).
		unwrapOrElse(func(err error) error {
			return fmt.Errorf("reexec failed: %w", err)
		})
}

// GetCommandFunctional builds a command from a cobra command and arguments
func GetCommandFunctional(command *cobra.Command, args ...string) (*exec.Cmd, error) {
	return getBinaryPath().
		flatMap(func(binary string) Result[*exec.Cmd] {
			return buildCommand(binary, command, args)
		}).
		unwrap()
}

// Pure functions for command building

// getBinaryPath returns the path to the current binary
func getBinaryPath() Result[string] {
	binary, err := os.Readlink("/proc/self/exe")
	if err != nil {
		return Failure[string](fmt.Errorf("failed to read /proc/self/exe: %w", err))
	}
	return Success(binary)
}

// buildCommand constructs a command with all flags and arguments
func buildCommand(binary string, command *cobra.Command, args []string) Result[*exec.Cmd] {
	cmdArgs := []string{}

	// Add the command name if available
	if command.CalledAs() != "" {
		cmdArgs = append(cmdArgs, command.CalledAs())
	}

	// Add flags
	cmdArgs = append(cmdArgs, getFlagArgs(command)...)
	
	// Add command arguments
	cmdArgs = append(cmdArgs, args...)

	fmt.Printf("Reconstructed command: %s %v\n", binary, cmdArgs)
	return Success(exec.Command(binary, cmdArgs...))
}

// getFlagArgs extracts flag arguments from a command
func getFlagArgs(command *cobra.Command) []string {
	var flagArgs []string
	
	command.Flags().Visit(func(flag *pflag.Flag) {
		if flag.Name == "help" {
			return
		}
		
		flagArgs = append(flagArgs, formatFlag(flag))
	})
	
	return flagArgs
}

// formatFlag formats a flag as a command-line argument
func formatFlag(flag *pflag.Flag) string {
	if flag.Value.Type() == "bool" {
		if flag.Value.String() == "true" {
			return "--" + flag.Name
		}
		return "--" + flag.Name + "=false"
	}
	return "--" + flag.Name + "=" + flag.Value.String()
}

// configureCommand sets up the command with proper I/O and namespace configuration
func configureCommand(c *container.Container) func(*exec.Cmd) *exec.Cmd {
	return func(cmd *exec.Cmd) *exec.Cmd {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		cmd.Env = append(os.Environ(), "_CONTAINER_INIT=1")
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Cloneflags: c.GetNamespaceFlags(),
		}
		return cmd
	}
}

// runCommand executes the command
func runCommand(cmd *exec.Cmd) Result[struct{}] {
	if err := cmd.Run(); err != nil {
		return Failure[struct{}](fmt.Errorf("failed to invoke self: %w", err))
	}
	return Success(struct{}{})
}

// Result type for functional error handling

// Result represents either a success value or an error
type Result[T any] struct {
	value T
	err   error
}

// Success creates a successful result
func Success[T any](value T) Result[T] {
	return Result[T]{value: value, err: nil}
}

// Failure creates a failed result
func Failure[T any](err error) Result[T] {
	var zero T
	return Result[T]{value: zero, err: err}
}

// map applies a function to the value if successful
func (r Result[T]) map[U any](f func(T) U) Result[U] {
	if r.err != nil {
		return Failure[U](r.err)
	}
	return Success(f(r.value))
}

// flatMap applies a function that returns a Result
func (r Result[T]) flatMap[U any](f func(T) Result[U]) Result[U] {
	if r.err != nil {
		return Failure[U](r.err)
	}
	return f(r.value)
}

// unwrap returns the value and error
func (r Result[T]) unwrap() (T, error) {
	return r.value, r.err
}

// unwrapOrElse returns the value or applies the function to the error
func (r Result[T]) unwrapOrElse(f func(error) error) error {
	if r.err != nil {
		return f(r.err)
	}
	return nil
}

