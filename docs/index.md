# `boxr`: a simple container manager and runtime

The goal of `boxr` is to learn how tools like `runc`, `podman`, `docker`, etc. work.
Mainly, how they isolate processes, how they manage data, and what weird problems they had
to solve to get there.

The end goal is to have a daemonless CLI, `boxr`, that allows me to do things like `boxr run my-command` or
`boxr create my-container && boxr start my-container`, and all of the other common container interactions
(stop, kill, pause, etc.). I am not trying to replace the image build/image management features of Docker,
although pulling images from dockerhub/other sources may be in scope if I feel like it.

The project will likely evolve as it progresses, with early components feeding into later ones.
Early on it will be more similar to a simplified `runc`, with features exapnding from there

## Specifying the interface

- `boxr config <file>` - write a stock config file to <file> with expected defaults

- `boxr run <name>` - launch the container (specified by the config in CWD)

- `boxr create <name>` - create the container specified but do not start any processes

- `boxr start <name>` - start the container specified

- `boxr stop <name>` - stop the processes in the container specified 

- `boxr kill <name>`  - delete all resources associated with container


