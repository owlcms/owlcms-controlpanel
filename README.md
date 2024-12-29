## Portable launcher and updater for owlcms server

This is meant to provide a small compiled binary to launch owlcms, removing the need for an installer
and providing the same user experience on all platforms.
- allows downloading one or more versions of owlcms
- select which one to run
- if Java is not present, a local copy is downloaded correctly for the current platform.

Currently supported: Windows, Raspberry Pi (Linux ARM64), Linux on Intel (AMD64)

In theory this would work on a Mac, author does not own a Mac to perform the required code signing. Volunteers welcome.

![image](https://github.com/user-attachments/assets/35ab61f3-c1d7-4397-8f9c-b85789c40092)

### Usage
1. go to the Releases page and download the owlcms launcher for your type of computer
2. move the downloaded program to your desktop
3. run the program by double-clicking on it
4. Select the latest version of OWLCMS from the dropdown
   - This will create a folder with that version inside
5. Click Launch to start OWLCMS
   - This will create a folder called java17 the first time
   - Starting the program takes 10 to 20 seconds depending on your laptop, the time it takes to read in and process the various configuration files and read in the database
6. Once OWLCMS is starting, a Stop button appears.  You can use this to stop the program.
   - If there are other users using OWLCMS, their browsers will stop responding
   - All browsers that are connected to OWLCMS will refresh automatically if you restart it.
   - You can either use the Stop button or the stop icon (X or red dot) at the top of the program


If you don't want to clutter your desktop, you can create a folder and move your program there.
