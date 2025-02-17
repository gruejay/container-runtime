# Part 1: Getting Started

## Executing Processes from Go

At its core, a container runtime is a way to use one process (the container manager) to launch
another process. That's a massive oversimplification, but launching processes is as good a place
to start as any for building `boxr`. 

Let's make a basic CLI for launching other commands through Go. There won't be any flash to it,
simply using `os/exec` package to take the user's input and pass it to the OS as a new command to run.
Using the Cobra CLI package, lets make a a command, `boxr`, and a subcommand, `run`,
that takes in the remaining CLI args and treats them as a command and arguments to run.

``` go
package main

import (
  "os"
  "os/exec"
  "fmt"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "boxr",
	Short: "Boxr is a simple container runtime",
	Long:  `A simple container runtime implementation written in Go.`,
}


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
     
    command := exec.Command(args[0], args[1:]...)
    command.Stdin = os.Stdin
    command.Stdout = os.Stdout
    command.Stderr = os.Stderr
    command.Run()
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}

func main() {

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
```


Now execute `go run step1.go run -- ls` and you should see the contents of your directory printed out. There's no file system
isolation yet, no process isolation, nothing like that. The `ls` that you are executing is the same `ls` binary that you could run directly,
and it has the same privileges on your machine as you do.

Play around with the command a bit-- even try launching a shell (zsh/bash/fish, whatever you like). Long running processes should give you enough
time to poke around at the process list while the command is still being executed to see what is happening. Tools like `pgrep` and `ps` can
be used to find the PIDs of the processes, and you can find out more by looking at `/proc/<pid>`


## Structuring the Project


To make our lives a little easier as the project expands, let's rename this file `cmd/main.go` and move the logic inside the `Run` field of `runCmd` struct into its own file inside `pkg/container/main.go`
and import it into the `cmd/main.go` file.

```go
package container

import (
  "os/exec"
  "fmt"
)

func run(args []string) {

    command := exec.Command(args[0], args[1:]...)
    command.Stdin = os.Stdin
    command.Stdout = os.Stdout
    command.Stderr = os.Stderr
    command.Run()
}
```


```go
package main

import (
  "os"
  "os/exec"
  "fmt"
  "github.com/spf13/cobra"
  "github.com/gruejay3/container-runtime/pkg/container"
)

var rootCmd = &cobra.Command{
	Use:   "boxr",
	Short: "Boxr is a simple container runtime",
	Long:  `A simple container runtime implementation written in Go.`,
}


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
    container.Run(args)
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}

func main() {

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
```


This gives us a good platform to add functionality to the `run` command. Namespaces, chroot, and cgroups can be applied
to the execution of the command, and that logic can be kept separate from the CLI logic. In the future, we will split out the 
logic even further, but the `container` package will be the "entrypoint" into running a container, meaning we can modify a lot "under the hood"
without needing to make drastic changes to the `cmd/` directory.


## Adding flags


To understand a bit more about Cobra CLI, and to add an important feature of containers, let's allow the caller to spawn
the process in "detached" mode, meaning the `boxr` command will return but the spawned process may still be running.

We can do this using a new variable `detached` that will be true if the user passes the `-d/--detached` flag.

`cmd/main.go`
```go
package main

import (
	"fmt"
	"os"

	"github.com/gruejay/container-runtime/pkg/container"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "boxr",
	Short: "Boxr is a simple container runtime",
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
		if err := container.Run(args, detach); err != nil {
			fmt.Printf("Error running container: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	runCmd.Flags().BoolVarP(&detach, "detach", "d", false, "Run container in background")
	rootCmd.AddCommand(runCmd)
}

func main() {

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
```

`pkg/container/container.go`
```go
package container

import (
  "os/exec"
  "fmt"
)

func run(args []string, detach bool) error {

  command := exec.Command(args[0], args[1:]...)
  if !detach {
    command.Stdin = os.Stdin
    command.Stdout = os.Stdout
    command.Stderr = os.Stderr
    if err := cmd.Run(); err != nil {
      return fmt.Errorf("command failed with %v", err)
    }
  } else {
    command.Start()
    pid := cmd.Process.Pid
    fmt.Printf("Spawned process: %d", pid)
    return nil
  }
  return nil
}
```

Now try running `go run cmd/main.go run -d -- sleep 100`, then afterwards `pgrep sleep` and you should get a PID matching the 
PID printed by `go run`. 

## Adding some Structure

Running a container will require knowing a lot of information:
- Namespace config (which namespaces to create, which to attach to)
- FS information (which directory is the root directory?)
- Mount information (which devices/filesystems to mount into the container, and where)
- Command information (which command to run, with what args)
- whether to attach `stdin`_`stdout`_`stderr`
- And more

It would be hard and messy to track all of that separately by passing arguments one-by-one. Each change we make
to the container (adding a feature, flag, argument) would require updating the function signatures for everything.
We can make our lives a lot easier by creating a `Container` type that will hold all of the configuration we care about.

Adding the following to `pkg/container/container.go` gives us a container type that has two fields: `Detach`, to tell us 
whether to attach to the running process, and `Args` to hold the info the user passed in about what to execute. Now, instead
of `container.Run()` being a standalone function, it will be a Method on the `Container` type. We will need to rewrite the logic
of the function to use the new container type, and rewrite `cmd/main.go` to use it, as well.

In addition to adding the type declaration, let's add a function `NewContainer()` that returns a pointer
to an instance of `Container` with some defaults. Those can, of course, be overridden, but it will be nice in the near
future when we start adding namespace configuration, but don't yet have config files.


```go
type Container struct {
	Detach     bool            `json:"detach"`
	Args       []string
}

func NewContainer() *Container {
  return &Container{
    Detach: false
  }
}

func (*Container) Run() {
	cmd := exec.Command(c.Args[0], c.Args[1:]...)

	if !c.Detach {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("command failed with %v", err)
		}
		slog.Info("started container process",
			"command", c.Args[0],
			"args", c.Args[1:],
		)
	} else {
		cmd.Start()
		pid := cmd.Process.Pid
		slog.Info("started detached container process",
			"command", c.Args[0],
			"args", c.Args[1:],
			"pid", pid,
		)
	}
	return nil

}
```

`cmd/main.go` (snippet)
```go
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
		if err := c.Run(); err != nil {
			fmt.Printf("Error running container: %v\n", err)
			os.Exit(1)
		}
	},
}
```

Now, when `boxr run` is called, first a new instance of `Container` is created, and the user's desired command
and detach behavior are set in the Container's fields. Then the `Run` method is called, which behaves pretty
much the same as before, except gets the necessary info from the `Container` instance its called on, instead
of via arguments. If we wanted to add, say, namespace changes, we could add that to the `Container` type and not
have to change the method signature. Let's do that in part 3!


## Appendix: Turning `boxr` into a real command


If you've used Go before, you know how to compile into an executable. But in case you don't:

`go build -o boxr cmd/main.go && chmod +x ./boxr` will give you a binary executable, `./boxr`, that you can now use. `./boxr run -- ls` or
`./boxr run -d -- sleep 100`
