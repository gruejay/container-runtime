# boxr container runtime

`boxr` is a simple container runtime project. The goal is to 
implement the basics of runc and a container manager like `podman`.
In short, this means implementing the core ideas of containers (cgroups, 
namespaces, filesystem isolation), basic management of OCI-compliant images
via overlayfs, and a CLI to manage container actions like create, start, run,
kill, pause, delete.

See the [docs](github.com/gruejay/container-runtime/docs)for more detailed notes on the plan and related concepts.

