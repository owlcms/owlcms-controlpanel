## Portable launcher and updater for owlcms server

This is meant to provide a small compiled binary to launch owlcms.
- allows downloading one or more versions of owlcms
- select which one to run
- if Java is not present, it is downloaded correctly for the current platform.

Currently supported: Windows, Raspberry Pi (Linux ARM64), Linux on Intel (AMD64)
In theory this would work on a Mac, looking for benevolent person to cross-compile and sign.


Building Notes:
- must be built on Linux (the fyne go-gl dependency on Windows is broken).
- use fyne-cross to generate the pi and Windows binary.
