## Portable launcher and updater for OWLCMS server

This provides a small compiled binary to launch OWLCMS, removing the need for an installer
and providing the same user experience on all platforms. The program allows
- easy updating from a version to the current one
- launching the program (downloading Java if needed)
- having several versions at once and copying configurations and data

Currently supported: Windows, Raspberry Pi (Linux ARM64), Linux on Intel (AMD64)<br>
Volunteers are welcome to compile and sign the program for the Mac (see [BUILD.md](./BUILD.md)).

![image](https://github.com/user-attachments/assets/6baca710-a65a-4491-a1e5-5ff678bf88f7)


### Usage
1. go to the Releases page and download the owlcms launcher for your type of computer
2. install the launcher and run it.
3. If there is no version of OWLCMS installed, the latest one will be downloaded
4. Click Launch to start OWLCMS
   - This will create a folder called java17 the first time
   - Starting the program takes 10 to 20 seconds depending on your laptop, the time it takes to read in and process the various configuration files and read in the database
5. Once OWLCMS is starting, a Stop button appears.  You can use this to stop the program.
   - If there are other users using OWLCMS, their browsers will stop responding
   - All browsers that are connected to OWLCMS will refresh automatically if you restart it.
   - You can either use the Stop button or the stop icon (X or red dot) at the top of the program
