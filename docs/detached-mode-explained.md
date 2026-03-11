# Detached Mode Implementation Explained

## High-Level Overview

The detached mode feature allows a container to run in the background, similar to how Docker's `-d` flag works. When a user runs `boxr run -d`, the container should:

1. Start in the background as a daemon process
2. Detach from the terminal 
3. Continue running even after the parent process exits
4. Log output to a file instead of the terminal

The main challenge was implementing proper Unix daemonization while preserving the container's namespace isolation and ensuring all configuration (especially file paths) survived the multiple process forks required for daemonization.

## Detailed Analysis

### The Original Problem

When implementing detached mode, several issues prevented the container from running properly:

1. **Process Lifetime**: The container process would exit immediately after starting
2. **Path Resolution**: The rootfs path would be lost or incorrectly resolved during the daemonization process
3. **I/O Redirection**: Output wasn't being captured to log files
4. **Process Visibility**: Even when the process started, it wasn't visible with standard tools like `ps`

### Root Causes

#### 1. Incomplete Daemonization
The original implementation only did a single fork with `Setsid`, which is insufficient for proper daemonization. A proper daemon requires:
- Double forking to ensure the process can't reacquire a controlling terminal
- Closing or redirecting all file descriptors
- Changing the working directory to /
- Clearing the umask

#### 2. Path Loss During Re-execution
The container runtime uses a "re-exec" pattern where it re-executes itself with special environment variables to create new namespaces. During detached mode, this happened multiple times:
```
Original process → First fork (detach) → Re-exec (namespaces) → Container init
```

The problem was that relative paths (like `./rootfs`) were being resolved differently after `chdir("/")` in the daemonization process, causing the path to become `/rootfs` instead of `/home/ubuntu/container-runtime/rootfs`.

#### 3. File Descriptor Inheritance
The re-exec process was overwriting carefully set up file descriptors (for logging) with the original stdin/stdout/stderr, causing log output to be lost.

### The Solution

The implementation addresses these issues with a multi-stage approach:

#### Stage 1: Path Resolution
```go
// Convert relative paths to absolute before any forking
absRoot, err := filepath.Abs(root)
if err != nil {
    fmt.Printf("Error getting absolute path: %v\n", err)
    os.Exit(1)
}
c.Root = absRoot
```

#### Stage 2: First Fork (Detachment)
```go
if c.Detach && os.Getenv("_CONTAINER_INIT") != "1" && os.Getenv("_CONTAINER_DETACH") != "1" {
    // Reconstruct args with absolute path
    args := []string{"run", "--detach", "--root", c.Root, c.Command}
    args = append(args, c.Args...)
    cmd := exec.Command(os.Args[0], args...)
    cmd.Env = append(os.Environ(), "_CONTAINER_DETACH=1")
    cmd.SysProcAttr = &syscall.SysProcAttr{
        Setsid: true, // Create new session
    }
    
    if err := cmd.Start(); err != nil {
        fmt.Printf("Error starting detached container: %v\n", err)
        os.Exit(1)
    }
    
    fmt.Printf("Started detached container (PID: %d)\n", cmd.Process.Pid)
    fmt.Printf("Logs will be written to: /var/log/boxr/container-%d.log\n", cmd.Process.Pid)
    os.Exit(0)
}
```

Key improvements:
- Reconstructs the command line with the absolute path instead of using `os.Args[1:]`
- Sets `_CONTAINER_DETACH=1` environment variable to track state
- Creates a new session with `Setsid`

#### Stage 2.5: Daemonization Setup
```go
if os.Getenv("_CONTAINER_DETACH") == "1" && os.Getenv("_CONTAINER_INIT") != "1" {
    logDir := "/var/log/boxr"
    os.MkdirAll(logDir, 0755)
    logFile := fmt.Sprintf("%s/container-%d.log", logDir, os.Getpid())
    
    // Change directory to /
    if err := os.Chdir("/"); err != nil {
        os.Exit(1)
    }
    
    // Clear umask
    syscall.Umask(0)
    
    // Set up environment with log file path
    os.Setenv("_CONTAINER_LOG", logFile)
    
    // Continue with normal flow
}
```

#### Stage 3: I/O Redirection (After Namespace Creation)
```go
if logFile := os.Getenv("_CONTAINER_LOG"); logFile != "" {
    // Open log file for output
    logFd, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
    if err == nil {
        // Redirect stdout and stderr to log file
        os.Stdout = logFd
        os.Stderr = logFd
        // Also redirect for child processes
        syscall.Dup2(int(logFd.Fd()), 1)
        syscall.Dup2(int(logFd.Fd()), 2)
    }
    
    // Redirect stdin to /dev/null
    devNull, err := os.Open("/dev/null")
    if err == nil {
        os.Stdin = devNull
        syscall.Dup2(int(devNull.Fd()), 0)
    }
}
```

This happens after the re-exec but before the final container execution, ensuring logs are captured.

### How It Works Now

1. User runs: `boxr run -d --root ./rootfs sleep 300`
2. Main process converts `./rootfs` to absolute path
3. Main process forks a child with `_CONTAINER_DETACH=1`
4. Main process exits, printing the PID
5. Detached child sets up logging path and daemonizes
6. Detached child re-execs itself with namespace flags
7. New process (in namespaces) redirects I/O to log files
8. Final exec replaces process with user command (`sleep 300`)

The result is a properly daemonized container process that:
- Runs in its own namespaces (PID, mount, UTS, etc.)
- Is detached from the terminal
- Logs to `/var/log/boxr/container-{PID}.log`
- Continues running after the parent exits
- Is visible in the host's process list

### Key Takeaways

1. **Preserve Absolute Paths**: When daemonizing, always convert relative paths to absolute before any `chdir` operations
2. **Track State with Environment Variables**: Use environment variables to track which stage of the multi-fork process you're in
3. **Defer I/O Redirection**: Don't redirect I/O until after all re-executions are complete
4. **Reconstruct Commands Carefully**: When forking, explicitly reconstruct command arguments rather than reusing `os.Args`
5. **Log Everything**: For daemon processes, always provide a way to capture output for debugging