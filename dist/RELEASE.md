[comment]: <> (EDIT THIS FILE IN THE dist DIRECTORY ONLY)
Owlcms-launcher is a "control center" for owlcms. 

-  It will automatically download the current version of owlcms when used for the first time. 
- You will be able to start and stop owlcms from the control panel.  
- The launcher can also install updates
- You can also have several versions at once, and copy your database and local changes between versions

> When using owlcms-launcher on Windows or Linux you don't have to use the "old-style" installation methods explained in the documentation.  It is expected that owlcms-launcher will replace them.
>

### Change Log for version _TAG_

[comment]: <> (EDIT THIS FILE IN THE dist DIRECTORY ONLY)

- Redesign of the update process -- the buttons on each version only show what makes sense.

### Installing the Launcher

- For Windows, download `owlcms.exe`  and copy it to your desktop
- For Raspberry Pi
  1. Download the file that ends with `_pi.deb` .
  2. Go to your Downloads directory, right-click on the downloaded file and select `Package Install`.  
     You will be prompted for the password of your pi user. This will create a desktop icon, and an entry in the "Other" section of the system menu.
- For Linux on Intel or AMD computers
  1. Download the file that ends with `_amd64.deb` .
  2. Start a terminal window and go to `~/Downloads`
  3. Use `sudo apt install ./name.deb` (replacing name with the actual name of the .deb file)

If you do not wish to use the package install, or lack privileges to do so, you may also copy the owlcms-pi or owlcms-linux binary file directly to your machine, but then there will be no icons.

### Running OWLCMS using the Launcher

- Double-click on the icon on your desktop
  - This will download the latest version of OWLCMS if there is none installed
  - When launching for the first time, the launcher will fetch an appropriate version of the Java programming language
  - Starting the program takes 20 to 30 seconds, and a browser window should open
  - A Stop button will be shown so you can politely stop the program.
  - If you stop the program, all the browsers will stay open and wait for a restart, so they must be stopped individually.