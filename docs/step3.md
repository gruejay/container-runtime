# Step 3: Filesystems (Part 1) 

As we saw last time, even though we puanched our process in a new `PID` namespace, `ps`
still showed other processes on the host. This was because `ps` works by reading from
`/proc`, which is the same "on the host" and "in the container". In this installment,
we will start our journey of learning how to get filesystem isolation inside the container.

## `chroot`

We need to start by sectioning off the "container" into its own directory on the host filesystem.
But since we want to isolate the process, that directory should look like root `/` to the process in
the container. We can do that using the `chroot` syscall/command. We can test in the CLI:

> Note: I've included a copy of `stage3-chroot` in the repo. This was generated on an m4 Macbook,
> you're system may be different. Remove my copy and follow along if you want to test this yourself.

```bash
$ mkdir stage3-chroot
$ sudo chroot stage3-chroot ls
chroot: failed to run command ‘ls’: No such file or directory
```

huh....let's try passing the whole path to the command.

```bash
$ sudo chroot stage3-chroot /usr/bin/ls
chroot: failed to run command ‘/bin/ls’: No such file or directory
```

By calling `chroot`, we are telling the new process "this new subdirectory is `/` to you".
And the new subdirectory is totally empty! So `/usr/bin/ls` can't exist-- there's nothing
at `/` at all! Let's try copying the `ls` binary into `stage3-chroot`:

```bash
$ mkdir -p stage3-chroot/usr/bin
$ sudo cp /usr/bin/ls stage3-chroot/usr/bin/ls
$ sudo chroot stage3-chroot ls
chroot: failed to run command ‘ls’: No such file or directory
```

Still nothing. The command is there, but still can't be found. At this point, the error message
is a bit misleading. It can _find_ the `ls` command but it cannot _execute_ it, because
`ls` dynamically loads several libraries, and once again, the new root is missing those.

```bash
$ ldd /usar/bin/ls
        linux-vdso.so.1 (0x00007ffda3d63000)
        libselinux.so.1 => /lib/x86_64-linux-gnu/libselinux.so.1 (0x00007e02439dc000)
        libc.so.6 => /lib/x86_64-linux-gnu/libc.so.6 (0x00007e0243600000)
        libpcre2-8.so.0 => /lib/x86_64-linux-gnu/libpcre2-8.so.0 (0x00007e0243942000)
        /lib64/ld-linux-x86-64.so.2 (0x00007e0243a38000)
```

> Note: `devbox` messes with `ldd` since it replaces the standard library paths with nix store paths. 
> If you try this on your machine, ensure you disable your devbox shell before running ldd. Even though 
> the nix store libraries should be fine, I noticed it was missing a required selinux library, so
> I created the `stage3-chroot` dir with binaries pulled directly from the system lib/lib64.

So now we need to copy each of those `/lib/*` and `/lib64/*` libraries into `stage3-chroot/lib` and
`stage3-chroot/lib64`:

```bash
$ mkdir stage3-chroot/lib stage3-chroot/lib64
$ cp /lib64/ld-linux-x86-64.so.2 stage3-chroot/lib64
$ cp /lib/x86_64-linux-gnu/libselinux.so.1 /lib/x86_64-linux-gnu/libc.so.6 /lib/x86_64-linux-gnu/libpcre2-8.so.0 stage3-chroot/lib
```

Now finally: 
```bash
$ sudo chroot stage3-chroot ls
lib  lib64 usr
```

Yay! So what have we learned? In order to run a command inside a chroot, we need the binary and any libraries
that the binary loads dynamically, associated files, etc. And in order to see process info, we need `/proc`.
And to configure the processes, we might need stuff in `/etc/`... its starting to feel like we need a whole copy
of the root filesystem inside our container in order to do arbitrary tasks. Indeed, thats how many container images handle
it. 


## Doing it in Go

Now that we know the principles, let's implement it in Go.

All we need to do is add a new config parameter for the root directory to run the container in, populate the field from a flag,
then use `syscall.Chroot` to enter it.

`pkg/container.go`
```go
type Container struct {
	Namespaces NamespaceConfig `json:"namespaces"`
	Detach     bool            `json:"detach"`
	Command    string          `json:"command"`
	Args       []string        `json:"args"`
	Root       string          `json:"root"`
}
```
> Note: I also refactored the `Args` parameter to split into `Command` (the root command) and `Args` (the rest of the args).
> this is more in line with other container systems

`cmd/main.go`
```go
func init() {
	runCmd.Flags().BoolVarP(&detach, "detach", "d", false, "Run container in background")
	runCmd.Flags().StringVarP(&root, "root", "r", "rootfs", "Root directory of container")
	runCmd.MarkFlagRequired("root")
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(killCmd)
}
```
Since there's no way to reasonably set a default value, we mark the flag as required. I won't show adding the var and setting the 
`c.Root` field since that's the same as detach and args.

Now we just have to update `c.Run` to use the new field to chroot. I won't replicate the entire func, but the only requirement
is to add the chroot and chdir before executung `cmd.Run` or `cmd.Start`

`pkg/container/container.go`
```go

func (c *Container) Run() error {
    ...
    syscall.Chroot(c.Root)
    os.Chdir("/")
    ...
}
```

Let's test using our `stage3-chroot` directory:

```zsh
# ./boxr run -r stage3-chroot -- ls
lib  lib64  usr
```

Success! There's plenty more to say about filesystems. In Step 4: Filesystems Part 2 we will
learn about mounts and masks to solve the `/proc` issue that sent us down this path.

## `chroot` is dangereous, take this \[*hands you `pivot_root`*\]

Great source that I rely heavily on for learning how `pivot_root` and `chroot` work together:
`https://tbhaxor.com/pivot-root-vs-chroot-for-containers/`

`chroot` alone gives pretty good filesystem isolation (protecting the host system from whatever happens in the container) 
but its not perfect. If (when) someone does something silly, like run the container in privileged mode, where the process
inside the container has `CAP_SYS_CHROOT`, they can do a "double chroot" exploit and get a root shell on the host. Using 
`pivot_root`, another syscall, before using `chroot` makes it possible to eliminate this exploit.

> Note: As always, its important to point out that none of these mitigations should be relied on entirely for security.
> Containers, even the ones executed via `runc`, are not perfectly secure.

We will dig into this more in a future installment, but I wanted to highlight it now since seeing `chroot` used in
the way we did in this installment may have set off alarm bells in some readers' heads.
