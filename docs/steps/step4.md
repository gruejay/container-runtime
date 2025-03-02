# Step 4: Filesystems Part 2

In the last part, we saw how we could run programs inside of our chroot environment. In
principle, that is all you need. You could build a container from "scratch" and load in
only the binaries, libraries, and data you need (or statically compile your binary, so
it doesn't require any dynamic libraries). But many workloads want or need more: they want
an entire OS without the full kernel or init system. We'll see how to make usable copies
of common linux distros later, but in this section we'll looks at how we get access to some
of the nice file systems that linux uses, like `/proc`, `/dev`, and `/sys`.

## Overview of Some Filesystems 

### `/proc`

Information in this section is largely taken from the [kernel docs](https://docs.kernel.org/filesystems/proc.html)
as well as my own poking around in the system. Like the docs say, this is incomplete and potentially less-than-
completely accurate, so do your own research etc. This will just be enough for us to do what we need, not a full
course on the kernel.

The `/proc` file system contains information about the running processes on the computer and access to change
certain things, like kernel parameters and process configuration.

Each process on the computer gets a subdirectory under `/proc` named by its PID. It also includes a special entry
`/proc/self` for the process that is reading the file system at that moment. [Table 1-1](https://docs.kernel.org/filesystems/proc.html#id9)
of the docs shows all of the files/directories that will be present inside any `/proc/<pid>` directory. As
you can see, there's a lot of stuff you can learn about the processes from this. The program that is running
is symlinked at `exe`, the commandline arguments are stored in `cmdline`, the environment variables that
the process has access to are in `environ`, etc.

The other directories in `/proc` are kernel data and configurations, such as `bootconfig` for boot parameters,
`cpuinfo`, `mounts`, etc. [Table 1-5](https://docs.kernel.org/filesystems/proc.html#id13) has the full list.
`/proc/net` has data on networking devices and `/proc/stat` has many other stats from the kernel. `/proc/sys`
allows the user to modify kernel parameters on a live system, document more in [section 2](https://docs.kernel.org/filesystems/proc.html#chapter-2-modifying-system-parameters)
of the kernel docs.

### `/sys`

Similar to `/proc`, `/sys` is a file system used to expose kernel data to user space. The
[kernel docs](https://docs.kernel.org/filesystems/sysfs.html) explain all of the details,
but the main question I wanted answered was, "How are `/sys` and `/proc/sys` different?".
From my research (Kagi search) and reading a few stackoverflow posts, the difference is larger
that `/proc/sys` has legacy system controls and tunables, while all newer drivers and systems
should expose their functionality via sysfs ([ref](https://unix.stackexchange.com/questions/4884/what-is-the-difference-between-procfs-and-sysfs).
sysfs has better structure and more rigorous use of the `kobject` struct for mapping kernel information 
to human-usable forms.

### `/dev`

Calling this `/dev` is a bit misleading. There isn't a single `devfs` implementation like there
is for `sysfs` or `procfs`. However, there are several special devices within `/dev` that are common,
and some containers will create them.

- `/dev/shm` is a tmpfs shared memory file system, used for interprocess communication via `shm_open()`
- `/dev/mqueue` is similarly meant for interprocess communication, but is a queue
- `/dev/pts` is for psuedoterminal slaves, used to implement terminal emulators or remote login via SSH.

## What's the upshot?

As we found in Part 2, even with our own PID namespace, we didn't get our own procfs. In this section, we will
mount our own procfs and see that we do in fact see only the container processes when running `ps`. Since
we still don't have a full set of system files, etc. in our container, we won't bother with pts or sys yet.

## Creating a new mount inside the container

We haven't tested our `ps axjf` command since we did the `chroot`, so let's see what we have now in our
copy of busybox:

```zsh
# sudo ./boxr run -r rootfs -- ps
PID   USER     TIME  COMMAND
```

Just an empty table. That makes sense, looking at `rootfs/proc`, its an empty directory! Let's mount a new procfs
inside the container manually.

```zsh
# sudo ./boxr run -r rootfs -- /bin/sh
```
Now inside the container:

```sh
/ # mount -t proc proc /proc
/ # ps axjf
PID   USER     TIME  COMMAND
    1 root      0:00 /bin/sh
    3 root      0:00 ps axjf
```

Success! Now, we haven't added a mount namespace yet, so we should expect that outside of the container,
we can still see the mount. Verify this by running `mount`, you should see something like:
`proc on <path/to/your/workingdir>/rootfs/proc type proc (rw,relatime)`. Unmount it with `sudo umount rootfs/proc`

To isolate mounts inside the container from the host, we should create the container with a new mount namespace. This is
easy for us with our `NamespaceConfig` type, set the `mount` field to `true`, which will set the clone flag
`CLONE_NEWNS`, which corresponds to mount namespaces.

Let's test this change by recompiling our code and redoing the previous steps. You will find that the mount is still viewable
from outside. But you can verify you are getting a new mount namespace by looking at `/proc/<pid>/ns/mnt` for the shell and
for the `boxr` command that launched the shell. Further, if we do the same test with `runc`, it also creates a new mount on 
`/proc`, but you can't see it from outside. Here we encounter one of the first major hurdles for containers: sometimes
we need to do setup for namespaces and mounts before we launch the target process. That requires entering the namespaces we
need to create (or join, if we are launching a container into an existing namespace). And that's not so simple with Go.

## Go routines, runtimes, and namespaces

### The problem(s)

A classic problem with using Go for containers is executing system calls from Go has to be done very carefully.
If you `unshare(2)` into a new namespace (or enter an existing namespace with `setns(2)`, that only applies to 
the thread that the unshare happened to be executed on, but Go can spawn and shift work onto/off of threads at 
random to satisfy the scheduler. That means work on other threads could suddenly start executing on your unshare'd
thread, and the code you thought would execute inside the namespace may suddenly move outside of it. Go's runtime 
package provides the `LockOSThread` function ([ref](https://pkg.go.dev/runtime#LockOSThread)), which includes the 
warning "A goroutine should call LockOSThread before calling OS services or non-Go library functions that depend on
per-thread state."

Unfortunately, even locking the os thread isn't a guarantee. In older versions of Go (<1.10), locked threads
could still be cloned, and the clones would inherit the parent namespace. So now work that was expected to be
in one namespace is now running in another. Weaveworks, a cloud-native gitops company, posted
a [great blog](https://web.archive.org/web/20240121103505/https://www.weave.works/blog/linux-namespaces-and-go-don-t-mix)
in 2017 that reveals the problem. Even though that's now been fixed, and locked threads can't spawn new clones,
we don't want to be forced to limit our entire program to a single thread, and if we need to enter existing namespaces
with `setns(2)`, we need multiple calls, which could (if we are unlucky) end up on separate threads.

There is another [interesting discussion](https://groups.google.com/g/golang-dev/c/6G4rq0DCKfo) in the Go-dev google group
(again, pre-1.10 release) on the problems with `setns(2)` in Go. It touches on some of the issues with multithreading and
syscalls, especially namespaces and fork/exec, some of which we don't avoid even after Go 1.10.

### The solutions

There are two common solutions to this:
1. Use a C program (or other single-threaded langauge) to set up and enter namespaces then launch Go.
2. Use a "re-exec" pattern to have the Go program re-launch itself inside of the new namespaces.

`runc` uses option 1, where they actually compile C into their Go using CGo (see [nsenter](https://github.com/opencontainers/runc/tree/main/libcontainer/nsenter)).
A far less portable solution would be using a bash script or C program to set everything up, then execute the compiled Go from there. 
This isn't great because the "single binary" becomes a binary plus a wrapper. 

`moby`, the engine behind Docker, uses [reexec.](https://github.com/moby/sys/blob/main/reexec/reexec.go) for the operations
it needs to do within other namespaces (it offloads container management itself to `runc`, but still needs to frequently
interact with containers.

CGo is fairly complicated to do well, especially given that I lack thorough C experience. So we will use reexec. Time for a rewrite...


## Rewriting for Re-exec

The idea behind re-exec is that on the first run of the program (by the user or some higher level of abstraction), a special flag
or env var is not set. This tells the program to enter the re-exec path. The re-exec path does some stuff then calls itself again
with the same args that the user passed, except this time it adds the special falg/env var to tell the program to skip re-exec.

In the re-exec code, we will read the namespace config and create the new namespaces using clone flags, very similarly to how we used
to, but this time the command to run in the new process is `boxr` itself, not the user's command. What we end up with is a copy of
`boxr` running in its own namespaces with the same parameters/config as the user originally called. This avoids all of the Go threading
issues, since we no longer have to `unshare` or create new namespaces after we've been re-executed, we just continue on with setting
up mounts, networks, and launching the user's command. I won't replicate the code for rexec in the notes, but I'll highlight a few key
parts (you can browse tag `step4-rewrite` to see a snapshot of the code post-rewrite).

We use the env var `_CONTAINER_INIT` to indicate whether we are on the first or second run of the program. SO the function executed by 
`boxr run` now looks something like:

```go
func(cmd *cobra.Command, args []string) {
    // initialize contain using `NewContainer` as before
    c := container.NewContainer()
    // Set the command and arguments
    c.Command = args[0]
    c.Args = args[1:]
    c.Root = root
    // Set detach mode from flag
    c.Detach = detach
   
    if os.Getenv("_CONTAINER_INIT") != "1" {
        // Pass the container and the cobra command
        err := reexec.Reexec(c, cmd)
        if err != nil {
            os.Exit(1)
        }
        os.Exit(0) // Reexec only re-runs the program and then should exit
    }
    // Only accessible after being re-execed and setting the env var
    // Run the container
    if err := c.Run(); err != nil {
        fmt.Printf("Error running container: %v\n", err)
        os.Exit(1)
    }
}
```


Coming back to our mount issues in particular, after we re-exec we can modify the mount paramters on `/` to set it to recursive 
private mounts, preventing the sharing issues we saw before. And since we are in our own mount namespace we won't affect anything
else in the system. Then we can proceed to `chroot` into our target directory and changedir into our root filesystem.

Note: I'm leaving out error handling, etc. here for brevity.

`pkg/container/container.go`
```go
func (c *Container) Run() error {
  ...
  flags := uintptr(syscall.MS_PRIVATE | syscall.MS_REC)
  syscall.Mount("none", "/", "", flags, "")
  syscall.Chroot(c.Root)
  os.Chdir("/")
  syscall.Mount("proc", "/proc", "proc", 0x0, "")
  ...
}
```
> Note: Our namespace info logging function, `LogNamespaceInfo`, must be called after mounting the new procfs 
> so that it has access to the proc info for the correct process. It will be PID 1, which prior to chroot and mounting
> a new procfs is the host's init process, not the container process.

Let's test:

```zsh
$ sudo ./boxr run -p
/ # 
```

Successfully got into the container, and while its running, checking `mount` on the host shows no signs of the mounts existing.

### Some caveats

In the current implementation, detached mode no longer works. If we attempt it, we see the `cmd.Start()` returns an error
as its unable to open `/dev/null`, which is the default location `exec.Command` uses for stdin, stdout, and stderr. That makes
sense, as we haven't created that device in our container. We'll get to that later.
