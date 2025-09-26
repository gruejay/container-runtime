# Fixing PID 1 in Container Runtime

## Problem

When using reexec to set up namespaces, the reexec'd `boxr` process was becoming PID 1 in the container, rather than the user-specified command. This is problematic because:

1. It wastes resources having an intermediate process
2. It doesn't match the behavior of standard container runtimes
3. The user's process doesn't receive signals sent to PID 1
4. It complicates process management and signal handling

## Solution

Replace the `boxr` process with the user's command using `syscall.Exec()` after namespace setup is complete.

## Implementation Details

### Flow Diagram

```
1. User runs: boxr run /bin/bash
2. First boxr process (no namespaces):
   - Creates container configuration
   - Calls reexec.Reexec() with namespace flags
   
3. Second boxr process (with namespaces, becomes PID 1 initially):
   - Sets up filesystem (chroot, mount /proc)
   - Calls syscall.Exec() to replace itself with /bin/bash
   
4. User's command (/bin/bash) is now PID 1 in container
```

### Key Changes

#### 1. Modified `container.Run()` in `pkg/container/container.go`

Instead of using `exec.Command().Run()` which creates a child process:

```go
// OLD: Creates child process
cmd := exec.Command(c.Command, c.Args...)
cmd.Run()

// NEW: Replaces current process
binary, err := exec.LookPath(c.Command)
args := append([]string{c.Command}, c.Args...)
syscall.Exec(binary, args, os.Environ())
```

#### 2. Detached Mode Handling in `cmd/boxr.go`

For detached mode, we need to fork before reexec to allow the parent to return:

```go
if c.Detach && os.Getenv("_CONTAINER_INIT") != "1" {
    cmd := exec.Command(os.Args[0], os.Args[1:]...)
    cmd.Env = append(os.Environ(), "_CONTAINER_DETACH=1")
    cmd.SysProcAttr = &syscall.SysProcAttr{
        Setsid: true,  // Create new session
    }
    // Redirect to /dev/null for true detachment
    cmd.Stdin = devNull
    cmd.Stdout = devNull
    cmd.Stderr = devNull
    cmd.Start()
    // Parent exits, child continues to reexec flow
}
```

### Benefits

1. **Correct PID 1**: The user's command is PID 1 in the container
2. **No Intermediate Process**: No wasted resources on a supervising process
3. **Proper Signal Handling**: Signals sent to the container go directly to the user's process
4. **Standard Behavior**: Matches how other container runtimes work

### Testing

You can verify the fix works with:

```bash
# Build
go build -o boxr cmd/boxr.go

# Test attached mode
sudo ./boxr run -r rootfs /bin/sh -c 'echo "PID: $$"; ps aux'

# Test with the check_pid.sh script
sudo ./boxr run -r rootfs /check_pid.sh
```

The output should show that the user's command (not `boxr`) is PID 1.

### Note on Detached Mode

In detached mode, the process tree looks like:

1. Original `boxr` forks and exits
2. Forked `boxr` reexecs with namespaces
3. Reexec'd `boxr` uses syscall.Exec to become user's command
4. User's command runs as PID 1 in its own session

This ensures proper detachment while still making the user's command PID 1.