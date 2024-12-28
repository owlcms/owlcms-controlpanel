## Portable launcher and updater for owlcms server

This is meant to provide a small compiled binary to launch owlcms, removing the need for an installer
and providing the same user experience on all platforms.
- allows downloading one or more versions of owlcms
- select which one to run
- if Java is not present, a local copy is downloaded correctly for the current platform.

Currently supported: Windows, Raspberry Pi (Linux ARM64), Linux on Intel (AMD64)

In theory this would work on a Mac, author does not own a Mac to perform the required code signing. Volunteers welcome.

### Usage
- create a directory where the progam will run
- go to the Releases page and download the owlcms launcher for your type of computer
- run the program by double-clicking on it
- Select the latest version of owlcms from the dropdown
- Click Launch
  
### Building Notes:
- must be built on Linux (the fyne go-gl dependency on Windows is broken).
- use fyne-cross to generate the pi and Windows binary.

### To be explored
- download and extract Federation-specific overrides
