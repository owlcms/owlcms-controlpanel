[comment]: <> (EDIT THIS FILE IN THE dist DIRECTORY ONLY)
Owlcms-launcher is a "control center" for owlcms.  It will automatically download the current version of owlcms when used for the first time. You will be able to start and stop owlcms from the control panel.  The launcher can also install updates directly, enabling you to test different versions.

When using owlcms-launcher on Windows or Linux you don't have to use the "old-style" installation methods explained in the documentation.  It is expected that owlcms-launcher will replace them.

### Change Log for version _TAG_

- show a correct message when the latest version installed is a prerelease newer than the latest stable release
- added an import button to get database and local changes from a previous install 
- modification times are preserved when downloading

### Installing the Launcher

- For Windows, download `owlcms.exe`  and copy it to your desktop
- For Raspberry Pi, there are two options
  1. **Download the file that ends with `_pi.deb` . Go to your Downloads directory, right-click on the downloaded file and select `Package Install`**.  You will be prompted for the password of your pi user. This will create a desktop icon, and an entry in the "Other" section of the menu.
  2. If you prefer, you can simply download and copy `owlcms-pi` to your desktop -- the only difference is that there will be no pretty icon.
- For Linux on Intel or AMD computers, there are two options
  1. Use a package manager
     1. Download the file that ends with `_amd64.deb` .
     2. Start a terminal window and go to that location
     3. Use `sudo apt install ./name.deb` (replacing name with the actual name of the .deb file)
  2. If you prefer, you can download and copy `owlcms-linux` to your desktop -- the only difference is that there will be no pretty icon.

### Running OWLCMS using the Launcher

- Double-click on the icon on your desktop
  - this will download the latest version of OWLCMS
  - when launching for the first time, the launcher will fetch an appropriate version of the Java programming language
  - starting the program takes 20 to 30 seconds, and a browser window should open
  - a Stop button will be shown so you can politely stop the program.
  - If you stop the program, all the browsers will stay open and wait for a restart, so they must be stopped individually.