# Step 2: Namespaces (Part 1)


Namespaces are a major piece of the container puzzle. I won't go into detail here on 
exactly what they are and how they work (because I am _not_ qualified for that. We'll focus
on how to use them in Go, how to see the effects for yourself, and what some of the challenges
are that we will need to fix in the future.


## Extending the Container type

To allow us to configure namespaces for the executed command, let's extend the `Container` type
to include a namespace configuration. First, lets create a new type that is `NamespaceConfiguration`. We
will include fields for all 7 current Linux Namespaces, and currently we will treat them all as booleans:
`true` if we should create a new namespace of that type for the container, `false` if not (it will inherit 
based on that namepsace type's inheritance rules).

`pkg/container/container.go`
```go
type NamespaceConfig struct {
	PID     bool `json:"pid"`     // Process ID namespace
	Network bool `json:"network"` // Network namespace
	Mount   bool `json:"mount"`   // Mount namespace
	UTS     bool `json:"uts"`     // Unix Timesharing System namespace
	IPC     bool `json:"ipc"`     // Inter-Process Communication namespace
	User    bool `json:"user"`    // User namespace
	Cgroup  bool `json:"cgroup"`  // Control Group namespace
}
```

And let's add a field for that to the main `Container` type

```go
type Container struct {
	Namespaces NamespaceConfig `json:"namespaces"`
	Detach     bool            `json:"detach"`
	Args       []string
}
```

Right now, we will use a hard-coded default Namespace configuration, set via the `NewContainer` function.
In future steps, we will create a configuration format that will allow users to specify which namespaces to 
create, or specify the ID of a namespace to join. This would allow, for example, launching 2 containers in
the same network namespace.

```go
func NewContainer() *Container {
	return &Container{
    Detach: false,
		Namespaces: NamespaceConfig{
			PID:     true,
			Network: false,
			Mount:   false,
			UTS:     true,
			IPC:     false,
      User:    false,
			Cgroup:  false,
		},
	}
}
```


## Syscalls and Clone Flags


Namespaces can be interacted with via some linux commands, like `unshare(1)` and `nsenter(1)`. New namespaces
can also be managed via flags set on the fork/exec syscalls. In Go, these flags are set in the `exec.Command.SysProcAttr` field, and
take the form of bit mask number. The available flags are defined in linux (see the manual for `clone(2)`)
and in Go, the [syscall package](https://pkg.go.dev/syscall#pkg-constants) defines them as constants.

So, all we need to do is turn our `NamespaceConfig` struct into a bitmask matching the enabled namespaces.

`pkg/container/container.go`
```go
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

  retu
  rn flags
}
```

We can use that helper function to make it easier to set the `SysProcAttr` when creating the `exec.Command`:

`pkg/container/container.go`
```go
func (c *Container) Run() error {

	cmd := exec.Command(c.Args[0], c.Args[1:]...)

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: c.getNamespaceFlags(),
  }
  ...
}
```


The rest of the `Run()` method doesn't need to change, the `cmd.Run()` and `cmd.Start()` calls use the
`cmd.SysProcAttr` to pass the correct flags to the syscalls underneath.


## Testing Our Changes


Now when we go `go run cmd/main.go run -- zsh`, then shell should run in its own PID and UTS namespaces,
meaning it should be PID 1 (from it's perspective) and we should be able to set the hostname inside the process without affecting
the host. Let's try it:

```bash
$ ./boxr run -- zsh
Error running container: command failed with fork/exec /usr/bin/zsh: operation not permitted
exit status 1
```

Huh...

Of course, to create new namespaces you need to be root, and since we've been running as our user so far, the `exec.Command` is
failing to run the command. We need to run as root:

```bash
$ sudo ./boxr run -- zsh
```

Okay-- now we have a shell. Let's check the current PID from inside the shell with `echo $$`

```zsh
# echo $$
1
```

Now let's check `ps`:

```zsh
# ps axjf
...
8816   47504   47504   47504 pts/2      49674 Ss    1000   0:06  \_ -zsh
47504   49674   49674   47504 pts/2      49674 Sl+   1000   0:00      \_ just run zsh
49674   49692   49674   47504 pts/2      49674 S+       0   0:00          \_ sudo ./boxr run -- zsh
49692   49693   49693   49693 pts/4      49748 Ss       0   0:00              \_ sudo ./boxr run -- zsh
49693   49694   49694   49693 pts/4      49748 Sl       0   0:00                  \_ ./boxr run -- zsh
49694   49698   49698   49693 pts/4      49748 S        0   0:00                      \_ zsh
49698   49748   49748   49693 pts/4      49748 R+       0   0:00                          \_ ps axjf
...
```

Huh... the shell thinks its current PID is 1, but `ps axjf` shows something entirely different. This is due to how `ps`
works. It reads from the `/proc` filesystem to get information about all of the processes on the host. From the
_hosts_ perspective, my "container" shell is just another process, spawned by the `boxr` executable. Since the "container"
shell was spawned without masking the host's `/proc` filesystem, `ps` inside the shell reads the same information as `ps`
outside, and thus "sees" the PID of the container process from the host's perspective. `$$`, on the other hand, is a builtin
that returns the process' own view of its PID, which, since we cloned into a new PID namespace, is 1. 

So that's PID namespace tested. It worked, but not without some quirks. Even PIDs aren't so simple, isolating PIDs completely
requires a bit of filesystem trickier to work as you'd expect. Now we can test the UTS namespace. For our purposes, this
is just the hostname. My host is named `cri-dev`, let's check outside and inside the container shell:

Outside:
```bash
 $ hostname
 cri-dev
``` 
```zsh
 # hostname
 cri-dev
```  

Okay, so the container has the same hostname. What if we change it:

Inside:
```zsh
 # hostname im-inside
```

Outside:
```bash
 $ hostname
 cri-dev
``` 

```zsh
 # hostname
  im-inside
```  

Nice! So we know changing the hostname inside the container doesn't affect the hostname outside. But when we launched the container,
it had the hostname of the host itself, which isn't ideal. We'd like to set our own hostname. We'll tuck that away for the future.
