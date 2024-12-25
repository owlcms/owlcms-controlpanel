Portable launcher for owlcms server

This is meant to provide a small compiled binary to launch owlcms.
Eventually this would download a release and launch java to run it.
Stopping the server would be provided.
Selection of which release to use would be an option.


Building Notes:
- Easier to build on Linux
- Getting all the prerequisites is a pain (a bunch of X11 libs)
- once it compiles and runs on Linux, using fyne-cross to run cross-compiling on Docker is best
  - When cross-compiling for Raspberry Pi, the build will fail unless you create the tmp directory
    (the cleanup removes it, but does not create a new one under linux-arm64)
