# Step 3: Filesystems (Part 1) 

As we saw last time, even though we puanched our process in a new `PID` namespace, `ps`
still showed other processes on the host. This was because `ps` works by reading from
`/proc`, which is the same "on the host" and "in the container". In this installment,
we will start our journey of learning how to get filesystem isolation inside the container.

## `chroot`

We need to start by sectioning off the "container" into its own directory on the host filesystem.
But since we want to isolate the process, that directory should look like root `/` to the process in
the container. We can do that using the `chroot` syscall/command. We can test in the CLI:

```bash
$ mkdir testing
$ sudo chroot testing ls
chroot: failed to run command ‘ls’: No such file or directory
```

huh....let's try passing the whole path to the command.

```bash
$ sudo chroot testing /bin/ls
chroot: failed to run command ‘/bin/ls’: No such file or directory
```

By calling `chroot`, we are telling the new process "this new subdirectory is `/` to you".
And the new subdirectory is totally empty! So `/usr/bin/ls` can't exist-- there's nothing
at `/` at all! Let's try copying the `ls` binary into `testing`:

```bash
$ mkdir testing/bin
$ sudo cp /bin/ls testing/bin/ls
$ sudo chroot testing ls
chroot: failed to run command ‘ls’: No such file or directory
```

Still nothing. The command is there, but still can't be found. At this point, the error message
is a bit misleading. It can _find_ the `ls` command but it cannot _execute_ it, because
`ls` dynamically loads several libraries, and once again, the new root is missing those.

```bash
$ ldd /bin/ls
        linux-vdso.so.1 (0x00007ffda3d63000)
        libselinux.so.1 => /lib/x86_64-linux-gnu/libselinux.so.1 (0x00007e02439dc000)
        libc.so.6 => /lib/x86_64-linux-gnu/libc.so.6 (0x00007e0243600000)
        libpcre2-8.so.0 => /lib/x86_64-linux-gnu/libpcre2-8.so.0 (0x00007e0243942000)
        /lib64/ld-linux-x86-64.so.2 (0x00007e0243a38000)
```

So now we need to copy each of those `/lib/*` and `/lib64/*` libraries into `testing/lib` and
`testing/lib64`:

```bash
$ mkdir testing/lib testing/lib64
$ cp /lib64/ld-linux-x86-64.so.2 testing/lib64
$ cp /lib/x86_64-linux-gnu/libselinux.so.1 /lib/x86_64-linux-gnu/libc.so.6 /lib/x86_64-linux-gnu/libpcre2-8.so.0 testing/lib
```

Now finally: 
```bash
$ sudo chroot testing ls
bin  lib  lib64
```

Yay! So what have we learned? In order to run a command inside a chroot, we need the binary and any libraries
that the binary loads dynamically, associated files, etc. And in order to see process info, we need `/proc`.
And to configure the processes, we might need stuff in `/etc/`... its starting to feel like we need a whole copy
of the root filesystem inside our container in order to do arbitrary tasks. Indeed, thats how many container images handle
it. 

## `chroot` is dangereous, take this \[*hands you `pivot_root`*\]

Great source that I rely heavily on for learning how `pivot_root` and `chroot` work together:
`https://tbhaxor.com/pivot-root-vs-chroot-for-containers/`

`chroot` alone gives pretty good filesystem isolation (protecting the host system from whatever happens in the container) 
but its not perfect. If (when) someone does something silly, like run the container in privileged mode, where the process
inside the container has `CAP_SYS_CHROOT`, they can do a "double chroot" exploit and get a root shell on the host. Using 
`pivot_root`, another syscall, before using `chroot` makes it possible to eliminate this exploit.

> Note: As always, its important to point out that none of these mitigations should be relied on entirely for security.
> Containers, even the ones executed via `runc`, are not perfectly secure.

